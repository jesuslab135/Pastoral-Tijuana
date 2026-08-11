package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store/testdb"
)

func TestCreateInitialParroco(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()

	if err := store.CreateInitialParroco(ctx, pool, "parroco@parroquia.mx", "hash"); err != nil {
		t.Fatalf("CreateInitialParroco: %v", err)
	}
	u, err := store.GetUserByEmail(ctx, pool, "parroco@parroquia.mx")
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	if u.Role != "parroco" || !u.IsActive || u.PasswordHash != "hash" {
		t.Errorf("unexpected initial user: %+v", u)
	}

	// Running setup again must not create a second admin.
	err = store.CreateInitialParroco(ctx, pool, "otro@parroquia.mx", "hash2")
	if !errors.Is(err, store.ErrUsersExist) {
		t.Errorf("expected ErrUsersExist, got %v", err)
	}
	n, err := store.CountUsers(ctx, pool)
	if err != nil || n != 1 {
		t.Errorf("CountUsers = %d, %v; setup must be one-shot", n, err)
	}
}
