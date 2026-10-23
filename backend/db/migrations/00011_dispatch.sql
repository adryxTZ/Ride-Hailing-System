-- +goose Up
-- Dispatch: every offer made to a driver for a trip, and how it ended.
-- One open offer per trip and per driver at a time (partial unique indexes),
-- so a race between two dispatch ticks cannot double-offer.

CREATE TYPE offer_outcome AS ENUM ('OFFERED', 'ACCEPTED', 'DECLINED', 'TIMED_OUT', 'CANCELLED');
CREATE TYPE offer_source AS ENUM ('AUTO', 'MANUAL');

CREATE TABLE trip_offers (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  trip_id      uuid NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
  driver_id    uuid NOT NULL REFERENCES users(id),
  wave         int  NOT NULL,                 -- 1st, 2nd … driver tried for this trip
  distance_m   int  NOT NULL,                 -- driver → pickup when offered
  eta_s        int  NOT NULL,                 -- estimated pickup time when offered
  source       offer_source  NOT NULL DEFAULT 'AUTO',
  outcome      offer_outcome NOT NULL DEFAULT 'OFFERED',
  offered_at   timestamptz NOT NULL DEFAULT now(),
  expires_at   timestamptz NOT NULL,
  responded_at timestamptz
);

CREATE UNIQUE INDEX trip_offers_one_open_per_trip   ON trip_offers (trip_id)   WHERE outcome = 'OFFERED';
CREATE UNIQUE INDEX trip_offers_one_open_per_driver ON trip_offers (driver_id) WHERE outcome = 'OFFERED';
CREATE INDEX trip_offers_trip_idx    ON trip_offers (trip_id, wave);
CREATE INDEX trip_offers_expiry_idx  ON trip_offers (expires_at) WHERE outcome = 'OFFERED';
CREATE INDEX trips_driver_active_idx ON trips (driver_id) WHERE status IN ('ACCEPTED','EN_ROUTE_TO_PICKUP','ARRIVED_AT_PICKUP','LOADING','IN_TRANSIT','ARRIVED_AT_DROPOFF','UNLOADING','AWAITING_PAYMENT');

-- +goose Down
DROP TABLE trip_offers;
DROP TYPE offer_source;
DROP TYPE offer_outcome;
DROP INDEX IF EXISTS trips_driver_active_idx;
