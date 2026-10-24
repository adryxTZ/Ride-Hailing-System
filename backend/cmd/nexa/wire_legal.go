package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"

	"github.com/jackc/pgx/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/branding"
	"github.com/kalen-limited/hauler-rider/backend/internal/legal"
	"github.com/kalen-limited/hauler-rider/backend/internal/privacy"
)

// startLegal wires the legal texts and seeds what a brand starts with: the
// starter pack when the brand has no texts yet (consent versions link to it),
// then the brand file's notices for an app that still has no consent version.
func startLegal(ctx context.Context, bs *bootstrap, brandID string, p *privacy.Service, b *branding.Service) (*legal.Service, error) {
	svc := newLegal(bs, brandID, p, b)
	if err := seedLegal(ctx, bs, svc); err != nil {
		return nil, err
	}
	if err := p.SeedDocuments(ctx); err != nil {
		return nil, fmt.Errorf("consent documents: %w", err)
	}
	return svc, nil
}

// newLegal builds the service; b nil = placeholders read the brand file's identity.
func newLegal(bs *bootstrap, brandID string, p *privacy.Service, b *branding.Service) *legal.Service {
	d := legal.Deps{Pool: bs.pool, BrandID: brandID, Brand: bs.brand, Consent: consentRecord{p: p}}
	if b != nil {
		d.Identity = brandingIdentity(b)
	}
	return legal.New(d)
}

// seedLegal publishes seed/legal when the brand has no texts (a missing folder only warns).
func seedLegal(ctx context.Context, bs *bootstrap, svc *legal.Service) error {
	pack, err := legal.LoadStarter(filepath.Join(bs.cfg.SeedDir, "legal"))
	if errors.Is(err, fs.ErrNotExist) {
		slog.Warn("no legal starter pack; brands without texts keep the brand file's consent notice", "dir", bs.cfg.SeedDir)
		return nil
	}
	if err != nil {
		return fmt.Errorf("legal starter pack: %w", err)
	}
	n, err := svc.SeedStarter(ctx, pack)
	if err != nil {
		return fmt.Errorf("legal starter pack: %w", err)
	}
	if n > 0 {
		slog.Info("legal starter pack published", "documents", n)
	}
	return nil
}

// brandingIdentity reads the placeholders' brand values from the live identity (cached).
func brandingIdentity(b *branding.Service) legal.IdentitySource {
	return func(ctx context.Context) (legal.Identity, error) {
		live, err := b.Published(ctx)
		if err != nil {
			return legal.Identity{}, err
		}
		id := live.Identity
		return legal.Identity{Name: id.Name, LegalEntity: id.LegalEntity, SupportEmail: id.SupportEmail, SupportPhone: id.SupportPhone,
			EmergencyPhone: id.EmergencyPhone}, nil
	}
}

// consentRecord adapts the privacy module's consent versions to the legal module's port.
type consentRecord struct{ p *privacy.Service }

func (c consentRecord) NewVersionTx(ctx context.Context, tx pgx.Tx, v legal.ConsentVersion) (int, error) {
	return c.p.NewVersionTx(ctx, tx, toNotice(v))
}

func (c consentRecord) SeedVersionTx(ctx context.Context, tx pgx.Tx, v legal.ConsentVersion) (int, error) {
	return c.p.SeedVersionTx(ctx, tx, toNotice(v))
}

func toNotice(v legal.ConsentVersion) privacy.NewNotice {
	n := privacy.NewNotice{App: v.App, Title: v.Title, Summary: v.Summary, URL: v.URL, Documents: v.Documents, Links: []privacy.NoticeLink{}}
	for _, l := range v.Links {
		n.Links = append(n.Links, privacy.NoticeLink{Kind: l.Kind, Title: l.Title, URL: l.URL})
	}
	return n
}
