package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrAlreadyPublished guards double publication, which would send the
	// parish a second announcement of the same event.
	ErrAlreadyPublished = errors.New("event already published")
	ErrNotPublished     = errors.New("event not published")
)

const eventCols = `id, title, coalesce(description,''), coalesce(place,''),
	starts_at, ends_at, group_id, rank::text, color_override::text,
	published_at, cancelled_at`

func scanEvent(row scanner) (Event, error) {
	var e Event
	var color *string
	err := row.Scan(&e.ID, &e.Title, &e.Description, &e.Place,
		&e.StartsAt, &e.EndsAt, &e.GroupID, &e.Rank, &color,
		&e.PublishedAt, &e.CancelledAt)
	e.ColorOverride = color
	return e, err
}

// GetEventAdmin returns any event, including drafts and cancelled ones.
func GetEventAdmin(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) (Event, error) {
	return scanEvent(pool.QueryRow(ctx, `SELECT `+eventCols+` FROM events WHERE id = $1`, id))
}

// ListEventsAdmin returns every event in the window, drafts included.
func ListEventsAdmin(ctx context.Context, pool *pgxpool.Pool, from, to time.Time) ([]Event, error) {
	rows, err := pool.Query(ctx,
		`SELECT `+eventCols+`
		 FROM events
		 WHERE starts_at >= $1 AND starts_at < $2
		 ORDER BY starts_at`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func payloadOf(e Event) OutboxPayload {
	return OutboxPayload{
		ID: e.ID, Title: e.Title, Description: e.Description, Place: e.Place,
		StartsAt: e.StartsAt, EndsAt: e.EndsAt, GroupID: e.GroupID, Rank: e.Rank,
	}
}

// broadcastWorthy reports whether the difference between two versions of an
// event is worth telling the parish about. Only time and place qualify: a
// title typo or a reworded description must not ring anyone's phone.
func broadcastWorthy(before, after Event) bool {
	return !before.StartsAt.Equal(after.StartsAt) ||
		!before.EndsAt.Equal(after.EndsAt) ||
		before.Place != after.Place
}

// UpdateEvent saves the event and, when it is published and the edit is
// broadcast-worthy, writes an `updated` outbox row in the same transaction.
func UpdateEvent(ctx context.Context, pool *pgxpool.Pool, e Event) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	// Rollback after a successful Commit is a no-op, so this is safe to ignore.
	defer func() { _ = tx.Rollback(ctx) }()

	before, err := scanEvent(tx.QueryRow(ctx,
		`SELECT `+eventCols+` FROM events WHERE id = $1 FOR UPDATE`, e.ID))
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE events SET title=$2, description=nullif($3,''), place=nullif($4,''),
		        starts_at=$5, ends_at=$6, group_id=$7, rank=$8, color_override=$9::season_color
		 WHERE id=$1`,
		e.ID, e.Title, e.Description, e.Place, e.StartsAt, e.EndsAt,
		e.GroupID, e.Rank, e.ColorOverride); err != nil {
		return err
	}
	if before.PublishedAt != nil && before.CancelledAt == nil && broadcastWorthy(before, e) {
		if err := insertOutbox(ctx, tx, e.ID, OutboxUpdated, payloadOf(e)); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// PublishEvent makes the event public and queues its announcement. The
// conditional UPDATE makes double publication impossible even under a race.
func PublishEvent(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	// Rollback after a successful Commit is a no-op, so this is safe to ignore.
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx,
		`UPDATE events SET published_at = now(), cancelled_at = NULL
		 WHERE id = $1 AND published_at IS NULL`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		// Either the event does not exist or it was already published.
		if _, err := scanEvent(tx.QueryRow(ctx,
			`SELECT `+eventCols+` FROM events WHERE id = $1`, id)); err != nil {
			return err
		}
		return ErrAlreadyPublished
	}
	e, err := scanEvent(tx.QueryRow(ctx, `SELECT `+eventCols+` FROM events WHERE id = $1`, id))
	if err != nil {
		return err
	}
	if err := insertOutbox(ctx, tx, id, OutboxPublished, payloadOf(e)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// UnpublishEvent hides the event again and queues a cancellation, so anyone
// already told about it learns it is off.
func UnpublishEvent(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	// Rollback after a successful Commit is a no-op, so this is safe to ignore.
	defer func() { _ = tx.Rollback(ctx) }()

	e, err := scanEvent(tx.QueryRow(ctx,
		`SELECT `+eventCols+` FROM events WHERE id = $1 FOR UPDATE`, id))
	if err != nil {
		return err
	}
	if e.PublishedAt == nil {
		return ErrNotPublished
	}
	if _, err := tx.Exec(ctx,
		`UPDATE events SET published_at = NULL WHERE id = $1`, id); err != nil {
		return err
	}
	if err := insertOutbox(ctx, tx, id, OutboxCancelled, payloadOf(e)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// DeleteEvent removes an event. A published event deleted with notify stays
// in the table as a soft cancellation: the .ics feed keeps serving it with
// STATUS:CANCELLED for 90 days so subscribed phones drop it, and the
// difusión engine announces the cancellation.
func DeleteEvent(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID, notify bool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	// Rollback after a successful Commit is a no-op, so this is safe to ignore.
	defer func() { _ = tx.Rollback(ctx) }()

	e, err := scanEvent(tx.QueryRow(ctx,
		`SELECT `+eventCols+` FROM events WHERE id = $1 FOR UPDATE`, id))
	if err != nil {
		return err
	}
	if e.PublishedAt != nil && notify {
		if _, err := tx.Exec(ctx,
			`UPDATE events SET cancelled_at = now() WHERE id = $1`, id); err != nil {
			return err
		}
		if err := insertOutbox(ctx, tx, id, OutboxCancelled, payloadOf(e)); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM events WHERE id = $1`, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
