import type { ApiGroup } from '../../lib/api';

interface Props {
  view: 'mes' | 'semana';
  rangeLabel: string;
  groups: ApiGroup[];
  filter: string[];
  onView: (v: 'mes' | 'semana') => void;
  onPrev: () => void;
  onNext: () => void;
  onToday: () => void;
  onToggle: (slug: string) => void;
}

const navBtn = {
  height: 36,
  borderRadius: 8,
  border: '1px solid rgba(34,29,21,.18)',
  background: 'var(--card)',
  cursor: 'pointer',
  color: 'var(--muted)',
  transition: 'all calc(.2s * var(--m))',
};

export default function Toolbar({
  view,
  rangeLabel,
  groups,
  filter,
  onView,
  onPrev,
  onNext,
  onToday,
  onToggle,
}: Props) {
  const tab = (v: 'mes' | 'semana', label: string, first: boolean) => {
    const on = view === v;
    return (
      <button
        type="button"
        onClick={() => onView(v)}
        style={{
          font: "600 12px/1 'IBM Plex Sans',system-ui,sans-serif",
          padding: '11px 18px',
          border: 'none',
          borderLeft: first ? 'none' : '1px solid rgba(34,29,21,.14)',
          cursor: 'pointer',
          transition: 'all calc(.22s * var(--m))',
          background: on ? 'var(--panel)' : 'var(--card)',
          color: on ? 'var(--panel-bright)' : 'var(--graphite)',
        }}
      >
        {label}
      </button>
    );
  };

  return (
    <div
      style={{
        display: 'flex',
        gap: 14,
        flexWrap: 'wrap',
        alignItems: 'center',
        marginBottom: 18,
      }}
    >
      <div
        style={{
          display: 'flex',
          border: '1px solid rgba(34,29,21,.18)',
          borderRadius: 8,
          overflow: 'hidden',
          background: 'var(--card)',
          flex: 'none',
        }}
      >
        {tab('mes', 'Mes', true)}
        {tab('semana', 'Semana', false)}
      </div>

      <div style={{ display: 'flex', gap: 7, alignItems: 'center', flex: 'none' }}>
        <button
          type="button"
          onClick={onPrev}
          aria-label="Anterior"
          style={{ ...navBtn, width: 36, font: "500 15px/1 'IBM Plex Sans',sans-serif" }}
        >
          ‹
        </button>
        <button
          type="button"
          onClick={onToday}
          style={{ ...navBtn, padding: '0 15px', font: "600 11.5px/1 'IBM Plex Sans',sans-serif" }}
        >
          Hoy
        </button>
        <button
          type="button"
          onClick={onNext}
          aria-label="Siguiente"
          style={{ ...navBtn, width: 36, font: "500 15px/1 'IBM Plex Sans',sans-serif" }}
        >
          ›
        </button>
      </div>

      <div
        style={{
          font: '400 clamp(20px,2.3vw,27px)/1.1 Marcellus,Georgia,serif',
          flex: 'none',
          minWidth: 200,
        }}
      >
        {rangeLabel}
      </div>

      <div
        style={{
          display: 'flex',
          gap: 6,
          flexWrap: 'wrap',
          marginLeft: 'auto',
          alignItems: 'center',
        }}
      >
        <span
          style={{
            font: "600 10px/1 'IBM Plex Mono',monospace",
            letterSpacing: '.14em',
            textTransform: 'uppercase',
            color: 'var(--graphite)',
            marginRight: 3,
          }}
        >
          Filtrar
        </span>
        {groups.map((g) => {
          const on = filter.includes(g.slug);
          return (
            <button
              key={g.id}
              type="button"
              aria-pressed={on}
              onClick={() => onToggle(g.slug)}
              style={{
                font: "500 11px/1 'IBM Plex Sans',system-ui,sans-serif",
                padding: '8px 12px',
                borderRadius: 18,
                cursor: 'pointer',
                transition: 'all calc(.2s * var(--m))',
                background: on ? 'var(--panel)' : 'var(--card)',
                color: on ? 'var(--panel-bright)' : 'var(--graphite)',
                border: `1px solid ${on ? 'var(--panel)' : 'rgba(34,29,21,.18)'}`,
              }}
            >
              {g.name}
            </button>
          );
        })}
      </div>
    </div>
  );
}
