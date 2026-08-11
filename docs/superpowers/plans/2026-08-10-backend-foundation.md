# Backend Foundation Implementation Plan (Plan 1 of 6)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A running Go API that serves the public calendar data (events, seasons, groups), .ics feeds, and health checks from PostgreSQL, with migrations, seeds, tests, and CI.

**Architecture:** Single Go module `backend/` with `cmd/api` binary. chi router → handler layer → store layer (pgx) → Postgres 16. Migrations embedded via goose. Dev Postgres+Redis via Docker Compose. Tests hit a real Postgres (`TEST_DATABASE_URL`), both locally (compose) and in CI (service container).

**Tech Stack:** Go 1.23, chi v5, pgx v5 (pgxpool + stdlib for goose), goose v3, google/uuid, Postgres 16, GitHub Actions.

## Global Constraints

- Go module path: `github.com/jesuslab135/pastoral-tijuana/backend`
- Go version: `1.23`
- **Commit messages: plain conventional commits. NEVER add `Co-Authored-By`, or any mention of Claude/Anthropic/AI. (Explicit user requirement.)**
- All user-facing error messages in **Spanish**; error JSON shape everywhere: `{"error":{"code":"...","message":"..."}}`
- Parish timezone: `America/Mexico_City` (env `PARISH_TZ`), used for "the date of an event"
- Dev ports: Postgres `5433`, Redis `6379`, API `8080` (avoid clashing with local installs)
- Season color enum values: `verde|violeta|rosa|blanco_oro|rojo`; rank enum: `solemnidad|fiesta|memoria|parroquial`
- Public GET endpoints send `Cache-Control: public, max-age=300`
- Run all Go commands from the `backend/` directory
- The `project/` directory is the design reference — never modify or serve it

---

### Task 1: Repo scaffold + dev Docker environment

**Files:**
- Create: `.gitignore`
- Create: `docker-compose.dev.yml`
- Create: `deploy/dev/init-test-db.sql`
- Create: `backend/go.mod`
- Create: `backend/cmd/api/main.go` (placeholder that compiles)

**Interfaces:**
- Produces: dev Postgres at `postgres://pastoral:pastoral@localhost:5433/pastoral?sslmode=disable`, test DB `pastoral_test` on the same server, Redis at `localhost:6379`. Later tasks assume these are up.

- [ ] **Step 1: Create `.gitignore`**

```gitignore
# OS / editor
.DS_Store
Thumbs.db
.idea/
.vscode/

# Go
backend/bin/
*.test
*.out

# Node (frontend, later plans)
node_modules/
frontend/dist/

# Env — never commit real secrets
.env
.env.*
!.env.example
```

- [ ] **Step 2: Create `docker-compose.dev.yml`**

```yaml
services:
  postgres:
    image: postgres:16
    environment:
      POSTGRES_USER: pastoral
      POSTGRES_PASSWORD: pastoral
      POSTGRES_DB: pastoral
    ports:
      - "5433:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
      - ./deploy/dev/init-test-db.sql:/docker-entrypoint-initdb.d/init-test-db.sql:ro
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U pastoral"]
      interval: 5s
      timeout: 5s
      retries: 10

  redis:
    image: redis:7
    ports:
      - "6379:6379"

volumes:
  pgdata:
```

- [ ] **Step 3: Create `deploy/dev/init-test-db.sql`**

```sql
CREATE DATABASE pastoral_test OWNER pastoral;
```

- [ ] **Step 4: Init the Go module and placeholder main**

Run (from repo root):
```bash
mkdir -p backend/cmd/api && cd backend && go mod init github.com/jesuslab135/pastoral-tijuana/backend
```

`backend/cmd/api/main.go`:
```go
package main

import "fmt"

func main() {
	fmt.Println("pastoral api: not wired yet")
}
```

- [ ] **Step 5: Verify everything builds and the DB comes up**

Run:
```bash
docker compose -f docker-compose.dev.yml up -d
cd backend && go build ./... && go vet ./...
docker compose -f ../docker-compose.dev.yml ps
```
Expected: build OK; `postgres` healthy; `redis` running.

- [ ] **Step 6: Commit**

```bash
git add .gitignore docker-compose.dev.yml deploy/dev/init-test-db.sql backend/
git commit -m "chore: scaffold backend module and dev docker environment"
```

---

### Task 2: Config package

**Files:**
- Create: `backend/internal/config/config.go`
- Test: `backend/internal/config/config_test.go`

**Interfaces:**
- Produces: `config.Load() config.Config` with fields `DatabaseURL, Port, ParishTZ, PublicBaseURL string`. All later tasks read config only through this.

- [ ] **Step 1: Write the failing test**

`backend/internal/config/config_test.go`:
```go
package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("PORT", "")
	t.Setenv("PARISH_TZ", "")
	t.Setenv("PUBLIC_BASE_URL", "")
	c := Load()
	if c.DatabaseURL != "postgres://pastoral:pastoral@localhost:5433/pastoral?sslmode=disable" {
		t.Errorf("DatabaseURL default wrong: %q", c.DatabaseURL)
	}
	if c.Port != "8080" {
		t.Errorf("Port default wrong: %q", c.Port)
	}
	if c.ParishTZ != "America/Mexico_City" {
		t.Errorf("ParishTZ default wrong: %q", c.ParishTZ)
	}
	if c.PublicBaseURL != "http://localhost:8080" {
		t.Errorf("PublicBaseURL default wrong: %q", c.PublicBaseURL)
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("PORT", "9999")
	c := Load()
	if c.Port != "9999" {
		t.Errorf("Port should come from env, got %q", c.Port)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -v`
Expected: FAIL (package does not compile: `Load` undefined)

- [ ] **Step 3: Write minimal implementation**

`backend/internal/config/config.go`:
```go
// Package config reads runtime configuration from environment variables.
package config

import "os"

type Config struct {
	DatabaseURL   string
	Port          string
	ParishTZ      string
	PublicBaseURL string
}

func Load() Config {
	return Config{
		DatabaseURL:   getenv("DATABASE_URL", "postgres://pastoral:pastoral@localhost:5433/pastoral?sslmode=disable"),
		Port:          getenv("PORT", "8080"),
		ParishTZ:      getenv("PARISH_TZ", "America/Mexico_City"),
		PublicBaseURL: getenv("PUBLIC_BASE_URL", "http://localhost:8080"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -v`
