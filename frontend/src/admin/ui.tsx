/** @jsxImportSource react */

// The primitives the four admin mockups repeat verbatim dozens of times. Every
// value here is lifted from `project/Admin Eventos.dc.html` (and the login
// mock's error panel), so screens never re-type a padding or a font stack.
// All of them take a `style` override for the one-off tweaks the mocks make.

import { useState } from 'react';
import type {
  ButtonHTMLAttributes,
  CSSProperties,
  InputHTMLAttributes,
  ReactNode,
} from 'react';

export const MONO = "'IBM Plex Mono',monospace";
export const SANS = "'IBM Plex Sans',system-ui,sans-serif";
export const SERIF = 'Marcellus,Georgia,serif';
export const BODY = "'EB Garamond',Georgia,serif";

/** The green the mocks use for focus rings and the publish action. */
export const GREEN = 'var(--sea-verde-color)';

interface Boxed {
  children: ReactNode;
  style?: CSSProperties;
}

export function Kicker({ children, style }: Boxed) {
  return (
    <div
      style={{
        font: `600 9.5px/1 ${MONO}`,
        letterSpacing: '.2em',
        textTransform: 'uppercase',
        color: 'var(--gold)',
        ...style,
      }}
    >
      {children}
    </div>
  );
}

export function H1({ children, style }: Boxed) {
  return (
    <h1
      style={{
        margin: 0,
        font: `400 clamp(28px,3.6vw,40px)/1.06 ${SERIF}`,
        letterSpacing: '-.01em',
        ...style,
      }}
    >
      {children}
    </h1>
  );
}

export function Field({
  label,
  children,
  style,
}: {
  label: ReactNode;
  children: ReactNode;
  style?: CSSProperties;
}) {
  return (
    <div style={style}>
      <label
        style={{
          display: 'block',
          font: `600 9.5px/1 ${MONO}`,
          letterSpacing: '.14em',
          textTransform: 'uppercase',
          color: 'var(--graphite)',
          marginBottom: 8,
        }}
      >
        {label}
      </label>
      {children}
    </div>
  );
}

export function TextInput({
  style,
  onFocus,
  onBlur,
  ...rest
}: InputHTMLAttributes<HTMLInputElement>) {
  const [focused, setFocused] = useState(false);
  return (
    <input
      {...rest}
      onFocus={(e) => {
        setFocused(true);
        onFocus?.(e);
      }}
      onBlur={(e) => {
        setFocused(false);
        onBlur?.(e);
      }}
      style={{
        width: '100%',
        padding: 14,
        border: '1px solid rgba(34,29,21,.22)',
        borderRadius: 8,
        background: 'var(--card)',
        font: `500 15px/1.3 ${SANS}`,
        color: 'var(--ink)',
        outline: 'none',
        transition: 'border-color calc(.2s * var(--m)),box-shadow calc(.2s * var(--m))',
        ...style,
        // Last so the ring survives a caller's own border colour.
        ...(focused
          ? { borderColor: GREEN, boxShadow: '0 0 0 3px rgba(47,107,79,.16)' }
          : null),
      }}
    />
  );
}

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement>;

function HoverButton({
  base,
  hover,
  style,
  onMouseEnter,
  onMouseLeave,
  ...rest
}: ButtonProps & { base: CSSProperties; hover: CSSProperties }) {
  const [over, setOver] = useState(false);
  return (
    <button
      type="button"
      {...rest}
      onMouseEnter={(e) => {
        setOver(true);
        onMouseEnter?.(e);
      }}
      onMouseLeave={(e) => {
        setOver(false);
        onMouseLeave?.(e);
      }}
      style={{ ...base, ...style, ...(over && !rest.disabled ? hover : null) }}
    />
  );
}

