package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// OutboxKind mirrors the outbox_kind enum.
type OutboxKind string

const (
	OutboxPublished OutboxKind = "published"
	OutboxUpdated   OutboxKind = "updated"
	OutboxCancelled OutboxKind = "cancelled"
)

// OutboxPayload is the event snapshot taken at write time. The difusión
// worker renders messages from this, never from the live row, so a later edit
// cannot rewrite the content of a message already queued.
type OutboxPayload struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Place       string    `json:"place"`
	StartsAt    time.Time `json:"starts_at"`
	EndsAt      time.Time `json:"ends_at"`
	GroupID     uuid.UUID `json:"group_id"`
	Rank        string    `json:"rank"`
}

// insertOutbox appends an outbox row inside the caller's transaction. It is
// never called outside one: the row must land atomically with the mutation it
// describes, or the difusión engine would announce something that did not
// happen (or stay silent about something that did).
func insertOutbox(ctx context.Context, tx pgx.Tx, eventID uuid.UUID, kind OutboxKind, payload OutboxPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO outbox (event_id, kind, payload) VALUES ($1,$2,$3)`,
		eventID, string(kind), body)
	return err
}
