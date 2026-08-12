package store

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotRetryable guards the panel's retry button: only a broadcast that
// failed can be sent again. Retrying a delivered one would announce the same
// event to the parish twice.
var ErrNotRetryable = errors.New("broadcast not retryable")

// Broadcast is one attempt to tell one channel about one event.
type Broadcast struct {
	ID        uuid.UUID
	EventID   uuid.UUID
	ChannelID uuid.UUID
	Kind      OutboxKind
	State     string // queued|sent|failed|dead
	Attempt   int
	DedupeKey string
	LastError *string
	SentAt    *time.Time
	CreatedAt time.Time
}

// BroadcastRow is a Broadcast with its channel resolved, which is what the
// panel's "a dónde se fue" screen shows.
type BroadcastRow struct {
	Broadcast
	ChannelName string
	ChannelKind string
}

// DedupeKey identifies one announcement to one channel. The outbox id is part
// of it so that a later edit of the same event is a new announcement, while a
// retried fanout of the same outbox row is not.
func DedupeKey(eventID, channelID uuid.UUID, kind OutboxKind, outboxID int64) string {
	return fmt.Sprintf("%s:%s:%s:%d", eventID, channelID, kind, outboxID)
}

// OutboxIDFromDedupeKey recovers the outbox row a broadcast came from, which
// is what a manual retry needs in order to re-render the same snapshot.
func OutboxIDFromDedupeKey(key string) (int64, error) {
	i := strings.LastIndex(key, ":")
	if i < 0 {
		return 0, fmt.Errorf("dedupe key %q has no outbox id", key)
	}
	return strconv.ParseInt(key[i+1:], 10, 64)
}

const broadcastCols = `id, event_id, channel_id, kind::text, state::text,
	attempt, dedupe_key, last_error, sent_at, created_at`

func scanBroadcast(row scanner) (Broadcast, error) {
	var b Broadcast
	var kind string
	err := row.Scan(&b.ID, &b.EventID, &b.ChannelID, &kind, &b.State,
		&b.Attempt, &b.DedupeKey, &b.LastError, &b.SentAt, &b.CreatedAt)
	b.Kind = OutboxKind(kind)
	return b, err
}

// CreateBroadcast records the intent to deliver. The conflict clause is what
// makes fanout safe to replay: asynq retries the whole task, and a broadcast
// already recorded must not become a second message.
func CreateBroadcast(ctx context.Context, pool *pgxpool.Pool, b Broadcast) (bool, error) {
	tag, err := pool.Exec(ctx,
		`INSERT INTO broadcasts (id, event_id, channel_id, kind, state, dedupe_key)
		 VALUES ($1,$2,$3,$4,$5,$6)
		 ON CONFLICT (dedupe_key) DO NOTHING`,
		b.ID, b.EventID, b.ChannelID, string(b.Kind), b.State, b.DedupeKey)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func GetBroadcast(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) (Broadcast, error) {
	return scanBroadcast(pool.QueryRow(ctx,
		`SELECT `+broadcastCols+` FROM broadcasts WHERE id = $1`, id))
}

// ListBroadcasts returns the difusión log, newest first, optionally narrowed
// to one state or one event.
func ListBroadcasts(ctx context.Context, pool *pgxpool.Pool, state *string, eventID *uuid.UUID) ([]BroadcastRow, error) {
	rows, err := pool.Query(ctx,
		`SELECT b.id, b.event_id, b.channel_id, b.kind::text, b.state::text,
		        b.attempt, b.dedupe_key, b.last_error, b.sent_at, b.created_at,
		        c.name, c.kind::text
		 FROM broadcasts b
		 JOIN channels c ON c.id = b.channel_id
		 WHERE ($1::text IS NULL OR b.state::text = $1)
		   AND ($2::uuid IS NULL OR b.event_id = $2)
		 ORDER BY b.created_at DESC, b.id DESC`, state, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BroadcastRow
	for rows.Next() {
		var r BroadcastRow
		var kind string
		if err := rows.Scan(&r.ID, &r.EventID, &r.ChannelID, &kind, &r.State,
			&r.Attempt, &r.DedupeKey, &r.LastError, &r.SentAt, &r.CreatedAt,
			&r.ChannelName, &r.ChannelKind); err != nil {
			return nil, err
		}
		r.Kind = OutboxKind(kind)
		out = append(out, r)
	}
	return out, rows.Err()
}

// MarkBroadcastSent settles a delivery. The previous error is cleared so the
// panel does not show a red message next to something that went through.
func MarkBroadcastSent(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) error {
	_, err := pool.Exec(ctx,
		`UPDATE broadcasts SET state = 'sent', sent_at = now(), last_error = NULL
		 WHERE id = $1`, id)
	return err
}

// MarkBroadcastFailed counts the attempt and records why. dead means asynq has
// no retries left, so nothing will move this row again without a manual retry.
func MarkBroadcastFailed(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID, msg string, dead bool) error {
	state := "failed"
	if dead {
		state = "dead"
	}
	_, err := pool.Exec(ctx,
		`UPDATE broadcasts SET state = $2::broadcast_state, attempt = attempt + 1,
		        last_error = $3
		 WHERE id = $1`, id, state, msg)
	return err
}

// ResetBroadcastForRetry requeues a failed delivery. The state precondition
// lives in the WHERE clause, so a concurrent double-click cannot requeue a row
// that is already on its way out.
func ResetBroadcastForRetry(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) error {
	tag, err := pool.Exec(ctx,
		`UPDATE broadcasts SET state = 'queued'
		 WHERE id = $1 AND state IN ('failed','dead')`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotRetryable
	}
	return nil
}

// ActiveChannelsForGroup returns who hears about an event of this group: the
// group's own channels plus the ones addressed to the whole parish.
func ActiveChannelsForGroup(ctx context.Context, pool *pgxpool.Pool, groupID uuid.UUID) ([]Channel, error) {
	rows, err := pool.Query(ctx,
		`SELECT `+channelCols+`
		 FROM channels
		 WHERE is_active AND (group_id = $1 OR group_id IS NULL)
		 ORDER BY name`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Channel
	for rows.Next() {
		c, err := scanChannel(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// BroadcastRecipients lists the channels that actually received news of this
// event. A cancellation goes only to them: retracting an announcement a
// channel never got would be the first it hears of the event.
func BroadcastRecipients(ctx context.Context, pool *pgxpool.Pool, eventID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := pool.Query(ctx,
		`SELECT DISTINCT channel_id
		 FROM broadcasts
		 WHERE event_id = $1 AND state = 'sent' AND kind IN ('published','updated')`,
		eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
