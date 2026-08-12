# Public Frontend Implementation Plan (Plan 4 of 6)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The public calendar site: static Astro shell (header, season banner, horarios, grupos, footer) plus one Preact island rendering month/week/agenda views of the real `/api/v1` data, pixel-faithful to the design bundle.

**Architecture:** Astro static build with a single Preact island (`client:load`). The island owns all view state locally (no store) and fetches `/api/v1/events` per visible range plus `/seasons` and `/groups` once. Pure date/derivation logic lives in `lib/` so components stay markup. Dev runs against the Go API through a Vite proxy; production serves `frontend/dist` and the API behind one Caddy origin (Plan 6).

**Tech Stack:** Astro 5 + @astrojs/preact + TypeScript strict + fontsource (self-hosted fonts). **No test framework this plan** — the user explicitly wants a fast MVP; verification is `astro check` (types) + `astro build` + eyeballing the dev server against the mock. Vitest and Playwright are deferred to a later hardening pass (recorded in roadmap).

**Spec:** `docs/superpowers/specs/2026-08-10-pastoral-calendar-design.md` §9 (frontend), §10 (CI). **Design source of truth:** `project/Calendario Pastoral - Sitio.dc.html` — read it before implementing any component; every color, font shorthand, radius and padding below comes from it. Match the visual output; do not copy its class-less inline-style structure.

## Global Constraints

- **Commit messages: plain conventional commits. NEVER add `Co-Authored-By` or any mention of Claude/Anthropic/AI. (Explicit user requirement.)**
- `project/` is design reference — **never modify it**.
- All user-facing copy in **Spanish** (copy strings are pinned in the tasks; take longer prose verbatim from the mock).
- Backend CI must not be weakened; the frontend job is additive.
- Fonts self-hosted via fontsource — no Google Fonts CDN (spec §9).
- Respect `prefers-reduced-motion` (mock line 26 pattern).
- `frontend/package-lock.json` is committed; CI uses `npm ci`.
- Node 22 in CI. Run all npm commands from `frontend/`.
- The island renders **only published events from the API**. Ordinary masses are NOT events — they are the static horarios card (spec §9 rule 7). The mock's generated `masses()` and its `CELEB` santoral map are prototype-only data; the real celebration line derives from event ranks (Task 2).

## Design constants (single source, used by every task)

Season palette — key is the `season_color` enum the API returns in `event.color` and `season.color`:

| enum | color | deep | tint | ink | cname |
|---|---|---|---|---|---|
| `violeta` | `#5c3b7a` | `#4a2f63` | `#f0e9f5` | `#5c3b7a` | Violeta |
| `rosa` | `#c06f8d` | `#9d5474` | `#fbeef2` | `#a85170` | Rosa |
| `blanco_oro` | `#b1872f` | `#8a5a1f` | `#fcf5e6` | `#8a5a1f` | Blanco y oro |
| `verde` | `#2f6b4f` | `#255440` | `#eaf2ed` | `#2f6b4f` | Verde |
| `rojo` | `#a02f27` | `#7e241e` | `#f7e9e8` | `#a02f27` | Rojo |

Neutrals: paper `#f6f1e6`, card `#fffdf7`, ink `#221d15`, panel `#2a241a`, panel-fg `#f4efe3`, accent `#c9a961`, graphite `#6b6255`, muted `#5d5548`, gold-link `#8a5a1f`, header-wash `#faf5e9`, footer `#f1ebdc`.

Fonts: Marcellus (display), EB Garamond (prose), IBM Plex Sans (UI), IBM Plex Mono (labels/times).

Grid: `H0=5`, `H1=24`, `ROW=54` (px per hour), month cell min-height 118px desktop / 96px `<1040px`; phone breakpoint `<720px`.

Rank order: `solemnidad > fiesta > memoria > parroquial`.

