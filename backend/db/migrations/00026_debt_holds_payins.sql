-- +goose NO TRANSACTION
-- +goose Up
-- Driver debt limits and account holds, and mobile-money pay-ins of that debt.
-- GATEWAY_CLEARING holds money collected at the payment gateway that has not
-- yet been swept to the bank.
ALTER TYPE account_kind ADD VALUE IF NOT EXISTS 'GATEWAY_CLEARING';

-- Limits are account rules versioned with the fee policy (not pinned per trip).
ALTER TABLE fee_policies
  ADD COLUMN IF NOT EXISTS debt_soft_limit numeric(14,2) NOT NULL DEFAULT 1000,
  ADD COLUMN IF NOT EXISTS debt_hard_limit numeric(14,2) NOT NULL DEFAULT 5000,
  ADD COLUMN IF NOT EXISTS debt_soft_grace_days int NOT NULL DEFAULT 7,
  ADD COLUMN IF NOT EXISTS payin_min_amount numeric(14,2) NOT NULL DEFAULT 1000;
ALTER TABLE fee_policies DROP CONSTRAINT IF EXISTS fee_policies_debt_limits_chk;
ALTER TABLE fee_policies ADD CONSTRAINT fee_policies_debt_limits_chk CHECK (
  payin_min_amount > 0 AND debt_soft_limit >= payin_min_amount
  AND debt_hard_limit >= debt_soft_limit AND debt_soft_grace_days >= 0);

-- A hold stops a driver going online. DEBT holds follow the debt rule;
-- MANUAL holds are placed and released by staff. At most one active per kind.
CREATE TABLE IF NOT EXISTS driver_holds (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  brand_id       uuid NOT NULL REFERENCES brands(id),
  driver_id      uuid NOT NULL REFERENCES users(id),
  kind           text NOT NULL CHECK (kind IN ('DEBT','MANUAL')),
  cause          text NOT NULL CHECK (cause IN ('HARD_LIMIT','SOFT_LIMIT_OVERDUE','STAFF')),
  reason         text,
  placed_by      uuid REFERENCES users(id),
  placed_at      timestamptz NOT NULL DEFAULT now(),
  released_at    timestamptz,
  released_by    uuid REFERENCES users(id),
  release_reason text
);
CREATE UNIQUE INDEX IF NOT EXISTS driver_holds_active_uq ON driver_holds (brand_id, driver_id, kind) WHERE released_at IS NULL;
CREATE INDEX IF NOT EXISTS driver_holds_active_idx ON driver_holds (brand_id, placed_at DESC) WHERE released_at IS NULL;

-- When a driver's debt first reached the soft limit; the row exists only while it stays there.
CREATE TABLE IF NOT EXISTS driver_debt_clock (
  brand_id        uuid NOT NULL REFERENCES brands(id),
  driver_id       uuid NOT NULL REFERENCES users(id),
  over_soft_since timestamptz NOT NULL,
  PRIMARY KEY (brand_id, driver_id)
);

-- Money requested through a payment gateway. Generic so fares and payouts can reuse it.
CREATE TABLE IF NOT EXISTS gateway_transactions (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  brand_id        uuid NOT NULL REFERENCES brands(id),
  purpose         text NOT NULL CHECK (purpose IN ('DRIVER_DEBT')),
  payer_id        uuid NOT NULL REFERENCES users(id),
  phone           text NOT NULL,
  amount          numeric(14,2) NOT NULL CHECK (amount > 0),
  currency        text NOT NULL,
  gateway         text NOT NULL,
  provider_ref    text,
  status          text NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING','COLLECTED','FAILED')),
  failure_code    text,
  idempotency_key text NOT NULL,
  checks          int NOT NULL DEFAULT 0,
  last_checked_at timestamptz,
  expires_at      timestamptz NOT NULL,
  created_at      timestamptz NOT NULL DEFAULT now(),
  completed_at    timestamptz,
  UNIQUE (brand_id, payer_id, idempotency_key)
);
CREATE UNIQUE INDEX IF NOT EXISTS gateway_tx_one_pending ON gateway_transactions (brand_id, payer_id, purpose) WHERE status = 'PENDING';
CREATE INDEX IF NOT EXISTS gateway_tx_pending_idx ON gateway_transactions (last_checked_at NULLS FIRST) WHERE status = 'PENDING';
CREATE INDEX IF NOT EXISTS gateway_tx_recent_idx ON gateway_transactions (brand_id, created_at DESC);

-- +goose Down
-- The GATEWAY_CLEARING enum value stays: Postgres cannot drop enum values.
DROP TABLE IF EXISTS gateway_transactions;
DROP TABLE IF EXISTS driver_debt_clock;
DROP TABLE IF EXISTS driver_holds;
ALTER TABLE fee_policies DROP CONSTRAINT IF EXISTS fee_policies_debt_limits_chk;
ALTER TABLE fee_policies DROP COLUMN IF EXISTS debt_soft_limit, DROP COLUMN IF EXISTS debt_hard_limit,
  DROP COLUMN IF EXISTS debt_soft_grace_days, DROP COLUMN IF EXISTS payin_min_amount;
