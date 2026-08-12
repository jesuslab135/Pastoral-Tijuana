/** @jsxImportSource react */

// A dónde se fue cada aviso: `project/Admin Difusion.dc.html` lines 74-183,
// bound to the difusión engine's real four states instead of the mock's five.

import { useMemo, useState } from 'react';
import type { CSSProperties } from 'react';

import { useBroadcastsQuery, useRetryBroadcastMutation } from '../api';
import { isoToParishParts, monthWindow, parishStamp } from '../dates';
import { apiError } from '../types';
import type { Broadcast, BroadcastState } from '../types';
import {
  BODY,
  Chip,
  DangerButton,
  ErrorBox,
  GREEN,
  H1,
  Kicker,
  MONO,
  SANS,
  SERIF,
  StatCard,
  Tag,
} from '../ui';
import ChannelsCard from './ChannelsCard';

type TabKey = 'todos' | 'fallos' | 'cola' | 'entregados';

const isFailing = (b: Broadcast) => b.state === 'failed' || b.state === 'dead';

const STATE_META: Record<
  BroadcastState,
  { label: string; color: string; tagBg: string; rowBg: string }
> = {
  sent: { label: 'ENTREGADO', color: GREEN, tagBg: 'rgba(47,107,79,.12)', rowBg: 'var(--card)' },
  queued: { label: 'EN COLA', color: 'var(--gold)', tagBg: 'rgba(177,135,47,.16)', rowBg: '#fdf6e8' },
  failed: { label: 'FALLÓ', color: 'var(--red)', tagBg: 'rgba(160,47,39,.14)', rowBg: '#fbeeec' },
  // The mock has no fifth "AGRUPADO" state on the backend; SIN REINTENTOS
  // reuses failed's red — it is still a message the parish never got.
  dead: { label: 'SIN REINTENTOS', color: 'var(--red)', tagBg: 'rgba(160,47,39,.14)', rowBg: '#fbeeec' },
};

const SIDE_HEADER: CSSProperties = {
  font: `600 9.5px/1 ${MONO}`,
  letterSpacing: '.18em',
  textTransform: 'uppercase',
  marginBottom: 15,
};

const RULES = [
  {
    name: 'Agrupar ediciones',
    desc: 'Cinco correcciones en diez minutos salen como un solo aviso.',
  },
  {
    name: 'Ventana de reposo',
    desc: 'Nada sale entre las 22:00 y las 07:00. Se reprograma, no se descarta.',
  },
  {
    name: 'Avisar cancelaciones',
    desc: 'Borrar un evento manda mensaje a los mismos canales que lo recibieron.',
  },
  {
    name: 'Solo cambios que importan',
    desc: 'Fecha, hora, lugar o cancelación disparan aviso. Una coma, no.',
  },
];

function RulesCard() {
  return (
    <div
      style={{
        flex: '1 1 288px',
        minWidth: 0,
        background: 'var(--panel)',
        color: 'var(--panel-fg)',
        borderRadius: 12,
        padding: 20,
      }}
    >
      <div style={{ ...SIDE_HEADER, color: 'var(--accent)' }}>Reglas activas</div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 13 }}>
        {RULES.map((r) => (
          <div key={r.name}>
            <div style={{ font: `600 12.5px/1.3 ${SANS}`, color: 'var(--panel-bright)', marginBottom: 3 }}>
              {r.name}
            </div>
            <div style={{ font: `400 13.5px/1.5 ${BODY}`, color: 'var(--dim)' }}>{r.desc}</div>
          </div>
        ))}
      </div>
    </div>
  );
}

// Replaces the mock's "Salud del proveedor" panel: there is no real WhatsApp
// session to report on in v1, so this explains what the log rows actually mean.
function SimuladoCard() {
  return (
    <div
      style={{
        flex: '1 1 288px',
        minWidth: 0,
        background: '#fbeeec',
        border: '1px solid rgba(160,47,39,.3)',
        borderRadius: 12,
        padding: 20,
      }}
    >
      <div style={{ ...SIDE_HEADER, color: 'var(--red)', marginBottom: 12 }}>
        Aviso: v1 es simulado
      </div>
      <p style={{ margin: 0, font: `400 14.5px/1.55 ${BODY}`, color: 'var(--muted)' }}>
        El WhatsApp de v1 es simulado: el panel registra a dónde <em>iría</em> cada aviso sin
        enviarlo. El correo sí sale de verdad. Un proveedor real se conecta detrás de la misma
        interfaz sin tocar el motor.
      </p>
    </div>
  );
}

