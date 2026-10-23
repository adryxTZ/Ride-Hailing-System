-- +goose Up
-- Intermediate stops between pickup and drop-off, in order: [{"lat":..,"lng":..}, …].
ALTER TABLE quotes ADD COLUMN waypoints jsonb NOT NULL DEFAULT '[]';

-- +goose Down
ALTER TABLE quotes DROP COLUMN waypoints;
