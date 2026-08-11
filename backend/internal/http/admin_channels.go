package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
)

var validChannelKinds = map[string]bool{"whatsapp": true, "email": true}

type channelJSON struct {
	ID       uuid.UUID  `json:"id"`
	Kind     string     `json:"kind"`
	Name     string     `json:"name"`
	Target   string     `json:"target"`
	GroupID  *uuid.UUID `json:"group_id"`
	IsActive bool       `json:"is_active"`
}

type channelInput struct {
	Kind     string     `json:"kind"`
	Name     string     `json:"name"`
	Target   string     `json:"target"`
	GroupID  *uuid.UUID `json:"group_id"`
	IsActive *bool      `json:"is_active"`
}

func (in channelInput) validate() string {
	switch {
	case !validChannelKinds[in.Kind]:
		return "El tipo debe ser whatsapp o email."
	case in.Name == "":
		return "El nombre es obligatorio."
	case in.Target == "":
		return "El destino es obligatorio."
	}
	return ""
}

func toChannelJSON(c store.Channel) channelJSON {
	return channelJSON{
		ID: c.ID, Kind: c.Kind, Name: c.Name,
		Target: c.Target, GroupID: c.GroupID, IsActive: c.IsActive,
	}
}

func decodeChannel(w http.ResponseWriter, r *http.Request) (channelInput, bool) {
	var in channelInput
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

func writeChannelError(w http.ResponseWriter, err error, fallback string) {
	var pgErr *pgconn.PgError
	switch {
	case errors.Is(err, store.ErrChannelNotFound), errors.Is(err, pgx.ErrNoRows):
		writeError(w, http.StatusNotFound, "no_encontrado", "No existe ese canal.")
	case errors.As(err, &pgErr) && pgErr.Code == fkViolation:
		// Either the group does not exist, or broadcasts still reference the
		// channel; deactivating is the safe alternative to deleting.
		writeError(w, http.StatusConflict, "conflicto",
			"No se puede: el canal tiene difusiones registradas o el grupo no existe. Desactívalo en su lugar.")
	default:
		writeError(w, http.StatusInternalServerError, "internal", fallback)
	}
}

func listChannelsHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cs, err := store.ListChannels(r.Context(), pool)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "No se pudieron cargar los canales.")
			return
		}
		out := make([]channelJSON, 0, len(cs))
		for _, c := range cs {
			out = append(out, toChannelJSON(c))
		}
		writeJSON(w, http.StatusOK, map[string]any{"channels": out})
	}
}

func createChannelHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		in, ok := decodeChannel(w, r)
		if !ok {
			return
		}
		active := true
		if in.IsActive != nil {
			active = *in.IsActive
		}
		c := store.Channel{
			ID: uuid.New(), Kind: in.Kind, Name: in.Name,
			Target: in.Target, GroupID: in.GroupID, IsActive: active,
		}
		if err := store.CreateChannel(r.Context(), pool, c); err != nil {
			writeChannelError(w, err, "No se pudo crear el canal.")
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"channel": toChannelJSON(c)})
	}
}

func updateChannelHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			writeError(w, http.StatusNotFound, "no_encontrado", "No existe ese canal.")
			return
		}
		in, ok := decodeChannel(w, r)
		if !ok {
			return
		}
		active := true
		if in.IsActive != nil {
			active = *in.IsActive
		}
		c := store.Channel{
			ID: id, Kind: in.Kind, Name: in.Name,
			Target: in.Target, GroupID: in.GroupID, IsActive: active,
		}
		if err := store.UpdateChannel(r.Context(), pool, c); err != nil {
			writeChannelError(w, err, "No se pudo guardar el canal.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"channel": toChannelJSON(c)})
	}
}

func deleteChannelHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			writeError(w, http.StatusNotFound, "no_encontrado", "No existe ese canal.")
			return
		}
		if err := store.DeleteChannel(r.Context(), pool, id); err != nil {
			writeChannelError(w, err, "No se pudo eliminar el canal.")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
