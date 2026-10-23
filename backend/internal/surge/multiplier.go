package surge

import "github.com/shopspring/decimal"

// Tuning: an area surges only with at least MinDemand waiting requests; each
// request per free driver above 1 adds Slope to the multiplier, in Step
// increments, up to MaxMultiplier.
const (
	MinDemand     = 2
	Slope         = 0.25
	Step          = 0.05
	MaxMultiplier = 2.0
)

// multiplierFor turns waiting requests and free drivers in an area into a multiplier ≥ 1.
func multiplierFor(demand, supply int) decimal.Decimal {
	one := decimal.NewFromInt(1)
	if demand < MinDemand {
		return one
	}
	ratio := float64(demand) / float64(max(supply, 1))
	if ratio <= 1 {
		return one
	}
	m := min(1+(ratio-1)*Slope, MaxMultiplier)
	steps := decimal.NewFromFloat(m - 1).Div(decimal.NewFromFloat(Step)).Floor()
	return one.Add(steps.Mul(decimal.NewFromFloat(Step)))
}
