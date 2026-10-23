package holds

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/presence"
)

// Debts reads what a driver owes (the money ledger) on the given transaction,
// so the read happens under the evaluation lock without a second pool connection.
type Debts interface {
	DriverDebtIn(ctx context.Context, tx pgx.Tx, driverID string) (decimal.Decimal, string, error)
}

// LimitSource returns the debt rules of the fee policy in force (cached by pricing).
type LimitSource func(ctx context.Context) (Limits, error)

// Presence takes held drivers offline. Nil when presence is disabled.
type Presence interface {
	Get(ctx context.Context, driverID string) (*presence.Driver, error)
	Offline(ctx context.Context, driverID string) error
}

// Notifier tells the driver's app that its wallet or holds changed.
type Notifier func(ctx context.Context, driverID string)
