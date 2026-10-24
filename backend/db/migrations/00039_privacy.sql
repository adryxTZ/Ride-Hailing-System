-- +goose Up
-- Data protection (spec 2026-10-23-compliance-trust.md, S4; planned as 00034,
-- that number was taken): versioned consent per app, privacy requests (export,
-- delete) handled by staff, and the daily retention job's run log.

-- The consent texts people accept, per app and version (synced from the brand file).
CREATE TABLE consent_documents (
  brand_id     uuid NOT NULL REFERENCES brands(id),
  app          text NOT NULL CHECK (app IN ('RIDER','DRIVER')),
  version      int NOT NULL CHECK (version > 0),
  title        jsonb NOT NULL,
  summary      jsonb NOT NULL,
  url          jsonb NOT NULL,
  published_at timestamptz NOT NULL,
  PRIMARY KEY (brand_id, app, version)
);

-- Who accepted which version, when and from where. Never changed or deleted.
CREATE TABLE user_consents (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     uuid NOT NULL REFERENCES users(id),
  app         text NOT NULL CHECK (app IN ('RIDER','DRIVER')),
  version     int NOT NULL,
  accepted_at timestamptz NOT NULL DEFAULT now(),
  install_id  uuid,
  ip          inet,
  user_agent  text,
  UNIQUE (user_id, app, version)
);
-- +goose StatementBegin
CREATE FUNCTION user_consents_append_only() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'consent records are append-only' USING ERRCODE = 'check_violation';
END $$;
-- +goose StatementEnd
CREATE TRIGGER user_consents_append_only BEFORE UPDATE OR DELETE ON user_consents FOR EACH ROW EXECUTE FUNCTION user_consents_append_only();

-- Never used: consent lives in user_consents per app and version.
ALTER TABLE users DROP COLUMN IF EXISTS pdpa_consent_version;

CREATE TABLE privacy_requests (
  id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  brand_id          uuid NOT NULL REFERENCES brands(id),
  user_id           uuid NOT NULL REFERENCES users(id),
  app               text NOT NULL,
  kind              text NOT NULL CHECK (kind IN ('EXPORT','DELETE')),
  status            text NOT NULL DEFAULT 'REQUESTED' CHECK (status IN ('REQUESTED','IN_PROGRESS','READY','DONE','REJECTED','CANCELLED')),
  reason            text,
  created_at        timestamptz NOT NULL DEFAULT now(),
  due_at            timestamptz NOT NULL,
  handled_by        uuid REFERENCES users(id),
  handled_at        timestamptz,
  export_key        text,
  export_expires_at timestamptz,
  blockers          jsonb NOT NULL DEFAULT '[]'
);
CREATE UNIQUE INDEX privacy_requests_one_open ON privacy_requests (user_id, kind) WHERE status IN ('REQUESTED','IN_PROGRESS','READY');
CREATE INDEX privacy_requests_queue_idx ON privacy_requests (brand_id, status, due_at);

CREATE TABLE retention_runs (
  id          bigserial PRIMARY KEY,
  brand_id    uuid NOT NULL REFERENCES brands(id),
  rule        text NOT NULL,
  run_date    date NOT NULL,
  started_at  timestamptz NOT NULL DEFAULT now(),
  finished_at timestamptz,
  rows        int NOT NULL DEFAULT 0,
  error       text,
  UNIQUE (brand_id, rule, run_date)
);

-- A document's file can be purged (retention) while its row stays as a record.
ALTER TABLE documents ADD COLUMN purged_at timestamptz;

-- Retention may strip GPS evidence and delete dismissed flags; nothing else may
-- change a reviewed flag. The job marks its transaction with nexa.retention.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION fraud_flags_reviewed_frozen() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF coalesce(current_setting('nexa.retention', true), '') = 'on' THEN
    RETURN CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END;
  END IF;
  IF TG_OP = 'DELETE' THEN
    RAISE EXCEPTION 'fraud flags are never deleted' USING ERRCODE = 'check_violation';
  END IF;
  IF OLD.status <> 'OPEN' THEN
    RAISE EXCEPTION 'a reviewed fraud flag cannot change' USING ERRCODE = 'check_violation';
  END IF;
  RETURN NEW;
END $$;
-- +goose StatementEnd

INSERT INTO config_versions (scope) VALUES ('legal') ON CONFLICT DO NOTHING;

-- Staff who handle privacy requests (new permission on Operations).
UPDATE roles SET permissions = array_append(permissions, 'privacy.handle')
  WHERE name = 'Operations' AND system = false AND NOT ('privacy.handle' = ANY(permissions));

-- +goose Down
UPDATE roles SET permissions = array_remove(permissions, 'privacy.handle');
DELETE FROM config_versions WHERE scope = 'legal';
ALTER TABLE documents DROP COLUMN IF EXISTS purged_at;
DROP TABLE IF EXISTS retention_runs;
DROP TABLE IF EXISTS privacy_requests;
ALTER TABLE users ADD COLUMN IF NOT EXISTS pdpa_consent_version int;
DROP TABLE IF EXISTS user_consents;
DROP FUNCTION IF EXISTS user_consents_append_only();
DROP TABLE IF EXISTS consent_documents;
