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

# 3. Run the difusión worker, in a second terminal
cd backend && go run ./cmd/worker
# relays the outbox every 2s and delivers the announcements

# 4. Create the first user (once per database)
cd backend && SETUP_EMAIL=parroco@parroquia.mx go run ./cmd/setup
# prints a generated password once — save it, then change it after logging in

# 5. Run the public site, in a third terminal
cd frontend && npm install && npm run dev
# → http://localhost:4321 (proxies /api and the .ics feeds to :8080)
# → http://localhost:4321/admin (the admin panel — same two dev processes as
#   the public site; step 3's worker is what actually delivers what it queues)

# 6. Tests (uses the pastoral_test database and Redis)
cd backend && go test ./...
```

Two processes: `cmd/api` serves HTTP and owns migrations, `cmd/worker` owns
sending. The worker refuses to start against an unmigrated database rather than
failing one query at a time.

## Admin API

Everything below also has a screen — see [Admin panel](#admin-panel) — but the
raw endpoints are here for scripting or debugging.

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

## Difusión

Publishing an event writes an outbox row in the same transaction as the change.
The worker relays it, resolves the event's channels (the group's own plus the
parish-wide ones), records one `broadcasts` row per channel and delivers it.

```bash
curl -s -b cookies.txt "localhost:8080/api/v1/admin/broadcasts?state=failed"
curl -s -b cookies.txt -X POST "localhost:8080/api/v1/admin/broadcasts/<id>/retry"
```

- **WhatsApp is a stub in v1.** Those broadcasts come back `simulated: true` and
  the worker logs them as `SIMULADO`; nothing is actually sent. A real provider
  drops in behind the `Sender` interface without touching the engine.
- **Email** goes over SMTP when `SMTP_HOST` is set and is logged otherwise, so
  development needs no mail server.
- Corrections are debounced for 10 minutes and collapse onto the newest edit;
  only time and place changes are announced at all.
- A cancellation reaches only the channels that actually received the event.
- Retries: 5 attempts, then the row turns `dead` and waits for a manual retry.

## Admin panel

`frontend/src/admin/`, served at `/admin`. Four screens over the admin API above:

- **Login** — password or magic link.
- **Eventos** — month list with per-event difusión counts and tabs; an editor
  with publish/draft/unpublish; a delete modal with a cancellation-notice
  checkbox (send an aviso to whoever already got the event, or don't).
- **Difusión** — the broadcasts log with retry, polling every 5s only while
  something is still queued, plus channel management (this is where the
  seeded placeholder WhatsApp targets get corrected).
- **Equipo** — párroco-only account management (create secretaría accounts,
  deactivate).

**Two frameworks, one project, on purpose.** The public calendar is a Preact
island (`frontend/src/islands/calendar/`); the admin panel is a separate React
island (`frontend/src/pages/admin/[...all].astro`, React Router inside with
`basename="/admin"`). They were built in different plans against different
mockups, and rewriting the calendar into React (or the panel into Preact)
bought nothing, so `astro.config.mjs` scopes each renderer to its own folder
(`include: ['**/islands/**']` vs. `include: ['**/admin/**']`) and every `.tsx`
under `src/admin/` carries `/** @jsxImportSource react */` because the
project's tsconfig defaults to Preact's JSX. State is a Redux Toolkit store
whose only slice is RTK Query over an axios baseQuery, with a 401 interceptor
that redirects to `/admin/login`.

The panel follows the mockups with a few deliberate departures, all forced by
what the backend actually has:

- The mockups show a third role, "Coordinador de grupo"; the backend enum has
  only `parroco` and `secretaria`, so only those two exist here (v2 backlog).
- The mockups' "Reglas activas" toggles are static text — the difusión
  engine's rules are compile-time constants, not per-parish settings.
- "Salud del proveedor" became a SIMULADO notice: v1 has no WhatsApp
  provider, so those broadcasts are simulated and only email is real.
- "Canales conectados" health bars became the real channel management
  mentioned above.
- "Invitar a alguien" creates the account directly — there is no invitation
  email, the person signs in with a magic link.
- "Registro de actividad" was dropped: no audit-log API exists to back it.
- The editor's free-text duration became a fixed `<select>`.

## Public site

Astro static build with one Preact island (`frontend/src/islands/calendar/`). The
island owns every view — month grid, week time grid, phone agenda, day panel,
event sheet — and fetches `/api/v1/events` per visible range plus `/seasons` and
`/groups` once.

- **Everything renders in parish time** (`frontend/src/lib/config.ts`, `PARISH_TZ`),
  `America/Tijuana` on both sides. If the two ever disagree, the site and the
  `.ics` feed show different hours for the same event.
- **Ordinary masses are not events.** The everyday schedule is the static
  horarios card; the calendar carries only what the parish publishes.
- Rank drives shape and weight, not just color: solemnidad fills, fiesta tints,
  memoria is a dot, and a group activity is dashed graphite — deliberately
  outside the liturgical palette.
- Vitest covers the calendar's date/rank logic and grid rendering (45 tests),
  but not every island rule yet, and the Playwright E2E smoke from spec §11
  is still outstanding. **The admin panel has no tests at all and no code
  review has run on `feat/admin-frontend`** — deferred by the project owner;
  see the roadmap. `npx astro check` and `npm run build` gate CI regardless.

## Environment

| Variable | Default | Notes |
|---|---|---|
| `DATABASE_URL` | dev Postgres on 5433 | |
| `PORT` | `8080` | |
| `PARISH_TZ` | `America/Tijuana` | tzdata is embedded in the binary; must match `frontend/src/lib/config.ts` |
| `PUBLIC_BASE_URL` | `http://localhost:8080` | must include the scheme; mints `.ics` UIDs |
| `REDIS_ADDR` | `localhost:6379` | magic-link single use and the asynq queues |
| `QUIET_START` / `QUIET_END` | `22` / `7` | parish-local hours during which messages are held; equal values disable |
| `STAGGER_SECONDS` | `8` | spacing between the deliveries of one announcement; `0` sends them at once |
| `AUTH_SECRET` | `dev-secret-change-me` | **set a real secret in production** |
| `TRUSTED_PROXY` | empty | CIDR of Caddy; only then is `X-Forwarded-For` trusted |
| `SMTP_HOST` / `_PORT` / `_USER` / `_PASS` / `_FROM` | empty / `587` | unset ⇒ mail is logged, not sent |

