package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store/testdb"
)

const catequesisID = "a1000000-0000-4000-8000-000000000002"

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

func newBroadcast(eventID, channelID uuid.UUID, kind store.OutboxKind, outboxID int64) store.Broadcast {
	return store.Broadcast{
		ID: uuid.New(), EventID: eventID, ChannelID: channelID, Kind: kind,
		State:     "queued",
		DedupeKey: store.DedupeKey(eventID, channelID, kind, outboxID),
	}
}

func countBroadcasts(t *testing.T, pool *pgxpool.Pool, eventID uuid.UUID) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM broadcasts WHERE event_id = $1`, eventID).Scan(&n); err != nil {
		t.Fatalf("count broadcasts: %v", err)
	}
	return n
}

func TestCreateBroadcastIsIdempotent(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	e := seedDraft(t, pool)
	ch := seedChannel(t, pool, "email", "boletin", nil, true)

	b := newBroadcast(e.ID, ch.ID, store.OutboxPublished, 1)
	inserted, err := store.CreateBroadcast(ctx, pool, b)
	if err != nil || !inserted {
		t.Fatalf("first insert: inserted=%v err=%v", inserted, err)
	}

	// A retried fanout replays the same key; it must not announce twice.
	again := newBroadcast(e.ID, ch.ID, store.OutboxPublished, 1)
	inserted, err = store.CreateBroadcast(ctx, pool, again)
	if err != nil {
		t.Fatalf("second insert: %v", err)
	}
	if inserted {
		t.Error("a duplicate dedupe key must report inserted=false")
	}
	if n := countBroadcasts(t, pool, e.ID); n != 1 {
		t.Errorf("expected 1 broadcast row, got %d", n)
	}

	// A different outbox row for the same channel is a different announcement.
	next := newBroadcast(e.ID, ch.ID, store.OutboxUpdated, 2)
	if inserted, err = store.CreateBroadcast(ctx, pool, next); err != nil || !inserted {
		t.Fatalf("distinct key: inserted=%v err=%v", inserted, err)
	}
}

func TestBroadcastStateTransitions(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	e := seedDraft(t, pool)
	ch := seedChannel(t, pool, "email", "boletin", nil, true)

	b := newBroadcast(e.ID, ch.ID, store.OutboxPublished, 1)
	if _, err := store.CreateBroadcast(ctx, pool, b); err != nil {
		t.Fatal(err)
	}

	if err := store.MarkBroadcastFailed(ctx, pool, b.ID, "smtp caído", false); err != nil {
		t.Fatalf("MarkBroadcastFailed: %v", err)
	}
	got, err := store.GetBroadcast(ctx, pool, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != "failed" || got.Attempt != 1 {
		t.Errorf("after one failure: state=%s attempt=%d", got.State, got.Attempt)
	}
	if got.LastError == nil || *got.LastError != "smtp caído" {
		t.Errorf("last_error not stored: %v", got.LastError)
	}

	if err := store.MarkBroadcastFailed(ctx, pool, b.ID, "sin reintentos", true); err != nil {
		t.Fatal(err)
	}
	if got, err = store.GetBroadcast(ctx, pool, b.ID); err != nil {
		t.Fatal(err)
	}
	if got.State != "dead" || got.Attempt != 2 {
		t.Errorf("after exhaustion: state=%s attempt=%d", got.State, got.Attempt)
	}

	// A dead broadcast is exactly what the panel's retry button targets.
	if err := store.ResetBroadcastForRetry(ctx, pool, b.ID); err != nil {
		t.Fatalf("ResetBroadcastForRetry: %v", err)
	}
	if got, err = store.GetBroadcast(ctx, pool, b.ID); err != nil {
		t.Fatal(err)
	}
	if got.State != "queued" {
		t.Errorf("retry should requeue, got %s", got.State)
	}

	if err := store.MarkBroadcastSent(ctx, pool, b.ID); err != nil {
		t.Fatalf("MarkBroadcastSent: %v", err)
	}
	if got, err = store.GetBroadcast(ctx, pool, b.ID); err != nil {
		t.Fatal(err)
	}
	if got.State != "sent" || got.SentAt == nil {
		t.Errorf("after send: state=%s sent_at=%v", got.State, got.SentAt)
	}

	// Retrying something already delivered would announce it twice.
	if err := store.ResetBroadcastForRetry(ctx, pool, b.ID); !errors.Is(err, store.ErrNotRetryable) {
		t.Errorf("retry on sent: expected ErrNotRetryable, got %v", err)
	}
	if err := store.ResetBroadcastForRetry(ctx, pool, uuid.New()); !errors.Is(err, store.ErrNotRetryable) {
		t.Errorf("retry on unknown id: expected ErrNotRetryable, got %v", err)
	}
}

func TestActiveChannelsForGroup(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	liturgia := uuid.MustParse(liturgiaID)
	catequesis := uuid.MustParse(catequesisID)

	mine := seedChannel(t, pool, "whatsapp", "liturgia", &liturgia, true)
	parish := seedChannel(t, pool, "email", "toda-la-parroquia", nil, true)
	seedChannel(t, pool, "whatsapp", "catequesis", &catequesis, true)
	seedChannel(t, pool, "whatsapp", "liturgia-apagado", &liturgia, false)
	seedChannel(t, pool, "email", "parroquia-apagado", nil, false)

	got, err := store.ActiveChannelsForGroup(ctx, pool, liturgia)
	if err != nil {
		t.Fatalf("ActiveChannelsForGroup: %v", err)
	}
	ids := map[uuid.UUID]bool{}
	for _, c := range got {
		ids[c.ID] = true
	}
	if len(got) != 2 || !ids[mine.ID] || !ids[parish.ID] {
		t.Errorf("expected the group's channel plus the parish-wide one, got %d: %v", len(got), ids)
	}
}

func TestBroadcastRecipientsOnlyCountsDelivered(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	e := seedDraft(t, pool)
	delivered := seedChannel(t, pool, "email", "llego", nil, true)
	bounced := seedChannel(t, pool, "whatsapp", "no-llego", nil, true)

	ok := newBroadcast(e.ID, delivered.ID, store.OutboxPublished, 1)
	bad := newBroadcast(e.ID, bounced.ID, store.OutboxPublished, 1)
	for _, b := range []store.Broadcast{ok, bad} {
		if _, err := store.CreateBroadcast(ctx, pool, b); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.MarkBroadcastSent(ctx, pool, ok.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkBroadcastFailed(ctx, pool, bad.ID, "sin conexión", true); err != nil {
		t.Fatal(err)
	}

	got, err := store.BroadcastRecipients(ctx, pool, e.ID)
	if err != nil {
		t.Fatalf("BroadcastRecipients: %v", err)
	}
	// Retracting an announcement nobody received would be the first time
	// that channel hears of the event.
	if len(got) != 1 || got[0] != delivered.ID {
		t.Errorf("expected only the channel that received it, got %v", got)
	}
}

func TestListBroadcastsFilters(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	e1 := seedDraft(t, pool)
	e2 := seedDraft(t, pool)
	ch := seedChannel(t, pool, "email", "boletin", nil, true)

	sent := newBroadcast(e1.ID, ch.ID, store.OutboxPublished, 1)
	queued := newBroadcast(e2.ID, ch.ID, store.OutboxPublished, 2)
	for _, b := range []store.Broadcast{sent, queued} {
		if _, err := store.CreateBroadcast(ctx, pool, b); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.MarkBroadcastSent(ctx, pool, sent.ID); err != nil {
		t.Fatal(err)
	}

	all, err := store.ListBroadcasts(ctx, pool, nil, nil)
	if err != nil {
		t.Fatalf("ListBroadcasts: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("unfiltered: expected 2, got %d", len(all))
	}
	if all[0].ChannelName != "boletin" || all[0].ChannelKind != "email" {
		t.Errorf("the panel needs the channel joined in, got %+v", all[0])
	}

	state := "sent"
	bySt, err := store.ListBroadcasts(ctx, pool, &state, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(bySt) != 1 || bySt[0].ID != sent.ID {
		t.Errorf("state filter: got %+v", bySt)
	}

	byEv, err := store.ListBroadcasts(ctx, pool, nil, &e2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(byEv) != 1 || byEv[0].ID != queued.ID {
		t.Errorf("event filter: got %+v", byEv)
	}
}
