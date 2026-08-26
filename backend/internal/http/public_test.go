package httpapi

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store/testdb"
)

const liturgiaID = "a1000000-0000-4000-8000-000000000001"

func TestGetEvents(t *testing.T) {
	pool := testdb.New(t)
	now := time.Now()
	err := store.CreateEvent(context.Background(), pool, store.Event{
		ID: uuid.New(), Title: "Hora santa",
		GroupID: uuid.MustParse(liturgiaID), Rank: "parroquial",
		StartsAt:    time.Date(2026, 8, 4, 19, 0, 0, 0, time.UTC),
		EndsAt:      time.Date(2026, 8, 4, 20, 0, 0, 0, time.UTC),
		PublishedAt: &now,
	})
	if err != nil {
		t.Fatalf("seed event: %v", err)
	}

	r := newRouter(t, pool)
	req := httptest.NewRequest("GET", "/api/v1/events?from=2026-08-01&to=2026-09-01", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=0, must-revalidate" {
		t.Errorf("Cache-Control = %q", cc)
	}
	var body struct {
		Events []struct {
			Title string `json:"title"`
			Color string `json:"color"`
			Group struct {
				Slug string `json:"slug"`
			} `json:"group"`
		} `json:"events"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if len(body.Events) != 1 || body.Events[0].Color != "verde" ||
		body.Events[0].Group.Slug != "liturgia" {
		t.Errorf("unexpected body: %s", rec.Body.String())
	}
}

func TestGetEventsBadRange(t *testing.T) {
	pool := testdb.New(t)
	r := newRouter(t, pool)
	for _, url := range []string{
		"/api/v1/events", // missing params
		"/api/v1/events?from=chido&to=2026-09-01",      // invalid date
		"/api/v1/events?from=2026-01-01&to=2028-01-01", // > 400 days
	} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest("GET", url, nil))
		if rec.Code != 400 {
			t.Errorf("%s: expected 400, got %d", url, rec.Code)
		}
		var e struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil || e.Error.Code != "bad_request" {
			t.Errorf("%s: expected bad_request error shape, got %s", url, rec.Body.String())
		}
	}
}

func TestGetSeasonsBadYear(t *testing.T) {
	pool := testdb.New(t)
	r := newRouter(t, pool)
	// "0000" parses in Go but has no counterpart in Postgres, so it must be
	// rejected as client error rather than reaching make_date and 500ing.
	for _, url := range []string{
		"/api/v1/seasons?year=0000",
		"/api/v1/seasons?year=chido",
		"/api/v1/seasons?year=26",
	} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest("GET", url, nil))
		if rec.Code != 400 {
			t.Errorf("%s: expected 400, got %d: %s", url, rec.Code, rec.Body.String())
		}
		var e struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil || e.Error.Code != "bad_request" {
			t.Errorf("%s: expected bad_request error shape, got %s", url, rec.Body.String())
		}
	}
}

func TestGetSeasonsAndGroups(t *testing.T) {
	pool := testdb.New(t)
	r := newRouter(t, pool)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/seasons?year=2026", nil))
	if rec.Code != 200 {
		t.Fatalf("seasons: expected 200, got %d", rec.Code)
	}
	var sb struct {
		Seasons []struct {
			Name, Color, Start, End string
		} `json:"seasons"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &sb); err != nil || len(sb.Seasons) < 6 {
		t.Errorf("seasons body: %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/groups", nil))
	if rec.Code != 200 {
		t.Fatalf("groups: expected 200, got %d", rec.Code)
	}
	var gb struct {
		Groups []struct{ Slug string } `json:"groups"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &gb); err != nil || len(gb.Groups) != 6 {
		t.Errorf("groups body: %s", rec.Body.String())
	}
}

// The calendar has to drop what the panel deletes. A fixed max-age let the
// browser keep serving a cancelled event for minutes with no way to ask
// whether anything had changed, which read as a deletion that never applied.
func TestGetEventsRevalidatesAfterDelete(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	now := time.Now()
	id := uuid.New()
	err := store.CreateEvent(ctx, pool, store.Event{
		ID: id, Title: "Hora Santa",
		GroupID: uuid.MustParse(liturgiaID), Rank: "parroquial",
		StartsAt:    time.Date(2027, 3, 4, 19, 0, 0, 0, time.UTC),
		EndsAt:      time.Date(2027, 3, 4, 20, 0, 0, 0, time.UTC),
		PublishedAt: &now,
	})
	if err != nil {
		t.Fatalf("seed event: %v", err)
	}
	r := newRouter(t, pool)

	get := func(ifNoneMatch string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "/api/v1/events?from=2027-03-01&to=2027-04-01", nil)
		if ifNoneMatch != "" {
			req.Header.Set("If-None-Match", ifNoneMatch)
		}
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}

	first := get("")
	if first.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", first.Code, first.Body.String())
	}
	etag := first.Header().Get("ETag")
	if etag == "" {
		t.Fatal("no ETag on a public response, so the browser can never revalidate")
	}

	// Nothing moved: the conditional request is answered without a body.
	if again := get(etag); again.Code != 304 {
		t.Errorf("unchanged calendar: expected 304, got %d", again.Code)
	}

	// The párroco deletes the event without a cancellation notice, which
	// removes the row outright.
	if err := store.DeleteEvent(ctx, pool, id, false); err != nil {
		t.Fatalf("delete event: %v", err)
	}

	after := get(etag)
	if after.Code != 200 {
		t.Fatalf("deleted event: expected a fresh 200, got %d", after.Code)
	}
	if after.Header().Get("ETag") == etag {
		t.Error("ETag unchanged after a delete, so caches would keep the stale calendar")
	}
	var body struct {
		Events []struct {
			Title string `json:"title"`
		} `json:"events"`
	}
	if err := json.Unmarshal(after.Body.Bytes(), &body); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if len(body.Events) != 0 {
		t.Errorf("deleted event still served: %s", after.Body.String())
	}
}
