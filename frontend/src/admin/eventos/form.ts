// The editor's form state and the two translations between it and the API.
// The form speaks parish wall time and a duration; the API speaks two instants.
//
// Duration is a closed list rather than the mock's free-text box: "1 h 30",
// "1:30", "90min" and "hora y media" all mean the same thing to a secretary and
// nothing reliable to a parser, and a misread duration ships a wrong end time
// to WhatsApp.

import { isoToParishParts, parishToISO } from '../dates';
import type { AdminEvent, EventInput, Rank } from '../types';

export interface EventForm {
  title: string;
  date: string;
  time: string;
  durationMin: number;
  place: string;
  group_id: string;
  rank: Rank;
  description: string;
  /**
   * Carried through the form untouched. No control sets it — the colour is
   * inherited from the day's liturgical season — but a patronal feast can have
   * one stored, and round-tripping it is what stops an ordinary edit from
   * quietly stripping it.
   */
  color_override: string | null;
}

export const DURATIONS: Array<{ min: number; label: string }> = [
  { min: 30, label: '30 min' },
  { min: 45, label: '45 min' },
  { min: 60, label: '1 h' },
  { min: 75, label: '1 h 15' },
  { min: 90, label: '1 h 30' },
  { min: 120, label: '2 h' },
  { min: 150, label: '2 h 30' },
  { min: 180, label: '3 h' },
  { min: 240, label: '4 h' },
];

const MINUTE_MS = 60_000;

export function toInput(f: EventForm): EventInput {
  const starts = parishToISO(f.date, f.time);
  return {
    title: f.title.trim(),
    description: f.description.trim(),
    place: f.place.trim(),
    starts_at: starts,
    ends_at: new Date(Date.parse(starts) + f.durationMin * MINUTE_MS).toISOString(),
    group_id: f.group_id,
    rank: f.rank,
    color_override: f.color_override,
  };
}

export function fromEvent(e: AdminEvent): EventForm {
  const { date, time } = isoToParishParts(e.starts_at);
  const span = Math.round((Date.parse(e.ends_at) - Date.parse(e.starts_at)) / MINUTE_MS);
  return {
    title: e.title,
    date,
    time,
    // A duration outside DURATIONS is kept as it is — the editor adds an option
    // for it — so opening an event never quietly moves its end time.
    durationMin: Math.max(0, span),
    place: e.place,
    group_id: e.group_id,
    rank: e.rank,
    description: e.description,
    color_override: e.color_override,
  };
}

/** The first thing missing before this event can be saved, or null when it can. */
export function validate(f: EventForm): string | null {
  if (!f.title.trim()) return 'Ponle un título al evento antes de guardarlo.';
  if (!f.date) return 'Falta la fecha del evento.';
  if (!f.time) return 'Falta la hora del evento.';
  if (!f.group_id) return 'Elige el grupo parroquial que organiza el evento.';
  return null;
}
