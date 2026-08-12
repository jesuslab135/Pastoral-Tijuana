package difusion

import (
	"context"
	"log"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
)

const (
	// relayBatch bounds one transaction; the loop comes back in relayTick.
	relayBatch = 20
	relayTick  = 2 * time.Second

	// debounceWindow holds an edit before announcing it, so a secretary
	// fixing the time and then the place produces one message. Rows written
	// during the window collapse onto the newest one.
	debounceWindow = 10 * time.Minute
)

// RelayOnce moves unprocessed outbox rows onto the fanout queue. The enqueue
// and the "processed" mark share one transaction, so a broker outage leaves
// the rows for the next tick instead of losing the announcement.
func RelayOnce(ctx context.Context, pool *pgxpool.Pool, enq Enqueuer) (int, error) {
	return store.ClaimOutboxBatch(ctx, pool, relayBatch, func(ctx context.Context, row store.OutboxRow) error {
		if row.Kind == store.OutboxUpdated {
			newer, err := store.HasNewerUpdated(ctx, pool, row.EventID, row.ID)
			if err != nil {
				return err
			}
			if newer {
				// Superseded: consumed, but never announced.
				return nil
			}
		}
		opts := []asynq.Option{asynq.Queue(QueueFanout)}
		if row.Kind == store.OutboxUpdated {
			opts = append(opts, asynq.ProcessIn(debounceWindow))
		}
		task, err := NewFanoutTask(row.ID)
		if err != nil {
			return err
		}
		_, err = enq.Enqueue(task, opts...)
		return err
	})
}

// RunRelay polls until the context is cancelled. Polling rather than
// LISTEN/NOTIFY keeps it crash-safe: whatever was missed is simply still
// unprocessed on the next tick.
func RunRelay(ctx context.Context, pool *pgxpool.Pool, enq Enqueuer) {
	ticker := time.NewTicker(relayTick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if n, err := RelayOnce(ctx, pool, enq); err != nil {
				log.Printf("difusion: relay: %v", err)
			} else if n > 0 {
				log.Printf("difusion: %d aviso(s) encolado(s)", n)
			}
		}
	}
}
