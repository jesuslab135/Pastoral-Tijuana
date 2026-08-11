package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	DisplayName  string
	Role         string
	IsActive     bool
	CreatedAt    time.Time
}

const userCols = `id, email::text, coalesce(password_hash,''), display_name, role::text, is_active, created_at`

// userColsU is the same list qualified with the alias `u`, for queries that
// join another table carrying an `id` column of its own.
const userColsU = `u.id, u.email::text, coalesce(u.password_hash,''), u.display_name, u.role::text, u.is_active, u.created_at`

type scanner interface{ Scan(...any) error }

func scanUser(row scanner) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.DisplayName,
		&u.Role, &u.IsActive, &u.CreatedAt)
	return u, err
}

func CreateUser(ctx context.Context, pool *pgxpool.Pool, u User) error {
	_, err := pool.Exec(ctx,
		`INSERT INTO users (id, email, password_hash, display_name, role, is_active)
		 VALUES ($1,$2,nullif($3,''),$4,$5,$6)`,
		u.ID, u.Email, u.PasswordHash, u.DisplayName, u.Role, u.IsActive)
	return err
}

func GetUserByEmail(ctx context.Context, pool *pgxpool.Pool, email string) (User, error) {
	return scanUser(pool.QueryRow(ctx,
		`SELECT `+userCols+` FROM users WHERE email = $1`, email))
}

func GetUserByID(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) (User, error) {
	return scanUser(pool.QueryRow(ctx,
		`SELECT `+userCols+` FROM users WHERE id = $1`, id))
}

func ListUsers(ctx context.Context, pool *pgxpool.Pool) ([]User, error) {
	rows, err := pool.Query(ctx, `SELECT `+userCols+` FROM users ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func UpdateUser(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID, displayName, role string) error {
	_, err := pool.Exec(ctx,
		`UPDATE users SET display_name = $2, role = $3 WHERE id = $1`,
		id, displayName, role)
	return err
}

// SetUserActive flips is_active. Deactivating also revokes every live session
// of that user in the same transaction, so a fired account dies immediately.
func SetUserActive(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID, active bool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	// Rollback after a successful Commit is a no-op, so this is safe to ignore.
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`UPDATE users SET is_active = $2 WHERE id = $1`, id, active); err != nil {
		return err
	}
	if !active {
		if _, err := tx.Exec(ctx,
			`UPDATE sessions SET revoked_at = now()
			 WHERE user_id = $1 AND revoked_at IS NULL`, id); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func CountUsers(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	var n int
	err := pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&n)
	return n, err
}
