package difusion

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/config"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store/testdb"
)

const (
	liturgiaID   = "a1000000-0000-4000-8000-000000000001"
	catequesisID = "a1000000-0000-4000-8000-000000000002"
)

// enqueued is one task an Enqueuer was handed, with the options it carried.
type enqueued struct {
	task *asynq.Task
	opts []asynq.Option
}

// queue reports which asynq queue the task was routed to.
func (e enqueued) queue() string {
	for _, o := range e.opts {
		if o.Type() == asynq.QueueOpt {
			if q, ok := o.Value().(string); ok {
				return q
			}
		}
	}
	return ""
}

func (e enqueued) option(kind asynq.OptionType) (any, bool) {
	for _, o := range e.opts {
		if o.Type() == kind {
			return o.Value(), true
		}
	}
	return nil, false
}

// fakeEnqueuer records instead of talking to Redis, so fanout's routing and
// scheduling decisions are observable without a broker.
type fakeEnqueuer struct {
	mu   sync.Mutex
	sent []enqueued
	err  error
}

func (f *fakeEnqueuer) Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	f.sent = append(f.sent, enqueued{task: task, opts: opts})
	return &asynq.TaskInfo{}, nil
}

func (f *fakeEnqueuer) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.sent)
}

func testConfig() config.Config {
	cfg := config.Load()
	cfg.ParishTZ = "America/Tijuana"
	cfg.StaggerSeconds = 0
	cfg.QuietStart, cfg.QuietEnd = 7, 7 // quiet hours off
	return cfg
}

func seedChannel(t *testing.T, pool *pgxpool.Pool, kind, name string, group *uuid.UUID, active bool) store.Channel {
	t.Helper()
	c := store.Channel{
		ID: uuid.New(), Kind: kind, Name: name,
		Target: "destino-" + name, GroupID: group, IsActive: active,
	}
	if err := store.CreateChannel(context.Background(), pool, c); err != nil {
		t.Fatalf("CreateChannel %s: %v", name, err)
	}
	return c
}

func seedPublishedEvent(t *testing.T, pool *pgxpool.Pool, group uuid.UUID) store.Event {
	t.Helper()
	e := store.Event{
		ID: uuid.New(), Title: "Hora santa", Place: "Templo",
		GroupID: group, Rank: "parroquial",
		StartsAt: time.Date(2026, 8, 4, 19, 0, 0, 0, time.UTC),
		EndsAt:   time.Date(2026, 8, 4, 20, 0, 0, 0, time.UTC),
	}
	ctx := context.Background()
	if err := store.CreateEvent(ctx, pool, e); err != nil {
		t.Fatalf("CreateEvent: %v", err)
	}
	if err := store.PublishEvent(ctx, pool, e.ID); err != nil {
		t.Fatalf("PublishEvent: %v", err)
	}
	return e
}

// latestOutboxID returns the newest outbox row for an event, which is what
// the relay would hand to fanout.
func latestOutboxID(t *testing.T, pool *pgxpool.Pool, eventID uuid.UUID) int64 {
	t.Helper()
	var id int64
	if err := pool.QueryRow(context.Background(),
		`SELECT id FROM outbox WHERE event_id = $1 ORDER BY id DESC LIMIT 1`,
		eventID).Scan(&id); err != nil {
		t.Fatalf("latest outbox row: %v", err)
	}
	return id
}

func TestFanoutTargetsTheGroupAndTheWholeParish(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	liturgia := uuid.MustParse(liturgiaID)
	catequesis := uuid.MustParse(catequesisID)

	wa := seedChannel(t, pool, "whatsapp", "liturgia", &liturgia, true)
	mail := seedChannel(t, pool, "email", "toda-la-parroquia", nil, true)
	seedChannel(t, pool, "whatsapp", "catequesis", &catequesis, true)
	seedChannel(t, pool, "email", "liturgia-apagado", &liturgia, false)

	e := seedPublishedEvent(t, pool, liturgia)
	outboxID := latestOutboxID(t, pool, e.ID)

	enq := &fakeEnqueuer{}
	if err := Fanout(ctx, pool, enq, testConfig(), outboxID); err != nil {
		t.Fatalf("Fanout: %v", err)
	}

	rows, err := store.ListBroadcasts(ctx, pool, nil, &e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 broadcasts (group + parish-wide), got %d: %+v", len(rows), rows)
	}
	if enq.count() != 2 {
		t.Fatalf("expected 2 deliver tasks, got %d", enq.count())
	}

	queues := map[string]int{}
	for _, s := range enq.sent {
		if s.task.Type() != TypeDeliver {
			t.Errorf("unexpected task type %q", s.task.Type())
		}
		queues[s.queue()]++
		if _, ok := s.option(asynq.MaxRetryOpt); !ok {
			t.Error("deliver tasks must carry a retry budget")
		}
	}
	// WhatsApp goes down a serialized queue; mail can run wide.
	if queues[QueueWA] != 1 || queues[QueueMail] != 1 {
		t.Errorf("wrong queue routing: %v (wa channel %s, mail channel %s)", queues, wa.Name, mail.Name)
	}
}

