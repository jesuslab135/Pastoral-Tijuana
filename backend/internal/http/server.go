package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/clientip"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/config"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/mail"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/ratelimit"
)

const (
	loginRateLimit  = 5
	loginRateWindow = time.Minute
)

func NewRouter(pool *pgxpool.Pool, rdb *redis.Client, mailer mail.Mailer, cfg config.Config) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", healthHandler(pool))

	r.Route("/api/v1", func(api chi.Router) {
		api.Get("/events", eventsHandler(pool, cfg.ParishTZ))
		api.Get("/seasons", seasonsHandler(pool))
		api.Get("/groups", groupsHandler(pool))
	})

	feed := icsHandler(pool, cfg)
	r.Get("/calendario.ics", feed)
	r.Get("/calendario/{slug}", feed)

	ips, err := clientip.NewResolver(cfg.TrustedProxy)
	if err != nil {
		// A malformed TRUSTED_PROXY is a config bug; main validates it too
		// and refuses to boot, so this only fires for routers built in tests.
		panic("TRUSTED_PROXY inválido: " + err.Error())
	}
	loginLimiter := ratelimit.New(loginRateLimit, loginRateWindow)

	r.Route("/api/v1/auth", func(a chi.Router) {
		a.Post("/login", loginHandler(pool, ips, loginLimiter))
		a.Post("/logout", logoutHandler(pool))
		a.With(requireSession(pool)).Get("/me", meHandler())
		a.Post("/magic-link", magicLinkRequestHandler(pool, mailer, cfg, ips, loginLimiter))
		a.Get("/magic-link/verify", magicLinkVerifyHandler(pool, rdb, cfg))
	})

	r.Route("/api/v1/admin", func(admin chi.Router) {
		admin.Use(requireSession(pool))
		admin.Get("/events", notImplementedYet)
	})

	return r
}
