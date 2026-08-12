import type { ComponentChildren } from 'preact';

import { dayLabel, type DayEvent } from '../../lib/calendar';

interface Props {
  event: DayEvent;
  onClose: () => void;
}

function Row({ label, children }: { label: string; children: ComponentChildren }) {
  return (
    <div style={{ display: 'flex', gap: 12, alignItems: 'baseline' }}>
      <span
        style={{
          flex: 'none',
          width: 56,
          font: "500 9px/1.6 'IBM Plex Mono',monospace",
          letterSpacing: '.12em',
          color: 'var(--graphite)',
        }}
      >
        {label}
      </span>
      <span style={{ flex: 1, minWidth: 0 }}>{children}</span>
    </div>
  );
}

const prose = {
  font: "400 14.5px/1.45 'EB Garamond',Georgia,serif",
  color: '#d8d1c1',
};

export default function EventSheet({ event, onClose }: Props) {
  return (
    <div
      style={{
        flex: '1 1 288px',
        background: 'var(--panel)',
        borderRadius: 12,
        padding: 22,
        color: 'var(--panel-fg)',
        animation: 'cpIn calc(.36s * var(--m)) cubic-bezier(.16,.84,.24,1) both',
      }}
    >
      <div
        style={{
          display: 'flex',
          alignItems: 'flex-start',
          justifyContent: 'space-between',
          gap: 12,
          marginBottom: 16,
        }}
      >
        <span
          style={{
            font: "500 9.5px/1 'IBM Plex Mono',monospace",
            letterSpacing: '.16em',
            textTransform: 'uppercase',
            color: 'var(--accent)',
          }}
        >
          {event.groupName}
        </span>
        <button
          type="button"
          onClick={onClose}
          aria-label="Cerrar"
          style={{
            flex: 'none',
            width: 24,
            height: 24,
            borderRadius: 6,
            border: '1px solid rgba(201,169,97,.3)',
            background: 'none',
            color: 'var(--dim)',
            cursor: 'pointer',
            font: "500 12px/1 'IBM Plex Sans',sans-serif",
            transition: 'all calc(.2s * var(--m))',
          }}
        >
          ✕
        </button>
      </div>

      <div
        style={{
          font: '400 25px/1.12 Marcellus,Georgia,serif',
          color: 'var(--panel-bright)',
          marginBottom: 14,
        }}
      >
        {event.title}
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
        <Row label="DÍA">
          <span style={prose}>{dayLabel(event.dateKey)}</span>
        </Row>
        <Row label="HORA">
          <span style={{ font: "500 14px/1.4 'IBM Plex Mono',monospace", color: 'var(--accent)' }}>
            {event.t}–{event.tEnd}
          </span>
        </Row>
        <Row label="LUGAR">
          <span style={prose}>{event.place || 'Templo parroquial'}</span>
        </Row>
      </div>

      {event.description && (
        <p
          style={{
            margin: '16px 0 0',
            paddingTop: 15,
            borderTop: '1px solid rgba(201,169,97,.16)',
            font: "400 14.5px/1.55 'EB Garamond',Georgia,serif",
            color: 'var(--dim)',
          }}
        >
          {event.description}
        </p>
      )}
    </div>
  );
}
