-- +goose Up
-- Trip queue (forward dispatch): a driver finishing a trip may accept the next
-- one. The next trip is ACCEPTED with the driver set and points at the trip it
-- waits for (queued_after) until that trip ends; then it becomes the driver's
-- current trip (queued_after cleared, queue_promoted_at stamped).
ALTER TABLE trips
  ADD COLUMN queued_after      uuid REFERENCES trips(id),
  ADD COLUMN queued_pickup_by  timestamptz, -- promised pickup time shown to the waiting rider
  ADD COLUMN queue_promoted_at timestamptz; -- when it became current (the free-cancel window restarts)

-- Offers made to a finishing driver are marked so the app labels them "Next trip".
ALTER TABLE trip_offers ADD COLUMN queued boolean NOT NULL DEFAULT false;

-- At most one queued trip per driver, and at most one current trip per driver.
-- Before this a second current trip was only prevented by dispatch offering
-- idle drivers; the index makes it a database rule now that drivers on a trip
-- receive offers.
CREATE UNIQUE INDEX trips_one_queued_per_driver ON trips (driver_id)
  WHERE queued_after IS NOT NULL AND status = 'ACCEPTED';
CREATE UNIQUE INDEX trips_one_current_per_driver ON trips (driver_id)
  WHERE queued_after IS NULL AND status IN ('ACCEPTED','EN_ROUTE_TO_PICKUP','ARRIVED_AT_PICKUP','IN_TRANSIT','ARRIVED_AT_DROPOFF','AWAITING_PAYMENT');

-- Per country: whether finishing drivers get the next trip, and which statuses
-- count as finishing. No row = the defaults below (on). Changes are audited.
CREATE TABLE dispatch_settings (
  brand_id           uuid NOT NULL REFERENCES brands(id),
  country_code       char(2) NOT NULL,
  trip_queue_enabled boolean NOT NULL DEFAULT true,
  finishing_statuses text[] NOT NULL DEFAULT '{ARRIVED_AT_DROPOFF,AWAITING_PAYMENT}'
    CHECK (cardinality(finishing_statuses) > 0
      AND finishing_statuses <@ ARRAY['IN_TRANSIT','ARRIVED_AT_DROPOFF','AWAITING_PAYMENT']::text[]),
  updated_by         uuid REFERENCES users(id),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (brand_id, country_code)
);
INSERT INTO config_versions (scope) VALUES ('dispatch') ON CONFLICT DO NOTHING;

-- +goose Down
DELETE FROM config_versions WHERE scope = 'dispatch';
DROP TABLE IF EXISTS dispatch_settings;
DROP INDEX IF EXISTS trips_one_current_per_driver;
DROP INDEX IF EXISTS trips_one_queued_per_driver;
ALTER TABLE trip_offers DROP COLUMN IF EXISTS queued;
ALTER TABLE trips
  DROP COLUMN IF EXISTS queue_promoted_at,
  DROP COLUMN IF EXISTS queued_pickup_by,
  DROP COLUMN IF EXISTS queued_after;
