// Package branding lets staff change how the Platform looks without a release
// (DOC/03-branding-theming.md): the brand's identity (name, legal entity,
// support contacts), and versioned themes (colours from a preset, a Color Hunt
// link or custom picks, a dark variant, fonts, corner radius, logos and icons)
// that are drafted, checked for contrast, published with a reason and rolled
// back. App config reads the published version through a versioned cache.
//
//   - model.go    Identity, ThemeVersion, DraftInput, allow-lists, error codes
//   - ports.go    object storage for logos and icons
//   - store.go    brands and brand_themes persistence, the config version bump
//   - service.go  Identity / SetIdentity, Published (cached), Versions, Get
//   - drafts.go   CreateDraft, UpdateDraft, DeleteDraft, Preview, validation
//   - publish.go  Publish (and rollback = publishing an archived version)
//   - assets.go   upload tickets, asset checks, public asset links
package branding
