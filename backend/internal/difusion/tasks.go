package difusion

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

const (
	TypeFanout  = "difusion:fanout"
	TypeDeliver = "difusion:deliver"

	// QueueWA is served with concurrency 1: WhatsApp providers throttle hard,
	// and a parish group flooded in one second reads as spam. Mail and fanout
	// have no such constraint.
	QueueWA     = "wa"
	QueueMail   = "mail"
	QueueFanout = "fanout"

	// DeliverMaxRetry is asynq's budget per delivery; the broadcast row turns
	// dead when it runs out, and only a manual retry moves it after that.
	DeliverMaxRetry = 5
)

// FanoutPayload names the outbox row to resolve. It carries no snapshot: the
// row is the single source of what was announced.
type FanoutPayload struct {
	OutboxID int64 `json:"outbox_id"`
}

// DeliverPayload names one recorded broadcast and the outbox row to render.
type DeliverPayload struct {
	BroadcastID uuid.UUID `json:"broadcast_id"`
	OutboxID    int64     `json:"outbox_id"`
}

func NewFanoutTask(outboxID int64, opts ...asynq.Option) (*asynq.Task, error) {
	body, err := json.Marshal(FanoutPayload{OutboxID: outboxID})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeFanout, body, opts...), nil
}

func NewDeliverTask(p DeliverPayload, opts ...asynq.Option) (*asynq.Task, error) {
	body, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeDeliver, body, opts...), nil
}

// Enqueuer is the slice of *asynq.Client the engine uses, so fanout and relay
// can be tested without a broker.
type Enqueuer interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// QueueFor routes a delivery by channel kind.
func QueueFor(channelKind string) string {
	if channelKind == "whatsapp" {
		return QueueWA
	}
	return QueueMail
}
