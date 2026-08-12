package difusion

import (
	"context"
	"encoding/json"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/config"
)

// NewMux registers the engine's two handlers. The worker binary and the
// end-to-end test share it, so what the test proves is what production runs.
func NewMux(pool *pgxpool.Pool, enq Enqueuer, senders map[string]Sender,
	loc *time.Location, cfg config.Config) *asynq.ServeMux {
	mux := asynq.NewServeMux()

	mux.HandleFunc(TypeFanout, func(ctx context.Context, t *asynq.Task) error {
		var p FanoutPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			// An undecodable payload never becomes decodable; retrying it
			// would only burn the budget.
			return asynq.SkipRetry
		}
		return Fanout(ctx, pool, enq, cfg, p.OutboxID)
	})

	mux.HandleFunc(TypeDeliver, func(ctx context.Context, t *asynq.Task) error {
		var p DeliverPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return asynq.SkipRetry
		}
		// asynq owns the retry budget; Deliver only needs to know whether
		// this attempt is the last one, so the row can be marked dead.
		retried, _ := asynq.GetRetryCount(ctx)
		maxRetry, _ := asynq.GetMaxRetry(ctx)
		return Deliver(ctx, pool, senders, loc, cfg.PublicBaseURL, p, retried, maxRetry)
	})

	return mux
}
