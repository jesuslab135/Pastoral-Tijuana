# Auth & Admin API Implementation Plan (Plan 2 of 6)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The párroco and secretaría can log in (password or magic link), manage events, channels and users through `/api/v1/admin`, and publishing/unpublishing/cancelling an event writes the outbox rows Plan 3's difusión engine will consume — all in the same transaction as the mutation.

**Architecture:** Extends the existing `backend/` module. New packages: `internal/auth` (argon2id + magic-link tokens), `internal/mail` (Mailer), `internal/clientip`, `internal/ratelimit`. `internal/store` gains users/sessions/admin-events/channels/outbox functions (pgx, transactions via `pgxpool.Pool.Begin`). `internal/http` gains auth handlers, session middleware, and the `/api/v1/admin` subrouter. Redis (go-redis) enters for magic-link single-use only. `cmd/setup` creates the initial párroco.

**Tech Stack:** existing Go module (chi v5.3.1, pgx v5.7.6, goose v3.26.0, uuid v1.6.0) + `golang.org/x/crypto` (argon2) + `github.com/redis/go-redis/v9`.

## Global Constraints

- **Commit messages: plain conventional commits. NEVER add `Co-Authored-By`, or any mention of Claude/Anthropic/AI. (Explicit user requirement.)**
- All user-facing error messages in **Spanish**; error shape everywhere: `{"error":{"code":"...","message":"..."}}` via the existing `writeError`.
- Run all Go commands from `backend/`. Dev Postgres 5433 / Redis 6379 come from `docker-compose.dev.yml` (already exists).
- **Dependency rule (Plan 1 decision #3):** after every `go get`, check `head -5 go.mod`; if the `go` directive rose above `1.23`, downgrade the dependency instead of accepting the bump.
- **Every test that touches Postgres gets its pool from `testdb.New(t)`** (it holds the cross-process advisory lock). Tests that touch Redis read `TEST_REDIS_ADDR` (default `localhost:6379`) and must namespace keys with a `test:` prefix.
- Advisory lock keys in use: `987654321` (testdb), `987654322` (migrations). Do not reuse.
- Session cookie: `pc_session`, `HttpOnly`, `SameSite=Lax`, `Secure` unless request host is localhost; 30-day expiry; value is the raw token, DB stores `sha256(token)`.
- Outbox rows are written **in the same transaction** as the event mutation, never after commit.
- The public API contract from Plan 1 (JSON shapes, cache headers) must not change; `go test ./...` green after every task.
- The `project/` directory is design reference — never modify or serve it.

---

### Task 1: Config additions for auth, mail, Redis and proxy trust

**Files:**
- Modify: `backend/internal/config/config.go`
- Modify: `backend/internal/config/config_test.go`

**Interfaces:**
- Produces: new `Config` fields all later tasks read: `RedisAddr`, `TrustedProxy`, `AuthSecret`, `SMTPHost`, `SMTPPort`, `SMTPUser`, `SMTPPass`, `SMTPFrom`. No behavior change for existing fields.

- [ ] **Step 1: Extend the failing test**

Append to `config_test.go`:
```go
func TestLoadAuthDefaults(t *testing.T) {
	for _, k := range []string{"REDIS_ADDR", "TRUSTED_PROXY", "AUTH_SECRET",
		"SMTP_HOST", "SMTP_PORT", "SMTP_USER", "SMTP_PASS", "SMTP_FROM"} {
		t.Setenv(k, "")
	}
	c := Load()
	if c.RedisAddr != "localhost:6379" {
		t.Errorf("RedisAddr default wrong: %q", c.RedisAddr)
	}
	if c.TrustedProxy != "" {
		t.Errorf("TrustedProxy must default empty, got %q", c.TrustedProxy)
	}
	if c.AuthSecret != "dev-secret-change-me" {
		t.Errorf("AuthSecret default wrong: %q", c.AuthSecret)
	}
	if c.SMTPHost != "" || c.SMTPPort != "587" {
		t.Errorf("SMTP defaults wrong: host=%q port=%q", c.SMTPHost, c.SMTPPort)
	}
}
```

- [ ] **Step 2: Run to verify failure** — `go test ./internal/config/` → FAIL (unknown fields).

- [ ] **Step 3: Implement**

Add fields to `Config` and entries in `Load()`:
```go
	RedisAddr    string
	TrustedProxy string // CIDR of the reverse proxy; empty = trust nobody
	AuthSecret   string // HMAC key for magic-link tokens
	SMTPHost     string // empty = LogMailer (dev)
	SMTPPort     string
	SMTPUser     string
	SMTPPass     string
	SMTPFrom     string
```
```go
		RedisAddr:    getenv("REDIS_ADDR", "localhost:6379"),
		TrustedProxy: getenv("TRUSTED_PROXY", ""),
		AuthSecret:   getenv("AUTH_SECRET", "dev-secret-change-me"),
		SMTPHost:     getenv("SMTP_HOST", ""),
		SMTPPort:     getenv("SMTP_PORT", "587"),
		SMTPUser:     getenv("SMTP_USER", ""),
		SMTPPass:     getenv("SMTP_PASS", ""),
		SMTPFrom:     getenv("SMTP_FROM", ""),
```

- [ ] **Step 4: Run to verify pass** — `go test ./internal/config/ -v` → PASS.

- [ ] **Step 5: Commit** — `git add internal/config/ && git commit -m "feat: add auth, mail, redis and proxy config"`

---

### Task 2: Password hashing (argon2id)

**Files:**
- Create: `backend/internal/auth/password.go`
- Test: `backend/internal/auth/password_test.go`

**Interfaces:**
- Produces:
  ```go
  // package auth
  func HashPassword(plain string) (string, error)      // PHC string: $argon2id$v=19$m=65536,t=1,p=4$<salt>$<hash>
  func VerifyPassword(plain, encoded string) bool      // constant-time compare; false on any parse error
  ```

- [ ] **Step 1: Write the failing test**

`backend/internal/auth/password_test.go`:
```go
package auth

import (
	"strings"
	"testing"
)

func TestHashAndVerify(t *testing.T) {
	h, err := HashPassword("contraseña-segura")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$") {
		t.Errorf("not a PHC argon2id string: %q", h)
	}
	if !VerifyPassword("contraseña-segura", h) {
		t.Error("correct password must verify")
	}
	if VerifyPassword("otra-cosa", h) {
		t.Error("wrong password must not verify")
	}
}

func TestHashesAreSalted(t *testing.T) {
	a, _ := HashPassword("x")
	b, _ := HashPassword("x")
	if a == b {
		t.Error("two hashes of the same password must differ (random salt)")
	}
}

func TestVerifyRejectsGarbage(t *testing.T) {
	for _, bad := range []string{"", "plaintext", "$argon2id$v=19$m=65536,t=1,p=4$!!$??", "$argon2i$v=19$m=1,t=1,p=1$AA$AA"} {
		if VerifyPassword("x", bad) {
			t.Errorf("VerifyPassword must reject %q", bad)
		}
	}
}
```

- [ ] **Step 2: Run to verify failure** — `go test ./internal/auth/` → FAIL (undefined).

- [ ] **Step 3: Implement**

`backend/internal/auth/password.go`:
```go
// Package auth implements password hashing and magic-link tokens.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonTime    = 1
	argonMemory  = 64 * 1024 // KiB
	argonThreads = 4
	argonKeyLen  = 32
	saltLen      = 16
)

func HashPassword(plain string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(plain), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

func VerifyPassword(plain, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false
	}
	var m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(plain), salt, t, m, p, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}
```

- [ ] **Step 4: Fetch dep, verify pass, check go directive**

```bash
go get golang.org/x/crypto
go mod tidy
head -5 go.mod   # go directive must still be 1.23.x — downgrade the dep if not
go test ./internal/auth/ -v
```

- [ ] **Step 5: Commit** — `git add internal/auth/ go.mod go.sum && git commit -m "feat: add argon2id password hashing"`

---

### Task 3: Users store (with session-revoking deactivation)

**Files:**
- Create: `backend/internal/store/users.go`
- Test: `backend/internal/store/users_test.go`

**Interfaces:**
- Produces:
  ```go
  type User struct {
      ID           uuid.UUID
      Email        string
      PasswordHash string // empty when unset (magic-link-only user)
      DisplayName  string
      Role         string // "parroco" | "secretaria"
      IsActive     bool
      CreatedAt    time.Time
  }
  func CreateUser(ctx, pool, u User) error
  func GetUserByEmail(ctx, pool, email string) (User, error)  // pgx.ErrNoRows when absent
  func GetUserByID(ctx, pool, id uuid.UUID) (User, error)
  func ListUsers(ctx, pool) ([]User, error)                    // ordered by created_at
  func UpdateUser(ctx, pool, id uuid.UUID, displayName, role string) error
  // SetUserActive flips is_active; deactivation revokes every session
  // of that user IN THE SAME TRANSACTION (spec §8).
  func SetUserActive(ctx, pool, id uuid.UUID, active bool) error
  func CountUsers(ctx, pool) (int, error)                      // cmd/setup guard
  ```

- [ ] **Step 1: Write the failing test**

`backend/internal/store/users_test.go`:
```go
package store_test

import (
	"context"
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
	got, _ = store.GetUserByID(ctx, pool, u.ID)
	if got.DisplayName != "Padre E." || got.Role != "secretaria" {
		t.Errorf("update not applied: %+v", got)
	}
	if _, err := store.GetUserByEmail(ctx, pool, "nadie@x.mx"); err != pgx.ErrNoRows {
		t.Errorf("expected pgx.ErrNoRows, got %v", err)
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
	if err := store.SetUserActive(ctx, pool, u.ID, false); err != nil {
		t.Fatalf("SetUserActive: %v", err)
	}
	if _, err := store.ValidateSession(ctx, pool, tok); err == nil {
		t.Error("session must be invalid after the user is deactivated")
	}
}
```
(The session functions arrive in Task 4 — this test file compiles only after both tasks, which is why Tasks 3 and 4 share one commit gate: **run the full store test suite only at the end of Task 4**. Implement Task 3's functions first, keep `users_test.go` unstaged until Task 4 passes, or write both tasks then test. Recommended order for a worker: Task 3 impl → Task 4 impl+tests → run everything → two commits in sequence.)

- [ ] **Step 2: Implement**

`backend/internal/store/users.go`:
```go
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

func CreateUser(ctx context.Context, pool *pgxpool.Pool, u User) error {
	_, err := pool.Exec(ctx,
		`INSERT INTO users (id, email, password_hash, display_name, role, is_active)
		 VALUES ($1,$2,nullif($3,''),$4,$5,$6)`,
		u.ID, u.Email, u.PasswordHash, u.DisplayName, u.Role, u.IsActive)
	return err
}

func scanUser(row interface{ Scan(...any) error }) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.DisplayName,
		&u.Role, &u.IsActive, &u.CreatedAt)
	return u, err
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
	defer tx.Rollback(ctx)
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
```

- [ ] **Step 3: Commit (after Task 4's tests pass)** — `git add internal/store/users.go internal/store/users_test.go && git commit -m "feat: add users store with session-revoking deactivation"`

---

### Task 4: Sessions store

**Files:**
- Create: `backend/internal/store/sessions.go`
- Test: `backend/internal/store/sessions_test.go`

**Interfaces:**
- Produces:
  ```go
  const SessionTTL = 30 * 24 * time.Hour
  // CreateSession mints a random 256-bit token, stores sha256(token), returns the raw token.
  func CreateSession(ctx, pool, userID uuid.UUID, ip, userAgent string) (string, error)
  // ValidateSession returns the user for a raw token; error if the token is
  // unknown, expired, revoked, or the user is inactive.
  func ValidateSession(ctx, pool, token string) (User, error)
  func RevokeSession(ctx, pool, token string) error // idempotent
  ```

- [ ] **Step 1: Write the failing test**

`backend/internal/store/sessions_test.go`:
```go
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
```
- [ ] **Step 2: Run to verify failure** — `go test ./internal/store/ -run TestSession` → FAIL (undefined).

- [ ] **Step 3: Implement**

`backend/internal/store/sessions.go`:
```go
package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const SessionTTL = 30 * 24 * time.Hour

var ErrSessionInvalid = errors.New("session invalid")

func hashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

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

func ValidateSession(ctx context.Context, pool *pgxpool.Pool, token string) (User, error) {
	row := pool.QueryRow(ctx,
		`SELECT `+userCols+`
		 FROM sessions s JOIN users u ON u.id = s.user_id
		 WHERE s.token_hash = $1
		   AND s.revoked_at IS NULL
		   AND s.expires_at > now()
		   AND u.is_active`, hashToken(token))
	u, err := scanUser(row)
	if err != nil {
		return User{}, ErrSessionInvalid
	}
	return u, nil
}

func RevokeSession(ctx context.Context, pool *pgxpool.Pool, token string) error {
	_, err := pool.Exec(ctx,
		`UPDATE sessions SET revoked_at = now()
		 WHERE token_hash = $1 AND revoked_at IS NULL`, hashToken(token))
	return err
}
```
Note the `u.is_active` predicate: `TestDeactivateRevokesSessions` (Task 3) passes through both the revocation and this belt-and-suspenders check.

- [ ] **Step 4: Run the full store suite** — `go test ./internal/store/ -count=1` → PASS (Tasks 3+4 together).

- [ ] **Step 5: Commit both tasks in order**

```bash
git add internal/store/users.go internal/store/users_test.go
git commit -m "feat: add users store with session-revoking deactivation"
git add internal/store/sessions.go internal/store/sessions_test.go
git commit -m "feat: add opaque-token sessions store"
```

---

### Task 5: Client IP resolution + login rate limiter

**Files:**
- Create: `backend/internal/clientip/clientip.go`
- Test: `backend/internal/clientip/clientip_test.go`
- Create: `backend/internal/ratelimit/ratelimit.go`
- Test: `backend/internal/ratelimit/ratelimit_test.go`

**Interfaces:**
- Produces:
  ```go
  // package clientip — the RealIP replacement mandated by Plan 1 decision #1.
  func NewResolver(trustedProxyCIDR string) (*Resolver, error) // "" = never trust XFF
  func (r *Resolver) FromRequest(req *http.Request) string
  // package ratelimit — fixed-window token bucket, in-memory (single instance).
  func New(limit int, window time.Duration) *Limiter
  func (l *Limiter) Allow(key string) bool
  ```
- Rule: `FromRequest` returns the last hop of `X-Forwarded-For` **only when** the socket peer (`req.RemoteAddr`) is inside the trusted CIDR; otherwise the socket peer's IP. Caddy *appends* the client to XFF, so the last hop is the one Caddy itself wrote.

- [ ] **Step 1: Write the failing tests**

`backend/internal/clientip/clientip_test.go`:
```go
package clientip

import (
	"net/http/httptest"
	"testing"
)

func TestUntrustedPeerIgnoresXFF(t *testing.T) {
	r, err := NewResolver("")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "203.0.113.7:9999"
	req.Header.Set("X-Forwarded-For", "10.0.0.1, 198.51.100.2")
	if got := r.FromRequest(req); got != "203.0.113.7" {
		t.Errorf("untrusted peer: got %q, want socket ip", got)
	}
}

func TestTrustedProxyUsesLastXFFHop(t *testing.T) {
	r, err := NewResolver("172.18.0.0/16")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "172.18.0.5:4321" // Caddy's container address
	// Client forged the first hop; Caddy appended the real one last.
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 198.51.100.2")
	if got := r.FromRequest(req); got != "198.51.100.2" {
		t.Errorf("trusted proxy: got %q, want last XFF hop", got)
	}
}

func TestTrustedProxyNoXFFFallsBack(t *testing.T) {
	r, _ := NewResolver("172.18.0.0/16")
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "172.18.0.5:4321"
	if got := r.FromRequest(req); got != "172.18.0.5" {
		t.Errorf("no XFF: got %q, want socket ip", got)
	}
}

func TestBadCIDRRejected(t *testing.T) {
	if _, err := NewResolver("not-a-cidr"); err == nil {
		t.Error("invalid CIDR must be rejected at construction")
	}
}
```

`backend/internal/ratelimit/ratelimit_test.go`:
```go
package ratelimit

import (
	"testing"
	"time"
)

func TestLimit(t *testing.T) {
	l := New(3, time.Hour)
	for i := 0; i < 3; i++ {
		if !l.Allow("1.2.3.4") {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
	if l.Allow("1.2.3.4") {
		t.Error("4th request in window must be denied")
	}
	if !l.Allow("5.6.7.8") {
		t.Error("different key must not be affected")
	}
}

func TestWindowResets(t *testing.T) {
	l := New(1, 30*time.Millisecond)
	if !l.Allow("k") {
		t.Fatal("first must pass")
	}
	if l.Allow("k") {
		t.Fatal("second must be denied")
	}
	time.Sleep(40 * time.Millisecond)
	if !l.Allow("k") {
		t.Error("after the window the key must be allowed again")
	}
}
```

- [ ] **Step 2: Run to verify failure**, then **Step 3: Implement**

`backend/internal/clientip/clientip.go`:
```go
// Package clientip resolves the real client address behind an optional
// trusted reverse proxy. chi's RealIP middleware was removed on purpose
// (it trusts X-Forwarded-For unconditionally); nothing else in the app may
// read that header directly.
package clientip

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

type Resolver struct {
	trusted *netip.Prefix // nil = trust nobody
}

func NewResolver(trustedProxyCIDR string) (*Resolver, error) {
	if trustedProxyCIDR == "" {
		return &Resolver{}, nil
	}
	p, err := netip.ParsePrefix(trustedProxyCIDR)
	if err != nil {
		return nil, err
	}
	return &Resolver{trusted: &p}, nil
}

func (r *Resolver) FromRequest(req *http.Request) string {
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		host = req.RemoteAddr
	}
	if r.trusted == nil {
		return host
	}
	peer, err := netip.ParseAddr(host)
	if err != nil || !r.trusted.Contains(peer) {
		return host
	}
	xff := req.Header.Get("X-Forwarded-For")
	if xff == "" {
		return host
	}
	hops := strings.Split(xff, ",")
	last := strings.TrimSpace(hops[len(hops)-1])
	if _, err := netip.ParseAddr(last); err != nil {
		return host
	}
	return last
}
```

`backend/internal/ratelimit/ratelimit.go`:
```go
// Package ratelimit is a fixed-window in-memory limiter, sufficient for a
// single-instance deployment (spec §8).
package ratelimit

import (
	"sync"
	"time"
)

type Limiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	hits   map[string]entry
}

type entry struct {
	count int
	start time.Time
}

func New(limit int, window time.Duration) *Limiter {
	return &Limiter{limit: limit, window: window, hits: make(map[string]entry)}
}

func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	e, ok := l.hits[key]
	if !ok || now.Sub(e.start) >= l.window {
		// Window rollover doubles as cleanup for the expired key; sweep the
		// rest occasionally so idle keys don't accumulate forever.
		if len(l.hits) > 4096 {
			for k, v := range l.hits {
				if now.Sub(v.start) >= l.window {
					delete(l.hits, k)
				}
			}
		}
		l.hits[key] = entry{count: 1, start: now}
		return true
	}
	if e.count >= l.limit {
		return false
	}
	e.count++
	l.hits[key] = e
	return true
}
```

- [ ] **Step 4: Verify** — `go test ./internal/clientip/ ./internal/ratelimit/ -v` → PASS.

- [ ] **Step 5: Commit** — `git add internal/clientip/ internal/ratelimit/ && git commit -m "feat: add trusted-proxy client ip resolver and login rate limiter"`

---

### Task 6: Session middleware + login/logout/me

**Files:**
- Create: `backend/internal/http/auth.go`
- Modify: `backend/internal/http/server.go`
- Test: `backend/internal/http/auth_test.go`

**Interfaces:**
- Produces routes `POST /api/v1/auth/login`, `POST /api/v1/auth/logout`, `GET /api/v1/auth/me`; middleware `requireSession` (loads the user into the request context) and `requireParroco`; an `/api/v1/admin` subrouter behind `requireSession` that later tasks attach to. Cookie per Global Constraints.
- Login request `{"email":"...","password":"..."}`; 200 → `{"user":{"id","email","display_name","role"}}` + cookie. Bad credentials → 401 `{"error":{"code":"credenciales_invalidas","message":"Correo o contraseña incorrectos."}}` (identical for unknown email vs wrong password — no enumeration). Rate limited 5/min/IP → 429 `rate_limited`.
- `NewRouter` signature stays `NewRouter(pool, cfg)`; it builds the resolver and limiter internally (resolver error → panic at construction, it is a config bug caught by main's fail-fast).

- [ ] **Step 1: Write the failing test**

`backend/internal/http/auth_test.go`:
```go
package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/auth"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/config"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store/testdb"
)

func seedParroco(t *testing.T, pool *pgxpool.Pool, email, password string) uuid.UUID {
	t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	if err := store.CreateUser(context.Background(), pool, store.User{
		ID: id, Email: email, PasswordHash: hash,
		DisplayName: "Padre Test", Role: "parroco", IsActive: true,
	}); err != nil {
		t.Fatal(err)
	}
	return id
}

func doLogin(t *testing.T, r http.Handler, email, password string) *httptest.ResponseRecorder {
	t.Helper()
	body := strings.NewReader(`{"email":"` + email + `","password":"` + password + `"}`)
	req := httptest.NewRequest("POST", "/api/v1/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestLoginLogoutMe(t *testing.T) {
	pool := testdb.New(t)
	seedParroco(t, pool, "p@x.mx", "secreta123")
	r := NewRouter(pool, config.Load())

	rec := doLogin(t, r, "p@x.mx", "secreta123")
	if rec.Code != 200 {
		t.Fatalf("login: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var cookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "pc_session" {
			cookie = c
		}
	}
	if cookie == nil || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("missing or misconfigured pc_session cookie: %+v", cookie)
	}

	req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	req.AddCookie(cookie)
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req)
	if rec2.Code != 200 {
		t.Fatalf("me: expected 200, got %d", rec2.Code)
	}
	var me struct {
		User struct{ Role string } `json:"user"`
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &me); err != nil || me.User.Role != "parroco" {
		t.Errorf("me body: %s", rec2.Body.String())
	}

	req = httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
	req.AddCookie(cookie)
	rec3 := httptest.NewRecorder()
	r.ServeHTTP(rec3, req)
	if rec3.Code != 204 {
		t.Fatalf("logout: expected 204, got %d", rec3.Code)
	}
	req = httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	req.AddCookie(cookie)
	rec4 := httptest.NewRecorder()
	r.ServeHTTP(rec4, req)
	if rec4.Code != 401 {
		t.Errorf("me after logout: expected 401, got %d", rec4.Code)
	}
}

func TestLoginRejectsBadCredentialsUniformly(t *testing.T) {
	pool := testdb.New(t)
	seedParroco(t, pool, "p@x.mx", "secreta123")
	r := NewRouter(pool, config.Load())

	a := doLogin(t, r, "p@x.mx", "equivocada")
	b := doLogin(t, r, "no-existe@x.mx", "loquesea")
	if a.Code != 401 || b.Code != 401 {
		t.Fatalf("expected 401/401, got %d/%d", a.Code, b.Code)
	}
	if a.Body.String() != b.Body.String() {
		t.Error("wrong-password and unknown-email responses must be identical (no enumeration)")
	}
}

func TestLoginRateLimited(t *testing.T) {
	pool := testdb.New(t)
	r := NewRouter(pool, config.Load())
	var last int
	for i := 0; i < 6; i++ {
		last = doLogin(t, r, "x@x.mx", "nope").Code
	}
	if last != 429 {
		t.Errorf("6th attempt from one IP: expected 429, got %d", last)
	}
}

func TestAdminRequiresSession(t *testing.T) {
	pool := testdb.New(t)
	r := NewRouter(pool, config.Load())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/admin/events", nil))
	if rec.Code != 401 {
		t.Errorf("admin without cookie: expected 401, got %d", rec.Code)
	}
}
```
(Add `"context"` and `"github.com/jackc/pgx/v5/pgxpool"` to the imports; `httptest.NewRequest` sets `RemoteAddr` to `192.0.2.1:1234`, so all six rate-limit attempts share one IP. Note each `NewRouter` call builds a fresh limiter, so tests don't bleed into each other.)

- [ ] **Step 2: Run to verify failure** — 404s.

- [ ] **Step 3: Implement**

`backend/internal/http/auth.go` — the essential pieces:
```go
package httpapi

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/auth"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/clientip"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/ratelimit"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
)

type ctxKey int

const userKey ctxKey = 0

const sessionCookie = "pc_session"

func setSessionCookie(w http.ResponseWriter, r *http.Request, token string, ttl time.Duration) {
	host := r.Host
	if h, _, err := net.SplitHostPort(r.Host); err == nil {
		host = h
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/",
		MaxAge: int(ttl.Seconds()), HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   host != "localhost" && host != "127.0.0.1",
	})
}

func loginHandler(pool *pgxpool.Pool, ips *clientip.Resolver, limiter *ratelimit.Limiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow(ips.FromRequest(r)) {
			writeError(w, http.StatusTooManyRequests, "rate_limited",
				"Demasiados intentos. Espera un minuto.")
			return
		}
		var in struct{ Email, Password string }
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "Cuerpo JSON inválido.")
			return
		}
		fail := func() {
			writeError(w, http.StatusUnauthorized, "credenciales_invalidas",
				"Correo o contraseña incorrectos.")
		}
		u, err := store.GetUserByEmail(r.Context(), pool, strings.TrimSpace(in.Email))
		if err != nil || !u.IsActive || u.PasswordHash == "" ||
			!auth.VerifyPassword(in.Password, u.PasswordHash) {
			fail()
			return
		}
		token, err := store.CreateSession(r.Context(), pool, u.ID,
			ips.FromRequest(r), r.UserAgent())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "No se pudo iniciar sesión.")
			return
		}
		setSessionCookie(w, r, token, store.SessionTTL)
		writeJSON(w, http.StatusOK, map[string]any{"user": userJSON(u)})
	}
}

func userJSON(u store.User) map[string]any {
	return map[string]any{
		"id": u.ID, "email": u.Email,
		"display_name": u.DisplayName, "role": u.Role, "is_active": u.IsActive,
	}
}

func logoutHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(sessionCookie); err == nil {
			_ = store.RevokeSession(r.Context(), pool, c.Value)
		}
		// -time.Second → MaxAge -1, which tells the browser to delete the
		// cookie. (A bare -1 would be -1ns and truncate to MaxAge 0 = no-op.)
		setSessionCookie(w, r, "", -time.Second)
		w.WriteHeader(http.StatusNoContent)
	}
}

func requireSession(pool *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, err := r.Cookie(sessionCookie)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "no_autenticado", "Inicia sesión.")
				return
			}
			u, err := store.ValidateSession(r.Context(), pool, c.Value)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "no_autenticado", "Inicia sesión.")
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey, u)))
		})
	}
}

func currentUser(r *http.Request) store.User {
	u, _ := r.Context().Value(userKey).(store.User)
	return u
}

func requireParroco(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if currentUser(r).Role != "parroco" {
			writeError(w, http.StatusForbidden, "prohibido", "Solo el párroco puede hacer esto.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func meHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"user": userJSON(currentUser(r))})
	}
}
```

In `server.go`, inside `NewRouter` (after the public `/api/v1` block):
```go
	ips, err := clientip.NewResolver(cfg.TrustedProxy)
	if err != nil {
		panic("TRUSTED_PROXY inválido: " + err.Error()) // config bug; main fails fast anyway
	}
	loginLimiter := ratelimit.New(5, time.Minute)

	r.Route("/api/v1/auth", func(a chi.Router) {
		a.Post("/login", loginHandler(pool, ips, loginLimiter))
		a.Post("/logout", logoutHandler(pool))
		a.With(requireSession(pool)).Get("/me", meHandler())
	})

	r.Route("/api/v1/admin", func(admin chi.Router) {
		admin.Use(requireSession(pool))
		// Task 11+ attach handlers here. A placeholder keeps the subtree alive:
		admin.Get("/events", notImplementedYet) // replaced in Task 11
	})
```
`notImplementedYet` returns 501 with the standard error shape; it is deleted in Task 11. Move the `TRUSTED_PROXY` validation into `main.go`'s fail-fast block too (alongside `PARISH_TZ`).

- [ ] **Step 4: Verify** — `go test ./internal/http/ -count=1 -v` → PASS (including all Plan 1 tests untouched).

- [ ] **Step 5: Commit** — `git add internal/http/ cmd/api/main.go && git commit -m "feat: add session auth with login, logout, me and admin guard"`

---

### Task 7: Mailer

**Files:**
- Create: `backend/internal/mail/mail.go`
- Test: `backend/internal/mail/mail_test.go`

**Interfaces:**
- Produces:
  ```go
  type Mailer interface {
      Send(ctx context.Context, to, subject, textBody string) error
  }
  func New(cfg config.Config) Mailer // SMTPMailer when SMTPHost set, else LogMailer
  type LogMailer struct{ Sink *log.Logger } // logs to sink (default log.Default()); never errors
  type SMTPMailer struct{ ... }             // net/smtp + STARTTLS, plain auth
  ```
- The SMTP path is exercised only by construction tests (no network); its correctness is verified manually in the runbook. `LogMailer` must log `to`, `subject` and the full body so magic links are copy-pasteable.

- [ ] **Step 1: Failing test**

`backend/internal/mail/mail_test.go`:
```go
package mail

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/config"
)

func TestNewSelectsLogMailerWithoutSMTP(t *testing.T) {
	m := New(config.Config{})
	if _, ok := m.(*LogMailer); !ok {
		t.Fatalf("expected LogMailer, got %T", m)
	}
}

func TestNewSelectsSMTPMailerWithHost(t *testing.T) {
	m := New(config.Config{SMTPHost: "smtp.ionos.mx", SMTPPort: "587",
		SMTPUser: "u", SMTPPass: "p", SMTPFrom: "cal@parroquia.mx"})
	if _, ok := m.(*SMTPMailer); !ok {
		t.Fatalf("expected SMTPMailer, got %T", m)
	}
}

func TestLogMailerLogsEverything(t *testing.T) {
	var buf bytes.Buffer
	m := &LogMailer{Sink: log.New(&buf, "", 0)}
	if err := m.Send(context.Background(), "p@x.mx", "Tu enlace",
		"https://pastoral.jesuslab135.com/verify?token=abc"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"p@x.mx", "Tu enlace", "token=abc"} {
		if !strings.Contains(out, want) {
			t.Errorf("log output missing %q:\n%s", want, out)
		}
	}
}
```

- [ ] **Step 2: verify failure**, **Step 3: Implement**

`backend/internal/mail/mail.go`:
```go
// Package mail delivers transactional email. Plan 3's difusión SMTPSender
// reuses the same SMTP_* env config but is a separate component.
package mail

import (
	"context"
	"fmt"
	"log"
	"net/smtp"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/config"
)

type Mailer interface {
	Send(ctx context.Context, to, subject, textBody string) error
}

func New(cfg config.Config) Mailer {
	if cfg.SMTPHost == "" {
		return &LogMailer{}
	}
	return &SMTPMailer{
		addr: cfg.SMTPHost + ":" + cfg.SMTPPort,
		auth: smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPHost),
		from: cfg.SMTPFrom,
	}
}

// LogMailer prints the message instead of sending it — the dev default, so
// magic links are copy-pasteable from the console.
type LogMailer struct{ Sink *log.Logger }

func (m *LogMailer) Send(_ context.Context, to, subject, textBody string) error {
	sink := m.Sink
	if sink == nil {
		sink = log.Default()
	}
	sink.Printf("MAIL (no SMTP configured)\nTo: %s\nSubject: %s\n\n%s\n", to, subject, textBody)
	return nil
}

type SMTPMailer struct {
	addr string
	auth smtp.Auth
	from string
}

func (m *SMTPMailer) Send(_ context.Context, to, subject, textBody string) error {
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s\r\n",
		m.from, to, subject, textBody)
	// smtp.SendMail negotiates STARTTLS when the server offers it.
	return smtp.SendMail(m.addr, m.auth, m.from, []string{to}, []byte(msg))
}
```

- [ ] **Step 4: Verify** — `go test ./internal/mail/ -v` → PASS.
- [ ] **Step 5: Commit** — `git add internal/mail/ && git commit -m "feat: add smtp and log mailers"`

---

### Task 8: Magic link (issue, email, verify via Redis single-use) + CI Redis service

**Files:**
- Create: `backend/internal/auth/magiclink.go`
- Test: `backend/internal/auth/magiclink_test.go`
- Modify: `backend/internal/http/auth.go` + `server.go` (two routes)
- Test: `backend/internal/http/magiclink_test.go`
- Modify: `.github/workflows/ci.yml` (add `redis:7` service) — **same commit**, or CI breaks.

**Interfaces:**
- Produces:
  ```go
  // package auth — stateless HMAC token: base64url(payload) . base64url(hmac)
  // payload = userID|jti|expiresUnix
  func IssueMagicToken(secret string, userID uuid.UUID, now time.Time) (token, jti string, err error) // 15-min TTL
  func ParseMagicToken(secret, token string, now time.Time) (userID uuid.UUID, jti string, err error)
  ```
- Routes: `POST /api/v1/auth/magic-link` `{"email":"..."}` → always 200 `{"message":"Si el correo existe, enviamos un enlace."}` (no enumeration; rate-limited with the same limiter). `GET /api/v1/auth/magic-link/verify?token=...` → on success creates a normal session, sets the cookie, 200 `{"user":{...}}`; failure → 401 `enlace_invalido` with Spanish message. Single use: `SET test-independent key magic:jti:<jti> NX EX 900` in Redis; if the key already exists the token is spent. Redis down → 401 + logged error (fails closed).
- The emailed link is `cfg.PublicBaseURL + "/api/v1/auth/magic-link/verify?token=..."` (the admin frontend later wraps this; the API link works standalone).
- `NewRouter` grows a third parameter: `NewRouter(pool *pgxpool.Pool, rdb *redis.Client, cfg config.Config)`. Update `main.go` (construct client from `cfg.RedisAddr`) and every test caller (helper `newTestRouter(t, pool)` in `internal/http` that builds a client from `TEST_REDIS_ADDR`, default `localhost:6379`).

- [ ] **Step 1: Token unit tests** (`internal/auth/magiclink_test.go`): round-trip; expired (now+16min) rejected; tampered payload rejected; token signed with a different secret rejected; two issues produce distinct jtis. Full listing left to the worker — assert with `errors.Is` on exported `ErrTokenInvalid`/`ErrTokenExpired`.

- [ ] **Step 2: Implement the token** (`internal/auth/magiclink.go`): HMAC-SHA256 over the payload with the secret; constant-time MAC compare; 15-minute expiry constant `MagicLinkTTL`.

- [ ] **Step 3: Handler tests** (`internal/http/magiclink_test.go`):
  - request link for an existing user with a `LogMailer` sink buffer → 200, buffer contains `token=`; extract the token from the buffer, hit verify → 200 + `pc_session` cookie; hit verify again with the same token → 401 (spent jti).
  - request link for an unknown email → identical 200 body, mailer buffer empty.
  - verify with garbage token → 401.
  - Requires the mailer to be injectable: `NewRouter` takes `mail.Mailer` too — final signature `NewRouter(pool, rdb, mailer, cfg)`. Update all callers; `main.go` passes `mail.New(cfg)`.

- [ ] **Step 4: Implement handlers**; key verify fragment:
```go
		userID, jti, err := auth.ParseMagicToken(cfg.AuthSecret, token, time.Now())
		if err != nil { /* 401 enlace_invalido */ }
		ok, err := rdb.SetNX(r.Context(), "magic:jti:"+jti, "1", auth.MagicLinkTTL).Result()
		if err != nil || !ok { /* 401 enlace_invalido (Redis down ⇒ fails closed, log err) */ }
		u, err := store.GetUserByID(r.Context(), pool, userID)
		if err != nil || !u.IsActive { /* 401 */ }
		// then: CreateSession + cookie + 200, same as password login
```

- [ ] **Step 5: CI** — in `.github/workflows/ci.yml` add under `services:`:
```yaml
      redis:
        image: redis:7
        ports:
          - 6379:6379
```
(no health options needed; redis starts in ms). Tests read `TEST_REDIS_ADDR` defaulting to `localhost:6379`, which matches both CI and dev compose.

- [ ] **Step 6: Verify** — `go get github.com/redis/go-redis/v9 && go mod tidy && head -5 go.mod` (directive check), then `go test ./... -count=1` → PASS with dev compose up.

- [ ] **Step 7: Commit** — `git add internal/auth/ internal/http/ internal/mail/ cmd/api/main.go .github/workflows/ci.yml go.mod go.sum && git commit -m "feat: add magic link login with redis single-use and ci redis service"`

---

### Task 9: cmd/setup — initial párroco

**Files:**
- Create: `backend/internal/store/setup.go`
- Test: `backend/internal/store/setup_test.go`
- Create: `backend/cmd/setup/main.go`

**Interfaces:**
- Produces: `store.CreateInitialParroco(ctx, pool, email, passwordHash string) error` — returns `store.ErrUsersExist` if `CountUsers > 0` (checked inside one transaction with the insert, `SELECT ... FOR UPDATE` not needed: use `INSERT ... WHERE NOT EXISTS (SELECT 1 FROM users)` and inspect rows affected). `cmd/setup` reads `SETUP_EMAIL` (required) and `DATABASE_URL`, generates a 16-char random password, hashes it, calls the store, prints the password **once** to stdout, exits 1 with a Spanish message if users already exist.

- [ ] **Step 1: Failing store test** — create parroco on empty users table succeeds and role is `parroco`; second call returns `ErrUsersExist`; test via `testdb.New` (users table is truncated by the helper, so the "empty" precondition holds).
- [ ] **Step 2: Implement store function.**
- [ ] **Step 3: `cmd/setup/main.go`** — thin main: config, pool, `auth.HashPassword`, print `Usuario creado: <email>\nContraseña (guárdala ahora, no se mostrará de nuevo): <password>`.
- [ ] **Step 4: Verify** — store test green; `go run ./cmd/setup` against dev DB creates the user (then delete it or run against a scratch DB; note `pastoral` dev DB, not `pastoral_test`).
- [ ] **Step 5: Commit** — `git commit -m "feat: add one-time setup command for the initial parroco"`

---

### Task 10: Admin events store — CRUD + outbox in one transaction

**Files:**
- Create: `backend/internal/store/events_admin.go`
- Create: `backend/internal/store/outbox.go`
- Test: `backend/internal/store/events_admin_test.go`

**Interfaces:**
- Produces:
  ```go
  // outbox.go
  type OutboxKind string // "published" | "updated" | "cancelled"
  // insertOutbox writes the event snapshot as jsonb payload inside tx.
  func insertOutbox(ctx, tx pgx.Tx, eventID uuid.UUID, kind OutboxKind, payload any) error

  // events_admin.go — all take the pool, open their own tx where needed
  func GetEventAdmin(ctx, pool, id uuid.UUID) (Event, error)          // drafts included
  func ListEventsAdmin(ctx, pool, from, to time.Time) ([]Event, error) // drafts + cancelled included
  // UpdateEvent saves the row; if the event is PUBLISHED and a broadcast-worthy
  // field changed (starts_at, ends_at, place — spec §7), it writes an
  // `updated` outbox row in the same tx. Title/description edits save silently.
  func UpdateEvent(ctx, pool, e Event) error
  // PublishEvent sets published_at=now() and writes a `published` outbox row; no-op error ErrAlreadyPublished if already published.
  func PublishEvent(ctx, pool, id uuid.UUID) error
  // UnpublishEvent clears published_at and writes a `cancelled` outbox row (spec Plan-2 decision #6).
  func UnpublishEvent(ctx, pool, id uuid.UUID) error
  // DeleteEvent: published && notify → soft-cancel (cancelled_at=now()) + `cancelled` outbox row;
  // otherwise hard DELETE. (Soft-cancelled events stay in the .ics for 90 days — Plan 1.)
  func DeleteEvent(ctx, pool, id uuid.UUID, notify bool) error
  ```
- Outbox payload: `{"id","title","description","place","starts_at","ends_at","group_id","rank"}` snapshot at write time (what Plan 3 renders into messages).
- `CreateEvent` from Plan 1 is reused for creation (add `CreatedBy *uuid.UUID` handling in the handler task, passed through the existing insert — extend the INSERT with `created_by` column and struct field `CreatedBy *uuid.UUID`; all existing tests keep passing because the zero value inserts NULL).

- [ ] **Step 1: Failing tests** — the decisive ones:
```go
func TestPublishWritesOutboxInSameTx(t *testing.T)   // publish → events.published_at set AND outbox row kind=published with payload.title matching, created in one tx (assert both post-conditions)
func TestPublishTwiceFails(t *testing.T)             // second publish → ErrAlreadyPublished, outbox still has exactly 1 row
func TestBroadcastWorthyEditWritesUpdated(t *testing.T)   // published event, change place → outbox kind=updated
func TestSilentEditWritesNothing(t *testing.T)       // published event, change title only → outbox count unchanged
func TestDraftEditWritesNothing(t *testing.T)        // draft event, change starts_at → no outbox row
func TestUnpublishWritesCancelled(t *testing.T)      // unpublish → published_at NULL + outbox kind=cancelled
func TestDeleteNotifySoftCancels(t *testing.T)       // delete notify=true on published → row still exists, cancelled_at set, outbox kind=cancelled
func TestDeleteNoNotifyHardDeletes(t *testing.T)     // delete notify=false → row gone, no outbox row
```
All use `testdb.New(t)`, seed with `store.CreateEvent` + `PublishEvent`, and read the outbox with a direct `pool.QueryRow` count/kind check. (`testdb` already truncates `outbox` and `events`.)

- [ ] **Step 2: Implement.** Transaction skeleton every mutator follows:
```go
	tx, err := pool.Begin(ctx)
	if err != nil { return err }
	defer tx.Rollback(ctx)
	// ... UPDATE/DELETE + optional insertOutbox(ctx, tx, ...) ...
	return tx.Commit(ctx)
```
`UpdateEvent` loads the current row `FOR UPDATE` inside the tx, compares `starts_at/ends_at/place`, writes the row, conditionally writes outbox. `PublishEvent` uses `UPDATE events SET published_at = now() WHERE id=$1 AND published_at IS NULL` and checks `RowsAffected()==1` to detect double publish without a race.

- [ ] **Step 3: Verify** — `go test ./internal/store/ -count=1` → PASS.
- [ ] **Step 4: Commit** — `git commit -m "feat: add admin event mutations with transactional outbox writes"`

---

### Task 11: Admin events handlers

**Files:**
- Create: `backend/internal/http/admin_events.go`
- Modify: `backend/internal/http/server.go` (replace the 501 placeholder)
- Test: `backend/internal/http/admin_events_test.go`

**Interfaces (JSON shapes the admin frontend consumes — Plan 5 depends on these):**
```
GET  /api/v1/admin/events?from&to      → {"events":[{...event, "published_at":null|ts, "cancelled_at":null|ts}]}
POST /api/v1/admin/events              ← {"title","description","place","starts_at","ends_at","group_id","rank","color_override"}
                                       → 201 {"event":{...}}   (draft; created_by = session user)
GET  /api/v1/admin/events/{id}         → {"event":{...}}       (404 no_encontrado)
PUT  /api/v1/admin/events/{id}         ← same shape as POST → 200 {"event":{...}}
POST /api/v1/admin/events/{id}/publish → 200 {"event":{...}} | 409 ya_publicado
POST /api/v1/admin/events/{id}/unpublish → 200
DELETE /api/v1/admin/events/{id}?notify=true|false → 204
```
- Validation (400 `bad_request`, Spanish): title required; `ends_at > starts_at`; `rank` in enum; `group_id` must exist (FK error mapped to 400, not 500); timestamps RFC 3339.
- All routes live under the existing `requireSession` admin subrouter; both roles may manage events (spec: secretaría manages events; only users-CRUD is párroco-only).

- [ ] **Step 1: Failing handler tests** — create→publish→list round-trip asserts the draft is invisible in the public `/api/v1/events` but visible in the admin list before publish; publish twice → 409; delete `?notify=false` → 204 and gone from admin list; malformed body → 400 with `bad_request`; unknown id → 404 `no_encontrado`.
- [ ] **Step 2: Implement** (thin handlers over Task 10's store; decode → validate → call → map errors: `pgx.ErrNoRows`→404, `ErrAlreadyPublished`→409, FK violation (pgErr.Code `23503`)→400).
- [ ] **Step 3: Verify** — `go test ./internal/http/ -count=1` → PASS.
- [ ] **Step 4: Commit** — `git commit -m "feat: add admin events endpoints with publish, unpublish and cancel"`

---

### Task 12: Channels CRUD

**Files:**
- Create: `backend/internal/store/channels.go` + test
- Create: `backend/internal/http/admin_channels.go` + test
- Modify: `backend/internal/http/server.go`

**Interfaces:**
```go
type Channel struct {
    ID       uuid.UUID
    Kind     string     // "whatsapp" | "email"
    Name     string
    Target   string     // WA group JID / email address
    GroupID  *uuid.UUID // nil = "toda la parroquia"
    IsActive bool
}
func ListChannels(ctx, pool) ([]Channel, error)
func CreateChannel(ctx, pool, c Channel) error
func UpdateChannel(ctx, pool, c Channel) error   // all fields by ID
func DeleteChannel(ctx, pool, id uuid.UUID) error // FK from broadcasts → map 23503 to 409 conflicto (deactivate instead)
```
Routes `GET/POST /api/v1/admin/channels`, `PUT/DELETE /api/v1/admin/channels/{id}` (both roles). JSON mirrors the struct (snake_case). Validation: kind in enum, target non-empty, group_id nil or existing.

- [ ] **Step 1: Failing tests** (store round-trip incl. nil group; handler create→list→update→delete cycle; invalid kind → 400).
- [ ] **Step 2: Implement.** — [ ] **Step 3: Verify** `go test ./... -count=1`. — [ ] **Step 4: Commit** — `git commit -m "feat: add channel management endpoints"`

---

### Task 13: Users admin endpoints (párroco only)

**Files:**
- Create: `backend/internal/http/admin_users.go` + test
- Modify: `backend/internal/http/server.go`

**Interfaces:**
```
GET  /api/v1/admin/users                  → {"users":[{id,email,display_name,role,is_active}]}
POST /api/v1/admin/users                  ← {"email","display_name","role","password"?} → 201
     (password optional: empty = magic-link-only account)
PUT  /api/v1/admin/users/{id}             ← {"display_name","role"} → 200
POST /api/v1/admin/users/{id}/deactivate  → 200   (SetUserActive false — revokes sessions)
POST /api/v1/admin/users/{id}/activate    → 200
```
- Subrouter: `admin.Route("/users", ...)` wrapped in `requireParroco`.
- Guards: a párroco cannot deactivate **themselves** (400 `no_permitido`, Spanish: "No puedes desactivar tu propia cuenta."); duplicate email → 409 `correo_duplicado` (unique violation 23505); role must be in enum.

- [ ] **Step 1: Failing tests** — secretaría session hitting `/admin/users` → 403; párroco full cycle: create secretaria (with password) → she can log in → deactivate her → her session 401s and login fails; self-deactivation → 400; duplicate email → 409.
- [ ] **Step 2: Implement.** — [ ] **Step 3: Verify** `go test ./... -count=1`. — [ ] **Step 4: Commit** — `git commit -m "feat: add parroco-only user management endpoints"`

---

### Task 14: Docs, roadmap, final verification

**Files:**
- Modify: `README.md` (auth section + setup command + new env vars)
- Modify: `docs/superpowers/plans/2026-08-10-roadmap.md` (Plan 2 → Done, Plan 3 → Next)

- [ ] **Step 1: README** — add under Development: `go run ./cmd/setup` (first user), the new env vars table (`AUTH_SECRET`, `REDIS_ADDR`, `TRUSTED_PROXY`, `SMTP_*` — note LogMailer prints magic links in dev), and a curl example of login + an authenticated admin call.
- [ ] **Step 2: Full local gate (what CI runs, plus the linter):**
```bash
go vet ./... && go test ./... -count=1
docker run --rm -v "<repo>:/src" -w /src/backend golangci/golangci-lint:v1.62.2 golangci-lint run ./...
```
- [ ] **Step 3: Boot smoke test** — compose up, `go run ./cmd/api`; `curl` login with the setup user; `GET /api/v1/auth/me` with the cookie; `POST` a draft event; publish it; confirm it appears in public `/api/v1/events` and the outbox row exists (`docker compose exec postgres psql -U pastoral -c "select kind from outbox"`).
- [ ] **Step 4: Roadmap update + commit** — `git commit -m "docs: document auth setup and mark plan 2 done"` — then push and verify CI green.

---

## Self-Review (performed)

1. **Spec coverage (Plan-2 slice):** §5 users/sessions (schema already migrated in Plan 1) → Tasks 3–4; §6 admin routes → Tasks 6, 11, 12, 13; §8 auth (argon2id, opaque sessions, magic link, rate limit, deactivation revokes) → Tasks 2, 4, 5, 6, 8; outbox-on-publish (§7 pipeline start + roadmap Plan 2 line) → Task 10; Plan-2 execution decisions 1–7 (spec addendum 2026-08-11) → mailer Task 7, client-IP Task 5, Redis Task 8, setup Task 9, cookie rule Task 6, unpublish-cancelled Task 10, channels-seed/broadcast-endpoints deferral honored (absent).
2. **Deliberately out of scope (Plan 3):** broadcasts endpoints + retry, difusión workers, initial channels seed, asynq, quiet hours/debounce.
3. **Type consistency:** `store.User` shared by Tasks 3/4/6/9/13; `NewRouter(pool, rdb, mailer, cfg)` final signature introduced in Task 8 — Tasks 6–7 compile against the intermediate signatures and each task leaves `go test ./...` green; `Event.CreatedBy` extension keeps Plan 1 tests green (zero value → NULL).
4. **Per-commit CI safety:** the only commit that adds a new externally-hosted dependency to tests (Redis) also amends `ci.yml` (Task 8 Step 5, same commit).
5. **Placeholder scan:** Tasks 8 (token test list), 9, 11–13 give test *names/behaviors* rather than full listings — intentional: they follow the exact patterns fully listed in Tasks 3–6 and 10; every interface, route shape, error code and Spanish message they need is specified inline.

