# Admin Frontend Implementation Plan (Plan 5 of 6)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The panel the párroco and secretaría actually use: log in, create/edit/publish/delete events, watch where every aviso went (with retry), manage channels, and manage the team — 1:1 with the four admin mockups against the real `/api/v1` admin API.

**Architecture:** One React island mounted `client:only="react"` from `pages/admin/[...all].astro`, with React Router (basename `/admin`) inside. Redux Toolkit store whose only slice is an RTK Query `createApi` over an axios baseQuery (401 → redirect to `/admin/login`). Screens are thin: all server state lives in RTK Query cache tags; only form state is local. The Preact calendar island is untouched — the two frameworks are separated by `include` globs in `astro.config.mjs`.

**Tech Stack:** React 19 + `@astrojs/react`, `@reduxjs/toolkit` (RTK Query), `react-redux`, `react-router-dom` v6, `axios`. **No new test framework work this plan** (user decision — review and tests happen when the user says so; recorded in roadmap). Verification per task: `astro check` + `npm run build` + the dev server against the running Go API. The existing vitest suite must stay green (`npm test`).

**Spec:** `docs/superpowers/specs/2026-08-10-pastoral-calendar-design.md` §9 (admin island), §8 (auth), §6 (API). **Design source of truth:** `project/Admin Login.dc.html`, `project/Admin Eventos.dc.html`, `project/Admin Difusion.dc.html`, `project/Admin Equipo.dc.html` — read each before implementing its screen. Reuse the palette in `frontend/src/styles/tokens.css`; the mockups introduce no new colors.

## Global Constraints

- **Commit messages: plain conventional commits. NEVER add `Co-Authored-By` or any mention of Claude/Anthropic/AI. (Explicit user requirement.)**
- `project/` is design reference — **never modify it**.
- All user-facing copy in **Spanish**; error messages come from the API's `{"error":{"code","message"}}` body and are shown verbatim.
- **Every `.tsx` file under `src/admin/` starts with the pragma `/** @jsxImportSource react */`** — the project tsconfig says `jsxImportSource: "preact"` for the calendar island, and this line is what keeps `astro check` compiling the admin tree against React. Omitting it produces baffling type errors, not a clear failure.
- Backend: `go.mod` directive stays `go 1.23.x`; never weaken CI; Postgres tests via `testdb.New(t)`.
- Existing checks must stay green after every task: `npm test`, `npx astro check`, `npm run build`, and (after Task 1) `go test ./... -count=1`.
- Run npm commands from `frontend/`, Go commands from `backend/`.
- The backend has exactly **two roles: `parroco` and `secretaria`**. The mockup's third role ("Coordinador de grupo") does not exist in the data model and is **out of scope** (v2 backlog); the Equipo screen shows the two real roles only.

## Reality bindings (mockup → real system; deviations are deliberate)

| Mockup element | Real binding |
|---|---|
| Eventos row "difusión 4/4" | `sent`-state broadcasts ÷ total broadcasts for that event, from `GET /admin/broadcasts` (all rows, joined client-side by `event_id`). Draft → `—`. |
| Difusión states ENTREGADO / EN COLA / FALLÓ / AGRUPADO | `sent` / `queued` / `failed` **and** `dead` / — (AGRUPADO has no backend state; omitted). `dead` renders as `SIN REINTENTOS` in the FALLÓ visual style, with the retry button. |
| "Reglas activas" toggles | The engine's rules are constants, not per-parish settings. Rendered as a **static** informational card (no toggles), same copy. |
| "Salud del proveedor" card | v1 has no WhatsApp provider. Replaced by a SIMULADO notice: WhatsApp broadcasts are simulated (`simulated: true`) until a provider is connected; email is real. |
| "Canales conectados" health bars | No health metrics exist. The card becomes channel **management**: list + edit name/target/active + create — this is where the placeholder JIDs from seed 00005 get fixed, which is the card's real job. |
| Equipo "Invitar a alguien" | `POST /admin/users` (párroco-only). No invitation email exists: the new account signs in via magic link with just their email, so the form explains exactly that. No "INVITACIÓN PENDIENTE" state. |
| Equipo "Registro de actividad" card | No audit-log API. Omitted (v2). |
| Editor "Duración" free text ("1 h 30") | A `<select>` of fixed durations (30/45/60/75/90/120/150/180/240 min, Spanish labels). Free-text parsing is a bug farm. |
| Editor group chip "Toda la parroquia" | Dropped. Every event belongs to one of the six real groups; parish-wide reach comes from NULL-group channels, which the "se difunde a" panel shows. |
| Sidebar user "P. Miguel Ramírez" | `GET /auth/me` (`display_name`, `role`). Role labels: `parroco` → "Párroco", `secretaria` → "Secretaría". |
| Sidebar "Eventos · 36" badge | Count of events in the loaded month. Difusión red dot: shown while any broadcast is `failed`/`dead`. |

