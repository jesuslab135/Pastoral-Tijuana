// Calendar domain logic: dates in parish time, season resolution, and the
// overlap layout the week grid needs. Kept apart from the components so the
// views stay markup.

import type { ApiEvent, ApiSeason, Rank, SeasonColor } from './api';
import { PARISH_TZ } from './config';

export interface Palette {
  color: string;
  deep: string;
  tint: string;
  ink: string;
  cname: string;
}

export const SEASON_PALETTE: Record<SeasonColor, Palette> = {
  violeta: { color: '#5c3b7a', deep: '#4a2f63', tint: '#f0e9f5', ink: '#5c3b7a', cname: 'Violeta' },
  rosa: { color: '#c06f8d', deep: '#9d5474', tint: '#fbeef2', ink: '#a85170', cname: 'Rosa' },
  blanco_oro: { color: '#b1872f', deep: '#8a5a1f', tint: '#fcf5e6', ink: '#8a5a1f', cname: 'Blanco y oro' },
  verde: { color: '#2f6b4f', deep: '#255440', tint: '#eaf2ed', ink: '#2f6b4f', cname: 'Verde' },
  rojo: { color: '#a02f27', deep: '#7e241e', tint: '#f7e9e8', ink: '#a02f27', cname: 'Rojo' },
};

export const GRAPHITE = '#6b6255';

// Season blurbs are editorial copy, matched by name so a season renamed in the
// database still finds its paragraph (or falls back to the ordinary one).
const BLURBS: Array<[RegExp, string]> = [
  [
    /gaudete/i,
    'El tercer domingo de Adviento la Iglesia se alegra: el rosa suaviza el violeta por un solo día.',
  ],
  [
    /adviento/i,
    'Cuatro semanas de espera. La parroquia se prepara con la Novena de Aguinaldo, las posadas y las confesiones comunitarias.',
  ],
  [
    /navidad|epifan/i,
    'De la Nochebuena al Bautismo del Señor. Octava de Navidad, Sagrada Familia y Epifanía.',
  ],
  [
    /cuaresma/i,
    'Cuarenta días de conversión. Vía crucis los viernes, confesiones comunitarias y el camino hacia la Pascua.',
  ],
  [
    /pascua/i,
    'Cincuenta días de alegría, del Domingo de Resurrección a Pentecostés. La Iglesia canta el aleluya sin medida.',
  ],
];

const DEFAULT_BLURB =
  'El verde de la vida cotidiana de la Iglesia: catequesis, grupos y la fiesta patronal marcan el paso del año.';

export function blurbFor(seasonName: string): string {
  for (const [pattern, text] of BLURBS) {
    if (pattern.test(seasonName)) return text;
  }
  return DEFAULT_BLURB;
}

export const MESES = [
  'enero', 'febrero', 'marzo', 'abril', 'mayo', 'junio',
  'julio', 'agosto', 'septiembre', 'octubre', 'noviembre', 'diciembre',
];
export const DIAS = ['Domingo', 'Lunes', 'Martes', 'Miércoles', 'Jueves', 'Viernes', 'Sábado'];
export const DIAS_CORTOS = ['dom', 'lun', 'mar', 'mié', 'jue', 'vie', 'sáb'];

/** Hours the week grid spans, and the pixel height of one hour. */
export const H0 = 5;
export const H1 = 24;
export const ROW = 54;
export const GRID_H = (H1 - H0) * ROW;

const RANK_ORDER: Record<Rank, number> = {
  solemnidad: 0,
  fiesta: 1,
  memoria: 2,
  parroquial: 3,
};

export interface DayEvent {
  id: string;
  title: string;
  place: string;
  description: string;
  groupSlug: string;
  groupName: string;
  rank: Rank;
  /** Hex of the event's liturgical color, resolved from the enum. */
  hex: string;
  isLiturgia: boolean;
  /** Start time as HH:MM in parish time. */
  t: string;
  /** End time as HH:MM in parish time. */
  tEnd: string;
  /** Minutes since parish-local midnight. */
  min: number;
  dur: number;
  dateKey: string;
  lane?: number;
  lanes?: number;
}

