# Calendario Pastoral — Design Spec

**Date:** 2026-08-10
**Status:** Approved in brainstorming; pending user review of this document
**Source designs:** `project/*.dc.html` (Claude Design handoff bundle — public site, admin login/eventos/difusión/equipo, architecture blueprint)

## 1. What this is

A liturgical calendar platform for the parish **Cristo de Los Álamos**:

1. **Public site** — read-only calendar (month/week views) painted by liturgical season, with mass schedules, parish-group filters, and phone-calendar subscription (.ics). No accounts, no login, fully cacheable.
2. **Admin panel** (`/admin`) — the párroco and secretaría log in to create/edit/publish events, manage channels and team, and monitor difusión.
3. **Difusión engine** — publishing an event automatically fans out notifications to channels (WhatsApp groups, email) via a transactional outbox → queue pipeline. Nothing is sent by hand.

**Production target:** `app.jesuslab135.com` on the user's IONOS VPS, Docker Compose, deployed by GitHub Actions.

## 2. Goals and non-goals

**Goals (v1)**

- Pixel-faithful implementation of the handoff designs (public site + 4 admin screens).
- Full difusión pipeline (outbox, queues, retries, per-channel state) with a **stub WhatsApp sender** and a **real SMTP email sender**.
- `.ics` feeds (full + per-group) so phones subscribe once and stay current.
- Automated CI (test/lint/build) and CD (build images → push GHCR → SSH deploy → migrate → health check) via GitHub Actions YAML.
- Complete VPS runbook: everything needed after code is written to reach production.

**Non-goals (v1)**

- No parishioner accounts, registration, RSVP, headcounts, or attendance (`cero registro público`).
- No real WhatsApp provider (whatsmeow is v2, slots in behind the `Sender` interface).
- No computed liturgical calendar (Easter math): seasons are seeded date ranges per year; saints' days are ordinary events entered by the team.
- No multi-parish support: single parish, branding in config.
- No weekly grouped boletín digest (email sends per event in v1; digest is v2).

## 3. Decisions log

