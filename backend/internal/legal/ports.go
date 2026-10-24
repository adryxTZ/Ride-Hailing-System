package legal

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Identity is the brand data the texts' placeholders read (the Branding page's
// identity when staff edited it, else the brand file).
type Identity struct {
	Name, LegalEntity, SupportEmail, SupportPhone, EmergencyPhone string
}

// IdentitySource returns the live identity (cached by its owner).
type IdentitySource func(ctx context.Context) (Identity, error)

// ConsentLink is a document people accept, as the consent screen links it.
type ConsentLink struct {
	Kind  string `json:"kind"`
	Title Text   `json:"title"`
	URL   Text   `json:"url"`
}

// ConsentVersion is what a new consent version covers. Empty Title or Summary
// keep the previous version's (the consent screen's heading and short text).
type ConsentVersion struct {
	App       string
	Title     Text
	Summary   Text
	URL       Text // the Privacy Notice's page by locale
	Links     []ConsentLink
	Documents map[string]int // kind -> legal version
}

// ConsentPublisher is the consent record (privacy module): publishing a text
// people accept writes a new version in the publishing transaction, so the
// apps ask everyone again; the starter pack links the existing version instead.
type ConsentPublisher interface {
	NewVersionTx(ctx context.Context, tx pgx.Tx, v ConsentVersion) (int, error)
	SeedVersionTx(ctx context.Context, tx pgx.Tx, v ConsentVersion) (int, error)
}
