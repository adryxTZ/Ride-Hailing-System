package privacy

import (
	"context"

	"github.com/shopspring/decimal"
)

// Money answers the deletion blockers that live in the ledger.
type Money interface {
	DriverDebt(ctx context.Context, driverID string) (decimal.Decimal, string, error)
	DriverOwed(ctx context.Context, driverID string) (decimal.Decimal, string, error)
	RiderDues(ctx context.Context, riderID string) (decimal.Decimal, error)
}

// Notify tells a person (SMS) that their export is ready.
type Notify func(ctx context.Context, phone, body string) error

// Offline takes a deleted driver out of presence.
type Offline func(ctx context.Context, driverID string) error
