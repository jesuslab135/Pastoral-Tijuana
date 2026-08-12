import { render, screen } from '@testing-library/preact';
import { describe, expect, it, vi } from 'vitest';

import type { ApiEvent } from '../../lib/api';
import { toDayEvents, type DayEvent } from '../../lib/calendar';
import MonthGrid from './MonthGrid';

const DAY = '2026-08-12';

/** An event at the given parish-local hour on DAY (Tijuana is UTC−7 in August). */
function apiEvent(title: string, hour: number, rank: ApiEvent['rank'] = 'parroquial'): ApiEvent {
  // Date.UTC rolls over past midnight, so an evening local hour still yields
  // a valid instant on the following UTC day.
  const start = new Date(Date.UTC(2026, 7, 12, hour + 7, 0));
  const end = new Date(start.getTime() + 3600_000);
  return {
    id: title,
    title,
    description: '',
    place: '',
    group: { id: 'g', name: 'Coro', slug: 'coro' },
    rank,
    color: 'verde',
    starts_at: start.toISOString(),
    ends_at: end.toISOString(),
  };
}

function renderGrid(items: DayEvent[], onPick = vi.fn()) {
  const byDay = new Map([[DAY, items]]);
  render(
    <MonthGrid
      anchor={DAY}
      sel={DAY}
      todayK={DAY}
      seasons={[]}
      itemsFor={(k) => byDay.get(k) ?? []}
      cellHeight={118}
      onPick={onPick}
    />,
  );
  return onPick;
}

function eventsFor(...api: ApiEvent[]): DayEvent[] {
  return toDayEvents(api).get(DAY) ?? [];
}

describe('MonthGrid cell capacity', () => {
  it('renders every activity when the day is quiet', () => {
    renderGrid(eventsFor(apiEvent('Ensayo', 17), apiEvent('Junta', 19)));
    expect(screen.getByText(/Ensayo/)).toBeInTheDocument();
    expect(screen.getByText(/Junta/)).toBeInTheDocument();
    expect(screen.queryByText(/más/)).not.toBeInTheDocument();
  });

  it('caps a busy day at two chips and counts the rest', () => {
    // A fixed-height cell cannot scroll, so the overflow has to be counted.
    renderGrid(
      eventsFor(
        apiEvent('Ensayo', 10),
        apiEvent('Junta', 12),
        apiEvent('Despensa', 14),
        apiEvent('Retiro', 16),
      ),
    );
    expect(screen.getByText('+2 más')).toBeInTheDocument();
  });

  it('gives up a chip line when the day has a celebration', () => {
    renderGrid(
      eventsFor(
        apiEvent('Asunción', 12, 'solemnidad'),
        apiEvent('Ensayo', 15),
        apiEvent('Junta', 17),
      ),
    );
    // The celebration takes the first line; one chip fits, one is counted.
    expect(screen.getByText('Asunción')).toBeInTheDocument();
    expect(screen.getByText('+1 más')).toBeInTheDocument();
  });

  it('never repeats the celebration as a chip below itself', () => {
    renderGrid(eventsFor(apiEvent('Asunción', 12, 'solemnidad')));
    expect(screen.getAllByText(/Asunción/)).toHaveLength(1);
  });

  it('shows the hour with each activity', () => {
    renderGrid(eventsFor(apiEvent('Ensayo', 17)));
    expect(screen.getByText('17:00 · Ensayo')).toBeInTheDocument();
  });
});

describe('MonthGrid structure', () => {
  it('lays out six Sunday-first weeks', () => {
    renderGrid([]);
    expect(screen.getByText('dom')).toBeInTheDocument();
    expect(screen.getByText('sáb')).toBeInTheDocument();
    // 42 cells, so the grid height never changes between months.
    expect(screen.getAllByText('12')).not.toHaveLength(0);
  });

  it('reports the day the visitor clicked', async () => {
    const onPick = renderGrid([]);
    screen.getByText('12').click();
    expect(onPick).toHaveBeenCalledWith(DAY);
  });
});
