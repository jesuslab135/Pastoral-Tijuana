/** @jsxImportSource react */

// The mock's "Canales conectados" card had fake health bars; the actual job
// here is letting the párroco fix what the seed only guessed at — a WhatsApp
// channel's target starts as a placeholder JID until someone edits it in.

import { useState } from 'react';
import type { CSSProperties } from 'react';

import {
  useChannelsQuery,
  useCreateChannelMutation,
  useDeleteChannelMutation,
  useGroupsQuery,
  useUpdateChannelMutation,
} from '../api';
import { apiError } from '../types';
import type { Channel, ChannelInput } from '../types';
import {
  BODY,
  DangerButton,
  ErrorBox,
  Field,
  GREEN,
  GhostButton,
  MONO,
  PrimaryButton,
  SANS,
  TextInput,
} from '../ui';

const KIND_ICON: Record<Channel['kind'], string> = { whatsapp: 'WA', email: '✉' };

// Sentinel for the "toda la parroquia" option: the wire value is `null`, but a
// <select> needs a real string to bind to.
const ALL_PARISH = '__toda_la_parroquia__';

const HEADER: CSSProperties = {
  font: `600 9.5px/1 ${MONO}`,
  letterSpacing: '.18em',
  textTransform: 'uppercase',
  color: 'var(--gold)',
};

const ROW_BOX: CSSProperties = {
  padding: 12,
  borderRadius: 8,
  border: '1px solid rgba(34,29,21,.12)',
  background: 'var(--wash)',
};

const SELECT: CSSProperties = {
  width: '100%',
  padding: 12,
  border: '1px solid rgba(34,29,21,.22)',
  borderRadius: 8,
  background: 'var(--card)',
  font: `500 13px/1.3 ${SANS}`,
  color: 'var(--ink)',
  outline: 'none',
};

function blankChannel(): ChannelInput {
  return { kind: 'whatsapp', name: '', target: '', group_id: null, is_active: true };
}

interface GroupOption {
  id: string;
  name: string;
}

interface FormProps {
  initial: ChannelInput;
  groupOptions: GroupOption[];
  busy: boolean;
  error: string | null;
  onCancel: () => void;
  onSave: (input: ChannelInput) => void;
}

function ChannelForm({ initial, groupOptions, busy, error, onCancel, onSave }: FormProps) {
  const [form, setForm] = useState<ChannelInput>(initial);
  const set = <K extends keyof ChannelInput>(key: K, value: ChannelInput[K]) =>
    setForm((f) => ({ ...f, [key]: value }));

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
      <Field label="Nombre">
        <TextInput value={form.name} onChange={(e) => set('name', e.currentTarget.value)} />
      </Field>
      <Field label="Destino">
        <TextInput
          value={form.target}
          onChange={(e) => set('target', e.currentTarget.value)}
          style={{ font: `500 13px/1.3 ${MONO}` }}
        />
      </Field>
      <Field label="Tipo">
        <select
          value={form.kind}
          onChange={(e) => set('kind', e.currentTarget.value as Channel['kind'])}
          style={SELECT}
        >
          <option value="whatsapp">WhatsApp</option>
          <option value="email">Correo</option>
        </select>
      </Field>
      <Field label="Grupo">
        <select
          value={form.group_id ?? ALL_PARISH}
          onChange={(e) =>
            set('group_id', e.currentTarget.value === ALL_PARISH ? null : e.currentTarget.value)
          }
          style={SELECT}
        >
          <option value={ALL_PARISH}>Toda la parroquia</option>
          {groupOptions.map((g) => (
            <option key={g.id} value={g.id}>
              {g.name}
            </option>
          ))}
        </select>
      </Field>
      <label
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 8,
          font: `500 12px/1 ${SANS}`,
          color: 'var(--graphite)',
        }}
      >
        <input
          type="checkbox"
          checked={form.is_active}
          onChange={(e) => set('is_active', e.currentTarget.checked)}
        />
        Activo
      </label>
      {error && <ErrorBox>{error}</ErrorBox>}
      <div style={{ display: 'flex', gap: 7 }}>
        <PrimaryButton
          disabled={busy}
          onClick={() => onSave(form)}
          style={{ flex: 1, padding: 11, minHeight: 40, font: `600 12px/1 ${SANS}` }}
        >
          Guardar
        </PrimaryButton>
        <GhostButton
          disabled={busy}
          onClick={onCancel}
          style={{ flex: 'none', padding: '11px 14px', minHeight: 40 }}
        >
          Cancelar
        </GhostButton>
      </div>
    </div>
  );
}

interface RowProps {
  channel: Channel;
  groupName: string;
  groupOptions: GroupOption[];
  editing: boolean;
  onEdit: () => void;
  onCancelEdit: () => void;
}

