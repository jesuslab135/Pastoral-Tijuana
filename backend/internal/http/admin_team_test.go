package httpapi

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store/testdb"
)

func TestChannelLifecycle(t *testing.T) {
	pool := testdb.New(t)
	r := newRouter(t, pool)
	cookie := loginAs(t, r, pool, "sec@x.mx", "secretaria")

	body := `{"kind":"whatsapp","name":"Avisos parroquia","target":"1234@g.us","group_id":null}`
	rec := authed(t, r, cookie, "POST", "/api/v1/admin/channels", body)
	if rec.Code != 201 {
		t.Fatalf("create: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Channel struct {
			ID       string `json:"id"`
			IsActive bool   `json:"is_active"`
		} `json:"channel"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if !created.Channel.IsActive {
		t.Error("a new channel should default to active")
	}
	id := created.Channel.ID

	rec = authed(t, r, cookie, "GET", "/api/v1/admin/channels", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Avisos parroquia") {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}

	upd := `{"kind":"whatsapp","name":"Avisos generales","target":"1234@g.us","group_id":null,"is_active":false}`
	rec = authed(t, r, cookie, "PUT", "/api/v1/admin/channels/"+id, upd)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Avisos generales") {
		t.Fatalf("update: %d %s", rec.Code, rec.Body.String())
	}

	rec = authed(t, r, cookie, "DELETE", "/api/v1/admin/channels/"+id, "")
	if rec.Code != 204 {
		t.Fatalf("delete: expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = authed(t, r, cookie, "PUT", "/api/v1/admin/channels/"+id, upd)
	if rec.Code != 404 {
		t.Errorf("update after delete: expected 404, got %d", rec.Code)
	}
}

func TestChannelValidation(t *testing.T) {
	pool := testdb.New(t)
	r := newRouter(t, pool)
	cookie := loginAs(t, r, pool, "sec@x.mx", "secretaria")

	for name, body := range map[string]string{
		"bad kind":  `{"kind":"paloma","name":"X","target":"t"}`,
		"no name":   `{"kind":"email","name":"","target":"t"}`,
		"no target": `{"kind":"email","name":"X","target":""}`,
		"bad json":  `{"kind":`,
	} {
		if rec := authed(t, r, cookie, "POST", "/api/v1/admin/channels", body); rec.Code != 400 {
			t.Errorf("%s: expected 400, got %d: %s", name, rec.Code, rec.Body.String())
		}
	}
}

func TestUsersEndpointsAreParrocoOnly(t *testing.T) {
	pool := testdb.New(t)
	r := newRouter(t, pool)
	cookie := loginAs(t, r, pool, "sec@x.mx", "secretaria")

	for _, url := range []string{"/api/v1/admin/users"} {
		if rec := authed(t, r, cookie, "GET", url, ""); rec.Code != 403 {
			t.Errorf("secretaria on %s: expected 403, got %d", url, rec.Code)
		}
	}
	// She can still manage events and channels.
	if rec := authed(t, r, cookie, "GET", "/api/v1/admin/channels", ""); rec.Code != 200 {
		t.Errorf("secretaria on channels: expected 200, got %d", rec.Code)
	}
}

func TestParrocoManagesTeam(t *testing.T) {
	pool := testdb.New(t)
	r := newRouter(t, pool)
	cookie := loginAs(t, r, pool, "parroco@x.mx", "parroco")

	// Create a secretaria with a password.
	body := `{"email":"nueva@x.mx","display_name":"Nueva","role":"secretaria","password":"clave12345"}`
	rec := authed(t, r, cookie, "POST", "/api/v1/admin/users", body)
	if rec.Code != 201 {
		t.Fatalf("create: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var created struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	// She can log in...
	hers := doLogin(t, r, "nueva@x.mx", "clave12345")
	if hers.Code != 200 {
		t.Fatalf("new user login: expected 200, got %d", hers.Code)
	}
	herCookie := sessionCookieFrom(t, hers)

	// ...until the párroco deactivates her, which also kills her session.
	rec = authed(t, r, cookie, "POST", "/api/v1/admin/users/"+created.User.ID+"/deactivate", "")
	if rec.Code != 200 {
		t.Fatalf("deactivate: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	me := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	me.AddCookie(herCookie)
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, me)
	if rec2.Code != 401 {
		t.Errorf("deactivated user's live session: expected 401, got %d", rec2.Code)
	}
	if again := doLogin(t, r, "nueva@x.mx", "clave12345"); again.Code != 401 {
		t.Errorf("deactivated user login: expected 401, got %d", again.Code)
	}

	// Reactivating lets her back in.
	rec = authed(t, r, cookie, "POST", "/api/v1/admin/users/"+created.User.ID+"/activate", "")
	if rec.Code != 200 {
		t.Fatalf("activate: expected 200, got %d", rec.Code)
	}
	if again := doLogin(t, r, "nueva@x.mx", "clave12345"); again.Code != 200 {
		t.Errorf("reactivated user login: expected 200, got %d", again.Code)
	}
}

func TestParrocoCannotLockThemselvesOut(t *testing.T) {
	pool := testdb.New(t)
	r := newRouter(t, pool)
	cookie := loginAs(t, r, pool, "parroco@x.mx", "parroco")

	rec := authed(t, r, cookie, "GET", "/api/v1/auth/me", "")
	var me struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &me); err != nil {
		t.Fatal(err)
	}
	rec = authed(t, r, cookie, "POST", "/api/v1/admin/users/"+me.User.ID+"/deactivate", "")
	if rec.Code != 400 {
		t.Errorf("self-deactivation: expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDuplicateEmailRejected(t *testing.T) {
	pool := testdb.New(t)
	r := newRouter(t, pool)
	cookie := loginAs(t, r, pool, "parroco@x.mx", "parroco")

	body := `{"email":"repetida@x.mx","display_name":"A","role":"secretaria"}`
	if rec := authed(t, r, cookie, "POST", "/api/v1/admin/users", body); rec.Code != 201 {
		t.Fatalf("first create: %d %s", rec.Code, rec.Body.String())
	}
	if rec := authed(t, r, cookie, "POST", "/api/v1/admin/users", body); rec.Code != 409 {
		t.Errorf("duplicate email: expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUserValidation(t *testing.T) {
	pool := testdb.New(t)
	r := newRouter(t, pool)
	cookie := loginAs(t, r, pool, "parroco@x.mx", "parroco")

	for name, body := range map[string]string{
		"no email":  `{"email":"","role":"secretaria"}`,
		"bad email": `{"email":"sin-arroba","role":"secretaria"}`,
		"bad role":  `{"email":"a@x.mx","role":"obispo"}`,
	} {
		if rec := authed(t, r, cookie, "POST", "/api/v1/admin/users", body); rec.Code != 400 {
			t.Errorf("%s: expected 400, got %d: %s", name, rec.Code, rec.Body.String())
		}
	}
}
