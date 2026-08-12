/** @jsxImportSource react */

// Equipo y permisos: `project/Admin Equipo.dc.html` ported to the system's two
// real roles. The mock's third role, "Coordinador de grupo", does not exist in
// the data model — it and its explainer are dropped. There is also no invite
// email: `POST /admin/users` creates the account directly and the person signs
// in later with a magic link, so the mock's "pending invitation" state and its
// activity-log card (there is no audit-log API behind it) go with it too.

import { useState } from 'react';
import type { CSSProperties } from 'react';

import {
  useCreateUserMutation,
  useSetUserActiveMutation,
  useUpdateUserMutation,
  useUsersQuery,
} from './api';
import { useSession } from './Shell';
import { apiError } from './types';
import type { AdminUser, Role, UserInput } from './types';
import {
  BODY,
  Chip,
  DangerButton,
  ErrorBox,
  Field,
  GREEN,
  GhostButton,
  H1,
  Kicker,
  MONO,
  PrimaryButton,
  SANS,
  Tag,
  TextInput,
} from './ui';

const ROLES: Role[] = ['parroco', 'secretaria'];
const ROLE_LABEL: Record<Role, string> = { parroco: 'Párroco', secretaria: 'Secretaría' };
// Literal hex, matching --sea-violeta-color / --sea-verde-color exactly: tag
// and chip backgrounds below append an alpha suffix, which a var() reference
// can't do.
const ROLE_HEX: Record<Role, string> = { parroco: '#5c3b7a', secretaria: '#2f6b4f' };

const CAP_LABELS = ['Crear y editar', 'Publicar y difundir', 'Ver difusión', 'Administrar equipo'];
// Index-aligned with CAP_LABELS; only párroco carries "Administrar equipo".
const ROLE_CAPS: Record<Role, boolean[]> = {
  parroco: [true, true, true, true],
  secretaria: [true, true, true, false],
};

const CARD: CSSProperties = {
  background: 'var(--card)',
  border: '1px solid rgba(34,29,21,.14)',
  borderRadius: 11,
  padding: 17,
};

const SIDE_HEADER: CSSProperties = {
  font: `600 9.5px/1 ${MONO}`,
  letterSpacing: '.18em',
  textTransform: 'uppercase',
  marginBottom: 15,
};

/** "P. Miguel Ramírez" -> "MR": initials of the first two real name words. */
function initials(name: string): string {
  const words = name.split(/\s+/).map((w) => w.replace(/\./g, ''));
  const real = words.filter((w) => w.length > 1);
  const pick = real.length > 0 ? real : words;
  return pick.slice(0, 2).map((w) => w.charAt(0).toUpperCase()).join('') || '?';
}

function RoleChips({ value, onPick }: { value: Role; onPick: (r: Role) => void }) {
  return (
    <div style={{ display: 'flex', gap: 7, flexWrap: 'wrap' }}>
      {ROLES.map((r) => {
        const on = value === r;
        return (
          <Chip
            key={r}
            on={on}
            onClick={() => onPick(r)}
            style={{
              background: on ? ROLE_HEX[r] : 'var(--card)',
              color: on ? '#fff' : 'var(--graphite)',
              border: `1px solid ${on ? ROLE_HEX[r] : 'rgba(34,29,21,.18)'}`,
            }}
          >
            {ROLE_LABEL[r]}
          </Chip>
        );
      })}
    </div>
  );
}

function CapRow({ caps }: { caps: boolean[] }) {
  return (
    <>
      {CAP_LABELS.map((label, i) => {
        const on = caps[i];
        return (
          <span
            key={label}
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 6,
              font: `400 11.5px/1.4 ${SANS}`,
              color: on ? '#4a4337' : 'var(--graphite)',
            }}
          >
            <span
              style={{
                width: 14,
                height: 14,
                borderRadius: 4,
                flex: 'none',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                font: `600 9px/1 ${SANS}`,
                color: '#fff',
                background: on ? GREEN : '#c4bcae',
              }}
            >
              {on ? '✓' : '–'}
            </span>
            {label}
          </span>
        );
      })}
    </>
  );
}