function LogRow({ b }: { b: Broadcast }) {
  const meta = STATE_META[b.state];
  const failing = isFailing(b);
  const [retryBroadcast, retrying] = useRetryBroadcastMutation();
  const [retryError, setRetryError] = useState<string | null>(null);

  async function onRetry() {
    setRetryError(null);
    try {
      await retryBroadcast(b.id).unwrap();
    } catch (e) {
      // A 409 means the row is no longer retryable (dead, or already resolved);
      // the API's Spanish message explains why, so it replaces the last error.
      setRetryError(apiError(e));
    }
  }

  return (
    <div
      style={{
        padding: '15px 17px',
        borderBottom: '1px solid rgba(34,29,21,.08)',
        borderLeft: `3px solid ${meta.color}`,
        background: meta.rowBg,
      }}
    >
      <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', alignItems: 'baseline', marginBottom: 6 }}>
        <span style={{ font: `600 13px/1.3 ${SANS}`, flex: 1, minWidth: 140 }}>{b.event_title}</span>
        <Tag bg={meta.tagBg} fg={meta.color}>
          {meta.label}
        </Tag>
      </div>
      <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap', alignItems: 'baseline' }}>
        <span style={{ font: `400 11.5px/1.4 ${SANS}`, color: 'var(--muted)' }}>
          {b.channel_name}
          {b.simulated ? ' · SIMULADO' : ''}
        </span>
        <span style={{ font: `400 10.5px/1.4 ${MONO}`, color: 'var(--graphite)' }}>
          {parishStamp(b.sent_at ?? b.created_at)}
        </span>
      </div>
      {failing && (
        <>
          <div
            style={{
              marginTop: 9,
              padding: '9px 11px',
              borderRadius: 6,
              background: 'rgba(160,47,39,.07)',
              border: '1px solid rgba(160,47,39,.2)',
              font: `400 11px/1.45 ${MONO}`,
              color: '#8f2a23',
            }}
          >
            {retryError ?? b.last_error}
          </div>
          <div style={{ display: 'flex', gap: 7, marginTop: 9, flexWrap: 'wrap' }}>
            <DangerButton
              onClick={onRetry}
              disabled={retrying.isLoading}
              style={{ padding: '9px 13px', borderRadius: 6, minHeight: 38, font: `600 11px/1 ${SANS}` }}
            >
              Reintentar ahora
            </DangerButton>
            <a
              href="#canales"
              style={{
                padding: '9px 13px',
                borderRadius: 6,
                border: '1px solid rgba(34,29,21,.16)',
                background: 'var(--card)',
                color: 'var(--muted)',
                font: `600 11px/1 ${SANS}`,
                minHeight: 38,
                lineHeight: '20px',
                display: 'inline-flex',
                alignItems: 'center',
              }}
            >
              Revisar el canal
            </a>
          </div>
        </>
      )}
    </div>
  );
}

