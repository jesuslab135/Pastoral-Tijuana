/** @jsxImportSource react */

// The month the parish secretary lives in: `project/Admin Eventos.dc.html`
// lines 76-171 (lista mode), bound to the admin API.

import { useMemo, useState } from 'react';
import type { CSSProperties, ReactNode } from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';

import type { SeasonColor } from '../../lib/api';
import { SEASON_PALETTE } from '../../lib/calendar';
import { useIsPhone } from '../Shell';
import { useBroadcastsQuery, useEventsQuery, useGroupsQuery } from '../api';
import { isoToParishParts, monthWindow, parishDayParts } from '../dates';
import { apiError } from '../types';
import type { AdminEvent, Rank } from '../types';
import {
  BODY,
  Chip,
  DangerButton,
  ErrorBox,
  GREEN,
  GhostButton,
  H1,
  Kicker,
  MONO,
  PrimaryButton,
  SANS,
  SERIF,
  StatCard,
  Tag,
} from '../ui';
import { castsByEvent } from './casts';
import type { Cast } from './casts';

/** The mock's gold-washed stat card, a one-off with no token of its own. */
const GOLD_WASH = '#fdf6e8';
const GOLD_WASH_LINE = 'rgba(177,135,47,.34)';

/** Día · Evento · Rango · Difusión · acciones (mock line 112). */
const COLUMNS = '72px minmax(0,1fr) 104px 92px 88px';

// The liturgy's two loud ranks wear gold; a memoria or a group activity is
// graphite, the same hierarchy the public calendar draws.
const RANK_COLOR: Record<Rank, string> = {
  solemnidad: 'var(--gold)',
  fiesta: 'var(--gold)',
  memoria: 'var(--graphite)',
  parroquial: 'var(--graphite)',
};

type TabKey = 'todos' | 'publicados' | 'borradores' | 'fallo' | 'cancelados';

const DAY_MS = 86_400_000;

const isDraft = (e: AdminEvent) => e.published_at === null;

/**
 * Whole parish days from today to the event, so "en 3 días" counts calendar
 * days the way the secretary does rather than 24-hour blocks.
 */
function daysUntil(iso: string): number {
  const at = (x: string) => Date.parse(`${isoToParishParts(x).date}T00:00:00Z`);
  return Math.round((at(iso) - at(new Date().toISOString())) / DAY_MS);
}

/**
 * The month the list should open on: the one carried back from the editor as
 * a `YYYY-MM-DD`, or the current month. The parts are read off the string
 * rather than parsed as an instant, so the anchor lands in the month the
 * secretary typed whatever zone the browser is in.
 */
function monthAnchor(state: unknown): Date {
  const saved = (state as { month?: string } | null)?.month;
  const [year, month] = (saved ?? '').split('-').map(Number);
  return year && month ? new Date(year, month - 1, 1) : new Date();
}

function untilLabel(iso: string): string {
  const n = daysUntil(iso);
  if (n <= 0) return 'hoy';
  if (n === 1) return 'mañana';
  return `en ${n} días`;
}

/** A patronal feast may claim a liturgical color of its own; rank decides otherwise. */
function dayColor(e: AdminEvent): string {
  const own = e.color_override
    ? SEASON_PALETTE[e.color_override as SeasonColor]
    : undefined;
  return own ? own.ink : RANK_COLOR[e.rank];
}

interface CastView {
  text: string;
  color: string;
  dot: boolean;
}

function castView(e: AdminEvent, cast: Cast | undefined): CastView {
  // A draft was never announced, and a just-published event has no outbox rows
  // until the worker picks them up: both read as "nothing to report yet".
  if (isDraft(e) || !cast || cast.total === 0) {
    return { text: '—', color: 'var(--graphite)', dot: false };
  }
  const color = cast.failed
    ? 'var(--red)'
    : cast.sent === cast.total
      ? GREEN
      : 'var(--gold-ring)';
  return { text: `${cast.sent}/${cast.total}`, color, dot: true };
}

function Panel({ children }: { children: ReactNode }) {
  return (
    <div
      style={{
        background: 'var(--card)',
        border: '1px solid rgba(34,29,21,.15)',
        borderRadius: 12,
        overflow: 'hidden',
      }}
    >
      {children}
    </div>
  );
}

function Note({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div style={{ padding: '38px 20px', textAlign: 'center' }}>
      <div style={{ font: `400 19px/1.2 ${SERIF}`, color: 'var(--graphite)', marginBottom: 7 }}>
        {title}
      </div>
      <p style={{ margin: 0, font: `400 14.5px/1.5 ${BODY}`, color: 'var(--graphite)' }}>
        {children}
      </p>
    </div>
  );
}

