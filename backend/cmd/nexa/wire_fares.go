package main

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/money"
	"github.com/kalen-limited/hauler-rider/backend/internal/payments"
	"github.com/kalen-limited/hauler-rider/backend/internal/trips"
)

// fareTrips adapts the trips service to the payments module's FareTrips port.
type fareTrips struct{ t *trips.Service }

func (a fareTrips) FarePendingTx(ctx context.Context, tx pgx.Tx, tripID, payinID string, auto bool) error {
	return a.t.FarePendingTx(ctx, tx, tripID, payinID, auto)
}

func (a fareTrips) FareFailed(ctx context.Context, tripID, payinID, code string) error {
	return a.t.FareFailed(ctx, tripID, payinID, code)
}

func (a fareTrips) CompletePaidTx(ctx context.Context, tx pgx.Tx, tripID, payinID string) (*payments.Settlement, error) {
	s, err := a.t.CompletePaidTx(ctx, tx, tripID, payinID)
	if err != nil || s == nil {
		return nil, err
	}
	return &payments.Settlement{TripID: s.TripID, RiderID: s.RiderID, DriverID: s.DriverID, Fees: s.Fees, Dues: s.Dues, Currency: s.Currency}, nil
}

func (a fareTrips) Notify(ctx context.Context, tripID string) { a.t.Notify(ctx, tripID) }

// fareLedger adapts the money ledger to the payments module's FareLedger port.
type fareLedger struct{ l *money.Ledger }

func (a fareLedger) PostTripPaidTx(ctx context.Context, tx pgx.Tx, s payments.Settlement, collected decimal.Decimal) (bool, error) {
	return a.l.PostTripPaidTx(ctx, tx, money.DigitalFare{TripID: s.TripID, RiderID: s.RiderID, DriverID: s.DriverID,
		Fees: s.Fees, Dues: s.Dues, Currency: s.Currency, Collected: collected})
}

func (a fareLedger) PostFareUnappliedTx(ctx context.Context, tx pgx.Tx, payinID, riderID string, amount decimal.Decimal, currency string) (bool, error) {
	return a.l.PostFareUnappliedTx(ctx, tx, payinID, riderID, amount, currency)
}

func (a fareLedger) WalletMoved(ctx context.Context, driverID string) { a.l.WalletMoved(ctx, driverID) }

// wireFares connects ride ends to fare collection: a mobile-money ride prompts
// the rider automatically when it ends (a fare under the gateway minimum falls
// back to cash), and trip reads carry the latest prompt.
func wireFares(t *trips.Service, p *payments.Service) {
	t.SetPromptSource(func(ctx context.Context, tripID string) (any, error) {
		st, err := p.FareStateOf(ctx, tripID)
		if err != nil || st == nil {
			return nil, err // a typed nil would show as an empty prompt
		}
		return st, nil
	})
	t.OnFareDue(func(ctx context.Context, trip *trips.Trip) {
		if trip.PaymentMethod != trips.MethodMobileMoney {
			return
		}
		due := payments.Fare{TripID: trip.ID, RiderID: trip.RiderID, Phone: trip.Rider.Phone, Amount: trip.Payment.Amount, Currency: trip.Currency}
		p.AutoPrompt(ctx, due, func(ctx context.Context) {
			_ = t.RecordEvent(ctx, trip.ID, "payment.cash_fallback", "SYSTEM", nil, map[string]any{"reason": "below_gateway_minimum"})
		})
	})
}
