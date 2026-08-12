-- +goose Up
-- The targets are deliberate placeholders: the párroco sets the real WhatsApp
-- group JIDs and the real mailing address from the admin panel. Nothing is
-- ever sent to them, because the WhatsApp sender is a stub in v1.
INSERT INTO channels (id, kind, name, target, group_id, is_active) VALUES
  ('b1000000-0000-4000-8000-000000000001', 'whatsapp', 'Avisos toda la parroquia', 'PENDIENTE-JID-GRUPO-GENERAL',  NULL, true),
  ('b1000000-0000-4000-8000-000000000002', 'whatsapp', 'Avisos liturgia',          'PENDIENTE-JID-GRUPO-LITURGIA', 'a1000000-0000-4000-8000-000000000001', true),
  ('b1000000-0000-4000-8000-000000000003', 'email',    'Boletín por correo',       'avisos@parroquia.mx',          NULL, true);

-- +goose Down
-- Scoped to the fixed seed UUIDs: channels the parish added by hand must
-- survive a rollback of this seed.
DELETE FROM channels WHERE id IN (
  'b1000000-0000-4000-8000-000000000001',
  'b1000000-0000-4000-8000-000000000002',
  'b1000000-0000-4000-8000-000000000003');
