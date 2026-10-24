-- +goose Up
-- Staff trip actions (dashboard leftovers): a fare set by hand before payment
-- (recorded on the trip as a MANUAL_PRICE line) and refunds after completion.
-- A refund is owed to the rider (RIDER_REFUNDS) until Finance pays it; it is
-- borne by the platform or recovered from the driver.
CREATE TABLE trip_refunds (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  brand_id    uuid NOT NULL REFERENCES brands(id),
  trip_id     uuid NOT NULL REFERENCES trips(id),
  rider_id    uuid NOT NULL REFERENCES users(id),
  amount      numeric(14,2) NOT NULL CHECK (amount > 0),
  currency    char(3) NOT NULL,
  borne_by    text NOT NULL CHECK (borne_by IN ('PLATFORM','DRIVER')),
  reason      text NOT NULL,
  created_by  uuid NOT NULL REFERENCES users(id),
  created_at  timestamptz NOT NULL DEFAULT now(),
  status      text NOT NULL DEFAULT 'OWED' CHECK (status IN ('OWED','PAID')),
  paid_at     timestamptz,
  paid_ref    text
);
CREATE INDEX trip_refunds_trip_idx ON trip_refunds (trip_id);
CREATE INDEX trip_refunds_owed_idx ON trip_refunds (brand_id, status, created_at);

-- Who may set a price and who may refund (Operations gets both; Super Admin has all).
UPDATE roles SET permissions = permissions || ARRAY['trips.price','trips.refund']
  WHERE name = 'Operations' AND system = false AND NOT ('trips.refund' = ANY(permissions));

-- +goose Down
UPDATE roles SET permissions = array_remove(array_remove(permissions, 'trips.price'), 'trips.refund');
DROP TABLE IF EXISTS trip_refunds;
