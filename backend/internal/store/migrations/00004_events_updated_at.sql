-- +goose Up
-- ICS DTSTAMP, SEQUENCE and the feed ETag are all derived from
-- events.updated_at, so it must advance on every write or subscribed phones
-- keep receiving 304 and never see a correction.
-- +goose StatementBegin
CREATE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
  NEW.updated_at := now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER events_set_updated_at
  BEFORE UPDATE ON events
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TRIGGER events_set_updated_at ON events;
DROP FUNCTION set_updated_at();