Expected: PASS (2 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat: add env-based config package"
```

---

### Task 3: Schema migrations (all 8 tables) + embedded goose runner

**Files:**
- Create: `backend/internal/store/migrations/00001_schema.sql`
- Create: `backend/internal/store/migrate.go`
- Create: `backend/internal/store/testdb/testdb.go` (test helper package)
- Test: `backend/internal/store/migrate_test.go`

**Interfaces:**
- Produces: `store.Migrate(db *sql.DB) error` (idempotent goose up); `testdb.New(t *testing.T) *pgxpool.Pool` — connects to `TEST_DATABASE_URL` (default `postgres://pastoral:pastoral@localhost:5433/pastoral_test?sslmode=disable`), runs migrations, truncates mutable tables, returns a pool. Every store/handler test uses `testdb.New`.

- [ ] **Step 1: Write the migration**

`backend/internal/store/migrations/00001_schema.sql`:
```sql
-- +goose Up
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TYPE season_color    AS ENUM ('verde','violeta','rosa','blanco_oro','rojo');
CREATE TYPE event_rank      AS ENUM ('solemnidad','fiesta','memoria','parroquial');
CREATE TYPE channel_kind    AS ENUM ('whatsapp','email');
CREATE TYPE outbox_kind     AS ENUM ('published','updated','cancelled');
CREATE TYPE broadcast_state AS ENUM ('queued','sent','failed','dead');
CREATE TYPE user_role       AS ENUM ('parroco','secretaria');

CREATE TABLE liturgical_seasons (
  id         smallserial PRIMARY KEY,
  name       text         NOT NULL,
  color      season_color NOT NULL,
  date_range daterange    NOT NULL,
  EXCLUDE USING gist (date_range WITH &&)
);

CREATE TABLE parish_groups (
  id        uuid PRIMARY KEY,
  name      text NOT NULL,
  slug      text NOT NULL UNIQUE,
  is_public boolean NOT NULL DEFAULT true,
  sort      integer NOT NULL DEFAULT 0
);

CREATE TABLE users (
  id            uuid PRIMARY KEY,
  email         citext NOT NULL UNIQUE,
  password_hash text,
  display_name  text NOT NULL DEFAULT '',
  role          user_role NOT NULL,
  is_active     boolean NOT NULL DEFAULT true,
  created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
  id         uuid PRIMARY KEY,
  user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash bytea NOT NULL UNIQUE,
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL,
  revoked_at timestamptz,
  ip         inet,
  user_agent text
);

CREATE TABLE events (
  id             uuid PRIMARY KEY,
  title          text NOT NULL,
  slug           text,
  description    text,
  place          text,
  starts_at      timestamptz NOT NULL,
  ends_at        timestamptz NOT NULL,
  group_id       uuid NOT NULL REFERENCES parish_groups(id),
  rank           event_rank NOT NULL,
  color_override season_color,
  published_at   timestamptz,
  cancelled_at   timestamptz,
  created_by     uuid REFERENCES users(id),
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  CHECK (ends_at > starts_at)
);
CREATE INDEX events_published_starts_idx
  ON events (starts_at) WHERE published_at IS NOT NULL;

CREATE TABLE channels (
  id        uuid PRIMARY KEY,
  kind      channel_kind NOT NULL,
  name      text NOT NULL,
  target    text NOT NULL,
  group_id  uuid REFERENCES parish_groups(id),
  is_active boolean NOT NULL DEFAULT true
);

CREATE TABLE outbox (
  id           bigserial PRIMARY KEY,
  event_id     uuid NOT NULL,
  kind         outbox_kind NOT NULL,
  payload      jsonb NOT NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  processed_at timestamptz
);
CREATE INDEX outbox_unprocessed_idx ON outbox (id) WHERE processed_at IS NULL;

CREATE TABLE broadcasts (
  id         uuid PRIMARY KEY,
  event_id   uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
  channel_id uuid NOT NULL REFERENCES channels(id),
  kind       outbox_kind NOT NULL,
  state      broadcast_state NOT NULL DEFAULT 'queued',
  attempt    integer NOT NULL DEFAULT 0,
  dedupe_key text NOT NULL UNIQUE,
  last_error text,
  sent_at    timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE broadcasts, outbox, channels, events, sessions, users, parish_groups, liturgical_seasons;
DROP TYPE user_role, broadcast_state, outbox_kind, channel_kind, event_rank, season_color;
DROP EXTENSION IF EXISTS citext;
```

- [ ] **Step 2: Write the goose runner**

`backend/internal/store/migrate.go`:
```go
// Package store provides database access: migrations, queries, and the pool.
package store

import (
	"database/sql"
	"embed"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrate applies all pending migrations. Safe to run repeatedly.
func Migrate(db *sql.DB) error {
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.Up(db, "migrations")
}
```

- [ ] **Step 3: Write the test helper**

`backend/internal/store/testdb/testdb.go`:
```go
// Package testdb gives tests a migrated, clean database pool.
package testdb

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
)

const defaultURL = "postgres://pastoral:pastoral@localhost:5433/pastoral_test?sslmode=disable"

// New migrates the test database, truncates mutable tables (seed data in
// liturgical_seasons and parish_groups is preserved), and returns a pool.
func New(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = defaultURL
	}

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
```

- [ ] **Step 4: Write the failing test**

`backend/internal/store/migrate_test.go`:
```go
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
	// 3000s: far future, cannot collide with seed data.
	_, err := pool.Exec(ctx, `INSERT INTO liturgical_seasons (name,color,date_range)
		VALUES ('Prueba A','verde','[3000-01-01,3000-02-01)')`)
	if err != nil {
		t.Fatalf("first insert: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM liturgical_seasons WHERE name LIKE 'Prueba %'`)
	})
	_, err = pool.Exec(ctx, `INSERT INTO liturgical_seasons (name,color,date_range)
		VALUES ('Prueba B','rojo','[3000-01-15,3000-03-01)')`)
	if err == nil {
		t.Fatal("overlapping season should be rejected by EXCLUDE constraint")
	}
}
```

- [ ] **Step 5: Fetch deps, run test to verify it fails then passes**

Run:
```bash
go get github.com/pressly/goose/v3 github.com/jackc/pgx/v5 github.com/google/uuid
go mod tidy
go test ./internal/store/... -v
```
Expected: PASS (compose DB from Task 1 must be up). If it fails with connection refused, run `docker compose -f ../docker-compose.dev.yml up -d` first.

- [ ] **Step 6: Commit**

```bash
git add internal/store/ go.mod go.sum
git commit -m "feat: add schema migration for all 8 tables with embedded goose runner"
```

---

### Task 4: Seed migrations — seasons 2026–2027 and parish groups

**Files:**
- Create: `backend/internal/store/migrations/00002_seed_seasons.sql`
- Create: `backend/internal/store/migrations/00003_seed_groups.sql`
- Test: `backend/internal/store/seed_test.go`

**Interfaces:**
- Produces: seeded `liturgical_seasons` covering 2026-01-01 → 2028-01-10 with no gaps, and 6 `parish_groups` with **fixed UUIDs** later tasks may reference (liturgia = `a1000000-0000-4000-8000-000000000001`, etc.).

- [ ] **Step 1: Write the seasons seed**

Liturgical dates used (verified): Easter 2026 = Apr 5, Ash Wednesday 2026 = Feb 18, Pentecost 2026 = May 24, Advent 2026 starts Nov 29, Gaudete 2026 = Dec 13, Baptism of the Lord 2027 = Jan 10; Ash Wednesday 2027 = Feb 10, Easter 2027 = Mar 28, Pentecost 2027 = May 16, Advent 2027 starts Nov 28, Gaudete 2027 = Dec 12, Baptism 2028 = Jan 9. Ranges are `[start,end)` (end exclusive).

`backend/internal/store/migrations/00002_seed_seasons.sql`:
```sql
-- +goose Up
INSERT INTO liturgical_seasons (name, color, date_range) VALUES
  ('Navidad',             'blanco_oro', '[2026-01-01,2026-01-12)'),
  ('Tiempo Ordinario',    'verde',      '[2026-01-12,2026-02-18)'),
  ('Cuaresma',            'violeta',    '[2026-02-18,2026-04-05)'),
  ('Pascua',              'blanco_oro', '[2026-04-05,2026-05-25)'),
  ('Tiempo Ordinario',    'verde',      '[2026-05-25,2026-11-29)'),
  ('Adviento',            'violeta',    '[2026-11-29,2026-12-13)'),
  ('Adviento · Gaudete',  'rosa',       '[2026-12-13,2026-12-14)'),
  ('Adviento',            'violeta',    '[2026-12-14,2026-12-25)'),
  ('Navidad',             'blanco_oro', '[2026-12-25,2027-01-11)'),
  ('Tiempo Ordinario',    'verde',      '[2027-01-11,2027-02-10)'),
  ('Cuaresma',            'violeta',    '[2027-02-10,2027-03-28)'),
  ('Pascua',              'blanco_oro', '[2027-03-28,2027-05-17)'),
  ('Tiempo Ordinario',    'verde',      '[2027-05-17,2027-11-28)'),
  ('Adviento',            'violeta',    '[2027-11-28,2027-12-12)'),
  ('Adviento · Gaudete',  'rosa',       '[2027-12-12,2027-12-13)'),
  ('Adviento',            'violeta',    '[2027-12-13,2027-12-25)'),
  ('Navidad',             'blanco_oro', '[2027-12-25,2028-01-10)');

