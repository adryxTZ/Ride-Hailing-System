package accounts

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/money"
)

// Ledger is what recording a payment needs from the ledger: its writer lock
// (so the owed balance cannot move between the check and the posting) and the
// balanced settlement posting.
type Ledger interface {
	LockTx(ctx context.Context, tx pgx.Tx) error
	PostSettlementTx(ctx context.Context, tx pgx.Tx, settlementID, memo, currency string, paid []money.Payable) (bool, error)
}
