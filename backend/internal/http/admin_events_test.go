package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store/testdb"
)

const liturgiaGroup = "a1000000-0000-4000-8000-000000000001"

func authed(t *testing.T, r http.Handler, cookie *http.Cookie, method, url, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, url, nil)
	} else {
		req = httptest.NewRequest(method, url, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func eventIDFrom(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Event struct {
			ID string `json:"id"`
		} `json:"event"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("bad json: %v (%s)", err, rec.Body.String())
	}
	return body.Event.ID
}

const draftBody = `{"title":"Hora santa","description":"","place":"Templo",
	"starts_at":"2026-08-04T19:00:00Z","ends_at":"2026-08-04T20:00:00Z",
	"group_id":"` + liturgiaGroup + `","rank":"parroquial"}`

func TestAdminEventLifecycle(t *testing.T) {
	pool := testdb.New(t)
	r := newRouter(t, pool)
	cookie := loginAs(t, r, pool, "sec@x.mx", "secretaria")

	// Create as a draft.
	rec := authed(t, r, cookie, "POST", "/api/v1/admin/events", draftBody)
	if rec.Code != 201 {
		t.Fatalf("create: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	id := eventIDFrom(t, rec)

	// A draft is visible to the admin but not to the public.
	rec = authed(t, r, cookie, "GET", "/api/v1/admin/events?from=2026-08-01&to=2026-09-01", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Hora santa") {
		t.Fatalf("admin list: %d %s", rec.Code, rec.Body.String())
	}
	pub := httptest.NewRecorder()
	r.ServeHTTP(pub, httptest.NewRequest("GET", "/api/v1/events?from=2026-08-01&to=2026-09-01", nil))
	if strings.Contains(pub.Body.String(), "Hora santa") {
		t.Error("an unpublished event must not appear in the public API")
	}

	// Publish, and it becomes public.
	rec = authed(t, r, cookie, "POST", "/api/v1/admin/events/"+id+"/publish", "")
	if rec.Code != 200 {
		t.Fatalf("publish: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	pub = httptest.NewRecorder()
	r.ServeHTTP(pub, httptest.NewRequest("GET", "/api/v1/events?from=2026-08-01&to=2026-09-01", nil))
	if !strings.Contains(pub.Body.String(), "Hora santa") {
		t.Error("a published event must appear in the public API")
	}

	// Publishing twice is a conflict, not a second announcement.
	rec = authed(t, r, cookie, "POST", "/api/v1/admin/events/"+id+"/publish", "")
	if rec.Code != 409 {
		t.Errorf("double publish: expected 409, got %d", rec.Code)
	}

	// Unpublish hides it again.
	rec = authed(t, r, cookie, "POST", "/api/v1/admin/events/"+id+"/unpublish", "")
	if rec.Code != 200 {
		t.Fatalf("unpublish: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	pub = httptest.NewRecorder()
	r.ServeHTTP(pub, httptest.NewRequest("GET", "/api/v1/events?from=2026-08-01&to=2026-09-01", nil))
	if strings.Contains(pub.Body.String(), "Hora santa") {
		t.Error("an unpublished event must disappear from the public API")
	}

	// Delete without notifying removes it outright.
	rec = authed(t, r, cookie, "DELETE", "/api/v1/admin/events/"+id+"?notify=false", "")
	if rec.Code != 204 {
		t.Fatalf("delete: expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = authed(t, r, cookie, "GET", "/api/v1/admin/events/"+id, "")
	if rec.Code != 404 {
		t.Errorf("after delete: expected 404, got %d", rec.Code)
	}
}

func TestAdminEventValidation(t *testing.T) {
	pool := testdb.New(t)
	r := newRouter(t, pool)
	cookie := loginAs(t, r, pool, "sec@x.mx", "secretaria")

	for name, body := range map[string]string{
		"no title":         `{"title":"","starts_at":"2026-08-04T19:00:00Z","ends_at":"2026-08-04T20:00:00Z","group_id":"` + liturgiaGroup + `","rank":"parroquial"}`,
		"end before start": `{"title":"X","starts_at":"2026-08-04T21:00:00Z","ends_at":"2026-08-04T20:00:00Z","group_id":"` + liturgiaGroup + `","rank":"parroquial"}`,
		"bad rank":         `{"title":"X","starts_at":"2026-08-04T19:00:00Z","ends_at":"2026-08-04T20:00:00Z","group_id":"` + liturgiaGroup + `","rank":"inventado"}`,
		"malformed json":   `{"title":`,
	} {
		rec := authed(t, r, cookie, "POST", "/api/v1/admin/events", body)
		if rec.Code != 400 {
			t.Errorf("%s: expected 400, got %d: %s", name, rec.Code, rec.Body.String())
		}
	}

	// A group that does not exist is client error, not a 500.
	ghost := `{"title":"X","starts_at":"2026-08-04T19:00:00Z","ends_at":"2026-08-04T20:00:00Z",
		"group_id":"a1000000-0000-4000-8000-0000000000aa","rank":"parroquial"}`
	if rec := authed(t, r, cookie, "POST", "/api/v1/admin/events", ghost); rec.Code != 400 {
		t.Errorf("unknown group: expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminEventNotFound(t *testing.T) {
	pool := testdb.New(t)
	r := newRouter(t, pool)
	cookie := loginAs(t, r, pool, "sec@x.mx", "secretaria")

	for _, url := range []string{
		"/api/v1/admin/events/no-es-uuid",
		"/api/v1/admin/events/a1000000-0000-4000-8000-0000000000ff",
	} {
		if rec := authed(t, r, cookie, "GET", url, ""); rec.Code != 404 {
			t.Errorf("%s: expected 404, got %d", url, rec.Code)
		}
	}
}

func TestDeleteWithNotifyKeepsEventForICS(t *testing.T) {
	pool := testdb.New(t)
	r := newRouter(t, pool)
	cookie := loginAs(t, r, pool, "sec@x.mx", "secretaria")

	rec := authed(t, r, cookie, "POST", "/api/v1/admin/events", draftBody)
	id := eventIDFrom(t, rec)
	if rec2 := authed(t, r, cookie, "POST", "/api/v1/admin/events/"+id+"/publish", ""); rec2.Code != 200 {
		t.Fatalf("publish: %d %s", rec2.Code, rec2.Body.String())
	}
	if rec2 := authed(t, r, cookie, "DELETE", "/api/v1/admin/events/"+id, ""); rec2.Code != 204 {
		t.Fatalf("delete: %d %s", rec2.Code, rec2.Body.String())
	}
	// Soft cancelled: still there, so subscribed phones learn it is off.
	if rec2 := authed(t, r, cookie, "GET", "/api/v1/admin/events/"+id, ""); rec2.Code != 200 {
		t.Errorf("notified deletion must keep the row, got %d", rec2.Code)
	}
}
