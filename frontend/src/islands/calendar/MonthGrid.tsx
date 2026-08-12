import type { ApiSeason } from '../../lib/api';
import {
  DIAS_CORTOS,
  celebrationOf,
  monthCellKeys,
  parseKey,
  seasonOf,
  type DayEvent,
} from '../../lib/calendar';
import { celebStyle } from './rankStyles';

interface Props {
  anchor: string;
  sel: string;
  todayK: string;
  seasons: ApiSeason[];
  itemsFor: (key: string) => DayEvent[];
  cellHeight: number;
  onPick: (key: string) => void;
}

export default function MonthGrid({
  anchor,
  sel,
  todayK,
  seasons,
  itemsFor,
  cellHeight,
  onPick,
}: Props) {
  const anchorMonth = parseKey(anchor).getMonth();
  const keys = monthCellKeys(anchor);

  return (
    <div
      style={{
        flex: '1 1 660px',
        minWidth: 0,
        background: 'var(--card)',
        border: '1px solid rgba(34,29,21,.16)',
        borderRadius: 12,
        overflow: 'hidden',
        boxShadow: '0 10px 30px rgba(34,29,21,.07)',
      }}
    >
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(7,minmax(0,1fr))',
          background: 'var(--wash)',
          borderBottom: '1px solid rgba(34,29,21,.11)',
        }}
      >
        {DIAS_CORTOS.map((d, i) => (
          <div
            key={d}
            style={{
              padding: '10px 11px',
              font: `600 ${i === 0 ? '9.5px' : '10.5px'}/1 'IBM Plex Mono',monospace`,
              letterSpacing: i === 0 ? '.14em' : '.12em',
              // Sunday is the Lord's day; it reads differently on purpose.
              color: i === 0 ? 'var(--red)' : 'var(--graphite)',
              textTransform: 'uppercase',
            }}
          >
            {d}
          </div>
        ))}
      </div>

      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(7,minmax(0,1fr))',
          gap: 1,
          background: 'rgba(34,29,21,.1)',
        }}
      >
        {keys.map((k) => {
          const outside = parseKey(k).getMonth() !== anchorMonth;
          const season = seasonOf(k, seasons);
          const items = itemsFor(k);
          const celeb = celebrationOf(items);
          const dayColor = celeb ? celeb.hex : season.color;
          const chips = items.filter((e) => e !== celeb);
          // A celebration takes one of the two lines the cell can hold.
          const room = celeb ? 1 : 2;
          const shown = chips.slice(0, room);
          const extra = chips.length - shown.length;

          const ring =
            k === sel
              ? '2px solid #221d15'
              : k === todayK
                ? '2px solid #b1872f'
                : '1px solid rgba(34,29,21,.09)';

          const cs = celeb ? celebStyle(celeb) : null;

          return (
            <div
              key={k}
              onClick={() => onPick(k)}
              style={{
                position: 'relative',
                padding: '8px 8px 9px',
                cursor: 'pointer',
                display: 'flex',
                flexDirection: 'column',
                gap: 4,
                transition: 'filter calc(.18s * var(--m))',
                minHeight: cellHeight,
                background: outside ? '#f3efe5' : season.tint,
                outline: ring,
                outlineOffset: -1,
              }}
            >
              <div
                style={{
                  position: 'absolute',
                  top: 0,
                  left: 0,
                  right: 0,
                  height: 2,
                  background: outside ? 'rgba(34,29,21,.12)' : dayColor,
                }}
              />
              <div
                style={{
                  display: 'flex',
                  alignItems: 'baseline',
                  justifyContent: 'space-between',
                  gap: 5,
                }}
              >
                <span
                  style={{
                    font: "500 13px/1 'IBM Plex Mono',monospace",
                    color: outside ? '#6f6557' : season.ink,
                  }}
                >
                  {parseKey(k).getDate()}
                </span>
              </div>

              {celeb && cs && (
                <div
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 5,
                    padding: '3px 6px',
                    borderRadius: 4,
                    overflow: 'hidden',
                    background: cs.bg,
                    color: cs.fg,
                    border: `1px solid ${cs.bd}`,
                    fontWeight: cs.fw,
                    fontSize: '10.5px',
                    lineHeight: 1.3,
                    fontFamily: "'IBM Plex Sans',system-ui,sans-serif",
                  }}
                >
                  <span
                    style={{
                      flex: 'none',
                      width: 4,
                      height: 4,
                      borderRadius: '50%',
                      background: cs.dot,
                    }}
                  />
                  <span
                    style={{
                      whiteSpace: 'nowrap',
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                    }}
                  >
                    {celeb.title}
                  </span>
                </div>
              )}

              {shown.map((e) => (
                <div
                  key={e.id}
                  style={{
                    padding: '3px 6px',
                    borderRadius: 4,
                    border: '1px dashed rgba(107,98,85,.42)',
                    color: 'var(--graphite)',
                    font: "400 10.5px/1.3 'IBM Plex Sans',system-ui,sans-serif",
                    whiteSpace: 'nowrap',
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                  }}
                >
                  {e.t} · {e.title}
                </div>
              ))}

              {extra > 0 && (
                <span
                  style={{
                    font: "500 10.5px/1.3 'IBM Plex Mono',monospace",
                    color: 'var(--graphite)',
                    paddingLeft: 2,
                  }}
                >
                  +{extra} más
                </span>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
