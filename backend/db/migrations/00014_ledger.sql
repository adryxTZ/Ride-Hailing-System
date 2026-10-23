-- +goose Up
-- Double-entry ledger. Every money movement is one transaction with balanced
-- entries (sum of debits = sum of credits). Entries are append-only and chained
-- by hash so any edit or deletion is detectable (`nexa verify-ledger`).

CREATE TYPE account_kind AS ENUM (
  'DRIVER_WALLET',      -- per driver: positive = platform owes the driver, negative = driver owes the platform
  'PLATFORM_REVENUE',   -- commission + booking fee, net of VAT and levy
  'VAT_PAYABLE',        -- owed to the tax authority (TRA)
  'LEVY_PAYABLE',       -- owed to the regulator (LATRA)
  'CASH_HELD_BY_DRIVERS'-- cash riders paid that drivers still hold
);

CREATE TABLE ledger_accounts (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  brand_id   uuid NOT NULL REFERENCES brands(id),
  kind       account_kind NOT NULL,
  owner_id   uuid,                        -- the driver for DRIVER_WALLET, NULL for platform accounts
  currency   text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE NULLS NOT DISTINCT (brand_id, kind, owner_id, currency)
);

CREATE TABLE ledger_transactions (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  brand_id    uuid NOT NULL REFERENCES brands(id),
  kind        text NOT NULL,              -- trip.completed, settlement, payout, adjustment
  reference   text NOT NULL,              -- e.g. the trip id; (kind, reference) is unique → posting is idempotent
  memo        text,
  created_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (brand_id, kind, reference)
);

CREATE TABLE ledger_entries (
  seq            bigserial PRIMARY KEY,   -- global order of the hash chain
  transaction_id uuid NOT NULL REFERENCES ledger_transactions(id),
  account_id     uuid NOT NULL REFERENCES ledger_accounts(id),
  amount         numeric(14,2) NOT NULL,  -- + debit / − credit; a transaction's entries sum to 0
  prev_hash      text NOT NULL,
  hash           text NOT NULL,
  created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ledger_entries_account_idx ON ledger_entries (account_id, seq DESC);

-- Append-only: no updates or deletes, ever.
-- +goose StatementBegin
CREATE FUNCTION ledger_entries_append_only() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'ledger_entries is append-only';
END $$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER ledger_entries_no_change BEFORE UPDATE OR DELETE ON ledger_entries
  FOR EACH ROW EXECUTE FUNCTION ledger_entries_append_only();

-- +goose Down
DROP TRIGGER ledger_entries_no_change ON ledger_entries;
DROP FUNCTION ledger_entries_append_only();
DROP TABLE ledger_entries;
DROP TABLE ledger_transactions;
DROP TABLE ledger_accounts;
DROP TYPE account_kind;
