package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store/testdb"
)

func TestSeasonOf(t *testing.T) {
	pool := testdb.New(t)
	day := time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)
	s, err := store.SeasonOf(context.Background(), pool, day)
	if err != nil {
		t.Fatalf("SeasonOf: %v", err)
	}
	if s.Name != "Tiempo Ordinario" || s.Color != "verde" {
		t.Errorf("2026-08-12: got %q/%q, want Tiempo Ordinario/verde", s.Name, s.Color)
	}
}

func TestListSeasonsForYear(t *testing.T) {
	pool := testdb.New(t)
	seasons, err := store.ListSeasonsForYear(context.Background(), pool, 2026)
	if err != nil {
		t.Fatalf("ListSeasonsForYear: %v", err)
	}
	if len(seasons) < 6 {
		t.Fatalf("expected at least 6 season ranges touching 2026, got %d", len(seasons))
	}
	for _, s := range seasons {
		if !s.End.After(s.Start) {
			t.Errorf("season %q has End <= Start", s.Name)
		}
	}
}
