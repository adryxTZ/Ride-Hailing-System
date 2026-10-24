-- +goose Up
-- Security review 2026-10-23 of S3/S4 and trip money:
-- privacy requests carry the person's country (staff limited to a country see
-- and handle only theirs); a refund can carry the client's idempotency key.
ALTER TABLE privacy_requests ADD COLUMN country_code char(2);
UPDATE privacy_requests r SET country_code = coalesce(
  (SELECT p.country_code FROM driver_profiles p WHERE p.user_id = r.user_id),
  (SELECT t.country_code FROM trips t WHERE t.rider_id = r.user_id AND t.country_code IS NOT NULL ORDER BY t.requested_at DESC LIMIT 1),
  (SELECT b.country FROM brands b WHERE b.id = r.brand_id));
CREATE INDEX privacy_requests_country_idx ON privacy_requests (brand_id, country_code, status, due_at);

ALTER TABLE trip_refunds ADD COLUMN idempotency_key text;
CREATE UNIQUE INDEX trip_refunds_idem_uq ON trip_refunds (trip_id, idempotency_key) WHERE idempotency_key IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS trip_refunds_idem_uq;
ALTER TABLE trip_refunds DROP COLUMN IF EXISTS idempotency_key;
DROP INDEX IF EXISTS privacy_requests_country_idx;
ALTER TABLE privacy_requests DROP COLUMN IF EXISTS country_code;