// One formatter, reused: Intl construction is the expensive part.
const partsFmt = new Intl.DateTimeFormat('en-CA', {
  timeZone: PARISH_TZ,
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
  hour12: false,
});

/** Splits an instant into the parish-local date key and clock time. */
function parishParts(iso: string): { key: string; hh: number; mm: number } {
  const parts = partsFmt.formatToParts(new Date(iso));
  const get = (type: string) => parts.find((p) => p.type === type)?.value ?? '00';
  const hour = Number(get('hour')) % 24; // some engines render midnight as 24
  return {
    key: `${get('year')}-${get('month')}-${get('day')}`,
    hh: hour,
    mm: Number(get('minute')),
  };
}

function pad(n: number): string {
  return String(n).padStart(2, '0');
}

export function hhmm(min: number): string {
  const m = ((min % 1440) + 1440) % 1440;
  return `${pad(Math.floor(m / 60))}:${pad(m % 60)}`;
}

/** Local calendar date as YYYY-MM-DD. Input is a Date built in local fields. */
export function dateKey(d: Date): string {
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

/** Local midnight of a YYYY-MM-DD key. */
export function parseKey(k: string): Date {
  const [y, m, d] = k.split('-').map(Number);
  return new Date(y!, (m ?? 1) - 1, d ?? 1);
}

/** Today, on the parish's clock rather than the visitor's. */
export function todayKey(): string {
  return new Intl.DateTimeFormat('en-CA', { timeZone: PARISH_TZ }).format(new Date());
}

/** Minutes since midnight, now, on the parish's clock. */
export function nowMinutes(): number {
  const p = parishParts(new Date().toISOString());
  return p.hh * 60 + p.mm;
}

/** The 42 day keys of the month grid, Sunday-first, including the spill. */
export function monthCellKeys(anchor: string): string[] {
  const a = parseKey(anchor);
  const first = new Date(a.getFullYear(), a.getMonth(), 1);
  const start = new Date(first);
  start.setDate(1 - first.getDay());
  const out: string[] = [];
  for (let i = 0; i < 42; i++) {
    out.push(dateKey(new Date(start.getFullYear(), start.getMonth(), start.getDate() + i)));
  }
  return out;
}

/** The seven day keys of the anchor's week, Sunday-first. */
export function weekKeys(anchor: string): string[] {
  const a = parseKey(anchor);
  const start = new Date(a.getFullYear(), a.getMonth(), a.getDate() - a.getDay());
  const out: string[] = [];
  for (let i = 0; i < 7; i++) {
    out.push(dateKey(new Date(start.getFullYear(), start.getMonth(), start.getDate() + i)));
  }
  return out;
}

/** Every day key of the anchor's own month, spill excluded. */
export function monthOwnKeys(anchor: string): string[] {
  const a = parseKey(anchor);
  const last = new Date(a.getFullYear(), a.getMonth() + 1, 0).getDate();
  const out: string[] = [];
  for (let i = 1; i <= last; i++) {
    out.push(dateKey(new Date(a.getFullYear(), a.getMonth(), i)));
  }
  return out;
}

/** The fetch window covering a set of day keys, `to` exclusive-safe. */
export function rangeOf(keys: string[]): { from: string; to: string } {
  const sorted = [...keys].sort();
  const last = parseKey(sorted[sorted.length - 1]!);
  last.setDate(last.getDate() + 1);
  return { from: sorted[0]!, to: dateKey(last) };
}

/** Groups events by parish-local day, each day ordered rank-first then time. */
export function toDayEvents(events: ApiEvent[]): Map<string, DayEvent[]> {
  const byDay = new Map<string, DayEvent[]>();
  for (const e of events) {
    const start = parishParts(e.starts_at);
    const end = parishParts(e.ends_at);
    const min = start.hh * 60 + start.mm;
    const rawDur = (new Date(e.ends_at).getTime() - new Date(e.starts_at).getTime()) / 60000;
    const item: DayEvent = {
      id: e.id,
      title: e.title,
      place: e.place,
      description: e.description,
      groupSlug: e.group.slug,
      groupName: e.group.name,
      rank: e.rank,
      hex: (SEASON_PALETTE[e.color] ?? SEASON_PALETTE.verde).color,
      isLiturgia: e.group.slug === 'liturgia',
      t: `${pad(start.hh)}:${pad(start.mm)}`,
      tEnd: `${pad(end.hh)}:${pad(end.mm)}`,
      min,
      dur: Math.max(15, Math.round(rawDur)),
      dateKey: start.key,
    };
    const bucket = byDay.get(item.dateKey);
    if (bucket) bucket.push(item);
    else byDay.set(item.dateKey, [item]);
  }
  for (const items of byDay.values()) {
    items.sort((a, b) => RANK_ORDER[a.rank] - RANK_ORDER[b.rank] || a.min - b.min);
  }
  return byDay;
}

export interface ResolvedSeason extends Palette {
  name: string;
}

/**
 * The season covering a day. Seasons are seeded a couple of years ahead, so a
 * date past the horizon falls back to ordinary green rather than disappearing
 * — the same rule the API applies to event colors.
 */
export function seasonOf(key: string, seasons: ApiSeason[]): ResolvedSeason {
  for (const s of seasons) {
    if (key >= s.start && key <= s.end) {
      return { ...(SEASON_PALETTE[s.color] ?? SEASON_PALETTE.verde), name: s.name };
    }
  }
  return { ...SEASON_PALETTE.verde, name: 'Tiempo Ordinario' };
}

/**
 * The day's celebration: the highest-ranked liturgical event. Group activities
 * (`parroquial`) never claim the day's headline — that line belongs to the
 * liturgy.
 */
export function celebrationOf(items: DayEvent[]): DayEvent | null {
  for (const e of items) {
    if (e.rank !== 'parroquial') return e;
  }
  return null;
}

/**
 * Assigns overlapping events to side-by-side lanes. Events are walked in time
 * order; a cluster ends as soon as an event starts after everything before it
 * has finished, so unrelated parts of the day never share a width.
 */
export function layoutLanes(items: DayEvent[]): DayEvent[] {
  const byTime = [...items].sort((a, b) => a.min - b.min);
  const out: DayEvent[] = [];
  let cluster: DayEvent[] = [];
  let clusterEnd = -1;

  const flush = () => {
    if (!cluster.length) return;
    const laneEnds: number[] = [];
    for (const e of cluster) {
      let li = 0;
      while (li < laneEnds.length && laneEnds[li]! > e.min) li++;
      laneEnds[li] = e.min + e.dur;
      e.lane = li;
    }
    for (const e of cluster) e.lanes = laneEnds.length;
    out.push(...cluster);
    cluster = [];
    clusterEnd = -1;
  };

  for (const e of byTime) {
    if (cluster.length && e.min >= clusterEnd) flush();
    cluster.push(e);
    clusterEnd = Math.max(clusterEnd, e.min + e.dur);
  }
  flush();
  return out;
}

export function rangeLabel(anchor: string, view: 'mes' | 'semana'): string {
  const a = parseKey(anchor);
  if (view === 'mes') {
    const name = MESES[a.getMonth()]!;
    return `${name.charAt(0).toUpperCase()}${name.slice(1)} ${a.getFullYear()}`;
  }
  const keys = weekKeys(anchor);
  const start = parseKey(keys[0]!);
  const end = parseKey(keys[6]!);
  if (start.getMonth() === end.getMonth()) {
    return `${start.getDate()}–${end.getDate()} de ${MESES[start.getMonth()]}`;
  }
  return (
    `${start.getDate()} ${MESES[start.getMonth()]!.slice(0, 3)} – ` +
    `${end.getDate()} ${MESES[end.getMonth()]!.slice(0, 3)}`
  );
}

export function dayLabel(key: string): string {
  const d = parseKey(key);
  return `${DIAS[d.getDay()]} ${d.getDate()} de ${MESES[d.getMonth()]}`;
}

/** Shifts an anchor by whole months (month view) or days (week view). */
export function shiftAnchor(anchor: string, view: 'mes' | 'semana', dir: -1 | 1): string {
  const d = parseKey(anchor);
  if (view === 'mes') d.setMonth(d.getMonth() + dir, 1);
  else d.setDate(d.getDate() + dir * 7);
  return dateKey(d);
}
