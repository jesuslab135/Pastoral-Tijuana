package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const SessionTTL = 30 * 24 * time.Hour

var ErrSessionInvalid = errors.New("session invalid")

func hashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

// CreateSession mints a random 256-bit token and stores only its sha256, so a
// database leak does not hand out live sessions. The raw token is returned
// once, for the cookie.
func CreateSession(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID, ip, userAgent string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	_, err := pool.Exec(ctx,
		`INSERT INTO sessions (id, user_id, token_hash, expires_at, ip, user_agent)
		 VALUES ($1,$2,$3,$4,nullif($5,'')::inet,nullif($6,''))`,
		uuid.New(), userID, hashToken(token), time.Now().Add(SessionTTL), ip, userAgent)
	if err != nil {
		return "", err
	}
	return token, nil
}

// ValidateSession resolves a raw token to its user. The u.is_active predicate
// backs up session revocation on deactivation: even a session created in the
// same instant cannot outlive the account.
func ValidateSession(ctx context.Context, pool *pgxpool.Pool, token string) (User, error) {
	row := pool.QueryRow(ctx,
		`SELECT `+userColsU+`
		 FROM sessions s JOIN users u ON u.id = s.user_id
		 WHERE s.token_hash = $1
		   AND s.revoked_at IS NULL
		   AND s.expires_at > now()
		   AND u.is_active`, hashToken(token))
	u, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrSessionInvalid
	}
	if err != nil {
		// Never report a database outage as a bad session: callers turn
		// ErrSessionInvalid into "log in again", which would hide the fault.
		return User{}, err
	}
	return u, nil
}

func RevokeSession(ctx context.Context, pool *pgxpool.Pool, token string) error {
	_, err := pool.Exec(ctx,
		`UPDATE sessions SET revoked_at = now()
		 WHERE token_hash = $1 AND revoked_at IS NULL`, hashToken(token))
	return err
}