function InviteCard({ onDone }: { onDone: () => void }) {
  const [createUser, creating] = useCreateUserMutation();
  const [name, setName] = useState('');
  const [mail, setMail] = useState('');
  const [role, setRole] = useState<Role>('secretaria');
  const [error, setError] = useState<string | null>(null);

  async function onSubmit() {
    setError(null);
    try {
      // Empty password: the account exists but signs in only by magic link.
      await createUser({ display_name: name, email: mail, role, password: '' }).unwrap();
      onDone();
    } catch (e) {
      // 409 correo_duplicado lands here with the API's own Spanish message.
      setError(apiError(e));
    }
  }

  return (
    <div
      style={{
        ...CARD,
        marginBottom: 16,
        animation: 'cpRise calc(.34s * var(--m)) cubic-bezier(.16,.84,.24,1) both',
      }}
    >
      <div
        style={{
          font: `600 9.5px/1 ${MONO}`,
          letterSpacing: '.16em',
          textTransform: 'uppercase',
          color: 'var(--gold)',
          marginBottom: 15,
        }}
      >
        Nueva cuenta
      </div>
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit,minmax(190px,1fr))',
          gap: 14,
          marginBottom: 16,
        }}
      >
        <Field label="Nombre">
          <TextInput
            value={name}
            onChange={(e) => setName(e.currentTarget.value)}
            placeholder="Ej. Rosa Elena Ávila"
          />
        </Field>
        <Field label="Correo">
          <TextInput
            type="email"
            value={mail}
            onChange={(e) => setMail(e.currentTarget.value)}
            placeholder="nombre@cristodelosalamos.mx"
          />
        </Field>
      </div>
      <Field label="Papel" style={{ marginBottom: 16 }}>
        <RoleChips value={role} onPick={setRole} />
      </Field>
      <p style={{ margin: '0 0 16px', font: `400 13.5px/1.5 ${BODY}`, color: 'var(--graphite)' }}>
        No se manda invitación: con el correo dado de alta, la persona entra con un enlace de
        acceso desde la pantalla de entrada.
      </p>
      {error && <ErrorBox style={{ marginBottom: 14 }}>{error}</ErrorBox>}
      <div style={{ display: 'flex', gap: 9, flexWrap: 'wrap' }}>
        <PrimaryButton
          disabled={creating.isLoading || !name.trim() || !mail.trim()}
          onClick={onSubmit}
          style={{ flex: '1 1 180px', background: GREEN, color: '#fff' }}
        >
          Crear cuenta
        </PrimaryButton>
        <GhostButton
          onClick={onDone}
          disabled={creating.isLoading}
          style={{ flex: '1 1 120px', padding: 13, minHeight: 48 }}
        >
          Cancelar
        </GhostButton>
      </div>
    </div>
  );
}

