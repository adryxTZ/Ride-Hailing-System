-- +goose NO TRANSACTION
-- +goose Up
-- Phase B, slice 2: riders pay fares by mobile money, drivers cash out their
-- earnings, and every completed trip gets one immutable, numbered receipt.

-- Money a rider paid that no trip could take (e.g. it arrived after they paid
-- cash): owed back to the rider until Finance refunds it.
ALTER TYPE account_kind ADD VALUE IF NOT EXISTS 'RIDER_REFUNDS';

-- How a trip's fare was settled. NULL until the ride ends; DUE when it ends;
-- PENDING while a mobile-money prompt is open; FAILED when it did not go
-- through; COLLECTED (money at the gateway) or CASH_CONFIRMED (driver took cash).
ALTER TABLE trips
  ADD COLUMN IF NOT EXISTS payment_status text
    CHECK (payment_status IN ('DUE','PENDING','FAILED','COLLECTED','CASH_CONFIRMED')),
  ADD COLUMN IF NOT EXISTS payment_ref uuid,
  -- The rider said they will pay cash instead (the driver then confirms it).
  ADD COLUMN IF NOT EXISTS cash_chosen boolean NOT NULL DEFAULT false;
UPDATE trips SET payment_status = 'CASH_CONFIRMED' WHERE status = 'COMPLETED' AND payment_status IS NULL;
UPDATE trips SET payment_status = 'DUE' WHERE status = 'AWAITING_PAYMENT' AND payment_status IS NULL;
ALTER TABLE trips DROP CONSTRAINT IF EXISTS trips_completed_paid_chk;
-- A trip never reaches COMPLETED without a terminal payment state.
ALTER TABLE trips ADD CONSTRAINT trips_completed_paid_chk
  CHECK (status <> 'COMPLETED' OR payment_status IN ('COLLECTED','CASH_CONFIRMED'));

-- Gateway rows now also carry ride fares (payer = the rider, tied to a trip).
ALTER TABLE gateway_transactions DROP CONSTRAINT IF EXISTS gateway_transactions_purpose_check;
ALTER TABLE gateway_transactions ADD CONSTRAINT gateway_transactions_purpose_check
  CHECK (purpose IN ('DRIVER_DEBT','RIDE_FARE'));
ALTER TABLE gateway_transactions ADD COLUMN IF NOT EXISTS trip_id uuid REFERENCES trips(id);
CREATE INDEX IF NOT EXISTS gateway_tx_trip_idx ON gateway_transactions (trip_id, created_at DESC) WHERE trip_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS gateway_tx_phone_idx ON gateway_transactions (brand_id, phone, created_at DESC);

-- Payout rules live with the other account rules in the versioned fee policy.
ALTER TABLE fee_policies
  ADD COLUMN IF NOT EXISTS payout_min_amount numeric(14,2) NOT NULL DEFAULT 5000 CHECK (payout_min_amount > 0),
  ADD COLUMN IF NOT EXISTS payout_auto_approve boolean NOT NULL DEFAULT false;

-- A driver's request to be paid what the platform owes them, to their account phone.
-- REQUESTED waits for Finance (unless automatic); PENDING is with the gateway;
-- the ledger moves only on PAID.
CREATE TABLE IF NOT EXISTS driver_payouts (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  brand_id        uuid NOT NULL REFERENCES brands(id),
  driver_id       uuid NOT NULL REFERENCES users(id),
  amount          numeric(14,2) NOT NULL CHECK (amount > 0),
  currency        text NOT NULL,
  phone           text NOT NULL,
  gateway         text NOT NULL,
  status          text NOT NULL DEFAULT 'REQUESTED'
                    CHECK (status IN ('REQUESTED','PENDING','PAID','FAILED','REJECTED','CANCELLED')),
  failure_code    text,
  provider_ref    text,
  idempotency_key text NOT NULL,
  decided_by      uuid REFERENCES users(id),
  decided_at      timestamptz,
  decision_reason text,
  sent_at         timestamptz,
  checks          int NOT NULL DEFAULT 0,
  last_checked_at timestamptz,
  created_at      timestamptz NOT NULL DEFAULT now(),
  completed_at    timestamptz,
  UNIQUE (brand_id, driver_id, idempotency_key)
);
CREATE UNIQUE INDEX IF NOT EXISTS driver_payouts_one_open ON driver_payouts (brand_id, driver_id) WHERE status IN ('REQUESTED','PENDING');
CREATE INDEX IF NOT EXISTS driver_payouts_pending_idx ON driver_payouts (last_checked_at NULLS FIRST) WHERE status = 'PENDING';
CREATE INDEX IF NOT EXISTS driver_payouts_recent_idx ON driver_payouts (brand_id, created_at DESC);
CREATE INDEX IF NOT EXISTS driver_payouts_driver_idx ON driver_payouts (driver_id, created_at DESC);