interface RowProps {
  event: AdminEvent;
  group: string;
  cast: Cast | undefined;
  isPhone: boolean;
  onEdit: () => void;
  onDelete: () => void;
}

function Row({ event, group, cast, isPhone, onEdit, onDelete }: RowProps) {
  const [over, setOver] = useState(false);
  const { day, dow, time } = parishDayParts(event.starts_at);
  const cancelled = event.cancelled_at !== null;
  const draft = isDraft(event);
  const view = castView(event, cast);
  const accent = dayColor(event);

  const base = cancelled
    ? 'rgba(160,47,39,.05)'
    : draft
      ? GOLD_WASH
      : 'var(--card)';
  const background = over && !isPhone ? 'var(--wash)' : base;

  const titleStyle: CSSProperties = {
    font: `600 ${isPhone ? 14 : 13.5}px/1.3 ${SANS}`,
    ...(cancelled ? { textDecoration: 'line-through', color: 'var(--graphite)' } : null),
  };

  const stateTag = cancelled ? (
    <Tag bg="var(--red)" fg="#fff">
      CANCELADO
    </Tag>
  ) : draft ? (
    <Tag bg="var(--gold)" fg="#fff">
      BORRADOR
    </Tag>
  ) : null;

  const castChip = (size: number) => {
    const chip = (
      <span
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 6,
          font: `500 ${size}px/1.3 ${MONO}`,
          color: view.color,
        }}
      >
        {view.dot && (
          <span
            style={{
              width: 6,
              height: 6,
              borderRadius: '50%',
              flex: 'none',
              background: view.color,
            }}
          />
        )}
        {view.text}
      </span>
    );
    // The footer promises a red number leads to the provider's own error.
    return view.dot && cast?.failed ? <Link to="/difusion">{chip}</Link> : chip;
  };

  const rank = (size: number) => (
    <span
      style={{
        font: `500 ${size}px/1.3 ${MONO}`,
        letterSpacing: '.1em',
        color: RANK_COLOR[event.rank],
      }}
    >
      {event.rank.toUpperCase()}
    </span>
  );

  if (isPhone) {
    return (
      <div
        style={{
          padding: '14px 15px',
          borderBottom: '1px solid rgba(34,29,21,.08)',
          borderLeft: `3px solid ${accent}`,
          background,
        }}
      >
        <div style={{ display: 'flex', alignItems: 'baseline', gap: 9, marginBottom: 7 }}>
          <span style={{ font: `500 12px/1 ${MONO}`, color: accent }}>
            {dow} {day}
          </span>
          <span style={{ font: `500 11px/1 ${MONO}`, color: 'var(--graphite)' }}>{time}</span>
          <span style={{ marginLeft: 'auto' }}>{castChip(9.5)}</span>
        </div>
        <div style={{ ...titleStyle, marginBottom: 5 }}>{event.title}</div>
        <div
          style={{
            display: 'flex',
            gap: 10,
            flexWrap: 'wrap',
            alignItems: 'center',
            marginBottom: 11,
          }}
        >
          {rank(9)}
          <span style={{ font: `400 11px/1 ${SANS}`, color: 'var(--graphite)' }}>{group}</span>
          {stateTag}
        </div>
        <div style={{ display: 'flex', gap: 7 }}>
          <GhostButton
            onClick={onEdit}
            style={{
              flex: 1,
              padding: 11,
              borderRadius: 7,
              border: '1px solid rgba(34,29,21,.18)',
              font: `600 11.5px/1 ${SANS}`,
              minHeight: 44,
            }}
          >
            Editar
          </GhostButton>
          <DangerButton
            onClick={onDelete}
            style={{
              flex: 'none',
              padding: '11px 16px',
              borderRadius: 7,
              border: '1px solid rgba(160,47,39,.26)',
              font: `600 11.5px/1 ${SANS}`,
              minHeight: 44,
            }}
          >
            Borrar
          </DangerButton>
        </div>
      </div>
    );
  }

  return (
    <div
      onMouseEnter={() => setOver(true)}
      onMouseLeave={() => setOver(false)}
      style={{
        display: 'grid',
        gridTemplateColumns: COLUMNS,
        gap: 12,
        padding: '14px 18px',
        alignItems: 'center',
        borderBottom: '1px solid rgba(34,29,21,.08)',
        transition: 'background calc(.2s * var(--m))',
        background,
      }}
    >
      <span style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
        <span style={{ font: `500 13px/1 ${MONO}`, color: accent }}>{day}</span>
        <span
          style={{
            font: `400 10px/1 ${MONO}`,
            letterSpacing: '.08em',
            textTransform: 'uppercase',
            color: 'var(--graphite)',
          }}
        >
          {dow} · {time}
        </span>
      </span>

      <span style={{ minWidth: 0, display: 'flex', flexDirection: 'column', gap: 3 }}>
        <span style={{ display: 'flex', alignItems: 'center', gap: 8, minWidth: 0 }}>
          <span
            style={{
              ...titleStyle,
              // Without it the flex item refuses to shrink and a long title
              // pushes the BORRADOR tag out of the column instead of ellipsing.
              minWidth: 0,
              whiteSpace: 'nowrap',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
            }}
          >
            {event.title}
          </span>
          {stateTag}
        </span>
        <span
          style={{
            font: `400 11px/1.3 ${SANS}`,
            color: 'var(--graphite)',
            whiteSpace: 'nowrap',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
          }}
        >
          {group}
        </span>
      </span>

      {rank(9.5)}
      {castChip(10)}

      <span style={{ display: 'flex', gap: 5, justifyContent: 'flex-end' }}>
        <GhostButton onClick={onEdit}>Editar</GhostButton>
        <DangerButton onClick={onDelete} aria-label={`Borrar ${event.title}`}>
          ✕
        </DangerButton>
      </span>
    </div>
  );
}

