-- +goose Up
-- Who receives each kind of extra charge by default, per country (owner
-- 2026-10-24: tolls and parking are the driver's expenses, so they go to the
-- driver — configurable because operators differ). A charge saved without a
-- recipient takes its category's default; the admin pre-fills it too. Rows
-- exist only where staff changed the built-in default.
CREATE TABLE charge_category_defaults (
  brand_id     uuid NOT NULL REFERENCES brands(id),
  country_code char(2) NOT NULL,
  category     text NOT NULL CHECK (category IN ('TOLL','PARKING','LEVY','AIRPORT_FEE','ENTRY_FEE','NIGHT','OTHER')),
  recipient    text NOT NULL CHECK (recipient IN ('FARE','PLATFORM','DRIVER','THIRD_PARTY')),
  payee_id     uuid REFERENCES charge_payees(id),
  updated_by   uuid REFERENCES users(id),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (brand_id, country_code, category),
  CHECK ((recipient = 'THIRD_PARTY') = (payee_id IS NOT NULL))
);

-- +goose Down
DROP TABLE IF EXISTS charge_category_defaults;
