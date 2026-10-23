-- +goose NO TRANSACTION
-- +goose Up
-- On-site services (mechanic, electrician) are neither goods nor passengers.
ALTER TYPE service_category ADD VALUE IF NOT EXISTS 'SERVICE';

-- +goose Down
-- enum values cannot be dropped; left in place
SELECT 1;
