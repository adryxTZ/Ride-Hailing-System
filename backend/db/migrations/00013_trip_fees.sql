-- +goose Up
-- The fare split for the trip (money.Breakdown): estimated at booking, final at completion.
ALTER TABLE trips ADD COLUMN fees jsonb;

-- +goose Down
ALTER TABLE trips DROP COLUMN fees;
