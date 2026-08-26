import { afterEach, describe, expect, it, vi } from 'vitest';

import { isoToParishParts, parishToISO } from './dates';

// The runner is pinned to Europe/Madrid (vitest.config.ts), so any of these
// that reached for the browser's clock instead of the parish's would fail.

describe('parishToISO', () => {
  it('reads a time as parish wall time, not UTC', () => {
    // 19:00 in Tijuana (UTC-7 in September) is 02:00Z the next day.
    expect(parishToISO('2026-09-03', '19:00')).toBe('2026-09-04T02:00:00.000Z');
  });

  it('accepts a time that carries seconds', () => {
    // Some platform time pickers hand back HH:MM:SS. Pasting that into the
    // instant template built an Invalid Date, the editor swallowed the
    // RangeError as "Algo salió mal.", and the event was never sent.
    expect(parishToISO('2026-09-03', '19:00:00')).toBe('2026-09-04T02:00:00.000Z');
    expect(parishToISO('2026-09-03', '19:00:30')).toBe('2026-09-04T02:00:30.000Z');
  });

  it('never yields an invalid instant for a value the time input can hold', () => {
    for (const time of ['00:00', '9:05', '09:05:00', '23:59:59']) {
      expect(Number.isNaN(Date.parse(parishToISO('2026-09-03', time)))).toBe(false);
    }
  });

  it('round-trips through isoToParishParts', () => {
    const parts = { date: '2026-09-03', time: '19:00' };
    expect(isoToParishParts(parishToISO(parts.date, parts.time))).toEqual(parts);
  });

  it('holds the wall time across the DST change', () => {
    // Tijuana leaves DST on 2026-11-01; 09:00 stays 09:00 either side of it.
    expect(isoToParishParts(parishToISO('2026-10-31', '09:00')).time).toBe('09:00');
    expect(isoToParishParts(parishToISO('2026-11-01', '09:00')).time).toBe('09:00');
  });
});

describe('isoToParishParts', () => {
  afterEach(() => vi.useRealTimers());

  it('gives the parish date, which is not the UTC date late in the evening', () => {
    // 2026-08-27T01:00Z is still 18:00 on the 26th in Tijuana. The new-event
    // form seeded its date from the UTC half of this and opened a day late.
    expect(isoToParishParts('2026-08-27T01:00:00.000Z').date).toBe('2026-08-26');
    expect(isoToParishParts('2026-08-27T01:00:00.000Z').time).toBe('18:00');
  });

  it("today on the parish clock trails the UTC date after 17:00 in Tijuana", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-08-27T01:00:00.000Z'));
    const now = new Date().toISOString();
    expect(now.slice(0, 10)).toBe('2026-08-27');
    expect(isoToParishParts(now).date).toBe('2026-08-26');
  });
});
