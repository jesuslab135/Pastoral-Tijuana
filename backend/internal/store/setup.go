package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrUsersExist means the one-time setup has already been run.
var ErrUsersExist = errors.New("users already exist")

// CreateInitialParroco creates the first parish user. The guard lives in the
// INSERT itself (WHERE NOT EXISTS), so two concurrent setup runs cannot both
// succeed and hand out two admin passwords.
func CreateInitialParroco(ctx context.Context, pool *pgxpool.Pool, email, passwordHash string) error {
	tag, err := pool.Exec(ctx,
		`INSERT INTO users (id, email, password_hash, display_name, role, is_active)
		 SELECT $1, $2, $3, $4, 'parroco', true
		 WHERE NOT EXISTS (SELECT 1 FROM users)`,
		uuid.New(), email, passwordHash, "Párroco")
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrUsersExist
	}
	return nil
}
