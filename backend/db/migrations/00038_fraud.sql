-- +goose Up
-- Fraud v1 (spec 2026-10-23-compliance-trust.md, S3; planned as 00033, that
-- number was taken): app installs seen at sign-in, and the flags three
-- detectors raise (fake GPS, self-booking, cancellation abuse) for staff to
-- review. Nothing here punishes anyone; staff act through suspend, hold, block.

-- Which app installs a person signed in from (install ids are random per install).
CREATE TABLE user_installs (
  user_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  install_id    uuid NOT NULL,
  app           text NOT NULL,
  first_seen_at timestamptz NOT NULL DEFAULT now(),
  last_seen_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, install_id)
);
CREATE INDEX user_installs_install_idx ON user_installs (install_id, last_seen_at DESC);

ALTER TABLE devices ADD COLUMN install_id uuid;

CREATE TABLE fraud_flags (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  brand_id     uuid NOT NULL REFERENCES brands(id),
  country_code char(2),
  rule         text NOT NULL CHECK (rule IN ('GPS_SPOOF','SELF_BOOKING','DRIVER_CANCEL_RATE','DRIVER_NO_SHOW_RATE','RIDER_CANCEL_RATE')),
  severity     text NOT NULL CHECK (severity IN ('LOW','MEDIUM','HIGH')),
  subject_kind text NOT NULL CHECK (subject_kind IN ('DRIVER','RIDER')),
  subject_id   uuid NOT NULL REFERENCES users(id),
  trip_id      uuid REFERENCES trips(id),
  evidence     jsonb NOT NULL DEFAULT '{}',
  hits         int NOT NULL DEFAULT 1,
  dedupe_key   text NOT NULL,
  status       text NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN','DISMISSED','CONFIRMED')),
  opened_at    timestamptz NOT NULL DEFAULT now(),
  last_seen_at timestamptz NOT NULL DEFAULT now(),
  reviewed_by  uuid REFERENCES users(id),
  reviewed_at  timestamptz,
  review_note  text,
  UNIQUE (brand_id, dedupe_key),
  CHECK ((status = 'OPEN') = (reviewed_at IS NULL))
);
CREATE INDEX fraud_flags_queue_idx ON fraud_flags (brand_id, status, opened_at DESC);
CREATE INDEX fraud_flags_subject_idx ON fraud_flags (subject_kind, subject_id);

-- One cancellation scan per country and local day (the job runs hourly).
CREATE TABLE fraud_scan_runs (
  brand_id     uuid NOT NULL REFERENCES brands(id),
  country_code char(2) NOT NULL,
  day          date NOT NULL,
  flags        int NOT NULL DEFAULT 0,
  ran_at       timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (brand_id, country_code, day)
);

-- A reviewed flag never changes again.
-- +goose StatementBegin
CREATE FUNCTION fraud_flags_reviewed_frozen() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    RAISE EXCEPTION 'fraud flags are never deleted' USING ERRCODE = 'check_violation';
  END IF;
  IF OLD.status <> 'OPEN' THEN
    RAISE EXCEPTION 'a reviewed fraud flag cannot change' USING ERRCODE = 'check_violation';
  END IF;
  RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER fraud_flags_reviewed_frozen BEFORE UPDATE OR DELETE ON fraud_flags FOR EACH ROW EXECUTE FUNCTION fraud_flags_reviewed_frozen();

-- Staff who review fraud flags (new permission on Operations).
UPDATE roles SET permissions = array_append(permissions, 'fraud.review')
  WHERE name = 'Operations' AND system = false AND NOT ('fraud.review' = ANY(permissions));

-- +goose Down
UPDATE roles SET permissions = array_remove(permissions, 'fraud.review');
DROP TABLE IF EXISTS fraud_flags;
DROP TABLE IF EXISTS fraud_scan_runs;
DROP FUNCTION IF EXISTS fraud_flags_reviewed_frozen();
ALTER TABLE devices DROP COLUMN IF EXISTS install_id;
DROP TABLE IF EXISTS user_installs;
