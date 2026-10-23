package main

import (
	"context"
	"fmt"

	"github.com/kalen-limited/hauler-rider/backend/internal/money"
)

// runVerifyLedger recomputes the ledger hash chain and fails on the first broken entry.
func runVerifyLedger() error {
	ctx := context.Background()
	bs, err := load(ctx, true)
	if err != nil {
		return err
	}
	defer bs.pool.Close()
	brandID, err := bs.brandID(ctx)
	if err != nil {
		return err
	}
	broken, checked, err := money.NewLedger(bs.pool, brandID).Verify(ctx)
	if err != nil {
		return err
	}
	if broken != 0 {
		return fmt.Errorf("ledger chain BROKEN at entry %d (after %d good entries)", broken, checked)
	}
	fmt.Printf("ledger chain OK: %d entries verified\n", checked)
	return nil
}
