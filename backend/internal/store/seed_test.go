package store_test

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
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

// dbURLWith returns TEST_DATABASE_URL pointed at another database on the same
// server, so the seed can be observed in a scratch database of its own.
func dbURLWith(t *testing.T, name string) string {
	t.Helper()
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		raw = "postgres://pastoral:pastoral@localhost:5433/pastoral_test?sslmode=disable"
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	u.Path = "/" + name
	return u.String()
}

// TestFreshDatabaseHasNoChannels checks what production's first boot gets:
// migrations must leave channels EMPTY — the párroco creates the real ones in
// the admin panel. (A channels seed once lived here as migration 00005; its
// active email channel pointed at a placeholder domain that real SMTP would
// have mailed.) testdb.New truncates channels on every call, so the shared
// test database is vacuous for this; instead this migrates a scratch database
// of its own and counts there. Calling testdb.New first is what makes that
// safe: it takes the process-wide advisory lock, so no other test binary is
// creating or dropping databases on this server at the same time.
func TestFreshDatabaseHasNoChannels(t *testing.T) {
	testdb.New(t)

	admin, err := sql.Open("pgx", dbURLWith(t, "postgres"))
	if err != nil {
		t.Fatalf("open maintenance db: %v", err)
	}
	defer admin.Close()

	const scratch = "pastoral_seed_test"
	drop := func() {
		if _, err := admin.Exec(`DROP DATABASE IF EXISTS ` + scratch + ` WITH (FORCE)`); err != nil {
			t.Errorf("drop scratch db: %v", err)
		}
	}
	drop() // a killed earlier run may have left it behind
	if _, err := admin.Exec(`CREATE DATABASE ` + scratch); err != nil {
		t.Fatalf("create scratch db: %v", err)
	}
	// Deferred rather than t.Cleanup: cleanups run after the deferred
	// admin.Close, which would leave nothing to drop the database with.
	defer drop()

	db, err := sql.Open("pgx", dbURLWith(t, scratch))
	if err != nil {
		t.Fatalf("open scratch db: %v", err)
	}
	defer db.Close()

	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate scratch db: %v", err)
	}
	// Migrations must be re-runnable: a second Up is what every restarting
	// API instance does.
	if err := store.Migrate(db); err != nil {
		t.Fatalf("second migrate must be a no-op: %v", err)
	}

	var n int
	if err := db.QueryRow(`SELECT count(*) FROM channels`).Scan(&n); err != nil {
		t.Fatalf("count channels: %v", err)
	}
	if n != 0 {
		t.Errorf("a fresh database must have no channels, got %d", n)
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
