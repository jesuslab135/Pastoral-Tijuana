package store_test

import (
	"context"
	"testing"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store/testdb"
)

func TestSeasonSeedHasNoGaps(t *testing.T) {
	pool := testdb.New(t)
	// Every date from 2026-01-01 to 2028-01-09 must fall in exactly one season.
	var gaps int
	err := pool.QueryRow(context.Background(), `
		SELECT count(*) FROM generate_series(
			'2026-01-01'::date, '2028-01-09'::date, interval '1 day') AS d
		WHERE NOT EXISTS (
			SELECT 1 FROM liturgical_seasons s WHERE s.date_range @> d::date)`,
	).Scan(&gaps)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if gaps != 0 {
		t.Errorf("found %d dates not covered by any season", gaps)
	}
}

func TestGaudeteIsRosa(t *testing.T) {
	pool := testdb.New(t)
	var color string
	err := pool.QueryRow(context.Background(),
		`SELECT color FROM liturgical_seasons WHERE date_range @> '2026-12-13'::date`,
	).Scan(&color)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if color != "rosa" {
		t.Errorf("2026-12-13 should be rosa (Gaudete), got %s", color)
	}
}

func TestGroupsSeeded(t *testing.T) {
	pool := testdb.New(t)
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM parish_groups WHERE is_public`).Scan(&n); err != nil {
		t.Fatalf("query: %v", err)
	}
	if n != 6 {
		t.Errorf("expected 6 public groups, got %d", n)
	}
}
