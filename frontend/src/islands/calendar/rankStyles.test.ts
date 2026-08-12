import { describe, expect, it } from 'vitest';

import type { Rank } from '../../lib/api';
import type { DayEvent } from '../../lib/calendar';
import { blockStyle, celebStyle } from './rankStyles';

const VIOLETA = '#5c3b7a';

function ev(rank: Rank, isLiturgia = true): DayEvent {
  return {
    id: rank,
    title: rank,
    place: '',
    description: '',
    groupSlug: isLiturgia ? 'liturgia' : 'coro',
    groupName: isLiturgia ? 'Liturgia' : 'Coro',
    rank,
    hex: VIOLETA,
    isLiturgia,
    t: '12:00',
    tEnd: '13:00',
    min: 720,
    dur: 60,
    dateKey: '2026-08-12',
  };
}

describe('celebStyle', () => {
  it('fills a solemnity and reverses the text out of it', () => {
    const s = celebStyle(ev('solemnidad'));
    expect(s.bg).toBe(VIOLETA);
    expect(s.fg).toBe('#fffdf7');
    expect(s.fw).toBe(600);
  });

  it('tints a feast instead of filling it', () => {
    const s = celebStyle(ev('fiesta'));
    expect(s.bg).toBe(`${VIOLETA}20`);
    expect(s.bd).toBe(`${VIOLETA}55`);
    expect(s.fg).toBe(VIOLETA);
  });

  it('gives a memorial a dot and no box', () => {
    const s = celebStyle(ev('memoria'));
    expect(s.bg).toBe('transparent');
    expect(s.bd).toBe('transparent');
    expect(s.dot).toBe(VIOLETA);
  });

  it('keeps a group activity outside the liturgical palette', () => {
    const s = celebStyle(ev('parroquial', false));
    expect(s.fg).toBe('#6b6255');
    expect(s.bd).toContain('107,98,85');
    expect(s.fw).toBe(400);
  });

  it('separates the ranks by weight as well as color', () => {
    // Color must never be the only carrier: weight has to differ too.
    const weights = (['solemnidad', 'fiesta', 'memoria', 'parroquial'] as Rank[]).map(
      (r) => celebStyle(ev(r)).fw,
    );
    expect(new Set(weights).size).toBeGreaterThan(1);
    expect(celebStyle(ev('solemnidad')).fw).toBeGreaterThan(celebStyle(ev('parroquial')).fw);
  });
});

describe('blockStyle', () => {
  it('draws liturgy solid in the day’s color', () => {
    const s = blockStyle(ev('memoria'));
    expect(s.dash).toBe('solid');
    expect(s.bd).toBe(VIOLETA);
    expect(s.bg).toBe(`${VIOLETA}1c`);
  });

  it('draws a group activity dashed and graphite', () => {
    const s = blockStyle(ev('parroquial', false));
    expect(s.dash).toBe('dashed');
    expect(s.fg).toBe('#6b6255');
    // Deliberately outside the liturgical palette.
    expect(s.bd).not.toContain(VIOLETA);
  });
});
