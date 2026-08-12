import type { ApiSeason } from '../../lib/api';
import {
  DIAS,
  DIAS_CORTOS,
  GRID_H,
  H0,
  ROW,
  celebrationOf,
  layoutLanes,
  parseKey,
  seasonOf,
  weekKeys,
  type DayEvent,
} from '../../lib/calendar';
import { blockStyle, celebStyle } from './rankStyles';
import { HOURS, blockBox } from './WeekGrid';

interface Props {
  anchor: string;
  dayIx: number;
  todayK: string;
  seasons: ApiSeason[];
  itemsFor: (key: string) => DayEvent[];
  nowMin: number;
  onPickDay: (ix: number, key: string) => void;
  onOpen: (id: string) => void;
}

/** The phone week: a day picker over a single full-width day column. */
export default function WeekDayCol({
  anchor,
  dayIx,
  todayK,
  seasons,
  itemsFor,
  nowMin,
  onPickDay,
  onOpen,
}: Props) {
  const keys = weekKeys(anchor);
  const ix = Math.min(6, Math.max(0, dayIx));
  const key = keys[ix]!;
  const season = seasonOf(key, seasons);
  const items = itemsFor(key);
  const celeb = celebrationOf(items);
  const cs = celeb ? celebStyle(celeb) : null;
  const d = parseKey(key);
  const isToday = key === todayK;
  const nowTop = Math.round(((nowMin - H0 * 60) / 60) * ROW);

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
      <div
        style={{
          display: 'flex',
          gap: 5,
          padding: '11px 10px',
          borderBottom: '1px solid rgba(34,29,21,.11)',
          background: 'var(--wash)',
        }}
      >
        {keys.map((k, i) => {
          const s = seasonOf(k, seasons);
          const c = celebrationOf(itemsFor(k));
          const on = i === ix;
          return (
            <button
              key={k}
              type="button"
              onClick={() => onPickDay(i, k)}
              style={{
                flex: 1,
                minWidth: 0,
                border: 'none',
                borderRadius: 8,
                padding: '8px 2px 7px',
                cursor: 'pointer',
                minHeight: 48,
                transition: 'all calc(.2s * var(--m))',
                background: on ? 'var(--panel)' : 'var(--card)',
                color: on ? 'var(--panel-bright)' : s.ink,
              }}
            >
              <div
                style={{
                  height: 3,
                  borderRadius: 2,
                  margin: '0 auto 6px',
                  width: '60%',
                  background: c ? c.hex : s.color,
                }}
              />
              <div
                style={{
                  font: "600 8.5px/1 'IBM Plex Mono',monospace",
                  letterSpacing: '.1em',
                  textTransform: 'uppercase',
                  opacity: 0.7,
                }}
              >
                {DIAS_CORTOS[parseKey(k).getDay()]}
              </div>
              <div style={{ marginTop: 3, font: "500 14px/1 'IBM Plex Mono',monospace" }}>
                {parseKey(k).getDate()}
              </div>
            </button>
          );
        })}
      </div>

      <div
        style={{
          padding: '13px 14px 12px',
          borderBottom: '1px solid rgba(34,29,21,.1)',
          background: isToday ? '#fdf6e8' : 'var(--card)',
        }}
      >
        <div
          style={{
            font: '400 20px/1.1 Marcellus,Georgia,serif',
            color: isToday ? 'var(--gold)' : season.ink,
            marginBottom: 8,
          }}
        >
          {DIAS[d.getDay()]} {d.getDate()}
        </div>
        {celeb && cs && (
          <div
            style={{
              display: 'inline-block',
              padding: '5px 9px',
              borderRadius: 5,
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
      </div>

      <div style={{ display: 'flex', position: 'relative' }}>
        <div
          style={{
            flex: 'none',
            width: 46,
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
                right: 6,
                textAlign: 'right',
                transform: 'translateY(-6px)',
                font: "500 9.5px/1 'IBM Plex Mono',monospace",
                color: 'var(--graphite)',
                top: h.top,
              }}
            >
              {h.label}
            </div>
          ))}
        </div>

        <div
          style={{
            flex: 1,
            minWidth: 0,
            position: 'relative',
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

          {layoutLanes(items).map((e) => {
            const box = blockBox(e);
            const bs = blockStyle(e);
            return (
              <div
                key={e.id}
                onClick={() => onOpen(e.id)}
                style={{
                  position: 'absolute',
                  overflow: 'hidden',
                  padding: '5px 8px',
                  borderRadius: 5,
                  cursor: 'pointer',
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
                <div style={{ font: "500 9.5px/1.2 'IBM Plex Mono',monospace", opacity: 0.8 }}>
                  {e.t}
                </div>
                <div
                  style={{
                    marginTop: 2,
                    fontSize: 12,
                    lineHeight: 1.25,
                    fontFamily: "'IBM Plex Sans',system-ui,sans-serif",
                    overflowWrap: 'anywhere',
                  }}
                >
                  {e.title}
                </div>
              </div>
            );
          })}

          {isToday && nowTop >= 0 && nowTop <= GRID_H && (
            <div
              style={{
                position: 'absolute',
                left: 0,
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
                    left: -5,
                    top: -4,
                    width: 9,
                    height: 9,
                    borderRadius: '50%',
                    background: 'var(--red)',
                  }}
                />
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
