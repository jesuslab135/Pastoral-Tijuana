package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store/testdb"
)

// privateGroupID is a fixed UUID so repeated runs reuse the same row instead
// of accumulating groups; testdb does not truncate parish_groups.
const privateGroupID = "a1000000-0000-4000-8000-0000000000ff"

func TestEventOutsideSeededSeasonsIsStillListed(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	now := time.Now()

	// 2035 is far beyond the seeded season ranges (which stop at 2028-01-10).
	if err := store.CreateEvent(ctx, pool, store.Event{
		ID: uuid.New(), Title: "Misa del futuro",
		GroupID: uuid.MustParse(liturgiaID), Rank: "parroquial",
		StartsAt:    time.Date(2035, 6, 10, 19, 0, 0, 0, time.UTC),
		EndsAt:      time.Date(2035, 6, 10, 20, 0, 0, 0, time.UTC),
		PublishedAt: &now,
	}); err != nil {
		t.Fatalf("CreateEvent: %v", err)
	}

	got, err := store.ListPublishedEvents(ctx, pool,
		time.Date(2035, 6, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2035, 7, 1, 0, 0, 0, 0, time.UTC), "America/Tijuana")
	if err != nil {
		t.Fatalf("ListPublishedEvents: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("event outside the seeded seasons must still be listed, got %d events", len(got))
	}
	if got[0].Color != store.DefaultSeasonColor {
		t.Errorf("expected fallback color %q, got %q", store.DefaultSeasonColor, got[0].Color)
	}
}

func TestNonPublicGroupEventsAreHidden(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	now := time.Now()

	if _, err := pool.Exec(ctx,
		`INSERT INTO parish_groups (id, name, slug, is_public, sort)
		 VALUES ($1, 'Grupo privado', 'privado', false, 99)
		 ON CONFLICT (id) DO UPDATE SET is_public = false`,
		privateGroupID); err != nil {
		t.Fatalf("seed private group: %v", err)
	}

	if err := store.CreateEvent(ctx, pool, store.Event{
		ID: uuid.New(), Title: "Reunión reservada",
		GroupID: uuid.MustParse(privateGroupID), Rank: "parroquial",
		StartsAt:    time.Date(2026, 8, 20, 19, 0, 0, 0, time.UTC),
		EndsAt:      time.Date(2026, 8, 20, 20, 0, 0, 0, time.UTC),
		PublishedAt: &now,
	}); err != nil {
		t.Fatalf("CreateEvent: %v", err)
	}

	got, err := store.ListPublishedEvents(ctx, pool,
		time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), "America/Tijuana")
	if err != nil {
		t.Fatalf("ListPublishedEvents: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("events of a non-public group must not be listed, got %+v", got)
	}

	feed, err := store.ListEventsForICS(ctx, pool, nil, time.Now())
	if err != nil {
		t.Fatalf("ListEventsForICS: %v", err)
	}
	for _, e := range feed {
		if e.Title == "Reunión reservada" {
			t.Error("events of a non-public group must not appear in the ics feed")
		}
	}
}

func TestUpdatedAtAdvancesOnWrite(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	now := time.Now()
	id := uuid.New()

	if err := store.CreateEvent(ctx, pool, store.Event{
		ID: id, Title: "Hora santa",
		GroupID: uuid.MustParse(liturgiaID), Rank: "parroquial",
		StartsAt:    time.Date(2026, 8, 4, 19, 0, 0, 0, time.UTC),
		EndsAt:      time.Date(2026, 8, 4, 20, 0, 0, 0, time.UTC),
		PublishedAt: &now,
	}); err != nil {
		t.Fatalf("CreateEvent: %v", err)
	}

	var before, after time.Time
	if err := pool.QueryRow(ctx,
		`SELECT updated_at FROM events WHERE id = $1`, id).Scan(&before); err != nil {
		t.Fatalf("read updated_at: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE events SET title = 'Hora santa (nuevo horario)' WHERE id = $1`, id); err != nil {
		t.Fatalf("update: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT updated_at FROM events WHERE id = $1`, id).Scan(&after); err != nil {
		t.Fatalf("re-read updated_at: %v", err)
	}
	if !after.After(before) {
		t.Errorf("updated_at must advance on UPDATE (ics ETag and SEQUENCE depend on it): before=%s after=%s",
			before, after)
	}
}
