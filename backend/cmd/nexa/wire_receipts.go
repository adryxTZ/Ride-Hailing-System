package main

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/fleet"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/auth"
	"github.com/kalen-limited/hauler-rider/backend/internal/receipts"
	"github.com/kalen-limited/hauler-rider/backend/internal/trips"
)

// receiptLinkPurpose names the secret that signs receipt PDF links.
const receiptLinkPurpose = "receipt-links"

// startReceipts issues a receipt inside every completing transaction, so a
// completed trip never exists without one.
func startReceipts(bs *bootstrap, brandID string, keys *auth.KeySet, t *trips.Service, f *fleet.Service) *receipts.Service {
	svc := receipts.New(receipts.Deps{Pool: bs.pool, BrandID: brandID, LinkKey: keys.Derive(receiptLinkPurpose), Issuer: issuerFor(bs)})
	t.OnCompletedTx(func(ctx context.Context, tx pgx.Tx, trip *trips.Trip) error {
		return svc.IssueTx(ctx, tx, receipts.FactsFromTrip(trip, plateOf(ctx, f, trip)))
	})
	return svc
}

// plateOf is the driver's vehicle plate for the receipt ("" when unknown).
func plateOf(ctx context.Context, f *fleet.Service, t *trips.Trip) string {
	if t.DriverID == nil {
		return ""
	}
	v, err := f.VehicleOf(ctx, *t.DriverID)
	if err != nil || v == nil {
		slog.Warn("receipts: vehicle not found for the receipt", "trip", t.ID, "err", err)
		return ""
	}
	return v.Plate
}

// issuerFor is the brand as receipt issuer in a country (its timezone numbers the year).
func issuerFor(bs *bootstrap) func(country string) receipts.Issuer {
	return func(country string) receipts.Issuer {
		tz := bs.brand.Timezone
		if c := bs.brand.CountryByCode(country); c != nil {
			tz = c.Timezone
		}
		return receipts.Issuer{Name: bs.brand.DisplayName, LegalEntity: bs.brand.LegalEntity, SupportPhone: bs.brand.SupportPhone,
			SupportEmail: bs.brand.SupportEmail, Country: country, Timezone: tz}
	}
}
