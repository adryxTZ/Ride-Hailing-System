-- +goose Up
-- Branding from the dashboard: the brand's identity becomes editable (the brand
-- file only seeds it), theme versions are drafted, published and rolled back,
-- and app config reads the published version through a versioned cache.

ALTER TABLE brands
  ADD COLUMN IF NOT EXISTS emergency_phone text,
  ADD COLUMN IF NOT EXISTS identity_edited_at timestamptz,   -- set once staff edit; the seed then stops overwriting
  ADD COLUMN IF NOT EXISTS identity_edited_by uuid REFERENCES users(id);

ALTER TABLE brand_themes
  ADD COLUMN IF NOT EXISTS roles jsonb NOT NULL DEFAULT '{}';  -- the colours staff chose per role, before derivation

INSERT INTO config_versions (scope) VALUES ('branding') ON CONFLICT DO NOTHING;

-- The new branding.manage permission is not granted to any role by default:
-- Super Admin holds every permission and can add it to a role.

-- +goose Down
DELETE FROM config_versions WHERE scope = 'branding';
ALTER TABLE brand_themes DROP COLUMN IF EXISTS roles;
ALTER TABLE brands DROP COLUMN IF EXISTS identity_edited_by, DROP COLUMN IF EXISTS identity_edited_at, DROP COLUMN IF EXISTS emergency_phone;
