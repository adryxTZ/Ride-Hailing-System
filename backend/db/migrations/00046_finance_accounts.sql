-- +goose Up
-- Finance accounts (2026-10-24): every party paid from fares has a ledger
-- account the Finance › Accounts page reports on, and staff record payments
-- against what the platform owes (VAT, levy, a charge payee, rider refunds).
-- A payment is one balanced ledger transaction: the payable is debited and
-- PLATFORM_BANK, the platform's own bank or cash, is credited.
ALTER TYPE account_kind ADD VALUE IF NOT EXISTS 'PLATFORM_BANK';

CREATE TABLE finance_settlements (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  brand_id        uuid NOT NULL REFERENCES brands(id),
  account_kind    text NOT NULL CHECK (account_kind IN ('VAT_PAYABLE','LEVY_PAYABLE','CHARGES_PAYABLE','RIDER_REFUNDS')),
  payee_id        uuid REFERENCES charge_payees(id),  -- CHARGES_PAYABLE only
  rider_id        uuid REFERENCES users(id),          -- RIDER_REFUNDS paid to one rider
  currency        char(3) NOT NULL,
  amount          numeric(14,2) NOT NULL CHECK (amount > 0),
  paid_on         date NOT NULL,
  method          text NOT NULL CHECK (method IN ('BANK_TRANSFER','MOBILE_MONEY','CASH','CHEQUE','OTHER')),
  reference       text NOT NULL CHECK (char_length(reference) BETWEEN 1 AND 120),
  reason          text NOT NULL CHECK (char_length(reason) BETWEEN 1 AND 1000),
  idempotency_key text NOT NULL CHECK (char_length(idempotency_key) BETWEEN 1 AND 200),
  created_by      uuid NOT NULL REFERENCES users(id),
  created_at      timestamptz NOT NULL DEFAULT now(),
  CHECK ((account_kind = 'CHARGES_PAYABLE') = (payee_id IS NOT NULL)),
  CHECK (rider_id IS NULL OR account_kind = 'RIDER_REFUNDS')
);
-- A retried request (same key) finds its payment instead of paying twice.
CREATE UNIQUE INDEX finance_settlements_idem_uq ON finance_settlements (brand_id, idempotency_key);
CREATE INDEX finance_settlements_account_idx ON finance_settlements (brand_id, account_kind, currency, created_at DESC);

-- The accounts list reads the period's trip.completed postings for the fare base.
CREATE INDEX IF NOT EXISTS ledger_transactions_kind_time_idx ON ledger_transactions (brand_id, kind, created_at);
-- ... and the entries of those transactions (fare shares per account).
CREATE INDEX IF NOT EXISTS ledger_entries_tx_idx ON ledger_entries (transaction_id);
-- Rider refunds are paid oldest first per currency.
CREATE INDEX IF NOT EXISTS trip_refunds_owed_cur_idx ON trip_refunds (brand_id, currency, created_at) WHERE status = 'OWED';

-- Recording payments is its own permission; the Finance role gets it (Super Admin has all).
UPDATE roles SET permissions = permissions || ARRAY['finance.settle']
  WHERE name = 'Finance' AND system = false AND NOT ('finance.settle' = ANY(permissions));

-- +goose Down
-- The PLATFORM_BANK enum value stays: Postgres cannot drop enum values, and
-- ledger entries posted to it are append-only.
UPDATE roles SET permissions = array_remove(permissions, 'finance.settle');
DROP INDEX IF EXISTS trip_refunds_owed_cur_idx;
DROP INDEX IF EXISTS ledger_entries_tx_idx;
DROP INDEX IF EXISTS ledger_transactions_kind_time_idx;
DROP TABLE IF EXISTS finance_settlements;
