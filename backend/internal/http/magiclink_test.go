package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store/testdb"
)

var tokenRe = regexp.MustCompile(`token=([A-Za-z0-9_\-.]+)`)

func requestMagicLink(t *testing.T, r http.Handler, email string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/api/v1/auth/magic-link",
		strings.NewReader(`{"email":"`+email+`"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func verifyMagicLink(t *testing.T, r http.Handler, token string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET",
		"/api/v1/auth/magic-link/verify?token="+token, nil))
	return rec
}

func TestMagicLinkLogsInOnceOnly(t *testing.T) {
	pool := testdb.New(t)
	r, mailbox := newRouterWithMail(t, pool)
	seedUser(t, pool, "p@x.mx", "", "parroco") // no password: magic link only

	if rec := requestMagicLink(t, r, "p@x.mx"); rec.Code != 200 {
		t.Fatalf("request: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	m := tokenRe.FindStringSubmatch(mailbox.String())
	if m == nil {
		t.Fatalf("no token in the sent mail:\n%s", mailbox.String())
	}
	token := m[1]

	rec := verifyMagicLink(t, r, token)
	if rec.Code != 200 {
		t.Fatalf("verify: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	cookie := sessionCookieFrom(t, rec)

	// The session works.
	req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	req.AddCookie(cookie)
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req)
	if rec2.Code != 200 {
		t.Errorf("me with magic-link session: expected 200, got %d", rec2.Code)
	}

	// Replaying the same link must fail: the jti is spent.
	if again := verifyMagicLink(t, r, token); again.Code != 401 {
		t.Errorf("replayed link: expected 401, got %d", again.Code)
	}
}

func TestMagicLinkDoesNotRevealUnknownEmail(t *testing.T) {
	pool := testdb.New(t)
	r, mailbox := newRouterWithMail(t, pool)
	seedUser(t, pool, "real@x.mx", "", "secretaria")

	known := requestMagicLink(t, r, "real@x.mx")
	mailbox.Reset()
	unknown := requestMagicLink(t, r, "fantasma@x.mx")

	if known.Code != unknown.Code || known.Body.String() != unknown.Body.String() {
		t.Errorf("responses must be identical:\n%d %s\n%d %s",
			known.Code, known.Body.String(), unknown.Code, unknown.Body.String())
	}
	if mailbox.Len() != 0 {
		t.Errorf("no mail may be sent for an unknown address:\n%s", mailbox.String())
	}
}

func TestMagicLinkRejectsGarbageToken(t *testing.T) {
	pool := testdb.New(t)
	r := newRouter(t, pool)
	if rec := verifyMagicLink(t, r, "no-es-un-token"); rec.Code != 401 {
		t.Errorf("garbage token: expected 401, got %d", rec.Code)
	}
}

func TestMagicLinkRejectsDeactivatedUser(t *testing.T) {
	pool := testdb.New(t)
	r, mailbox := newRouterWithMail(t, pool)
	id := seedUser(t, pool, "ex@x.mx", "", "secretaria")

	requestMagicLink(t, r, "ex@x.mx")
	m := tokenRe.FindStringSubmatch(mailbox.String())
	if m == nil {
		t.Fatalf("no token in mail:\n%s", mailbox.String())
	}
	// Deactivated between issuing the link and clicking it.
	if err := store.SetUserActive(context.Background(), pool, id, false); err != nil {
		t.Fatal(err)
	}
	if rec := verifyMagicLink(t, r, m[1]); rec.Code != 401 {
		t.Errorf("deactivated user: expected 401, got %d", rec.Code)
	}
}
