-- +goose Up
-- Staff roles become data: each role is a named set of permissions that a Super
-- Admin can create and edit. Sign-in sessions are recorded with where they came
-- from, so staff can see them and end them.
CREATE TABLE roles (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  brand_id    uuid NOT NULL REFERENCES brands(id),
  name        text NOT NULL,
  description text NOT NULL DEFAULT '',
  permissions text[] NOT NULL DEFAULT '{}',
  system      boolean NOT NULL DEFAULT false, -- the built-in Super Admin: cannot be edited or deleted
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (brand_id, name)
);

-- The five roles the dashboard shipped with, as editable defaults (Super Admin is locked).
INSERT INTO roles (brand_id, name, description, permissions, system)
SELECT b.id, r.name, r.description, r.permissions, r.system FROM brands b CROSS JOIN (VALUES
  ('Super Admin', 'Everything, including staff, roles and sessions.', ARRAY['*'], true),
  ('Operations', 'Runs the service day to day: trips, drivers, riders, prices and regions.',
     ARRAY['trips.cancel','riders.block','drivers.review','pricing.edit','areas.edit','holds.manage','safety.resolve'], false),
  ('Finance', 'Money: the ledger, pay-ins, holds and fees.', ARRAY['finance.view','fees.edit','holds.manage'], false),
  ('Support', 'Helps riders and drivers: ends stuck trips, blocks abuse, answers SOS.', ARRAY['trips.cancel','riders.block','safety.resolve'], false),
  ('Driver Review', 'Checks driver documents and approves or rejects drivers.', ARRAY['drivers.review'], false)
) AS r(name, description, permissions, system)
ON CONFLICT DO NOTHING;

-- One row per sign-in session (a refresh-token family).
CREATE TABLE sessions (
  family_id    uuid PRIMARY KEY,
  user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  app          text NOT NULL,
  ip           text,
  user_agent   text,
  device       text,
  location     jsonb,
  started_at   timestamptz NOT NULL DEFAULT now(),
  last_used_at timestamptz NOT NULL DEFAULT now(),
  ended_at     timestamptz,
  ended_by     uuid REFERENCES users(id),
  end_reason   text
);
CREATE INDEX sessions_user_idx ON sessions (user_id, started_at DESC);

-- Where a staff member may work. No rows = every country and region; a row with
-- no area = the whole country; a row with an area = that region only.
CREATE TABLE staff_access (
  user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  country_code char(2) NOT NULL,
  area_id      uuid REFERENCES areas(id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX staff_access_uq ON staff_access (user_id, country_code, coalesce(area_id, '00000000-0000-0000-0000-000000000000'::uuid));

-- +goose Down
DROP TABLE IF EXISTS staff_access;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS roles;
