-- +goose Up
-- Phase C, slice 2: driver documents are configured per country by staff
-- (mandatory, optional or off; expiry tracked; an expired one may hold the
-- driver), expiry reminders are sent once each, and renewals sit beside the
-- current document until staff approve them.

CREATE TABLE IF NOT EXISTS document_types (
  brand_id            uuid NOT NULL REFERENCES brands(id),
  country_code        char(2) NOT NULL,
  kind                text NOT NULL CHECK (kind ~ '^[A-Z][A-Z0-9_]{1,31}$'),
  owner               document_owner NOT NULL,
  label               jsonb NOT NULL,           -- {en, sw}
  requirement         text NOT NULL DEFAULT 'MANDATORY' CHECK (requirement IN ('MANDATORY','OPTIONAL','OFF')),
  tracks_expiry       boolean NOT NULL DEFAULT false,
  expiry_required     boolean NOT NULL DEFAULT false, -- the driver must give the expiry date when uploading
  blocks_when_expired boolean NOT NULL DEFAULT false, -- an expired current copy puts the driver on hold
  sort_order          int NOT NULL DEFAULT 0,
  updated_by          uuid REFERENCES users(id),
  updated_at          timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (brand_id, country_code, kind),
  CHECK (tracks_expiry OR NOT (expiry_required OR blocks_when_expired))
);

-- Every country starts with the documents the Platform asked for until now
-- (all mandatory), plus police clearance and vehicle inspection as optional.
INSERT INTO document_types (brand_id, country_code, kind, owner, label, requirement, tracks_expiry, expiry_required, blocks_when_expired, sort_order)
SELECT c.brand_id, c.code, d.kind, d.owner::document_owner, d.label::jsonb, d.requirement, d.tracks, d.required, d.blocks, d.sort
FROM countries c CROSS JOIN (VALUES
  ('PHOTO',            'DRIVER',  '{"en":"Your photo","sw":"Picha yako"}',                                         'MANDATORY', false, false, false, 10),
  ('LICENCE',          'DRIVER',  '{"en":"Driving licence","sw":"Leseni ya udereva"}',                              'MANDATORY', true,  true,  true,  20),
  ('NATIONAL_ID',      'DRIVER',  '{"en":"National ID","sw":"Kitambulisho cha taifa"}',                             'MANDATORY', false, false, false, 30),
  ('POLICE_CLEARANCE', 'DRIVER',  '{"en":"Police clearance certificate","sw":"Cheti cha tabia njema (polisi)"}',    'OPTIONAL',  true,  false, false, 40),
  ('REGISTRATION',     'VEHICLE', '{"en":"Vehicle registration card","sw":"Kadi ya usajili wa gari"}',              'MANDATORY', false, false, false, 50),
  ('INSURANCE',        'VEHICLE', '{"en":"Insurance sticker","sw":"Stika ya bima"}',                                'MANDATORY', true,  true,  true,  60),
  ('INSPECTION',       'VEHICLE', '{"en":"Vehicle inspection sticker","sw":"Stika ya ukaguzi wa gari"}',            'OPTIONAL',  true,  false, false, 70),
  ('VEHICLE_PHOTO',    'VEHICLE', '{"en":"Photo of the vehicle","sw":"Picha ya gari"}',                             'MANDATORY', false, false, false, 80)
) AS d(kind, owner, label, requirement, tracks, required, blocks, sort)
ON CONFLICT (brand_id, country_code, kind) DO NOTHING;

-- Renewals: a new upload for an approved driver sits beside the current
-- document (renewal_of) until staff approve it; the old one is then archived.
ALTER TABLE documents
  ADD COLUMN IF NOT EXISTS renewal_of uuid REFERENCES documents(id),
  ADD COLUMN IF NOT EXISTS archived_at timestamptz;
DROP INDEX IF EXISTS documents_current_idx;
CREATE UNIQUE INDEX IF NOT EXISTS documents_current_uq ON documents (owner_kind, owner_id, kind) WHERE archived_at IS NULL AND renewal_of IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS documents_renewal_uq ON documents (owner_kind, owner_id, kind) WHERE archived_at IS NULL AND renewal_of IS NOT NULL;
CREATE INDEX IF NOT EXISTS documents_expiry_idx ON documents (brand_id, expires_on) WHERE archived_at IS NULL AND renewal_of IS NULL AND expires_on IS NOT NULL;

-- The licence expiry the profile form used to keep moves onto the document.
UPDATE documents d SET expires_on = p.licence_expiry
FROM driver_profiles p WHERE d.kind='LICENCE' AND d.owner_kind='DRIVER' AND d.owner_id=p.user_id
  AND d.expires_on IS NULL AND p.licence_expiry IS NOT NULL;

-- Each expiry reminder is sent at most once per document and threshold.
CREATE TABLE IF NOT EXISTS document_reminders (
  document_id    uuid NOT NULL REFERENCES documents(id),
  threshold_days int NOT NULL,
  sent_at        timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (document_id, threshold_days)
);

-- A DOCUMENT hold is placed and lifted by the system when a required document expires.
ALTER TABLE driver_holds DROP CONSTRAINT IF EXISTS driver_holds_kind_check;
ALTER TABLE driver_holds ADD CONSTRAINT driver_holds_kind_check CHECK (kind IN ('DEBT','MANUAL','DOCUMENT'));
ALTER TABLE driver_holds DROP CONSTRAINT IF EXISTS driver_holds_cause_check;
ALTER TABLE driver_holds ADD CONSTRAINT driver_holds_cause_check CHECK (cause IN ('HARD_LIMIT','SOFT_LIMIT_OVERDUE','STAFF','DOCUMENT_EXPIRED'));
ALTER TABLE driver_holds ADD COLUMN IF NOT EXISTS details jsonb NOT NULL DEFAULT '{}';

-- +goose Down
DELETE FROM driver_holds WHERE kind='DOCUMENT';
ALTER TABLE driver_holds DROP COLUMN IF EXISTS details;
ALTER TABLE driver_holds DROP CONSTRAINT IF EXISTS driver_holds_cause_check;
ALTER TABLE driver_holds ADD CONSTRAINT driver_holds_cause_check CHECK (cause IN ('HARD_LIMIT','SOFT_LIMIT_OVERDUE','STAFF'));
ALTER TABLE driver_holds DROP CONSTRAINT IF EXISTS driver_holds_kind_check;
ALTER TABLE driver_holds ADD CONSTRAINT driver_holds_kind_check CHECK (kind IN ('DEBT','MANUAL'));
DROP TABLE IF EXISTS document_reminders;
DELETE FROM documents WHERE renewal_of IS NOT NULL OR archived_at IS NOT NULL;
DROP INDEX IF EXISTS documents_expiry_idx;
DROP INDEX IF EXISTS documents_renewal_uq;
DROP INDEX IF EXISTS documents_current_uq;
CREATE UNIQUE INDEX IF NOT EXISTS documents_current_idx ON documents (owner_kind, owner_id, kind);
ALTER TABLE documents DROP COLUMN IF EXISTS archived_at, DROP COLUMN IF EXISTS renewal_of;
DROP TABLE IF EXISTS document_types;
