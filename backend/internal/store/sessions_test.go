package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store/testdb"
)

func TestSessionRoundTrip(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	u := store.User{ID: uuid.New(), Email: "a@p.mx", Role: "parroco", IsActive: true}
	if err := store.CreateUser(ctx, pool, u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	tok, err := store.CreateSession(ctx, pool, u.ID, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if len(tok) < 40 {
		t.Errorf("token looks too short: %d chars", len(tok))
	}

	// The raw token must never be what is stored.
	var stored int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM sessions WHERE token_hash = $1::bytea`, []byte(tok)).Scan(&stored); err != nil {
		t.Fatalf("probe: %v", err)
	}
	if stored != 0 {
		t.Error("sessions must store the hash, never the raw token")
	}

	got, err := store.ValidateSession(ctx, pool, tok)
	if err != nil {
		t.Fatalf("ValidateSession: %v", err)
	}
	if got.ID != u.ID {
		t.Errorf("wrong user: %+v", got)
	}

	if _, err := store.ValidateSession(ctx, pool, "no-existe"); err == nil {
		t.Error("unknown token must not validate")
	}

	if err := store.RevokeSession(ctx, pool, tok); err != nil {
		t.Fatalf("RevokeSession: %v", err)
	}
	if _, err := store.ValidateSession(ctx, pool, tok); err == nil {
		t.Error("revoked token must not validate")
	}
	if err := store.RevokeSession(ctx, pool, tok); err != nil {
		t.Errorf("revoke must be idempotent: %v", err)
	}
}

func TestExpiredSessionRejected(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	u := store.User{ID: uuid.New(), Email: "b@p.mx", Role: "secretaria", IsActive: true}
	if err := store.CreateUser(ctx, pool, u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	tok, err := store.CreateSession(ctx, pool, u.ID, "", "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE sessions SET expires_at = now() - interval '1 minute'`); err != nil {
		t.Fatalf("age session: %v", err)
	}
	if _, err := store.ValidateSession(ctx, pool, tok); err == nil {
		t.Error("expired token must not validate")
	}
}
