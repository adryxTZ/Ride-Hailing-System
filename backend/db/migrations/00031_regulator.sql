-- +goose Up
-- Phase C, slice 1: the transport regulator (LATRA in Tanzania). Drivers and
-- vehicles are registered on approval, completed trips and the daily report
-- are submitted, all through one outbox retried on a fixed ladder.

-- What a country accepts as a driving licence (loaded from the brand file).
ALTER TABLE countries
  ADD COLUMN IF NOT EXISTS licence_pattern text,
  ADD COLUMN IF NOT EXISTS licence_classes text[] NOT NULL DEFAULT '{}';
ALTER TABLE driver_profiles ADD COLUMN IF NOT EXISTS licence_class text;

-- Per country: whether submissions are sent, and whether unregistered drivers
-- may go online (fail-closed). Changes are audited.
CREATE TABLE IF NOT EXISTS regulator_settings (
  brand_id     uuid NOT NULL REFERENCES brands(id),
  country_code char(2) NOT NULL,
  enabled      boolean NOT NULL DEFAULT false,
  fail_closed  boolean NOT NULL DEFAULT false,
  updated_by   uuid REFERENCES users(id),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (brand_id, country_code)
);

-- The outbox: one row per thing the regulator must hear about, kept forever.
CREATE TABLE IF NOT EXISTS regulator_submissions (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  brand_id        uuid NOT NULL REFERENCES brands(id),
  country_code    char(2) NOT NULL,
  regulator       text NOT NULL,
  kind            text NOT NULL CHECK (kind IN ('DRIVER','VEHICLE','TRIP','DAILY_REPORT')),
  subject_key     text NOT NULL, -- driver/vehicle/trip id, or the local date YYYY-MM-DD
  driver_id       uuid REFERENCES users(id),
  vehicle_id      uuid REFERENCES vehicles(id),
  trip_id         uuid REFERENCES trips(id),
  status          text NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING','ACCEPTED','SKIPPED')),
  attempts        int NOT NULL DEFAULT 0,
  next_attempt_at timestamptz NOT NULL DEFAULT now(),
  last_attempt_at timestamptz,
  last_code       text,
  last_error      text,
  reference       text,
  payload         jsonb,
  accepted_at     timestamptz,
  stuck_at        timestamptz,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  UNIQUE (brand_id, regulator, kind, subject_key)
);
CREATE INDEX IF NOT EXISTS regulator_due_idx ON regulator_submissions (next_attempt_at) WHERE status = 'PENDING';
CREATE INDEX IF NOT EXISTS regulator_list_idx ON regulator_submissions (brand_id, country_code, kind, status, created_at DESC);
CREATE INDEX IF NOT EXISTS regulator_trip_idx ON regulator_submissions (trip_id) WHERE trip_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS regulator_driver_idx ON regulator_submissions (driver_id) WHERE driver_id IS NOT NULL;

-- Drivers approved before this migration are queued for registration too, so
-- turning fail-closed on never locks out the existing fleet.
INSERT INTO regulator_submissions (brand_id, country_code, regulator, kind, subject_key, driver_id)
SELECT p.brand_id, p.country_code, c.regulator, 'DRIVER', p.user_id::text, p.user_id
FROM driver_profiles p JOIN countries c ON c.brand_id=p.brand_id AND c.code=p.country_code
WHERE p.status='APPROVED' AND c.regulator IS NOT NULL AND c.regulator NOT IN ('', 'none')
ON CONFLICT (brand_id, regulator, kind, subject_key) DO NOTHING;
INSERT INTO regulator_submissions (brand_id, country_code, regulator, kind, subject_key, driver_id, vehicle_id)
SELECT v.brand_id, p.country_code, c.regulator, 'VEHICLE', v.id::text, v.driver_id, v.id
FROM vehicles v JOIN driver_profiles p ON p.user_id=v.driver_id JOIN countries c ON c.brand_id=p.brand_id AND c.code=p.country_code
WHERE v.status='APPROVED' AND p.status='APPROVED' AND c.regulator IS NOT NULL AND c.regulator NOT IN ('', 'none')
ON CONFLICT (brand_id, regulator, kind, subject_key) DO NOTHING;

-- Parcel deliveries are not passenger trips: not submitted (owner question 3).
UPDATE services SET compliance = jsonb_set(compliance, '{latra,submit_trips}', 'false')
  WHERE key = 'parcel' AND compliance ? 'latra';

-- Staff who run compliance (new permission on Operations).
UPDATE roles SET permissions = array_append(permissions, 'compliance.manage')
  WHERE name = 'Operations' AND system = false AND NOT ('compliance.manage' = ANY(permissions));

-- +goose Down
UPDATE roles SET permissions = array_remove(permissions, 'compliance.manage');
DROP TABLE IF EXISTS regulator_submissions;
DROP TABLE IF EXISTS regulator_settings;
ALTER TABLE driver_profiles DROP COLUMN IF EXISTS licence_class;
ALTER TABLE countries DROP COLUMN IF EXISTS licence_classes, DROP COLUMN IF EXISTS licence_pattern;