function ChannelRow({ channel, groupName, groupOptions, editing, onEdit, onCancelEdit }: RowProps) {
  const [updateChannel, updating] = useUpdateChannelMutation();
  const [deleteChannel, deleting] = useDeleteChannelMutation();
  const [saveError, setSaveError] = useState<string | null>(null);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  async function onSave(input: ChannelInput) {
    setSaveError(null);
    try {
      await updateChannel({ id: channel.id, body: input }).unwrap();
      onCancelEdit();
    } catch (e) {
      setSaveError(apiError(e));
    }
  }

  async function onDelete() {
    setDeleteError(null);
    try {
      await deleteChannel(channel.id).unwrap();
    } catch (e) {
      // A channel still referenced by broadcasts answers 409; the API's own
      // message tells the párroco to deactivate instead, so it is shown as-is.
      setDeleteError(apiError(e));
    }
  }

  if (editing) {
    return (
      <div style={ROW_BOX}>
        <ChannelForm
          initial={{
            kind: channel.kind,
            name: channel.name,
            target: channel.target,
            group_id: channel.group_id,
            is_active: channel.is_active,
          }}
          groupOptions={groupOptions}
          busy={updating.isLoading}
          error={saveError}
          onCancel={onCancelEdit}
          onSave={onSave}
        />
      </div>
    );
  }

  return (
    <div style={ROW_BOX}>
      <div style={{ display: 'flex', gap: 9, alignItems: 'center', marginBottom: 7 }}>
        <span
          style={{
            flex: 'none',
            font: `700 9px/1 ${MONO}`,
            letterSpacing: '.04em',
            color: 'var(--graphite)',
          }}
        >
          {KIND_ICON[channel.kind]}
        </span>
        <span
          style={{
            flex: 1,
            minWidth: 0,
            font: `600 12.5px/1.3 ${SANS}`,
            whiteSpace: 'nowrap',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
          }}
        >
          {channel.name}
        </span>
        <span
          style={{
            flex: 'none',
            width: 8,
            height: 8,
            borderRadius: '50%',
            background: channel.is_active ? GREEN : 'var(--graphite)',
          }}
        />
        <span
          style={{
            font: `500 9px/1 ${MONO}`,
            letterSpacing: '.08em',
            color: channel.is_active ? GREEN : 'var(--graphite)',
          }}
        >
          {channel.is_active ? 'ACTIVO' : 'INACTIVO'}
        </span>
      </div>
      <div
        style={{
          font: `400 10.5px/1.4 ${MONO}`,
          color: 'var(--graphite)',
          marginBottom: 4,
          wordBreak: 'break-all',
        }}
      >
        {channel.target}
      </div>
      <div style={{ font: `400 11px/1.3 ${SANS}`, color: 'var(--graphite)', marginBottom: 10 }}>
        {groupName}
      </div>
      {deleteError && <ErrorBox style={{ marginBottom: 8 }}>{deleteError}</ErrorBox>}
      <div style={{ display: 'flex', gap: 7 }}>
        <GhostButton onClick={onEdit} style={{ flex: 1 }}>
          Editar
        </GhostButton>
        <DangerButton onClick={onDelete} disabled={deleting.isLoading} aria-label={`Eliminar ${channel.name}`}>
          ✕
        </DangerButton>
      </div>
    </div>
  );
}

export default function ChannelsCard() {
  const { data: channels } = useChannelsQuery();
  const { data: groups } = useGroupsQuery();
  const [createChannel, creating] = useCreateChannelMutation();
  const [editingId, setEditingId] = useState<string | 'new' | null>(null);
  const [createError, setCreateError] = useState<string | null>(null);

  const groupOptions: GroupOption[] = (groups ?? []).map((g) => ({ id: g.id, name: g.name }));
  const groupName = (id: string | null) =>
    id === null ? 'Toda la parroquia' : (groupOptions.find((g) => g.id === id)?.name ?? '—');

  async function onCreate(input: ChannelInput) {
    setCreateError(null);
    try {
      await createChannel(input).unwrap();
      setEditingId(null);
    } catch (e) {
      setCreateError(apiError(e));
    }
  }

  return (
    <div
      id="canales"
      style={{
        flex: '1 1 288px',
        minWidth: 0,
        background: 'var(--card)',
        border: '1px solid rgba(34,29,21,.14)',
        borderRadius: 12,
        padding: 20,
      }}
    >
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: 8,
          marginBottom: 15,
        }}
      >
        <div style={HEADER}>Canales conectados</div>
        {editingId === null && (
          <GhostButton
            onClick={() => setEditingId('new')}
            style={{ padding: '6px 10px', minHeight: 30, font: `600 10.5px/1 ${SANS}` }}
          >
            + Nuevo canal
          </GhostButton>
        )}
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: 11 }}>
        {editingId === 'new' && (
          <div style={ROW_BOX}>
            <ChannelForm
              initial={blankChannel()}
              groupOptions={groupOptions}
              busy={creating.isLoading}
              error={createError}
              onCancel={() => {
                setEditingId(null);
                setCreateError(null);
              }}
              onSave={onCreate}
            />
          </div>
        )}

        {(channels ?? []).map((c) => (
          <ChannelRow
            key={c.id}
            channel={c}
            groupName={groupName(c.group_id)}
            groupOptions={groupOptions}
            editing={editingId === c.id}
            onEdit={() => setEditingId(c.id)}
            onCancelEdit={() => setEditingId(null)}
          />
        ))}

        {(channels ?? []).length === 0 && editingId !== 'new' && (
          <p style={{ margin: 0, font: `400 14px/1.5 ${BODY}`, color: 'var(--graphite)' }}>
            No hay canales configurados todavía.
          </p>
        )}
      </div>
    </div>
  );
}