Rank treatments (blueprint rules, are requirements):
- **solemnidad**: full fill in day color, `#fffdf7` text, weight 600.
- **fiesta**: `color + '20'` background, `color + '55'` border, colored text, weight 600.
- **memoria**: dot only (colored 4px dot, transparent box), weight 500.
- **parroquial**: `1px dashed rgba(107,98,85,.42)` border, graphite text, weight 400 — deliberately outside the liturgical palette.
- Week/agenda blocks: liturgia-group events get solid border + `color+'1c'` tint; every other group gets dashed graphite (`rgba(107,98,85,.4)` border, `rgba(107,98,85,.07)` bg).

---

### Task 1: Scaffold — Astro + Preact + fonts + tokens

**Files:**
- Create: `frontend/package.json`, `frontend/astro.config.mjs`, `frontend/tsconfig.json`, `frontend/src/env.d.ts`, `frontend/src/styles/tokens.css`, `frontend/.gitignore`
- Modify: nothing outside `frontend/`

**Interfaces:**
- Produces: the `frontend/` npm project every later task builds inside; `tokens.css` custom properties (`--paper`, `--card`, `--ink`, `--panel`, `--panel-fg`, `--accent`, `--graphite`, `--muted`, `--gold`, `--wash`, `--footer`, `--m`) consumed by all markup.

- [ ] **Step 1: Write `frontend/package.json`** (npm resolves minors; the lockfile pins):

```json
{
  "name": "pastoral-frontend",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "astro dev",
    "build": "astro build",
    "check": "astro check",
    "preview": "astro preview"
  },
  "dependencies": {
    "@astrojs/check": "^0.9.4",
    "@astrojs/preact": "^4.0.0",
    "@fontsource/eb-garamond": "^5.1.0",
    "@fontsource/ibm-plex-mono": "^5.1.0",
    "@fontsource/ibm-plex-sans": "^5.1.0",
    "@fontsource/marcellus": "^5.1.0",
    "astro": "^5.1.0",
    "preact": "^10.25.0",
    "typescript": "^5.7.0"
  }
}
```

- [ ] **Step 2: Write `frontend/astro.config.mjs`** — Preact integration + dev proxy so the island and the `.ics` links hit the Go API without CORS:

```js
import { defineConfig } from 'astro/config';
import preact from '@astrojs/preact';

export default defineConfig({
  integrations: [preact()],
  vite: {
    server: {
      proxy: {
        '/api': 'http://localhost:8080',
        '/calendario.ics': 'http://localhost:8080',
        '/calendario': 'http://localhost:8080',
      },
    },
  },
});
```

- [ ] **Step 3: Write `frontend/tsconfig.json`**:

```json
{
  "extends": "astro/tsconfigs/strict",
  "compilerOptions": {
    "jsx": "react-jsx",
    "jsxImportSource": "preact"
  }
}
```

- [ ] **Step 4: Write `frontend/src/styles/tokens.css`** — the neutrals table above as `:root` custom properties, the season palette as `--sea-<enum>-{color,deep,tint,ink}` (five enums × four values), the `--m:1` motion multiplier, the four `@keyframes` from mock lines 20–25 (`cpRise`, `cpIn`, `cpGrow`, `cpSheen`, `cpPulse`), the reduced-motion guard from line 26, `*{box-sizing:border-box}`, body defaults (`margin:0; background:var(--paper); color:var(--ink)`), link colors (`a{color:var(--gold)}` hover `var(--ink)`), and `::selection{background:#e9d8ad}`.

- [ ] **Step 5: Write `frontend/.gitignore`** (`dist/`, `node_modules/`, `.astro/`) and `frontend/src/env.d.ts` (`/// <reference types="astro/client" />`).

- [ ] **Step 6: Install and verify** — `cd frontend && npm install && npx astro check && npx astro build` (build fails until a page exists — create a placeholder `src/pages/index.astro` with `<h1>Calendario</h1>` so the pipeline is green; Task 3 replaces it). Expected: build succeeds, `package-lock.json` exists.

