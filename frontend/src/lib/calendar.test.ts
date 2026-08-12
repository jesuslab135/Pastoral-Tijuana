import { describe, expect, it } from 'vitest';

import type { ApiEvent, ApiSeason, Rank, SeasonColor } from './api';
import {
  celebrationOf,
  dayLabel,
  layoutLanes,
  monthCellKeys,
  monthOwnKeys,
  rangeLabel,
  rangeOf,
  seasonOf,
  shiftAnchor,
  toDayEvents,
  weekKeys,
  type DayEvent,
} from './calendar';

function event(over: Partial<ApiEvent> & { starts_at: string; ends_at: string }): ApiEvent {
  return {
    id: over.id ?? Math.random().toString(36).slice(2),
    title: over.title ?? 'Evento',
    description: over.description ?? '',
    place: over.place ?? '',
    group: over.group ?? { id: 'g1', name: 'Liturgia', slug: 'liturgia' },
    rank: over.rank ?? 'parroquial',
    color: over.color ?? 'verde',
    starts_at: over.starts_at,
    ends_at: over.ends_at,
  };
}

/** A DayEvent with only the fields the layout cares about. */
function block(min: number, dur: number, id = `${min}`): DayEvent {
  return {
    id,
    title: id,
    place: '',
    description: '',
    groupSlug: 'liturgia',
    groupName: 'Liturgia',
    rank: 'parroquial',
    hex: '#2f6b4f',
    isLiturgia: true,
    t: '00:00',
    tEnd: '00:00',
    min,
    dur,
    dateKey: '2026-08-12',
  };
}

describe('toDayEvents', () => {
  it('files an event under the parish day, not the viewer’s', () => {
    // 03:00Z on the 16th is still 20:00 on the 15th in Tijuana. A viewer in
    // Madrid must still see it on the parish's day.
    const [e] = toDayEvents([
      event({ starts_at: '2026-08-16T03:00:00Z', ends_at: '2026-08-16T04:30:00Z' }),
    ]).get('2026-08-15')!;

    expect(e).toBeDefined();
    expect(e!.t).toBe('20:00');
    expect(e!.tEnd).toBe('21:30');
    expect(e!.min).toBe(20 * 60);
    expect(e!.dur).toBe(90);
  });

  it('orders a day by rank first and time second', () => {
    const byDay = toDayEvents([
      event({ id: 'tarde', title: 'Ensayo', starts_at: '2026-08-15T22:00:00Z', ends_at: '2026-08-15T23:00:00Z' }),
      event({ id: 'misa', title: 'Misa', rank: 'solemnidad', starts_at: '2026-08-16T02:00:00Z', ends_at: '2026-08-16T03:00:00Z' }),
      event({ id: 'manana', title: 'Despensa', starts_at: '2026-08-15T17:00:00Z', ends_at: '2026-08-15T18:00:00Z' }),
    ]);
    // The solemnity wins the first line even though it starts last.
    expect(byDay.get('2026-08-15')!.map((e) => e.id)).toEqual(['misa', 'manana', 'tarde']);
  });

  it('resolves the liturgical color and flags the liturgy group', () => {
    const byDay = toDayEvents([
      event({ color: 'rojo', starts_at: '2026-08-15T17:00:00Z', ends_at: '2026-08-15T18:00:00Z' }),
      event({
        id: 'coro',
        group: { id: 'g4', name: 'Coro', slug: 'coro' },
        starts_at: '2026-08-15T19:00:00Z',
        ends_at: '2026-08-15T20:00:00Z',
      }),
    ]);
    const [lit, coro] = byDay.get('2026-08-15')!;
    expect(lit!.hex).toBe('#a02f27');
    expect(lit!.isLiturgia).toBe(true);
    expect(coro!.isLiturgia).toBe(false);
  });

  it('never renders a zero-height block', () => {
    const [e] = toDayEvents([
      event({ starts_at: '2026-08-15T17:00:00Z', ends_at: '2026-08-15T17:00:00Z' }),
    ]).get('2026-08-15')!;
    expect(e!.dur).toBeGreaterThanOrEqual(15);
  });

  it('handles an unknown season color rather than rendering undefined', () => {
    const [e] = toDayEvents([
      event({
        color: 'turquesa' as SeasonColor,
        starts_at: '2026-08-15T17:00:00Z',
        ends_at: '2026-08-15T18:00:00Z',
      }),
    ]).get('2026-08-15')!;
    expect(e!.hex).toBe('#2f6b4f');
  });
});

