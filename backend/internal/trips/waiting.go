package trips

import (
	"encoding/json"
	"math"
	"time"

	"github.com/shopspring/decimal"
)

// Adjustment is a charge added to the quoted fare during the ride (e.g. waiting at the pickup).
type Adjustment struct {
	Code   string          `json:"code"`
	Amount decimal.Decimal `json:"amount"`
	Meta   map[string]any  `json:"meta,omitempty"`
}

// AdjustmentWaiting is the code of the pickup waiting fee line.
const AdjustmentWaiting = "WAITING"

// waitingTerms are the pickup waiting rules snapshotted on the quote option.
type waitingTerms struct {
	GraceS     int             `json:"waiting_grace_s"`
	RatePerMin decimal.Decimal `json:"waiting_rate_per_min"`
	Cap        decimal.Decimal `json:"waiting_cap"`
}

func termsOf(option json.RawMessage) waitingTerms {
	var w waitingTerms
	_ = json.Unmarshal(option, &w)
	return w
}

// waitingFee is the charge for waiting at the pickup from arrival to the start
// of the ride: every started minute beyond the free time, at the quoted rate,
// capped when the quote carries a cap. Whole currency units.
func waitingFee(w waitingTerms, arrived, started time.Time) (decimal.Decimal, int) {
	over := started.Sub(arrived) - time.Duration(w.GraceS)*time.Second
	if over <= 0 || !w.RatePerMin.IsPositive() {
		return decimal.Zero, 0
	}
	minutes := int(math.Ceil(over.Minutes()))
	fee := w.RatePerMin.Mul(decimal.NewFromInt(int64(minutes)))
	if w.Cap.IsPositive() && fee.GreaterThan(w.Cap) {
		fee = w.Cap
	}
	return fee.Round(0), minutes
}
