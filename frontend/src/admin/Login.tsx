/** @jsxImportSource react */

// The one screen outside the session guard: the split hero from
// `project/Admin Login.dc.html`. The mock's two decorative status pills
// (lines 46-49, "Tiempo Ordinario · verde" and "4 grupos de difusión
// activos") are dropped — they're hardcoded mock data, and fetching real
// values isn't worth a request on a logged-out screen. The prose above them
// stays as written.

import { useEffect, useState } from 'react';
import type { ButtonHTMLAttributes } from 'react';
import { Navigate, useNavigate } from 'react-router-dom';

import { useLoginMutation, useMagicLinkMutation, useMeQuery } from './api';
import { apiError } from './types';
import { BODY, ErrorBox, Field, H1, Kicker, MONO, PrimaryButton, SANS, SERIF, TextInput } from './ui';
import { MESES, parseKey, todayKey } from '../lib/calendar';

const PHONE = 860;

/** The mock's #fffdf7 read as ink on the dark panel rather than as a surface. */
const BRIGHT = 'var(--card)';

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

function LogoDisc({ size, ring, dot }: { size: number; ring: string; dot: string }) {
  return (
    <div
      style={{
        width: size,
        height: size,
        borderRadius: '50%',
        border: `2px solid ${ring}`,
        position: 'relative',
        flex: 'none',
      }}
    >
      <div style={{ position: 'absolute', inset: 6, borderRadius: '50%', background: dot }} />
    </div>
  );
}

// No ui.tsx primitive matches this shape (full-width, light, no lift-on-hover)
// closely enough to reuse, so it stays local to the one screen that needs it.
function MagicLinkButton({ style, disabled, ...rest }: ButtonHTMLAttributes<HTMLButtonElement>) {
  const [over, setOver] = useState(false);
  return (
    <button
      type="button"
      disabled={disabled}
      {...rest}
      onMouseEnter={() => setOver(true)}
      onMouseLeave={() => setOver(false)}
      style={{
        width: '100%',
        padding: 14,
        borderRadius: 8,
        border: '1px solid rgba(34,29,21,.22)',
        background: 'var(--card)',
        // One-off ink shade the mock uses only for this button; no shared token.
        color: '#4a4337',
        font: `600 12.5px/1 ${SANS}`,
        minHeight: 48,
        cursor: disabled ? 'default' : 'pointer',
        opacity: disabled ? 0.6 : 1,
        transition: 'all calc(.2s * var(--m))',
        ...(over && !disabled ? { background: 'var(--footer)', borderColor: 'rgba(34,29,21,.3)' } : null),
        ...style,
      }}
    />
  );
}

