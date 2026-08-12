// The parish's own clock. Every date the site shows — "hoy", the hour on a
// block, which day an event belongs to — is resolved in this zone, so a
// visitor abroad still reads the parish's calendar, not their own.
export const PARISH_TZ = 'America/Tijuana';

export const PARISH_NAME = 'Cristo de Los Álamos';

// Ordinary Sunday-to-Saturday mass schedule. These are not events: the
// calendar only carries what the parish publishes, and the everyday schedule
// would drown it. Shown as a static card instead.
export const HORARIOS: Array<{ day: string; times: string; note?: string }> = [
  { day: 'Domingo', times: '07:00 · 09:00\n12:00 · 19:00' },
  { day: 'Lunes a viernes', times: '07:00' },
  { day: 'Sábado', times: '07:00 · 19:00', note: 'vespertina' },
];
