-- +goose Up
-- Fares are quoted in round amounts (e.g. to the nearest 50 TZS); 0 = no rounding.
ALTER TABLE fee_policies ADD COLUMN fare_rounding numeric(14,2) NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE fee_policies DROP COLUMN fare_rounding;
