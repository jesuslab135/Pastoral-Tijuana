-- +goose Up
INSERT INTO parish_groups (id, name, slug, is_public, sort) VALUES
  ('a1000000-0000-4000-8000-000000000001', 'Liturgia',         'liturgia',   true, 1),
  ('a1000000-0000-4000-8000-000000000002', 'Catequesis',       'catequesis', true, 2),
  ('a1000000-0000-4000-8000-000000000003', 'Pastoral juvenil', 'juvenil',    true, 3),
  ('a1000000-0000-4000-8000-000000000004', 'Coro',             'coro',       true, 4),
  ('a1000000-0000-4000-8000-000000000005', 'Caridad',          'caridad',    true, 5),
  ('a1000000-0000-4000-8000-000000000006', 'Formación',        'formacion',  true, 6);

-- +goose Down
DELETE FROM parish_groups;
