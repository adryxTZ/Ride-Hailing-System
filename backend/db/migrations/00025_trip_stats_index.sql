-- +goose Up
-- The overview stats and the admin trip list filter trips by brand and request time.
CREATE INDEX IF NOT EXISTS trips_brand_requested_idx ON trips (brand_id, requested_at DESC);

-- +goose Down
DROP INDEX IF EXISTS trips_brand_requested_idx;