- [ ] **Step 7: Commit** — `feat: scaffold astro frontend with preact and design tokens`

---

### Task 2: Typed API client + calendar domain logic

**Files:**
- Create: `frontend/src/lib/config.ts`, `frontend/src/lib/api.ts`, `frontend/src/lib/calendar.ts`

**Interfaces (later tasks import exactly these):**

```ts
// config.ts — must match the backend deployment's PARISH_TZ
export const PARISH_TZ = 'America/Tijuana';
export const PARISH_NAME = 'Cristo de Los Álamos';

// api.ts
export interface ApiGroup { id: string; name: string; slug: string }
export interface ApiEvent {
  id: string; title: string; description: string; place: string;
  starts_at: string; ends_at: string; group: ApiGroup;
  rank: 'solemnidad' | 'fiesta' | 'memoria' | 'parroquial';
  color: SeasonColor;                    // enum name, NOT hex
}
export interface ApiSeason { name: string; color: SeasonColor; start: string; end: string }
export type SeasonColor = 'verde' | 'violeta' | 'rosa' | 'blanco_oro' | 'rojo';
export function fetchEvents(from: string, to: string): Promise<ApiEvent[]>;  // GET /api/v1/events → body.events ?? []
export function fetchSeasons(year: number): Promise<ApiSeason[]>;            // GET /api/v1/seasons?year=
export function fetchGroups(): Promise<ApiGroup[]>;                          // GET /api/v1/groups → body.groups ?? []

// calendar.ts
export interface Palette { color: string; deep: string; tint: string; ink: string; cname: string }
export const SEASON_PALETTE: Record<SeasonColor, Palette>;   // the design-constants table
export const SEASON_BLURBS: Record<string, string>;          // keyed by season NAME prefix, see Step 3
export interface DayEvent {                                   // one event resolved for one day's rendering
  id: string; title: string; place: string; description: string;
  groupSlug: string; groupName: string; rank: ApiEvent['rank'];
  hex: string;              // SEASON_PALETTE[event.color].color
  isLiturgia: boolean;      // group.slug === 'liturgia'
  t: string;                // 'HH:MM' in PARISH_TZ
  min: number;              // minutes since 00:00 parish time
  dur: number;              // minutes
  dateKey: string;          // 'YYYY-MM-DD' parish-local date of starts_at
  lane?: number; lanes?: number;
}
export function dateKey(d: Date): string;                     // local Y-M-D, zero-padded
export function parseKey(k: string): Date;                    // local midnight of 'YYYY-MM-DD'
export function todayKey(): string;                           // today in PARISH_TZ (Intl-based, NOT new Date() fields)
export function monthGridRange(anchor: string): { from: string; to: string }; // 42-cell Sunday-start window
export function monthCellKeys(anchor: string): string[];      // the 42 'YYYY-MM-DD' keys
export function weekKeys(anchor: string): string[];           // 7 keys, Sunday-start
export function toDayEvents(events: ApiEvent[]): Map<string, DayEvent[]>;  // group by dateKey, sorted rank-then-time
export function seasonOf(key: string, seasons: ApiSeason[]): Palette & { name: string };
export function celebrationOf(items: DayEvent[]): DayEvent | null;  // highest-rank non-parroquial, else null
export function layoutLanes(items: DayEvent[]): DayEvent[];    // overlap → lanes (mock laneize/layout port)
export function rangeLabel(anchor: string, view: 'mes' | 'semana'): string; // 'Agosto 2026' | '9–15 de agosto' | '31 ago – 6 sep'
export const MESES: string[]; export const DIAS: string[]; export const DIAS_CORTOS: string[];
```

- [ ] **Step 1: Write `config.ts` and `api.ts`.** Fetchers use relative URLs (`/api/v1/...`), throw on `!res.ok`, and return the unwrapped arrays. No axios — plain `fetch` (axios is for the admin island, Plan 5).

