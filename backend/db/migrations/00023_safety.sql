-- +goose Up
-- Safety: people a rider trusts (alerted on SOS), public share links for a live
-- trip, and SOS alerts staff follow up.
CREATE TABLE trusted_contacts (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name       text NOT NULL,
  phone      text NOT NULL,                        -- E.164
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, phone)
);

ALTER TABLE trips ADD COLUMN share_token text UNIQUE;

CREATE TABLE sos_alerts (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  brand_id    uuid NOT NULL REFERENCES brands(id),
  trip_id     uuid NOT NULL REFERENCES trips(id),
  user_id     uuid NOT NULL REFERENCES users(id),
  lat         double precision,
  lng         double precision,
  contacts_alerted int NOT NULL DEFAULT 0,
  created_at  timestamptz NOT NULL DEFAULT now(),
  resolved_at timestamptz,
  resolved_by uuid REFERENCES users(id),
  resolution  text
);
CREATE INDEX sos_alerts_open_idx ON sos_alerts (brand_id, created_at DESC) WHERE resolved_at IS NULL;

-- +goose Down
DROP TABLE sos_alerts;
ALTER TABLE trips DROP COLUMN share_token;
DROP TABLE trusted_contacts;
