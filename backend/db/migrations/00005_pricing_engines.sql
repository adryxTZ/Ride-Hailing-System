-- +goose Up
-- Pricing modules are selectable per tariff; params hold the chosen engine's settings.
ALTER TABLE tariffs ADD COLUMN engine text NOT NULL DEFAULT 'bracket_meter';
ALTER TABLE tariffs ADD COLUMN params jsonb NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE tariffs DROP COLUMN params;
ALTER TABLE tariffs DROP COLUMN engine;