- [ ] **Step 2: Write the date core in `calendar.ts`.** `todayKey()` must resolve in parish time so a visitor in Madrid sees the parish's "hoy":

```ts
export function todayKey(): string {
  return new Intl.DateTimeFormat('en-CA', { timeZone: PARISH_TZ }).format(new Date());
}
```

`toDayEvents` converts each ISO timestamp with one `Intl.DateTimeFormat(PARISH_TZ)` formatter to get `dateKey`, `t`, `min`; `dur = (ends_at − starts_at) / 60000` (clamped ≥ 15). Sort each day: rank order first (use `const RANK_ORDER = { solemnidad: 0, fiesta: 1, memoria: 2, parroquial: 3 }`), then `min` (spec rendering rule 3).

- [ ] **Step 3: Port the mock's pure algorithms** (mock lines 502–535, 633–668 date math): `monthCellKeys` (42 cells from `1 − firstDay.getDay()`), `weekKeys`, `layoutLanes` (the `laneize`/`layout` cluster algorithm verbatim, typed), `seasonOf` (linear scan of fetched ranges, `start ≤ key ≤ end`; **fallback `verde` palette with name `'Tiempo Ordinario'`** when no season covers the date — mirrors the backend's `DefaultSeasonColor`), `rangeLabel` (mock lines 729–733). `SEASON_BLURBS`: the four blurbs from mock lines 371–374 keyed by season-name prefix (`Adviento`, `Gaudete`, `Navidad`, default → Ordinario blurb); `seasonOf` attaches the matching blurb line via `celebrationOf`-free lookup in the banner (Task 4 does the lookup — this file only exports the map).

- [ ] **Step 4: Verify** — `npx astro check` clean. No unit tests (MVP decision); the algorithms are ports of working mock code and get eyeballed in Tasks 5–7.

- [ ] **Step 5: Commit** — `feat: add typed api client and calendar domain logic`

---

### Task 3: Static shell — layout, header, grupos, suscripción, footer

**Files:**
- Create: `frontend/src/layouts/Base.astro`, `frontend/src/components/Header.astro`, `frontend/src/components/Grupos.astro`, `frontend/src/components/Footer.astro`
- Modify: `frontend/src/pages/index.astro` (replace Task 1 placeholder)

**Interfaces:**
- Consumes: `tokens.css`.
- Produces: `index.astro` with a `<div id="isla-calendario">` slot where Task 4 mounts the island. `Header.astro` accepts no props (the season swatch inside the logo is painted by the island via the CSS variable `--season-swatch`, default `var(--sea-verde-color)`).

- [ ] **Step 1: Write `Base.astro`** — `<html lang="es">`, meta viewport, `<title>Calendario Pastoral — Cristo de Los Álamos</title>`, fontsource imports in frontmatter (`@fontsource/marcellus`; `@fontsource/eb-garamond/400.css`, `/500.css`, `/600.css`, `/400-italic.css`; `@fontsource/ibm-plex-mono/400.css`, `/500.css`, `/600.css`; `@fontsource/ibm-plex-sans/400.css`, `/500.css`, `/600.css`, `/700.css`), then `tokens.css`, then `<slot />`.

- [ ] **Step 2: Write `Header.astro`** from mock lines 30–43: sticky, `rgba(246,241,230,.9)` + `backdrop-filter:blur(14px)`, the double-ring logo (30px ring `2px solid #b1872f`, inner disc `background:var(--season-swatch, var(--sea-verde-color))`), name in Marcellus 15px, "CALENDARIO PASTORAL" in Plex Mono 9.5px letterspaced, and the right-aligned `Suscribirse` pill (`#2a241a` bg → `#suscribirse` anchor).

- [ ] **Step 3: Write `Grupos.astro`** from mock lines 338–359: the three-column `auto-fit minmax(270px,1fr)` section — heading + prose, the six static group pills (Liturgia, Catequesis, Pastoral juvenil, Coro, Caridad, Formación), and the dark `#suscribirse` card whose two buttons are real links: `Suscribir calendario` → `href="/calendario.ics"`, `Recibir el boletín` → `href="mailto:avisos@parroquia.mx"`. Copy verbatim from the mock.