## API contract (bound in Task 2's `types.ts`; shapes verified against the Go handlers)

```
POST /api/v1/auth/login {email,password} → 200 {user} | 401 | 429   (sets pc_session cookie)
POST /api/v1/auth/logout → 204
GET  /api/v1/auth/me → 200 {user:{id,email,display_name,role,is_active}} | 401
POST /api/v1/auth/magic-link {email} → 200 (always, no user enumeration)
GET  /api/v1/groups → {groups:[{id,name,slug}]}                      (public, for editor chips)
GET  /api/v1/admin/events?from&to → {events:[{id,title,description,place,starts_at,ends_at,
       group_id,rank,color_override,published_at,cancelled_at}]}
POST /api/v1/admin/events {title,description,place,starts_at,ends_at,group_id,rank,color_override}
       → 201 {event} | 400 (Spanish message)
PUT  /api/v1/admin/events/{id} → 200 {event}
POST /api/v1/admin/events/{id}/publish → 200 {event} | 409 ya_publicado
POST /api/v1/admin/events/{id}/unpublish → 200 {event} | 409 no_publicado
DELETE /api/v1/admin/events/{id}?notify=true|false → 204              (default notify=true)
GET  /api/v1/admin/broadcasts?state=&event_id= → {broadcasts:[{id,event_id,event_title,channel_id,
       channel_name,channel_kind,kind,state,attempt,last_error,sent_at,created_at,simulated}]}
POST /api/v1/admin/broadcasts/{id}/retry → 200 {broadcast} | 409 no_reintentable | 404
GET  /api/v1/admin/channels → {channels:[{id,kind,name,target,group_id,is_active}]}
POST /api/v1/admin/channels · PUT /{id} · DELETE /{id} (409 conflicto if broadcasts reference it)
GET  /api/v1/admin/users → {users:[...]} · POST · PUT /{id} · POST /{id}/activate · /{id}/deactivate
       (all users endpoints párroco-only → 403 for secretaría; deactivating yourself → 400)
```

`event_title` does not exist yet — Task 1 adds it.

---

### Task 1: Backend — event title in the difusión log

The panel's log lines read "Misa patronal solemne → Grupo WA · Coro". The broadcasts endpoint returns `event_id` but not the title, and the event may lie outside whatever month the Eventos screen has loaded, so the client cannot reliably join it. One more column in the existing join fixes it.

**Files:**
- Modify: `backend/internal/store/broadcasts.go` (ListBroadcasts query + `BroadcastRow`)
- Modify: `backend/internal/http/admin_broadcasts.go` (`broadcastJSON` + mapper)
- Modify: `backend/internal/http/admin_broadcasts_test.go` (extend the existing list assertion)

**Interfaces:**
- Produces: `BroadcastRow.EventTitle string`; JSON field `"event_title"`.