-- Receipt numbers are gapless per brand and year: the counter row is taken
-- inside the transaction that completes the trip.
CREATE TABLE IF NOT EXISTS receipt_counters (
  brand_id uuid NOT NULL REFERENCES brands(id),
  year     int  NOT NULL,
  last     int  NOT NULL DEFAULT 0,
  PRIMARY KEY (brand_id, year)
);

-- One receipt per completed trip; never changed or deleted once issued.
CREATE TABLE IF NOT EXISTS receipts (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  brand_id    uuid NOT NULL REFERENCES brands(id),
  trip_id     uuid NOT NULL UNIQUE REFERENCES trips(id),
  number      text NOT NULL,
  issued_at   timestamptz NOT NULL DEFAULT now(),
  currency    text NOT NULL,
  total       numeric(14,2) NOT NULL,
  lines       jsonb NOT NULL,
  taxes       jsonb NOT NULL,
  payment     jsonb NOT NULL,
  issuer      jsonb NOT NULL,
  trip_info   jsonb NOT NULL,
  driver_info jsonb NOT NULL,
  rider_id    uuid NOT NULL REFERENCES users(id),
  driver_id   uuid NOT NULL REFERENCES users(id),
  UNIQUE (brand_id, number)
);

-- Finance approves driver cash-outs (new permission on the default Finance role).
UPDATE roles SET permissions = array_append(permissions, 'payouts.approve')
  WHERE name = 'Finance' AND system = false AND NOT ('payouts.approve' = ANY(permissions));

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION receipts_immutable() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'receipts are immutable (receipt %)', OLD.number;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
DROP TRIGGER IF EXISTS receipts_no_change ON receipts;
CREATE TRIGGER receipts_no_change BEFORE UPDATE OR DELETE ON receipts
  FOR EACH ROW EXECUTE FUNCTION receipts_immutable();

-- +goose Down
-- The RIDER_REFUNDS enum value stays: Postgres cannot drop enum values.
UPDATE roles SET permissions = array_remove(permissions, 'payouts.approve');
DROP TRIGGER IF EXISTS receipts_no_change ON receipts;
DROP FUNCTION IF EXISTS receipts_immutable();
DROP TABLE IF EXISTS receipts;
DROP TABLE IF EXISTS receipt_counters;
DROP TABLE IF EXISTS driver_payouts;
ALTER TABLE fee_policies DROP COLUMN IF EXISTS payout_min_amount, DROP COLUMN IF EXISTS payout_auto_approve;
DROP INDEX IF EXISTS gateway_tx_phone_idx;
DROP INDEX IF EXISTS gateway_tx_trip_idx;
ALTER TABLE gateway_transactions DROP COLUMN IF EXISTS trip_id;
DELETE FROM gateway_transactions WHERE purpose <> 'DRIVER_DEBT';
ALTER TABLE gateway_transactions DROP CONSTRAINT IF EXISTS gateway_transactions_purpose_check;
ALTER TABLE gateway_transactions ADD CONSTRAINT gateway_transactions_purpose_check CHECK (purpose IN ('DRIVER_DEBT'));
ALTER TABLE trips DROP CONSTRAINT IF EXISTS trips_completed_paid_chk;
ALTER TABLE trips DROP COLUMN IF EXISTS cash_chosen, DROP COLUMN IF EXISTS payment_ref, DROP COLUMN IF EXISTS payment_status;
