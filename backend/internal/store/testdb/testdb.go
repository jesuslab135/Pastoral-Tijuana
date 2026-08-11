// Package testdb gives tests a migrated, clean database pool.
package testdb

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
)

const defaultURL = "postgres://pastoral:pastoral@localhost:5433/pastoral_test?sslmode=disable"

// New migrates the test database, truncates mutable tables (seed data in
// liturgical_seasons and parish_groups is preserved), and returns a pool.
func New(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = defaultURL
	}

	sqldb, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	defer sqldb.Close()
	if err := store.Migrate(sqldb); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	if _, err := sqldb.Exec(
		`TRUNCATE broadcasts, outbox, channels, events, sessions, users CASCADE`,
	); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}
