import type { ApiSeason } from '../../lib/api';
import { celebrationOf, dayLabel, seasonOf, type DayEvent } from '../../lib/calendar';

interface Props {
  sel: string;
  seasons: ApiSeason[];
  itemsFor: (key: string) => DayEvent[];
  onOpen: (id: string) => void;
}

export default function DayPanel({ sel, seasons, itemsFor, onOpen }: Props) {
  const season = seasonOf(sel, seasons);
  const items = itemsFor(sel);
  const celeb = celebrationOf(items);

  return (
    <div
      style={{
        flex: '1 1 288px',
        background: 'var(--panel)',
        borderRadius: 12,
        padding: 22,
        color: 'var(--panel-fg)',
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 5 }}>
        <span
          style={{
            width: 9,
            height: 9,
            borderRadius: '50%',
            background: celeb ? celeb.hex : season.color,
          }}
        />
        <span
          style={{
            font: "500 9.5px/1 'IBM Plex Mono',monospace",
            letterSpacing: '.14em',
            textTransform: 'uppercase',
            color: 'var(--dim)',
          }}
        >
          {season.name}
        </span>
      </div>

      <div
        style={{
          font: '400 23px/1.14 Marcellus,Georgia,serif',
          color: 'var(--panel-bright)',
          marginBottom: 4,
        }}
      >
        {dayLabel(sel)}
      </div>
      <div
        style={{
          font: "400 14px/1.5 'EB Garamond',Georgia,serif",
          color: 'var(--accent)',
          marginBottom: 18,
        }}
      >
        {celeb ? celeb.title : 'Feria · sin celebración propia'}
      </div>

      <div style={{ display: 'flex', flexDirection: 'column' }}>
        {items.map((e) => (
          <div
            key={e.id}
            onClick={() => onOpen(e.id)}
            style={{
              display: 'flex',
              gap: 12,
              alignItems: 'flex-start',
              padding: '10px 0',
              borderTop: '1px solid rgba(201,169,97,.16)',
              cursor: 'pointer',
              transition: 'opacity calc(.2s * var(--m))',
            }}
          >
            <span
              style={{
                flex: 'none',
                font: "500 11px/1.5 'IBM Plex Mono',monospace",
                color: 'var(--accent)',
                width: 40,
              }}
            >
              {e.t}
            </span>
            <div style={{ flex: 1, minWidth: 0 }}>
              <div
                style={{
                  font: "600 13px/1.35 'IBM Plex Sans',system-ui,sans-serif",
                  color: 'var(--panel-bright)',
                }}
              >
                {e.title}
              </div>
              <div
                style={{
                  marginTop: 3,
                  font: "500 9px/1 'IBM Plex Mono',monospace",
                  letterSpacing: '.12em',
                  textTransform: 'uppercase',
                  color: 'var(--dim)',
                }}
              >
                {e.groupName}
              </div>
            </div>
          </div>
        ))}
      </div>

      {items.length === 0 && (
        <p
          style={{
            margin: 0,
            font: "400 14.5px/1.55 'EB Garamond',Georgia,serif",
            color: 'var(--graphite)',
          }}
        >
          Sin actividades publicadas para este día.
        </p>
      )}
    </div>
  );
}
