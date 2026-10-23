-- +goose NO TRANSACTION
-- +goose Up
-- Rides booked ahead wait as SCHEDULED until the dispatcher starts matching
-- shortly before the pickup time (trips.scheduled_for already exists).
ALTER TYPE trip_status ADD VALUE IF NOT EXISTS 'SCHEDULED';
CREATE INDEX IF NOT EXISTS trips_scheduled_idx ON trips (brand_id, scheduled_for) WHERE status = 'SCHEDULED';

-- +goose Down
DROP INDEX IF EXISTS trips_scheduled_idx;
