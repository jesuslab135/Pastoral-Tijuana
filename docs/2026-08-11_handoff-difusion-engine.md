# Handoff — resume at Plan 3 execution (Difusión Engine)

State as of 2026-08-11, end of session. `main` = `eaa506e`, CI green on every commit referenced here.

## Where the project stands

| Plan | Status |
|---|---|
| 1 — Backend Foundation | **Done**, merged (`acf4e22`). Public API, .ics feeds, schema+seeds, CI. 10-finding review fixed. |
| 2 — Auth & Admin API | **Done**, merged (`8f69e6f`). Sessions, magic link, admin events/channels/users, transactional outbox writes. |
| 3 — Difusión Engine | **Planned, NOT executed.** Plan: `docs/superpowers/plans/2026-08-11-difusion-engine.md` (14 TDD tasks). Spec addendum: §"Plan 3 execution decisions" in the design spec. |
| 4 — Public Frontend / 5 — Admin Frontend / 6 — Deploy | Pending; plans not written yet (rule: write each plan only when the previous is done). |

Branches `feat/backend-foundation` and `feat/auth-admin-api` are fully merged — safe to delete. The roadmap (`docs/superpowers/plans/2026-08-10-roadmap.md`) is the authoritative status + execution-notes ledger; read it first.

## How to resume Plan 3

1. `docker compose -f docker-compose.dev.yml up -d` (Postgres 5433, Redis 6379).
2. `git checkout -b feat/difusion-engine` from up-to-date `main`.
3. Execute `2026-08-11-difusion-engine.md` task by task: test first, implement, `go test ./... -count=1` green, **one commit per task**, plan's exact commit messages.
4. After Task 14: whole-branch review (`/code-review high feat/difusion-engine`), fix findings, then `merge --no-ff` into `main` (message pattern: `Merge Plan 3: Difusión Engine`).

## Hard rules (do not relax)

- Commits: plain conventional. **Never** `Co-Authored-By`, never any Claude/Anthropic/AI mention.
- `go.mod` directive stays `go 1.23.x`. Always `go get <module>@<explicit version>`, then check `head -3 go.mod`. This bit twice in Plan 2 (x/crypto → 1.25, go-redis → 1.24). Current pins: chi v5.3.1, pgx v5.7.6, goose v3.26.0, uuid v1.6.0, x/crypto v0.40.0, go-redis v9.7.3. Plan 3 adds asynq — pin explicitly.
- Never weaken CI (no `continue-on-error`, no deleted steps, no `-p 1`).
- Postgres tests only via `testdb.New(t)`; Redis tests fail (not skip) when `TEST_REDIS_ADDR` is set.
- All user-facing strings Spanish; error shape `{"error":{"code","message"}}`.
- `project/` is design reference — never modify.

## Verification gates that caught real bugs (rerun them)

```bash
# Fresh-database + Redis run, identical to CI (from repo root):
docker network create ci-net; docker run -d --name ci-pg --network ci-net -e POSTGRES_USER=pastoral -e POSTGRES_PASSWORD=pastoral -e POSTGRES_DB=pastoral_test postgres:16
docker run -d --name ci-redis --network ci-net redis:7
docker run --rm --network ci-net -v "<repo>:/src" -w /src/backend \
  -e TEST_DATABASE_URL="postgres://pastoral:pastoral@ci-pg:5432/pastoral_test?sslmode=disable" \
  -e TEST_REDIS_ADDR="ci-redis:6379" golang:1.23 sh -c "go vet ./... && go test ./... -count=1"
# Plan 3 extra: after migrating the empty DB, channels seed count for the three b1… UUIDs must be 3.
# Lint exactly as CI:
docker run --rm -v "<repo>:/src" -w /src/backend golangci/golangci-lint:v1.62.2 golangci-lint run ./...
# cleanup: docker rm -f ci-pg ci-redis; docker network rm ci-net
```

Boot smoke test pattern (Plan 2's, extend with the worker): scratch DB → `cmd/api` + `cmd/worker` → `cmd/setup` → login → create+publish event → worker logs SIMULADO + mail → `psql`: outbox processed, broadcasts `sent`.

## Environment facts

- Local Go is 1.26.x; fine — the *directive* is what must stay 1.23.
- `go run` does not forward SIGTERM; graceful-shutdown checks need the compiled binary as PID 1.
- Advisory locks in use: `987654321` (testdb), `987654322` (migrations). Don't reuse.
- `PUBLIC_BASE_URL` mints .ics UIDs — permanent once phones subscribe. Prod value: `https://pastoral.jesuslab135.com`.
- `app.jesuslab135.com` is an unrelated server. The calendar gets a NEW Namecheap A record `pastoral → <VPS IP from IONOS panel>`; wait for propagation before Caddy issues certs (Plan 6 runbook step 1).

## User actions pending (independent of code, can be done anytime)

1. Add the `pastoral` A record in Namecheap (leave `app` alone).
2. VPS hardening (runbook step 2): deploy user + SSH key, disable password auth, **rotate the password shared in chat**, ufw 22/80/443 + fail2ban. The SSH key becomes the `VPS_SSH_KEY` GitHub secret in Plan 6 — the only secrets ever needed are `VPS_HOST`, `VPS_USER`, `VPS_SSH_KEY`; runtime config lives in `/opt/pastoral/.env` on the VPS.
