-- +goose Up
-- Legal documents edited in the admin (spec 2026-10-24-legal-documents.md):
-- per app and kind, versioned DRAFT -> PUBLISHED (immutable) -> ARCHIVED, with
-- EN and SW Markdown bodies. The values the texts' placeholders need that are
-- not brand data (registered address, DPO e-mail ...) live in legal_details.
-- consent_documents (00039) stays the record of what people accept; a consent
-- version now lists the legal document versions it covers and their links.

CREATE TABLE legal_documents (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  brand_id      uuid NOT NULL REFERENCES brands(id),
  app           text NOT NULL CHECK (app IN ('RIDER','DRIVER')),
  kind          text NOT NULL CHECK (kind IN ('PRIVACY_NOTICE','TERMS','DRIVER_AGREEMENT')),
  version       int NOT NULL CHECK (version > 0),
  status        text NOT NULL DEFAULT 'DRAFT' CHECK (status IN ('DRAFT','PUBLISHED','ARCHIVED')),
  title         jsonb NOT NULL,              -- by locale
  summary       jsonb NOT NULL DEFAULT '{}', -- the consent screen's short text, by locale
  body          jsonb NOT NULL,              -- Markdown by locale
  source        text NOT NULL DEFAULT 'STAFF' CHECK (source IN ('STARTER','STAFF')),
  change_reason text,
  created_by    uuid REFERENCES users(id),
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  published_by  uuid REFERENCES users(id),
  published_at  timestamptz,
  UNIQUE (brand_id, app, kind, version),
  CHECK (kind <> 'DRIVER_AGREEMENT' OR app = 'DRIVER'),
  CHECK ((status = 'DRAFT') = (published_at IS NULL))
);
-- One live version and at most one draft per document.
CREATE UNIQUE INDEX legal_documents_one_live ON legal_documents (brand_id, app, kind) WHERE status = 'PUBLISHED';
CREATE UNIQUE INDEX legal_documents_one_draft ON legal_documents (brand_id, app, kind) WHERE status = 'DRAFT';

-- A published text is evidence of what people agreed to: it never changes,
-- except that it is archived when the next version goes live.
-- +goose StatementBegin
CREATE FUNCTION legal_documents_frozen() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF OLD.status = 'DRAFT' THEN
    RETURN CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END;
  END IF;
  IF TG_OP = 'UPDATE' AND OLD.status = 'PUBLISHED' AND NEW.status = 'ARCHIVED'
     AND (to_jsonb(NEW) - 'status' - 'updated_at') = (to_jsonb(OLD) - 'status' - 'updated_at') THEN
    RETURN NEW;
  END IF;
  RAISE EXCEPTION 'a published legal document cannot change' USING ERRCODE = 'check_violation';
END $$;
-- +goose StatementEnd
CREATE TRIGGER legal_documents_frozen BEFORE UPDATE OR DELETE ON legal_documents FOR EACH ROW EXECUTE FUNCTION legal_documents_frozen();

-- Placeholder values staff fill in on the Legal page (keys are an allow-list in code).
CREATE TABLE legal_details (
  brand_id   uuid PRIMARY KEY REFERENCES brands(id),
  fields     jsonb NOT NULL DEFAULT '{}',
  updated_by uuid REFERENCES users(id),
  updated_at timestamptz NOT NULL DEFAULT now()
);

-- What a consent version covers: {kind: legal version} and the links the apps show.
ALTER TABLE consent_documents ADD COLUMN documents jsonb NOT NULL DEFAULT '{}';
ALTER TABLE consent_documents ADD COLUMN links jsonb NOT NULL DEFAULT '[]';

-- Staff who edit and publish the legal texts (new permission on Operations).
UPDATE roles SET permissions = array_append(permissions, 'legal.manage')
  WHERE name = 'Operations' AND system = false AND NOT ('legal.manage' = ANY(permissions));

-- +goose Down
UPDATE roles SET permissions = array_remove(permissions, 'legal.manage');
ALTER TABLE consent_documents DROP COLUMN IF EXISTS links;
ALTER TABLE consent_documents DROP COLUMN IF EXISTS documents;
DROP TABLE IF EXISTS legal_details;
DROP TABLE IF EXISTS legal_documents;
DROP FUNCTION IF EXISTS legal_documents_frozen();
