/** @jsxImportSource react */

// The event editor. Publishing from here is not a save — it announces the event
// to the parish's WhatsApp groups and mailing list — so the actions are named
// for what they do to the parish, not for what they do to the row.

import { useEffect, useMemo, useState } from 'react';
import { useLocation, useNavigate, useParams } from 'react-router-dom';

import {
  useBroadcastsQuery,
  useChannelsQuery,
  useCreateEventMutation,
  useGetEventQuery,
  useGroupsQuery,
  usePublishEventMutation,
  useUnpublishEventMutation,
  useUpdateEventMutation,
} from '../api';
import { parishToISO } from '../dates';
import { apiError } from '../types';
import type { Channel, Rank } from '../types';
import {
  BODY,
  Chip,
  ErrorBox,
  Field,
  GREEN,
  GhostButton,
  H1,
  MONO,
  PrimaryButton,
  SANS,
  TextInput,
} from '../ui';
import { castsByEvent } from './casts';
import DeleteModal from './DeleteModal';
import { DURATIONS, fromEvent, toInput, validate } from './form';
import type { EventForm } from './form';

const RANKS: Array<{ value: Rank; label: string }> = [
  { value: 'solemnidad', label: 'Solemnidad' },
  { value: 'fiesta', label: 'Fiesta' },
  { value: 'memoria', label: 'Memoria' },
  { value: 'parroquial', label: 'Parroquial' },
];

const INPUT_BORDER = '1px solid rgba(34,29,21,.22)';

/** Today on the parish clock, as the date input wants it. */
function todayParish(): string {
  return parishToISO(new Date().toISOString().slice(0, 10), '12:00').slice(0, 10);
}

function emptyForm(groupID: string): EventForm {
  return {
    title: '',
    date: todayParish(),
    time: '12:00',
    durationMin: 90,
    place: 'Templo parroquial',
    group_id: groupID,
    rank: 'parroquial',
    description: '',
    color_override: null,
  };
}

/** Channels this event would reach: its group's, plus the parish-wide ones. */
function reaches(c: Channel, groupID: string): boolean {
  return c.is_active && (c.group_id === null || c.group_id === groupID);
}

