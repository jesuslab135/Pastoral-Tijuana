import { useCallback, useEffect, useMemo, useRef, useState } from 'preact/hooks';

import {
  fetchEvents,
  fetchGroups,
  fetchSeasons,
  type ApiEvent,
  type ApiGroup,
  type ApiSeason,
} from '../../lib/api';
import {
  monthCellKeys,
  nowMinutes,
  parseKey,
  rangeLabel,
  rangeOf,
  seasonOf,
  shiftAnchor,
  todayKey,
  toDayEvents,
  weekKeys,
  type DayEvent,
} from '../../lib/calendar';

import Banner from './Banner';
import DayPanel from './DayPanel';
import EventSheet from './EventSheet';
import Horarios from './Horarios';
import Leyenda from './Leyenda';
import MonthAgenda from './MonthAgenda';
import MonthGrid from './MonthGrid';
import Toolbar from './Toolbar';
import WeekDayCol from './WeekDayCol';
import WeekGrid from './WeekGrid';

type View = 'mes' | 'semana';

const PHONE = 720;
const TABLET = 1040;

export default function Calendar() {
  const today = useMemo(todayKey, []);

  const [view, setView] = useState<View>('mes');
  const [anchor, setAnchor] = useState(today);
  const [sel, setSel] = useState(today);
  const [filter, setFilter] = useState<string[]>([]);
  const [sheet, setSheet] = useState<string | null>(null);
  const [dayIx, setDayIx] = useState(() => parseKey(today).getDay());

  const [seasons, setSeasons] = useState<ApiSeason[]>([]);
  const [groups, setGroups] = useState<ApiGroup[]>([]);
  const [events, setEvents] = useState<ApiEvent[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [nowMin, setNowMin] = useState(nowMinutes);

  const [width, setWidth] = useState(
    typeof window === 'undefined' ? 1280 : window.innerWidth,
  );
  const isPhone = width < PHONE;
  const isTablet = width < TABLET;

  // Ranges already fetched, so paging back and forth does not refetch.
  const fetched = useRef(new Set<string>());
  const seenIds = useRef(new Set<string>());

  useEffect(() => {
    let raf = 0;
    const onResize = () => {
      if (raf) return;
      raf = requestAnimationFrame(() => {
        raf = 0;
        setWidth(window.innerWidth);
      });
    };
    window.addEventListener('resize', onResize);
    return () => {
      window.removeEventListener('resize', onResize);
      if (raf) cancelAnimationFrame(raf);
    };
  }, []);

  // The red now-line would otherwise freeze at page load.
  useEffect(() => {
    const id = setInterval(() => setNowMin(nowMinutes()), 60_000);
    return () => clearInterval(id);
  }, []);

  useEffect(() => {
    const year = parseKey(anchor).getFullYear();
    fetchSeasons(year)
      .then(setSeasons)
      .catch(() => {
        /* Seasons only tint the calendar; the fallback palette covers it. */
      });
  }, [anchor]);

  useEffect(() => {
    fetchGroups()
      .then(setGroups)
      .catch(() => setError('No se pudieron cargar los grupos.'));
  }, []);

  // Events for whatever the current view can show, fetched once per range.
  useEffect(() => {
    const keys = view === 'mes' ? monthCellKeys(anchor) : weekKeys(anchor);
    const { from, to } = rangeOf(keys);
    const tag = `${from}:${to}`;
    if (fetched.current.has(tag)) return;
    fetched.current.add(tag);

    fetchEvents(from, to)
      .then((incoming) => {
        const fresh = incoming.filter((e) => !seenIds.current.has(e.id));
        if (fresh.length === 0) return;
        for (const e of fresh) seenIds.current.add(e.id);
        setEvents((prev) => [...prev, ...fresh]);
      })
      .catch(() => {
        fetched.current.delete(tag);
        setError('No se pudo cargar el calendario. Revisa tu conexión.');
      });
  }, [anchor, view]);

  const byDay = useMemo(() => toDayEvents(events), [events]);

  const itemsFor = useCallback(
    (key: string): DayEvent[] => {
      const items = byDay.get(key) ?? [];
      if (filter.length === 0) return items;
      return items.filter((e) => filter.includes(e.groupSlug));
    },
    [byDay, filter],
  );

  const todaySeason = useMemo(() => seasonOf(today, seasons), [today, seasons]);

  // The header's logo disc follows the season without the shell knowing how.
  useEffect(() => {
    document.documentElement.style.setProperty('--season-swatch', todaySeason.color);
  }, [todaySeason.color]);

  const openSheet = useCallback(
    (id: string, key?: string) => {
      setSheet(id);
      if (key) {
        setSel(key);
        setDayIx(parseKey(key).getDay());
        setView('semana');
      }
    },
    [],
  );

  const sheetEvent = useMemo(() => {
    if (!sheet) return null;
    for (const items of byDay.values()) {
      const hit = items.find((e) => e.id === sheet);
      if (hit) return hit;
    }
    return null;
  }, [sheet, byDay]);

  const toggleFilter = (slug: string) =>
    setFilter((f) => (f.includes(slug) ? f.filter((s) => s !== slug) : [...f, slug]));

  const go = (dir: -1 | 1) => {
    const next = shiftAnchor(anchor, view, dir);
    setAnchor(next);
    if (view === 'semana') setSel(weekKeys(next)[dayIx]!);
  };

  const goToday = () => {
    setAnchor(today);
    setSel(today);
    setDayIx(parseKey(today).getDay());
  };

  const showMonthGrid = view === 'mes' && !isPhone;
  const showMonthAgenda = view === 'mes' && isPhone;
  const showWeekGrid = view === 'semana' && !isPhone;
  const showWeekDay = view === 'semana' && isPhone;

  return (
    <>
      <Banner season={todaySeason} todayK={today} isPhone={isPhone} />

      <main
        id="calendario"
        style={{
          maxWidth: 1360,
          margin: '0 auto',
          padding: isPhone ? '20px 16px 56px' : '26px 28px 70px',
        }}
      >
        <Toolbar
          view={view}
          rangeLabel={rangeLabel(anchor, view)}
          groups={groups}
          filter={filter}
          onView={(v) => {
            setView(v);
            if (v === 'mes') setSheet(null);
          }}
          onPrev={() => go(-1)}
          onNext={() => go(1)}
          onToday={goToday}
          onToggle={toggleFilter}
        />

        {error && (
          <div
            style={{
              marginBottom: 16,
              padding: '12px 14px',
              borderRadius: 8,
              background: 'var(--sea-rojo-tint)',
              border: '1px solid rgba(160,47,39,.3)',
              font: "400 14px/1.5 'EB Garamond',Georgia,serif",
              color: 'var(--red)',
            }}
          >
            {error}
          </div>
        )}

        <div
          style={{
            display: 'flex',
            gap: 20,
            flexWrap: 'wrap',
            alignItems: 'flex-start',
            animation: 'cpRise calc(.4s * var(--m)) cubic-bezier(.16,.84,.24,1) both',
          }}
        >
          {showMonthGrid && (
            <MonthGrid
              anchor={anchor}
              sel={sel}
              todayK={today}
              seasons={seasons}
              itemsFor={itemsFor}
              cellHeight={isTablet ? 96 : 118}
              onPick={setSel}
            />
          )}

          {showMonthAgenda && (
            <MonthAgenda
              anchor={anchor}
              todayK={today}
              seasons={seasons}
              itemsFor={itemsFor}
              onOpen={openSheet}
            />
          )}

          {showWeekGrid && (
            <WeekGrid
              anchor={anchor}
              todayK={today}
              seasons={seasons}
              itemsFor={itemsFor}
              nowMin={nowMin}
              onOpen={(id) => openSheet(id)}
            />
          )}

          {showWeekDay && (
            <WeekDayCol
              anchor={anchor}
              dayIx={dayIx}
              todayK={today}
              seasons={seasons}
              itemsFor={itemsFor}
              nowMin={nowMin}
              onPickDay={(i, key) => {
                setDayIx(i);
                setSel(key);
              }}
              onOpen={(id) => openSheet(id)}
            />
          )}

          <div
            style={{
              flex: '1 1 300px',
              minWidth: 0,
              display: 'flex',
              flexWrap: 'wrap',
              gap: 16,
              alignItems: 'flex-start',
            }}
          >
            {sheetEvent && <EventSheet event={sheetEvent} onClose={() => setSheet(null)} />}
            {view === 'mes' ? (
              <>
                <DayPanel
                  sel={sel}
                  seasons={seasons}
                  itemsFor={itemsFor}
                  onOpen={(id) => openSheet(id)}
                />
                <Horarios />
              </>
            ) : (
              <>
                <Leyenda />
                <Horarios />
              </>
            )}
          </div>
        </div>
      </main>
    </>
  );
}
