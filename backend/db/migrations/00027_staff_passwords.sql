-- +goose Up
-- Dashboard staff sign in with email and password (riders and drivers keep phone + code).
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS password_hash text,
  ADD COLUMN IF NOT EXISTS failed_logins int NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS locked_until timestamptz;
CREATE UNIQUE INDEX IF NOT EXISTS users_brand_email_uq ON users (brand_id, email) WHERE email IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS users_brand_email_uq;
ALTER TABLE users DROP COLUMN IF EXISTS password_hash, DROP COLUMN IF EXISTS failed_logins, DROP COLUMN IF EXISTS locked_until;