export function PrimaryButton(props: ButtonProps) {
  return (
    <HoverButton
      base={{
        padding: '14px 18px',
        borderRadius: 8,
        border: 'none',
        background: 'var(--panel)',
        color: 'var(--panel-bright)',
        font: `600 12.5px/1 ${SANS}`,
        minHeight: 48,
        cursor: 'pointer',
        transition: 'transform calc(.2s * var(--m)),box-shadow calc(.2s * var(--m))',
      }}
      // The lift's shadow reads through a custom property so a recoloured
      // button (the mock's green "Publicar y difundir") can match its own
      // background without a second component.
      hover={{
        transform: 'translateY(-2px)',
        boxShadow: '0 10px 22px var(--btn-shadow,rgba(34,29,21,.2))',
      }}
      {...props}
    />
  );
}

export function DangerButton(props: ButtonProps) {
  return (
    <HoverButton
      base={{
        padding: '7px 10px',
        borderRadius: 6,
        border: '1px solid rgba(160,47,39,.24)',
        background: 'var(--card)',
        color: 'var(--red)',
        font: `500 10.5px/1 ${SANS}`,
        minHeight: 32,
        cursor: 'pointer',
        transition: 'all calc(.2s * var(--m))',
      }}
      hover={{ background: 'rgba(160,47,39,.08)' }}
      {...props}
    />
  );
}

export function GhostButton(props: ButtonProps) {
  return (
    <HoverButton
      base={{
        padding: '7px 10px',
        borderRadius: 6,
        border: '1px solid rgba(34,29,21,.16)',
        background: 'var(--card)',
        color: 'var(--muted)',
        font: `500 10.5px/1 ${SANS}`,
        minHeight: 32,
        cursor: 'pointer',
        transition: 'all calc(.2s * var(--m))',
      }}
      hover={{ background: 'var(--footer)', color: 'var(--ink)' }}
      {...props}
    />
  );
}

export function Chip({
  on,
  children,
  onClick,
  style,
}: {
  on: boolean;
  children: ReactNode;
  onClick?: () => void;
  style?: CSSProperties;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      style={{
        padding: '9px 14px',
        borderRadius: 20,
        cursor: 'pointer',
        font: `500 11.5px/1 ${SANS}`,
        minHeight: 38,
        transition: 'all calc(.2s * var(--m))',
        background: on ? 'var(--panel)' : 'var(--card)',
        color: on ? 'var(--panel-bright)' : 'var(--graphite)',
        border: `1px solid ${on ? 'var(--panel)' : 'rgba(34,29,21,.18)'}`,
        ...style,
      }}
    >
      {children}
    </button>
  );
}

export function StatCard({
  value,
  color,
  children,
  style,
}: {
  value: ReactNode;
  color: string;
  children: ReactNode;
  style?: CSSProperties;
}) {
  return (
    <div
      style={{
        background: 'var(--card)',
        border: '1px solid rgba(34,29,21,.14)',
        borderRadius: 10,
        padding: '16px 17px',
        ...style,
      }}
    >
      <div style={{ font: `400 30px/1 ${SERIF}`, color }}>{value}</div>
      <div
        style={{
          marginTop: 6,
          font: `500 10.5px/1.35 ${SANS}`,
          color: 'var(--graphite)',
        }}
      >
        {children}
      </div>
    </div>
  );
}

export function ErrorBox({ children, style }: Boxed) {
  return (
    <div
      style={{
        padding: '12px 14px',
        borderRadius: 8,
        background: 'rgba(160,47,39,.08)',
        border: '1px solid rgba(160,47,39,.3)',
        font: `400 14px/1.45 ${BODY}`,
        // Darker than --red: the mock deepens the text over its own tint.
        color: '#8f2a23',
        ...style,
      }}
    >
      {children}
    </div>
  );
}

export function Tag({
  bg,
  fg,
  children,
  style,
}: {
  bg: string;
  fg: string;
  children: ReactNode;
  style?: CSSProperties;
}) {
  return (
    <span
      style={{
        flex: 'none',
        font: `600 8.5px/1 ${MONO}`,
        letterSpacing: '.1em',
        padding: '4px 7px',
        borderRadius: 4,
        background: bg,
        color: fg,
        ...style,
      }}
    >
      {children}
    </span>
  );
}
