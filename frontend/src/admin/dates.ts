// The panel edits instants but its forms speak parish wall time: the secretary
// types "22 de agosto, 09:00" meaning nine in Tijuana, whatever clock her
// laptop is on. These convert in both directions without a date library.

import { DIAS_CORTOS, MESES } from '../lib/calendar';
import { PARISH_TZ } from '../lib/config';

const fmt = new Intl.DateTimeFormat('en-CA', {
  timeZone: PARISH_TZ,
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
  second: '2-digit',
  hour12: false,
});

interface Wall {
  year: string;
  month: string;
  day: string;
  hour: string;
  minute: string;
  second: string;
}

function wallAt(instant: Date): Wall {
  const p = Object.fromEntries(
    fmt.formatToParts(instant).map((x) => [x.type, x.value]),
  ) as unknown as Wall;
  // Some engines render midnight as hour 24; normalising here keeps every
  // caller from having to remember it.
  return { ...p, hour: String(Number(p.hour) % 24).padStart(2, '0') };
}

/** Milliseconds the parish clock is ahead of UTC at the given instant (negative in Tijuana). */
function offsetAt(instant: Date): number {
  const p = wallAt(instant);
  const wall = Date.UTC(+p.year, +p.month - 1, +p.day, +p.hour, +p.minute, +p.second);
  return wall - instant.getTime();
}

/** Interprets a date+time the user typed as parish wall time and returns the instant. */
export function parishToISO(date: string, time: string): string {
  const naive = new Date(`${date}T${time}:00Z`); // pretend it's UTC
  const once = new Date(naive.getTime() - offsetAt(naive)); // correct by the offset there
  return new Date(naive.getTime() - offsetAt(once)).toISOString(); // second pass rides out DST edges
}

function dayOf(p: Wall): string {
  return `${p.year}-${p.month}-${p.day}`;
}

/** A YYYY-MM-DD key built from calendar fields, month and day overflow allowed. */
function key(year: number, month: number, day: number): string {
  const d = new Date(Date.UTC(year, month, day));
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getUTCFullYear()}-${pad(d.getUTCMonth() + 1)}-${pad(d.getUTCDate())}`;
}

/** Splits an instant back into the two form fields that produced it. */
export function isoToParishParts(iso: string): { date: string; time: string } {
  const p = wallAt(new Date(iso));
  return { date: dayOf(p), time: `${p.hour}:${p.minute}` };
}

/** The pieces a compact date chip renders: '22', 'sáb', '09:00'. */
export function parishDayParts(iso: string): { day: string; dow: string; time: string } {
  const p = wallAt(new Date(iso));
  // Weekday off the parish-local calendar date rather than the instant: a UTC
  // date built from those fields lands on the same weekday by construction.
  const dow = new Date(Date.UTC(+p.year, +p.month - 1, +p.day)).getUTCDay();
  return { day: p.day, dow: DIAS_CORTOS[dow]!, time: `${p.hour}:${p.minute}` };
}

/** Relative on the last two parish days, absolute before that. */
export function parishStamp(iso: string): string {
  const p = wallAt(new Date(iso));
  const clock = `${p.hour}:${p.minute}`;
  const day = dayOf(p);
  // Yesterday is the day before today's parish date, not 24 hours back: the
  // two disagree on the mornings either side of a DST change.
  const today = wallAt(new Date());
  if (day === dayOf(today)) return `hoy ${clock}`;
  if (day === key(+today.year, +today.month - 1, +today.day - 1)) return `ayer ${clock}`;
  return `${Number(p.day)} ${MESES[Number(p.month) - 1]!.slice(0, 3)} ${clock}`;
}

/** The [from, to) window covering the anchor's month, plus its heading. */
export function monthWindow(anchor: Date): { from: string; to: string; label: string } {
  const y = anchor.getFullYear();
  const m = anchor.getMonth();
  const name = MESES[m]!;
  return {
    from: key(y, m, 1),
    // `to` is exclusive on the API side, so the next first covers the month.
    to: key(y, m + 1, 1),
    label: `${name.charAt(0).toUpperCase()}${name.slice(1)} ${y}`,
  };
}
