package payments

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/holds"
)

// Debts reads what a driver owes (the money ledger).
type Debts interface {
	DriverDebt(ctx context.Context, driverID string) (decimal.Decimal, string, error)
}

// Ledger posts a collected pay-in inside the caller's transaction.
type Ledger interface {
	PostPayinCollected(ctx context.Context, tx pgx.Tx, payinID, driverID string, amount decimal.Decimal, currency, gateway, providerRef string) (bool, error)
}

// Holds re-evaluates a driver's debt hold after money moved.
type Holds interface {
	Evaluate(ctx context.Context, driverID string) (holds.Decision, error)
}

// LimitSource returns the debt rules in force (the minimum pay-in lives there).
type LimitSource func(ctx context.Context) (holds.Limits, error)

// JobEnqueuer inserts jobs (the River client): in the caller's transaction, or on its own.
type JobEnqueuer interface {
	InsertTx(ctx context.Context, tx pgx.Tx, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
	Insert(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// Notifier tells the driver's app that its wallet changed. Nil = nobody to tell.
type Notifier func(ctx context.Context, driverID string)

// Settlement is a trip completed by a collected fare, as the ledger posts it.
type Settlement struct {
	TripID   string
	RiderID  string
	DriverID string
	Fees     []byte          // the fare split fixed when the ride ended
	Dues     decimal.Decimal // earlier cancellation fees included in the amount
	Currency string
}

// FareTrips is the trips module as fares need it: it records where a fare
// stands and completes the trip when the money arrives.
type FareTrips interface {
	FarePendingTx(ctx context.Context, tx pgx.Tx, tripID, payinID string, auto bool) error
	FareFailed(ctx context.Context, tripID, payinID, code string) error
	// CompletePaidTx completes the trip; nil when it no longer waited for this payment.
	CompletePaidTx(ctx context.Context, tx pgx.Tx, tripID, payinID string) (*Settlement, error)
	Notify(ctx context.Context, tripID string)
}

// FareLedger posts fare money inside the caller's transaction.
type FareLedger interface {
	PostTripPaidTx(ctx context.Context, tx pgx.Tx, s Settlement, collected decimal.Decimal) (bool, error)
	PostFareUnappliedTx(ctx context.Context, tx pgx.Tx, payinID, riderID string, amount decimal.Decimal, currency string) (bool, error)
	// WalletMoved tells wallet listeners after commit.
	WalletMoved(ctx context.Context, driverID string)
}
