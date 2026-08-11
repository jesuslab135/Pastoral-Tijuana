package httpapi

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/ics"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
)

const calName = "Calendario Pastoral · Cristo de Los Álamos"

func icsHandler(pool *pgxpool.Pool, publicBaseURL string) http.HandlerFunc {
	host := "app.jesuslab135.com"
	if u, err := url.Parse(publicBaseURL); err == nil && u.Host != "" {
		host = u.Hostname()
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var groupSlug *string
		if raw := chi.URLParam(r, "slug"); raw != "" {
			slug := strings.TrimSuffix(raw, ".ics")
			groups, err := store.ListPublicGroups(r.Context(), pool)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal",
					"No se pudo generar el calendario.")
				return
			}
			found := false
			for _, g := range groups {
				if g.Slug == slug {
					found = true
					break
				}
			}
			if !found {
				writeError(w, http.StatusNotFound, "not_found",
					"No existe ese grupo parroquial.")
				return
			}
			groupSlug = &slug
		}

		evs, err := store.ListEventsForICS(r.Context(), pool, groupSlug, time.Now())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal",
				"No se pudo generar el calendario.")
			return
		}

		var maxUpdated int64
		for _, e := range evs {
			if u := e.UpdatedAt.Unix(); u > maxUpdated {
				maxUpdated = u
			}
		}
		etag := fmt.Sprintf(`W/"%d-%d"`, maxUpdated, len(evs))
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		icsEvents := make([]ics.Event, 0, len(evs))
		for _, e := range evs {
			icsEvents = append(icsEvents, ics.Event{
				ID: e.ID, Title: e.Title, Place: e.Place, Description: e.Description,
				StartsAt: e.StartsAt, EndsAt: e.EndsAt,
				UpdatedAt: e.UpdatedAt, Cancelled: e.Cancelled,
			})
		}

		w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=300")
		w.Header().Set("ETag", etag)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(ics.Build(calName, host, icsEvents)))
	}
}
