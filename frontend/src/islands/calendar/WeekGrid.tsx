import type { ApiSeason } from '../../lib/api';
import {
  DIAS_CORTOS,
  GRID_H,
  H0,
  H1,
  ROW,
  celebrationOf,
  layoutLanes,
  parseKey,
  seasonOf,
  weekKeys,
  type DayEvent,
} from '../../lib/calendar';
import { blockStyle, celebStyle } from './rankStyles';

interface Props {
  anchor: string;
  todayK: string;
  seasons: ApiSeason[];
  itemsFor: (key: string) => DayEvent[];
  nowMin: number;
  onOpen: (id: string) => void;
}

export const HOURS = Array.from({ length: H1 - H0 }, (_, i) => {
  const h = H0 + i;
  return { h, label: `${String(h).padStart(2, '0')}:00`, top: i * ROW };
});

/** Where an event sits in the hour column, in pixels. */
export function blockBox(e: DayEvent) {
  const lanes = e.lanes ?? 1;
  const lane = e.lane ?? 0;
  const width = 100 / lanes;
  const height = Math.max(28, Math.round((e.dur / 60) * ROW) - 3);
  return {
    top: Math.round(((e.min - H0 * 60) / 60) * ROW),
    height,
    left: lane * width,
    width,
    showTime: lanes === 1 || height >= 62,
    fontSize: lanes > 1 ? '9.5px' : '10.5px',
  };
}

