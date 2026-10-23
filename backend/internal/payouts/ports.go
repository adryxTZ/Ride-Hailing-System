package payouts

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/shopspring/decimal"
)

// Ledger is the money ledger as payouts need it.
type Ledger interface {
	DriverOwed(ctx context.Context, driverID string) (decimal.Decimal, string, error)
	DriverOwedIn(ctx context.Context, tx pgx.Tx, driverID string) (decimal.Decimal, string, error)
	PostPayoutPaidTx(ctx context.Context, tx pgx.Tx, payoutID, driverID string, amount decimal.Decimal, currency, ref string) (bool, error)
	// WalletMoved tells wallet listeners after commit.
	WalletMoved(ctx context.Context, driverID string)
}

// Rules are the payout rules in force (from the fee policy).
type Rules struct {
	Min         decimal.Decimal
	AutoApprove bool
}

// RuleSource returns the rules in force.
type RuleSource func(ctx context.Context) (Rules, error)

// Blocked reports whether a staff hold stops the driver cashing out.
type Blocked func(ctx context.Context, driverID string) (bool, error)

// Person returns a driver's account phone and name (payouts go to the account phone only).
type Person func(ctx context.Context, driverID string) (phone, name string, err error)

// JobEnqueuer inserts jobs (the River client): in the caller's transaction, or on its own.
type JobEnqueuer interface {
	InsertTx(ctx context.Context, tx pgx.Tx, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
	Insert(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// Notifier tells the driver's app that its wallet changed. Nil = nobody to tell.
type Notifier func(ctx context.Context, driverID string)