function PersonCard({ user, me }: { user: AdminUser; me: AdminUser }) {
  const [updateUser, updating] = useUpdateUserMutation();
  const [setUserActive, settingActive] = useSetUserActiveMutation();
  const [editing, setEditing] = useState(false);
  const [role, setRole] = useState<Role>(user.role);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [activeError, setActiveError] = useState<string | null>(null);

  // The backend also rejects self-deactivation with a 400, but the button
  // should not be offered in the first place.
  const removable = user.id !== me.id;
  const accent = ROLE_HEX[user.role];

  function beginEdit() {
    setRole(user.role);
    setSaveError(null);
    setEditing(true);
  }

  async function saveRole() {
    setSaveError(null);
    // UserInput requires email/display_name/password too; the person's own
    // values ride along unchanged since only the role is being edited.
    const body: UserInput = {
      email: user.email,
      display_name: user.display_name,
      role,
      password: '',
    };
    try {
      await updateUser({ id: user.id, body }).unwrap();
      setEditing(false);
    } catch (e) {
      setSaveError(apiError(e));
    }
  }

  async function toggleActive() {
    setActiveError(null);
    try {
      await setUserActive({ id: user.id, active: !user.is_active }).unwrap();
    } catch (e) {
      setActiveError(apiError(e));
    }
  }

  return (
    <div style={{ ...CARD, borderLeft: `3px solid ${accent}`, opacity: user.is_active ? 1 : 0.6 }}>
      <div style={{ display: 'flex', gap: 13, alignItems: 'flex-start', flexWrap: 'wrap' }}>
        <div
          style={{
            flex: 'none',
            width: 40,
            height: 40,
            borderRadius: '50%',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            font: `600 13px/1 ${SANS}`,
            color: '#fff',
            background: accent,
          }}
        >
          {initials(user.display_name)}
        </div>
        <div style={{ flex: 1, minWidth: 150 }}>
          <div style={{ font: `600 14.5px/1.3 ${SANS}`, marginBottom: 3 }}>{user.display_name}</div>
          <div style={{ font: `400 11.5px/1.4 ${MONO}`, color: 'var(--graphite)' }}>{user.email}</div>
        </div>
        <div style={{ flex: 'none', display: 'flex', flexDirection: 'column', gap: 7, alignItems: 'flex-end' }}>
          {editing ? (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 8, alignItems: 'flex-end' }}>
              <RoleChips value={role} onPick={setRole} />
              {saveError && <ErrorBox style={{ padding: '8px 10px' }}>{saveError}</ErrorBox>}
              <div style={{ display: 'flex', gap: 6 }}>
                <GhostButton
                  disabled={updating.isLoading}
                  onClick={saveRole}
                  style={{ color: GREEN, borderColor: 'rgba(47,107,79,.3)' }}
                >
                  Guardar
                </GhostButton>
                <GhostButton disabled={updating.isLoading} onClick={() => setEditing(false)}>
                  Cancelar
                </GhostButton>
              </div>
            </div>
          ) : (
            <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
              <Tag bg={`${accent}18`} fg={accent}>
                {ROLE_LABEL[user.role]}
              </Tag>
              <GhostButton onClick={beginEdit}>Editar</GhostButton>
            </div>
          )}
        </div>
      </div>

      <div
        style={{
          marginTop: 14,
          paddingTop: 13,
          borderTop: '1px solid rgba(34,29,21,.09)',
          display: 'flex',
          gap: 14,
          flexWrap: 'wrap',
          alignItems: 'center',
        }}
      >
        <CapRow caps={ROLE_CAPS[user.role]} />
        {removable &&
          (user.is_active ? (
            <DangerButton onClick={toggleActive} disabled={settingActive.isLoading} style={{ marginLeft: 'auto' }}>
              Quitar acceso
            </DangerButton>
          ) : (
            <GhostButton
              onClick={toggleActive}
              disabled={settingActive.isLoading}
              style={{ marginLeft: 'auto', color: GREEN, borderColor: 'rgba(47,107,79,.3)' }}
            >
              Reactivar
            </GhostButton>
          ))}
      </div>
      {activeError && <ErrorBox style={{ marginTop: 10 }}>{activeError}</ErrorBox>}
    </div>
  );
}

