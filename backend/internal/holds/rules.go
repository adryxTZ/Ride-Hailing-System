package holds

import (
	"time"

	"github.com/shopspring/decimal"
)

// day is the unit of the soft-limit grace period.
const day = 24 * time.Hour

// Limits are the brand's debt rules from the fee policy in force.
type Limits struct {
	Soft      decimal.Decimal `json:"soft_limit"`
	Hard      decimal.Decimal `json:"hard_limit"`
	GraceDays int             `json:"soft_grace_days"`
	MinPayin  decimal.Decimal `json:"min_payin"`
}

// Decision is what the debt rule says about one driver right now.
type Decision struct {
	Held        bool
	Cause       string          // HARD_LIMIT or SOFT_LIMIT_OVERDUE when held
	ClockStart  *time.Time      // when debt first reached the soft limit; nil below it
	HoldAt      *time.Time      // when the soft rule holds (or held) the driver; nil below it
	PayToUnhold decimal.Decimal // smallest whole payment that frees the driver; zero when not held
}

// Decide applies the debt rule. It is stateless: the only memory is the soft
// clock, which the caller keeps while debt stays at or above the soft limit (a
// partial payment that leaves the driver above it does not restart the grace days).
// Because the policy guarantees Soft ≥ MinPayin, nobody is held for less than they can pay.
func Decide(debt decimal.Decimal, l Limits, overSoftSince *time.Time, now time.Time) Decision {
	if debt.LessThan(l.Soft) {
		return Decision{}
	}
	clock := now
	if overSoftSince != nil {
		clock = *overSoftSince
	}
	holdAt := clock.Add(time.Duration(l.GraceDays) * day)
	d := Decision{ClockStart: &clock, HoldAt: &holdAt}
	overdue := !now.Before(holdAt)
	switch {
	case overdue:
		d.Held, d.Cause = true, CauseSoftLimitOverdue
		d.PayToUnhold = payBelow(debt, l.Soft, l.MinPayin)
	case debt.GreaterThanOrEqual(l.Hard):
		d.Held, d.Cause = true, CauseHardLimit
		d.PayToUnhold = payBelow(debt, l.Hard, l.MinPayin)
	}
	return d
}

// payBelow is the smallest whole amount that brings debt strictly under limit,
// kept between the minimum payment and the whole (rounded-up) debt.
func payBelow(debt, limit, min decimal.Decimal) decimal.Decimal {
	pay := debt.Sub(limit).Floor().Add(decimal.NewFromInt(1))
	if pay.LessThan(min) {
		pay = min
	}
	if all := debt.Ceil(); pay.GreaterThan(all) {
		pay = all
	}
	return pay
}
