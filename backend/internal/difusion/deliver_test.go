package difusion

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store/testdb"
)

// recordingSender captures what a channel would have received.
type recordingSender struct {
	sent []OutboundMessage
	err  error
}

func (s *recordingSender) Send(_ context.Context, msg OutboundMessage) error {
	if s.err != nil {
		return s.err
	}
	s.sent = append(s.sent, msg)
	return nil
}

func deliverFixture(t *testing.T, pool *pgxpool.Pool, kind string) (store.Broadcast, DeliverPayload, store.Channel) {
	t.Helper()
	ctx := context.Background()
	liturgia := uuid.MustParse(liturgiaID)
	ch := seedChannel(t, pool, kind, "boletin", nil, true)
	e := seedPublishedEvent(t, pool, liturgia)
	outboxID := latestOutboxID(t, pool, e.ID)

	b := store.Broadcast{
		ID: uuid.New(), EventID: e.ID, ChannelID: ch.ID,
		Kind: store.OutboxPublished, State: "queued",
		DedupeKey: store.DedupeKey(e.ID, ch.ID, store.OutboxPublished, outboxID),
	}
	if _, err := store.CreateBroadcast(ctx, pool, b); err != nil {
		t.Fatal(err)
	}
	return b, DeliverPayload{BroadcastID: b.ID, OutboxID: outboxID}, ch
}

func deliverWith(t *testing.T, pool *pgxpool.Pool, s Sender, kind string, p DeliverPayload, retried, maxRetry int) error {
	t.Helper()
	return Deliver(context.Background(), pool, map[string]Sender{kind: s},
		parishTZ(t, "America/Tijuana"), baseURL, p, retried, maxRetry)
}

func TestDeliverSendsAndSettles(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	b, p, ch := deliverFixture(t, pool, "email")

	sender := &recordingSender{}
	if err := deliverWith(t, pool, sender, "email", p, 0, DeliverMaxRetry); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if len(sender.sent) != 1 {
		t.Fatalf("expected exactly one message, got %d", len(sender.sent))
	}
	msg := sender.sent[0]
	if msg.Target != ch.Target {
		t.Errorf("wrong target: %q", msg.Target)
	}
	if !strings.HasPrefix(msg.Subject, "Nuevo evento: ") || !strings.Contains(msg.Body, "Hora santa") {
		t.Errorf("message not rendered from the snapshot: %+v", msg)
	}

	got, err := store.GetBroadcast(ctx, pool, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != "sent" || got.SentAt == nil {
		t.Errorf("expected a settled broadcast, got state=%s sent_at=%v", got.State, got.SentAt)
	}
}

func TestDeliverFailureCountsTheAttempt(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	b, p, _ := deliverFixture(t, pool, "email")

	boom := errors.New("smtp rechazó la conexión")
	sender := &recordingSender{err: boom}
	// The error has to travel back to asynq, or the task is retired as done.
	if err := deliverWith(t, pool, sender, "email", p, 0, DeliverMaxRetry); !errors.Is(err, boom) {
		t.Fatalf("expected the sender error back, got %v", err)
	}
	got, err := store.GetBroadcast(ctx, pool, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != "failed" || got.Attempt != 1 {
		t.Errorf("expected failed/attempt 1, got %s/%d", got.State, got.Attempt)
	}
	if got.LastError == nil || !strings.Contains(*got.LastError, "smtp") {
		t.Errorf("the reason must be visible in the panel, got %v", got.LastError)
	}
}

func TestDeliverLastAttemptIsDead(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	b, p, _ := deliverFixture(t, pool, "email")

	sender := &recordingSender{err: errors.New("sin ruta al host")}
	if err := deliverWith(t, pool, sender, "email", p, DeliverMaxRetry, DeliverMaxRetry); err == nil {
		t.Fatal("the final attempt still reports the error so asynq archives the task")
	}
	got, err := store.GetBroadcast(ctx, pool, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != "dead" {
		t.Errorf("with no retries left the broadcast is dead, got %s", got.State)
	}
}

func TestDeliverSkipsWhatIsAlreadySent(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	b, p, _ := deliverFixture(t, pool, "email")
	if err := store.MarkBroadcastSent(ctx, pool, b.ID); err != nil {
		t.Fatal(err)
	}

	sender := &recordingSender{}
	// A duplicated queue entry must not announce the same event twice.
	if err := deliverWith(t, pool, sender, "email", p, 0, DeliverMaxRetry); err != nil {
		t.Fatalf("a delivered broadcast is skipped, not retried: %v", err)
	}
	if len(sender.sent) != 0 {
		t.Errorf("nothing may be sent again, got %d messages", len(sender.sent))
	}
}

func TestDeliverMissingBroadcastIsSkipped(t *testing.T) {
	pool := testdb.New(t)
	sender := &recordingSender{}
	p := DeliverPayload{BroadcastID: uuid.New(), OutboxID: 1}
	if err := deliverWith(t, pool, sender, "email", p, 0, DeliverMaxRetry); err != nil {
		t.Errorf("a stale queue entry must be skipped, got %v", err)
	}
	if len(sender.sent) != 0 {
		t.Errorf("expected no message, got %d", len(sender.sent))
	}
}

func TestDeliverToDeactivatedChannelStops(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	b, p, ch := deliverFixture(t, pool, "email")
	ch.IsActive = false
	if err := store.UpdateChannel(ctx, pool, ch); err != nil {
		t.Fatal(err)
	}

	sender := &recordingSender{}
	if err := deliverWith(t, pool, sender, "email", p, 0, DeliverMaxRetry); err != nil {
		t.Fatalf("a channel switched off mid-flight is not a failure to retry: %v", err)
	}
	if len(sender.sent) != 0 {
		t.Errorf("a deactivated channel must receive nothing, got %d", len(sender.sent))
	}
	got, err := store.GetBroadcast(ctx, pool, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != "dead" {
		t.Errorf("expected the broadcast marked dead, got %s", got.State)
	}
	if got.LastError == nil || *got.LastError != "canal desactivado" {
		t.Errorf("the panel must say why, got %v", got.LastError)
	}
}

func TestDeliverUnknownChannelKindFails(t *testing.T) {
	pool := testdb.New(t)
	_, p, _ := deliverFixture(t, pool, "whatsapp")
	// Only an email sender is wired: a whatsapp channel has nowhere to go.
	err := Deliver(context.Background(), pool, map[string]Sender{"email": &recordingSender{}},
		parishTZ(t, "America/Tijuana"), baseURL, p, 0, DeliverMaxRetry)
	if err == nil {
		t.Fatal("a channel kind with no sender must not report success")
	}
}
