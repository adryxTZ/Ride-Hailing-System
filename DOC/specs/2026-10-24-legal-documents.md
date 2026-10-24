# Legal documents (spec + factory log)

Owner's request (Kalen): "by copying Bolt or Uber generate a starter pack for this, this should be configurable in admin dash for both apps, since the app is generic and made to be sold to various people".

Builds on S4 of `2026-10-23-compliance-trust.md` (consent, privacy requests). That spec listed "editing consent texts in admin" as out of scope; this slice brings it in.

## 1. Goal
Every brand runs on its own legal texts without a release:
- the rider app has a **Privacy Notice** and **Terms of Use**;
- the driver app has a **Privacy Notice**, **Terms of Use** and a **Driver Agreement**.

Staff edit them in the admin in English and Swahili, publish a version with a reason, and the apps ask people to accept again. A new brand starts with a generic **starter pack**. Its structure follows what the large ride-hailing platforms' public notices cover, but the wording is original. It is marked as a template for a lawyer to review.

## 2. Scope
**In:**
- `legal_documents`:
  - per app, kind and version: DRAFT, then PUBLISHED (immutable, enforced by a trigger), then ARCHIVED;
  - EN/SW title, a consent-screen summary and a Markdown body;
  - source STARTER or STAFF;
  - change reason, published by and published at.
- `legal_details`: placeholder values that are not brand data, edited on the Legal page and audited.
- Placeholders, filled at render time:
  - `{{brand.name}}`, `{{legal_entity}}`, `{{registered_address}}`, `{{company_number}}`;
  - `{{dpo_name}}`, `{{dpo_email}}`, `{{data_protection_authority}}`, `{{regulator}}`, `{{minimum_age}}`;
  - `{{support_email}}`, `{{support_phone}}`, `{{emergency_phone}}`, `{{website}}`;
  - `{{country.name}}`, `{{effective_date}}`, `{{version}}`.
  - Unknown keys are refused when a draft is saved. A key with no value shows as `[key]` and is listed as missing.
- A safe Markdown subset is rendered on the server (`internal/legal/markdown*.go`): headings, paragraphs, lists, quotes, rules, bold, italic, code, links (http, https, mailto, tel, same-site only) and autolinked e-mails and URLs. Every character is escaped, so raw HTML and scripts arrive as text.
- The starter pack lives in `backend/seed/legal/`: `manifest.json` plus 10 Markdown files. It is data, not code. It is published as version 1 when the brand has no texts, on `serve` start and on `nexa seed legal` (also part of `seed all`). The seed is idempotent and runs under an advisory lock.
- Consent: the database is the source of truth (details in §3).
- Public pages: `GET /v1/legal/{app}/{kind}?lang=en|sw[&format=json]`.
  - Sanitised HTML, or `LegalPage` JSON.
  - Headers: ETag/304, `Cache-Control: public, max-age=300`, `Vary: Accept-Language`.
  - A CSP with `default-src 'none'` (no script can run), `nosniff`, `no-referrer`.
  - The page uses the published theme's colours.
- Admin, Settings › Legal (`/settings/legal`):
  - the documents table and the Legal details form;
  - a document page (`/settings/legal/{rider|driver}/{slug}`):
    - an EN/SW editor;
    - a live server preview with placeholders filled, warning about missing or unknown placeholders;
    - a placeholder list with values and Insert at the cursor;
    - Save draft, Discard draft, and Publish (reason dialog; texts people accept warn that "all riders/drivers will be asked to accept again");
    - version history with a folded line diff against the previous version, the text, and "Copy into editor".
- Permission `legal.manage` (group Settings). Super Admin holds it through `*`. The migration adds it to the default **Operations** role.
- Apps: the consent screen keeps "Read the Privacy Notice" (`url`) and adds a link for each other covered document (`links[]`: Terms, Driver Agreement).

**Out:**
- Per-country texts. A brand has one set; `{{country.name}}` is the home country.
- Rich text or WYSIWYG editing.
- Scheduled publishing.
- Rolling back as an operation. "Copy into editor" followed by publishing gives the same result as a new version.
- Showing in-app the text of an archived version people accepted. It stays in the database as evidence.

## 3. Consent versioning (how it works now)
- `consent_documents` (00039) remains the record of what people accept. 00042 adds:
  - `documents` (`{kind: legal version}`), the texts a consent version covers;
  - `links` (`[{kind, title{}, url{}}]`).
- Publishing a PRIVACY_NOTICE, TERMS or DRIVER_AGREEMENT for an app calls `privacy.NewVersionTx` **in the publishing transaction**. That writes consent version *n+1* for the app and keeps the previous title. The summary comes from the Privacy Notice's summary, or stays the previous one when that is empty.
- The same transaction bumps `config_versions('legal')`. `privacy` reads consent versions through a versioned cache on that scope (5 s check), so the gate (`Required`), `GET /legal/consent` and `GET /me` see the new version within seconds. Hot paths stay off Postgres.
- `GET /legal/consent?app=` and `POST /me/consents {app, version}` are unchanged for the apps:
  - `ConsentDocument.url` is the Privacy Notice page;
  - `links[]` is new;
  - a stale version still answers 409 `CONSENT_VERSION_STALE`.
