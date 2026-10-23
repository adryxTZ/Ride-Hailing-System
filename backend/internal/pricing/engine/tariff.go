package engine

import "github.com/shopspring/decimal"

// Bracket prices a range [Min, Max) of metres (distance) or seconds (duration).
// Max == nil means open-ended. UnitPrice is per km or per minute.
type Bracket struct {
	Min       decimal.Decimal
	Max       *decimal.Decimal
	UnitPrice decimal.Decimal
}

// Tariff is the fare configuration for one vehicle class. Engine picks the
// pricing module; Params holds that engine's numbers (see SettingsSchema);
// bracket engines read Distance/Duration instead.
type Tariff struct {
	Engine              string
	Params              map[string]decimal.Decimal
	Currency            string
	Distance            []Bracket // ordered by Min; the first one's UnitPrice is also the base fare
	Duration            []Bracket
	MinimumFare         decimal.Decimal // 0 = fall back to the base fare (reference-system semantics)
	WaitingRatePerMin   decimal.Decimal
	PickupApproachPerKm decimal.Decimal
	LoaderRate          decimal.Decimal // flat fee per loader (reference system: courier_hourly_rate × count, applied once)
	LoadingGraceS       int
	UnloadingGraceS     int
	SurgeEnabled        bool
	// CancellationFee is charged for a late cancel or a no-show (not part of the fare arithmetic).
	CancellationFee decimal.Decimal
}

// BasePrice is the first distance bracket's unit price (reference system: base_price).
func (t Tariff) BasePrice() decimal.Decimal {
	if len(t.Distance) == 0 {
		return decimal.Zero
	}
	return t.Distance[0].UnitPrice
}

// Floor is the minimum chargeable fare after surge.
func (t Tariff) Floor() decimal.Decimal {
	if t.MinimumFare.IsPositive() {
		return t.MinimumFare
	}
	return t.BasePrice()
}

// RateFor returns the unit price whose bracket contains value, or the last
// bracket's price when value is beyond every bracket (reference system: find_bracket_rate).
func RateFor(value decimal.Decimal, brackets []Bracket) decimal.Decimal {
	for _, b := range brackets {
		if value.GreaterThanOrEqual(b.Min) && (b.Max == nil || value.LessThan(*b.Max)) {
			return b.UnitPrice
		}
	}
	if len(brackets) == 0 {
		return decimal.Zero
	}
	return brackets[len(brackets)-1].UnitPrice
}
