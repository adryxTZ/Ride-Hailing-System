package main

import (
	"context"
	"log/slog"

	"github.com/kalen-limited/hauler-rider/backend/internal/money"
	"github.com/kalen-limited/hauler-rider/backend/internal/trips"
)

// wireCancellationMoney carries cancellation fees into the ledger and onto the
// rider's next trip: a fee is posted when a trip ends with one, earlier dues are
// added to new bookings and settled when that trip completes.
func wireCancellationMoney(t *trips.Service, l *money.Ledger) {
	t.SetDues(l.RiderDues)
	t.OnChanged(func(ctx context.Context, trip *trips.Trip) {
		if trip.CancellationFee != nil && trip.CancellationFee.IsPositive() {
			driver := ""
			if trip.DriverID != nil {
				driver = *trip.DriverID
			}
			if _, err := l.PostCancellationFee(ctx, trip.ID, trip.RiderID, driver, *trip.CancellationFee, trip.CancelShareOf(), trip.Currency); err != nil {
				slog.Warn("ledger: cancellation fee not posted", "trip", trip.ID, "err", err)
			}
		}
		// Dues paid by mobile money were cleared in the trip's own posting.
		if trip.Status == trips.Completed && trip.DriverID != nil && trip.DuesIn().IsPositive() && paidInCash(trip) {
			if _, err := l.PostDuesCollected(ctx, trip.ID, trip.RiderID, *trip.DriverID, trip.DuesIn(), trip.Currency); err != nil {
				slog.Warn("ledger: dues not settled", "trip", trip.ID, "err", err)
			}
		}
	})
}
