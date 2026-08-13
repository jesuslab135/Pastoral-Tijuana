// Package store provides database access: migrations, queries, and the pool.
package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

// Number 00005 is retired: it seeded placeholder channels, and its active
// email channel pointed at a domain the parish does not own — real SMTP would
// have mailed it. Long-lived dev databases still record version 5 in
// goose_db_version, so the NEXT migration must be 00006 or later; reusing
// 00005 would silently never run there.
//
//go:embed migrations/*.sql
var migrationsFS embed.FS

// migrationLockKey guards concurrent migrators. It is deliberately distinct
// from the key testdb uses to serialize test binaries, so a test process
// holding that lock never blocks a migration.
const migrationLockKey = 987654322

// Migrate applies all pending migrations. Safe to run repeatedly.
func Migrate(db *sql.DB) error {
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.Up(db, "migrations")
}

// MigrateLocked applies pending migrations while holding a Postgres advisory
// lock, so that API instances starting together (rolling deploy, restart
// loop, scaled service) migrate one at a time instead of colliding on
// CREATE EXTENSION or the seed exclusion constraint.
func MigrateLocked(ctx context.Context, db *sql.DB) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("migration lock connection: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", migrationLockKey); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	defer func() {
		// Best effort: closing the connection releases the lock anyway.
		_, _ = conn.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", migrationLockKey)
	}()

	return Migrate(db)
}