export default function WeekGrid({
  anchor,
  todayK,
  seasons,
  itemsFor,
  nowMin,
  onOpen,
}: Props) {
  const keys = weekKeys(anchor);
  const showNow = keys.includes(todayK);
  const nowTop = Math.round(((nowMin - H0 * 60) / 60) * ROW);

  return (
    <div
      style={{
        flex: '1 1 700px',
        minWidth: 0,
        background: 'var(--card)',
        border: '1px solid rgba(34,29,21,.16)',
        borderRadius: 12,
        overflow: 'hidden',
        boxShadow: '0 10px 30px rgba(34,29,21,.07)',
      }}
    >
      <div style={{ display: 'flex', borderBottom: '1px solid rgba(34,29,21,.13)' }}>
        <div
          style={{
            flex: 'none',
            width: 56,
            borderRight: '1px solid rgba(34,29,21,.1)',
            background: 'var(--wash)',
          }}
        />
        {keys.map((k) => {
          const season = seasonOf(k, seasons);
          const celeb = celebrationOf(itemsFor(k));
          const cs = celeb ? celebStyle(celeb) : null;
          const d = parseKey(k);
          const isToday = k === todayK;
          return (
            <div
              key={k}
              style={{
                flex: 1,
                minWidth: 0,
                borderRight: '1px solid rgba(34,29,21,.08)',
                background: isToday ? '#fdf6e8' : 'var(--card)',
              }}
            >
              <div style={{ height: 3, background: celeb ? celeb.hex : season.color }} />
              <div
                style={{
                  padding: '11px 10px 9px',
                  display: 'flex',
                  alignItems: 'baseline',
                  gap: 7,
                }}
              >
                <span
                  style={{
                    font: "600 9.5px/1 'IBM Plex Mono',monospace",
                    letterSpacing: '.14em',
                    textTransform: 'uppercase',
                    color: 'var(--graphite)',
                  }}
                >
                  {DIAS_CORTOS[d.getDay()]}
                </span>
                <span
                  style={{
                    font: '400 21px/1 Marcellus,Georgia,serif',
                    color: isToday ? 'var(--gold)' : season.ink,
                  }}
                >
                  {d.getDate()}
                </span>
              </div>
              <div style={{ padding: '0 7px 9px', minHeight: 40 }}>
                {celeb && cs && (
                  <div
                    style={{
                      padding: '5px 7px',
                      borderRadius: 5,
                      background: cs.bg,
                      color: cs.fg,
                      border: `1px solid ${cs.bd}`,
                      font: "600 10px/1.32 'IBM Plex Sans',system-ui,sans-serif",
                      fontWeight: cs.fw,
                    }}
                  >
                    {celeb.title}
                  </div>
                )}
              </div>
            </div>
          );
        })}
      </div>

      <div style={{ display: 'flex', position: 'relative', background: 'var(--card)' }}>
        <div
          style={{
            flex: 'none',
            width: 56,
            position: 'relative',
            borderRight: '1px solid rgba(34,29,21,.1)',
            background: 'var(--wash)',
            height: GRID_H,
          }}
        >
          {HOURS.map((h) => (
            <div
              key={h.h}
              style={{
                position: 'absolute',
                left: 0,
                right: 7,
                textAlign: 'right',
                top: h.top,
                transform: 'translateY(-6px)',
                font: "500 9.5px/1 'IBM Plex Mono',monospace",
                color: 'var(--graphite)',
              }}
            >
              {h.label}
            </div>
          ))}
        </div>

        {keys.map((k) => {
          const season = seasonOf(k, seasons);
          const blocks = layoutLanes(itemsFor(k));
          return (
            <div
              key={k}
              style={{
                flex: 1,
                minWidth: 0,
                position: 'relative',
                borderRight: '1px solid rgba(34,29,21,.08)',
                background: season.tint,
                height: GRID_H,
              }}
            >
              {HOURS.map((h) => (
                <div
                  key={h.h}
                  style={{
                    position: 'absolute',
                    left: 0,
                    right: 0,
                    height: 1,
                    background: 'rgba(34,29,21,.07)',
                    top: h.top,
                  }}
                />
              ))}
              {blocks.map((e) => {
                const box = blockBox(e);
                const bs = blockStyle(e);
                return (
                  <div
                    key={e.id}
                    onClick={() => onOpen(e.id)}
                    style={{
                      position: 'absolute',
                      overflow: 'hidden',
                      padding: '3px 4px',
                      borderRadius: 5,
                      cursor: 'pointer',
                      transition:
                        'transform calc(.18s * var(--m)),box-shadow calc(.18s * var(--m))',
                      animation: 'cpGrow calc(.42s * var(--m)) cubic-bezier(.16,.84,.24,1) both',
                      transformOrigin: '50% 0',
                      top: box.top,
                      height: box.height,
                      left: `${box.left}%`,
                      width: `${box.width}%`,
                      background: bs.bg,
                      color: bs.fg,
                      border: `1px ${bs.dash} ${bs.bd}`,
                      fontWeight: bs.fw,
                    }}
                  >
                    {box.showTime && (
                      <div style={{ font: "500 9px/1.2 'IBM Plex Mono',monospace", opacity: 0.8 }}>
                        {e.t}
                      </div>
                    )}
                    <div
                      style={{
                        marginTop: 2,
                        lineHeight: 1.16,
                        fontFamily: "'IBM Plex Sans',system-ui,sans-serif",
                        display: '-webkit-box',
                        WebkitLineClamp: 3,
                        WebkitBoxOrient: 'vertical',
                        overflow: 'hidden',
                        overflowWrap: 'anywhere',
                        hyphens: 'auto',
                        fontSize: box.fontSize,
                      }}
                    >
                      {e.title}
                    </div>
                  </div>
                );
              })}
            </div>
          );
        })}

        {showNow && nowTop >= 0 && nowTop <= GRID_H && (
          <div
            style={{
              position: 'absolute',
              left: 56,
              right: 0,
              height: 0,
              pointerEvents: 'none',
              top: nowTop,
            }}
          >
            <div style={{ height: 1.5, background: 'var(--red)', position: 'relative' }}>
              <span
                style={{
                  position: 'absolute',
                  left: -6,
                  top: -4,
                  width: 9,
                  height: 9,
                  borderRadius: '50%',
                  background: 'var(--red)',
                  animation: 'cpPulse calc(2.2s * var(--m)) ease-in-out infinite',
                }}
              />
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
