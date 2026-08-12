package difusion

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/config"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
)

// Fanout resolves who should hear about one outbox row, records a broadcast
// per channel and queues a delivery for each one that is new. It is safe to
// replay: the dedupe key absorbs a retry after a partial run.
func Fanout(ctx context.Context, pool *pgxpool.Pool, enq Enqueuer, cfg config.Config, outboxID int64) error {
	row, err := store.GetOutboxRow(ctx, pool, outboxID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// A queue entry can outlive its row after a cleanup or a test
			// truncation. Retrying forever would never find it.
			log.Printf("difusion: outbox %d ya no existe, se omite", outboxID)
			return nil
		}
		return err
	}

	if row.Kind == store.OutboxUpdated {
		newer, err := store.HasNewerUpdated(ctx, pool, row.EventID, row.ID)
		if err != nil {
			return err
		}
		if newer {
			// A fresher correction is already queued; sending both would
			// contradict the parish's own message.
			return nil
		}
	}

	channels, err := recipients(ctx, pool, row)
	if err != nil {
		return err
	}

	loc, err := time.LoadLocation(cfg.ParishTZ)
	if err != nil {
		return err
	}
	base := time.Duration(cfg.StaggerSeconds) * time.Second
	now := time.Now()

	for n, ch := range channels {
		b := store.Broadcast{
			ID: uuid.New(), EventID: row.EventID, ChannelID: ch.ID,
			Kind: row.Kind, State: "queued",
			DedupeKey: store.DedupeKey(row.EventID, ch.ID, row.Kind, row.ID),
		}
		inserted, err := store.CreateBroadcast(ctx, pool, b)
		if err != nil {
			return err
		}
		if !inserted {
			// An earlier attempt already recorded and queued this one.
			continue
		}

		task, err := NewDeliverTask(DeliverPayload{BroadcastID: b.ID, OutboxID: row.ID})
		if err != nil {
			return err
		}
		at := NextAllowed(now.Add(Stagger(n, base)), loc, cfg.QuietStart, cfg.QuietEnd)
		if _, err := enq.Enqueue(task,
			asynq.Queue(QueueFor(ch.Kind)),
			asynq.MaxRetry(DeliverMaxRetry),
			asynq.ProcessAt(at),
		); err != nil {
			// The broadcast row is already there, so the retried fanout will
			// skip straight past it — but this delivery still needs queuing.
			return err
		}
	}
	return nil
}

// recipients answers who hears about this row. New and corrected events reach
// the group plus the whole parish; a cancellation reaches only the channels
// that were actually told, so nobody learns of an event through its
// retraction.
func recipients(ctx context.Context, pool *pgxpool.Pool, row store.OutboxRow) ([]store.Channel, error) {
	if row.Kind != store.OutboxCancelled {
		return store.ActiveChannelsForGroup(ctx, pool, row.Payload.GroupID)
	}

	ids, err := store.BroadcastRecipients(ctx, pool, row.EventID)
	if err != nil {
		return nil, err
	}
	var out []store.Channel
	for _, id := range ids {
		ch, err := store.GetChannel(ctx, pool, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			return nil, err
		}
		out = append(out, ch)
	}
	return out, nil
}