- [ ] **Step 1:** In `broadcasts.go`, add `EventTitle string` to `BroadcastRow`; in `ListBroadcasts`, join `events e ON e.id = b.event_id` and select `e.title` (scan into `r.EventTitle`). `broadcasts.event_id` has an FK to `events` with `ON DELETE CASCADE`, so an inner join cannot drop rows.
- [ ] **Step 2:** In `admin_broadcasts.go`, add `EventTitle string \`json:"event_title"\`` to `broadcastJSON`, populate it in `toBroadcastJSON` from `r.EventTitle`. The retry handler builds a `BroadcastRow` by hand — set `EventTitle` there via `store.GetEventAdmin` title lookup? No: keep it cheap — the retry response's `event_title` comes from re-reading the row with `ListBroadcasts`' single-row sibling. Simplest correct version: after the retry succeeds, call `store.ListBroadcasts(ctx, pool, nil, &b.EventID)` and pick the row with `b.ID`; it carries both channel and title. Replace the hand-built row with that.
- [ ] **Step 3:** In `admin_broadcasts_test.go`, extend `broadcastJSONList` with `EventTitle string \`json:"event_title"\`` and assert in `TestListBroadcastsFiltersAndFlagsSimulated` that every returned broadcast has `EventTitle == "Hora santa"` (the fixture's title); in `TestRetryBroadcastRequeuesAndEnqueues` assert the response's `broadcast.event_title` is non-empty.
- [ ] **Step 4:** `go vet ./... && TEST_REDIS_ADDR=localhost:6379 go test ./... -count=1` green (dev compose up).
- [ ] **Step 5:** Commit — `feat: include the event title in the difusion log`

---

### Task 2: React island scaffold — integration, types, RTK Query API, store, router mount

**Files:**
- Modify: `frontend/package.json`, `frontend/astro.config.mjs`
- Create: `frontend/src/pages/admin/[...all].astro`, `frontend/src/admin/types.ts`, `frontend/src/admin/api.ts`, `frontend/src/admin/store.ts`, `frontend/src/admin/dates.ts`

**Interfaces (every later task imports exactly these):**

```ts
// types.ts — mirrors the Go handlers verbatim
export type Role = 'parroco' | 'secretaria';
export type Rank = 'solemnidad' | 'fiesta' | 'memoria' | 'parroquial';
export interface AdminUser { id: string; email: string; display_name: string; role: Role; is_active: boolean }
export interface AdminEvent {
  id: string; title: string; description: string; place: string;
  starts_at: string; ends_at: string; group_id: string; rank: Rank;
  color_override: string | null; published_at: string | null; cancelled_at: string | null;
}
export interface EventInput {
  title: string; description: string; place: string;
  starts_at: string; ends_at: string; group_id: string; rank: Rank; color_override: string | null;
}
export type BroadcastState = 'queued' | 'sent' | 'failed' | 'dead';
export interface Broadcast {
  id: string; event_id: string; event_title: string; channel_id: string;
  channel_name: string; channel_kind: 'whatsapp' | 'email';
  kind: 'published' | 'updated' | 'cancelled'; state: BroadcastState;
  attempt: number; last_error: string | null; sent_at: string | null;
  created_at: string; simulated: boolean;
}
export interface Channel {
  id: string; kind: 'whatsapp' | 'email'; name: string; target: string;
  group_id: string | null; is_active: boolean;
}
export interface ChannelInput { kind: Channel['kind']; name: string; target: string; group_id: string | null; is_active: boolean }
export interface Group { id: string; name: string; slug: string }
export interface ApiError { code: string; message: string }
export function apiError(e: unknown): string;  // digs {error:{message}} out of an RTK Query error; fallback 'Algo salió mal.'

// dates.ts — parish-local form values ↔ API instants
export function parishToISO(date: string, time: string): string;      // '2026-08-22','09:00' → instant ISO
export function isoToParishParts(iso: string): { date: string; time: string }; // inverse, for the editor
export function parishStamp(iso: string): string;                      // 'hoy 11:04' | 'ayer 19:12' | '14 ago 20:30'
export function monthWindow(anchor: Date): { from: string; to: string; label: string }; // 'Agosto 2026'

// api.ts — RTK Query
export const adminApi: created with reducerPath 'adminApi',
  tagTypes ['Me','Events','Broadcasts','Channels','Users'],
  endpoints (hook names in parentheses):
    me (useMeQuery), login (useLoginMutation), logout (useLogoutMutation), magicLink (useMagicLinkMutation),
    groups (useGroupsQuery),
    events({from,to}) (useEventsQuery), createEvent, updateEvent({id,body}), publishEvent(id),
    unpublishEvent(id), deleteEvent({id,notify}),
    broadcasts({state?,eventId?}) (useBroadcastsQuery), retryBroadcast(id),
    channels (useChannelsQuery), createChannel, updateChannel({id,body}), deleteChannel(id),
    users (useUsersQuery), createUser, updateUser({id,body}), setUserActive({id,active});
// store.ts
export const store: configureStore with adminApi.reducer + middleware;
```

- [ ] **Step 1: Dependencies** — from `frontend/`:

```bash
npm install react react-dom @astrojs/react @reduxjs/toolkit react-redux react-router-dom axios
npm install -D @types/react @types/react-dom
```

- [ ] **Step 2: `astro.config.mjs`** — scope the two JSX frameworks so they never see each other's files:

```js
import react from '@astrojs/react';
// ...
integrations: [
  preact({ include: ['**/islands/**'] }),
  react({ include: ['**/admin/**'] }),
],
```

- [ ] **Step 3: `pages/admin/[...all].astro`** — static shell for every admin route (hard refresh on a deep client route needs a Caddy fallback to `/admin` in Plan 6; dev serves it dynamically):

```astro
---
import Base from '../../layouts/Base.astro';
import App from '../../admin/App';

export function getStaticPaths() {
  return [
    { params: { all: undefined } },
    { params: { all: 'login' } },
    { params: { all: 'eventos' } },
    { params: { all: 'difusion' } },
    { params: { all: 'equipo' } },
  ];
}
---

<Base title="Panel pastoral — Cristo de Los Álamos">
  <App client:only="react" />
</Base>
```

(`App.tsx` arrives as a placeholder here — `/** @jsxImportSource react */ export default function App(){return null}` — Task 3 replaces it.)

- [ ] **Step 4: `types.ts` and `dates.ts`.** `parishToISO` uses the two-pass offset trick (no library):

```ts
import { PARISH_TZ } from '../lib/config';

const fmt = new Intl.DateTimeFormat('en-CA', {
  timeZone: PARISH_TZ, year: 'numeric', month: '2-digit', day: '2-digit',
  hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false,
});

/** Milliseconds the parish clock is ahead of UTC at the given instant (negative in Tijuana). */
function offsetAt(instant: Date): number {
  const p = Object.fromEntries(fmt.formatToParts(instant).map((x) => [x.type, x.value]));
  const wall = Date.UTC(+p.year, +p.month - 1, +p.day, +p.hour % 24, +p.minute, +p.second);
  return wall - instant.getTime();
}

/** Interprets a date+time the user typed as parish wall time and returns the instant. */
export function parishToISO(date: string, time: string): string {
  const naive = new Date(`${date}T${time}:00Z`);          // pretend it's UTC
  const once = new Date(naive.getTime() - offsetAt(naive)); // correct by the offset there
  return new Date(naive.getTime() - offsetAt(once)).toISOString(); // second pass rides out DST edges
}
```

- [ ] **Step 5: `api.ts`.** The axios baseQuery and the 401 rule:

```ts
const http = axios.create({ baseURL: '/api/v1' });
http.interceptors.response.use(undefined, (err) => {
  const s = err.response?.status;
  // A dead session anywhere in the panel lands on the login screen — except
  // on auth calls themselves, where 401 is the screen's own business.
  if (s === 401 && !err.config.url?.startsWith('/auth') && location.pathname !== '/admin/login') {
    location.assign('/admin/login');
  }
  return Promise.reject(err);
});

const axiosBaseQuery =
  (): BaseQueryFn<{ url: string; method?: Method; data?: unknown; params?: unknown }, unknown, { status?: number; data?: { error?: ApiError } }> =>
  async ({ url, method = 'GET', data, params }) => {
    try {
      const res = await http.request({ url, method, data, params });
      return { data: res.data };
    } catch (e) {
      const err = e as AxiosError<{ error: ApiError }>;
      return { error: { status: err.response?.status, data: err.response?.data } };
    }
  };
```

Tag rules: every event mutation invalidates `['Events', 'Broadcasts']` (publishing writes outbox rows that become broadcasts); `retryBroadcast` invalidates `['Broadcasts']`; channel mutations `['Channels']`; user mutations `['Users']`; `login`/`logout` invalidate `['Me']` (logout also `dispatch(adminApi.util.resetApiState())` in its `onQueryStarted`). Each `query` unwraps the envelope (`transformResponse: (r: {events: AdminEvent[]|null}) => r.events ?? []` and likewise for every list).

- [ ] **Step 6: Verify** — `npx astro check && npm test && npm run build` all green (the island is an empty `App`, but the whole toolchain — React include globs, pragmas, static paths — is proven).
- [ ] **Step 7: Commit** — `feat: scaffold the admin island with rtk query over the admin api`

---

### Task 3: Shell — router, session guard, sidebar, phone header, shared UI

**Files:**
- Create: `frontend/src/admin/ui.tsx`, `frontend/src/admin/Shell.tsx`
- Replace: `frontend/src/admin/App.tsx`

**Interfaces:**

```tsx
// ui.tsx — the mockups' shared primitives, styled once (all take style overrides)
export function Kicker({children}): small-caps gold label (mock: 9.5px Plex Mono .2em #8a5a1f)
export function H1({children}): Marcellus clamp(28–40px) heading
export function Field({label, children}): label row (9.5px mono uppercase graphite) + control
export function TextInput(props: JSX.InputHTMLAttributes): the bordered card input w/ green focus ring
export function PrimaryButton / DangerButton / GhostButton(props): the three mock button styles
export function Chip({on, children, onClick}): pill toggle (dark when on, card when off)
export function StatCard({value, color, children}): the 30px Marcellus number cards
export function ErrorBox({children}): red-tinted EB Garamond error panel (login mock line 84)
export function Tag({bg, fg, children}): tiny mono state tag
// Shell.tsx
export default function Shell(): sidebar (>=900px) or sticky phone header (<900px) + <Outlet/>
// App.tsx
export default function App(): Provider(store) > BrowserRouter(basename='/admin') > Routes
```

- [ ] **Step 1: `ui.tsx`.** Port the recurring primitives from the mockups (they repeat identical inline styles dozens of times; write them once). Exact values from `Admin Eventos.dc.html`: inputs lines 184/190, buttons 82/136/246, chips 106/211, stat cards 86–101, tags 127.

- [ ] **Step 2: `App.tsx`:**

```tsx
/** @jsxImportSource react */
// RequireSession: useMeQuery(); while loading → null; error → <Navigate to="/login" replace/>;
// data → <Outlet context={me}/> (context carries the user to Shell and screens).
<Provider store={store}>
  <BrowserRouter basename="/admin">
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route element={<RequireSession />}>
        <Route element={<Shell />}>
          <Route index element={<Navigate to="/eventos" replace />} />
          <Route path="/eventos" element={<EventosList />} />
          <Route path="/eventos/nuevo" element={<EventoEditor />} />
          <Route path="/eventos/:id" element={<EventoEditor />} />
          <Route path="/difusion" element={<Difusion />} />
          <Route path="/equipo" element={<RequireParroco><Equipo /></RequireParroco>} />
          <Route path="*" element={<Navigate to="/eventos" replace />} />
        </Route>
      </Route>
    </Routes>
  </BrowserRouter>
</Provider>
```

`RequireParroco` reads the outlet context; a secretaría lands back on `/eventos`. For this task, `EventosList`/`EventoEditor`/`Difusion`/`Equipo`/`Login` are stub components rendering their `<H1>` only — each later task replaces one.

- [ ] **Step 3: `Shell.tsx`** from `Admin Eventos.dc.html` lines 31–72: dark `#2a241a` sidebar (236px, sticky, full height) — logo block, `NavLink`s Eventos (with month event count from `useEventsQuery`), Difusión (red pulsing dot when any broadcast `failed`/`dead`), Equipo y permisos (**hidden unless `me.role === 'parroco'`**), "Ver sitio público ↗" → `/` (plain `<a>`); footer block with initials avatar (`display_name` → two initials), name, role label, CERRAR SESIÓN → `useLogoutMutation` then `location.assign('/admin/login')`. Below 900px (same `requestAnimationFrame` resize listener pattern as the calendar island): the sticky phone header with horizontal chip nav instead.

- [ ] **Step 4: Verify** — dev server: `/admin` with no session redirects to `/admin/login` (stub); after `curl` login… simpler: log in from the Login stub in Task 4. For now assert redirect-to-login works and `astro check`/`npm test`/build are green.
- [ ] **Step 5: Commit** — `feat: add admin shell with router and session guard`

---

### Task 4: Login screen

**Files:**
- Create: `frontend/src/admin/Login.tsx` (replaces stub import in `App.tsx`)

Port `Admin Login.dc.html` in full: split layout ≥860px (left: green gradient panel with sheen, kicker "Hoy · <parish date>", the Marcellus display line and prose — copy verbatim from mock lines 42–48; the season pill may show the static "Tiempo Ordinario · verde" only if the public seasons fetch is not worth it here — **fetch nothing: hardcode nothing; drop the two pills** and keep the prose block, they're decorative), right: the form card.

Behavior:
- If `useMeQuery` already succeeds → `<Navigate to="/eventos" replace />` (a logged-in visitor to /admin/login skips the form).
- Email + password fields (mock lines 71–81, with MOSTRAR/OCULTAR toggle flipping input type), client-side guard: both empty → `ErrorBox` "Escribe tu correo parroquial y tu contraseña para continuar." without hitting the API.
- Submit → `useLoginMutation`; 401 → show the API's message via `apiError`; 429 → likewise (rate limiter's message). Success → `navigate('/eventos')`.
- "O BIEN" divider + magic link button → `useMagicLinkMutation(email)`; needs a non-empty email (else `ErrorBox` "Escribe tu correo para mandarte el enlace."). After success the button label becomes "Enlace enviado a tu correo" and disables (mock line 139). In dev the link lands in the API log — the plan's verification uses that.
- Footer links: "El párroco puede restablecerlo desde Equipo y permisos" (plain text, no link — the reader isn't logged in) and "← Volver al calendario público" → `/`.

- [ ] **Step 1:** Implement as above.
- [ ] **Step 2: Verify against the running API** — wrong password shows the Spanish 401 message; right credentials land on `/eventos` with the sidebar showing the user's name; magic link from the API log signs in (`/api/v1/auth/magic-link/verify?token=…` then visit `/admin`); `astro check` + `npm test` + build green.
- [ ] **Step 3: Commit** — `feat: add admin login with password and magic link`

---

### Task 5: Eventos — list, stats, filters

**Files:**
- Create: `frontend/src/admin/eventos/EventosList.tsx`, `frontend/src/admin/eventos/casts.ts`

**Interfaces:**

```ts
// casts.ts — the difusión column, shared with the editor's delete modal
export interface Cast { sent: number; total: number; failed: boolean }
export function castsByEvent(broadcasts: Broadcast[]): Map<string, Cast>;
// EventosList consumes useEventsQuery(monthWindow), useBroadcastsQuery(), useGroupsQuery()
```

Port `Admin Eventos.dc.html` lines 76–171 (lista mode), bound to reality:

- Header: `Kicker` "<Mes Año> · Eventos", `H1` "Eventos del mes", month nav `‹ Hoy ›` (local `anchor: Date` state driving `monthWindow`), "+ Nuevo evento" → `/eventos/nuevo`.
- Stat cards from the loaded month + broadcasts: publicados (green), borradores (gold, `published_at === null`), difusiones con fallo (red, events whose `Cast.failed`), and the fourth card: next `solemnidad` with `starts_at > now` in the window → "«title» en N días" (gold wash card); none → card hidden.
- Tabs `Todos · n / Publicados · n / Borradores · n / Con fallo · n` filtering client-side.
- Desktop ≥900px: the 5-column grid rows (mock lines 112–138) — day number colored by rank (`solemnidad|fiesta` → gold, else graphite; a `color_override` wins), dow+time (via `parishStamp` parts), title + BORRADOR tag, group name (from `useGroupsQuery` by `group_id`), rank in small caps, cast `sent/total` dot (green all-sent, red when `failed`, `—` for drafts), Editar → `/eventos/{id}`, ✕ → `/eventos/{id}` with `location.state = { delete: true }` (the editor opens its modal). Phone: the stacked cards (mock lines 141–159).
- Empty filter state: mock lines 163–166 verbatim.
- Footer prose (mock line 170) with the link pointing to `/difusion`.
- Cancelled events (`cancelled_at !== null`) render with a `CANCELADO` red tag instead of BORRADOR and no action buttons except ✕? No — a cancelled event is a tombstone; show the tag and **only** Editar disabled. Simplest true rule: exclude cancelled events from Todos/Publicados/Borradores and give them their own client-side tab `Cancelados · n` when any exist.

- [ ] **Step 1:** Implement `casts.ts` + screen.
- [ ] **Step 2: Verify** — with the seeded dev data: counts match, tabs filter, casts show `n/n` green (the worker delivered them in Plan 3's seed session) or `—` after creating a draft; `astro check` + `npm test` + build green.
- [ ] **Step 3: Commit** — `feat: add admin eventos list with stats and difusion counts`

---

### Task 6: Evento editor — create, edit, publish, draft, unpublish

**Files:**
- Create: `frontend/src/admin/eventos/EventoEditor.tsx`, `frontend/src/admin/eventos/form.ts`

**Interfaces:**

```ts
// form.ts
export interface EventForm { title: string; date: string; time: string; durationMin: number;
  place: string; group_id: string; rank: Rank; description: string }
export const DURATIONS: Array<{ min: number; label: string }>; // 30 '0 h 30' … 240 '4 h'
export function toInput(f: EventForm): EventInput;      // parishToISO + ends = starts + durationMin
export function fromEvent(e: AdminEvent): EventForm;    // isoToParishParts + duration from the pair
export function validate(f: EventForm): string | null;  // Spanish message or null (title, date, time, group required)
```

Port `Admin Eventos.dc.html` lines 173–257 (editor mode):

- `/eventos/nuevo` → empty form (defaults: today's date, `12:00`, 90 min, place "Templo parroquial", rank `parroquial`, first group). `/eventos/:id` → `fromEvent` of the row from the events cache (`useEventsQuery` for the event's month via a lazy fetch if not cached — simplest correct: `useEventsQuery` for the *current* anchor month won't have other months' events, so the editor does its own `GET /admin/events/{id}`… **add the `getEvent(id)` endpoint to `api.ts`** — the handler exists (`GET /api/v1/admin/events/{id}` → `{event}`), Task 2's endpoint list gains it here).
- Form left column: título, fecha/hora/duración grid, lugar, grupo chips (six real groups, green when selected), rango segmented control (4 options, dark when selected), descripción textarea + the "Esto es lo que la gente leerá en WhatsApp" hint (mock line 228).
- Right column "Al guardar se difunde a" (mock lines 233–249), computed from `useChannelsQuery`: the active channels whose `group_id` is null or equals the selected group — solid border + green dot; other channels dashed at 60% opacity. Meta line: `whatsapp` → "WhatsApp · SIMULADO en v1"; `email` → the target address.
- Action buttons by status:
  - new: **Publicar y difundir** (create → publish → navigate `/eventos`), **Guardar como borrador** (create → navigate).
  - draft: **Publicar y difundir** (update → publish), **Guardar borrador** (update).
  - published: **Guardar cambios** (update; the engine decides whether the edit is broadcast-worthy), **Despublicar** (unpublish; ghost button; confirmation not needed — it queues a cancellation, say so in the caption under it: "Despublicar lo quita de la web y avisa la cancelación a quienes lo recibieron.").
  - The mock's caption (line 248) stays for draft/new modes.
- Validation errors and API errors render in an `ErrorBox` above the buttons (`apiError`).
- "Zona delicada" card (mock lines 250–254) only when editing an existing event → opens the Task 7 modal.
- "← VOLVER A EVENTOS" → `/eventos`.

- [ ] **Step 1:** Implement `form.ts` + editor + the `getEvent` endpoint.
- [ ] **Step 2: Verify end to end** — create a draft (appears in Borradores, public site does NOT show it), publish it (public site shows it after refetch; a broadcast row appears in `/difusion` within ~2s if the worker is running), edit its time (the engine debounces — a `queued` updated broadcast appears after the relay tick), unpublish (cancellation broadcasts). `astro check` + `npm test` + build green.
- [ ] **Step 3: Commit** — `feat: add admin evento editor with publish and draft flows`

---

### Task 7: Delete modal

**Files:**
- Create: `frontend/src/admin/eventos/DeleteModal.tsx`
- Modify: `frontend/src/admin/eventos/EventoEditor.tsx` (mount + `location.state.delete` auto-open)

Port `Admin Eventos.dc.html` lines 260–281: red-banded confirm card. Props: `event: AdminEvent`, `cast: Cast | undefined`, `onClose()`. Copy binds to reality: published with `total > 0` → "Este evento ya se difundió a {total} canales…"; draft → "Este evento es un borrador; nunca se difundió." The **Avisar la cancelación** checkbox (default on) renders only for published events — a draft's delete always hard-deletes (`notify=false`; the backend hard-deletes unpublished rows regardless, but the UI should not promise an aviso it won't send). Confirm → `deleteEvent({id, notify})` → navigate `/eventos`. Conservarlo → close.

- [ ] **Step 1:** Implement; the editor's ✕ route-state opens it on mount.
- [ ] **Step 2: Verify** — deleting a published event with the box checked produces `cancelled` broadcasts to exactly the channels that got the original (watch `/difusion`); with it unchecked the event vanishes silently; a draft's modal shows no checkbox. Checks green.
- [ ] **Step 3: Commit** — `feat: add delete confirmation with cancellation notice`

---

### Task 8: Difusión — log, retry, polling, channel management

**Files:**
- Create: `frontend/src/admin/difusion/Difusion.tsx`, `frontend/src/admin/difusion/ChannelsCard.tsx`

Port `Admin Difusion.dc.html`, bound per the Reality-bindings table:

- Header kicker/H1/prose verbatim (lines 75–77).
- Stat cards from `useBroadcastsQuery()`: entregados (state `sent`, `sent_at` in the current month), en cola (`queued`), fallidos (`failed` + `dead`, red card), and the fourth card: `8 s / retraso entre grupo y grupo` is the engine's stagger — render it static with `STAGGER_SECONDS`' default (`8 s`) and caption "retraso entre canal y canal".
- **Polling:** `useBroadcastsQuery(undefined, { pollingInterval: anyQueued ? 5000 : 0 })` — spec §9 requires the 5s poll only while something is queued.
- Log tabs Todo/Fallos/En cola/Entregados (client-side; Fallos = `failed`+`dead`). Rows (mock lines 108–126): left border + row tint by state (`sent` green/card, `queued` gold/`#fdf6e8`, `failed|dead` red/`#fbeeec`), `event_title`, state `Tag` (ENTREGADO/EN COLA/FALLÓ/SIN REINTENTOS), channel line "`channel_name` · SIMULADO" when `simulated`, stamp via `parishStamp(sent_at ?? created_at)`. For `failed|dead`: the `last_error` note box + **Reintentar ahora** → `useRetryBroadcastMutation` (409 → `apiError` inline) + "Revisar el canal" scrolling to the channels card (`href="#canales"`).
- Empty-filter state verbatim (lines 128–131).
- `ChannelsCard` (`id="canales"`): each channel — kind icon text (`WA` / `✉`), name, target (mono, the part the párroco must fix from the seed placeholders), group name or "Toda la parroquia" (null group), active toggle → `updateChannel`; **Editar** flips the row to an inline form (name, target, kind select, group select from `useGroupsQuery` + "Toda la parroquia" = null, activo checkbox) with Guardar/Cancelar; "+ Nuevo canal" opens the same form empty → `createChannel`; delete button → `deleteChannel`, and a 409 shows the API's "desactívalo en su lugar" message inline.
- "Reglas activas" dark card: static copy of the four rules (mock lines 285–288 text, no toggles).
- SIMULADO notice card (replaces "Salud del proveedor"): "El WhatsApp de v1 es simulado: el panel registra a dónde *iría* cada aviso sin enviarlo. El correo sí sale. Un proveedor real se conecta detrás de la misma interfaz sin tocar el motor."

- [ ] **Step 1:** Implement both files.
- [ ] **Step 2: Verify** — with the worker running: publish an event from the panel and watch the rows move `EN COLA → ENTREGADO` under the 5s poll without a manual refresh; force a failure (deactivate a channel, publish, reactivate) and retry it; edit a channel's target. Checks green.
- [ ] **Step 3: Commit** — `feat: add difusion log with retry and channel management`

---

### Task 9: Equipo — team management (párroco only)

**Files:**
- Create: `frontend/src/admin/Equipo.tsx`

Port `Admin Equipo.dc.html`, two-role reality:

- Kicker/H1/prose verbatim (lines 73–75).
- "+ Invitar a alguien" (párroco sees this screen at all) opens the inline card (mock lines 85–109): nombre, correo, papel chips — **Párroco (violeta `#5c3b7a`) / Secretaría (verde `#2f6b4f`)** only — and the caption: "No se manda invitación: con el correo dado de alta, la persona entra con un enlace de acceso desde la pantalla de entrada." Enviar → `createUser` (no password → magic-link-only) → list refetches; 409 `correo_duplicado` shows inline.
- Count line: "N cuentas activas" (+ " · M desactivadas" when any).
- Person cards (mock lines 111–139): initials avatar + left accent in role color, name, email, role `Tag`; capability rows from the real matrix — Párroco: Crear y editar ✓ / Publicar y difundir ✓ / Ver difusión ✓ / Administrar equipo ✓; Secretaría: ✓ ✓ ✓ –. Role change: the role tag becomes a two-chip toggle in an "Editar" mode per card → `updateUser`. **Quitar acceso** → `setUserActive({id, active:false})`; deactivated cards render at 60% opacity with a **Reactivar** button; the self-deactivation 400 shows the API's message. `removable` = `user.id !== me.id`.
- Right column: "Qué puede hacer cada papel" card with the **two** real roles (drop the coordinator block; its explanation is v2), and the dark "Por qué son tan pocas cuentas" card verbatim (lines 161–165). The activity-log card is omitted.

- [ ] **Step 1:** Implement. Log in as a secretaría (create one first) and confirm the nav hides Equipo and direct navigation bounces to `/eventos` (the API would 403 anyway).
- [ ] **Step 2: Verify** — create an account, sign in with its magic link in a private window, deactivate it from the párroco session and watch the other session's next request land on `/admin/login` (Plan 2 revokes sessions in-tx). Checks green.
- [ ] **Step 3: Commit** — `feat: add equipo screen with account management`

---

### Task 10: Docs, roadmap, boot smoke pass

**Files:**
- Modify: `README.md`, `docs/superpowers/plans/2026-08-10-roadmap.md`

- [ ] **Step 1: README** — Development section: the admin panel lives at `http://localhost:4321/admin` (same dev processes as before); Admin API section gains "or use the panel". Note the deferred admin tests (vitest for screens, Playwright E2E) next to the existing deferral note. Repository layout: `frontend/src/admin/` line.
- [ ] **Step 2: Full pass** — `go test ./... -count=1` (Task 1 touched backend), `npm test`, `astro check`, `npm run build`; then the boot smoke: fresh browser, login → create → publish → see it on the public site and in Difusión (SIMULADO + mail in the worker log) → edit time → debounced update → delete with aviso → cancellation rows → team: create secretaría, magic-link login, deactivate.
- [ ] **Step 3: Roadmap** — Plan 5 → **Done** with execution notes (deviations from the Reality-bindings table that changed during execution, the deferred-tests ledger), Plan 6 → **Next**.
- [ ] **Step 4: Commit** — `docs: document the admin panel and mark plan 5 done`; push `feat/admin-frontend`; verify both CI jobs green.

---

## Self-Review (performed)

1. **Spec §9 admin coverage:** `pages/admin/[...all].astro` client:only + React Router → Tasks 2–3; RTK store + RTK Query (axios baseQuery) with endpoints events/broadcasts/channels/users/auth → Task 2; 401 interceptor → `/admin/login` → Task 2; four screens 1:1 with mockups → Tasks 4 (Login), 5–7 (Eventos list/editor/delete-modal), 8 (Difusión), 9 (Equipo); Difusión 5s polling while queued → Task 8. §8 auth flows (password, magic link, logout, session revocation) → Tasks 4, 9. §6 admin API consumed in full — every endpoint the backend exposes has a screen; `event_title` gap closed by Task 1.
2. **Deliberate deviations** are centralized in the Reality-bindings table and each is grounded in what the backend actually has; the coordinator role, audit log, provider health, and rule toggles go to the v2 backlog via the roadmap notes.
3. **Type consistency:** `types.ts` mirrors the Go JSON verbatim (checked against `admin_events.go`, `admin_broadcasts.go`, `admin_channels.go`, `admin_users.go`, `auth.go`); `Cast` produced in Task 5 is consumed by Task 7's modal; `EventForm`/`toInput` (Task 6) feed `createEvent`/`updateEvent` from Task 2; `parishToISO`/`isoToParishParts` defined once in Task 2 and used by Tasks 5–8; hook names follow RTK Query's `use<Name><Query|Mutation>` convention throughout.
4. **Placeholder scan:** every screen task points at exact mockup line ranges plus pinned copy and state→style maps; the two nontrivial algorithms (`parishToISO`, axios baseQuery) are written out; no TBDs. The one deliberately open point is styling minutiae already carried by `ui.tsx` + the mock references.
5. **Per-task green:** Task 1 keeps the backend suite green on its own; Tasks 2–9 each end with `astro check` + `npm test` + build; nothing touches the Preact island or the public site.
