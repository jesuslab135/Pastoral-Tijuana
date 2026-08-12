import { MESES, blurbFor, parseKey, type ResolvedSeason } from '../../lib/calendar';

interface Props {
  season: ResolvedSeason;
  todayK: string;
  isPhone: boolean;
}

export default function Banner({ season, todayK, isPhone }: Props) {
  const today = parseKey(todayK);
  const todayLabel = `${today.getDate()} ${MESES[today.getMonth()]!.slice(0, 3)}`;

  return (
    <div
      style={{
        position: 'relative',
        overflow: 'hidden',
        background: season.deep,
        transition: 'background calc(.6s * var(--m)) ease',
      }}
    >
      <div
        style={{
          position: 'absolute',
          inset: 0,
          pointerEvents: 'none',
          background:
            'linear-gradient(100deg,transparent 38%,rgba(255,255,255,.18) 50%,transparent 62%)',
          backgroundSize: '220% 100%',
          animation: 'cpSheen calc(9s * var(--m)) linear infinite',
        }}
      />
      <div
        style={{
          position: 'relative',
          maxWidth: 1360,
          margin: '0 auto',
          display: 'flex',
          gap: 28,
          flexWrap: 'wrap',
          alignItems: 'flex-end',
          padding: isPhone ? '26px 16px 24px' : '34px 28px 30px',
        }}
      >
        <div style={{ flex: 1, minWidth: 280 }}>
          <div
            style={{
              font: "600 10px/1 'IBM Plex Mono',monospace",
              letterSpacing: '.22em',
              textTransform: 'uppercase',
              color: 'rgba(255,255,255,.66)',
              marginBottom: 13,
            }}
          >
            Tiempo litúrgico en curso
          </div>
          <div
            style={{
              font: '400 clamp(34px,4.6vw,54px)/1.02 Marcellus,Georgia,serif',
              color: '#fffdf7',
              letterSpacing: '-.01em',
            }}
          >
            {season.name}
          </div>
          <div
            style={{
              marginTop: 11,
              font: "400 17px/1.5 'EB Garamond',Georgia,serif",
              color: 'rgba(255,255,255,.8)',
              maxWidth: '52ch',
            }}
          >
            {blurbFor(season.name)}
          </div>
        </div>

        <div style={{ flex: 'none', display: 'flex', gap: 9, alignItems: 'flex-end' }}>
          <div
            style={{
              padding: '14px 17px',
              borderRadius: 9,
              background: 'rgba(0,0,0,.2)',
              border: '1px solid rgba(255,255,255,.18)',
            }}
          >
            <div
              style={{
                font: "600 9px/1 'IBM Plex Mono',monospace",
                letterSpacing: '.16em',
                color: 'rgba(255,255,255,.6)',
                marginBottom: 8,
              }}
            >
              HOY
            </div>
            <div style={{ font: '400 26px/1 Marcellus,Georgia,serif', color: '#fffdf7' }}>
              {todayLabel}
            </div>
          </div>

          <div
            style={{
              padding: '14px 17px',
              borderRadius: 9,
              background: 'rgba(0,0,0,.2)',
              border: '1px solid rgba(255,255,255,.18)',
            }}
          >
            <div
              style={{
                font: "600 9px/1 'IBM Plex Mono',monospace",
                letterSpacing: '.16em',
                color: 'rgba(255,255,255,.6)',
                marginBottom: 8,
              }}
            >
              COLOR
            </div>
            <div
              style={{
                font: "400 15px/1.6 'IBM Plex Sans',system-ui,sans-serif",
                color: '#fffdf7',
                display: 'flex',
                alignItems: 'center',
                gap: 8,
              }}
            >
              <span
                style={{
                  flex: 'none',
                  width: 22,
                  height: 22,
                  borderRadius: 5,
                  background: '#fffdf7',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  boxShadow: '0 1px 3px rgba(0,0,0,.25)',
                }}
              >
                <span
                  style={{ width: 12, height: 12, borderRadius: 3, background: season.color }}
                />
              </span>
              {season.cname}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
