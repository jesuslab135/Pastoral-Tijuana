# Handoff — Plan 5 (Admin Frontend) built, review and tests deferred

State as of 2026-08-12. **Merged into `main` as `2e17de9`** (`Merge Plan 5: Admin Frontend`), CI green
on both jobs before the merge.

**This code was merged without any review and without tests** — a deliberate owner decision to keep
the MVP moving. Everything in the two sections below is therefore outstanding *on `main`*, not on a
side branch, and a defect found later is a fix-forward rather than a branch that never landed.

## What exists now

The parish panel at `/admin`: login (password + magic link), the month event list, the event editor
with publish/draft/unpublish, the delete confirmation, the difusión log with retry and channel
management, and párroco-only team management. 14 commits, ~3,900 lines.

| Plan | Status |
|---|---|
| 1 Backend Foundation · 2 Auth & Admin API · 3 Difusión Engine · 4 Public Frontend | Done, merged |
| **5 Admin Frontend** | **Done, merged (`2e17de9`) — review and tests still owed, see below** |
| 6 Deploy | Next; plan not written yet |

## THE TWO THINGS THAT ARE PENDING

### 1. No tests were written for the admin panel

Not one. The 45 vitest tests on this branch belong to the **public calendar island** and cover none
of the admin code. What the spec (§11) still owes, and what a reasonable first pass would be:

- **Unit** — `admin/dates.ts` (`parishToISO` round-trip, DST edges), `eventos/form.ts`
  (`toInput`/`fromEvent` round-trip, `validate`), `eventos/casts.ts` (`castsByEvent` counting).
  These are pure functions; they are the cheapest and highest-value tests here.
- **Component (vitest + testing-library, msw for the API)** — the editor's publish/draft/unpublish
  state machine, the delete modal's notify checkbox appearing only for published events, the
  difusión log's state→style mapping, Equipo's two-role rendering.
- **E2E (Playwright, spec §11)** — still outstanding from Plan 4 as well.

### 2. No code review has run on this branch — none at all

Not per-task, not whole-branch. Both were deferred at your instruction. Every screen's only gate was
its own author's self-review plus a green `astro check` / `npm test` / `npm run build`. That catches
compile errors; it does not catch spec drift, a screen binding the wrong field, or a state machine
that is subtly wrong.

**Suggested command when you pick this up:** `/code-review high feat/admin-frontend`

Self-review did catch real defects during the run, which is some evidence the code is not careless —
a stray query param that would have split an RTK Query cache entry, a lost `line-height`, and a
title that pushed its tag out of the row. But those were caught by the authors, not by a reviewer.

### Known open items, worth the reviewer's attention

1. **`ListBroadcasts` now inner-joins `events`.** A broadcast whose event was hard-deleted
   (`DELETE ?notify=false`) would silently vanish from the difusión log. No such case exists in v1,
   but a LEFT JOIN is the safe form.
2. **The admin pages carry no `noindex`.** The panel shell could be indexed by a crawler. No data
   leaks (the API requires a session), but it should not be in search results.
3. **A React warning in the console:** the group chips in the event editor mix the `border`
   shorthand with `borderColor`. Cosmetic, but it is a real warning on every render.
4. **`deleteEvent` / `deleteChannel` / `logout` are typed `void`** but the API answers 204, so axios
   stores `''`. Callers `unwrap()` and ignore it; a stricter type would be honest.
5. **Task 6 (the event editor) was written by the controller session, not a fresh implementer** —
   the subagent was killed mid-file by an API session limit. It therefore has the weakest independent
   scrutiny of any screen. Review it first.
6. **Deep-link refresh on `/admin/eventos` needs the Caddy fallback** planned for Plan 6; in dev the
   Astro server handles it.

### Not exercised end to end, by anyone

Creating → publishing → deleting an event **through the UI** (the API contracts were curl-verified,
the UI path was not), and the magic-link round trip.

## Two environmental traps that cost time today — read before debugging

- **A blank admin panel in dev is almost certainly a stale Vite cache**, not your code. Installing
  the React deps while a dev server was running left `frontend/node_modules/.vite` holding a dead
  optimizer hash (`504 Outdated Optimize Dep` on `react-redux` / `react-router-dom`). Fix:
  `rm -rf frontend/node_modules/.vite`, then restart the dev server. The production build was never
  affected — `npm run build` was green the whole time the dev server showed nothing.
- **`go run` does not hot-reload.** The difusión log showed no event titles for a while purely
  because the API process had been started before `event_title` was added. Restart the API after
  backend changes.
- **Never run `cmd/worker` against the test Redis while `go test ./...` runs.** The worker consumes
  the same asynq queues the integration test drives and will steal its deliver tasks, making
  `TestPipelineEndToEnd` time out. This was briefly misdiagnosed today as a pre-existing test
  failure — it is not; the suite is green. CI is unaffected (clean Redis, no worker).

## How to pick it up

```bash
docker compose -f docker-compose.dev.yml up -d          # Postgres 5433, Redis 6379
cd backend && PUBLIC_BASE_URL=http://localhost:8080 go run ./cmd/api
cd backend && go run ./cmd/worker                       # only when NOT running the test suite
cd frontend && npm run dev                              # http://localhost:4321/admin
```

Dev account: `parroco@parroquia.mx` / `qaYBd4reJhwwohW6`
Dev data: 37 published events in August 2026, 108 broadcasts, 3 channels (two carry
`PENDIENTE-JID-…` placeholder targets on purpose — the Difusión screen's channel card is where a
párroco replaces them).

An SDD ledger with every decision, deferred minor and implementer note from this run lives at
`.superpowers/sdd/2026-08-12-admin-frontend/progress.md` (git-ignored, local to this machine).

## Deliberate deviations from the mockups

These are design decisions, not omissions — the mockups describe things the backend does not have:

| Mockup | Reality |
|---|---|
| Third role "Coordinador de grupo" | The enum has `parroco` and `secretaria` only. v2 backlog. |
| "Reglas activas" toggles | Static text — the engine's rules are compile-time constants. |
| "Salud del proveedor" | A SIMULADO notice; v1 has no WhatsApp provider. |
| "Canales conectados" health bars | Real channel management — where placeholder JIDs get fixed. |
| "Invitar a alguien" + pending state | Creates the account directly; the person signs in by magic link. No invitation email exists. |
| "Registro de actividad" | Dropped — there is no audit-log API. |
| Free-text duration ("1 h 30") | A fixed `<select>`; parsing free text was a bug farm. |
