package difusion

import (
	"bytes"
	"context"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/mail"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store/testdb"
)

// safeBuffer collects what the senders wrote. The mail queue runs four
// handlers at once, so the buffer needs its own lock.
type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// testRedisAddr resolves the broker the pipeline test talks to. When
// TEST_REDIS_ADDR is set (CI does), an unreachable Redis fails the test
// instead of quietly skipping the only end-to-end coverage there is.
func testRedisAddr(t *testing.T) string {
	t.Helper()
	addr, explicit := os.LookupEnv("TEST_REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	c := redis.NewClient(&redis.Options{Addr: addr})
	defer c.Close()
	if err := c.Ping(context.Background()).Err(); err != nil {
		if explicit {
			t.Fatalf("redis no disponible en %s: %v", addr, err)
		}
		t.Skipf("redis no disponible en %s (define TEST_REDIS_ADDR para exigirlo): %v", addr, err)
	}
	// Earlier runs can leave scheduled tasks pointing at rows this test's
	// truncation removed. They are harmless (delivery skips them) but they
	// make the queue counts unreadable.
	if err := c.FlushDB(context.Background()).Err(); err != nil {
		t.Fatalf("limpiar redis de pruebas: %v", err)
	}
	return addr
}

// waitFor polls until cond holds, which is how a caller observes work done by
// the asynq servers on their own goroutines.
func waitFor(t *testing.T, timeout time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("tiempo agotado esperando %s", what)
}

func broadcastStates(t *testing.T, pool *pgxpool.Pool, eventID uuid.UUID) map[uuid.UUID]store.BroadcastRow {
	t.Helper()
	rows, err := store.ListBroadcasts(context.Background(), pool, nil, &eventID)
	if err != nil {
		t.Fatalf("ListBroadcasts: %v", err)
	}
	out := map[uuid.UUID]store.BroadcastRow{}
	for _, r := range rows {
		out[r.ID] = r
	}
	return out
}

func allSent(rows map[uuid.UUID]store.BroadcastRow) bool {
	if len(rows) == 0 {
		return false
	}
	for _, r := range rows {
		if r.State != "sent" {
			return false
		}
	}
	return true
}

// TestPipelineEndToEnd runs the real relay, the real mux and real asynq
// servers over a real Postgres and Redis: publishing an event has to reach
// both of its channels, and cancelling it has to reach exactly those two.
func TestPipelineEndToEnd(t *testing.T) {
	// testdb.New first: it takes the process-wide advisory lock, so no other
	// package's test binary is using this Redis when the flush below runs.
	pool := testdb.New(t)
	addr := testRedisAddr(t)
	ctx := context.Background()

	liturgia := uuid.MustParse(liturgiaID)
	catequesis := uuid.MustParse(catequesisID)
	seedChannel(t, pool, "whatsapp", "grupo-liturgia", &liturgia, true)
	seedChannel(t, pool, "email", "boletin-parroquia", nil, true)
	// Another group's channel proves the fanout is targeted, not a broadcast
	// to everything that exists.
	seedChannel(t, pool, "email", "boletin-catequesis", &catequesis, true)

	cfg := testConfig()
	cfg.PublicBaseURL = baseURL
	loc := parishTZ(t, cfg.ParishTZ)

	waLog := &safeBuffer{}
	mailLog := &safeBuffer{}
	senders := map[string]Sender{
		"whatsapp": &StubWhatsAppSender{Sink: log.New(waLog, "", 0)},
		"email":    &EmailSender{Mailer: &mail.LogMailer{Sink: log.New(mailLog, "", 0)}},
	}

	redisOpt := asynq.RedisClientOpt{Addr: addr}
	client := asynq.NewClient(redisOpt)
	t.Cleanup(func() { client.Close() })

	mux := NewMux(pool, client, senders, loc, cfg)
	srvWA := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: 1,
		Queues:      map[string]int{QueueWA: 1},
		LogLevel:    asynq.ErrorLevel,
	})
	srvMail := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: 4,
		Queues:      map[string]int{QueueMail: 1, QueueFanout: 1},
		LogLevel:    asynq.ErrorLevel,
	})
	for _, srv := range []*asynq.Server{srvWA, srvMail} {
		if err := srv.Start(mux); err != nil {
			t.Fatalf("start server: %v", err)
		}
		t.Cleanup(srv.Shutdown)
	}

	e := seedPublishedEvent(t, pool, liturgia)

	if n, err := RelayOnce(ctx, pool, client); err != nil || n != 1 {
		t.Fatalf("RelayOnce: n=%d err=%v", n, err)
	}

	waitFor(t, 20*time.Second, "que se entreguen los dos avisos", func() bool {
		rows := broadcastStates(t, pool, e.ID)
		return len(rows) == 2 && allSent(rows)
	})

	rows := broadcastStates(t, pool, e.ID)
	if len(rows) != 2 {
		t.Fatalf("expected exactly the group's channel and the parish-wide one, got %d", len(rows))
	}
	for _, r := range rows {
		if r.ChannelName == "boletin-catequesis" {
			t.Error("another group's channel must not be told")
		}
	}
	if !strings.Contains(waLog.String(), "Hora santa") || !strings.Contains(waLog.String(), "SIMULADO") {
		t.Errorf("the whatsapp stub log is wrong:\n%s", waLog.String())
	}
	if !strings.Contains(mailLog.String(), "Hora santa") {
		t.Errorf("the mail log is missing the event:\n%s", mailLog.String())
	}

	var unprocessed int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM outbox WHERE processed_at IS NULL`).Scan(&unprocessed); err != nil {
		t.Fatal(err)
	}
	if unprocessed != 0 {
		t.Errorf("the relay must consume what it enqueued, %d left", unprocessed)
	}

	// Cancelling reaches exactly the channels that heard about it.
	if err := store.DeleteEvent(ctx, pool, e.ID, true); err != nil {
		t.Fatalf("DeleteEvent: %v", err)
	}
	if n, err := RelayOnce(ctx, pool, client); err != nil || n != 1 {
		t.Fatalf("RelayOnce (cancelación): n=%d err=%v", n, err)
	}

	waitFor(t, 20*time.Second, "que se entreguen las cancelaciones", func() bool {
		rows := broadcastStates(t, pool, e.ID)
		return len(rows) == 4 && allSent(rows)
	})

	cancelled := map[string]int{}
	for _, r := range broadcastStates(t, pool, e.ID) {
		if r.Kind == store.OutboxCancelled {
			cancelled[r.ChannelName]++
		}
	}
	if len(cancelled) != 2 || cancelled["grupo-liturgia"] != 1 || cancelled["boletin-parroquia"] != 1 {
		t.Errorf("a retraction goes to exactly the channels that received the news, got %v", cancelled)
	}
	if !strings.Contains(mailLog.String(), "Este evento se canceló.") {
		t.Errorf("the cancellation text never went out:\n%s", mailLog.String())
	}
}
