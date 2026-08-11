// Package testdb gives tests a migrated, clean database pool.
package testdb

import (
	"context"
	"database/sql"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
)

const defaultURL = "postgres://pastoral:pastoral@localhost:5433/pastoral_test?sslmode=disable"

// advisoryLockKey is an arbitrary, fixed key used to serialize every test
// binary process that touches the shared test database. Different Go
// packages under `go test ./...` run as separate OS processes in parallel
// by default; without this lock, one package's TRUNCATE/seed can corrupt
// another package's in-flight test against the same live Postgres instance.
const advisoryLockKey = 987654321

var (
	lockOnce sync.Once
	lockErr  error
)

// acquireProcessLock takes a PostgreSQL session-level advisory lock on a
// dedicated connection (not from a pool, so the lock stays on one stable
// session) the first time it is called in this process, and holds it for
// the lifetime of the process. Later calls in the same process reuse the
// already-held lock via sync.Once. The lock is intentionally never released
// explicitly -- releasing it between tests would defeat the serialization
// this exists to provide. The OS/Postgres releases it when the test binary
// process exits and the connection closes.
func acquireProcessLock(t *testing.T, url string) {
	t.Helper()
	lockOnce.Do(func() {
		conn, err := pgx.Connect(context.Background(), url)
		if err != nil {
			lockErr = err
			return
		}
		if _, err := conn.Exec(context.Background(), "SELECT pg_advisory_lock($1)", advisoryLockKey); err != nil {
			lockErr = err
			return
		}
	})
	if lockErr != nil {
		t.Fatalf("acquire test db advisory lock: %v", lockErr)
	}
}

// New migrates the test database, truncates mutable tables (seed data in
// liturgical_seasons and parish_groups is preserved), and returns a pool.
func New(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = defaultURL
	}

	acquireProcessLock(t, url)

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