export default function EventosList() {
  const navigate = useNavigate();
  const isPhone = useIsPhone();
  const location = useLocation();
  // The editor hands back the date it just saved so the list opens on that
  // month instead of on today's; without it an event created for a later
  // month landed off-screen and read as a save that never happened.
  const [anchor, setAnchor] = useState(() => monthAnchor(location.state));
  const [tab, setTab] = useState<TabKey>('todos');

  const month = useMemo(() => monthWindow(anchor), [anchor]);
  // The label never reaches the query: it would split the cache entry the shell
  // already filled for this same window.
  const {
    data: events,
    isLoading,
    isError,
    error,
  } = useEventsQuery({ from: month.from, to: month.to });
  // Unfiltered, so the shell, this screen and Difusión share one cache entry.
  const { data: broadcasts } = useBroadcastsQuery({});
  const { data: groups } = useGroupsQuery();

  const casts = useMemo(() => castsByEvent(broadcasts ?? []), [broadcasts]);
  const groupNames = useMemo(
    () => new Map((groups ?? []).map((g) => [g.id, g.name])),
    [groups],
  );

  const sorted = useMemo(
    () =>
      [...(events ?? [])].sort(
        (a, b) => Date.parse(a.starts_at) - Date.parse(b.starts_at),
      ),
    [events],
  );

  // A cancelled event is a tombstone, not a lighter kind of draft: it leaves
  // the working tabs entirely and keeps its own.
  const live = sorted.filter((e) => e.cancelled_at === null);
  const cancelled = sorted.filter((e) => e.cancelled_at !== null);
  const published = live.filter((e) => !isDraft(e));
  const drafts = live.filter(isDraft);
  const broken = live.filter((e) => casts.get(e.id)?.failed);

  const nextSolemnidad = live.find(
    (e) => e.rank === 'solemnidad' && Date.parse(e.starts_at) > Date.now(),
  );

  const tabs: Array<[TabKey, string, number]> = [
    ['todos', 'Todos', live.length],
    ['publicados', 'Publicados', published.length],
    ['borradores', 'Borradores', drafts.length],
    ['fallo', 'Con fallo', broken.length],
  ];
  if (cancelled.length > 0) tabs.push(['cancelados', 'Cancelados', cancelled.length]);

  // Paging to a month with nothing cancelled would otherwise leave the list on
  // a tab that no longer has a chip to switch away from.
  const active: TabKey = tab === 'cancelados' && cancelled.length === 0 ? 'todos' : tab;
  const rows =
    active === 'publicados'
      ? published
      : active === 'borradores'
        ? drafts
        : active === 'fallo'
          ? broken
          : active === 'cancelados'
            ? cancelled
            : live;

  const stepMonth = (delta: number) =>
    setAnchor((a) => new Date(a.getFullYear(), a.getMonth() + delta, 1));

  const navBtn: CSSProperties = { minHeight: 38, padding: '0 12px', font: `600 13px/1 ${SANS}` };

  return (
    <div>
      <div
        style={{
          display: 'flex',
          gap: 16,
          flexWrap: 'wrap',
          alignItems: 'flex-end',
          marginBottom: 22,
        }}
      >
        <div style={{ flex: 1, minWidth: 220 }}>
          <Kicker style={{ marginBottom: 11 }}>{month.label} · Eventos</Kicker>
          <H1>Eventos del mes</H1>
        </div>
        <div style={{ display: 'flex', gap: 6, flex: 'none' }}>
          <GhostButton onClick={() => stepMonth(-1)} aria-label="Mes anterior" style={navBtn}>
            ‹
          </GhostButton>
          <GhostButton onClick={() => setAnchor(new Date())} style={navBtn}>
            Hoy
          </GhostButton>
          <GhostButton onClick={() => stepMonth(1)} aria-label="Mes siguiente" style={navBtn}>
            ›
          </GhostButton>
        </div>
        <PrimaryButton style={{ flex: 'none' }} onClick={() => navigate('/eventos/nuevo')}>
          + Nuevo evento
        </PrimaryButton>
      </div>

      {isError ? (
        <ErrorBox>{apiError(error)}</ErrorBox>
      ) : isLoading ? (
        <Panel>
          <Note title="Cargando el mes">Un momento, se está leyendo el calendario.</Note>
        </Panel>
      ) : (
        <>
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(auto-fit,minmax(132px,1fr))',
              gap: 12,
              marginBottom: 22,
            }}
          >
            <StatCard value={published.length} color={GREEN}>
              publicados
              <br />
              y visibles
            </StatCard>
            <StatCard value={drafts.length} color="var(--gold-ring)">
              borradores
              <br />
              sin difundir
            </StatCard>
            <StatCard value={broken.length} color="var(--red)">
              difusión
              <br />
              con fallo
            </StatCard>
            {nextSolemnidad && (
              <StatCard
                value={Number(parishDayParts(nextSolemnidad.starts_at).day)}
                color="var(--gold)"
                style={{ background: GOLD_WASH, border: `1px solid ${GOLD_WASH_LINE}` }}
              >
                «{nextSolemnidad.title}»
                <br />
                {untilLabel(nextSolemnidad.starts_at)}
              </StatCard>
            )}
          </div>

          <div
            style={{
              display: 'flex',
              gap: 8,
              flexWrap: 'wrap',
              alignItems: 'center',
              marginBottom: 16,
            }}
          >
            {tabs.map(([key, label, count]) => (
              <Chip key={key} on={active === key} onClick={() => setTab(key)}>
                {label} · {count}
              </Chip>
            ))}
          </div>

          <Panel>
            {!isPhone && (
              <div
                style={{
                  display: 'grid',
                  gridTemplateColumns: COLUMNS,
                  gap: 12,
                  padding: '11px 18px',
                  background: 'var(--wash)',
                  borderBottom: '1px solid rgba(34,29,21,.11)',
                  font: `600 10px/1 ${MONO}`,
                  letterSpacing: '.12em',
                  textTransform: 'uppercase',
                  color: 'var(--graphite)',
                }}
              >
                <span>Día</span>
                <span>Evento</span>
                <span>Rango</span>
                <span>Difusión</span>
                <span />
              </div>
            )}
            {rows.map((e) => (
              <Row
                key={e.id}
                event={e}
                group={groupNames.get(e.group_id) ?? '—'}
                cast={casts.get(e.id)}
                isPhone={isPhone}
                onEdit={() => navigate(`/eventos/${e.id}`)}
                // The editor owns the confirmation modal; the list only says
                // which event the secretary meant to remove.
                onDelete={() => navigate(`/eventos/${e.id}`, { state: { delete: true } })}
              />
            ))}
            {rows.length === 0 && (
              <Note title="Nada en este filtro">Cambia de pestaña o crea un evento nuevo.</Note>
            )}
          </Panel>

          <p
            style={{
              margin: '18px 0 0',
              font: `400 14px/1.55 ${BODY}`,
              color: 'var(--graphite)',
              maxWidth: '76ch',
            }}
          >
            La columna de <strong style={{ fontWeight: 600 }}>difusión</strong> cuenta cuántos
            canales recibieron el aviso. Un número en rojo es clicable hasta el error concreto del
            proveedor, en <Link to="/difusion">Difusión</Link>.
          </p>
        </>
      )}
    </div>
  );
}
