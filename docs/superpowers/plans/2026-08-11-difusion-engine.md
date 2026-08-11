# Difusión Engine Implementation Plan (Plan 3 of 6)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Publishing an event automatically notifies the parish: a worker relays outbox rows, fans out to the event's channels, delivers via a stub WhatsApp sender and a real (or log-fallback) SMTP sender, and settles every attempt in `broadcasts` — with quiet hours, edit debounce, dedupe, and cancellation targeting enforced by the engine, never by checkboxes.

**Architecture:** New package `internal/difusion` (templates, senders, schedule, fanout, deliver, relay) + new binary `cmd/worker` (relay loop + two asynq servers: queue `wa` concurrency 1, queues `mail`+`fanout` concurrency 4). `internal/store` gains broadcasts functions and outbox relay queries. The API only writes outbox rows (Plan 2); the worker owns all sending. Admin gains the difusión log endpoints.

**Tech Stack:** existing module + `github.com/hibiken/asynq` (pin the newest version whose requirement keeps `go.mod` at `go 1.23.x`; check after `go get`, downgrade if needed — asynq shares go-redis v9, already pinned at v9.7.3).

## Global Constraints

- **Commit messages: plain conventional commits. NEVER add `Co-Authored-By`, or any mention of Claude/Anthropic/AI. (Explicit user requirement.)**
- All user-facing text (error JSON, message templates) in **Spanish**; error shape via existing `writeError`.
- Run Go commands from `backend/`; dev services from `docker-compose.dev.yml` (Postgres 5433, Redis 6379).
- **Dependency rule:** `go get <module>@<explicit version>`, then `head -3 go.mod` — directive must stay `1.23.x`.
- Postgres tests via `testdb.New(t)`; Redis via the existing `testRedis`-style helper semantics (fail, don't skip, when `TEST_REDIS_ADDR` is set). `testdb` already truncates `broadcasts` and `outbox`.
- Messages render **only from the outbox snapshot payload**, never the live event row.
- Duplicate enqueues are absorbed by `dedupe_key` (`eventID:channelID:kind:outboxID`), `ON CONFLICT DO NOTHING`.
- A deliver task whose broadcast row no longer exists logs and returns nil (skip, not retry) — this makes stale queue entries self-healing after test truncation or manual cleanup.
- Public API contract and Plan 1/2 tests must stay green after every task.

---

### Task 1: Config — quiet hours and stagger

**Files:** modify `backend/internal/config/config.go` + test.

**Interfaces:**
- New fields: `QuietStart int` (env `QUIET_START`, default `22`), `QuietEnd int` (env `QUIET_END`, default `7`), `StaggerSeconds int` (env `STAGGER_SECONDS`, default `8`). `QuietStart == QuietEnd` means quiet hours disabled (documented semantic; used by tests).
- Add helper `getenvInt(key string, fallback int) int` (non-numeric → fallback).

- [ ] **Step 1:** failing test `TestLoadDifusionDefaults` (defaults 22/7/8; `QUIET_START=9` env override; `QUIET_START=chido` → fallback 22).
- [ ] **Step 2:** implement; run `go test ./internal/config/ -v` → PASS.
- [ ] **Step 3:** commit `feat: add quiet hours and stagger config`.

---

### Task 2: healthz reports Redis

**Files:** modify `backend/internal/http/health.go`, `server.go` (pass `rdb` to `healthHandler`), `health_test.go`.

**Interfaces:** `GET /healthz` → `200 {"ok":true,"redis":true|false}`; `503 db_unavailable` only when Postgres is down. Redis ping uses a 1-second timeout context so a hung Redis cannot slow the endpoint.

- [ ] **Step 1:** extend `TestHealthz` to assert `"redis":true` (test Redis is up); add `TestHealthzRedisDown` using a client pointed at a closed port asserting `200` + `"redis":false`.
- [ ] **Step 2:** implement; full http suite green.
- [ ] **Step 3:** commit `feat: report redis health in healthz`.

---

### Task 3: Seed migration 00005 — initial channels

**Files:** create `backend/internal/store/migrations/00005_seed_channels.sql`; test in `backend/internal/store/seed_test.go`.

```sql
-- +goose Up
INSERT INTO channels (id, kind, name, target, group_id, is_active) VALUES
  ('b1000000-0000-4000-8000-000000000001', 'whatsapp', 'Avisos toda la parroquia', 'PENDIENTE-JID-GRUPO-GENERAL',  NULL, true),
  ('b1000000-0000-4000-8000-000000000002', 'whatsapp', 'Avisos liturgia',          'PENDIENTE-JID-GRUPO-LITURGIA', 'a1000000-0000-4000-8000-000000000001', true),
  ('b1000000-0000-4000-8000-000000000003', 'email',    'Boletín por correo',       'avisos@parroquia.mx',          NULL, true);

-- +goose Down
DELETE FROM channels WHERE id IN (
  'b1000000-0000-4000-8000-000000000001',
  'b1000000-0000-4000-8000-000000000002',
  'b1000000-0000-4000-8000-000000000003');
```
Placeholder targets are deliberate (real JIDs/addresses get set in the admin); the WhatsApp sender is a stub, so nothing is ever sent to them. **Note:** `testdb.New` truncates `channels`, so seeded channels exist only after a fresh migration — tests that need channels seed their own (existing pattern). Scoped `Down` per the Plan-1 review rule.

- [ ] **Step 1: Test the seed where it can actually be observed.** `testdb.New` truncates `channels` on every call, so an in-suite count is always 0 — asserting there would be vacuous. Instead `TestChannelsSeedApplied` (in `seed_test.go`) creates a scratch schema-free check: open `TEST_DATABASE_URL` directly with `sql.Open`, run `store.Migrate` (must not error — proves 00005 applies and is idempotent), then run `testdb.New(t)` as usual so the next test starts clean. The three-row assertion lives in Task 13's fresh-database verification, which migrates an empty database and checks `SELECT count(*) FROM channels WHERE id IN (…) = 3` before any truncation can run.
- [ ] **Step 2:** run `go test ./internal/store/ -count=1` → PASS; commit `feat: seed initial whatsapp stub and email channels`.

---

### Task 4: Broadcasts store

**Files:** create `backend/internal/store/broadcasts.go` + `broadcasts_test.go`.

**Interfaces:**
```go
type Broadcast struct {
    ID        uuid.UUID
    EventID   uuid.UUID
    ChannelID uuid.UUID
    Kind      OutboxKind
    State     string // queued|sent|failed|dead
    Attempt   int
    DedupeKey string
    LastError *string
    SentAt    *time.Time
    CreatedAt time.Time
}
// CreateBroadcast inserts with ON CONFLICT (dedupe_key) DO NOTHING;
// returns inserted=false when the key already exists (idempotent fanout).
func CreateBroadcast(ctx, pool, b Broadcast) (inserted bool, err error)
func GetBroadcast(ctx, pool, id uuid.UUID) (Broadcast, error)
// ListBroadcasts filters optionally by state and event; newest first; includes
// channel name+kind via join (the panel's "a dónde se fue" screen).
type BroadcastRow struct { Broadcast; ChannelName, ChannelKind string }
func ListBroadcasts(ctx, pool, state *string, eventID *uuid.UUID) ([]BroadcastRow, error)
func MarkBroadcastSent(ctx, pool, id uuid.UUID) error                    // state=sent, sent_at=now()
func MarkBroadcastFailed(ctx, pool, id uuid.UUID, msg string, dead bool) error // attempt++, last_error, state failed|dead
func ResetBroadcastForRetry(ctx, pool, id uuid.UUID) error               // failed|dead → queued; else ErrNotRetryable
// ActiveChannelsForGroup: active channels of the group + active group-NULL channels.
func ActiveChannelsForGroup(ctx, pool, groupID uuid.UUID) ([]Channel, error)
// BroadcastRecipients: distinct channel ids that RECEIVED (state=sent) a
// published/updated broadcast for the event — cancellation targeting.
func BroadcastRecipients(ctx, pool, eventID uuid.UUID) ([]uuid.UUID, error)
var ErrNotRetryable = errors.New("broadcast not retryable")
```

- [ ] **Step 1:** failing tests: dedupe (second Create with same key → inserted=false, count stays 1); sent/failed/dead transitions with attempt increments; `ResetBroadcastForRetry` on `sent` → `ErrNotRetryable`; `ActiveChannelsForGroup` returns group's + NULL-group channels but not another group's, and excludes inactive; `BroadcastRecipients` returns only channels whose broadcast reached `sent` (a `failed` one is excluded). Seed channels/users/events with existing helpers; broadcasts need an FK-valid event + channel.
- [ ] **Step 2:** implement (single-statement UPDATEs with `RowsAffected` guards where a state precondition exists).
- [ ] **Step 3:** `go test ./internal/store/ -count=1` green; commit `feat: add broadcasts store with dedupe and retry transitions`.

---

### Task 5: Outbox relay queries

**Files:** modify `backend/internal/store/outbox.go` + create `outbox_test.go`.

**Interfaces:**
```go
type OutboxRow struct {
    ID        int64
    EventID   uuid.UUID
    Kind      OutboxKind
    Payload   OutboxPayload
    CreatedAt time.Time
}
func GetOutboxRow(ctx, pool, id int64) (OutboxRow, error)
// ClaimOutboxBatch runs fn inside one transaction over up to limit unprocessed
// rows locked FOR UPDATE SKIP LOCKED (oldest first). fn returns, per row,
// whether to enqueue (the relay's skip rule lives in the caller); every
// claimed row is marked processed in the same tx. If fn errors, the tx rolls
// back and rows stay unprocessed. Duplicate side effects after a commit
// failure are absorbed downstream by dedupe_key.
func ClaimOutboxBatch(ctx, pool, limit int, fn func(ctx context.Context, row OutboxRow) error) (int, error)
// HasNewerUpdated reports whether a newer `updated` outbox row exists for the
// same event (the relay's debounce-collapse check).
func HasNewerUpdated(ctx context.Context, q Querier, eventID uuid.UUID, afterID int64) (bool, error)
```
(`Querier` is the minimal `QueryRow` interface so it works on tx and pool; define it next to `scanner`.)

- [ ] **Step 1:** failing tests: `ClaimOutboxBatch` processes oldest-first, marks processed, count decreases; a second concurrent claim (two goroutines, small sleep in fn) never double-processes a row (SKIP LOCKED); fn error → nothing marked processed; `HasNewerUpdated` true only when a newer `updated` row for the same event exists.
- [ ] **Step 2:** implement; suite green; commit `feat: add outbox claim and debounce queries`.

---

### Task 6: Message templates (Spanish)

**Files:** create `backend/internal/difusion/template.go` + `template_test.go`.

**Interfaces:**
```go
// package difusion
// Render produces the Spanish subject and plain-text body for a broadcast
// kind from the outbox snapshot. Times format in the parish timezone.
func Render(kind store.OutboxKind, p store.OutboxPayload, loc *time.Location, publicBaseURL string) (subject, body string)
```
- Subjects: `published` → `Nuevo evento: <title>`; `updated` → `Cambio de horario o lugar: <title>`; `cancelled` → `Evento cancelado: <title>`.
- Body lines: title; `📅 <lunes 2 de enero, 15:04>`–`<16:00>` (Spanish weekday/month names via a small lookup table — Go's stdlib has no locale); `📍 <place>` when set; description when set; for `cancelled` a leading `Este evento se canceló.`; final line `Ver calendario: <publicBaseURL>`.

- [ ] **Step 1:** failing tests: each kind contains its subject prefix and the title; times rendered in `America/Mexico_City` (a 19:00Z August event shows `12:00`? no — 19:00 UTC = 12:00 in -07:00 — assert `12:00`); place omitted when empty; Spanish weekday correct for a known date.
- [ ] **Step 2:** implement (lookup tables `[...]string{"domingo",...}`, `enero…diciembre`); commit `feat: add spanish difusion message templates`.

---

### Task 7: Senders

**Files:** create `backend/internal/difusion/sender.go` + `sender_test.go`.

**Interfaces (spec §7 verbatim + bindings):**
```go
type OutboundMessage struct {
    Target  string // WA group JID or email address
    Subject string
    Body    string
}
type Sender interface {
    Send(ctx context.Context, msg OutboundMessage) error
}
// StubWhatsAppSender logs and always succeeds; the panel shows these
// broadcasts as SIMULADO (kind=whatsapp ⇒ simulated in v1).
type StubWhatsAppSender struct{ Sink *log.Logger }
// EmailSender adapts mail.Mailer (SMTP when configured, log otherwise).
type EmailSender struct{ Mailer mail.Mailer }
// SendersFromConfig returns the kind→Sender map the worker uses.
func SendersFromConfig(cfg config.Config) map[string]Sender // keys: whatsapp, email
```

- [ ] **Step 1:** failing tests: stub logs target+subject+`SIMULADO` and returns nil; `EmailSender` forwards to a buffer-backed `LogMailer` with the right to/subject/body; `SendersFromConfig` has both keys.
- [ ] **Step 2:** implement; commit `feat: add stub whatsapp and email senders`.

---

### Task 8: Schedule — stagger and quiet hours

**Files:** create `backend/internal/difusion/schedule.go` + `schedule_test.go`.

**Interfaces:**
```go
// Stagger returns n*base ± up to 3s of jitter, never negative.
func Stagger(n int, base time.Duration) time.Duration
// NextAllowed returns t unchanged when outside quiet hours, else the next
// quietEnd o'clock in loc. quietStart==quietEnd disables quiet hours.
// Handles windows that cross midnight (22→7) and same-day windows (13→15).
func NextAllowed(t time.Time, loc *time.Location, quietStart, quietEnd int) time.Time
```

- [ ] **Step 1:** failing table tests: 23:30 → next 07:00; 03:00 → same-day 07:00; 12:00 → unchanged; boundary 22:00 → 07:00 next day; boundary 07:00 → unchanged; disabled (7,7) → unchanged at 23:30; same-day window (13,15) at 14:00 → 15:00; `Stagger(0)` in `[0,3s]`, `Stagger(3)` in `[21s,27s]` with base 8s, never negative.
- [ ] **Step 2:** implement; commit `feat: add stagger and quiet hours scheduling`.

---

### Task 9: Task definitions + fanout

**Files:** create `backend/internal/difusion/tasks.go`, `fanout.go` + `fanout_test.go`.

**Interfaces:**
```go
// tasks.go
const (
    TypeFanout  = "difusion:fanout"
    TypeDeliver = "difusion:deliver"
    QueueWA     = "wa"
    QueueMail   = "mail"
    QueueFanout = "fanout"
)
type FanoutPayload struct{ OutboxID int64 `json:"outbox_id"` }
type DeliverPayload struct {
    BroadcastID uuid.UUID `json:"broadcast_id"`
    OutboxID    int64     `json:"outbox_id"`
}
func NewFanoutTask(outboxID int64, opts ...asynq.Option) (*asynq.Task, error)
func NewDeliverTask(p DeliverPayload, opts ...asynq.Option) (*asynq.Task, error)
// Enqueuer is what fanout/relay need from *asynq.Client; tests fake it.
type Enqueuer interface {
    Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// fanout.go
// Fanout resolves the recipients of one outbox row, records one broadcast per
// channel (dedupe-idempotent) and enqueues a deliver task per NEW broadcast.
// published/updated → active channels of the event's group + group-NULL ones;
// cancelled → only channels that already RECEIVED this event (BroadcastRecipients).
// Deliver options per channel index n: queue by kind (wa|mail),
// asynq.MaxRetry(5), asynq.ProcessAt(NextAllowed(now.Add(Stagger(n, base))...)).
func Fanout(ctx context.Context, pool *pgxpool.Pool, enq Enqueuer, cfg config.Config, outboxID int64) error
```
Rules inside `Fanout`:
1. Load the row (`GetOutboxRow`); missing row → log + return nil (self-healing skip).
2. For `updated`: if `HasNewerUpdated` → return nil (a fresher round exists; this one collapsed).
3. Resolve channels per kind (above). For `cancelled` with no recipients → return nil (nothing was ever delivered; nothing to retract).
4. For each channel: `CreateBroadcast` with `dedupe_key = fmt.Sprintf("%s:%s:%s:%d", eventID, channelID, kind, outboxID)`; when `inserted`, enqueue the deliver task. Not-inserted means a previous fanout attempt already handled it — skip silently.
5. Enqueue error after insert → return the error (asynq retries the whole fanout; dedupe absorbs the replays).

- [ ] **Step 1:** failing tests with a fake `Enqueuer` (records tasks+opts): published event on the liturgia group with 3 seeded channels (2 matching: liturgia + NULL-group; 1 other-group must NOT receive) → 2 broadcasts + 2 deliver tasks on the right queues; running `Fanout` twice → still 2 broadcasts, 0 new tasks; `cancelled` targets only `sent` recipients (seed one sent + one failed broadcast → 1 new cancelled broadcast); `updated` with a newer updated row → no broadcasts; missing outbox id → nil error, nothing created.
- [ ] **Step 2:** `go get github.com/hibiken/asynq@<latest 1.23-compatible>` + `go mod tidy` + **directive check**; implement; suite green.
- [ ] **Step 3:** commit `feat: add fanout with dedupe, debounce and cancellation targeting`.

---

### Task 10: Deliver + settle

**Files:** create `backend/internal/difusion/deliver.go` + `deliver_test.go`.

**Interfaces:**
```go
// Deliver renders the outbox snapshot and sends via the channel's Sender,
// then settles the broadcast. Designed to be wrapped as the asynq handler.
// Skip-not-retry cases (log + nil): broadcast missing, channel missing or
// inactive, broadcast already sent. Failure: MarkBroadcastFailed with
// attempt++ and dead=exhausted, then return the error so asynq retries.
func Deliver(ctx context.Context, pool *pgxpool.Pool, senders map[string]Sender,
    loc *time.Location, publicBaseURL string, p DeliverPayload, retried, maxRetry int) error
// asynq glue (thin, in worker main): reads retried/maxRetry via
// asynq.GetRetryCount / asynq.GetMaxRetry and calls Deliver.
```

- [ ] **Step 1:** failing tests: success path → sender received rendered subject/body/target and broadcast is `sent` with `sent_at`; failing sender (error stub) with `retried=0,maxRetry=5` → state `failed`, attempt 1, error returned; with `retried=5,maxRetry=5` → state `dead`, error still returned (asynq archives); already-sent broadcast → sender NOT called, nil; missing broadcast → nil; inactive channel → nil + broadcast marked `dead` with explanatory `last_error` (`canal desactivado`).
- [ ] **Step 2:** implement; commit `feat: add delivery with settle and retry accounting`.

---

### Task 11: Relay + worker binary

**Files:** create `backend/internal/difusion/relay.go` + `relay_test.go`, create `backend/cmd/worker/main.go`.

**Interfaces:**
```go
// RelayOnce claims a batch (limit 20) and, per row: updated superseded by a
// newer updated row → processed without enqueue (collapse); otherwise enqueue
// difusion:fanout on QueueFanout — `updated` rows with asynq.ProcessIn(10min)
// (the debounce window), others immediately. Returns rows processed.
func RelayOnce(ctx context.Context, pool *pgxpool.Pool, enq Enqueuer) (int, error)
// RunRelay ticks every 2s until ctx is done.
func RunRelay(ctx context.Context, pool *pgxpool.Pool, enq Enqueuer)
```
`cmd/worker/main.go`:
1. `config.Load()`; fail fast on `PARISH_TZ` and `PublicHost()` (templates embed the link).
2. pgxpool (no migrations — the API owns them; log a fatal if `goose_db_version` is missing so a worker booted against an empty database says why).
3. `asynq.NewClient(asynq.RedisClientOpt{Addr: cfg.RedisAddr})` for the relay.
4. Two `asynq.Server`s: `srvWA` (Concurrency 1, Queues `{wa:1}`) and `srvMail` (Concurrency 4, Queues `{mail:1, fanout:1}`); one `asynq.ServeMux` registering `TypeFanout` → Fanout glue, `TypeDeliver` → Deliver glue.
5. Run relay goroutine + both servers; SIGTERM: stop relay, `srv.Shutdown()` both (asynq waits for in-flight handlers), close pool. Exit 0.

- [ ] **Step 1:** failing relay tests (fake enqueuer): normal row enqueued immediately + processed; `updated` row gets a ProcessIn ≈10min option — recorded `asynq.Option`s expose `Type()`/`Value()`, so assert `opt.Type() == asynq.ProcessInOpt && opt.Value() == 10*time.Minute`; superseded `updated` processed without enqueue; enqueue error → row stays unprocessed (rollback).
- [ ] **Step 2:** implement relay + worker main; `go vet ./...`; `go build ./cmd/worker`.
- [ ] **Step 3:** commit `feat: add outbox relay and difusion worker binary`.

---

### Task 12: Admin broadcasts endpoints

**Files:** create `backend/internal/http/admin_broadcasts.go` + test; modify `server.go`.

**Interfaces (Plan 5's panel consumes these):**
```
GET /api/v1/admin/broadcasts?state=queued|sent|failed|dead&event_id=<uuid>
  → {"broadcasts":[{"id","event_id","channel_id","channel_name","channel_kind",
                    "kind","state","attempt","last_error","sent_at","created_at",
                    "simulated":true|false}]}   // simulated = channel_kind=="whatsapp" (v1 stub)
POST /api/v1/admin/broadcasts/{id}/retry
  → 200 {"broadcast":{...}} | 409 no_reintentable (state not failed|dead) | 404
```
Retry resets the row to `queued` and enqueues a deliver task (outbox id parsed from the last `:` segment of `dedupe_key`; parse failure → 500 with logged detail). `NewRouter` builds its enqueuer with `asynq.NewClientFromRedisClient(rdb)` — reusing the Redis client it already receives, so no second connection pool and **no signature change**. Both roles may retry (the subrouter already requires a session). Invalid `state` filter → 400; invalid `event_id` → 400.

- [ ] **Step 1:** failing handler tests: list filters by state and event; retry on a seeded `failed` broadcast → 200, state `queued`, a task landed on the right queue (inspect via `asynq.NewInspector` against test Redis); retry on `sent` → 409; unknown id → 404.
- [ ] **Step 2:** implement; full http suite green; commit `feat: add difusion log and retry endpoints`.

---

### Task 13: End-to-end integration test + fresh-database gate

**Files:** create `backend/internal/difusion/integration_test.go`.

- [ ] **Step 1: In-process pipeline test.** With test Postgres+Redis: seed a group channel + a NULL-group email channel (fixed UUIDs, own inserts); create+publish an event via `store` (writes the outbox row); `RelayOnce` with a real `asynq.Client`; start `srvMail`+`srvWA` with the real mux wired to buffer-backed senders and `cfg` with `StaggerSeconds=0`, quiet disabled (`QuietStart==QuietEnd`); poll `ListBroadcasts` until both rows are `sent` (timeout 15s); assert both sender buffers contain the event title and the WhatsApp buffer contains `SIMULADO`; assert the outbox row is processed. Then `DeleteEvent(notify=true)` → `RelayOnce` → poll: exactly the two original channels get `cancelled` broadcasts (targeting), and they settle `sent`.
- [ ] **Step 2: Fresh-database gate (the same containers CI uses).** Run the full suite against a brand-new Postgres+Redis pair (docker network, `golang:1.23` container) and additionally assert the 00005 seed: after migrating the empty database, `SELECT count(*) FROM channels WHERE id IN ('b1…01','b1…02','b1…03')` = 3. Lint with `golangci/golangci-lint:v1.62.2`.
- [ ] **Step 3:** commit `test: cover the difusion pipeline end to end`.

---

### Task 14: Docs, roadmap, boot smoke test

**Files:** modify `README.md`, `docs/superpowers/plans/2026-08-10-roadmap.md`.

- [ ] **Step 1: README** — dev now runs two processes (`go run ./cmd/api` + `go run ./cmd/worker`); document `QUIET_START`/`QUIET_END`/`STAGGER_SECONDS` in the env table; note WhatsApp = stub (SIMULADO) and email logs without SMTP.
- [ ] **Step 2: Boot smoke test** — scratch database; api + worker running; setup user; login; create+publish an event via curl; watch the worker log deliver both channels (SIMULADO + logged mail); `psql`: outbox row processed, broadcasts `sent`; `DELETE ?notify=true` → cancelled broadcasts to the same channels. Kill everything, drop the scratch database.
- [ ] **Step 3:** roadmap: Plan 3 → Done (+ execution-notes section), Plan 4 → Next; commit `docs: document the difusion worker and mark plan 3 done`; push branch `feat/difusion-engine`; verify CI green.

---

## Self-Review (performed)

1. **Spec §7 coverage:** relay (2s poll, SKIP LOCKED, crash-safe) → Tasks 5+11; fanout (channel resolution, dedupe `ON CONFLICT`, stagger, per-kind queues) → Task 9; delivery (queue concurrency wa=1/mail=4, Spanish templates, Sender bindings stub-WA + SMTP/log email) → Tasks 6, 7, 10, 11; settle (sent/failed ×5 backoff/dead, panel reads broadcasts) → Tasks 4, 10, 12; engine rules — broadcast-worthy edits already gate outbox writes (Plan 2), quiet hours → Task 8, debounce → Tasks 5+11 (relay skip) + 11 (10-min ProcessIn), cancellation targeting → Tasks 4+9. Plan-3 spec addendum items 1–8 all mapped (worker binary 11, dedupe key 9, quiet config 1, debounce 4→5/11, dev email 7, healthz 2, seeds+endpoints 3/12, delivery data flow 9/10).
2. **Deliberately out of scope:** real WhatsApp provider (v2, behind `Sender`), weekly digest, admin frontend (Plan 5), deploy (Plan 6).
3. **Type consistency:** `store.OutboxKind`/`OutboxPayload` (Plan 2) reused by templates/fanout/deliver; `Enqueuer` satisfied by `*asynq.Client`; `mail.Mailer` reused by `EmailSender`; `config.Config` grows only ints. `NewRouter` signature unchanged (asynq client built internally) — no test-caller churn this plan.
4. **Per-commit CI safety:** asynq arrives in Task 9's commit with no CI change needed (Redis service exists since Plan 2). Every task leaves `go test ./...` green.
5. **Placeholder scan:** Tasks 4, 5, 9–12 specify tests by behavior rather than full listings — they follow the fully-listed patterns of Plans 1–2 (store tests via `testdb`, handler tests via `newRouter`/`loginAs`, fake-enqueuer pattern defined in Task 9). Every interface, rule, queue name, error code and Spanish string is pinned inline. The one deliberately open value is the asynq version: pin the newest release whose `go` directive requirement stays ≤1.23, verified by the mandatory directive check.

