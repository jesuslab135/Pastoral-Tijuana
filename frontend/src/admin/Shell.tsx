/** @jsxImportSource react */

// The frame every signed-in screen sits in: the dark sidebar from
// `project/Admin Eventos.dc.html` lines 31-55, or its sticky phone header
// (lines 59-71) below 900px.

import { useEffect, useMemo, useState } from 'react';
import type { CSSProperties, ReactNode } from 'react';
import { NavLink, Outlet, useOutletContext } from 'react-router-dom';

import { useBroadcastsQuery, useEventsQuery, useLogoutMutation } from './api';
import { monthWindow } from './dates';
import type { AdminUser, Role } from './types';
import { MONO, SANS, SERIF } from './ui';

const PHONE = 900;

const ROLE_LABEL: Record<Role, string> = {
  parroco: 'Párroco',
  secretaria: 'Secretaría',
};

/** The mock's #fffdf7 read as ink on the dark panel rather than as a surface. */
const BRIGHT = 'var(--card)';
const HAIRLINE = '1px solid rgba(201,169,97,.16)';
/** Avatar violet, a one-off in the mock with no token of its own. */
const AVATAR = '#5c3b7a';

/** The signed-in user, handed down by `RequireSession` through the outlets. */
export function useSession(): AdminUser {
  return useOutletContext<AdminUser>();
}

/** "P. Miguel Ramírez" → "MR": initials of the first two real names. */
function initials(name: string): string {
  const words = name.split(/\s+/).map((w) => w.replace(/\./g, ''));
  const real = words.filter((w) => w.length > 1);
  const pick = real.length > 0 ? real : words;
  return pick.slice(0, 2).map((w) => w.charAt(0).toUpperCase()).join('') || '?';
}

function useViewportWidth(): number {
  const [width, setWidth] = useState(() =>
    typeof window === 'undefined' ? 1280 : window.innerWidth,
  );

  useEffect(() => {
    let raf = 0;
    const onResize = () => {
      if (raf) return;
      raf = requestAnimationFrame(() => {
        raf = 0;
        setWidth(window.innerWidth);
      });
    };
    window.addEventListener('resize', onResize);
    return () => {
      window.removeEventListener('resize', onResize);
      if (raf) cancelAnimationFrame(raf);
    };
  }, []);

  return width;
}

/** The panel's one phone breakpoint, so a screen's layout flips with the frame's. */
export function useIsPhone(): boolean {
  return useViewportWidth() < PHONE;
}

function LogoDisc({ size }: { size: number }) {
  return (
    <div
      style={{
        width: size,
        height: size,
        borderRadius: '50%',
        border: '2px solid rgba(201,169,97,.6)',
        position: 'relative',
        flex: 'none',
      }}
    >
      <div
        style={{
          position: 'absolute',
          inset: 5,
          borderRadius: '50%',
          background: 'var(--sea-verde-color)',
        }}
      />
    </div>
  );
}

function navStyle(active: boolean, over: boolean): CSSProperties {
  return {
    padding: '11px 12px',
    borderRadius: 7,
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    gap: 8,
    font: `${active ? 600 : 500} 12.5px/1 ${SANS}`,
    color: active || over ? BRIGHT : 'var(--dim)',
    background: active
      ? 'rgba(201,169,97,.16)'
      : over
        ? 'rgba(201,169,97,.09)'
        : 'none',
    transition: 'all calc(.2s * var(--m))',
  };
}

function NavItem({
  to,
  children,
  badge,
}: {
  to: string;
  children: ReactNode;
  badge?: ReactNode;
}) {
  const [over, setOver] = useState(false);
  return (
    <NavLink
      to={to}
      onMouseEnter={() => setOver(true)}
      onMouseLeave={() => setOver(false)}
      style={({ isActive }) => navStyle(isActive, over)}
    >
      {children}
      {badge}
    </NavLink>
  );
}

function PublicSiteLink() {
  const [over, setOver] = useState(false);
  return (
    <a
      href="/"
      onMouseEnter={() => setOver(true)}
      onMouseLeave={() => setOver(false)}
      style={navStyle(false, over)}
    >
      Ver sitio público ↗
    </a>
  );
}

function EventCount({ count }: { count: number }) {
  return <span style={{ font: `500 9.5px/1 ${MONO}`, color: 'var(--accent)' }}>{count}</span>;
}

function FailureDot() {
  return (
    <span
      style={{
        width: 7,
        height: 7,
        borderRadius: '50%',
        background: 'var(--red)',
        animation: 'cpPulse calc(2.2s * var(--m)) ease-in-out infinite',
      }}
    />
  );
}

function phoneChipStyle(active: boolean): CSSProperties {
  return {
    flex: 'none',
    padding: '9px 13px',
    borderRadius: 18,
    minHeight: 36,
    font: `${active ? 600 : 500} 11.5px/1 ${SANS}`,
    // After `font`, which would otherwise reset it: 9 + 18 + 9 fills the chip.
    lineHeight: '18px',
    color: active ? BRIGHT : 'var(--dim)',
    background: active ? 'rgba(201,169,97,.18)' : 'none',
    // The mock leaves the active chip unbordered; a transparent one keeps the
    // row from shifting 2px as the selection moves.
    border: active ? '1px solid transparent' : '1px solid rgba(201,169,97,.2)',
  };
}

function PhoneChip({ to, children }: { to: string; children: ReactNode }) {
  return (
    <NavLink to={to} style={({ isActive }) => phoneChipStyle(isActive)}>
      {children}
    </NavLink>
  );
}