function RoleExplainerCard() {
  return (
    <div style={{ ...CARD, flex: '1 1 288px', minWidth: 0, padding: 20 }}>
      <div style={{ ...SIDE_HEADER, color: 'var(--gold)' }}>Qué puede hacer cada papel</div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 15 }}>
        <div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 6 }}>
            <span style={{ width: 11, height: 11, borderRadius: 3, background: ROLE_HEX.parroco }} />
            <span style={{ font: `600 12.5px/1.3 ${SANS}` }}>Párroco</span>
          </div>
          <p style={{ margin: 0, font: `400 14px/1.5 ${BODY}`, color: 'var(--muted)' }}>
            Todo. Publica, borra, cambia canales y administra al equipo. Solo debería haber uno.
          </p>
        </div>
        <div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 6 }}>
            <span style={{ width: 11, height: 11, borderRadius: 3, background: ROLE_HEX.secretaria }} />
            <span style={{ font: `600 12.5px/1.3 ${SANS}` }}>Secretaría</span>
          </div>
          <p style={{ margin: 0, font: `400 14px/1.5 ${BODY}`, color: 'var(--muted)' }}>
            Publica y edita eventos, ve la difusión. No toca canales ni cuentas.
          </p>
        </div>
      </div>
    </div>
  );
}

function WhyFewAccountsCard() {
  return (
    <div
      style={{
        flex: '1 1 288px',
        minWidth: 0,
        background: 'var(--panel)',
        color: 'var(--panel-fg)',
        borderRadius: 12,
        padding: 20,
      }}
    >
      <div style={{ ...SIDE_HEADER, color: 'var(--accent)' }}>Por qué son tan pocas cuentas</div>
      <p style={{ margin: '0 0 12px', font: `400 14.5px/1.58 ${BODY}`, color: 'var(--dim)' }}>
        No hay tabla de usuarios públicos, ni inscripciones, ni listas de asistentes. Nadie inicia
        sesión para ver una misa.
      </p>
      <p style={{ margin: 0, font: `400 14.5px/1.58 ${BODY}`, color: 'var(--dim)' }}>
        Eso significa nada de datos personales de la comunidad que cuidar, y una web entera que se
        puede servir estática. La difusión va a <em>canales</em>, y un canal es una cadena de
        texto.
      </p>
    </div>
  );
}

export default function Equipo() {
  const me = useSession();
  const { data: users, isLoading, isError, error } = useUsersQuery();
  const [inviting, setInviting] = useState(false);

  const list = users ?? [];
  const active = list.filter((u) => u.is_active).length;
  const inactive = list.length - active;
  const countLine = `${active} cuentas activas${inactive > 0 ? ` · ${inactive} desactivadas` : ''}`;

  return (
    <div>
      <Kicker style={{ marginBottom: 11 }}>Cuentas del equipo pastoral</Kicker>
      <H1 style={{ marginBottom: 8 }}>Equipo y permisos</H1>
      <p style={{ margin: '0 0 24px', maxWidth: '68ch', font: `400 16px/1.58 ${BODY}`, color: 'var(--muted)' }}>
        Las únicas cuentas del sistema. La comunidad nunca se registra: el calendario es público y
        de solo lectura, así que aquí solo hay cuatro o cinco personas, para siempre.
      </p>

      {isError ? (
        <ErrorBox>{apiError(error)}</ErrorBox>
      ) : isLoading ? (
        <p style={{ font: `400 15px/1.55 ${BODY}`, color: 'var(--graphite)' }}>Cargando…</p>
      ) : (
        <div style={{ display: 'flex', gap: 20, flexWrap: 'wrap', alignItems: 'flex-start' }}>
          <div style={{ flex: '1 1 480px', minWidth: 0 }}>
            <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap', alignItems: 'center', marginBottom: 16 }}>
              <PrimaryButton onClick={() => setInviting(true)}>+ Invitar a alguien</PrimaryButton>
              <span style={{ font: `400 13.5px/1.4 ${BODY}`, color: 'var(--graphite)' }}>{countLine}</span>
            </div>

            {inviting && <InviteCard onDone={() => setInviting(false)} />}

            <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
              {list.map((u) => (
                <PersonCard key={u.id} user={u} me={me} />
              ))}
            </div>
          </div>

          <div style={{ flex: '1 1 300px', minWidth: 0, display: 'flex', flexWrap: 'wrap', gap: 16, alignItems: 'flex-start' }}>
            <RoleExplainerCard />
            <WhyFewAccountsCard />
          </div>
        </div>
      )}
    </div>
  );
}