-- +goose Down
DELETE FROM liturgical_seasons;
```

- [ ] **Step 2: Write the groups seed**

`backend/internal/store/migrations/00003_seed_groups.sql`:
```sql
-- +goose Up
INSERT INTO parish_groups (id, name, slug, is_public, sort) VALUES
  ('a1000000-0000-4000-8000-000000000001', 'Liturgia',         'liturgia',   true, 1),
  ('a1000000-0000-4000-8000-000000000002', 'Catequesis',       'catequesis', true, 2),
  ('a1000000-0000-4000-8000-000000000003', 'Pastoral juvenil', 'juvenil',    true, 3),
  ('a1000000-0000-4000-8000-000000000004', 'Coro',             'coro',       true, 4),
  ('a1000000-0000-4000-8000-000000000005', 'Caridad',          'caridad',    true, 5),
  ('a1000000-0000-4000-8000-000000000006', 'Formación',        'formacion',  true, 6);

-- +goose Down
DELETE FROM parish_groups;
```

- [ ] **Step 3: Write the failing test**

`backend/internal/store/seed_test.go`:
```go
package store_test

import (
	"context"
	"testing"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store/testdb"
)

func TestSeasonSeedHasNoGaps(t *testing.T) {
	pool := testdb.New(t)
	// Every date from 2026-01-01 to 2028-01-09 must fall in exactly one season.
	var gaps int
	err := pool.QueryRow(context.Background(), `
		SELECT count(*) FROM generate_series(
			'2026-01-01'::date, '2028-01-09'::date, interval '1 day') AS d
		WHERE NOT EXISTS (
			SELECT 1 FROM liturgical_seasons s WHERE s.date_range @> d::date)`,
	).Scan(&gaps)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if gaps != 0 {
		t.Errorf("found %d dates not covered by any season", gaps)
	}
}

func TestGaudeteIsRosa(t *testing.T) {
	pool := testdb.New(t)
	var color string
	err := pool.QueryRow(context.Background(),
		`SELECT color FROM liturgical_seasons WHERE date_range @> '2026-12-13'::date`,
	).Scan(&color)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if color != "rosa" {
		t.Errorf("2026-12-13 should be rosa (Gaudete), got %s", color)
	}
}

