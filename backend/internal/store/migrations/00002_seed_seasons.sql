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
DELETE FROM liturgical_seasons;