- [ ] **Step 4: Write `Footer.astro`** from mock lines 362–367 — but the "Actualizado el…" timestamp is prototype-only: replace that span with the parish line only, and keep `Acceso del equipo →` pointing at `/admin/login` (dead until Plan 5 — acceptable).

- [ ] **Step 5: Rewrite `index.astro`**: `Base` → `Header` → `<div id="isla-calendario"></div>` (island placeholder; Task 4 replaces the div with the component) → `Grupos` → `Footer`.

- [ ] **Step 6: Verify** — `npm run dev`, open `http://localhost:4321`: header, grupos and footer render with the right fonts/colors; `npx astro check && npm run build` green.

- [ ] **Step 7: Commit** — `feat: add public static shell`

---

### Task 4: Island skeleton — banner, toolbar, state, data flow

**Files:**
- Create: `frontend/src/islands/calendar/Calendar.tsx`, `frontend/src/islands/calendar/Banner.tsx`, `frontend/src/islands/calendar/Toolbar.tsx`
- Modify: `frontend/src/pages/index.astro` (mount `<Calendar client:load />` between Header and Grupos)

**Interfaces:**
- Consumes: everything from Task 2.
- Produces: the state contract every view component receives:

```tsx
export interface CalState {
  view: 'mes' | 'semana';
  anchor: string;            // dateKey the visible range derives from
  sel: string;               // selected day (day panel)
  filter: string[];          // active group slugs; empty = all
  sheet: string | null;      // open event id
  dayIx: number;             // phone week view selected column 0–6
}
// Calendar.tsx internal data: seasons: ApiSeason[], groups: ApiGroup[],
// eventsByDay: Map<string, DayEvent[]> (merged cache), loading: boolean.
// Helper passed down: itemsFor(key: string): DayEvent[]  — applies filter.
```

- [ ] **Step 1: Write `Calendar.tsx`.** Hooks only. On mount: `fetchSeasons(currentYear)`, `fetchGroups()`. On mount + whenever the visible range leaves fetched territory: `fetchEvents(from, to)` for `monthGridRange(anchor)` (month view) or the week span (semana), merged into a `Map` cache keyed by dateKey; track fetched ranges in a `Set` of month keys to avoid refetching. `w` (viewport width) via a `resize` listener with `requestAnimationFrame` throttle (mock lines 570–579). Derived: `isPhone = w < 720`, `isTablet = w < 1040`. Renders `Banner` + `Toolbar` + (Task 5/6/7 views — render `null` placeholders this task) inside `<main id="calendario" style="max-width:1360px;margin:0 auto">`. Also sets `document.documentElement.style.setProperty('--season-swatch', todaySeason.color)` so the header logo disc follows the season.

- [ ] **Step 2: Write `Banner.tsx`** from mock lines 45–64: `seasonOf(todayKey(), seasons)` drives `deep` background + `cpSheen` overlay, "TIEMPO LITÚRGICO EN CURSO" kicker, season name in Marcellus clamp(34–54px), blurb from `SEASON_BLURBS`, and the HOY / COLOR cards (`todayLabel` = `día + mes corto`, swatch + `cname`).

- [ ] **Step 3: Write `Toolbar.tsx`** from mock lines 68–85: Mes/Semana segmented control, `‹ Hoy ›` (prev/next shift month or ±7 days; Hoy resets `anchor` and `sel` to `todayKey()`), `rangeLabel`, and the group filter chips built from fetched `groups` (active chip: `#2a241a` bg / `#fbf7ee` fg; inactive: card bg / graphite). Filter toggles mutate `filter` by slug.

- [ ] **Step 4: Verify** — dev server against the running Go API (`docker compose -f docker-compose.dev.yml up -d` + `go run ./cmd/api` with a few published events seeded via the admin API): banner shows the current season, chips list the six real groups, view toggle and month arrows update the range label. `astro check` + build green.