export default function Difusion() {
  const month = useMemo(() => monthWindow(new Date()), []);
  const { data: broadcasts, isLoading, isError, error } = useBroadcastsQuery({});
  const anyQueued = (broadcasts ?? []).some((b) => b.state === 'queued');
  // Re-subscribes to the same cache entry, this time asking to poll: an idle
  // panel with nothing queued has no reason to hit the API every 5s.
  useBroadcastsQuery({}, { pollingInterval: anyQueued ? 5000 : 0 });

  const [tab, setTab] = useState<TabKey>('todos');

  const sorted = useMemo(
    () =>
      [...(broadcasts ?? [])].sort(
        (a, b) => Date.parse(b.sent_at ?? b.created_at) - Date.parse(a.sent_at ?? a.created_at),
      ),
    [broadcasts],
  );

  const inMonth = (iso: string) => {
    const d = isoToParishParts(iso).date;
    return d >= month.from && d < month.to;
  };

  const sentThisMonth = sorted.filter(
    (b) => b.state === 'sent' && b.sent_at !== null && inMonth(b.sent_at),
  ).length;
  const queuedCount = sorted.filter((b) => b.state === 'queued').length;
  const failedCount = sorted.filter(isFailing).length;
  const entregadosCount = sorted.filter((b) => b.state === 'sent').length;

  const tabs: Array<[TabKey, string, number]> = [
    ['todos', 'Todo', sorted.length],
    ['fallos', 'Fallos', failedCount],
    ['cola', 'En cola', queuedCount],
    ['entregados', 'Entregados', entregadosCount],
  ];

  const rows =
    tab === 'fallos'
      ? sorted.filter(isFailing)
      : tab === 'cola'
        ? sorted.filter((b) => b.state === 'queued')
        : tab === 'entregados'
          ? sorted.filter((b) => b.state === 'sent')
          : sorted;

  return (
    <div>
      <Kicker style={{ marginBottom: 11 }}>Motor de difusión</Kicker>
      <H1 style={{ marginBottom: 8 }}>A dónde se fue cada aviso</H1>
      <p style={{ margin: '0 0 26px', maxWidth: '66ch', font: `400 16px/1.58 ${BODY}`, color: 'var(--muted)' }}>
        Nada se manda a mano. Cada evento publicado abre una tarea por canal, con su propio ritmo y
        sus reintentos.
      </p>

      {isError ? (
        <ErrorBox>{apiError(error)}</ErrorBox>
      ) : isLoading ? (
        <p style={{ font: `400 15px/1.55 ${BODY}`, color: 'var(--graphite)' }}>Cargando…</p>
      ) : (
        <>
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(auto-fit,minmax(150px,1fr))',
              gap: 12,
              marginBottom: 24,
            }}
          >
            <StatCard value={sentThisMonth} color={GREEN}>
              entregados
              <br />
              este mes
            </StatCard>
            <StatCard value={queuedCount} color="var(--gold-ring)">
              en cola
              <br />
              ahora mismo
            </StatCard>
            <StatCard
              value={failedCount}
              color="var(--red)"
              style={{ background: '#fbeeec', border: '1px solid rgba(160,47,39,.28)' }}
            >
              fallidos
              <br />
              con reintento
            </StatCard>
            {/* Static: the engine's stagger between channel sends is a compile-time
                constant, not a per-parish setting the panel can read or change. */}
            <StatCard value="8 s" color="#5c3b7a">
              retraso entre
              <br />
              canal y canal
            </StatCard>
          </div>

          <div style={{ display: 'flex', gap: 20, flexWrap: 'wrap', alignItems: 'flex-start' }}>
            <div style={{ flex: '1 1 460px', minWidth: 0 }}>
              <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginBottom: 14 }}>
                {tabs.map(([key, label, count]) => (
                  <Chip key={key} on={tab === key} onClick={() => setTab(key)}>
                    {label} · {count}
                  </Chip>
                ))}
              </div>

              <div
                style={{
                  background: 'var(--card)',
                  border: '1px solid rgba(34,29,21,.15)',
                  borderRadius: 12,
                  overflow: 'hidden',
                }}
              >
                {rows.map((b) => (
                  <LogRow key={b.id} b={b} />
                ))}
                {rows.length === 0 && (
                  <div style={{ padding: '36px 20px', textAlign: 'center' }}>
                    <div
                      style={{
                        font: `400 19px/1.2 ${SERIF}`,
                        color: 'var(--graphite)',
                        marginBottom: 7,
                      }}
                    >
                      Nada en este filtro
                    </div>
                    <p style={{ margin: 0, font: `400 14.5px/1.5 ${BODY}`, color: 'var(--graphite)' }}>
                      Buena señal si buscabas fallos.
                    </p>
                  </div>
                )}
              </div>
            </div>

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
              <ChannelsCard />
              <RulesCard />
              <SimuladoCard />
            </div>
          </div>
        </>
      )}
    </div>
  );
}
