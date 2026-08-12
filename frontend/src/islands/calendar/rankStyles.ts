// The blueprint's rank treatments. Color is never the only carrier: each rank
// also differs in shape and weight, so the calendar stays readable without
// color vision.

import { GRAPHITE, type DayEvent } from '../../lib/calendar';

export interface CelebStyle {
  bg: string;
  fg: string;
  bd: string;
  fw: number;
  dot: string;
}

/** The day's headline celebration, styled by rank. */
export function celebStyle(e: DayEvent): CelebStyle {
  const c = e.hex;
  switch (e.rank) {
    case 'solemnidad':
      return { bg: c, fg: '#fffdf7', bd: c, fw: 600, dot: 'rgba(255,255,255,.85)' };
    case 'fiesta':
      return { bg: `${c}20`, fg: c, bd: `${c}55`, fw: 600, dot: c };
    case 'memoria':
      return { bg: 'transparent', fg: c, bd: 'transparent', fw: 500, dot: c };
    default:
      return {
        bg: 'transparent',
        fg: GRAPHITE,
        bd: 'rgba(107,98,85,.42)',
        fw: 400,
        dot: GRAPHITE,
      };
  }
}

export interface BlockStyle {
  bg: string;
  fg: string;
  bd: string;
  dash: 'solid' | 'dashed';
  fw: number;
}

/**
 * A block in the week or agenda. Liturgy takes the day's color with a solid
 * border; a group activity is deliberately outside the liturgical palette,
 * dashed and graphite, so the two never read as the same kind of thing.
 */
export function blockStyle(e: DayEvent): BlockStyle {
  if (e.isLiturgia) {
    return { bg: `${e.hex}1c`, fg: e.hex, bd: e.hex, dash: 'solid', fw: 600 };
  }
  return {
    bg: 'rgba(107,98,85,.07)',
    fg: GRAPHITE,
    bd: 'rgba(107,98,85,.4)',
    dash: 'dashed',
    fw: 400,
  };
}
