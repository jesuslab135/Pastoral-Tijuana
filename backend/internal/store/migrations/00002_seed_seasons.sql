-- +goose Up
INSERT INTO liturgical_seasons (name, color, date_range) VALUES
  ('Navidad',             'blanco_oro', '[2026-01-01,2026-01-12)'),
  ('Tiempo Ordinario',    'verde',      '[2026-01-12,2026-02-18)'),
  ('Cuaresma',            'violeta',    '[2026-02-18,2026-04-05)'),
  ('Pascua',              'blanco_oro', '[2026-04-05,2026-05-25)'),
  ('Tiempo Ordinario',    'verde',      '[2026-05-25,2026-11-29)'),
  ('Adviento',            'violeta',    '[2026-11-29,2026-12-13)'),
  ('Adviento · Gaudete',  'rosa',       '[2026-12-13,2026-12-14)'),
  ('Adviento',            'violeta',    '[2026-12-14,2026-12-25)'),
  ('Navidad',             'blanco_oro', '[2026-12-25,2027-01-11)'),
  ('Tiempo Ordinario',    'verde',      '[2027-01-11,2027-02-10)'),
  ('Cuaresma',            'violeta',    '[2027-02-10,2027-03-28)'),
  ('Pascua',              'blanco_oro', '[2027-03-28,2027-05-17)'),
  ('Tiempo Ordinario',    'verde',      '[2027-05-17,2027-11-28)'),
  ('Adviento',            'violeta',    '[2027-11-28,2027-12-12)'),
  ('Adviento · Gaudete',  'rosa',       '[2027-12-12,2027-12-13)'),
  ('Adviento',            'violeta',    '[2027-12-13,2027-12-25)'),
  ('Navidad',             'blanco_oro', '[2027-12-25,2028-01-10)');

-- +goose Down
-- Scoped to the exact seeded rows: season ranges the parish added by hand
-- must survive a rollback of this seed.
DELETE FROM liturgical_seasons WHERE (name, date_range) IN (VALUES
  ('Navidad',             '[2026-01-01,2026-01-12)'::daterange),
  ('Tiempo Ordinario',    '[2026-01-12,2026-02-18)'::daterange),
  ('Cuaresma',            '[2026-02-18,2026-04-05)'::daterange),
  ('Pascua',              '[2026-04-05,2026-05-25)'::daterange),
  ('Tiempo Ordinario',    '[2026-05-25,2026-11-29)'::daterange),
  ('Adviento',            '[2026-11-29,2026-12-13)'::daterange),
  ('Adviento · Gaudete',  '[2026-12-13,2026-12-14)'::daterange),
  ('Adviento',            '[2026-12-14,2026-12-25)'::daterange),
  ('Navidad',             '[2026-12-25,2027-01-11)'::daterange),
  ('Tiempo Ordinario',    '[2027-01-11,2027-02-10)'::daterange),
  ('Cuaresma',            '[2027-02-10,2027-03-28)'::daterange),
  ('Pascua',              '[2027-03-28,2027-05-17)'::daterange),
  ('Tiempo Ordinario',    '[2027-05-17,2027-11-28)'::daterange),
  ('Adviento',            '[2027-11-28,2027-12-12)'::daterange),
  ('Adviento · Gaudete',  '[2027-12-12,2027-12-13)'::daterange),
  ('Adviento',            '[2027-12-13,2027-12-25)'::daterange),
  ('Navidad',             '[2027-12-25,2028-01-10)'::daterange));
