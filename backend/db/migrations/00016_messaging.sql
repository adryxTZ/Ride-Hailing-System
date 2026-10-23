-- +goose Up
-- In-trip chat between rider and driver, and the two ratings after a trip.
CREATE TABLE trip_messages (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  brand_id    uuid NOT NULL REFERENCES brands(id),
  trip_id     uuid NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
  sender_id   uuid NOT NULL REFERENCES users(id),
  sender_role text NOT NULL CHECK (sender_role IN ('RIDER', 'DRIVER')),
  body        text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 500),
  quick_reply text,
  created_at  timestamptz NOT NULL DEFAULT now(),
  read_at     timestamptz
);
CREATE INDEX trip_messages_trip_idx ON trip_messages (trip_id, created_at);

CREATE TABLE trip_ratings (
  trip_id    uuid NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
  rater_role text NOT NULL CHECK (rater_role IN ('RIDER', 'DRIVER')),
  rater_id   uuid NOT NULL REFERENCES users(id),
  ratee_id   uuid NOT NULL REFERENCES users(id),
  score      smallint NOT NULL CHECK (score BETWEEN 1 AND 5),
  aspects    text[] NOT NULL DEFAULT '{}',
  comment    text CHECK (char_length(comment) <= 500),
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (trip_id, rater_role)
);
CREATE INDEX trip_ratings_ratee_idx ON trip_ratings (ratee_id);

-- +goose Down
DROP TABLE trip_ratings;
DROP TABLE trip_messages;
