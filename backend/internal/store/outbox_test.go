package store_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store/testdb"
)

// seedOutbox writes an outbox row directly, so a test can build the exact
// sequence the relay has to cope with without going through the API.
func seedOutbox(t *testing.T, pool *pgxpool.Pool, eventID uuid.UUID, kind store.OutboxKind) int64 {
	t.Helper()
	var id int64
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO outbox (event_id, kind, payload)
		 VALUES ($1,$2,jsonb_build_object('id',$3::text,'title','Hora santa'))
		 RETURNING id`, eventID, string(kind), eventID.String()).Scan(&id); err != nil {
		t.Fatalf("seed outbox: %v", err)
	}
	return id
}

func unprocessedCount(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM outbox WHERE processed_at IS NULL`).Scan(&n); err != nil {
		t.Fatalf("count outbox: %v", err)
	}
	return n
}

func TestGetOutboxRow(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	e := seedDraft(t, pool)
	if err := store.PublishEvent(ctx, pool, e.ID); err != nil {
		t.Fatal(err)
	}

	var id int64
	if err := pool.QueryRow(ctx,
		`SELECT id FROM outbox WHERE event_id = $1`, e.ID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	row, err := store.GetOutboxRow(ctx, pool, id)
	if err != nil {
		t.Fatalf("GetOutboxRow: %v", err)
	}
	if row.Kind != store.OutboxPublished || row.EventID != e.ID {
		t.Errorf("wrong row: %+v", row)
	}
	// The snapshot is what messages render from, so it must survive the trip.
	if row.Payload.Title != "Hora santa" || row.Payload.Place != "Templo" {
		t.Errorf("payload not decoded: %+v", row.Payload)
	}
}

func TestClaimOutboxBatchMarksProcessed(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	e := seedDraft(t, pool)
	first := seedOutbox(t, pool, e.ID, store.OutboxPublished)
	second := seedOutbox(t, pool, e.ID, store.OutboxUpdated)

	var seen []int64
	n, err := store.ClaimOutboxBatch(ctx, pool, 10, func(_ context.Context, row store.OutboxRow) error {
		seen = append(seen, row.ID)
		return nil
	})
	if err != nil {
		t.Fatalf("ClaimOutboxBatch: %v", err)
	}
	if n != 2 || len(seen) != 2 || seen[0] != first || seen[1] != second {
		t.Errorf("expected the two rows oldest first, got n=%d seen=%v", n, seen)
	}
	if left := unprocessedCount(t, pool); left != 0 {
		t.Errorf("claimed rows must be marked processed, %d left", left)
	}

	// A second pass has nothing to do.
	if n, err = store.ClaimOutboxBatch(ctx, pool, 10, func(context.Context, store.OutboxRow) error {
		t.Error("no row should be claimed twice")
		return nil
	}); err != nil || n != 0 {
		t.Errorf("second pass: n=%d err=%v", n, err)
	}
}

func TestClaimOutboxBatchRollsBackOnError(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	e := seedDraft(t, pool)
	seedOutbox(t, pool, e.ID, store.OutboxPublished)

	boom := errors.New("enqueue caído")
	if _, err := store.ClaimOutboxBatch(ctx, pool, 10,
		func(context.Context, store.OutboxRow) error { return boom }); !errors.Is(err, boom) {
		t.Fatalf("expected the callback error back, got %v", err)
	}
	// Marking a row processed after failing to enqueue it would lose the
	// announcement silently.
	if left := unprocessedCount(t, pool); left != 1 {
		t.Errorf("a failed batch must stay unprocessed, got %d", left)
	}
}

func TestClaimOutboxBatchSkipsLockedRows(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	e := seedDraft(t, pool)
	for i := 0; i < 4; i++ {
		seedOutbox(t, pool, e.ID, store.OutboxPublished)
	}

	// Two relay loops (two worker instances) claim at the same time. SKIP
	// LOCKED is what keeps them from both sending the same announcement.
	var mu sync.Mutex
	seen := map[int64]int{}
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = store.ClaimOutboxBatch(ctx, pool, 10, func(_ context.Context, row store.OutboxRow) error {
				mu.Lock()
				seen[row.ID]++
				mu.Unlock()
				time.Sleep(50 * time.Millisecond)
				return nil
			})
		}()
	}
	wg.Wait()

	for id, times := range seen {
		if times != 1 {
			t.Errorf("outbox row %d was claimed %d times", id, times)
		}
	}
	if len(seen) != 4 {
		t.Errorf("expected all 4 rows claimed exactly once between both loops, got %d", len(seen))
	}
	if left := unprocessedCount(t, pool); left != 0 {
		t.Errorf("expected every row processed, %d left", left)
	}
}

func TestHasNewerUpdated(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	e := seedDraft(t, pool)
	other := seedDraft(t, pool)

	first := seedOutbox(t, pool, e.ID, store.OutboxUpdated)
	newer, err := store.HasNewerUpdated(ctx, pool, e.ID, first)
	if err != nil {
		t.Fatalf("HasNewerUpdated: %v", err)
	}
	if newer {
		t.Error("with a single updated row there is nothing newer")
	}

	// A second edit within the debounce window collapses the first.
	seedOutbox(t, pool, e.ID, store.OutboxUpdated)
	if newer, err = store.HasNewerUpdated(ctx, pool, e.ID, first); err != nil || !newer {
		t.Errorf("a later updated row must supersede: newer=%v err=%v", newer, err)
	}

	// Neither another event's edit nor a different kind counts.
	base := seedOutbox(t, pool, other.ID, store.OutboxUpdated)
	seedOutbox(t, pool, other.ID, store.OutboxCancelled)
	if newer, err = store.HasNewerUpdated(ctx, pool, other.ID, base); err != nil || newer {
		t.Errorf("only newer `updated` rows of the same event count: newer=%v err=%v", newer, err)
	}
}
