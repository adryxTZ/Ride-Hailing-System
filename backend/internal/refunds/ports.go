package refunds

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

// Ledger posts the refund in the refund's transaction.
type Ledger interface {
	PostRefundTx(ctx context.Context, tx pgx.Tx, refundID, riderID, driverID, bearer string, amount decimal.Decimal, currency string) (bool, error)
}

// Notify tells listeners after commit (the driver's wallet moved; the trip changed).
type Notify func(ctx context.Context, tripID, driverID string, bearer string)