func TestFanoutIsIdempotent(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	liturgia := uuid.MustParse(liturgiaID)
	seedChannel(t, pool, "email", "boletin", nil, true)
	e := seedPublishedEvent(t, pool, liturgia)
	outboxID := latestOutboxID(t, pool, e.ID)

	enq := &fakeEnqueuer{}
	if err := Fanout(ctx, pool, enq, testConfig(), outboxID); err != nil {
		t.Fatal(err)
	}
	first := enq.count()

	// asynq retries the whole task; the parish must not hear it twice.
	if err := Fanout(ctx, pool, enq, testConfig(), outboxID); err != nil {
		t.Fatal(err)
	}
	rows, err := store.ListBroadcasts(ctx, pool, nil, &e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Errorf("a replayed fanout must not add broadcasts, got %d", len(rows))
	}
	if enq.count() != first {
		t.Errorf("a replayed fanout must not enqueue again, got %d then %d", first, enq.count())
	}
}

func TestFanoutSkipsSupersededUpdate(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	liturgia := uuid.MustParse(liturgiaID)
	seedChannel(t, pool, "email", "boletin", nil, true)
	e := seedPublishedEvent(t, pool, liturgia)

	stale := seedOutboxRow(t, pool, e.ID, store.OutboxUpdated)
	seedOutboxRow(t, pool, e.ID, store.OutboxUpdated) // the correction that wins

	enq := &fakeEnqueuer{}
	if err := Fanout(ctx, pool, enq, testConfig(), stale); err != nil {
		t.Fatalf("a collapsed update is not an error: %v", err)
	}
	rows, err := store.ListBroadcasts(ctx, pool, nil, &e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 || enq.count() != 0 {
		t.Errorf("a superseded edit must send nothing: %d broadcasts, %d tasks", len(rows), enq.count())
	}
}

func TestFanoutCancellationOnlyReachesThoseWhoHeard(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	liturgia := uuid.MustParse(liturgiaID)
	heard := seedChannel(t, pool, "email", "llego", nil, true)
	seedChannel(t, pool, "whatsapp", "no-llego", &liturgia, true)

	e := seedPublishedEvent(t, pool, liturgia)
	published := latestOutboxID(t, pool, e.ID)

	enq := &fakeEnqueuer{}
	if err := Fanout(ctx, pool, enq, testConfig(), published); err != nil {
		t.Fatal(err)
	}
	// Only one of the two actually made it out.
	rows, err := store.ListBroadcasts(ctx, pool, nil, &e.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.ChannelID == heard.ID {
			if err := store.MarkBroadcastSent(ctx, pool, r.ID); err != nil {
				t.Fatal(err)
			}
		} else if err := store.MarkBroadcastFailed(ctx, pool, r.ID, "sin conexión", true); err != nil {
			t.Fatal(err)
		}
	}

	cancelled := seedOutboxRow(t, pool, e.ID, store.OutboxCancelled)
	enq2 := &fakeEnqueuer{}
	if err := Fanout(ctx, pool, enq2, testConfig(), cancelled); err != nil {
		t.Fatal(err)
	}

	state := "queued"
	queued, err := store.ListBroadcasts(ctx, pool, &state, &e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(queued) != 1 || queued[0].ChannelID != heard.ID {
		t.Fatalf("a retraction goes only to channels that received the news, got %+v", queued)
	}
	if queued[0].Kind != store.OutboxCancelled {
		t.Errorf("expected a cancelled broadcast, got %s", queued[0].Kind)
	}
	if enq2.count() != 1 {
		t.Errorf("expected exactly 1 deliver task, got %d", enq2.count())
	}
}

func TestFanoutCancellationWithNoRecipientsDoesNothing(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	liturgia := uuid.MustParse(liturgiaID)
	seedChannel(t, pool, "email", "boletin", nil, true)
	e := seedPublishedEvent(t, pool, liturgia)

	cancelled := seedOutboxRow(t, pool, e.ID, store.OutboxCancelled)
	enq := &fakeEnqueuer{}
	if err := Fanout(ctx, pool, enq, testConfig(), cancelled); err != nil {
		t.Fatalf("nothing to retract is not an error: %v", err)
	}
	if enq.count() != 0 {
		t.Errorf("nothing was ever delivered, so nothing is retracted: %d tasks", enq.count())
	}
}

func TestFanoutMissingOutboxRowIsSkipped(t *testing.T) {
	pool := testdb.New(t)
	enq := &fakeEnqueuer{}
	// A queue entry can outlive its row after a manual cleanup; retrying
	// forever would just fill the dead-letter queue.
	if err := Fanout(context.Background(), pool, enq, testConfig(), 999999); err != nil {
		t.Errorf("a missing outbox row must be skipped, got %v", err)
	}
	if enq.count() != 0 {
		t.Errorf("expected no tasks, got %d", enq.count())
	}
}

// seedOutboxRow writes an outbox row carrying a real snapshot of the event.
func seedOutboxRow(t *testing.T, pool *pgxpool.Pool, eventID uuid.UUID, kind store.OutboxKind) int64 {
	t.Helper()
	var id int64
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO outbox (event_id, kind, payload)
		 SELECT $1, $2, jsonb_build_object(
		   'id', e.id::text, 'title', e.title, 'place', coalesce(e.place,''),
		   'description', coalesce(e.description,''),
		   'starts_at', e.starts_at, 'ends_at', e.ends_at,
		   'group_id', e.group_id::text, 'rank', e.rank::text)
		 FROM events e WHERE e.id = $1
		 RETURNING id`, eventID, string(kind)).Scan(&id); err != nil {
		t.Fatalf("seed outbox: %v", err)
	}
	return id
}
