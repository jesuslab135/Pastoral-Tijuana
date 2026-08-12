/** @jsxImportSource react */

// The last gate before a delete fires a cancellation broadcast. The mock hard-
// codes "se difundió a 4 canales"; here the copy and the checkbox both read
// off the real event and its broadcast tally, because promising an aviso a
// draft never sent (or hiding one a published event needs) is worse than
// showing nothing.

import { useState } from 'react';
import { useNavigate } from 'react-router-dom';

import { useDeleteEventMutation } from '../api';
import { apiError } from '../types';
import type { AdminEvent } from '../types';
import { BODY, ErrorBox, MONO, SANS, SERIF } from '../ui';
import type { Cast } from './casts';

interface DeleteModalProps {
  event: AdminEvent;
  cast: Cast | undefined;
  onClose: () => void;
}

/** What borrar will do, in plain terms — bound to whether this event ever went out. */
function warning(event: AdminEvent, cast: Cast | undefined): string {
  if (event.published_at === null) {
    return 'Este evento es un borrador; nunca se difundió.';
  }
  if (cast && cast.total > 0) {
    return `Este evento ya se difundió a ${cast.total} canales. Si lo borras, desaparece del calendario público al instante.`;
  }
  return 'Este evento está publicado pero todavía no se ha difundido a ningún canal.';
}

export default function DeleteModal({ event, cast, onClose }: DeleteModalProps) {
  const navigate = useNavigate();
  const [deleteEvent, deleting] = useDeleteEventMutation();
  const [error, setError] = useState<string | null>(null);
  const [notify, setNotify] = useState(true);
  const [pressed, setPressed] = useState(false);

  const published = event.published_at !== null;

  async function onConfirm() {
    setError(null);
    try {
      // A draft was never announced, so its delete never promises an aviso —
      // the checkbox doesn't even render for one, so its own state is moot.
      await deleteEvent({ id: event.id, notify: published && notify }).unwrap();
      navigate('/eventos');
    } catch (e) {
      setError(apiError(e));
    }
  }

  return (
    <div
      role="presentation"
      onClick={onClose}
      style={{
        position: 'fixed',
        inset: 0,
        background: 'rgba(34,29,21,.45)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: 20,
        zIndex: 40,
      }}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Confirmar borrado"
        onClick={(e) => e.stopPropagation()}
        style={{
          width: '100%',
          maxWidth: 560,
          animation: 'cpRise calc(.34s * var(--m)) cubic-bezier(.16,.84,.24,1) both',
        }}
      >
        <div
          style={{
            background: 'var(--card)',
            border: '1px solid rgba(160,47,39,.32)',
            borderRadius: 12,
            overflow: 'hidden',
            boxShadow: '0 16px 40px rgba(34,29,21,.14)',
          }}
        >
          <div
            style={{
              padding: '5px 18px',
              background: 'var(--red)',
              font: `600 9px/1 ${MONO}`,
              letterSpacing: '.18em',
              textTransform: 'uppercase',
              color: '#fff',
            }}
          >
            Confirmar borrado
          </div>

          <div style={{ padding: 24 }}>
            <h2 style={{ margin: '0 0 10px', font: `400 25px/1.14 ${SERIF}` }}>
              {event.title || 'Evento sin título'}
            </h2>
            <p style={{ margin: '0 0 20px', font: `400 15.5px/1.6 ${BODY}`, color: 'var(--muted)' }}>
              {warning(event, cast)}
            </p>

            {published && (
              <button
                type="button"
                onClick={() => setNotify((n) => !n)}
                style={{
                  display: 'flex',
                  gap: 12,
                  alignItems: 'flex-start',
                  width: '100%',
                  textAlign: 'left',
                  padding: 14,
                  borderRadius: 8,
                  background: 'var(--wash)',
                  border: '1px solid rgba(34,29,21,.14)',
                  cursor: 'pointer',
                  marginBottom: 20,
                }}
              >
                <span
                  style={{
                    flex: 'none',
                    width: 20,
                    height: 20,
                    borderRadius: 5,
                    marginTop: 1,
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    font: `600 11px/1 ${SANS}`,
                    background: notify ? 'var(--red)' : 'transparent',
                    color: '#fff',
                    border: `1px solid ${notify ? 'var(--red)' : 'rgba(34,29,21,.3)'}`,
                  }}
                >
                  {notify ? '✓' : ''}
                </span>
                <span style={{ flex: 1 }}>
                  <span
                    style={{
                      display: 'block',
                      font: `600 12.5px/1.3 ${SANS}`,
                      marginBottom: 3,
                      color: 'var(--ink)',
                    }}
                  >
                    Avisar la cancelación
                  </span>
                  <span style={{ display: 'block', font: `400 14px/1.5 ${BODY}`, color: 'var(--muted)' }}>
                    Manda un mensaje de cancelación a los mismos grupos. El silencio es peor que el
                    aviso.
                  </span>
                </span>
              </button>
            )}

            {error && <ErrorBox style={{ marginBottom: 20 }}>{error}</ErrorBox>}

            <div style={{ display: 'flex', gap: 9, flexWrap: 'wrap' }}>
              <button
                type="button"
                onClick={onConfirm}
                disabled={deleting.isLoading}
                onMouseEnter={() => setPressed(true)}
                onMouseLeave={() => setPressed(false)}
                style={{
                  flex: '1 1 180px',
                  padding: 14,
                  borderRadius: 8,
                  border: 'none',
                  background: 'var(--red)',
                  color: '#fff',
                  font: `600 12.5px/1 ${SANS}`,
                  minHeight: 48,
                  cursor: deleting.isLoading ? 'default' : 'pointer',
                  opacity: deleting.isLoading ? 0.7 : 1,
                  transform: pressed && !deleting.isLoading ? 'translateY(-2px)' : 'none',
                  transition: 'transform calc(.2s * var(--m))',
                }}
              >
                Borrar el evento
              </button>
              <button
                type="button"
                onClick={onClose}
                disabled={deleting.isLoading}
                style={{
                  flex: '1 1 140px',
                  padding: 14,
                  borderRadius: 8,
                  border: '1px solid rgba(34,29,21,.2)',
                  background: 'var(--card)',
                  color: 'var(--muted)',
                  font: `600 12.5px/1 ${SANS}`,
                  minHeight: 48,
                  cursor: 'pointer',
                }}
              >
                Conservarlo
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
