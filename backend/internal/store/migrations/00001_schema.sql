-- +goose Up
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TYPE season_color    AS ENUM ('verde','violeta','rosa','blanco_oro','rojo');
CREATE TYPE event_rank      AS ENUM ('solemnidad','fiesta','memoria','parroquial');
CREATE TYPE channel_kind    AS ENUM ('whatsapp','email');
CREATE TYPE outbox_kind     AS ENUM ('published','updated','cancelled');
CREATE TYPE broadcast_state AS ENUM ('queued','sent','failed','dead');
CREATE TYPE user_role       AS ENUM ('parroco','secretaria');

CREATE TABLE liturgical_seasons (
  id         smallserial PRIMARY KEY,
  name       text         NOT NULL,
  color      season_color NOT NULL,
  date_range daterange    NOT NULL,
  EXCLUDE USING gist (date_range WITH &&)
);

CREATE TABLE parish_groups (
  id        uuid PRIMARY KEY,
  name      text NOT NULL,
  slug      text NOT NULL UNIQUE,
  is_public boolean NOT NULL DEFAULT true,
  sort      integer NOT NULL DEFAULT 0
);

CREATE TABLE users (
  id            uuid PRIMARY KEY,
  email         citext NOT NULL UNIQUE,
  password_hash text,
  display_name  text NOT NULL DEFAULT '',
  role          user_role NOT NULL,
  is_active     boolean NOT NULL DEFAULT true,
  created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
  id         uuid PRIMARY KEY,
  user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash bytea NOT NULL UNIQUE,
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL,
  revoked_at timestamptz,
  ip         inet,
  user_agent text
);

CREATE TABLE events (
  id             uuid PRIMARY KEY,
  title          text NOT NULL,
  slug           text,
  description    text,
  place          text,
  starts_at      timestamptz NOT NULL,
  ends_at        timestamptz NOT NULL,
  group_id       uuid NOT NULL REFERENCES parish_groups(id),
  rank           event_rank NOT NULL,
  color_override season_color,
  published_at   timestamptz,
  cancelled_at   timestamptz,
  created_by     uuid REFERENCES users(id),
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  CHECK (ends_at > starts_at)
);
CREATE INDEX events_published_starts_idx
  ON events (starts_at) WHERE published_at IS NOT NULL;

CREATE TABLE channels (
  id        uuid PRIMARY KEY,
  kind      channel_kind NOT NULL,
  name      text NOT NULL,
  target    text NOT NULL,
  group_id  uuid REFERENCES parish_groups(id),
  is_active boolean NOT NULL DEFAULT true
);

CREATE TABLE outbox (
  id           bigserial PRIMARY KEY,
  event_id     uuid NOT NULL,
  kind         outbox_kind NOT NULL,
  payload      jsonb NOT NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  processed_at timestamptz
);
CREATE INDEX outbox_unprocessed_idx ON outbox (id) WHERE processed_at IS NULL;

CREATE TABLE broadcasts (
  id         uuid PRIMARY KEY,
  event_id   uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
  channel_id uuid NOT NULL REFERENCES channels(id),
  kind       outbox_kind NOT NULL,
  state      broadcast_state NOT NULL DEFAULT 'queued',
  attempt    integer NOT NULL DEFAULT 0,
  dedupe_key text NOT NULL UNIQUE,
  last_error text,
  sent_at    timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE broadcasts, outbox, channels, events, sessions, users, parish_groups, liturgical_seasons;
DROP TYPE user_role, broadcast_state, outbox_kind, channel_kind, event_rank, season_color;
DROP EXTENSION IF EXISTS citext;