- [ ] **Step 5: Commit** — `feat: add calendar island shell with season banner and toolbar`

---

### Task 5: Month view — grid, day panel, horarios

**Files:**
- Create: `frontend/src/islands/calendar/MonthGrid.tsx`, `frontend/src/islands/calendar/DayPanel.tsx`, `frontend/src/islands/calendar/Horarios.tsx`, `frontend/src/islands/calendar/rankStyles.ts`
- Modify: `frontend/src/islands/calendar/Calendar.tsx` (render them when `view==='mes' && !isPhone`)

**Interfaces:**

```ts
// rankStyles.ts — the blueprint treatments, shared by month/agenda/week
export function celebStyle(e: DayEvent): { bg: string; fg: string; bd: string; fw: number; dot: string };
export function blockStyle(e: DayEvent): { bg: string; fg: string; bd: string; dash: 'solid' | 'dashed'; fw: number };
// MonthGrid props: { anchor, sel, todayK: string, seasons, itemsFor, onPick(key: string): void }
// DayPanel props: { sel, seasons, itemsFor, onOpen(id: string): void }
```

- [ ] **Step 1: Write `rankStyles.ts`** implementing the Rank treatments table from Design constants (solemnidad full fill / fiesta 20-tint+55-border / memoria dot / parroquial dashed graphite; `blockStyle` keys off `isLiturgia`).

- [ ] **Step 2: Write `MonthGrid.tsx`** from mock lines 90–123 + `monthCells` logic (633–668), real-data version: weekday header row (dom in `#a02f27`, rest graphite); 42 cells via `monthCellKeys(anchor)`; per cell: season tint bg (outside-month cells `#f3efe5`), 2px top bar = `celebrationOf(items)?.hex ?? season.color` (outside-month: `rgba(34,29,21,.12)`), day number Plex Mono (today's ring `2px solid #b1872f`, selected ring `2px solid #221d15`), celebration line styled by `celebStyle` when present, then chips: `t.slice(0,5) + ' · ' + title` — **cap: 2 chips, 1 if a celebration is present; overflow renders `+n más`** (rule 2, no scroll); min-height 118/96px. The mock's "n misas" label is dropped (prototype-only data). Cell click → `onPick(key)`.

- [ ] **Step 3: Write `DayPanel.tsx`** from mock lines 157–178: dark `#2a241a` card — season dot + name (of `sel`), day label in Marcellus (`DIAS[dow] + ' N de ' + MESES[m]`), celebration line in accent EB Garamond (`celebrationOf` title, else `Feria · sin celebración propia`), then the day's items (time in accent Plex Mono, title, group name uppercase); empty state `Sin actividades publicadas para este día.` Item click → `onOpen(id)` (sheet opens in week view — set `view:'semana'` too, as the mock does on agenda pick).

- [ ] **Step 4: Write `Horarios.tsx`** from mock lines 179–187 verbatim (static schedule card: Domingo 07:00 · 09:00 · 12:00 · 19:00; Lunes a viernes 07:00; Sábado 07:00 · 19:00 vespertina; the "El calendario siempre manda" prose). Renders beside DayPanel in the month view's right column (mock's flex `1 1 300px` wrap).

- [ ] **Step 5: Verify** — with seeded events: chips capped, celebration styled by rank, tint follows season, clicking a day fills the panel. Resize to ~1000px: cells drop to 96px. `astro check` + build green.

- [ ] **Step 6: Commit** — `feat: add month grid, day panel and horarios card`

---

### Task 6: Week view — time grid, now line, event sheet, leyenda

**Files:**
- Create: `frontend/src/islands/calendar/WeekGrid.tsx`, `frontend/src/islands/calendar/EventSheet.tsx`, `frontend/src/islands/calendar/Leyenda.tsx`
- Modify: `frontend/src/islands/calendar/Calendar.tsx` (render when `view==='semana' && !isPhone`)

