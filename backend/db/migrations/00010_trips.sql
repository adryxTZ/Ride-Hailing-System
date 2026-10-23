-- +goose Up
-- Trips: one explicit state machine, server-stamped phase timestamps, events as
-- the outbox/replay log. Stops are rows from day one (multi-stop later).
CREATE TYPE trip_status AS ENUM (
  'REQUESTED','OFFERED','ACCEPTED','EN_ROUTE_TO_PICKUP','ARRIVED_AT_PICKUP','LOADING','IN_TRANSIT',
  'ARRIVED_AT_DROPOFF','UNLOADING','AWAITING_PAYMENT','COMPLETED',
  'CANCELLED_BY_RIDER','CANCELLED_BY_DRIVER','NO_DRIVER_FOUND','EXPIRED'
);
CREATE TYPE payment_method AS ENUM ('CASH','MOBILE_MONEY','CARD','WALLET');

CREATE TABLE trips (
  id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  brand_id          uuid NOT NULL REFERENCES brands(id),
  code              text NOT NULL,                       -- short human code, per brand
  service_key       text NOT NULL,
  vehicle_class_key text NOT NULL,
  rider_id          uuid NOT NULL REFERENCES users(id),
  driver_id         uuid REFERENCES users(id),
  status            trip_status NOT NULL DEFAULT 'REQUESTED',
  lifecycle         jsonb NOT NULL,                      -- snapshot of the service's lifecycle profile
  attributes        jsonb NOT NULL DEFAULT '{}',
  quote_id          uuid REFERENCES quotes(id),
  quote_option      jsonb NOT NULL,                      -- the chosen option (fare, lines, source)
  pricing_source    text NOT NULL DEFAULT 'METERED',
  fixed_route_id    uuid,
  surge_multiplier  numeric(6,3) NOT NULL DEFAULT 1,
  estimated_fare    numeric(14,2) NOT NULL,
  final_fare        numeric(14,2),
  currency          char(3) NOT NULL,
  payment_method    payment_method NOT NULL DEFAULT 'CASH',
  route_distance_m  int,
  route_duration_s  int,
  notes             text,
  scheduled_for     timestamptz,
  requested_at      timestamptz NOT NULL DEFAULT now(),
  offered_at        timestamptz,
  accepted_at       timestamptz,
  en_route_at       timestamptz,
  arrived_pickup_at timestamptz,
  loading_started_at timestamptz,
  in_transit_at     timestamptz,
  arrived_dropoff_at timestamptz,
  unloading_started_at timestamptz,
  awaiting_payment_at timestamptz,
  completed_at      timestamptz,
  cancelled_at      timestamptz,
  cancelled_by      text,
  cancel_reason     text,
  cancel_note       text,
  dispatch_window_ends_at timestamptz,
  redispatch_count  int NOT NULL DEFAULT 0,
  seq               int NOT NULL DEFAULT 0,              -- last trip_events.seq
  version           int NOT NULL DEFAULT 1,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  UNIQUE (brand_id, code)
);
-- one active trip per rider, enforced by the database
CREATE UNIQUE INDEX trips_one_active_per_rider ON trips (rider_id)
  WHERE status IN ('REQUESTED','OFFERED','ACCEPTED','EN_ROUTE_TO_PICKUP','ARRIVED_AT_PICKUP','LOADING','IN_TRANSIT','ARRIVED_AT_DROPOFF','UNLOADING','AWAITING_PAYMENT');
CREATE INDEX trips_rider_idx ON trips (rider_id, created_at DESC);
CREATE INDEX trips_driver_idx ON trips (driver_id, created_at DESC);
CREATE INDEX trips_status_idx ON trips (brand_id, status) WHERE status IN ('REQUESTED','OFFERED');

CREATE TYPE stop_kind AS ENUM ('PICKUP','DROPOFF','WAYPOINT');
CREATE TABLE trip_stops (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  trip_id     uuid NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
  seq         int NOT NULL,
  kind        stop_kind NOT NULL,
  location    geography(Point, 4326) NOT NULL,
  address     text,
  contact     jsonb,
  arrived_at  timestamptz,
  departed_at timestamptz,
  UNIQUE (trip_id, seq)
);

CREATE TABLE trip_events (
  id         bigserial PRIMARY KEY,
  trip_id    uuid NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
  seq        int NOT NULL,
  type       text NOT NULL,                              -- e.g. trip.requested, trip.status, trip.cancelled
  actor_kind text NOT NULL,                              -- RIDER | DRIVER | SYSTEM | STAFF
  actor_id   uuid,
  payload    jsonb NOT NULL DEFAULT '{}',
  at         timestamptz NOT NULL DEFAULT now(),
  UNIQUE (trip_id, seq)
);

-- per-brand short-code sequence
CREATE SEQUENCE trip_code_seq;

-- +goose Down
DROP TABLE trip_events;
DROP TABLE trip_stops;
DROP TYPE stop_kind;
DROP TABLE trips;
DROP TYPE payment_method;
DROP TYPE trip_status;
DROP SEQUENCE trip_code_seq;
