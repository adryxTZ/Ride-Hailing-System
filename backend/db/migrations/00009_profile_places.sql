-- +goose Up
ALTER TABLE users ADD COLUMN photo_key text;          -- object-storage key of the profile photo
ALTER TABLE users ADD COLUMN email_verified_at timestamptz;

CREATE TYPE place_kind AS ENUM ('HOME', 'WORK', 'OTHER');
CREATE TABLE saved_places (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kind        place_kind NOT NULL DEFAULT 'OTHER',
  label       text NOT NULL,
  address     text NOT NULL,
  location    geography(Point, 4326) NOT NULL,
  use_count   int NOT NULL DEFAULT 0,
  last_used_at timestamptz,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX saved_places_one_home ON saved_places (user_id) WHERE kind = 'HOME';
CREATE UNIQUE INDEX saved_places_one_work ON saved_places (user_id) WHERE kind = 'WORK';
CREATE INDEX saved_places_user_idx ON saved_places (user_id, kind);

-- +goose Down
DROP TABLE saved_places;
DROP TYPE place_kind;
ALTER TABLE users DROP COLUMN email_verified_at;
ALTER TABLE users DROP COLUMN photo_key;
