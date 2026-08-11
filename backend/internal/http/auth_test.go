package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/auth"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store/testdb"
)

func seedUser(t *testing.T, pool *pgxpool.Pool, email, password, role string) uuid.UUID {
	t.Helper()
	var hash string
	if password != "" {
		h, err := auth.HashPassword(password)
		if err != nil {
			t.Fatal(err)
		}
		hash = h
	}
	id := uuid.New()
	if err := store.CreateUser(context.Background(), pool, store.User{
		ID: id, Email: email, PasswordHash: hash,
		DisplayName: "Persona Test", Role: role, IsActive: true,
	}); err != nil {
		t.Fatal(err)
	}
	return id
}

func doLogin(t *testing.T, r http.Handler, email, password string) *httptest.ResponseRecorder {
	t.Helper()
	body := strings.NewReader(`{"email":"` + email + `","password":"` + password + `"}`)
	req := httptest.NewRequest("POST", "/api/v1/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func sessionCookieFrom(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie {
			return c
		}
	}
	t.Fatalf("no %s cookie in response", sessionCookie)
	return nil
}

// loginAs seeds a user and returns a live session cookie for them.
func loginAs(t *testing.T, r http.Handler, pool *pgxpool.Pool, email, role string) *http.Cookie {
	t.Helper()
	seedUser(t, pool, email, "secreta123", role)
	rec := doLogin(t, r, email, "secreta123")
	if rec.Code != 200 {
		t.Fatalf("login as %s: %d %s", email, rec.Code, rec.Body.String())
	}
	return sessionCookieFrom(t, rec)
}

func TestLoginLogoutMe(t *testing.T) {
	pool := testdb.New(t)
	seedUser(t, pool, "p@x.mx", "secreta123", "parroco")
	r := newRouter(t, pool)

	rec := doLogin(t, r, "p@x.mx", "secreta123")
	if rec.Code != 200 {
		t.Fatalf("login: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	cookie := sessionCookieFrom(t, rec)
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie misconfigured: %+v", cookie)
	}

	req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	req.AddCookie(cookie)
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req)
	if rec2.Code != 200 {
		t.Fatalf("me: expected 200, got %d", rec2.Code)
	}
	var me struct {
		User struct {
			Role  string `json:"role"`
			Email string `json:"email"`
		} `json:"user"`
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &me); err != nil || me.User.Role != "parroco" {
		t.Errorf("me body: %s", rec2.Body.String())
	}

	req = httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
	req.AddCookie(cookie)
	rec3 := httptest.NewRecorder()
	r.ServeHTTP(rec3, req)
	if rec3.Code != 204 {
		t.Fatalf("logout: expected 204, got %d", rec3.Code)
	}

	req = httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	req.AddCookie(cookie)
	rec4 := httptest.NewRecorder()
	r.ServeHTTP(rec4, req)
	if rec4.Code != 401 {
		t.Errorf("me after logout: expected 401, got %d", rec4.Code)
	}
}

func TestLoginRejectsBadCredentialsUniformly(t *testing.T) {
	pool := testdb.New(t)
	seedUser(t, pool, "p@x.mx", "secreta123", "parroco")
	r := newRouter(t, pool)

	a := doLogin(t, r, "p@x.mx", "equivocada")
	b := doLogin(t, r, "no-existe@x.mx", "loquesea")
	if a.Code != 401 || b.Code != 401 {
		t.Fatalf("expected 401/401, got %d/%d", a.Code, b.Code)
	}
	if a.Body.String() != b.Body.String() {
		t.Errorf("wrong-password and unknown-email responses must be identical:\n%s\n%s",
			a.Body.String(), b.Body.String())
	}
}

func TestInactiveUserCannotLogIn(t *testing.T) {
	pool := testdb.New(t)
	id := seedUser(t, pool, "ex@x.mx", "secreta123", "secretaria")
	if err := store.SetUserActive(context.Background(), pool, id, false); err != nil {
		t.Fatal(err)
	}
	r := newRouter(t, pool)
	if rec := doLogin(t, r, "ex@x.mx", "secreta123"); rec.Code != 401 {
		t.Errorf("deactivated user: expected 401, got %d", rec.Code)
	}
}

func TestLoginRateLimited(t *testing.T) {
	pool := testdb.New(t)
	r := newRouter(t, pool)
	var last int
	for i := 0; i < 6; i++ {
		last = doLogin(t, r, "x@x.mx", "nope").Code
	}
	if last != 429 {
		t.Errorf("6th attempt from one IP: expected 429, got %d", last)
	}
}

func TestAdminRequiresSession(t *testing.T) {
	pool := testdb.New(t)
	r := newRouter(t, pool)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/admin/events", nil))
	if rec.Code != 401 {
		t.Errorf("admin without cookie: expected 401, got %d", rec.Code)
	}
}
