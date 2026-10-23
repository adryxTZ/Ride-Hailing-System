-- +goose NO TRANSACTION
-- +goose Up
-- Late cancellations and no-shows cost the rider the ride type's cancellation
-- fee (most of it to the driver). Cash riders owe it until their next trip:
-- RIDER_DUES holds what each rider owes.
ALTER TYPE account_kind ADD VALUE IF NOT EXISTS 'RIDER_DUES';
ALTER TABLE tariffs ADD COLUMN IF NOT EXISTS cancellation_fee numeric(14,2) NOT NULL DEFAULT 0;
ALTER TABLE fee_policies
  ADD COLUMN IF NOT EXISTS cancel_free_s int NOT NULL DEFAULT 120,
  ADD COLUMN IF NOT EXISTS no_show_after_s int NOT NULL DEFAULT 300,
  ADD COLUMN IF NOT EXISTS cancel_driver_share numeric(6,4) NOT NULL DEFAULT 0.8000;
ALTER TABLE trips ADD COLUMN IF NOT EXISTS cancellation_fee numeric(14,2);

-- +goose Down
ALTER TABLE trips DROP COLUMN IF EXISTS cancellation_fee;
ALTER TABLE fee_policies DROP COLUMN IF EXISTS cancel_free_s, DROP COLUMN IF EXISTS no_show_after_s, DROP COLUMN IF EXISTS cancel_driver_share;
ALTER TABLE tariffs DROP COLUMN IF EXISTS cancellation_fee;
