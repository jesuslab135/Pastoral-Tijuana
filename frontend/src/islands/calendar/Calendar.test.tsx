import { render, screen, waitFor } from '@testing-library/preact';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { ApiEvent } from '../../lib/api';
import { todayKey } from '../../lib/calendar';
import Calendar from './Calendar';

const TODAY = todayKey();

/** An event today at the given parish-local hour, in the given group. */
function apiEvent(title: string, hour: number, slug: string, name: string): ApiEvent {
  const [y, m, d] = TODAY.split('-').map(Number);
  // Parish time trails UTC, so adding the offset keeps it on the same day.
  const start = new Date(Date.UTC(y!, m! - 1, d!, hour + 7, 0));
  return {
    id: `${slug}-${hour}`,
    title,
    description: '',
    place: 'Templo',
    group: { id: slug, name, slug },
    rank: 'parroquial',
    color: 'verde',
    starts_at: start.toISOString(),
    ends_at: new Date(start.getTime() + 3600_000).toISOString(),
  };
}

const EVENTS = [
  apiEvent('Ensayo del coro', 17, 'coro', 'Coro'),
  apiEvent('Despensa solidaria', 10, 'caridad', 'Caridad'),
];

const GROUPS = [
  { id: 'coro', name: 'Coro', slug: 'coro' },
  { id: 'caridad', name: 'Caridad', slug: 'caridad' },
];

function stubApi() {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string) => {
      const body = url.includes('/events')
        ? { events: EVENTS }
        : url.includes('/groups')
          ? { groups: GROUPS }
          : { seasons: [] };
      return { ok: true, json: async () => body } as Response;
    }),
  );
}

function setWidth(px: number) {
  Object.defineProperty(window, 'innerWidth', { value: px, configurable: true, writable: true });
}

beforeEach(() => {
  stubApi();
  setWidth(1440);
  // The month grid renders a fixed 42 cells; nothing here depends on rAF.
  vi.stubGlobal('requestAnimationFrame', (cb: FrameRequestCallback) => {
    cb(0);
    return 0;
  });
});

afterEach(() => vi.unstubAllGlobals());

describe('group filters', () => {
  it('shows every group until one is chosen', async () => {
    render(<Calendar />);
    await waitFor(() => expect(screen.getAllByText(/Ensayo del coro/).length).toBeGreaterThan(0));
    expect(screen.getAllByText(/Despensa solidaria/).length).toBeGreaterThan(0);
  });

  it('narrows the calendar to the chosen group', async () => {
    render(<Calendar />);
    await waitFor(() => expect(screen.getAllByText(/Ensayo del coro/).length).toBeGreaterThan(0));

    screen.getByRole('button', { name: 'Coro' }).click();

    await waitFor(() => expect(screen.queryByText(/Despensa solidaria/)).not.toBeInTheDocument());
    expect(screen.getAllByText(/Ensayo del coro/).length).toBeGreaterThan(0);
  });

  it('restores everything when the filter is switched off', async () => {
    render(<Calendar />);
    await waitFor(() => expect(screen.getAllByText(/Ensayo del coro/).length).toBeGreaterThan(0));

    const coro = screen.getByRole('button', { name: 'Coro' });
    coro.click();
    await waitFor(() => expect(screen.queryByText(/Despensa solidaria/)).not.toBeInTheDocument());
    coro.click();
    await waitFor(() => expect(screen.getAllByText(/Despensa/).length).toBeGreaterThan(0));
  });

  it('marks the active chip for assistive tech, not just visually', async () => {
    render(<Calendar />);
    const coro = await screen.findByRole('button', { name: 'Coro' });
    expect(coro).toHaveAttribute('aria-pressed', 'false');
    coro.click();
    await waitFor(() => expect(coro).toHaveAttribute('aria-pressed', 'true'));
  });
});

describe('breakpoint', () => {
  it('shows the month grid on a desktop', async () => {
    render(<Calendar />);
    // The weekday header only exists in the grid.
    expect(await screen.findByText('dom')).toBeInTheDocument();
  });

  it('turns the month into an agenda on a phone', async () => {
    setWidth(390);
    render(<Calendar />);
    await waitFor(() => expect(screen.getAllByText(/Ensayo del coro/).length).toBeGreaterThan(0));
    // A 7-column grid is unreadable at this width, so it must not be there.
    expect(screen.queryByText('dom')).not.toBeInTheDocument();
    // The agenda lists the group under each activity.
    expect(screen.getAllByText('Coro').length).toBeGreaterThan(0);
  });

  it('follows a resize across the breakpoint', async () => {
    render(<Calendar />);
    expect(await screen.findByText('dom')).toBeInTheDocument();

    setWidth(390);
    window.dispatchEvent(new Event('resize'));

    await waitFor(() => expect(screen.queryByText('dom')).not.toBeInTheDocument());
  });
});

describe('view switching', () => {
  it('moves between month and week', async () => {
    render(<Calendar />);
    expect(await screen.findByText('dom')).toBeInTheDocument();

    screen.getByRole('button', { name: 'Semana' }).click();

    // The week grid has an hour gutter; the month grid does not.
    await waitFor(() => expect(screen.getByText('05:00')).toBeInTheDocument());
    expect(screen.getByText('Cómo leer la semana')).toBeInTheDocument();
  });
});

describe('failures', () => {
  it('tells the visitor when the calendar cannot be loaded', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => ({ ok: false, status: 500 }) as Response));
    render(<Calendar />);
    expect(await screen.findByText(/No se pudo cargar el calendario/)).toBeInTheDocument();
  });
});
