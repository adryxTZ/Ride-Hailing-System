// Package legal is the apps' legal texts, edited in the admin (spec
// 2026-10-24-legal-documents.md): per app (RIDER, DRIVER) a Privacy Notice and
// Terms of Use, and for drivers a Driver Agreement. Each is versioned: a draft
// is edited (EN and SW Markdown), published with a reason (audited, then never
// changed) and archived when the next version goes live. Publishing a text
// people accept moves the app's consent version on (privacy module), so riders
// or drivers accept again. Placeholders ({{brand.name}}, {{dpo_email}} ...) are
// filled when a text is shown, from brand data and the Legal details form. A
// generic starter pack (seed/legal, templates for a lawyer to review) is
// published as version 1 when a brand has no texts yet.
//
//   - model.go            types, constants, error codes
//   - kinds.go            the kinds per app, consent kinds, URL slugs
//   - placeholders.go     the placeholder catalogue, Fill, unknown-key check
//   - markdown.go         block Markdown to HTML (safe subset, all text escaped)
//   - markdown_inline.go  bold, italic, code, links (safe schemes only), autolinks
//   - dates.go            the effective date in each language
//   - render.go           Page (public, cached), Preview, placeholder values
//   - service.go          Service, the cache, Overview, Versions
//   - drafts.go           SaveDraft, DeleteDraft, validation
//   - publish.go          Publish and the consent version it writes
//   - details.go          the Legal details form, placeholder values for the editor
//   - starter.go          loading and seeding the starter pack
//   - ports.go            brand identity and the consent record
//   - store.go            persistence
package legal