export default function EventoEditor() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const location = useLocation();

  const groups = useGroupsQuery();
  const channels = useChannelsQuery();
  // Unfiltered, so this reads the same cache entry the list and Difusión fill.
  const broadcasts = useBroadcastsQuery({});
  // `skip` keeps the new-event route from requesting an event that has no id.
  const existing = useGetEventQuery(id ?? '', { skip: !id });

  const [createEvent, creating] = useCreateEventMutation();
  const [updateEvent, updating] = useUpdateEventMutation();
  const [publishEvent, publishing] = usePublishEventMutation();
  const [unpublishEvent, unpublishing] = useUnpublishEventMutation();

  const [form, setForm] = useState<EventForm | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [askDelete, setAskDelete] = useState(false);

  const event = existing.data;
  const groupList = useMemo(() => groups.data ?? [], [groups.data]);
  const casts = useMemo(() => castsByEvent(broadcasts.data ?? []), [broadcasts.data]);

  // The form is seeded once, from whichever source this route has: a loaded
  // event, or the first group for a blank one.
  useEffect(() => {
    if (form) return;
    if (id) {
      if (event) setForm(fromEvent(event));
      return;
    }
    if (groupList.length > 0) setForm(emptyForm(groupList[0]!.id));
  }, [form, id, event, groupList]);

  // The list's ✕ navigates here with this flag so the secretary lands
  // straight on the confirmation instead of the editor she didn't ask for.
  useEffect(() => {
    if ((location.state as { delete?: boolean } | null)?.delete) setAskDelete(true);
  }, [location.state]);

  const set = <K extends keyof EventForm>(key: K, value: EventForm[K]) =>
    setForm((f) => (f ? { ...f, [key]: value } : f));

  if (!form) {
    return (
      <p style={{ font: `400 15px/1.55 ${BODY}`, color: 'var(--graphite)' }}>Cargando…</p>
    );
  }

  const published = event?.published_at != null;
  const busy =
    creating.isLoading || updating.isLoading || publishing.isLoading || unpublishing.isLoading;

  /** Saves the form, and returns the event id — the caller decides what next. */
  async function save(): Promise<string | null> {
    const problem = validate(form!);
    if (problem) {
      setError(problem);
      return null;
    }
    setError(null);
    try {
      const body = toInput(form!);
      if (id) {
        await updateEvent({ id, body }).unwrap();
        return id;
      }
      const created = await createEvent(body).unwrap();
      return created.id;
    } catch (e) {
      setError(apiError(e));
      return null;
    }
  }

  async function onSave() {
    if ((await save()) !== null) navigate('/eventos');
  }

  async function onPublish() {
    const savedID = await save();
    if (savedID === null) return;
    try {
      await publishEvent(savedID).unwrap();
      navigate('/eventos');
    } catch (e) {
      setError(apiError(e));
    }
  }

  async function onUnpublish() {
    if (!id) return;
    setError(null);
    try {
      await unpublishEvent(id).unwrap();
      navigate('/eventos');
    } catch (e) {
      setError(apiError(e));
    }
  }

  const recipients = (channels.data ?? []).filter((c) => reaches(c, form.group_id));
  const others = (channels.data ?? []).filter((c) => !reaches(c, form.group_id));

  return (
    <div style={{ animation: 'cpRise calc(.4s * var(--m)) cubic-bezier(.16,.84,.24,1) both' }}>
      <button
        type="button"
        onClick={() => navigate('/eventos')}
        style={{
          border: 'none',
          background: 'none',
          padding: 0,
          marginBottom: 16,
          cursor: 'pointer',
          font: `500 11.5px/1 ${MONO}`,
          letterSpacing: '.08em',
          color: 'var(--gold)',
          minHeight: 32,
        }}
      >
        ← VOLVER A EVENTOS
      </button>

      <H1 style={{ marginBottom: 6 }}>{id ? 'Editar evento' : 'Nuevo evento'}</H1>
      <p
        style={{
          margin: '0 0 28px',
          maxWidth: '62ch',
          font: `400 16px/1.55 ${BODY}`,
          color: 'var(--muted)',
        }}
      >
        El color no se elige: lo hereda del tiempo litúrgico del día. Solo una fiesta patronal
        puede pedir un color propio.
      </p>

      <div style={{ display: 'flex', gap: 20, flexWrap: 'wrap', alignItems: 'flex-start' }}>
        <div
          style={{
            flex: '1 1 440px',
            minWidth: 0,
            display: 'flex',
            flexDirection: 'column',
            gap: 18,
          }}
        >
          <Field label="Título">
            <TextInput
              value={form.title}
              placeholder="Ej. Kermés parroquial"
              onChange={(e) => set('title', e.currentTarget.value)}
            />
          </Field>

          <div
            style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(auto-fit,minmax(140px,1fr))',
              gap: 14,
            }}
          >
            <Field label="Fecha">
              <TextInput
                type="date"
                value={form.date}
                onChange={(e) => set('date', e.currentTarget.value)}
                style={{ font: `500 14px/1.3 ${MONO}`, padding: '13px 14px' }}
              />
            </Field>
            <Field label="Hora">
              <TextInput
                type="time"
                value={form.time}
                onChange={(e) => set('time', e.currentTarget.value)}
                style={{ font: `500 14px/1.3 ${MONO}`, padding: '13px 14px' }}
              />
            </Field>
            <Field label="Duración">
              <select
                value={String(form.durationMin)}
                onChange={(e) => set('durationMin', Number(e.currentTarget.value))}
                style={{
                  width: '100%',
                  padding: '13px 14px',
                  border: INPUT_BORDER,
                  borderRadius: 8,
                  background: 'var(--card)',
                  font: `500 14px/1.3 ${MONO}`,
                  color: 'var(--ink)',
                  outline: 'none',
                }}
              >
                {/* An event whose stored span is not on the list keeps its own
                    option, so opening it never moves its end time. */}
                {!DURATIONS.some((d) => d.min === form.durationMin) && (
                  <option value={form.durationMin}>{form.durationMin} min</option>
                )}
                {DURATIONS.map((d) => (
                  <option key={d.min} value={d.min}>
                    {d.label}
                  </option>
                ))}
              </select>
            </Field>
          </div>

          <Field label="Lugar">
            <TextInput
              value={form.place}
              onChange={(e) => set('place', e.currentTarget.value)}
              style={{ font: `500 14px/1.3 ${SANS}` }}
            />
          </Field>

          <Field label="Grupo parroquial">
            <div style={{ display: 'flex', gap: 7, flexWrap: 'wrap' }}>
              {groupList.map((g) => {
                const on = form.group_id === g.id;
                return (
                  <Chip
                    key={g.id}
                    on={on}
                    onClick={() => set('group_id', g.id)}
                    style={{
                      padding: '10px 14px',
                      minHeight: 40,
                      font: `500 12px/1 ${SANS}`,
                      background: on ? GREEN : 'var(--card)',
                      borderColor: on ? GREEN : 'rgba(34,29,21,.18)',
                      color: on ? '#fff' : 'var(--graphite)',
                    }}
                  >
                    {g.name}
                  </Chip>
                );
              })}
            </div>
          </Field>

          <Field label="Rango litúrgico">
            <div
              style={{
                display: 'flex',
                flexWrap: 'wrap',
                border: '1px solid rgba(34,29,21,.18)',
                borderRadius: 8,
                overflow: 'hidden',
                width: 'fit-content',
                maxWidth: '100%',
              }}
            >
              {RANKS.map((r) => {
                const on = form.rank === r.value;
                return (
                  <button
                    key={r.value}
                    type="button"
                    onClick={() => set('rank', r.value)}
                    style={{
                      padding: '11px 15px',
                      border: 'none',
                      borderRight: '1px solid rgba(34,29,21,.13)',
                      cursor: 'pointer',
                      font: `500 12px/1 ${SANS}`,
                      minHeight: 42,
                      transition: 'all calc(.2s * var(--m))',
                      background: on ? 'var(--panel)' : 'var(--card)',
                      color: on ? 'var(--panel-bright)' : 'var(--graphite)',
                    }}
                  >
                    {r.label}
                  </button>
                );
              })}
            </div>
          </Field>

          <Field label="Descripción para el aviso">
            <textarea
              rows={3}
              value={form.description}
              onChange={(e) => set('description', e.currentTarget.value)}
              style={{
                width: '100%',
                padding: 14,
                border: INPUT_BORDER,
                borderRadius: 8,
                background: 'var(--card)',
                font: `400 15px/1.55 ${BODY}`,
                color: 'var(--ink)',
                outline: 'none',
                resize: 'vertical',
              }}
            />
            <p
              style={{
                margin: '8px 0 0',
                font: `400 13.5px/1.5 ${BODY}`,
                color: 'var(--graphite)',
              }}
            >
              Esto es lo que la gente leerá en WhatsApp. Dos líneas bastan.
            </p>
          </Field>
        </div>

        <div
          style={{
            flex: '1 1 300px',
            minWidth: 0,
            display: 'flex',
            flexDirection: 'column',
            gap: 16,
          }}
        >
          <div
            style={{
              background: 'var(--wash)',
              border: '1px solid rgba(34,29,21,.14)',
              borderRadius: 11,
              padding: 20,
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
              Al guardar se difunde a
            </div>

            <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
              {[...recipients, ...others].map((c) => {
                const on = reaches(c, form.group_id);
                return (
                  <div
                    key={c.id}
                    style={{
                      display: 'flex',
                      gap: 10,
                      alignItems: 'center',
                      padding: 11,
                      background: 'var(--card)',
                      borderRadius: 7,
                      border: `1px ${on ? 'solid' : 'dashed'} ${
                        on ? 'rgba(34,29,21,.12)' : 'rgba(34,29,21,.2)'
                      }`,
                      opacity: on ? 1 : 0.6,
                    }}
                  >
                    <span
                      style={{
                        width: 8,
                        height: 8,
                        borderRadius: '50%',
                        flex: 'none',
                        background: on ? GREEN : 'var(--graphite)',
                      }}
                    />
                    <div style={{ flex: 1, minWidth: 0 }}>
                      <div style={{ font: `600 12px/1.3 ${SANS}` }}>{c.name}</div>
                      <div style={{ font: `400 10px/1.3 ${MONO}`, color: 'var(--graphite)' }}>
                        {c.kind === 'whatsapp' ? 'WhatsApp · SIMULADO en v1' : c.target}
                      </div>
                    </div>
                  </div>
                );
              })}
              {channels.data?.length === 0 && (
                <p style={{ margin: 0, font: `400 14px/1.5 ${BODY}`, color: 'var(--graphite)' }}>
                  No hay canales configurados todavía.
                </p>
              )}
            </div>

            {error && <ErrorBox style={{ marginTop: 16 }}>{error}</ErrorBox>}

            <PrimaryButton
              onClick={published ? onSave : onPublish}
              disabled={busy}
              style={{
                width: '100%',
                marginTop: 16,
                background: GREEN,
                color: '#fff',
                // Lets the shared hover shadow take this button's own colour.
                ['--btn-shadow' as string]: 'rgba(47,107,79,.3)',
              }}
            >
              {published ? 'Guardar cambios' : 'Publicar y difundir'}
            </PrimaryButton>

            <GhostButton
              onClick={published ? onUnpublish : onSave}
              disabled={busy}
              style={{
                width: '100%',
                marginTop: 8,
                padding: 13,
                minHeight: 46,
                borderRadius: 8,
                border: '1px solid rgba(34,29,21,.18)',
                background: 'none',
                font: `500 12px/1 ${SANS}`,
              }}
            >
              {published ? 'Despublicar' : id ? 'Guardar borrador' : 'Guardar como borrador'}
            </GhostButton>

            <p
              style={{
                margin: '13px 0 0',
                font: `400 13.5px/1.5 ${BODY}`,
                color: 'var(--graphite)',
              }}
            >
              {published
                ? 'Despublicar lo quita de la web y avisa la cancelación a quienes lo recibieron.'
                : 'Borrador no difunde y no aparece en la web. Es el único freno de emergencia.'}
            </p>
          </div>

          {id && (
            <div
              style={{
                background: 'var(--card)',
                border: '1px dashed rgba(160,47,39,.4)',
                borderRadius: 11,
                padding: 20,
              }}
            >
              <div
                style={{
                  font: `600 9.5px/1 ${MONO}`,
                  letterSpacing: '.16em',
                  textTransform: 'uppercase',
                  color: 'var(--red)',
                  marginBottom: 11,
                }}
              >
                Zona delicada
              </div>
              <p
                style={{
                  margin: '0 0 14px',
                  font: `400 14px/1.5 ${BODY}`,
                  color: 'var(--muted)',
                }}
              >
                Borrar un evento ya publicado dispara un aviso de cancelación a los mismos canales
                que recibieron el original.
              </p>
              <button
                type="button"
                onClick={() => setAskDelete(true)}
                style={{
                  width: '100%',
                  padding: 12,
                  borderRadius: 7,
                  border: '1px solid rgba(160,47,39,.32)',
                  background: 'none',
                  color: 'var(--red)',
                  font: `600 12px/1 ${SANS}`,
                  minHeight: 44,
                  cursor: 'pointer',
                }}
              >
                Borrar este evento
              </button>
            </div>
          )}
        </div>
      </div>

      {askDelete && event && (
        <DeleteModal
          event={event}
          cast={casts.get(event.id)}
          onClose={() => setAskDelete(false)}
        />
      )}
    </div>
  );
}
