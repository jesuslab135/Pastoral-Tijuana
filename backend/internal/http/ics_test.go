package httpapi

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store/testdb"
)

func TestICSFeed(t *testing.T) {
	pool := testdb.New(t)
	now := time.Now()
	err := store.CreateEvent(context.Background(), pool, store.Event{
		ID: uuid.New(), Title: "Hora santa",
		GroupID: uuid.MustParse(liturgiaID), Rank: "parroquial",
		StartsAt:    now.Add(48 * time.Hour),
		EndsAt:      now.Add(49 * time.Hour),
		PublishedAt: &now,
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	r := newRouter(t, pool)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/calendario.ics", nil))

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/calendar; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
	if !strings.Contains(rec.Body.String(), "SUMMARY:Hora santa") {
		t.Errorf("feed missing event:\n%s", rec.Body.String())
	}

	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("missing ETag")
	}
	req := httptest.NewRequest("GET", "/calendario.ics", nil)
	req.Header.Set("If-None-Match", etag)
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req)
	if rec2.Code != 304 {
		t.Errorf("expected 304 with matching ETag, got %d", rec2.Code)
	}
}

func TestICSGroupFeedAndUnknownSlug(t *testing.T) {
	pool := testdb.New(t)
	r := newRouter(t, pool)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/calendario/liturgia.ics", nil))
	if rec.Code != 200 {
		t.Errorf("liturgia feed: expected 200, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/calendario/nope.ics", nil))
	if rec.Code != 404 {
		t.Errorf("unknown slug: expected 404, got %d", rec.Code)
	}
}
