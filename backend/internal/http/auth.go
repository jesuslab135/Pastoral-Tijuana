package httpapi

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/auth"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/clientip"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/ratelimit"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
)

type ctxKey int

const userKey ctxKey = 0

const sessionCookie = "pc_session"

// setSessionCookie writes the session cookie. Secure is dropped on localhost
// so development over plain HTTP still works.
func setSessionCookie(w http.ResponseWriter, r *http.Request, token string, ttl time.Duration) {
	host := r.Host
	if h, _, err := net.SplitHostPort(r.Host); err == nil {
		host = h
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int(ttl.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   host != "localhost" && host != "127.0.0.1",
	})
}

func userJSON(u store.User) map[string]any {
	return map[string]any{
		"id":           u.ID,
		"email":        u.Email,
		"display_name": u.DisplayName,
		"role":         u.Role,
		"is_active":    u.IsActive,
	}
}

func loginHandler(pool *pgxpool.Pool, ips *clientip.Resolver, limiter *ratelimit.Limiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := ips.FromRequest(r)
		if !limiter.Allow(ip) {
			writeError(w, http.StatusTooManyRequests, "rate_limited",
				"Demasiados intentos. Espera un minuto.")
			return
		}
		var in struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "Cuerpo JSON inválido.")
			return
		}
		// One response for every failure mode: a different body or status for
		// "unknown email" would let anyone enumerate the parish team.
		fail := func() {
			writeError(w, http.StatusUnauthorized, "credenciales_invalidas",
				"Correo o contraseña incorrectos.")
		}
		u, err := store.GetUserByEmail(r.Context(), pool, strings.TrimSpace(in.Email))
		if err != nil || !u.IsActive || u.PasswordHash == "" ||
			!auth.VerifyPassword(in.Password, u.PasswordHash) {
			fail()
			return
		}
		token, err := store.CreateSession(r.Context(), pool, u.ID, ip, r.UserAgent())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal",
				"No se pudo iniciar sesión.")
			return
		}
		setSessionCookie(w, r, token, store.SessionTTL)
		writeJSON(w, http.StatusOK, map[string]any{"user": userJSON(u)})
	}
}

func logoutHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(sessionCookie); err == nil {
			_ = store.RevokeSession(r.Context(), pool, c.Value)
		}
		// A negative duration yields MaxAge -1, which deletes the cookie.
		setSessionCookie(w, r, "", -time.Second)
		w.WriteHeader(http.StatusNoContent)
	}
}

func requireSession(pool *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, err := r.Cookie(sessionCookie)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "no_autenticado", "Inicia sesión.")
				return
			}
			u, err := store.ValidateSession(r.Context(), pool, c.Value)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "no_autenticado", "Inicia sesión.")
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey, u)))
		})
	}
}

func currentUser(r *http.Request) store.User {
	u, _ := r.Context().Value(userKey).(store.User)
	return u
}

func requireParroco(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if currentUser(r).Role != "parroco" {
			writeError(w, http.StatusForbidden, "prohibido",
				"Solo el párroco puede hacer esto.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func meHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"user": userJSON(currentUser(r))})
	}
}

func notImplementedYet(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotImplemented, "no_implementado", "Aún no disponible.")
}
