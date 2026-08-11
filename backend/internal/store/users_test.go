package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store/testdb"
)

func TestUserLifecycle(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	u := store.User{
		ID: uuid.New(), Email: "Parroco@Parroquia.MX", PasswordHash: "h",
		DisplayName: "Padre Ejemplo", Role: "parroco", IsActive: true,
	}
	if err := store.CreateUser(ctx, pool, u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	// citext: lookup must be case-insensitive.
	got, err := store.GetUserByEmail(ctx, pool, "parroco@parroquia.mx")
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	if got.ID != u.ID || got.Role != "parroco" || !got.IsActive {
		t.Errorf("unexpected user: %+v", got)
	}

	if err := store.UpdateUser(ctx, pool, u.ID, "Padre E.", "secretaria"); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
	got, err = store.GetUserByID(ctx, pool, u.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if got.DisplayName != "Padre E." || got.Role != "secretaria" {
		t.Errorf("update not applied: %+v", got)
	}

	if _, err := store.GetUserByEmail(ctx, pool, "nadie@x.mx"); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("expected pgx.ErrNoRows, got %v", err)
	}

	users, err := store.ListUsers(ctx, pool)
	if err != nil || len(users) != 1 {
		t.Errorf("ListUsers = %d users, %v", len(users), err)
	}

	n, err := store.CountUsers(ctx, pool)
	if err != nil || n != 1 {
		t.Errorf("CountUsers = %d, %v", n, err)
	}
}

func TestDeactivateRevokesSessions(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	u := store.User{ID: uuid.New(), Email: "s@p.mx", Role: "secretaria", IsActive: true}
	if err := store.CreateUser(ctx, pool, u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	tok, err := store.CreateSession(ctx, pool, u.ID, "", "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if _, err := store.ValidateSession(ctx, pool, tok); err != nil {
		t.Fatalf("session must be valid before deactivation: %v", err)
	}

	if err := store.SetUserActive(ctx, pool, u.ID, false); err != nil {
		t.Fatalf("SetUserActive: %v", err)
	}
	if _, err := store.ValidateSession(ctx, pool, tok); err == nil {
		t.Error("session must be invalid after the user is deactivated")
	}

	// Reactivating must not resurrect the revoked session.
	if err := store.SetUserActive(ctx, pool, u.ID, true); err != nil {
		t.Fatalf("SetUserActive(true): %v", err)
	}
	if _, err := store.ValidateSession(ctx, pool, tok); err == nil {
		t.Error("reactivation must not revive a revoked session")
	}
}
