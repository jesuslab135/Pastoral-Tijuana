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

# 3. Create the first user (once per database)
cd backend && SETUP_EMAIL=parroco@parroquia.mx go run ./cmd/setup
# prints a generated password once — save it, then change it after logging in

# 4. Tests (uses the pastoral_test database and Redis)
cd backend && go test ./...
```

## Admin API

```bash
# Log in and keep the session cookie
curl -s -c cookies.txt -X POST localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"parroco@parroquia.mx","password":"<la contraseña>"}'

# Use it
curl -s -b cookies.txt localhost:8080/api/v1/auth/me
curl -s -b cookies.txt "localhost:8080/api/v1/admin/events?from=2026-08-01&to=2026-09-01"
```

Passwordless entry: `POST /api/v1/auth/magic-link` with `{"email":"..."}`. Without
SMTP configured the link is printed to the API log instead of emailed, so it is
usable in development. Links last 15 minutes and work once.

Roles: `secretaria` manages events and channels; `parroco` additionally manages
the team (`/api/v1/admin/users`). Deactivating someone revokes their live
sessions immediately.

## Environment

| Variable | Default | Notes |
|---|---|---|
| `DATABASE_URL` | dev Postgres on 5433 | |
| `PORT` | `8080` | |
| `PARISH_TZ` | `America/Mexico_City` | tzdata is embedded in the binary |
| `PUBLIC_BASE_URL` | `http://localhost:8080` | must include the scheme; mints `.ics` UIDs |
| `REDIS_ADDR` | `localhost:6379` | magic-link single-use only |
| `AUTH_SECRET` | `dev-secret-change-me` | **set a real secret in production** |
| `TRUSTED_PROXY` | empty | CIDR of Caddy; only then is `X-Forwarded-For` trusted |
| `SMTP_HOST` / `_PORT` / `_USER` / `_PASS` / `_FROM` | empty / `587` | unset ⇒ mail is logged, not sent |

Tests share one live `pastoral_test` database across packages, so every test
that touches Postgres must obtain its pool from `internal/store/testdb.New`,
which serializes test binaries with a session advisory lock.

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
