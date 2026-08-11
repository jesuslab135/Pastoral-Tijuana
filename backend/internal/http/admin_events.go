package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
)

const fkViolation = "23503"

var validRanks = map[string]bool{
	"solemnidad": true, "fiesta": true, "memoria": true, "parroquial": true,
}

type adminEventJSON struct {
	ID            uuid.UUID  `json:"id"`
	Title         string     `json:"title"`
	Description   string     `json:"description"`
	Place         string     `json:"place"`
	StartsAt      time.Time  `json:"starts_at"`
	EndsAt        time.Time  `json:"ends_at"`
	GroupID       uuid.UUID  `json:"group_id"`
	Rank          string     `json:"rank"`
	ColorOverride *string    `json:"color_override"`
	PublishedAt   *time.Time `json:"published_at"`
	CancelledAt   *time.Time `json:"cancelled_at"`
}

func toAdminJSON(e store.Event) adminEventJSON {
	return adminEventJSON{
		ID: e.ID, Title: e.Title, Description: e.Description, Place: e.Place,
		StartsAt: e.StartsAt, EndsAt: e.EndsAt, GroupID: e.GroupID,
		Rank: e.Rank, ColorOverride: e.ColorOverride,
		PublishedAt: e.PublishedAt, CancelledAt: e.CancelledAt,
	}
}

type eventInput struct {
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	Place         string    `json:"place"`
	StartsAt      time.Time `json:"starts_at"`
	EndsAt        time.Time `json:"ends_at"`
	GroupID       uuid.UUID `json:"group_id"`
	Rank          string    `json:"rank"`
	ColorOverride *string   `json:"color_override"`
}

// validate returns a Spanish message describing the first problem, or "".
func (in eventInput) validate() string {
	switch {
	case in.Title == "":
		return "El título es obligatorio."
	case in.StartsAt.IsZero() || in.EndsAt.IsZero():
		return "Las fechas de inicio y fin son obligatorias (formato ISO 8601)."
	case !in.EndsAt.After(in.StartsAt):
		return "La hora de fin debe ser posterior a la de inicio."
	case in.GroupID == uuid.Nil:
		return "Selecciona un grupo parroquial."
	case !validRanks[in.Rank]:
		return "El rango debe ser solemnidad, fiesta, memoria o parroquial."
	}
	return ""
}

func decodeEvent(w http.ResponseWriter, r *http.Request) (eventInput, bool) {
	var in eventInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Cuerpo JSON inválido.")
		return in, false
	}
	if msg := in.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, "bad_request", msg)
		return in, false
	}
	return in, true
}

// eventIDParam extracts and validates the {id} path parameter.
func eventIDParam(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "no_encontrado", "No existe ese evento.")
		return uuid.Nil, false
	}
	return id, true
}

// writeStoreError maps store failures onto the API's error vocabulary.
func writeStoreError(w http.ResponseWriter, err error, fallback string) {
	var pgErr *pgconn.PgError
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		writeError(w, http.StatusNotFound, "no_encontrado", "No existe ese evento.")
	case errors.Is(err, store.ErrAlreadyPublished):
		writeError(w, http.StatusConflict, "ya_publicado", "El evento ya está publicado.")
	case errors.Is(err, store.ErrNotPublished):
		writeError(w, http.StatusConflict, "no_publicado", "El evento no está publicado.")
	case errors.As(err, &pgErr) && pgErr.Code == fkViolation:
		writeError(w, http.StatusBadRequest, "bad_request", "El grupo parroquial no existe.")
	default:
		writeError(w, http.StatusInternalServerError, "internal", fallback)
	}
}

func listAdminEventsHandler(pool *pgxpool.Pool, tz string) http.HandlerFunc {
	loc, locErr := time.LoadLocation(tz)
	return func(w http.ResponseWriter, r *http.Request) {
		if locErr != nil {
			writeError(w, http.StatusInternalServerError, "internal", "Zona horaria mal configurada.")
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
			writeError(w, http.StatusBadRequest, "bad_request", "El rango máximo es de 400 días.")
			return
		}
		evs, err := store.ListEventsAdmin(r.Context(), pool, from, to)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "No se pudieron cargar los eventos.")
			return
		}
		out := make([]adminEventJSON, 0, len(evs))
		for _, e := range evs {
			out = append(out, toAdminJSON(e))
		}
		writeJSON(w, http.StatusOK, map[string]any{"events": out})
	}
}

func createEventHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		in, ok := decodeEvent(w, r)
		if !ok {
			return
		}
		creator := currentUser(r).ID
		e := store.Event{
			ID: uuid.New(), Title: in.Title, Description: in.Description, Place: in.Place,
			StartsAt: in.StartsAt, EndsAt: in.EndsAt, GroupID: in.GroupID,
			Rank: in.Rank, ColorOverride: in.ColorOverride, CreatedBy: &creator,
		}
		if err := store.CreateEvent(r.Context(), pool, e); err != nil {
			writeStoreError(w, err, "No se pudo crear el evento.")
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"event": toAdminJSON(e)})
	}
}

func getEventHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := eventIDParam(w, r)
		if !ok {
			return
		}
		e, err := store.GetEventAdmin(r.Context(), pool, id)
		if err != nil {
			writeStoreError(w, err, "No se pudo cargar el evento.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"event": toAdminJSON(e)})
	}
}

func updateEventHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := eventIDParam(w, r)
		if !ok {
			return
		}
		in, ok := decodeEvent(w, r)
		if !ok {
			return
		}
		e := store.Event{
			ID: id, Title: in.Title, Description: in.Description, Place: in.Place,
			StartsAt: in.StartsAt, EndsAt: in.EndsAt, GroupID: in.GroupID,
			Rank: in.Rank, ColorOverride: in.ColorOverride,
		}
		if err := store.UpdateEvent(r.Context(), pool, e); err != nil {
			writeStoreError(w, err, "No se pudo guardar el evento.")
			return
		}
		saved, err := store.GetEventAdmin(r.Context(), pool, id)
		if err != nil {
			writeStoreError(w, err, "No se pudo cargar el evento.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"event": toAdminJSON(saved)})
	}
}

// eventStateHandler backs publish and unpublish, which differ only in the
// store call they make.
func eventStateHandler(pool *pgxpool.Pool, apply func(*http.Request, uuid.UUID) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := eventIDParam(w, r)
		if !ok {
			return
		}
		if err := apply(r, id); err != nil {
			writeStoreError(w, err, "No se pudo cambiar el estado del evento.")
			return
		}
		e, err := store.GetEventAdmin(r.Context(), pool, id)
		if err != nil {
			writeStoreError(w, err, "No se pudo cargar el evento.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"event": toAdminJSON(e)})
	}
}

func deleteEventHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := eventIDParam(w, r)
		if !ok {
			return
		}
		// Default to notifying: silently dropping an event the parish was
		// already told about is the worse mistake.
		notify := r.URL.Query().Get("notify") != "false"
		if err := store.DeleteEvent(r.Context(), pool, id, notify); err != nil {
			writeStoreError(w, err, "No se pudo eliminar el evento.")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
