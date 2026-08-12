// The difusión column, folded once per screen: the broadcast log is a flat list
// of one row per channel per event, and every screen that shows it wants the
// same per-event tally.

import type { Broadcast } from '../types';

export interface Cast {
  sent: number;
  total: number;
  failed: boolean;
}

/**
 * Tallies the broadcast log by event. `failed` covers `dead` too: a message
 * that ran out of retries is still a message the parish never received.
 */
export function castsByEvent(broadcasts: Broadcast[]): Map<string, Cast> {
  const byEvent = new Map<string, Cast>();
  for (const b of broadcasts) {
    let cast = byEvent.get(b.event_id);
    if (!cast) {
      cast = { sent: 0, total: 0, failed: false };
      byEvent.set(b.event_id, cast);
    }
    cast.total += 1;
    if (b.state === 'sent') cast.sent += 1;
    if (b.state === 'failed' || b.state === 'dead') cast.failed = true;
  }
  return byEvent;
}
