package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/config"
)

func NewRouter(pool *pgxpool.Pool, cfg config.Config) http.Handler {
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

	return r
}
