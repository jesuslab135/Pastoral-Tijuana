import { HORARIOS } from '../../lib/config';

/**
 * The everyday mass schedule. It is not in the calendar feed on purpose: the
 * ordinary hours never change and would bury what the parish actually
 * publishes.
 */
export default function Horarios() {
  return (
    <div
      id="horarios"
      style={{
        flex: '1 1 288px',
        background: 'var(--card)',
        border: '1px solid rgba(34,29,21,.14)',
        borderRadius: 12,
        padding: 22,
      }}
    >
      <div
        style={{
          font: "600 9.5px/1 'IBM Plex Mono',monospace",
          letterSpacing: '.18em',
          textTransform: 'uppercase',
          color: 'var(--gold)',
          marginBottom: 15,
        }}
      >
        Horarios ordinarios de misa
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: 11 }}>
        {HORARIOS.map((h, i) => (
          <div
            key={h.day}
            style={{
              display: 'flex',
              justifyContent: 'space-between',
              gap: 12,
              paddingBottom: i < HORARIOS.length - 1 ? 11 : 0,
              borderBottom:
                i < HORARIOS.length - 1 ? '1px solid rgba(34,29,21,.1)' : 'none',
            }}
          >
            <span style={{ font: "600 12.5px/1.3 'IBM Plex Sans',system-ui,sans-serif" }}>
              {h.day}
            </span>
            <span
              style={{
                font: "500 12px/1.3 'IBM Plex Mono',monospace",
                color: 'var(--muted)',
                textAlign: 'right',
                whiteSpace: 'pre-line',
              }}
            >
              {h.times}
              {h.note && (
                <>
                  {'\n'}
                  <span style={{ color: 'var(--graphite)', fontSize: '10.5px' }}>{h.note}</span>
                </>
              )}
            </span>
          </div>
        ))}
      </div>

      <p
        style={{
          margin: '16px 0 0',
          font: "400 13.5px/1.5 'EB Garamond',Georgia,serif",
          color: 'var(--graphite)',
        }}
      >
        Las solemnidades y fiestas pueden cambiar el horario. El calendario siempre manda sobre
        esta tabla.
      </p>
    </div>
  );
}
