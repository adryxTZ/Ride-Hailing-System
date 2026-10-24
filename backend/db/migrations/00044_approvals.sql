-- +goose Up
-- Configurable approval workflows (spec 2026-10-24-approval-workflows):
-- a sensitive staff action whose workflow is enabled and whose conditions match
-- is not applied at once; it becomes an approval request that waits for the
-- workflow's steps. Workflows are data edited in the dashboard and versioned;
-- a request keeps the definition it started under. No rows = every workflow off.
CREATE TABLE approval_workflows (
  brand_id    uuid NOT NULL REFERENCES brands(id),
  action      text NOT NULL,
  enabled     boolean NOT NULL DEFAULT false,
  definition  jsonb NOT NULL DEFAULT '{}', -- {conditions, steps, expire_hours}
  version     int NOT NULL DEFAULT 1,
  updated_by  uuid REFERENCES users(id),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (brand_id, action)
);

-- Every saved definition, append-only: the history the editor shows.
CREATE TABLE approval_workflow_versions (
  brand_id    uuid NOT NULL REFERENCES brands(id),
  action      text NOT NULL,
  version     int NOT NULL,
  enabled     boolean NOT NULL,
  definition  jsonb NOT NULL,
  created_by  uuid REFERENCES users(id),
  reason      text NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (brand_id, action, version)
);

-- One held-back change. payload is what the action's executor needs; workflow
-- is the definition snapshot it is judged by; summary is the readable diff.
CREATE TABLE approval_requests (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  brand_id         uuid NOT NULL REFERENCES brands(id),
  action           text NOT NULL,
  status           text NOT NULL DEFAULT 'PENDING'
                   CHECK (status IN ('PENDING','APPROVED','EXECUTING','EXECUTED','FAILED','REJECTED','EXPIRED','CANCELLED')),
  requester_id     uuid NOT NULL REFERENCES users(id),
  reason           text NOT NULL,
  country_code     char(2),
  amount           numeric,
  target           text NOT NULL DEFAULT '',
  summary          jsonb NOT NULL DEFAULT '{}',
  payload          jsonb NOT NULL,
  payload_hash     text NOT NULL,
  workflow         jsonb NOT NULL,
  workflow_version int NOT NULL,
  current_step     int NOT NULL DEFAULT 0,
  expires_at       timestamptz NOT NULL,
  created_at       timestamptz NOT NULL DEFAULT now(),
  decided_at       timestamptz,
  executed_at      timestamptz,
  result           jsonb,
  error            text
);
-- The same change asked for twice while pending is the same request (retries are safe).
CREATE UNIQUE INDEX approval_requests_pending_uq ON approval_requests (brand_id, action, payload_hash) WHERE status = 'PENDING';
CREATE INDEX approval_requests_status_idx ON approval_requests (brand_id, status, created_at DESC);
CREATE INDEX approval_requests_requester_idx ON approval_requests (requester_id, created_at DESC);
CREATE INDEX approval_requests_expiry_idx ON approval_requests (expires_at) WHERE status = 'PENDING';

-- One person's decision on a request; a person decides a request at most once.
CREATE TABLE approval_decisions (
  id           bigserial PRIMARY KEY,
  request_id   uuid NOT NULL REFERENCES approval_requests(id) ON DELETE CASCADE,
  step         int NOT NULL,
  approver_id  uuid NOT NULL REFERENCES users(id),
  decision     text NOT NULL CHECK (decision IN ('APPROVE','REJECT')),
  as_role      text NOT NULL DEFAULT '', -- the role (or "perm:<key>") the approval counted as
  note         text NOT NULL DEFAULT '',
  created_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (request_id, approver_id)
);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION approvals_refuse_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION '% rows are immutable', TG_TABLE_NAME USING ERRCODE = 'check_violation';
END $$;

-- The database also refuses a requester deciding their own request.
CREATE OR REPLACE FUNCTION approvals_no_self_decision() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF EXISTS (SELECT 1 FROM approval_requests r WHERE r.id = NEW.request_id AND r.requester_id = NEW.approver_id) THEN
    RAISE EXCEPTION 'a requester cannot decide their own approval request' USING ERRCODE = 'check_violation';
  END IF;
  RETURN NEW;
END $$;
-- +goose StatementEnd

CREATE TRIGGER approval_workflow_versions_append_only BEFORE UPDATE OR DELETE ON approval_workflow_versions
  FOR EACH ROW EXECUTE FUNCTION approvals_refuse_change();
CREATE TRIGGER approval_decisions_append_only BEFORE UPDATE OR DELETE ON approval_decisions
  FOR EACH ROW EXECUTE FUNCTION approvals_refuse_change();
CREATE TRIGGER approval_decisions_not_self BEFORE INSERT ON approval_decisions
  FOR EACH ROW EXECUTE FUNCTION approvals_no_self_decision();

INSERT INTO config_versions (scope) VALUES ('approvals') ON CONFLICT DO NOTHING;

-- approvals.manage (edit workflows): Super Admin holds it through "*"; Operations
-- runs the service day to day and gets it by default.
UPDATE roles SET permissions = array_append(permissions, 'approvals.manage'), updated_at = now()
WHERE name = 'Operations' AND NOT system AND NOT ('approvals.manage' = ANY (permissions));

-- +goose Down
UPDATE roles SET permissions = array_remove(permissions, 'approvals.manage') WHERE 'approvals.manage' = ANY (permissions);
DELETE FROM config_versions WHERE scope = 'approvals';
DROP TABLE IF EXISTS approval_decisions;
DROP TABLE IF EXISTS approval_requests;
DROP TABLE IF EXISTS approval_workflow_versions;
DROP TABLE IF EXISTS approval_workflows;
DROP FUNCTION IF EXISTS approvals_no_self_decision();
DROP FUNCTION IF EXISTS approvals_refuse_change();