- Links are stored relative to the API (`/legal/rider/terms?lang=en`). The handler makes them absolute from `NEXA_PUBLIC_API_URL`, falling back to the caller's host in development.
- The brand file's `legal.consent[]` is now seed and fallback only. `privacy.SeedDocuments` writes it for an app that has **no** consent version in the database, and never overwrites one.
- The starter pack links an app's existing latest consent version (via `SeedVersionTx`) instead of creating a new one. On an existing database, people who accepted the brand-file notice are therefore not asked again just because the templates were loaded. On an empty database, it writes version 1.

## 4. Contract
- Routes, all declared in `backend/api/openapi.yaml`:
  - `GET /legal/{app}/{kind}` (public, `security: []`);
  - `GET /admin/legal`;
  - `PUT /admin/legal/details`;
  - `POST /admin/legal/preview`;
  - `GET /admin/legal/{app}/{kind}`;
  - `PUT|DELETE /admin/legal/{app}/{kind}/draft`;
  - `POST /admin/legal/{app}/{kind}/publish`.
  - Every admin route needs `legal.manage`.
- New schemas: `ConsentLink`, `LegalDocument`, `LegalVersionRef`, `LegalOverviewItem`, `LegalOverview`, `LegalPlaceholder`, `LegalDetails`, `LegalDetailsInput`, `LegalVersions`, `LegalDraftInput`, `LegalPreviewInput`, `LegalPreview`, `LegalPage`.
- New parameters: `LegalApp`, `LegalKind`. `ConsentDocument.links` (required) was added.
- Errors:
  - `LEGAL_DOCUMENT_NOT_FOUND` 404;
  - `LEGAL_NO_DRAFT` 409;
  - `REASON_REQUIRED`;
  - `VALIDATION_FAILED` with `details.fields` codes `NOT_ALLOWED`, `LENGTH`, `UNKNOWN_PLACEHOLDER`, `REQUIRED` (publish needs a title and body in every brand language), `FORMAT`. The new codes are in `i18n/{en,sw}.json`.
- Both TS clients were regenerated.

## 5. Data (`00042_legal_documents.sql`)
- `legal_documents`:
  - one PUBLISHED and one DRAFT per (brand, app, kind), as partial unique indexes;
  - the trigger `legal_documents_frozen` allows only PUBLISHED to ARCHIVED (nothing else changes) and refuses to delete a non-draft;
  - the driver agreement is CHECKed to the driver app.
- `legal_details (brand_id PK, fields jsonb)`.
- `consent_documents.documents` and `consent_documents.links`.
- The `legal.manage` grant to Operations.
- The Down migration reverses all of it and was run (down, then up) on `nexa_legal`.

## 6. Acceptance
- [x] Draft, then publish with a reason. The live version is archived, the published row cannot be updated (trigger), and it is audited (`legal.draft`, `legal.publish`, `legal.details`, `legal.starter`).
- [x] Publishing a consent kind moves the app's consent version on. The rider is refused booking (403 `CONSENT_REQUIRED`), accepts the new version, and the old version is refused as stale.
- [x] The public page fills placeholders. `<script>`, `<img onerror>` and `javascript:` links in the text arrive as text. CSP is set, the ETag gives 304, and the JSON variant and the Swahili page answer.
- [x] The starter pack covers every document of both apps in EN and SW, opens with the lawyer-review note, and uses only catalogue placeholders (test).
- [x] Admin: overview, editor with preview, history diff, and the publish dialog with the "asked again" warning (screenshots read). The dialogs are portalled so the backdrop covers the window.

## 7. Factory log (2026-10-24)
- **Read:** S4 spec, `internal/privacy`, `brand/legal.go`, `handlers_privacy.go`, the apps' consent store and screen, branding (draft/publish pattern, versioned cache).
- **Decisions:**
  - Own Markdown renderer instead of goldmark/bluemonday. They are not in the module cache, and escaping everything is safe by construction.
  - `legal` owns texts, rendering and the starter pack. `privacy` owns consent versions. They are joined by the port `legal.ConsentPublisher`, adapted in `cmd/nexa/wire_legal.go`, so neither package imports the other.
  - The driver agreement is a consent kind.
- **Built:**
  - `internal/legal`: 14 files plus tests;
  - privacy `notices.go`, `store_notices.go` and its DB-backed consent;
  - `handlers_legal.go`, `handlers_legal_page.go`, `pages/legal.html`;
  - admin `features/legal/*`, two routes, nav item "Legal", `components/ui/Portal.tsx`;
  - the apps' consent screen links;
  - `apps/tools/e2e-legal.mjs`, `apps/tools/shot-legal.mjs`.
- **Fixed after screenshots:**
  - kind titles on the Legal page showed in the brand's default language (sw). The dashboard now uses English;
  - the publish and details dialogs' backdrops were clipped to the card. They now use a Portal;
  - the history diff printed whole texts. Unchanged runs now fold, with 2 lines of context.
- **Tests:**
  - `internal/legal`: markdown safety, placeholders, kinds, starter pack completeness, and a DB test of starter, draft, publish, archive, the frozen row, details and the page;
  - privacy: consent versions from the database (seed, link, new version, asked again); the existing privacy tests run on static notices.
  - Test brands are removed in one transaction, with the frozen and audit triggers switched off inside it.
- **Known:** the admin has no unit-test runner, so `diff.ts` is exercised through the UI only.
