package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store/testdb"
)

const liturgiaID = "a1000000-0000-4000-8000-000000000001"

func TestListPublishedEvents(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	now := time.Now()
	gid := uuid.MustParse(liturgiaID)

	published := store.Event{
		ID: uuid.New(), Title: "Hora santa", GroupID: gid, Rank: "parroquial",
		StartsAt:    time.Date(2026, 8, 4, 19, 0, 0, 0, time.UTC),
		EndsAt:      time.Date(2026, 8, 4, 20, 0, 0, 0, time.UTC),
		PublishedAt: &now,
	}
	draft := store.Event{
		ID: uuid.New(), Title: "Borrador secreto", GroupID: gid, Rank: "parroquial",
		StartsAt: time.Date(2026, 8, 5, 19, 0, 0, 0, time.UTC),
		EndsAt:   time.Date(2026, 8, 5, 20, 0, 0, 0, time.UTC),
	}
	for _, e := range []store.Event{published, draft} {
		if err := store.CreateEvent(ctx, pool, e); err != nil {
			t.Fatalf("CreateEvent: %v", err)
		}
	}

	from := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	got, err := store.ListPublishedEvents(ctx, pool, from, to, "America/Tijuana")
	if err != nil {
		t.Fatalf("ListPublishedEvents: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 published event (draft excluded), got %d", len(got))
	}
	e := got[0]
	if e.Title != "Hora santa" || e.GroupSlug != "liturgia" {
		t.Errorf("unexpected event: %+v", e)
	}
	// August 2026 is Tiempo Ordinario → verde.
	if e.Color != "verde" {
		t.Errorf("expected season color verde, got %q", e.Color)
	}
}

func TestColorOverrideWins(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	now := time.Now()
	rojo := "rojo"
	e := store.Event{
		ID: uuid.New(), Title: "Misa patronal", GroupID: uuid.MustParse(liturgiaID),
		Rank: "solemnidad", ColorOverride: &rojo,
		StartsAt:    time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC),
		EndsAt:      time.Date(2026, 8, 29, 13, 45, 0, 0, time.UTC),
		PublishedAt: &now,
	}
	if err := store.CreateEvent(ctx, pool, e); err != nil {
		t.Fatalf("CreateEvent: %v", err)
	}
	got, err := store.ListPublishedEvents(ctx, pool,
		time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), "America/Tijuana")
	if err != nil {
		t.Fatalf("ListPublishedEvents: %v", err)
	}
	if len(got) != 1 || got[0].Color != "rojo" {
		t.Fatalf("color_override should win over season color, got %+v", got)
	}
}

func TestListPublicGroups(t *testing.T) {
	pool := testdb.New(t)
	groups, err := store.ListPublicGroups(context.Background(), pool)
	if err != nil {
		t.Fatalf("ListPublicGroups: %v", err)
	}
	if len(groups) != 6 || groups[0].Slug != "liturgia" {
		t.Errorf("expected 6 groups sorted, liturgia first; got %+v", groups)
	}
}
