import type { ApiSeason } from '../../lib/api';
import {
  DIAS_CORTOS,
  celebrationOf,
  monthOwnKeys,
  parseKey,
  seasonOf,
  type DayEvent,
} from '../../lib/calendar';
import { blockStyle, celebStyle } from './rankStyles';

interface Props {
  anchor: string;
  todayK: string;
  seasons: ApiSeason[];
  itemsFor: (key: string) => DayEvent[];
  onOpen: (id: string, key: string) => void;
}

/**
 * The phone month: a 7×6 grid cannot hold a legible chip, so the month becomes
 * a vertical agenda of the days that actually have something on them.
 */
export default function MonthAgenda({ anchor, todayK, seasons, itemsFor, onOpen }: Props) {
  const days = monthOwnKeys(anchor)
    .map((k) => ({ k, items: itemsFor(k) }))
    .filter((d) => d.items.length > 0);

  if (days.length === 0) {
    return (
      <div
        style={{
          flex: '1 1 300px',
          background: 'var(--card)',
          border: '1px solid rgba(34,29,21,.16)',
          borderRadius: 12,
          padding: 22,
          font: "400 14.5px/1.55 'EB Garamond',Georgia,serif",
          color: 'var(--graphite)',
        }}
      >
        Sin actividades publicadas este mes.
      </div>
    );
  }

  return (
    <div
      style={{
        flex: '1 1 300px',
        minWidth: 0,
        background: 'var(--card)',
        border: '1px solid rgba(34,29,21,.16)',
        borderRadius: 12,
        overflow: 'hidden',
      }}
    >
      {days.map(({ k, items }) => {
        const season = seasonOf(k, seasons);
        const celeb = celebrationOf(items);
        const cs = celeb ? celebStyle(celeb) : null;
        const d = parseKey(k);
        const isToday = k === todayK;

        return (
          <div key={k} style={{ display: 'flex', borderBottom: '1px solid rgba(34,29,21,.09)' }}>
            <div style={{ flex: 'none', width: 52, padding: '13px 0 14px 10px' }}>
              <div
                style={{
                  font: "600 10px/1 'IBM Plex Mono',monospace",
                  letterSpacing: '.12em',
                  textTransform: 'uppercase',
                  color: 'var(--graphite)',
                }}
              >
                {DIAS_CORTOS[d.getDay()]}
              </div>
              <div
                style={{
                  marginTop: 4,
                  font: '400 23px/1 Marcellus,Georgia,serif',
                  color: isToday ? 'var(--gold)' : season.ink,
                }}
              >
                {d.getDate()}
              </div>
              {isToday && (
                <div
                  style={{
                    marginTop: 4,
                    font: "600 8px/1 'IBM Plex Mono',monospace",
                    letterSpacing: '.12em',
                    color: 'var(--gold-ring)',
                  }}
                >
                  HOY
                </div>
              )}
            </div>

            <div
              style={{
                flex: 1,
                minWidth: 0,
                padding: '13px 12px 14px',
                background: season.tint,
                borderLeft: `3px solid ${celeb ? celeb.hex : season.color}`,
              }}
            >
              {celeb && cs && (
                <div
                  style={{
                    display: 'inline-block',
                    padding: '4px 8px',
                    borderRadius: 5,
                    marginBottom: 9,
                    background: cs.bg,
                    color: cs.fg,
                    border: `1px solid ${cs.bd}`,
                    font: "600 11.5px/1.3 'IBM Plex Sans',system-ui,sans-serif",
                    fontWeight: cs.fw,
                  }}
                >
                  {celeb.title}
                </div>
              )}

              <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                {items.map((e) => {
                  const bs = blockStyle(e);
                  return (
                    <div
                      key={e.id}
                      onClick={() => onOpen(e.id, k)}
                      style={{
                        display: 'flex',
                        gap: 10,
                        alignItems: 'baseline',
                        padding: '6px 8px',
                        borderRadius: 5,
                        cursor: 'pointer',
                        minHeight: 38,
                        background: bs.bg,
                        borderLeft: `2px ${bs.dash} ${bs.bd}`,
                      }}
                    >
                      <span
                        style={{
                          flex: 'none',
                          font: "500 11px/1.5 'IBM Plex Mono',monospace",
                          width: 38,
                          color: bs.fg,
                        }}
                      >
                        {e.t}
                      </span>
                      <div style={{ flex: 1, minWidth: 0 }}>
                        <div
                          style={{
                            font: "600 13px/1.35 'IBM Plex Sans',system-ui,sans-serif",
                            color: 'var(--panel)',
                          }}
                        >
                          {e.title}
                        </div>
                        <div
                          style={{
                            marginTop: 2,
                            font: "500 8.5px/1 'IBM Plex Mono',monospace",
                            letterSpacing: '.12em',
                            textTransform: 'uppercase',
                            color: 'var(--dim)',
                          }}
                        >
                          {e.groupName}
                        </div>
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
          </div>
        );
      })}
    </div>
  );
}