describe('layoutLanes', () => {
  it('puts overlapping events side by side', () => {
    const laid = layoutLanes([block(600, 60, 'a'), block(630, 60, 'b')]);
    expect(laid.map((e) => e.lane).sort()).toEqual([0, 1]);
    expect(laid.every((e) => e.lanes === 2)).toBe(true);
  });

  it('gives consecutive events the full width', () => {
    const laid = layoutLanes([block(600, 60, 'a'), block(660, 60, 'b')]);
    expect(laid.every((e) => e.lanes === 1)).toBe(true);
  });

  it('does not let one busy cluster narrow the rest of the day', () => {
    const laid = layoutLanes([
      block(600, 60, 'a'),
      block(600, 60, 'b'),
      block(1200, 60, 'tarde'),
    ]);
    expect(laid.find((e) => e.id === 'tarde')!.lanes).toBe(1);
  });

  it('reuses a lane once it is free', () => {
    // a: 10:00–12:00, b: 10:30–11:00, c: 11:00–12:00 → c takes b's lane.
    const laid = layoutLanes([block(600, 120, 'a'), block(630, 30, 'b'), block(660, 60, 'c')]);
    expect(laid.every((e) => e.lanes === 2)).toBe(true);
    expect(laid.find((e) => e.id === 'c')!.lane).toBe(laid.find((e) => e.id === 'b')!.lane);
  });
});

describe('celebrationOf', () => {
  const lit = (rank: Rank, id: string): DayEvent => ({ ...block(600, 60, id), rank });

  it('picks the highest-ranked liturgical event', () => {
    // Already rank-ordered by toDayEvents, which is the contract.
    expect(celebrationOf([lit('fiesta', 'f'), lit('memoria', 'm')])!.id).toBe('f');
  });

  it('never lets a group activity claim the day', () => {
    expect(celebrationOf([lit('parroquial', 'kermes')])).toBeNull();
    expect(celebrationOf([])).toBeNull();
  });
});

describe('seasonOf', () => {
  const seasons: ApiSeason[] = [
    { name: 'Adviento', color: 'violeta', start: '2026-11-29', end: '2026-12-12' },
    { name: 'Adviento · Gaudete', color: 'rosa', start: '2026-12-13', end: '2026-12-13' },
  ];

  it('resolves a covered day, including a single-day season', () => {
    expect(seasonOf('2026-12-01', seasons).name).toBe('Adviento');
    const gaudete = seasonOf('2026-12-13', seasons);
    expect(gaudete.name).toBe('Adviento · Gaudete');
    expect(gaudete.color).toBe('#c06f8d');
  });

  it('falls back to ordinary green past the seeded horizon', () => {
    // Seasons are seeded a couple of years ahead; a later date must still
    // render rather than disappear.
    const far = seasonOf('2035-06-01', seasons);
    expect(far.name).toBe('Tiempo Ordinario');
    expect(far.color).toBe('#2f6b4f');
  });
});

describe('date windows', () => {
  it('builds a 42-cell Sunday-first month grid with the spill', () => {
    const keys = monthCellKeys('2026-08-12');
    expect(keys).toHaveLength(42);
    expect(keys[0]).toBe('2026-07-26'); // the Sunday before August 1
    expect(keys[41]).toBe('2026-09-05');
    expect(keys).toContain('2026-08-01');
    expect(keys).toContain('2026-08-31');
  });

  it('builds a Sunday-first week', () => {
    expect(weekKeys('2026-08-12')).toEqual([
      '2026-08-09', '2026-08-10', '2026-08-11', '2026-08-12',
      '2026-08-13', '2026-08-14', '2026-08-15',
    ]);
  });

  it('lists only the anchor month for the agenda', () => {
    const own = monthOwnKeys('2026-02-10');
    expect(own[0]).toBe('2026-02-01');
    expect(own).toHaveLength(28);
  });

  it('covers the last day when turning keys into a fetch range', () => {
    // `to` is exclusive in the API, so it has to sit past the final day.
    expect(rangeOf(['2026-08-09', '2026-08-15'])).toEqual({
      from: '2026-08-09',
      to: '2026-08-16',
    });
  });
});

describe('shiftAnchor', () => {
  it('moves by whole months without spilling into the next one', () => {
    // From the 31st, a naive month shift lands in March.
    expect(shiftAnchor('2026-01-31', 'mes', 1)).toBe('2026-02-01');
    expect(shiftAnchor('2026-01-15', 'mes', -1)).toBe('2025-12-01');
  });

  it('moves a week at a time in the week view', () => {
    expect(shiftAnchor('2026-08-12', 'semana', 1)).toBe('2026-08-19');
    expect(shiftAnchor('2026-08-12', 'semana', -1)).toBe('2026-08-05');
  });
});

describe('labels', () => {
  it('names the month and the week in Spanish', () => {
    expect(rangeLabel('2026-08-12', 'mes')).toBe('Agosto 2026');
    expect(rangeLabel('2026-08-12', 'semana')).toBe('9–15 de agosto');
  });

  it('abbreviates a week that straddles two months', () => {
    expect(rangeLabel('2026-09-02', 'semana')).toBe('30 ago – 5 sep');
  });

  it('writes a day the way the parish reads it', () => {
    expect(dayLabel('2026-08-12')).toBe('Miércoles 12 de agosto');
  });
});