Tests share one live `pastoral_test` database across packages, so every test
that touches Postgres must obtain its pool from `internal/store/testdb.New`,
which serializes test binaries with a session advisory lock.

## Despliegue

Producción: `https://pastoral.jesuslab135.com` — un VPS de IONOS con Docker
Compose en `/opt/pastoral`, detrás de Caddy (HTTPS automático). Solo existe
producción: se despliega `main` y nada más.

- **Automático:** cada push a `main` con CI verde construye las imágenes
  (`ghcr.io/jesuslab135/pastoral-backend`, `ghcr.io/jesuslab135/pastoral-web`),
  las publica en GHCR y despliega por SSH (`.github/workflows/deploy.yml`).
  Si `/healthz` no responde tras el despliegue, se revierte sola a la
  etiqueta anterior (`/opt/pastoral/.tag.prev`).
- **Manual:** `gh workflow run Deploy` (despliega el HEAD de `main`).
- **Migraciones:** las corre `cmd/api` al arrancar (goose embebido con
  advisory lock). No existe paso de migración separado.
- **Base de datos limpia:** una base recién migrada trae solo el esquema,
  las temporadas litúrgicas y los 6 grupos. Sin eventos y **sin canales** —
  el párroco crea los canales reales en la pantalla de Difusión, y la cuenta
  inicial se crea una sola vez con `/app/setup`.
- **Config de runtime:** solo en `/opt/pastoral/.env` (plantilla:
  `deploy/.env.example`). Los únicos secretos en GitHub son `VPS_HOST`,
  `VPS_USER` y `VPS_SSH_KEY`.
- **Primer arranque:** el runbook completo (DNS, hardening, primer deploy,
  cuenta inicial del párroco) está en
  `docs/superpowers/plans/2026-08-12-deploy.md`, tareas 9–14.

## Manual database snapshot (no automated backups by design)

Development:

```bash
docker compose -f docker-compose.dev.yml exec postgres \
  pg_dump -U pastoral pastoral > snapshot-$(date +%Y%m%d).sql
```

Production (from any machine with the deploy key):

```bash
ssh deploy@198.71.54.171 "cd /opt/pastoral && docker compose exec -T postgres pg_dump -U pastoral pastoral" > pastoral-$(date +%F).sql
```

## Repository layout

- `backend/` — Go API + worker (cmd/api, cmd/worker, cmd/setup, internal/…)
- `frontend/` — Astro site: `src/pages`, `src/components` (static shell),
  `src/islands/calendar` (Preact island), `src/admin` (React admin panel),
  `src/lib` (API client + date logic), `src/styles/tokens.css` (design tokens)
- `deploy/` — compose files, Caddy config
- `project/` — original design handoff bundle (reference only)
- `docs/superpowers/` — spec and implementation plans
