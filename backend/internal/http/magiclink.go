package httpapi

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/auth"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/clientip"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/config"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/mail"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/ratelimit"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
)

const magicSubject = "Tu enlace de acceso · Calendario Pastoral"

// magicLinkRequestHandler always answers the same way, whether or not the
// address belongs to a real account: a different answer would let anyone
// enumerate the parish team.
func magicLinkRequestHandler(pool *pgxpool.Pool, mailer mail.Mailer, cfg config.Config,
	ips *clientip.Resolver, limiter *ratelimit.Limiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow(ips.FromRequest(r)) {
			writeError(w, http.StatusTooManyRequests, "rate_limited",
				"Demasiados intentos. Espera un minuto.")
			return
		}
		var in struct {
			Email string `json:"email"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "Cuerpo JSON inválido.")
			return
		}
		ok := map[string]any{"message": "Si el correo existe, enviamos un enlace."}

		u, err := store.GetUserByEmail(r.Context(), pool, strings.TrimSpace(in.Email))
		if err != nil || !u.IsActive {
			writeJSON(w, http.StatusOK, ok)
			return
		}
		token, _, err := auth.IssueMagicToken(cfg.AuthSecret, u.ID, time.Now())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal",
				"No se pudo generar el enlace.")
			return
		}
		link := strings.TrimSuffix(cfg.PublicBaseURL, "/") +
			"/api/v1/auth/magic-link/verify?token=" + token
		body := "Hola" + displayName(u) + ",\n\n" +
			"Entra al Calendario Pastoral con este enlace (válido 15 minutos, un solo uso):\n\n" +
			link + "\n\nSi no lo pediste, ignora este correo.\n"
		if err := mailer.Send(r.Context(), u.Email, magicSubject, body); err != nil {
			log.Printf("magic link: no se pudo enviar a %s: %v", u.Email, err)
			writeError(w, http.StatusInternalServerError, "internal",
				"No se pudo enviar el correo.")
			return
		}
		writeJSON(w, http.StatusOK, ok)
	}
}

func displayName(u store.User) string {
	if u.DisplayName == "" {
		return ""
	}
	return " " + u.DisplayName
}

// magicLinkVerifyHandler spends the token's jti in Redis before creating the
// session, so a link works exactly once. If Redis is unreachable the request
// fails closed rather than granting an unbounded-reuse link.
func magicLinkVerifyHandler(pool *pgxpool.Pool, rdb *redis.Client, cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		invalid := func() {
			writeError(w, http.StatusUnauthorized, "enlace_invalido",
				"El enlace no es válido o ya se usó. Pide uno nuevo.")
		}
		userID, jti, err := auth.ParseMagicToken(cfg.AuthSecret, r.URL.Query().Get("token"), time.Now())
		if err != nil {
			invalid()
			return
		}
		spent, err := rdb.SetNX(r.Context(), "magic:jti:"+jti, "1", auth.MagicLinkTTL).Result()
		if err != nil {
			log.Printf("magic link: redis no disponible: %v", err)
			invalid()
			return
		}
		if !spent {
			invalid()
			return
		}
		u, err := store.GetUserByID(r.Context(), pool, userID)
		if err != nil || !u.IsActive {
			invalid()
			return
		}
		token, err := store.CreateSession(r.Context(), pool, u.ID, "", r.UserAgent())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal",
				"No se pudo iniciar sesión.")
			return
		}
		setSessionCookie(w, r, token, store.SessionTTL)
		writeJSON(w, http.StatusOK, map[string]any{"user": userJSON(u)})
	}
}