export default function Shell() {
  const me = useSession();
  const isPhone = useIsPhone();
  const [logout] = useLogoutMutation();

  const month = useMemo(() => monthWindow(new Date()), []);
  // Only the window, never the label: the arg doubles as the request's query
  // string and as the cache key the eventos screen shares.
  const { data: events } = useEventsQuery({ from: month.from, to: month.to });
  // Unfiltered so the same cache entry serves the Difusión screen; the badge
  // only needs to know whether anything is broken.
  const { data: broadcasts } = useBroadcastsQuery({});
  const broken = (broadcasts ?? []).some((b) => b.state === 'failed' || b.state === 'dead');

  const signOut = async () => {
    try {
      await logout().unwrap();
    } catch {
      // The cookie is gone either way once the browser reloads on the login page.
    }
    location.assign('/admin/login');
  };

  const sidebar = (
    <aside
      style={{
        flex: 'none',
        width: 236,
        background: 'var(--panel)',
        color: 'var(--panel-fg)',
        padding: '22px 16px',
        display: 'flex',
        flexDirection: 'column',
        position: 'sticky',
        top: 0,
        height: '100vh',
      }}
    >
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 10,
          padding: '0 6px 22px',
          borderBottom: HAIRLINE,
        }}
      >
        <LogoDisc size={28} />
        <div style={{ minWidth: 0 }}>
          <div
            style={{
              font: `400 13.5px/1.2 ${SERIF}`,
              color: BRIGHT,
              whiteSpace: 'nowrap',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
            }}
          >
            Cristo de Los Álamos
          </div>
          <div
            style={{
              font: `400 8.5px/1.3 ${MONO}`,
              letterSpacing: '.14em',
              textTransform: 'uppercase',
              color: 'var(--dim)',
            }}
          >
            Panel pastoral
          </div>
        </div>
      </div>

      <nav style={{ display: 'flex', flexDirection: 'column', gap: 3, paddingTop: 16 }}>
        <NavItem to="/eventos" badge={events ? <EventCount count={events.length} /> : null}>
          Eventos
        </NavItem>
        <NavItem to="/difusion" badge={broken ? <FailureDot /> : null}>
          Difusión
        </NavItem>
        {me.role === 'parroco' && <NavItem to="/equipo">Equipo y permisos</NavItem>}
        <PublicSiteLink />
      </nav>

      <div style={{ marginTop: 'auto', padding: '16px 6px 0', borderTop: HAIRLINE }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
          <div
            style={{
              width: 30,
              height: 30,
              borderRadius: '50%',
              background: AVATAR,
              flex: 'none',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              font: `600 11px/1 ${SANS}`,
              color: '#fff',
            }}
          >
            {initials(me.display_name)}
          </div>
          <div style={{ minWidth: 0, flex: 1 }}>
            <div
              style={{
                font: `600 12px/1.3 ${SANS}`,
                color: BRIGHT,
                whiteSpace: 'nowrap',
                overflow: 'hidden',
                textOverflow: 'ellipsis',
              }}
            >
              {me.display_name}
            </div>
            <div
              style={{
                font: `400 9px/1.3 ${MONO}`,
                letterSpacing: '.1em',
                textTransform: 'uppercase',
                color: 'var(--dim)',
              }}
            >
              {ROLE_LABEL[me.role]}
            </div>
          </div>
        </div>
        <button
          type="button"
          onClick={signOut}
          style={{
            display: 'block',
            marginTop: 12,
            padding: 0,
            border: 'none',
            background: 'none',
            cursor: 'pointer',
            font: `500 10.5px/1 ${MONO}`,
            letterSpacing: '.1em',
            color: 'var(--dim)',
          }}
        >
          CERRAR SESIÓN
        </button>
      </div>
    </aside>
  );

  const phoneHeader = (
    <div
      style={{
        background: 'var(--panel)',
        color: 'var(--panel-fg)',
        padding: '13px 16px',
        position: 'sticky',
        top: 0,
        zIndex: 30,
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 12 }}>
        <LogoDisc size={26} />
        <div style={{ font: `400 14px/1.2 ${SERIF}`, color: BRIGHT, flex: 1, minWidth: 0 }}>
          Cristo de Los Álamos
        </div>
        <button
          type="button"
          onClick={signOut}
          style={{
            padding: 0,
            border: 'none',
            background: 'none',
            cursor: 'pointer',
            font: `500 9.5px/1 ${MONO}`,
            letterSpacing: '.1em',
            color: 'var(--dim)',
          }}
        >
          SALIR
        </button>
      </div>
      <div style={{ display: 'flex', gap: 5, overflowX: 'auto' }}>
        <PhoneChip to="/eventos">Eventos</PhoneChip>
        <PhoneChip to="/difusion">Difusión</PhoneChip>
        {me.role === 'parroco' && <PhoneChip to="/equipo">Equipo</PhoneChip>}
        <a href="/" style={phoneChipStyle(false)}>
          Sitio ↗
        </a>
      </div>
    </div>
  );

  return (
    <div
      style={{
        minHeight: '100vh',
        display: 'flex',
        flexDirection: isPhone ? 'column' : 'row',
      }}
    >
      {isPhone ? phoneHeader : sidebar}
      <main
        style={{
          flex: 1,
          minWidth: 0,
          padding: isPhone ? '22px 16px 52px' : '32px 34px 60px',
        }}
      >
        <Outlet context={me} />
      </main>
    </div>
  );
}
