package difusion

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store/testdb"
)

func fanoutOutboxID(t *testing.T, e enqueued) int64 {
	t.Helper()
	if e.task.Type() != TypeFanout {
		t.Fatalf("expected a fanout task, got %q", e.task.Type())
	}
	var p FanoutPayload
	if err := json.Unmarshal(e.task.Payload(), &p); err != nil {
		t.Fatalf("fanout payload: %v", err)
	}
	return p.OutboxID
}

func TestRelayOnceEnqueuesAndMarksProcessed(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	liturgia := uuid.MustParse(liturgiaID)
	e := seedPublishedEvent(t, pool, liturgia)
	outboxID := latestOutboxID(t, pool, e.ID)

	enq := &fakeEnqueuer{}
	n, err := RelayOnce(ctx, pool, enq)
	if err != nil {
		t.Fatalf("RelayOnce: %v", err)
	}
	if n != 1 || enq.count() != 1 {
		t.Fatalf("expected 1 row relayed and 1 task, got n=%d tasks=%d", n, enq.count())
	}
	if got := fanoutOutboxID(t, enq.sent[0]); got != outboxID {
		t.Errorf("relayed the wrong row: %d, want %d", got, outboxID)
	}
	if enq.sent[0].queue() != QueueFanout {
		t.Errorf("fanout belongs on its own queue, got %q", enq.sent[0].queue())
	}
	// A published announcement waits for nothing.
	if v, ok := enq.sent[0].option(asynq.ProcessInOpt); ok && v != time.Duration(0) {
		t.Errorf("a new event must go out immediately, got ProcessIn %v", v)
	}

	// Nothing is left for the next tick.
	if n, err = RelayOnce(ctx, pool, enq); err != nil || n != 0 {
		t.Errorf("second tick: n=%d err=%v", n, err)
	}
}

func TestRelayOnceDebouncesEdits(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	liturgia := uuid.MustParse(liturgiaID)
	e := seedPublishedEvent(t, pool, liturgia)

	// Drain the published row so only the edit is in play.
	if _, err := RelayOnce(ctx, pool, &fakeEnqueuer{}); err != nil {
		t.Fatal(err)
	}
	seedOutboxRow(t, pool, e.ID, store.OutboxUpdated)

	enq := &fakeEnqueuer{}
	if _, err := RelayOnce(ctx, pool, enq); err != nil {
		t.Fatal(err)
	}
	if enq.count() != 1 {
		t.Fatalf("expected the edit relayed once, got %d", enq.count())
	}
	// The window exists so a secretary correcting three fields in a row
	// produces one message, not three.
	v, ok := enq.sent[0].option(asynq.ProcessInOpt)
	if !ok {
		t.Fatal("an edit must be held for the debounce window")
	}
	if v != debounceWindow {
		t.Errorf("expected ProcessIn %v, got %v", debounceWindow, v)
	}
}

func TestRelayOnceCollapsesSupersededEdits(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	liturgia := uuid.MustParse(liturgiaID)
	e := seedPublishedEvent(t, pool, liturgia)
	if _, err := RelayOnce(ctx, pool, &fakeEnqueuer{}); err != nil {
		t.Fatal(err)
	}

	seedOutboxRow(t, pool, e.ID, store.OutboxUpdated)
	seedOutboxRow(t, pool, e.ID, store.OutboxUpdated)
	seedOutboxRow(t, pool, e.ID, store.OutboxUpdated)

	enq := &fakeEnqueuer{}
	n, err := RelayOnce(ctx, pool, enq)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("all three rows are consumed, got %d", n)
	}
	if enq.count() != 1 {
		t.Errorf("only the newest edit is announced, got %d tasks", enq.count())
	}
	var left int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM outbox WHERE processed_at IS NULL`).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Errorf("collapsed rows must still be marked processed, %d left", left)
	}
}

func TestRelayOnceLeavesRowsWhenEnqueueFails(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	liturgia := uuid.MustParse(liturgiaID)
	seedPublishedEvent(t, pool, liturgia)

	broken := &fakeEnqueuer{err: errors.New("redis caído")}
	if _, err := RelayOnce(ctx, pool, broken); err == nil {
		t.Fatal("a broker failure must be reported")
	}
	// Marking it processed would lose the announcement for good.
	var left int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM outbox WHERE processed_at IS NULL`).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 1 {
		t.Errorf("the row must survive for the next tick, got %d unprocessed", left)
	}
}