export default function Login() {
  const navigate = useNavigate();
  const { data: me, isLoading: meLoading } = useMeQuery();
  const [login, { isLoading: loggingIn }] = useLoginMutation();
  const [magicLink, { isLoading: sendingMagic }] = useMagicLinkMutation();

  const isPhone = useViewportWidth() < PHONE;

  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [show, setShow] = useState(false);
  const [error, setError] = useState('');
  const [magicSent, setMagicSent] = useState(false);

  // A live session has no business seeing the form.
  if (meLoading) return null;
  if (me) return <Navigate to="/eventos" replace />;

  const today = parseKey(todayKey());
  const todayLabel = `${today.getDate()} de ${MESES[today.getMonth()]}`;

  async function submitLogin() {
    if (!email && !password) {
      setError('Escribe tu correo parroquial y tu contraseña para continuar.');
      return;
    }
    try {
      await login({ email, password }).unwrap();
      navigate('/eventos');
    } catch (err) {
      setError(apiError(err));
    }
  }

  async function handleMagicLink() {
    if (!email) {
      setError('Escribe tu correo para mandarte el enlace.');
      return;
    }
    try {
      await magicLink({ email }).unwrap();
      setMagicSent(true);
      setError('');
    } catch (err) {
      setError(apiError(err));
    }
  }

  return (
    <div style={{ minHeight: '100vh', display: 'flex', flexWrap: 'wrap' }}>
      {!isPhone && (
        <div
          style={{
            flex: '1 1 46%',
            minWidth: 340,
            position: 'relative',
            overflow: 'hidden',
            background: 'linear-gradient(165deg,var(--sea-verde-color) 0%,var(--sea-verde-deep) 58%,#1e4433 100%)',
            padding: '56px 48px',
            display: 'flex',
            flexDirection: 'column',
            justifyContent: 'space-between',
          }}
        >
          {/* Decorative orb: static rather than `cpFloat`, which isn't a token yet. */}
          <div
            style={{
              position: 'absolute',
              top: -140,
              right: -120,
              width: 460,
              height: 460,
              borderRadius: '50%',
              background: 'radial-gradient(circle,rgba(255,253,247,.13),transparent 68%)',
            }}
          />
          <div
            style={{
              position: 'absolute',
              inset: 0,
              pointerEvents: 'none',
              background: 'linear-gradient(100deg,transparent 40%,rgba(255,255,255,.1) 50%,transparent 60%)',
              backgroundSize: '220% 100%',
              animation: 'cpSheen calc(11s * var(--m)) linear infinite',
            }}
          />

          <div style={{ position: 'relative', display: 'flex', alignItems: 'center', gap: 12 }}>
            <LogoDisc size={32} ring="rgba(255,253,247,.55)" dot="var(--accent)" />
            <div>
              <div style={{ font: `400 16px/1.15 ${SERIF}`, color: BRIGHT }}>Cristo de Los Álamos</div>
              <div
                style={{
                  font: `400 9.5px/1.3 ${MONO}`,
                  letterSpacing: '.14em',
                  textTransform: 'uppercase',
                  color: 'rgba(255,253,247,.6)',
                }}
              >
                Panel pastoral
              </div>
            </div>
          </div>

          <div style={{ position: 'relative' }}>
            <Kicker style={{ font: `600 10px/1 ${MONO}`, letterSpacing: '.22em', color: 'rgba(255,253,247,.55)', marginBottom: 18 }}>
              Hoy · {todayLabel}
            </Kicker>
            <div
              style={{
                font: `400 clamp(32px,3.8vw,46px)/1.06 ${SERIF}`,
                color: BRIGHT,
                maxWidth: '20ch',
              }}
            >
              Publicar es avisar a toda la parroquia
            </div>
            <p style={{ margin: '20px 0 0', maxWidth: '44ch', font: `400 17px/1.6 ${BODY}`, color: 'rgba(255,253,247,.75)' }}>
              Cada evento que guardes aquí aparece en el calendario público y sale a los grupos de
              WhatsApp de su grupo parroquial. No hay paso intermedio.
            </p>
          </div>
        </div>
      )}

      <div
        style={{
          flex: '1 1 420px',
          minWidth: 0,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          padding: isPhone ? '34px 20px 44px' : '56px 48px',
        }}
      >
        <div style={{ width: '100%', maxWidth: 400, animation: 'cpRise calc(.5s * var(--m)) cubic-bezier(.16,.84,.24,1) both' }}>
          {isPhone && (
            <div style={{ display: 'flex', alignItems: 'center', gap: 11, marginBottom: 30 }}>
              <LogoDisc size={30} ring="var(--gold-ring)" dot="var(--sea-verde-color)" />
              <div>
                <div style={{ font: `400 15px/1.15 ${SERIF}` }}>Cristo de Los Álamos</div>
                <div
                  style={{
                    font: `400 9.5px/1.3 ${MONO}`,
                    letterSpacing: '.14em',
                    textTransform: 'uppercase',
                    color: 'var(--dim)',
                  }}
                >
                  Panel pastoral
                </div>
              </div>
            </div>
          )}

          <Kicker style={{ marginBottom: 14 }}>Acceso del equipo</Kicker>
          <H1 style={{ marginBottom: 10 }}>Entrar al panel</H1>
          <p style={{ margin: '0 0 28px', font: `400 16px/1.55 ${BODY}`, color: 'var(--muted)' }}>
            Solo para el párroco y la secretaría parroquial. La comunidad no necesita cuenta para
            ver el calendario.
          </p>

          <form
            onSubmit={(e) => {
              e.preventDefault();
              void submitLogin();
            }}
            style={{ display: 'flex', flexDirection: 'column', gap: 16 }}
          >
            <Field label="Correo parroquial">
              <TextInput
                type="email"
                value={email}
                onChange={(e) => {
                  setEmail(e.target.value);
                  setError('');
                }}
                placeholder="secretaria@cristodelosalamos.mx"
              />
            </Field>

            <div>
              <div style={{ display: 'flex', alignItems: 'baseline', justifyContent: 'space-between', gap: 10, marginBottom: 8 }}>
                <label
                  style={{
                    font: `600 9.5px/1 ${MONO}`,
                    letterSpacing: '.14em',
                    textTransform: 'uppercase',
                    color: 'var(--graphite)',
                  }}
                >
                  Contraseña
                </label>
                <button
                  type="button"
                  onClick={() => setShow((s) => !s)}
                  style={{
                    border: 'none',
                    background: 'none',
                    padding: 0,
                    cursor: 'pointer',
                    font: `500 10px/1 ${MONO}`,
                    letterSpacing: '.08em',
                    color: 'var(--gold)',
                  }}
                >
                  {show ? 'OCULTAR' : 'MOSTRAR'}
                </button>
              </div>
              <TextInput
                type={show ? 'text' : 'password'}
                value={password}
                onChange={(e) => {
                  setPassword(e.target.value);
                  setError('');
                }}
                placeholder="••••••••"
              />
            </div>

            {error && <ErrorBox>{error}</ErrorBox>}

            <PrimaryButton type="submit" disabled={loggingIn}>
              Entrar
            </PrimaryButton>

            <div style={{ display: 'flex', alignItems: 'center', gap: 12, margin: '2px 0' }}>
              <span style={{ flex: 1, height: 1, background: 'var(--line)' }} />
              <span style={{ font: `500 9.5px/1 ${MONO}`, letterSpacing: '.12em', color: 'var(--graphite)' }}>O BIEN</span>
              <span style={{ flex: 1, height: 1, background: 'var(--line)' }} />
            </div>

            <MagicLinkButton onClick={handleMagicLink} disabled={sendingMagic || magicSent}>
              {magicSent ? 'Enlace enviado a tu correo' : 'Enviarme un enlace de acceso'}
            </MagicLinkButton>
          </form>

          <p style={{ margin: '26px 0 0', paddingTop: 20, borderTop: '1px solid var(--line)', font: `400 14px/1.55 ${BODY}`, color: 'var(--graphite)' }}>
            ¿Perdiste el acceso? El párroco puede restablecerlo desde Equipo y permisos.
          </p>
          <p style={{ margin: '10px 0 0', font: `400 14px/1.55 ${BODY}`, color: 'var(--graphite)' }}>
            <a href="/">← Volver al calendario público</a>
          </p>
        </div>
      </div>
    </div>
  );
}
