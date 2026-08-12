type Swatch = { background: string; border?: string };

/** Teaches the visual language once, so the grid needs no captions. */
const FILAS: Array<{ swatch: Swatch; strong: string; text: string }> = [
  {
    swatch: { background: 'rgba(92,59,122,.11)', border: '1px solid #5c3b7a' },
    strong: 'Borde continuo:',
    text: 'liturgia. Toma el color del tiempo — o el del mártir, si el día lo pide.',
  },
  {
    swatch: { background: 'rgba(107,98,85,.07)', border: '1px dashed rgba(107,98,85,.5)' },
    strong: 'Borde discontinuo:',
    text: 'actividad de un grupo parroquial. Fuera de la paleta litúrgica, a propósito.',
  },
  {
    swatch: { background: '#5c3b7a' },
    strong: 'Franja superior rellena:',
    text: 'la celebración del día. Solemnidad en pleno, fiesta en tinte.',
  },
];

export default function Leyenda() {
  return (
    <div
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
        Cómo leer la semana
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: 13 }}>
        {FILAS.map((f) => (
          <div key={f.strong} style={{ display: 'flex', gap: 11, alignItems: 'flex-start' }}>
            <span
              style={{
                flex: 'none',
                width: 34,
                height: 22,
                borderRadius: 5,
                marginTop: 2,
                ...f.swatch,
              }}
            />
            <p
              style={{
                margin: 0,
                font: "400 14px/1.5 'EB Garamond',Georgia,serif",
                color: 'var(--muted)',
              }}
            >
              <strong style={{ fontWeight: 600 }}>{f.strong}</strong> {f.text}
            </p>
          </div>
        ))}

        <div style={{ display: 'flex', gap: 11, alignItems: 'flex-start' }}>
          <span
            style={{
              flex: 'none',
              width: 34,
              height: 22,
              borderRadius: 5,
              position: 'relative',
              marginTop: 2,
            }}
          >
            <span
              style={{
                position: 'absolute',
                left: 0,
                right: 0,
                top: 10,
                height: 1.5,
                background: 'var(--red)',
              }}
            />
            <span
              style={{
                position: 'absolute',
                left: -2,
                top: 7,
                width: 8,
                height: 8,
                borderRadius: '50%',
                background: 'var(--red)',
              }}
            />
          </span>
          <p
            style={{
              margin: 0,
              font: "400 14px/1.5 'EB Garamond',Georgia,serif",
              color: 'var(--muted)',
            }}
          >
            <strong style={{ fontWeight: 600 }}>Línea roja:</strong> la hora actual. Solo aparece
            en la semana en curso.
          </p>
        </div>
      </div>
    </div>
  );
}
