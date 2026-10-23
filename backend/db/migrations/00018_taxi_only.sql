-- +goose Up
-- The Platform is taxi only (2026-10-20): goods services and their cargo types are gone.
-- Other services stay as DISABLED rows so historical trips keep their names.
DROP TABLE cargo_types;
ALTER TABLE vehicle_classes DROP COLUMN capacity_kg_min, DROP COLUMN capacity_kg_max, DROP COLUMN volume_l_max;

-- +goose Down
ALTER TABLE vehicle_classes ADD COLUMN capacity_kg_min int, ADD COLUMN capacity_kg_max int, ADD COLUMN volume_l_max int;
CREATE TABLE cargo_types (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  service_id      uuid NOT NULL REFERENCES services(id) ON DELETE CASCADE,
  key             text NOT NULL,
  name            jsonb NOT NULL,
  description     jsonb NOT NULL DEFAULT '{}',
  icon            text,
  sort_order      int NOT NULL DEFAULT 0,
  allowed_classes text[] NOT NULL DEFAULT '{}',
  status          service_status NOT NULL DEFAULT 'ACTIVE',
  UNIQUE (service_id, key)
);
