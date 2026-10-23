-- +goose Up
-- Drivers, their vehicles and the documents that prove both (KYC). A driver may
-- go online only when the profile and the vehicle in use are APPROVED.
CREATE TYPE driver_status AS ENUM ('ONBOARDING', 'PENDING_REVIEW', 'APPROVED', 'REJECTED', 'SUSPENDED');
CREATE TYPE review_status AS ENUM ('PENDING', 'APPROVED', 'REJECTED');

CREATE TABLE driver_profiles (
  user_id          uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  brand_id         uuid NOT NULL REFERENCES brands(id),
  status           driver_status NOT NULL DEFAULT 'ONBOARDING',
  licence_number   text,
  licence_expiry   date,
  national_id      text,
  status_reason    text,                      -- why rejected / suspended
  reviewed_by      uuid REFERENCES users(id),
  reviewed_at      timestamptz,
  submitted_at     timestamptz,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX driver_profiles_status_idx ON driver_profiles (brand_id, status, submitted_at);

CREATE TABLE vehicles (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  brand_id      uuid NOT NULL REFERENCES brands(id),
  driver_id     uuid REFERENCES users(id) ON DELETE SET NULL,   -- who drives it now
  service_key   text NOT NULL,
  class_key     text NOT NULL,
  plate         text NOT NULL,
  make          text,
  model         text,
  colour        text,
  year          int CHECK (year BETWEEN 1950 AND 2100),
  status        review_status NOT NULL DEFAULT 'PENDING',
  status_reason text,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  UNIQUE (brand_id, plate)
);
CREATE INDEX vehicles_driver_idx ON vehicles (driver_id);

CREATE TYPE document_owner AS ENUM ('DRIVER', 'VEHICLE');
CREATE TABLE documents (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  brand_id      uuid NOT NULL REFERENCES brands(id),
  owner_kind    document_owner NOT NULL,
  owner_id      uuid NOT NULL,                -- users.id or vehicles.id
  kind          text NOT NULL,                -- LICENCE, NATIONAL_ID, PHOTO, REGISTRATION, INSURANCE, VEHICLE_PHOTO, PERMIT
  file_key      text NOT NULL,
  content_type  text NOT NULL,
  status        review_status NOT NULL DEFAULT 'PENDING',
  status_reason text,
  expires_on    date,
  reviewed_by   uuid REFERENCES users(id),
  reviewed_at   timestamptz,
  created_at    timestamptz NOT NULL DEFAULT now()
);
-- One current document per owner and kind; a re-upload replaces it.
CREATE UNIQUE INDEX documents_current_idx ON documents (owner_kind, owner_id, kind);

-- Every staff decision on a driver, vehicle or document, append-only.
CREATE TABLE audit_log (
  id         bigserial PRIMARY KEY,
  brand_id   uuid NOT NULL REFERENCES brands(id),
  actor_id   uuid REFERENCES users(id),
  action     text NOT NULL,
  target     text NOT NULL,                   -- e.g. driver:<uuid>
  reason     text,
  data       jsonb NOT NULL DEFAULT '{}',
  at         timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_log_target_idx ON audit_log (brand_id, target, at DESC);
-- +goose StatementBegin
CREATE FUNCTION audit_log_append_only() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'audit_log is append-only'; END $$;
-- +goose StatementEnd
CREATE TRIGGER audit_log_no_change BEFORE UPDATE OR DELETE ON audit_log
  FOR EACH ROW EXECUTE FUNCTION audit_log_append_only();

-- +goose Down
DROP TRIGGER audit_log_no_change ON audit_log;
DROP FUNCTION audit_log_append_only();
DROP TABLE audit_log;
DROP TABLE documents;
DROP TYPE document_owner;
DROP TABLE vehicles;
DROP TABLE driver_profiles;
DROP TYPE review_status;
DROP TYPE driver_status;