func TestGroupsSeeded(t *testing.T) {
	pool := testdb.New(t)
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM parish_groups WHERE is_public`).Scan(&n); err != nil {
		t.Fatalf("query: %v", err)
	}
	if n != 6 {
		t.Errorf("expected 6 public groups, got %d", n)
	}
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/store/ -run 'TestSeason|TestGroups|TestGaudete' -v`
Expected: PASS (goose applies the new migrations automatically via testdb.New)

- [ ] **Step 5: Commit**

```bash
git add internal/store/migrations/ internal/store/seed_test.go
git commit -m "feat: seed liturgical seasons 2026-2027 and parish groups"
```

---

### Task 5: Seasons store

**Files:**
- Create: `backend/internal/store/seasons.go`
- Test: `backend/internal/store/seasons_test.go`

**Interfaces:**
- Produces:
  ```go
  type Season struct {
      Name  string    // "Adviento"
      Color string    // "violeta"
      Start time.Time // inclusive (date at 00:00 UTC)
      End   time.Time // exclusive
  }
  func ListSeasonsForYear(ctx context.Context, pool *pgxpool.Pool, year int) ([]Season, error)
  func SeasonOf(ctx context.Context, pool *pgxpool.Pool, day time.Time) (Season, error)
  ```

- [ ] **Step 1: Write the failing test**

`backend/internal/store/seasons_test.go`:
```go
package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store/testdb"
)

func TestSeasonOf(t *testing.T) {
	pool := testdb.New(t)
	day := time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)
	s, err := store.SeasonOf(context.Background(), pool, day)
	if err != nil {
		t.Fatalf("SeasonOf: %v", err)
	}
	if s.Name != "Tiempo Ordinario" || s.Color != "verde" {
		t.Errorf("2026-08-12: got %q/%q, want Tiempo Ordinario/verde", s.Name, s.Color)
	}
}

func TestListSeasonsForYear(t *testing.T) {
	pool := testdb.New(t)
	seasons, err := store.ListSeasonsForYear(context.Background(), pool, 2026)
	if err != nil {
		t.Fatalf("ListSeasonsForYear: %v", err)
	}
	if len(seasons) < 6 {
		t.Fatalf("expected at least 6 season ranges touching 2026, got %d", len(seasons))
	}
	for _, s := range seasons {
		if !s.End.After(s.Start) {
			t.Errorf("season %q has End <= Start", s.Name)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestSeason -v`
Expected: FAIL (compile error: `SeasonOf` undefined)

- [ ] **Step 3: Write minimal implementation**

`backend/internal/store/seasons.go`:
```go
package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Season struct {
	Name  string
	Color string
	Start time.Time // inclusive
	End   time.Time // exclusive
}

// SeasonOf returns the liturgical season containing the given calendar date.
func SeasonOf(ctx context.Context, pool *pgxpool.Pool, day time.Time) (Season, error) {
	var s Season
	err := pool.QueryRow(ctx,
		`SELECT name, color::text, lower(date_range), upper(date_range)
		 FROM liturgical_seasons WHERE date_range @> $1::date`,
		day.Format("2006-01-02"),
	).Scan(&s.Name, &s.Color, &s.Start, &s.End)
	return s, err
}

// ListSeasonsForYear returns all season ranges overlapping the given year,
// ordered by start date.
func ListSeasonsForYear(ctx context.Context, pool *pgxpool.Pool, year int) ([]Season, error) {
	rows, err := pool.Query(ctx,
		`SELECT name, color::text, lower(date_range), upper(date_range)
		 FROM liturgical_seasons
		 WHERE date_range && daterange(make_date($1,1,1), make_date($1+1,1,1))
		 ORDER BY lower(date_range)`, year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Season
	for rows.Next() {
		var s Season
		if err := rows.Scan(&s.Name, &s.Color, &s.Start, &s.End); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store/ -run TestSeason -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/seasons.go internal/store/seasons_test.go
git commit -m "feat: add seasons store with date lookup and year listing"
```

---

### Task 6: Groups + events store (the dominant query)

**Files:**
- Create: `backend/internal/store/groups.go`
- Create: `backend/internal/store/events.go`
- Test: `backend/internal/store/events_test.go`

**Interfaces:**
- Produces:
  ```go
  type Group struct{ ID uuid.UUID; Name, Slug string }
  func ListPublicGroups(ctx context.Context, pool *pgxpool.Pool) ([]Group, error)

  type Event struct {
      ID            uuid.UUID
      Title, Description, Place string
      StartsAt, EndsAt time.Time
      GroupID       uuid.UUID
      Rank          string
      ColorOverride *string
      PublishedAt   *time.Time
      CancelledAt   *time.Time
  }
  // CreateEvent inserts (used by tests now, admin API in Plan 2).
  func CreateEvent(ctx context.Context, pool *pgxpool.Pool, e Event) error

  type PublicEvent struct {
      ID uuid.UUID
      Title, Description, Place string
      StartsAt, EndsAt time.Time
      GroupID uuid.UUID; GroupName, GroupSlug string
      Rank  string
      Color string // color_override if set, else season color of starts_at (parish TZ)
      UpdatedAt time.Time
  }
  func ListPublishedEvents(ctx context.Context, pool *pgxpool.Pool, from, to time.Time, tz string) ([]PublicEvent, error)
  ```

- [ ] **Step 1: Write the failing test**

`backend/internal/store/events_test.go`:
```go
package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store/testdb"
)

const liturgiaID = "a1000000-0000-4000-8000-000000000001"

func TestListPublishedEvents(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	now := time.Now()
	gid := uuid.MustParse(liturgiaID)

	published := store.Event{
		ID: uuid.New(), Title: "Hora santa", GroupID: gid, Rank: "parroquial",
		StartsAt: time.Date(2026, 8, 4, 19, 0, 0, 0, time.UTC),
		EndsAt:   time.Date(2026, 8, 4, 20, 0, 0, 0, time.UTC),
		PublishedAt: &now,
	}
	draft := store.Event{
		ID: uuid.New(), Title: "Borrador secreto", GroupID: gid, Rank: "parroquial",
		StartsAt: time.Date(2026, 8, 5, 19, 0, 0, 0, time.UTC),
		EndsAt:   time.Date(2026, 8, 5, 20, 0, 0, 0, time.UTC),
	}
	for _, e := range []store.Event{published, draft} {
		if err := store.CreateEvent(ctx, pool, e); err != nil {
			t.Fatalf("CreateEvent: %v", err)
		}
	}

	from := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	got, err := store.ListPublishedEvents(ctx, pool, from, to, "America/Mexico_City")
	if err != nil {
		t.Fatalf("ListPublishedEvents: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 published event (draft excluded), got %d", len(got))
	}
	e := got[0]
	if e.Title != "Hora santa" || e.GroupSlug != "liturgia" {
		t.Errorf("unexpected event: %+v", e)
	}
	// August 2026 is Tiempo Ordinario → verde.
	if e.Color != "verde" {
		t.Errorf("expected season color verde, got %q", e.Color)
	}
}

func TestColorOverrideWins(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	now := time.Now()
	rojo := "rojo"
	e := store.Event{
		ID: uuid.New(), Title: "Misa patronal", GroupID: uuid.MustParse(liturgiaID),
		Rank: "solemnidad", ColorOverride: &rojo,
		StartsAt: time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC),
		EndsAt:   time.Date(2026, 8, 29, 13, 45, 0, 0, time.UTC),
		PublishedAt: &now,
	}
	if err := store.CreateEvent(ctx, pool, e); err != nil {
		t.Fatalf("CreateEvent: %v", err)
	}
	got, err := store.ListPublishedEvents(ctx, pool,
		time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), "America/Mexico_City")
	if err != nil {
		t.Fatalf("ListPublishedEvents: %v", err)
	}
	if len(got) != 1 || got[0].Color != "rojo" {
		t.Fatalf("color_override should win over season color, got %+v", got)
	}
}

func TestListPublicGroups(t *testing.T) {
	pool := testdb.New(t)
	groups, err := store.ListPublicGroups(context.Background(), pool)
	if err != nil {
		t.Fatalf("ListPublicGroups: %v", err)
	}
	if len(groups) != 6 || groups[0].Slug != "liturgia" {
		t.Errorf("expected 6 groups sorted, liturgia first; got %+v", groups)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run 'TestList|TestColor' -v`
Expected: FAIL (compile error: `store.Event` undefined)

- [ ] **Step 3: Write minimal implementation**

`backend/internal/store/groups.go`:
```go
package store

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Group struct {
	ID   uuid.UUID
	Name string
	Slug string
}

func ListPublicGroups(ctx context.Context, pool *pgxpool.Pool) ([]Group, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, name, slug FROM parish_groups WHERE is_public ORDER BY sort`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Group
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.Name, &g.Slug); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}
```

`backend/internal/store/events.go`:
```go
package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Event struct {
	ID            uuid.UUID
	Title         string
	Description   string
	Place         string
	StartsAt      time.Time
	EndsAt        time.Time
	GroupID       uuid.UUID
	Rank          string
	ColorOverride *string
	PublishedAt   *time.Time
	CancelledAt   *time.Time
}

