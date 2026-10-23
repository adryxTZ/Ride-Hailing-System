-- +goose Up
-- Pickup waiting fee: after the free minutes, waiting is charged per minute and
-- added to the fare as an adjustment line; the fee policy may cap it as a share of the fare.
ALTER TABLE trips ADD COLUMN adjustments jsonb NOT NULL DEFAULT '[]';
ALTER TABLE fee_policies ADD COLUMN waiting_cap_rate numeric(6,4) NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE fee_policies DROP COLUMN waiting_cap_rate;
ALTER TABLE trips DROP COLUMN adjustments;
