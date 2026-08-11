package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store/testdb"
)

func seedDraft(t *testing.T, pool *pgxpool.Pool) store.Event {
	t.Helper()
	e := store.Event{
		ID: uuid.New(), Title: "Hora santa", Place: "Templo",
		GroupID: uuid.MustParse(liturgiaID), Rank: "parroquial",
		StartsAt: time.Date(2026, 8, 4, 19, 0, 0, 0, time.UTC),
		EndsAt:   time.Date(2026, 8, 4, 20, 0, 0, 0, time.UTC),
	}
	if err := store.CreateEvent(context.Background(), pool, e); err != nil {
		t.Fatalf("CreateEvent: %v", err)
	}
	return e
}

func outboxKinds(t *testing.T, pool *pgxpool.Pool, eventID uuid.UUID) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`SELECT kind::text FROM outbox WHERE event_id = $1 ORDER BY id`, eventID)
	if err != nil {
		t.Fatalf("outbox query: %v", err)
	}
	defer rows.Close()
	var kinds []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			t.Fatal(err)
		}
		kinds = append(kinds, k)
	}
	return kinds
}

func TestPublishWritesOutbox(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	e := seedDraft(t, pool)

	if err := store.PublishEvent(ctx, pool, e.ID); err != nil {
		t.Fatalf("PublishEvent: %v", err)
	}
	got, err := store.GetEventAdmin(ctx, pool, e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.PublishedAt == nil {
		t.Error("published_at must be set")
	}
	if k := outboxKinds(t, pool, e.ID); len(k) != 1 || k[0] != "published" {
		t.Fatalf("outbox = %v, want [published]", k)
	}

	// The payload is the snapshot the difusión worker will render.
	var raw []byte
	if err := pool.QueryRow(ctx,
		`SELECT payload FROM outbox WHERE event_id = $1`, e.ID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var p store.OutboxPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("payload not valid json: %v", err)
	}
	if p.Title != "Hora santa" || p.Place != "Templo" || !p.StartsAt.Equal(e.StartsAt) {
		t.Errorf("payload snapshot wrong: %+v", p)
	}
}

func TestPublishTwiceFails(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	e := seedDraft(t, pool)

	if err := store.PublishEvent(ctx, pool, e.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.PublishEvent(ctx, pool, e.ID); !errors.Is(err, store.ErrAlreadyPublished) {
		t.Errorf("expected ErrAlreadyPublished, got %v", err)
	}
	if k := outboxKinds(t, pool, e.ID); len(k) != 1 {
		t.Errorf("a failed publish must not queue a second announcement: %v", k)
	}
}

func TestBroadcastWorthyEditWritesUpdated(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	e := seedDraft(t, pool)
	if err := store.PublishEvent(ctx, pool, e.ID); err != nil {
		t.Fatal(err)
	}

	e.Place = "Salón parroquial"
	if err := store.UpdateEvent(ctx, pool, e); err != nil {
		t.Fatalf("UpdateEvent: %v", err)
	}
	if k := outboxKinds(t, pool, e.ID); len(k) != 2 || k[1] != "updated" {
		t.Errorf("outbox = %v, want [published updated]", k)
	}
}

func TestSilentEditWritesNothing(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	e := seedDraft(t, pool)
	if err := store.PublishEvent(ctx, pool, e.ID); err != nil {
		t.Fatal(err)
	}

	e.Title = "Hora Santa (corregido)"
	e.Description = "Con exposición del Santísimo"
	if err := store.UpdateEvent(ctx, pool, e); err != nil {
		t.Fatalf("UpdateEvent: %v", err)
	}
	if k := outboxKinds(t, pool, e.ID); len(k) != 1 {
		t.Errorf("a title or description edit must stay silent, got %v", k)
	}
}

func TestDraftEditWritesNothing(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	e := seedDraft(t, pool)

	e.StartsAt = e.StartsAt.Add(time.Hour)
	e.EndsAt = e.EndsAt.Add(time.Hour)
	if err := store.UpdateEvent(ctx, pool, e); err != nil {
		t.Fatalf("UpdateEvent: %v", err)
	}
	if k := outboxKinds(t, pool, e.ID); len(k) != 0 {
		t.Errorf("an unpublished event must never queue difusión, got %v", k)
	}
}

func TestUnpublishWritesCancelled(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	e := seedDraft(t, pool)
	if err := store.PublishEvent(ctx, pool, e.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.UnpublishEvent(ctx, pool, e.ID); err != nil {
		t.Fatalf("UnpublishEvent: %v", err)
	}
	got, err := store.GetEventAdmin(ctx, pool, e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.PublishedAt != nil {
		t.Error("published_at must be cleared")
	}
	if k := outboxKinds(t, pool, e.ID); len(k) != 2 || k[1] != "cancelled" {
		t.Errorf("outbox = %v, want [published cancelled]", k)
	}
}

func TestDeleteNotifySoftCancels(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	e := seedDraft(t, pool)
	if err := store.PublishEvent(ctx, pool, e.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteEvent(ctx, pool, e.ID, true); err != nil {
		t.Fatalf("DeleteEvent: %v", err)
	}
	got, err := store.GetEventAdmin(ctx, pool, e.ID)
	if err != nil {
		t.Fatalf("a notified deletion must keep the row for the ics feed: %v", err)
	}
	if got.CancelledAt == nil {
		t.Error("cancelled_at must be set")
	}
	if k := outboxKinds(t, pool, e.ID); len(k) != 2 || k[1] != "cancelled" {
		t.Errorf("outbox = %v, want [published cancelled]", k)
	}
}

func TestDeleteWithoutNotifyHardDeletes(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	e := seedDraft(t, pool)
	if err := store.PublishEvent(ctx, pool, e.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteEvent(ctx, pool, e.ID, false); err != nil {
		t.Fatalf("DeleteEvent: %v", err)
	}
	if _, err := store.GetEventAdmin(ctx, pool, e.ID); err == nil {
		t.Error("event must be gone")
	}
	if k := outboxKinds(t, pool, e.ID); len(k) != 1 {
		t.Errorf("a silent deletion must not queue a cancellation, got %v", k)
	}
}

func TestListEventsAdminIncludesDrafts(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	seedDraft(t, pool)

	got, err := store.ListEventsAdmin(ctx, pool,
		time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("ListEventsAdmin: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("drafts must be visible to the admin, got %d events", len(got))
	}
}