func CreateEvent(ctx context.Context, pool *pgxpool.Pool, e Event) error {
	_, err := pool.Exec(ctx,
		`INSERT INTO events
		   (id, title, description, place, starts_at, ends_at,
		    group_id, rank, color_override, published_at, cancelled_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		e.ID, e.Title, e.Description, e.Place, e.StartsAt, e.EndsAt,
		e.GroupID, e.Rank, e.ColorOverride, e.PublishedAt, e.CancelledAt)
	return err
}

type PublicEvent struct {
	ID          uuid.UUID
	Title       string
	Description string
	Place       string
	StartsAt    time.Time
	EndsAt      time.Time
	GroupID     uuid.UUID
	GroupName   string
	GroupSlug   string
	Rank        string
	Color       string
	UpdatedAt   time.Time
}

// ListPublishedEvents is the dominant query: one month of published,
// non-cancelled events with their effective color (override or season).
func ListPublishedEvents(ctx context.Context, pool *pgxpool.Pool, from, to time.Time, tz string) ([]PublicEvent, error) {
	rows, err := pool.Query(ctx,
		`SELECT e.id, e.title, coalesce(e.description,''), coalesce(e.place,''),
		        e.starts_at, e.ends_at,
		        g.id, g.name, g.slug, e.rank::text,
		        coalesce(e.color_override::text, s.color::text) AS color,
		        e.updated_at
		 FROM events e
		 JOIN parish_groups g ON g.id = e.group_id
		 JOIN liturgical_seasons s
		   ON s.date_range @> (e.starts_at AT TIME ZONE $3)::date
		 WHERE e.published_at IS NOT NULL
		   AND e.cancelled_at IS NULL
		   AND e.starts_at >= $1 AND e.starts_at < $2
		 ORDER BY e.starts_at`, from, to, tz)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PublicEvent
	for rows.Next() {
		var e PublicEvent
		if err := rows.Scan(&e.ID, &e.Title, &e.Description, &e.Place,
			&e.StartsAt, &e.EndsAt, &e.GroupID, &e.GroupName, &e.GroupSlug,
			&e.Rank, &e.Color, &e.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store/ -v`
Expected: PASS (all store tests)

- [ ] **Step 5: Commit**

```bash
git add internal/store/groups.go internal/store/events.go internal/store/events_test.go
git commit -m "feat: add groups and events store with dominant season-color query"
```

---

### Task 7: HTTP server scaffold + healthz + error/JSON helpers

**Files:**
- Create: `backend/internal/http/server.go`
- Create: `backend/internal/http/respond.go`
- Create: `backend/internal/http/health.go`
- Modify: `backend/cmd/api/main.go`
- Test: `backend/internal/http/health_test.go`

**Interfaces:**
- Produces:
  ```go
  // package httpapi (dir internal/http)
  func NewRouter(pool *pgxpool.Pool, cfg config.Config) http.Handler
  func writeJSON(w http.ResponseWriter, status int, v any)
  func writeError(w http.ResponseWriter, status int, code, msg string) // {"error":{"code","message"}}
  ```
- Consumes: `config.Load()` (Task 2), migrated pool.

- [ ] **Step 1: Write the failing test**

`backend/internal/http/health_test.go`:
```go
package httpapi

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/config"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store/testdb"
)

func TestHealthz(t *testing.T) {
	pool := testdb.New(t)
	r := NewRouter(pool, config.Load())
	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]bool
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if !body["ok"] {
		t.Errorf("expected ok:true, got %v", body)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/http/ -v`
Expected: FAIL (package does not exist / `NewRouter` undefined)

- [ ] **Step 3: Write minimal implementation**

`backend/internal/http/respond.go`:
```go
// Package httpapi wires the chi router, handlers, and JSON helpers.
package httpapi

import (
	"encoding/json"
	"net/http"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type errBody struct {
	Error errDetail `json:"error"`
}
type errDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, errBody{Error: errDetail{Code: code, Message: msg}})
}
```

`backend/internal/http/health.go`:
```go
package httpapi

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

func healthHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			writeError(w, http.StatusServiceUnavailable, "db_unavailable",
				"La base de datos no responde.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}
```

`backend/internal/http/server.go`:
```go
package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/config"
)

func NewRouter(pool *pgxpool.Pool, cfg config.Config) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", healthHandler(pool))
	return r
}
```

`backend/cmd/api/main.go` (replace placeholder):
```go
package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/config"
	httpapi "github.com/jesuslab135/pastoral-tijuana/backend/internal/http"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
)

func main() {
	cfg := config.Load()

	sqldb, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	if err := store.Migrate(sqldb); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	sqldb.Close()

	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	log.Printf("pastoral api listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, httpapi.NewRouter(pool, cfg)); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 4: Fetch chi, run tests, boot the server once**

Run:
```bash
go get github.com/go-chi/chi/v5
go mod tidy
go test ./internal/http/ -v
go run ./cmd/api &   # then: curl -s localhost:8080/healthz  → {"ok":true}; stop it after
```
Expected: test PASS; curl returns `{"ok":true}`.

- [ ] **Step 5: Commit**

```bash
git add internal/http/ cmd/api/main.go go.mod go.sum
git commit -m "feat: add chi server with healthz and JSON error helpers"
```

---

### Task 8: Public API handlers — /events, /seasons, /groups

**Files:**
- Create: `backend/internal/http/public.go`
- Modify: `backend/internal/http/server.go` (add routes)
- Test: `backend/internal/http/public_test.go`

**Interfaces:**
- Consumes: `store.ListPublishedEvents`, `store.ListSeasonsForYear`, `store.ListPublicGroups` (Tasks 5–6).
- Produces (JSON shapes the frontend islands consume — do not change without updating Plan 4):
  ```json
  GET /api/v1/events?from=2026-08-01&to=2026-09-01
  {"events":[{"id":"…","title":"…","description":"…","place":"…",
              "starts_at":"2026-08-04T19:00:00Z","ends_at":"…",
              "group":{"id":"…","name":"Liturgia","slug":"liturgia"},
              "rank":"parroquial","color":"verde"}]}

  GET /api/v1/seasons?year=2026
  {"seasons":[{"name":"Adviento","color":"violeta","start":"2026-11-29","end":"2026-12-13"}]}

  GET /api/v1/groups
  {"groups":[{"id":"…","name":"Liturgia","slug":"liturgia"}]}
  ```
- `from`/`to` are `YYYY-MM-DD`, interpreted in `PARISH_TZ`; `to` exclusive. Missing/invalid → 400 `{"error":{"code":"bad_request","message":"<Spanish>"}}`. Range wider than 400 days → 400. All three endpoints set `Cache-Control: public, max-age=300`.

- [ ] **Step 1: Write the failing test**

`backend/internal/http/public_test.go`:
```go
package httpapi

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/config"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store/testdb"
)

const liturgiaID = "a1000000-0000-4000-8000-000000000001"

func TestGetEvents(t *testing.T) {
	pool := testdb.New(t)
	now := time.Now()
	err := store.CreateEvent(context.Background(), pool, store.Event{
		ID: uuid.New(), Title: "Hora santa",
		GroupID: uuid.MustParse(liturgiaID), Rank: "parroquial",
		StartsAt:    time.Date(2026, 8, 4, 19, 0, 0, 0, time.UTC),
		EndsAt:      time.Date(2026, 8, 4, 20, 0, 0, 0, time.UTC),
		PublishedAt: &now,
	})
	if err != nil {
		t.Fatalf("seed event: %v", err)
	}

	r := NewRouter(pool, config.Load())
	req := httptest.NewRequest("GET", "/api/v1/events?from=2026-08-01&to=2026-09-01", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=300" {
		t.Errorf("Cache-Control = %q", cc)
	}
	var body struct {
		Events []struct {
			Title string `json:"title"`
			Color string `json:"color"`
			Group struct {
				Slug string `json:"slug"`
			} `json:"group"`
		} `json:"events"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if len(body.Events) != 1 || body.Events[0].Color != "verde" ||
		body.Events[0].Group.Slug != "liturgia" {
		t.Errorf("unexpected body: %s", rec.Body.String())
	}
}

func TestGetEventsBadRange(t *testing.T) {
	pool := testdb.New(t)
	r := NewRouter(pool, config.Load())
	for _, url := range []string{
		"/api/v1/events",                              // missing params
		"/api/v1/events?from=chido&to=2026-09-01",     // invalid date
		"/api/v1/events?from=2026-01-01&to=2028-01-01", // > 400 days
	} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest("GET", url, nil))
		if rec.Code != 400 {
			t.Errorf("%s: expected 400, got %d", url, rec.Code)
		}
		var e struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil || e.Error.Code != "bad_request" {
			t.Errorf("%s: expected bad_request error shape, got %s", url, rec.Body.String())
		}
	}
}

func TestGetSeasonsAndGroups(t *testing.T) {
	pool := testdb.New(t)
	r := NewRouter(pool, config.Load())

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/seasons?year=2026", nil))
	if rec.Code != 200 {
		t.Fatalf("seasons: expected 200, got %d", rec.Code)
	}
	var sb struct {
		Seasons []struct {
			Name, Color, Start, End string
		} `json:"seasons"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &sb); err != nil || len(sb.Seasons) < 6 {
		t.Errorf("seasons body: %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/groups", nil))
	if rec.Code != 200 {
		t.Fatalf("groups: expected 200, got %d", rec.Code)
	}
	var gb struct {
		Groups []struct{ Slug string } `json:"groups"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &gb); err != nil || len(gb.Groups) != 6 {
		t.Errorf("groups body: %s", rec.Body.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/http/ -run 'TestGet' -v`
Expected: FAIL (404s — routes not registered)

- [ ] **Step 3: Write minimal implementation**

`backend/internal/http/public.go`:
```go
package httpapi

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
)

const maxRangeDays = 400

func cachePublic(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "public, max-age=300")
}

type groupJSON struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	Slug string    `json:"slug"`
}

type eventJSON struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Place       string    `json:"place"`
	StartsAt    time.Time `json:"starts_at"`
	EndsAt      time.Time `json:"ends_at"`
	Group       groupJSON `json:"group"`
	Rank        string    `json:"rank"`
	Color       string    `json:"color"`
}

func eventsHandler(pool *pgxpool.Pool, tz string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		loc, err := time.LoadLocation(tz)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal",
				"Zona horaria mal configurada.")
			return
		}
		from, err1 := time.ParseInLocation("2006-01-02", r.URL.Query().Get("from"), loc)
		to, err2 := time.ParseInLocation("2006-01-02", r.URL.Query().Get("to"), loc)
		if err1 != nil || err2 != nil || !to.After(from) {
			writeError(w, http.StatusBadRequest, "bad_request",
				"Parámetros from y to requeridos en formato AAAA-MM-DD, con to posterior a from.")
			return
		}
		if to.Sub(from) > maxRangeDays*24*time.Hour {
			writeError(w, http.StatusBadRequest, "bad_request",
				"El rango máximo es de 400 días.")
			return
		}
		evs, err := store.ListPublishedEvents(r.Context(), pool, from, to, tz)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal",
				"No se pudieron cargar los eventos.")
			return
		}
		out := make([]eventJSON, 0, len(evs))
		for _, e := range evs {
			out = append(out, eventJSON{
				ID: e.ID, Title: e.Title, Description: e.Description, Place: e.Place,
				StartsAt: e.StartsAt, EndsAt: e.EndsAt,
				Group: groupJSON{ID: e.GroupID, Name: e.GroupName, Slug: e.GroupSlug},
				Rank:  e.Rank, Color: e.Color,
			})
		}
		cachePublic(w)
		writeJSON(w, http.StatusOK, map[string]any{"events": out})
	}
}

type seasonJSON struct {
	Name  string `json:"name"`
	Color string `json:"color"`
	Start string `json:"start"`
	End   string `json:"end"`
}

func seasonsHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		year := time.Now().Year()
		if q := r.URL.Query().Get("year"); q != "" {
			t, err := time.Parse("2006", q)
			if err != nil {
				writeError(w, http.StatusBadRequest, "bad_request",
					"El parámetro year debe ser un año de cuatro dígitos.")
				return
			}
			year = t.Year()
		}
		seasons, err := store.ListSeasonsForYear(r.Context(), pool, year)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal",
				"No se pudieron cargar los tiempos litúrgicos.")
			return
		}
		out := make([]seasonJSON, 0, len(seasons))
		for _, s := range seasons {
			out = append(out, seasonJSON{
				Name: s.Name, Color: s.Color,
				Start: s.Start.Format("2006-01-02"),
				End:   s.End.Format("2006-01-02"),
			})
		}
		cachePublic(w)
		writeJSON(w, http.StatusOK, map[string]any{"seasons": out})
	}
}

func groupsHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groups, err := store.ListPublicGroups(r.Context(), pool)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal",
				"No se pudieron cargar los grupos.")
			return
		}
		out := make([]groupJSON, 0, len(groups))
		for _, g := range groups {
			out = append(out, groupJSON{ID: g.ID, Name: g.Name, Slug: g.Slug})
		}
		cachePublic(w)
		writeJSON(w, http.StatusOK, map[string]any{"groups": out})
	}
}
```

In `backend/internal/http/server.go`, add inside `NewRouter` after the healthz route:
```go
	r.Route("/api/v1", func(api chi.Router) {
		api.Get("/events", eventsHandler(pool, cfg.ParishTZ))
		api.Get("/seasons", seasonsHandler(pool))
		api.Get("/groups", groupsHandler(pool))
	})
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/http/ -v`
Expected: PASS (all handler tests)

- [ ] **Step 5: Commit**

```bash
git add internal/http/
git commit -m "feat: add public events, seasons and groups endpoints"
```

---

### Task 9: iCalendar generation package

**Files:**
- Create: `backend/internal/ics/ics.go`
- Test: `backend/internal/ics/ics_test.go`
- Test fixture: `backend/internal/ics/testdata/calendar.golden`

**Interfaces:**
- Produces:
  ```go
  // package ics
  type Event struct {
      ID          uuid.UUID
      Title, Place, Description string
      StartsAt, EndsAt time.Time // UTC
      UpdatedAt   time.Time      // drives SEQUENCE (unix seconds)
      Cancelled   bool           // STATUS:CANCELLED when true
  }
  // Build renders a complete VCALENDAR (CRLF line endings, 75-octet folding).
  func Build(calName, host string, events []Event) string
  ```
- UID format: `<event-id>@<host>`. `SEQUENCE` = `UpdatedAt.Unix()` (monotonically increases on every update, which is all RFC 5545 requires).

- [ ] **Step 1: Write the failing golden test**

`backend/internal/ics/ics_test.go`:
```go
package ics

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func fixedEvents() []Event {
	return []Event{
		{
			ID:          uuid.MustParse("11111111-1111-4111-8111-111111111111"),
			Title:       "Misa solemne de la Asunción",
			Place:       "Templo parroquial",
			Description: "Misa solemne; el horario ordinario cambia.",
			StartsAt:    time.Date(2026, 8, 15, 18, 0, 0, 0, time.UTC),
			EndsAt:      time.Date(2026, 8, 15, 19, 30, 0, 0, time.UTC),
			UpdatedAt:   time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC),
		},
		{
			ID:        uuid.MustParse("22222222-2222-4222-8222-222222222222"),
			Title:     "Junta de pastoral; sala 2",
			StartsAt:  time.Date(2026, 8, 12, 1, 30, 0, 0, time.UTC),
			EndsAt:    time.Date(2026, 8, 12, 3, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC),
			Cancelled: true,
		},
	}
}

func TestBuildGolden(t *testing.T) {
	got := Build("Calendario Pastoral · Cristo de Los Álamos",
		"app.jesuslab135.com", fixedEvents())

	golden := filepath.Join("testdata", "calendar.golden")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden (run once with UPDATE_GOLDEN=1): %v", err)
	}
	if got != string(want) {
		t.Errorf("output differs from golden file.\nGot:\n%s", got)
	}
}

func TestBuildProperties(t *testing.T) {
	got := Build("Cal", "app.jesuslab135.com", fixedEvents())
	for _, want := range []string{
		"BEGIN:VCALENDAR", "END:VCALENDAR",
		"UID:11111111-1111-4111-8111-111111111111@app.jesuslab135.com",
		"DTSTART:20260815T180000Z",
		"STATUS:CANCELLED",
		"SEQUENCE:", "METHOD:PUBLISH",
		"SUMMARY:Junta de pastoral\\; sala 2", // semicolon escaped
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q", want)
		}
	}
	if strings.Contains(got, "\n") && !strings.Contains(got, "\r\n") {
		t.Error("lines must end with CRLF")
	}
	for _, line := range strings.Split(got, "\r\n") {
		if len([]byte(line)) > 75 {
			t.Errorf("line exceeds 75 octets: %q", line)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ics/ -v`
Expected: FAIL (`Build` undefined)

- [ ] **Step 3: Write minimal implementation**

`backend/internal/ics/ics.go`:
```go
// Package ics renders RFC 5545 iCalendar feeds for phone subscriptions.
package ics

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type Event struct {
	ID          uuid.UUID
	Title       string
	Place       string
	Description string
	StartsAt    time.Time
	EndsAt      time.Time
	UpdatedAt   time.Time
	Cancelled   bool
}

const stamp = "20060102T150405Z"

// escape implements RFC 5545 §3.3.11 TEXT escaping.
func escape(s string) string {
	r := strings.NewReplacer(`\`, `\\`, ";", `\;`, ",", `\,`, "\n", `\n`, "\r", "")
	return r.Replace(s)
}

// fold breaks a content line into 75-octet chunks with continuation lines.
func fold(line string) []string {
	const limit = 75
	b := []byte(line)
	if len(b) <= limit {
		return []string{line}
	}
	var out []string
	cur := limit
	// Never split inside a UTF-8 rune.
	for cur < len(b) && b[cur]&0xC0 == 0x80 {
		cur--
	}
	out = append(out, string(b[:cur]))
	rest := " " + string(b[cur:])
	out = append(out, fold(rest)...)
	return out
}

func Build(calName, host string, events []Event) string {
	var lines []string
	add := func(l string) { lines = append(lines, fold(l)...) }

	add("BEGIN:VCALENDAR")
	add("VERSION:2.0")
	add("PRODID:-//Calendario Pastoral//" + host + "//ES")
	add("CALSCALE:GREGORIAN")
	add("METHOD:PUBLISH")
	add("X-WR-CALNAME:" + escape(calName))

	for _, e := range events {
		add("BEGIN:VEVENT")
		add("UID:" + e.ID.String() + "@" + host)
		add("DTSTAMP:" + e.UpdatedAt.UTC().Format(stamp))
		add("DTSTART:" + e.StartsAt.UTC().Format(stamp))
		add("DTEND:" + e.EndsAt.UTC().Format(stamp))
		add("SUMMARY:" + escape(e.Title))
		if e.Place != "" {
			add("LOCATION:" + escape(e.Place))
		}
		if e.Description != "" {
			add("DESCRIPTION:" + escape(e.Description))
		}
		add("SEQUENCE:" + itoa(e.UpdatedAt.Unix()))
		if e.Cancelled {
			add("STATUS:CANCELLED")
		} else {
			add("STATUS:CONFIRMED")
		}
		add("END:VEVENT")
	}
	add("END:VCALENDAR")
	return strings.Join(lines, "\r\n") + "\r\n"
}

func itoa(n int64) string {
	if n < 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
		if n == 0 {
			break
		}
	}
	return string(b[i:])
}
```

- [ ] **Step 4: Generate golden file, then verify both tests pass**

Run:
```bash
mkdir -p internal/ics/testdata
UPDATE_GOLDEN=1 go test ./internal/ics/ -run TestBuildGolden
go test ./internal/ics/ -v
```
Expected: PASS. Inspect `testdata/calendar.golden` once by eye: two VEVENTs, escaped semicolon, CANCELLED status present.

- [ ] **Step 5: Commit**

```bash
git add internal/ics/
git commit -m "feat: add RFC 5545 ics generation with folding, escaping and cancellations"
```

---

### Task 10: .ics feed endpoints with ETag

**Files:**
- Create: `backend/internal/store/ics_query.go`
- Create: `backend/internal/http/ics.go`
- Modify: `backend/internal/http/server.go` (add routes)
- Test: `backend/internal/http/ics_test.go`

**Interfaces:**
- Consumes: `ics.Build` (Task 9).
- Produces:
  ```go
  // store: published events (incl. cancelled within last 90 days) in window
  // [now-90d, now+365d); groupSlug nil = all groups.
  func ListEventsForICS(ctx context.Context, pool *pgxpool.Pool, groupSlug *string, now time.Time) ([]PublicEventICS, error)
  type PublicEventICS struct {
      ID uuid.UUID
      Title, Place, Description string
      StartsAt, EndsAt, UpdatedAt time.Time
      Cancelled bool
  }
  ```
- Routes: `GET /calendario.ics` and `GET /calendario/{slug}.ics`. Headers: `Content-Type: text/calendar; charset=utf-8`, `Cache-Control: public, max-age=300`, `ETag: W/"<maxUpdatedUnix>-<count>"`; `If-None-Match` match → `304`. Unknown slug → 404 with the standard error shape.

- [ ] **Step 1: Write the store query**

`backend/internal/store/ics_query.go`:
```go
package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PublicEventICS struct {
	ID          uuid.UUID
	Title       string
	Place       string
	Description string
	StartsAt    time.Time
	EndsAt      time.Time
	UpdatedAt   time.Time
	Cancelled   bool
}

// ListEventsForICS returns published events for the feed window, including
// events cancelled within the last 90 days (STATUS:CANCELLED lets phones
// remove them). groupSlug nil means all groups.
func ListEventsForICS(ctx context.Context, pool *pgxpool.Pool, groupSlug *string, now time.Time) ([]PublicEventICS, error) {
	rows, err := pool.Query(ctx,
		`SELECT e.id, e.title, coalesce(e.place,''), coalesce(e.description,''),
		        e.starts_at, e.ends_at, e.updated_at,
		        (e.cancelled_at IS NOT NULL) AS cancelled
		 FROM events e
		 JOIN parish_groups g ON g.id = e.group_id
		 WHERE e.published_at IS NOT NULL
		   AND ($1::text IS NULL OR g.slug = $1)
		   AND (e.cancelled_at IS NULL OR e.cancelled_at > $2 - interval '90 days')
		   AND e.starts_at >= $2 - interval '90 days'
		   AND e.starts_at <  $2 + interval '365 days'
		 ORDER BY e.starts_at`, groupSlug, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PublicEventICS
	for rows.Next() {
		var e PublicEventICS
		if err := rows.Scan(&e.ID, &e.Title, &e.Place, &e.Description,
			&e.StartsAt, &e.EndsAt, &e.UpdatedAt, &e.Cancelled); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
```

- [ ] **Step 2: Write the failing handler test**

`backend/internal/http/ics_test.go`:
```go
package httpapi

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/config"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store/testdb"
)

func TestICSFeed(t *testing.T) {
	pool := testdb.New(t)
	now := time.Now()
	err := store.CreateEvent(context.Background(), pool, store.Event{
		ID: uuid.New(), Title: "Hora santa",
		GroupID: uuid.MustParse(liturgiaID), Rank: "parroquial",
		StartsAt:    now.Add(48 * time.Hour),
		EndsAt:      now.Add(49 * time.Hour),
		PublishedAt: &now,
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	r := NewRouter(pool, config.Load())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/calendario.ics", nil))

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/calendar; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
	if !strings.Contains(rec.Body.String(), "SUMMARY:Hora santa") {
		t.Errorf("feed missing event:\n%s", rec.Body.String())
	}

	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("missing ETag")
	}
	req := httptest.NewRequest("GET", "/calendario.ics", nil)
	req.Header.Set("If-None-Match", etag)
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req)
	if rec2.Code != 304 {
		t.Errorf("expected 304 with matching ETag, got %d", rec2.Code)
	}
}

func TestICSGroupFeedAndUnknownSlug(t *testing.T) {
	pool := testdb.New(t)
	r := NewRouter(pool, config.Load())

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/calendario/liturgia.ics", nil))
	if rec.Code != 200 {
		t.Errorf("liturgia feed: expected 200, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/calendario/nope.ics", nil))
	if rec.Code != 404 {
		t.Errorf("unknown slug: expected 404, got %d", rec.Code)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/http/ -run TestICS -v`
Expected: FAIL (404 — routes not registered)

- [ ] **Step 4: Write the handler**

`backend/internal/http/ics.go`:
```go
package httpapi

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/ics"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
)

const calName = "Calendario Pastoral · Cristo de Los Álamos"

func icsHandler(pool *pgxpool.Pool, publicBaseURL string) http.HandlerFunc {
	host := "app.jesuslab135.com"
	if u, err := url.Parse(publicBaseURL); err == nil && u.Host != "" {
		host = u.Hostname()
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var groupSlug *string
		if raw := chi.URLParam(r, "slug"); raw != "" {
			slug := strings.TrimSuffix(raw, ".ics")
			groups, err := store.ListPublicGroups(r.Context(), pool)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal",
					"No se pudo generar el calendario.")
				return
			}
			found := false
			for _, g := range groups {
				if g.Slug == slug {
					found = true
					break
				}
			}
			if !found {
				writeError(w, http.StatusNotFound, "not_found",
					"No existe ese grupo parroquial.")
				return
			}
			groupSlug = &slug
		}

		evs, err := store.ListEventsForICS(r.Context(), pool, groupSlug, time.Now())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal",
				"No se pudo generar el calendario.")
			return
		}

		var maxUpdated int64
		for _, e := range evs {
			if u := e.UpdatedAt.Unix(); u > maxUpdated {
				maxUpdated = u
			}
		}
		etag := fmt.Sprintf(`W/"%d-%d"`, maxUpdated, len(evs))
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		icsEvents := make([]ics.Event, 0, len(evs))
		for _, e := range evs {
			icsEvents = append(icsEvents, ics.Event{
				ID: e.ID, Title: e.Title, Place: e.Place, Description: e.Description,
				StartsAt: e.StartsAt, EndsAt: e.EndsAt,
				UpdatedAt: e.UpdatedAt, Cancelled: e.Cancelled,
			})
		}

		w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=300")
		w.Header().Set("ETag", etag)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(ics.Build(calName, host, icsEvents)))
	}
}
```

In `backend/internal/http/server.go`, add after the `/api/v1` route block:
```go
	r.Get("/calendario.ics", icsHandler(pool, cfg.PublicBaseURL))
	r.Get("/calendario/{slug}", icsHandler(pool, cfg.PublicBaseURL))
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/http/ -v`
Expected: PASS (all http tests)

- [ ] **Step 6: Commit**

```bash
git add internal/http/ internal/store/ics_query.go
git commit -m "feat: serve full and per-group ics feeds with etag support"
```

---

### Task 11: Backend CI workflow

**Files:**
- Create: `.github/workflows/ci.yml`

**Interfaces:**
- Produces: `backend` job that Plans 4–6 extend with `frontend` and deploy jobs. Job name `backend` is referenced by `deploy.yml` (Plan 6) via `needs`.

- [ ] **Step 1: Write the workflow**

`.github/workflows/ci.yml`:
```yaml
name: CI

on:
  pull_request:
  push:
    branches: [main]

jobs:
  backend:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:16
        env:
          POSTGRES_USER: pastoral
          POSTGRES_PASSWORD: pastoral
          POSTGRES_DB: pastoral_test
        ports:
          - 5433:5432
        options: >-
          --health-cmd "pg_isready -U pastoral"
          --health-interval 5s
          --health-timeout 5s
          --health-retries 10
    defaults:
      run:
        working-directory: backend
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.23"
          cache-dependency-path: backend/go.sum
      - name: Vet
        run: go vet ./...
      - name: Lint
        uses: golangci/golangci-lint-action@v6
        with:
          working-directory: backend
      - name: Test
        run: go test ./... -count=1
        env:
          TEST_DATABASE_URL: postgres://pastoral:pastoral@localhost:5433/pastoral_test?sslmode=disable
```

- [ ] **Step 2: Run the test suite locally one more time (what CI will run)**

Run (from `backend/`):
```bash
go vet ./... && go test ./... -count=1
```
Expected: PASS.

- [ ] **Step 3: Commit and push; verify CI goes green**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: add backend workflow with postgres service"
git push
```
Then check: `gh run watch` (or the Actions tab) — the `backend` job must pass. If golangci-lint reports issues, fix them (they are real findings) and commit the fixes with `style: fix lint findings`.

---

### Task 12: README + roadmap status update

**Files:**
- Create: `README.md` → **replace** the design-handoff README (move its content to `project/README-design-bundle.md` first)
- Modify: `docs/superpowers/plans/2026-08-10-roadmap.md` (mark Plan 1 done)

**Interfaces:**
- Produces: canonical developer onboarding doc; later plans append their sections.

- [ ] **Step 1: Preserve the design-bundle README, write the project README**

```bash
git mv README.md project/README-design-bundle.md
```

New `README.md`:
```markdown
# Calendario Pastoral — Cristo de Los Álamos

Liturgical calendar platform: public read-only calendar, admin panel, and a
difusión engine that pushes published events to WhatsApp groups and email.

**Stack:** Go 1.23 · PostgreSQL 16 · Redis 7 (asynq) · Astro + islands · Docker

## Development

```bash
# 1. Start Postgres (5433) and Redis (6379)
docker compose -f docker-compose.dev.yml up -d

# 2. Run the API (auto-migrates on boot)
cd backend && go run ./cmd/api
# → http://localhost:8080/healthz
# → http://localhost:8080/api/v1/events?from=2026-08-01&to=2026-09-01
# → http://localhost:8080/calendario.ics

# 3. Tests (uses the pastoral_test database)
cd backend && go test ./...
```

## Manual database snapshot (no automated backups by design)

```bash
docker compose -f docker-compose.dev.yml exec postgres \
  pg_dump -U pastoral pastoral > snapshot-$(date +%Y%m%d).sql
```

## Repository layout

- `backend/` — Go API + worker (cmd/api, internal/…)
- `frontend/` — Astro site (Plan 4+)
- `deploy/` — compose files, Caddy config
- `project/` — original design handoff bundle (reference only)
- `docs/superpowers/` — spec and implementation plans
```

- [ ] **Step 2: Update the roadmap**

In `docs/superpowers/plans/2026-08-10-roadmap.md`, change Plan 1's status from `Planned` to `Done`, and Plan 2's from `Pending` to `Next`.

- [ ] **Step 3: Commit**

```bash
git add README.md project/README-design-bundle.md docs/superpowers/plans/2026-08-10-roadmap.md
git commit -m "docs: add developer README and update roadmap"
git push
```

---

## Self-Review (performed)

1. **Spec coverage (Plan-1 slice):** schema §5 (all 8 tables + cancelled_at refinement) → Tasks 3–4; dominant query → Task 6; public API §6 → Task 8; .ics §6/§9 (webcal, ETag, CANCELLED, per-group) → Tasks 9–10; healthz → Task 7; CI backend job §10 → Task 11; dev environment → Task 1. Auth/admin/difusión/frontend/deploy are Plans 2–6 by design.
2. **Placeholder scan:** none — every step has full code or exact commands.
3. **Type consistency:** verified — `store.Event`/`PublicEvent`/`PublicEventICS`, `ics.Event`, `NewRouter(pool, cfg)`, `testdb.New(t)` used consistently across Tasks 3–10; fixed-UUID `liturgiaID` constant is defined in both test packages that use it (they are separate packages, so each declares it).