**Interfaces:**

```ts
// WeekGrid props: { anchor, todayK, seasons, itemsFor, onOpen(id): void }
// EventSheet props: { event: DayEvent, onClose(): void }   // Calendar resolves sheet id → DayEvent
// Leyenda: no props (static explainer cards, mock lines 309–334)
```

- [ ] **Step 1: Write `WeekGrid.tsx`** from mock lines 196–243 + `weekDays()` logic (671–714): header row (56px gutter; per day: 3px season/celebration bar, dow + number, celebration pill via `celebStyle`; today's column header `#fdf6e8`); body: hour gutter 05:00–23:00 (`ROW=54`), hairlines, per-day columns in season tint; blocks from `layoutLanes(itemsFor(key))` — `top=(min−300)/60·ROW`, `height=max(28, dur/60·ROW−3)`, lane widths `100/lanes%`, styled by `blockStyle`, time shown when `lanes===1 || height≥62`, 3-line clamp on titles; **red now-line** (`#a02f27`, pulsing dot) positioned from current parish time, **rendered only when the visible week contains today** (rule 6) and re-computed every 60s via `setInterval`.

- [ ] **Step 2: Write `EventSheet.tsx`** from mock lines 294–308, real fields: group name kicker in accent, ✕ close, title in Marcellus 25px, then HORA `t–end` / LUGAR `place || 'Templo parroquial'` / rows; add DETALLE row with `description` when non-empty. Drop the mock's hardcoded AVISO/prose lines. `cpIn` entrance animation.

- [ ] **Step 3: Write `Leyenda.tsx`**: the two static cards from mock lines 309–334 (Cómo leer la semana — four explainer rows; keep copy verbatim) — but replace the "El verde no es monotonía" card's hardcoded August prose with the generic closing sentence only if kept; **simplest: omit that second card** (it narrates mock data). Right column of week view = EventSheet (when open) + Leyenda.

- [ ] **Step 4: Verify** — semana view: blocks laid out in lanes without overlap, liturgia solid vs. grupo dashed, now-line only in the current week, clicking a block opens the sheet with real place/description. `astro check` + build green.

- [ ] **Step 5: Commit** — `feat: add week time grid with event sheet and legend`

---

### Task 7: Phone variants — month agenda + week day column

**Files:**
- Create: `frontend/src/islands/calendar/MonthAgenda.tsx`, `frontend/src/islands/calendar/WeekDayCol.tsx`
- Modify: `frontend/src/islands/calendar/Calendar.tsx` (breakpoint switching + responsive paddings)

**Interfaces:**

```ts
// MonthAgenda props: { anchor, todayK, seasons, itemsFor, onOpen(id): void }
// WeekDayCol props: { anchor, dayIx, todayK, seasons, itemsFor, onPickDay(ix): void, onOpen(id): void }
```

- [ ] **Step 1: Write `MonthAgenda.tsx`** from mock lines 126–154 + `agendaDays()` (584–616): one row per day of the anchor month **that has events** (skip empty days); left rail dow/number/HOY tag, right cell in season tint with 3px bar, celebration pill, event rows (time, title, group) styled like the mock's `e.bg/bd/dash`; tapping an event sets `sheet` + `sel` and switches to `semana` (mock behavior).

- [ ] **Step 2: Write `WeekDayCol.tsx`** from mock lines 246–290: 7-button day picker (3px bar, dow, number; active = `#2a241a`/`#fbf7ee`), then the single-day time column (46px gutter, same ROW math as WeekGrid, full-width blocks — no lanes needed but reuse `layoutLanes` anyway), now-line when that day is today.

- [ ] **Step 3: Wire the switching in `Calendar.tsx`** (mock lines 741–754): `mes && !isPhone` → MonthGrid + DayPanel + Horarios; `mes && isPhone` → MonthAgenda + DayPanel + Horarios stacked; `semana && !isPhone` → WeekGrid + sheet/leyenda column; `semana && isPhone` → WeekDayCol + sheet. Responsive paddings: main `20px 16px 56px` phone / `26px 28px 70px` desktop; banner and header paddings per mock lines 748–750.

- [ ] **Step 4: Verify** — narrow the window below 720px: month becomes agenda, week becomes picker + column; widen: grids return; no horizontal scroll at 360px. `astro check` + build green.

- [ ] **Step 5: Commit** — `feat: add phone agenda and single-day week views`

---

### Task 8: Frontend CI, README, roadmap

**Files:**
- Modify: `.github/workflows/ci.yml` (add job — do not touch the `backend` job), `README.md`, `docs/superpowers/plans/2026-08-10-roadmap.md`

- [ ] **Step 1: Add the `frontend` job to `ci.yml`:**

```yaml
  frontend:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: frontend
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: "22"
          cache: npm
          cache-dependency-path: frontend/package-lock.json
      - name: Install
        run: npm ci
      - name: Typecheck
        run: npx astro check
      - name: Build
        run: npm run build
```

- [ ] **Step 2: README** — Development section gains the frontend: `cd frontend && npm install && npm run dev` (→ `http://localhost:4321`, proxies `/api` and the `.ics` feeds to `:8080`); note fonts are self-hosted and there is no frontend test suite yet (deferred with vitest/Playwright to a hardening pass). Repository layout: `frontend/` line updated from "(Plan 4+)" to the real structure.

- [ ] **Step 3: Roadmap** — Plan 4 → **Done** with an execution-notes section (any deviations found while building, the no-tests MVP decision and what the deferred hardening pass owes: vitest island rules, Playwright smoke), Plan 5 → **Next**.

- [ ] **Step 4: Full verify** — `npx astro check && npm run build` green; backend suite still green (`go test ./... -count=1` — untouched, but run it); manual smoke: dev server + API, walk month/week/agenda/filters/sheet against seeded events.

- [ ] **Step 5: Commit** — `docs: document the public frontend and mark plan 4 done`; push `feat/public-frontend`; verify CI green (both jobs).

---

## Self-Review (performed)

1. **Spec §9 coverage:** static shell (header/banner/horarios/grupos/footer) → Tasks 3–5; tokens.css palette+fonts+motion → Task 1; island month/week/agenda/filters/day-panel/sheet → Tasks 4–7; rendering rules 1–7 → rule 1 (2px bar + tint) Tasks 5–6, rule 2 (chip cap, fixed heights) Task 5, rule 3 (rank-then-time order) Task 2, rules 4–5 (rank treatments, shape+weight not just color) Task 5 `rankStyles.ts`, rule 6 (breakpoint + now-line only current week) Tasks 6–7, rule 7 (chips filter client-side; ordinary masses = static card) Tasks 4–5; data flow (fetch on mount + range change, local state) → Task 4; `lib/api.ts` types → Task 2. Admin island and `pages/admin` are Plan 5 (spec splits them); `astro check`+build CI → Task 8.
2. **Deliberate deviations (user-approved direction):** no vitest/Playwright this plan (fast MVP; recorded in roadmap); mock's santoral map and generated masses replaced by rank-derived celebrations from real events; "Actualizado el" footer stamp and the mock-narrating "verde no es monotonía" card dropped; banner rendered inside the island because the season is runtime data on a static build.
3. **Type consistency:** `DayEvent` produced by Task 2 is consumed untouched by Tasks 5–7; `celebStyle`/`blockStyle` defined once in Task 5 and reused by 6–7; state contract (`view/anchor/sel/filter/sheet/dayIx`) defined in Task 4 and matched by every view's props; enum `SeasonColor` matches the API's `season_color` values exactly (`blanco_oro` underscore).
4. **Placeholder scan:** every markup task points at exact mock line ranges plus pinned colors/copy; algorithms are specified by port-source lines plus typed signatures; no TBDs.

