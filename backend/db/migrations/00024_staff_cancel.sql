-- +goose NO TRANSACTION
-- +goose Up
-- Operations staff can end a trip (e.g. a stuck ride or a safety case).
ALTER TYPE trip_status ADD VALUE IF NOT EXISTS 'CANCELLED_BY_STAFF';

-- +goose Down
SELECT 1;
