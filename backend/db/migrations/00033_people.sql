-- +goose Up
-- Rider and driver pages for staff: document file sizes, staff notes on a
-- person, and the indexes the per-person figures read through.

-- Measured from the stored file when it is attached (older files stay NULL).
ALTER TABLE documents ADD COLUMN IF NOT EXISTS size_bytes bigint CHECK (size_bytes >= 0);

-- Notes staff keep on a rider or driver (support history), append-only.
CREATE TABLE IF NOT EXISTS user_notes (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  brand_id   uuid NOT NULL REFERENCES brands(id),
  user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  author_id  uuid NOT NULL REFERENCES users(id),
  body       text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 2000),
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS user_notes_user_idx ON user_notes (user_id, created_at DESC);

-- Per-person reads: offers a driver answered, ratings a person gave, SOS a person raised.
CREATE INDEX IF NOT EXISTS trip_offers_driver_idx ON trip_offers (driver_id, offered_at DESC);
CREATE INDEX IF NOT EXISTS trip_ratings_rater_idx ON trip_ratings (rater_id);
CREATE INDEX IF NOT EXISTS sos_alerts_user_idx ON sos_alerts (user_id, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS sos_alerts_user_idx;
DROP INDEX IF EXISTS trip_ratings_rater_idx;
DROP INDEX IF EXISTS trip_offers_driver_idx;
DROP TABLE IF EXISTS user_notes;
ALTER TABLE documents DROP COLUMN IF EXISTS size_bytes;
