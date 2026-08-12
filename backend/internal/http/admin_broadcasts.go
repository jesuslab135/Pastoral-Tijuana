package httpapi

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/difusion"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
)

var validBroadcastStates = map[string]bool{
	"queued": true, "sent": true, "failed": true, "dead": true,
}

type broadcastJSON struct {
	ID          uuid.UUID  `json:"id"`
	EventID     uuid.UUID  `json:"event_id"`
	ChannelID   uuid.UUID  `json:"channel_id"`
	ChannelName string     `json:"channel_name"`
	ChannelKind string     `json:"channel_kind"`
	EventTitle  string     `json:"event_title"`
	Kind        string     `json:"kind"`
	State       string     `json:"state"`
	Attempt     int        `json:"attempt"`
	LastError   *string    `json:"last_error"`
	SentAt      *time.Time `json:"sent_at"`
	CreatedAt   time.Time  `json:"created_at"`
	// Simulated marks what the stub WhatsApp sender only pretended to send,
	// so nobody reads the log as proof the parish was told.
	Simulated bool `json:"simulated"`
}

func toBroadcastJSON(r store.BroadcastRow) broadcastJSON {
	return broadcastJSON{
		ID: r.ID, EventID: r.EventID, ChannelID: r.ChannelID,
		ChannelName: r.ChannelName, ChannelKind: r.ChannelKind, EventTitle: r.EventTitle,
		Kind: string(r.Kind), State: r.State, Attempt: r.Attempt,
		LastError: r.LastError, SentAt: r.SentAt, CreatedAt: r.CreatedAt,
		Simulated: r.ChannelKind == "whatsapp",
	}
}

func listBroadcastsHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var state *string
		if s := r.URL.Query().Get("state"); s != "" {
			if !validBroadcastStates[s] {
				writeError(w, http.StatusBadRequest, "bad_request",
					"El estado debe ser queued, sent, failed o dead.")
				return
			}
			state = &s
		}
		var eventID *uuid.UUID
		if v := r.URL.Query().Get("event_id"); v != "" {
			id, err := uuid.Parse(v)
			if err != nil {
				writeError(w, http.StatusBadRequest, "bad_request", "El event_id no es válido.")
				return
			}
			eventID = &id
		}

		rows, err := store.ListBroadcasts(r.Context(), pool, state, eventID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal",
				"No se pudo cargar el registro de difusión.")
			return
		}
		out := make([]broadcastJSON, 0, len(rows))
		for _, row := range rows {
			out = append(out, toBroadcastJSON(row))
		}
		writeJSON(w, http.StatusOK, map[string]any{"broadcasts": out})
	}
}

// retryBroadcastHandler requeues a failed delivery and puts it back on the
// queue its channel belongs to. Both roles may use it: fixing a bounced
// announcement is ordinary secretarial work.
func retryBroadcastHandler(pool *pgxpool.Pool, enq difusion.Enqueuer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			writeError(w, http.StatusNotFound, "no_encontrado", "No existe ese envío.")
			return
		}
		b, err := store.GetBroadcast(r.Context(), pool, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeError(w, http.StatusNotFound, "no_encontrado", "No existe ese envío.")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "No se pudo cargar el envío.")
			return
		}
		ch, err := store.GetChannel(r.Context(), pool, b.ChannelID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "No se pudo cargar el canal.")
			return
		}
		outboxID, err := store.OutboxIDFromDedupeKey(b.DedupeKey)
		if err != nil {
			log.Printf("difusion: dedupe_key ilegible en broadcast %s: %v", b.ID, err)
			writeError(w, http.StatusInternalServerError, "internal", "No se pudo reintentar el envío.")
			return
		}

		if err := store.ResetBroadcastForRetry(r.Context(), pool, id); err != nil {
			if errors.Is(err, store.ErrNotRetryable) {
				writeError(w, http.StatusConflict, "no_reintentable",
					"Solo se pueden reintentar los envíos fallidos.")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", "No se pudo reintentar el envío.")
			return
		}

		task, err := difusion.NewDeliverTask(difusion.DeliverPayload{BroadcastID: b.ID, OutboxID: outboxID})
		if err == nil {
			_, err = enq.Enqueue(task, asynq.Queue(difusion.QueueFor(ch.Kind)), asynq.MaxRetry(difusion.DeliverMaxRetry))
		}
		if err != nil {
			// The row is queued but nothing will pick it up; say so rather
			// than reporting a retry that never happens.
			log.Printf("difusion: no se pudo encolar el reintento de %s: %v", b.ID, err)
			writeError(w, http.StatusInternalServerError, "internal", "No se pudo encolar el reintento.")
			return
		}

		// Re-read via ListBroadcasts rather than hand-building the row: it is
		// the one place that already joins channel and event, so the retry
		// response cannot drift from what the list endpoint shows.
		rows, err := store.ListBroadcasts(r.Context(), pool, nil, &b.EventID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "No se pudo cargar el envío.")
			return
		}
		var updated *store.BroadcastRow
		for i := range rows {
			if rows[i].ID == id {
				updated = &rows[i]
				break
			}
		}
		if updated == nil {
			writeError(w, http.StatusInternalServerError, "internal", "No se pudo cargar el envío.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"broadcast": toBroadcastJSON(*updated),
		})
	}
}
