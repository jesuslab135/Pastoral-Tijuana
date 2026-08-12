package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Querier is the sliver of pgx that single-row lookups need, so they can run
// either on the pool or inside a caller's transaction.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

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

// OutboxRow is an unprocessed announcement waiting for the relay.
type OutboxRow struct {
	ID        int64
	EventID   uuid.UUID
	Kind      OutboxKind
	Payload   OutboxPayload
	CreatedAt time.Time
}

func scanOutboxRow(row scanner) (OutboxRow, error) {
	var r OutboxRow
	var kind string
	var body []byte
	if err := row.Scan(&r.ID, &r.EventID, &kind, &body, &r.CreatedAt); err != nil {
		return r, err
	}
	r.Kind = OutboxKind(kind)
	return r, json.Unmarshal(body, &r.Payload)
}

const outboxCols = `id, event_id, kind::text, payload, created_at`

func GetOutboxRow(ctx context.Context, q Querier, id int64) (OutboxRow, error) {
	return scanOutboxRow(q.QueryRow(ctx,
		`SELECT `+outboxCols+` FROM outbox WHERE id = $1`, id))
}

// ClaimOutboxBatch hands the oldest unprocessed rows to fn and marks them
// processed in the same transaction. SKIP LOCKED lets a second worker take
// different rows instead of blocking, and the shared transaction means a
// failure to enqueue leaves the rows for the next tick rather than losing the
// announcement. The reverse risk — a commit that fails after fn already
// enqueued — is absorbed downstream by the broadcast dedupe key.
func ClaimOutboxBatch(ctx context.Context, pool *pgxpool.Pool, limit int, fn func(ctx context.Context, row OutboxRow) error) (int, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	// Rollback after a successful Commit is a no-op, so this is safe to ignore.
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx,
		`SELECT `+outboxCols+`
		 FROM outbox
		 WHERE processed_at IS NULL
		 ORDER BY id
		 LIMIT $1
		 FOR UPDATE SKIP LOCKED`, limit)
	if err != nil {
		return 0, err
	}
	// Collected before fn runs: the transaction cannot issue another query
	// while these rows are still streaming.
	var claimed []OutboxRow
	for rows.Next() {
		r, err := scanOutboxRow(rows)
		if err != nil {
			rows.Close()
			return 0, err
		}
		claimed = append(claimed, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(claimed) == 0 {
		return 0, tx.Commit(ctx)
	}

	ids := make([]int64, 0, len(claimed))
	for _, r := range claimed {
		if err := fn(ctx, r); err != nil {
			return 0, err
		}
		ids = append(ids, r.ID)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE outbox SET processed_at = now() WHERE id = ANY($1)`, ids); err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return len(claimed), nil
}

// HasNewerUpdated reports whether a fresher edit of the same event is already
// queued. The relay collapses onto the newest one, so a burst of corrections
// reaches the parish as a single message.
func HasNewerUpdated(ctx context.Context, q Querier, eventID uuid.UUID, afterID int64) (bool, error) {
	var exists bool
	err := q.QueryRow(ctx,
		`SELECT EXISTS (
		   SELECT 1 FROM outbox
		   WHERE event_id = $1 AND kind = 'updated' AND id > $2)`,
		eventID, afterID).Scan(&exists)
	return exists, err
}