| Decision | Choice | Why |
|---|---|---|
| Frontend | **Astro + islands**, one project | Public calendar is content: static HTML + one small Preact island. Admin is a React island. Near-zero JS for parishioners. |
| Admin state | **Redux Toolkit + RTK Query with axios baseQuery** | User request; RTK Query handles caching/refetch, axios interceptors handle 401s. |
| Public island | Native `fetch`, local state | Keeps the island tiny; axios/Redux there would multiply its size for four state fields. |
| Backend | **Go 1.23**, chi router, two binaries (api, worker) from one module | User requirement; matches blueprint. |
| DB | **PostgreSQL 16** in Docker | User requirement. |
| Queue | **asynq + Redis 7** | Survives restarts, retries with backoff, dead-letter queue, inspectable. |
| WhatsApp v1 | **Stub sender** (logs, marks `SIMULADO`) | Zero ban risk while the parish adopts the tool; real provider later behind `Sender`. |
| Email v1 | **Real SMTP sender** | User request ("SMTP for email notification as well, just in case"); also powers magic links. |
| Auth | Email+password (argon2id) + magic link; **server-side `sessions` table**; cookie holds opaque token | User chose revocable sessions over stateless cookies. |
| People table | **`users`** (renamed from blueprint's `staff`), roles `parroco`/`secretaria` | User choice; admin-team-only login, no public accounts. |
| Hosting | IONOS VPS + Docker Compose behind **Caddy** (auto-HTTPS) | User has the VPS and domain; single-server ops. |
| Deploy | GH Actions → GHCR images → SSH → `docker compose up` | User requirement (YAML on GH Actions). |
| Masses ("horarios ordinarios") | Config file, not DB | It's a weekly template; special masses are events. |

## 4. Architecture

```
                 app.jesuslab135.com  (DNS A record — verify real VPS IP, see §12)
                        │
                  ┌─────▼─────┐
                  │   Caddy   │  TLS (Let's Encrypt), serves static Astro build,
                  └─┬───────┬─┘  proxies /api/* and /calendario*.ics
            /api/*  │       │  /*
              ┌─────▼───┐   │
              │  Go API │◄──┼──────► PostgreSQL 16  (volume + nightly pg_dump)
              └────┬────┘   │
                   │        │
              ┌────▼─────┐  │
              │ Go Worker│◄─┴──────► Redis 7  (asynq queues, volume)
              └──────────┘
```

- **Caddy** — one container; serves `frontend/dist` (baked into the image at build time), reverse-proxies `/api/*`, `/calendario*.ics`, `/healthz` to the API container.
- **Go API** — public read endpoints, admin CRUD, auth, .ics generation, outbox writes.
- **Go Worker** — outbox relay + asynq workers (fanout + senders). Same Go module, separate binary/container (`cmd/api`, `cmd/worker`).
- **Monorepo layout:**

```
/backend        Go module: cmd/api, cmd/worker, internal/{http,store,difusion,auth,ics,config}
/frontend       Astro project (public pages, calendar island, admin island)
/deploy         docker-compose.prod.yml, Caddyfile, .env.example
/.github/workflows   ci.yml, deploy.yml
/docs           this spec, implementation plan
/project        original design handoff (reference only, never served)
```

## 5. Data model (PostgreSQL, 8 tables)

Migrations via **goose**. All timestamps `timestamptz`. IDs `uuid` (v4) unless noted.

```sql
liturgical_seasons(
  id smallserial PK, name text, color season_color NOT NULL,  -- enum: verde|violeta|rosa|blanco_oro|rojo
  date_range daterange NOT NULL,
  EXCLUDE USING gist (date_range WITH &&)          -- no overlapping seasons
)  -- season of a date: WHERE date_range @> $1::date  (GIST index)

parish_groups(
  id uuid PK, name text, slug text UNIQUE, is_public bool DEFAULT true, sort int
)

events(
  id uuid PK, title text NOT NULL, slug text, description text, place text,
  starts_at timestamptz NOT NULL, ends_at timestamptz NOT NULL,
  group_id uuid FK→parish_groups, rank event_rank NOT NULL,  -- solemnidad|fiesta|memoria|parroquial
  color_override season_color NULL,                -- only for patronal feasts
  published_at timestamptz NULL,                   -- NULL = draft (invisible + no difusión)
  created_by uuid FK→users, created_at, updated_at
)
CREATE INDEX ON events(starts_at) WHERE published_at IS NOT NULL;  -- the dominant query

channels(
  id uuid PK, kind channel_kind NOT NULL,          -- whatsapp|email
  name text, target text NOT NULL,                 -- WA group JID / email address or list
  group_id uuid NULL FK→parish_groups,             -- NULL = "toda la parroquia" channel
  is_active bool DEFAULT true
)

outbox(
  id bigserial PK, event_id uuid, kind outbox_kind NOT NULL,  -- published|updated|cancelled
  payload jsonb NOT NULL,                          -- event snapshot at write time
  created_at, processed_at timestamptz NULL
)  -- written in the SAME transaction as the event mutation

broadcasts(
  id uuid PK, event_id uuid FK→events, channel_id uuid FK→channels,
  kind outbox_kind NOT NULL, state broadcast_state NOT NULL,  -- queued|sent|failed|dead
  attempt int DEFAULT 0, dedupe_key text UNIQUE NOT NULL,     -- event:channel:kind:rev
  last_error text NULL, sent_at timestamptz NULL, created_at
)

users(
  id uuid PK, email citext UNIQUE NOT NULL, password_hash text,
  display_name text, role user_role NOT NULL,      -- parroco|secretaria
  is_active bool DEFAULT true, created_at
)

sessions(
  id uuid PK, user_id uuid FK→users, token_hash bytea UNIQUE NOT NULL,  -- sha256 of opaque token
  created_at, expires_at timestamptz NOT NULL, revoked_at timestamptz NULL,
  ip inet NULL, user_agent text NULL
)
```

**Deliberately absent:** parishioner accounts, RSVP/attendance, subscriptions, notifications-per-person. The system broadcasts to *channels*; a channel is a string.

**Dominant query** (one query per month view, no N+1):

```sql
SELECT e.*, s.color AS season_color
FROM events e
JOIN liturgical_seasons s ON s.date_range @> (e.starts_at AT TIME ZONE 'America/Mexico_City')::date
WHERE e.published_at IS NOT NULL AND e.starts_at >= $1 AND e.starts_at < $2
ORDER BY e.starts_at;
```

**Seeds:** seasons 2026–2027 (Adviento, Gaudete day, Navidad, Cuaresma, Pascua, Ordinario ranges), the 6 parish groups from the design, initial channels (2 WA stubs + 1 email), one `parroco` user (credentials via env/one-time setup command).

**Timezone:** all business rules (quiet hours, "day" of an event, .ics floating times) use `America/Mexico_City`; configurable via env.

## 6. Backend API (Go, chi, `/api/v1`)

**Public — no auth, `Cache-Control: public, max-age=300`:**

| Route | Returns |
|---|---|
| `GET /api/v1/events?from&to` | Published events in range + `season_color` per event |
| `GET /api/v1/seasons?year` | Season ranges + colors (island paints the grid & banner) |
| `GET /api/v1/groups` | Public groups (filter chips) |
| `GET /calendario.ics` | Full iCalendar feed (`webcal://`), ETag from max `updated_at` |
| `GET /calendario/{group-slug}.ics` | Per-group feed |
| `GET /healthz` | `{"ok":true}` + DB/Redis ping |

**Admin — session cookie required (`HttpOnly, Secure, SameSite=Lax`):**

| Route | Notes |
|---|---|
| `POST /api/v1/auth/login` | email+password → creates session, sets cookie. Rate limit 5/min/IP |
| `POST /api/v1/auth/magic-link` → `/verify` | One-time signed token via SMTP, 15-min expiry |
| `POST /api/v1/auth/logout` | Revokes session |
| `GET /api/v1/auth/me` | Current user + role |
| `GET/POST/PUT /api/v1/admin/events` (+`/{id}`) | CRUD; PUT computes whether the edit is broadcast-worthy |
| `POST /api/v1/admin/events/{id}/publish` \| `/unpublish` | Publish writes outbox `published` in same tx |
| `DELETE /api/v1/admin/events/{id}?notify=bool` | If published & notify → outbox `cancelled` to the channels recorded in `broadcasts` |
| `GET /api/v1/admin/broadcasts?state&event_id` | Difusión log (the "a dónde se fue" screen) |
| `POST /api/v1/admin/broadcasts/{id}/retry` | Re-enqueue a failed/dead broadcast |
| CRUD `/api/v1/admin/channels` | Manage WA/email channels |
| CRUD `/api/v1/admin/users` | **parroco role only**; deactivation revokes all sessions |

Errors everywhere: `{"error":{"code":"...","message":"<Spanish, user-facing>"}}`.

## 7. Difusión engine

**Pipeline:** `COMMIT(event + outbox row)` → relay → fanout → per-channel delivery → settle in `broadcasts`.

1. **Relay** (worker): polls `outbox WHERE processed_at IS NULL` every 2 s with `FOR UPDATE SKIP LOCKED`; enqueues asynq task `difusion:fanout(outbox_id)`; marks processed. Crash-safe; duplicates absorbed downstream.
2. **Fanout**: resolves channels = active channels of the event's group + active group-NULL channels. For each: insert `broadcasts` row with `dedupe_key = eventID:channelID:kind:rev` (`rev` = count of prior broadcast rounds for that event; `ON CONFLICT DO NOTHING` = a retried fanout never double-sends). Enqueues `difusion:deliver(broadcast_id)` on queue `wa` or `mail` with delay `n × 8s ± 3s jitter` (n = channel index).
3. **Delivery**: queue concurrency `wa=1`, `mail=4`. Renders the message template (Spanish copy per kind: published/updated/cancelled) and calls:

   ```go
   type Sender interface {
       Send(ctx context.Context, msg OutboundMessage) error
   }
   // v1 bindings: whatsapp → StubSender (logs, always succeeds, panel shows "SIMULADO")
   //              email    → SMTPSender (SMTP creds from env; TLS)
   ```
4. **Settle**: success → `state=sent, sent_at`. Failure → asynq retry ×5 exponential backoff; exhausted → `state=dead, last_error`. Panel's red numbers read this table.

**Rules that live in the engine (never checkboxes):**

- **Broadcast-worthy edits only:** an `updated` outbox row is written only if `starts_at`, `ends_at`, `place` changed, or the event was cancelled. Title typos and description tweaks save silently.
- **Quiet hours:** nothing delivers 22:00–07:00 (America/Mexico_City). Tasks get `ProcessAt = next 07:00 + stagger`. Rescheduled, never dropped.
- **Edit debounce:** an `updated` fanout waits 10 minutes; further broadcast-worthy edits inside the window collapse into one message (relay skips outbox rows superseded by a newer row for the same event+kind).
- **Cancellation targeting:** `cancelled` goes only to channels that previously received `published`/`updated` for that event (from `broadcasts`), not to current channel config.

## 8. Auth

- Login: argon2id password verify → random 256-bit token → store `sha256(token)` in `sessions` (30-day expiry) → cookie `pc_session` (`HttpOnly, Secure, SameSite=Lax`).
- Middleware: look up token hash, reject expired/revoked, load user+role. `parroco`-only guard on user management.
- Magic link: `POST /auth/magic-link` emails a one-time URL containing an HMAC-signed token (user id + expiry + random jti, 15-min TTL). Single use is enforced by storing the consumed jti in Redis with a 15-min TTL — no extra table. The verify endpoint creates a normal session. Response is always "enlace enviado" (no account enumeration).
- Deactivating a user (`is_active=false`) revokes all their sessions in the same transaction.
- Rate limits: login + magic-link 5/min/IP (in-memory token bucket is fine on one instance).

## 9. Frontend (Astro project)

```
frontend/src/
  pages/index.astro          static public shell: sticky header, season banner,
                             horarios card (from config), grupos section, footer
  pages/admin/[...all].astro mounts Admin island (client:only), React Router inside
  islands/calendar/          Preact island (client:load): month grid, week grid,
                             agenda (phone), filters, day panel, event sheet
  admin/                     React island: RTK store, RTK Query (axios baseQuery),
                             screens = Login, Eventos (list/editor/delete-modal),
                             Difusión, Equipo — 1:1 with the four mockups
  styles/tokens.css          design tokens from mockups (see below)
  lib/api.ts                 typed API client types shared by both islands
```

**Design system (extracted from the handoff bundle, single source of truth in `tokens.css`):**

- Season palette: violeta `#5c3b7a`, rosa `#c06f8d`, oro `#b1872f`/`#8a5a1f`, verde `#2f6b4f`, rojo `#a02f27`, grafito `#6b6255`; paper `#f6f1e6`, card `#fffdf7`, ink `#221d15`, dark panel `#2a241a`, accent `#c9a961`.
- Fonts: Marcellus (display), EB Garamond (body prose), IBM Plex Sans (UI), IBM Plex Mono (labels/times) — self-hosted (fontsource) instead of Google CDN.
- Motion: `--m` multiplier pattern from mockups; respects `prefers-reduced-motion`.

**Calendar island rendering rules (from blueprint §04–05, are requirements):**

1. Season shown as 2 px top bar + ~6 % tint background — never full-bleed color.
2. Month cell: max 2 event chips + `+n más`; no scroll, fixed heights (118 px desktop / 96 px tablet).
3. Order within a day: rank first (solemnidad wins the first line), then time.
4. Rank treatments: solemnidad = full fill + white text + weight 600; fiesta = 12 % tint + 33 % border, colored text; memoria = dot only, no box; parroquial = dashed graphite border, weight 400, outside the liturgical palette.
5. Color is never the only carrier — rank also differs in shape/weight (accessibility).
6. `< 720 px`: month becomes vertical agenda; week becomes day-picker + single-day column. Week view shows red "now" line only in the current week.
7. Group filter chips filter events client-side; liturgy masses render from the events feed (special masses are events; ordinary schedule is the static horarios card).

**Public island data flow:** on mount + on month change → `fetch /api/v1/events?from&to` (+ `/seasons` once). All view state (`view`, `anchor`, `sel`, `filter`, `sheet`) is local component state.

**Admin island:** axios instance with 401 interceptor → redirect `/admin/login`. RTK Query endpoints: `events`, `broadcasts`, `channels`, `users`, `auth`. Difusión screen polls (`pollingInterval: 5s`) while any broadcast is `queued`.

## 10. CI/CD (GitHub Actions)

**`ci.yml`** — on PR + push to `main`:

- `backend`: `go vet`, `golangci-lint run`, `go test ./...` with Postgres 16 + Redis 7 service containers.
- `frontend`: `npm ci`, `astro check` + `tsc`, `vitest run`, `astro build`.

**`deploy.yml`** — on push to `main`, needs CI green, GitHub `production` environment:

1. Build & push to GHCR: `ghcr.io/<owner>/pastoral-backend` (multi-stage Go build, one image with both binaries) and `ghcr.io/<owner>/pastoral-web` (Caddy + baked `frontend/dist`), tagged `latest` + commit SHA.
2. SSH (appleboy/ssh-action) to VPS: `cd /opt/pastoral && docker compose pull && docker compose run --rm migrate && docker compose up -d`.
3. Health check `https://app.jesuslab135.com/healthz`; on failure, redeploy previous SHA tag and mark the run failed.

**GitHub secrets:** `VPS_HOST`, `VPS_USER`, `VPS_SSH_KEY` (deploy key), and nothing else — runtime config (`DB password, session secret, SMTP host/user/pass, TZ, parish name`) lives only in `/opt/pastoral/.env` on the VPS. `deploy/.env.example` documents every variable.

**`docker-compose.prod.yml` services:** `web` (Caddy, ports 80/443), `api`, `worker`, `postgres` (volume `data/postgres`), `redis` (volume, AOF), `migrate` (one-shot goose, profile-gated). No automated backups (user decision); a manual `docker compose exec postgres pg_dump ...` one-liner is documented in the README for on-demand snapshots.

## 11. Testing

- **Go unit:** difusión rules (broadcast-worthy diff, quiet-hours scheduling, debounce collapse, dedupe key), season resolution incl. Gaudete single-day, .ics generation (golden files: UID/SEQUENCE/CANCELLED), auth (argon2, session expiry/revocation).
- **Go integration** (dockertest, real Postgres+Redis): publish → outbox → relay → fanout → stub sender called once per channel → broadcasts settled; retry path; cancellation targets only previously-notified channels.
- **Frontend:** vitest + testing-library on island rules (rank treatments, cell cap, agenda switch, filters); admin flows with msw-mocked API.
- **E2E smoke** (Playwright in CI against compose): public month renders with events; login → create draft → publish → difusión rows appear (SIMULADO + email).

## 12. VPS runbook (post-implementation, step-by-step in the plan)

1. **Verify VPS IP** — user-provided `198.71.54.171` vs DNS A record `app → 74.208.73.82`. Confirm in IONOS panel; fix the Namecheap A record if needed; TTL automatic.
2. **Harden access** — log in once with password, install deploy user + SSH public key, disable password auth, **rotate the password shared in chat**, enable ufw (22/80/443) + fail2ban.
3. **Install Docker** Engine + compose plugin (official apt repo).
4. **Create** `/opt/pastoral/{data/postgres,data/redis,data/caddy}`; copy `docker-compose.prod.yml`, `Caddyfile`, filled `.env`.
5. **GitHub**: add `VPS_HOST/VPS_USER/VPS_SSH_KEY` secrets; push `main` → first deploy builds, migrates, seeds (seasons 2026–27, groups, initial párroco user — password printed once by a `setup` command, then changed on first login).
6. **SMTP**: create/use an IONOS mailbox (or other SMTP); creds → `.env`; send test magic link.
7. **Verify**: `https://app.jesuslab135.com` loads, `/healthz` ok, `.ics` subscribes on a phone (webcal), publish a test event → email arrives, WA row shows SIMULADO.

## 13. Risks & mitigations

| Risk | Mitigation |
|---|---|
| WhatsApp number ban (v2, unofficial provider) | `Sender` interface isolates it; .ics + web remain source of truth; dedicated number, slow warm-up, stub in v1 |
| VPS IP/DNS mismatch | Runbook step 1 verifies before anything else |
| SMTP deliverability (SPF) | Use IONOS SMTP matching the sending domain; runbook includes SPF check |
| Single server, no automated backups (user decision) | The Postgres volume is the only copy of the data; a manual `pg_dump` one-liner is documented for on-demand snapshots. Compose file + migrations in repo make the stack itself rebuildable |
| Leaked VPS password (shared in chat) | Rotate during runbook step 2; SSH-key-only from then on |

## 14. Out of scope / v2 backlog

whatsmeow WhatsApp sender (QR pairing UI, session persistence volume) · weekly grouped boletín digest · computed liturgical calendar (Easter) · Schedule-X/FullCalendar in admin (v1 admin uses list + forms only, per mockups) · recurring events UI (v1: duplicate action on events) · public event detail pages/SEO slugs.
