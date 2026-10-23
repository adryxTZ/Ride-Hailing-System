package receipts

import (
	"context"
	"log/slog"
	"net/url"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Service issues and serves receipts.
type Service struct {
	store  store
	issuer func(country string) Issuer // the issuer (with the country's timezone)
	signer signer
	now    func() time.Time
}

// Deps is everything the service needs.
type Deps struct {
	Pool    *pgxpool.Pool
	BrandID string
	// Issuer returns who issues receipts in a country, with its timezone (local times, numbering year).
	Issuer func(country string) Issuer
	// LinkKey signs PDF links.
	LinkKey []byte
}

// New builds the service.
func New(d Deps) *Service {
	return &Service{store: store{pool: d.Pool, brandID: d.BrandID}, issuer: d.Issuer, signer: signer{key: d.LinkKey}, now: time.Now}
}

// IssueTx issues the trip's receipt inside the transaction that completes it:
// built from the trip's facts, numbered from the year's counter, stored once.
// Issuing for a trip that already has a receipt does nothing.
func (s *Service) IssueTx(ctx context.Context, tx pgx.Tx, f Facts) error {
	if ok, err := s.store.exists(ctx, tx, f.TripID); err != nil || ok {
		return err
	}
	issuer := s.issuer(f.Country)
	r, err := Build(f, issuer)
	if err != nil {
		// The trip cannot complete without its receipt: say so loudly for Finance.
		slog.Error("receipts: receipt not issued; the trip stays open", "trip", f.TripID, "err", err)
		return err
	}
	r.IssuedAt = s.now().UTC().Truncate(time.Second)
	year := yearIn(r.IssuedAt, issuer.Timezone)
	seq, err := s.store.nextSeq(ctx, tx, year)
	if err != nil {
		return err
	}
	r.Number = Number(year, seq)
	return s.store.insert(ctx, tx, &r)
}

// ForTrip returns a trip's receipt to its rider or driver (staff pass staff=true).
func (s *Service) ForTrip(ctx context.Context, tripID, userID string, staff bool) (*Receipt, error) {
	r, err := s.store.byTrip(ctx, tripID)
	if err != nil {
		return nil, err
	}
	if r == nil || (!staff && userID != r.riderID && userID != r.driverID) {
		return nil, ErrReceiptNotFound // a stranger learns nothing, not even that it exists
	}
	return r, nil
}

// Link is a signed address of the receipt's PDF in a language, valid for LinkTTL.
type Link struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

// LinkFor signs a PDF link for a receipt under the public API base URL.
func (s *Service) LinkFor(r *Receipt, lang, base string) Link {
	lang = langOf(lang)
	exp := s.now().Add(LinkTTL).Truncate(time.Second)
	q := url.Values{"lang": {lang}, "exp": {strconv.FormatInt(exp.Unix(), 10)}, "sig": {s.signer.sign(r.ID, lang, exp.Unix())}}
	return Link{URL: base + "/v1/receipts/" + r.ID + "/pdf?" + q.Encode(), ExpiresAt: exp.UTC()}
}

// PDF renders a receipt reached through a signed link.
func (s *Service) PDF(ctx context.Context, id, lang, exp, sig string) ([]byte, error) {
	if err := s.signer.verify(id, lang, exp, sig, s.now()); err != nil {
		return nil, err
	}
	r, err := s.store.byID(ctx, id)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, ErrLinkInvalid
	}
	return Render(r, lang)
}
