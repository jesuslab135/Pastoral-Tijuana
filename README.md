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
