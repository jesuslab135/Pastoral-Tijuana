package httpapi

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
)

const (
	maxRangeDays = 400
	minYear      = 1
	maxYear      = 9999
)

func cachePublic(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "public, max-age=300")
}

type groupJSON struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	Slug string    `json:"slug"`
}

type eventJSON struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Place       string    `json:"place"`
	StartsAt    time.Time `json:"starts_at"`
	EndsAt      time.Time `json:"ends_at"`
	Group       groupJSON `json:"group"`
	Rank        string    `json:"rank"`
	Color       string    `json:"color"`
}

func eventsHandler(pool *pgxpool.Pool, tz string) http.HandlerFunc {
	// Resolved once: time.LoadLocation reads the tzdata files on every call
	// and has no cache. main embeds time/tzdata and fails fast on a bad zone,
	// so the error branch below only guards a router built outside main.
	loc, locErr := time.LoadLocation(tz)
	return func(w http.ResponseWriter, r *http.Request) {
		if locErr != nil {
			writeError(w, http.StatusInternalServerError, "internal",
				"Zona horaria mal configurada.")
			return
		}
		from, err1 := time.ParseInLocation("2006-01-02", r.URL.Query().Get("from"), loc)
		to, err2 := time.ParseInLocation("2006-01-02", r.URL.Query().Get("to"), loc)
		if err1 != nil || err2 != nil || !to.After(from) {
			writeError(w, http.StatusBadRequest, "bad_request",
				"Parámetros from y to requeridos en formato AAAA-MM-DD, con to posterior a from.")
			return
		}
		if to.Sub(from) > maxRangeDays*24*time.Hour {
			writeError(w, http.StatusBadRequest, "bad_request",
				"El rango máximo es de 400 días.")
			return
		}
		evs, err := store.ListPublishedEvents(r.Context(), pool, from, to, tz)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal",
				"No se pudieron cargar los eventos.")
			return
		}
		out := make([]eventJSON, 0, len(evs))
		for _, e := range evs {
			out = append(out, eventJSON{
				ID: e.ID, Title: e.Title, Description: e.Description, Place: e.Place,
				StartsAt: e.StartsAt, EndsAt: e.EndsAt,
				Group: groupJSON{ID: e.GroupID, Name: e.GroupName, Slug: e.GroupSlug},
				Rank:  e.Rank, Color: e.Color,
			})
		}
		cachePublic(w)
		writeJSON(w, http.StatusOK, map[string]any{"events": out})
	}
}

type seasonJSON struct {
	Name  string `json:"name"`
	Color string `json:"color"`
	Start string `json:"start"`
	End   string `json:"end"`
}

func seasonsHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		year := time.Now().Year()
		if q := r.URL.Query().Get("year"); q != "" {
			t, err := time.Parse("2006", q)
			// Go parses "0000", but Postgres has no year zero and make_date
			// would turn that into a 500 instead of a 400.
			if err != nil || t.Year() < minYear || t.Year() > maxYear {
				writeError(w, http.StatusBadRequest, "bad_request",
					"El parámetro year debe ser un año de cuatro dígitos entre 0001 y 9999.")
				return
			}
			year = t.Year()
		}
		seasons, err := store.ListSeasonsForYear(r.Context(), pool, year)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal",
				"No se pudieron cargar los tiempos litúrgicos.")
			return
		}
		out := make([]seasonJSON, 0, len(seasons))
		for _, s := range seasons {
			out = append(out, seasonJSON{
				Name: s.Name, Color: s.Color,
				Start: s.Start.Format("2006-01-02"),
				End:   s.End.Format("2006-01-02"),
			})
		}
		cachePublic(w)
		writeJSON(w, http.StatusOK, map[string]any{"seasons": out})
	}
}

func groupsHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groups, err := store.ListPublicGroups(r.Context(), pool)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal",
				"No se pudieron cargar los grupos.")
			return
		}
		out := make([]groupJSON, 0, len(groups))
		for _, g := range groups {
			out = append(out, groupJSON{ID: g.ID, Name: g.Name, Slug: g.Slug})
		}
		cachePublic(w)
		writeJSON(w, http.StatusOK, map[string]any{"groups": out})
	}
}
