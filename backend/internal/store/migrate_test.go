package store_test

import (
	"context"
	"testing"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store/testdb"
)

func TestMigrateCreatesSchema(t *testing.T) {
	pool := testdb.New(t)
	var n int
	err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM information_schema.tables
		 WHERE table_schema='public' AND table_name IN
		 ('liturgical_seasons','parish_groups','events','channels',
		  'outbox','broadcasts','users','sessions')`).Scan(&n)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if n != 8 {
		t.Errorf("expected 8 tables, got %d", n)
	}
}

func TestSeasonOverlapRejected(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	// Run inside a transaction that is always rolled back: testdb.New does
	// not truncate liturgical_seasons, so rows committed here would outlive
	// the test and permanently break every later run of it. A rollback also
	// covers an interrupted run, since Postgres discards the transaction
	// when the connection drops.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil {
			t.Logf("rollback: %v", err)
		}
	}()

	// 3000s: far future, cannot collide with seed data.
	if _, err := tx.Exec(ctx, `INSERT INTO liturgical_seasons (name,color,date_range)
		VALUES ('Prueba A','verde','[3000-01-01,3000-02-01)')`); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO liturgical_seasons (name,color,date_range)
		VALUES ('Prueba B','rojo','[3000-01-15,3000-03-01)')`); err == nil {
		t.Fatal("overlapping season should be rejected by EXCLUDE constraint")
	}
}
