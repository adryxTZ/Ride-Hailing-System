package engine

import "github.com/shopspring/decimal"

// Line codes on quotes and receipts (EN/SW labels live in the i18n catalogue).
const (
	LineBaseFare              = "BASE_FARE"
	LineSurge                 = "SURGE"
	LineWaiting               = "WAITING"
	LineLoaders               = "LOADERS"
	LinePickupApproach        = "PICKUP_APPROACH"
	LineMinimumFareAdjustment = "MINIMUM_FARE_ADJUSTMENT"
	LineBookingFee            = "BOOKING_FEE"
)

// Line is one component of a fare.
type Line struct {
	Code   string          `json:"code"`
	Amount decimal.Decimal `json:"amount"`
	Meta   map[string]any  `json:"meta,omitempty"`
	// Label names lines whose wording is data (extra charges): {en, sw}.
	Label map[string]string `json:"label,omitempty"`
}

// RouteFare is the metered fare before add-ons (reference system: calculate_fare).
//
//	fare = base_price + rate(distance)·km + rate(duration)·min
//	fare = fare × surge (surge ≤ 0 → 1)   [skipped when the tariff opts out]
//	fare = max(fare, floor)
//	rounded half-up to 2 dp
func RouteFare(t Tariff, distanceM, durationS, surge decimal.Decimal) decimal.Decimal {
	km := distanceM.Div(decimal.NewFromInt(1000))
	minutes := durationS.Div(decimal.NewFromInt(60))
	fare := t.BasePrice().
		Add(RateFor(distanceM, t.Distance).Mul(km)).
		Add(RateFor(durationS, t.Duration).Mul(minutes))
	fare = fare.Mul(effectiveSurge(t, surge))
	if floor := t.Floor(); fare.LessThan(floor) {
		fare = floor
	}
	return fare.Round(2)
}

// QuoteInput is what a prospective trip is priced on.
type QuoteInput struct {
	DistanceM  decimal.Decimal
	DurationS  decimal.Decimal
	Surge      decimal.Decimal // locked at quote time; 1 = none
	Loaders    int
	BookingFee decimal.Decimal
}

// Quote prices a prospective trip: route fare (floored) + loaders + booking fee.
// Loaders are a flat fee per loader (reference-system semantics).
func Quote(t Tariff, in QuoteInput) (total decimal.Decimal, lines []Line) {
	surge := effectiveSurge(t, in.Surge)
	fare := RouteFare(t, in.DistanceM, in.DurationS, surge)
	unsurged := RouteFare(t, in.DistanceM, in.DurationS, decimal.NewFromInt(1))
	surgeAmount := decimal.Max(decimal.Zero, fare.Sub(unsurged))
	lines = append(lines, Line{Code: LineBaseFare, Amount: fare.Sub(surgeAmount), Meta: map[string]any{
		"distance_m": in.DistanceM.IntPart(), "duration_s": in.DurationS.IntPart()}})
	if surgeAmount.IsPositive() {
		lines = append(lines, Line{Code: LineSurge, Amount: surgeAmount, Meta: map[string]any{"multiplier": surge.String()}})
	}
	total = fare
	if loaders := LoaderCost(t, in.Loaders); loaders.IsPositive() {
		lines = append(lines, Line{Code: LineLoaders, Amount: loaders, Meta: map[string]any{"count": in.Loaders}})
		total = total.Add(loaders)
	}
	if in.BookingFee.IsPositive() {
		lines = append(lines, Line{Code: LineBookingFee, Amount: in.BookingFee})
		total = total.Add(in.BookingFee)
	}
	return total.Round(2), lines
}

// LoaderCost = loaders × flat loader rate (reference system: courier_hourly_rate × number_of_carriers).
func LoaderCost(t Tariff, loaders int) decimal.Decimal {
	if loaders <= 0 || !t.LoaderRate.IsPositive() {
		return decimal.Zero
	}
	return t.LoaderRate.Mul(decimal.NewFromInt(int64(loaders))).Round(2)
}

// BillInput is what a completed trip is billed on.
type BillInput struct {
	DistanceM       decimal.Decimal // route estimate or GPS-driven, already resolved by the caller
	DurationS       decimal.Decimal
	Surge           decimal.Decimal // the multiplier locked at quote time
	Loaders         int
	LoadingTakenS   int // measured server-side
	UnloadingTakenS int
	ApproachM       decimal.Decimal // driver's approach distance to pickup
	ApproachApplies bool            // false when billing GPS-covered distance (reference-system rule)
	BookingFee      decimal.Decimal
}

// Bill reproduces the reference system's calculate_final_payment for METERED trips:
//
//	trip_fare = RouteFare(surged, floored)
//	waiting   = max(0, loading − grace) + max(0, unloading − grace), per minute
//	total     = trip_fare + waiting
//	adjust    = max(0, floor − total)        → MINIMUM_FARE_ADJUSTMENT line
//	final     = max(total, floor) + loaders + approach_fee + booking_fee
func Bill(t Tariff, in BillInput) (total decimal.Decimal, lines []Line) {
	surge := effectiveSurge(t, in.Surge)
	tripFare := RouteFare(t, in.DistanceM, in.DurationS, surge)
	unsurged := RouteFare(t, in.DistanceM, in.DurationS, decimal.NewFromInt(1))
	surgeAmount := decimal.Max(decimal.Zero, tripFare.Sub(unsurged))

	waitS := max(0, in.LoadingTakenS-t.LoadingGraceS) + max(0, in.UnloadingTakenS-t.UnloadingGraceS)
	waitMin := decimal.NewFromInt(int64(waitS)).Div(decimal.NewFromInt(60))
	waiting := waitMin.Mul(t.WaitingRatePerMin).Round(2)

	subtotal := tripFare.Add(waiting)
	floor := t.Floor()
	adjustment := decimal.Max(decimal.Zero, floor.Sub(subtotal)).Round(2)
	final := decimal.Max(subtotal, floor)

	lines = append(lines, Line{Code: LineBaseFare, Amount: tripFare.Sub(surgeAmount), Meta: map[string]any{
		"distance_m": in.DistanceM.IntPart(), "duration_s": in.DurationS.IntPart()}})
	if surgeAmount.IsPositive() {
		lines = append(lines, Line{Code: LineSurge, Amount: surgeAmount, Meta: map[string]any{"multiplier": surge.String()}})
	}
	if waiting.IsPositive() {
		lines = append(lines, Line{Code: LineWaiting, Amount: waiting, Meta: map[string]any{"minutes": waitMin.Round(1).String()}})
	}
	if adjustment.IsPositive() {
		lines = append(lines, Line{Code: LineMinimumFareAdjustment, Amount: adjustment})
	}
	if loaders := LoaderCost(t, in.Loaders); loaders.IsPositive() {
		lines = append(lines, Line{Code: LineLoaders, Amount: loaders, Meta: map[string]any{"count": in.Loaders}})
		final = final.Add(loaders)
	}
	if in.ApproachApplies && t.PickupApproachPerKm.IsPositive() && in.ApproachM.IsPositive() {
		fee := t.PickupApproachPerKm.Mul(in.ApproachM.Div(decimal.NewFromInt(1000))).Round(2)
		lines = append(lines, Line{Code: LinePickupApproach, Amount: fee, Meta: map[string]any{"approach_m": in.ApproachM.IntPart()}})
		final = final.Add(fee)
	}
	if in.BookingFee.IsPositive() {
		lines = append(lines, Line{Code: LineBookingFee, Amount: in.BookingFee})
		final = final.Add(in.BookingFee)
	}
	return final.Round(2), lines
}

// effectiveSurge applies the reference system's guards: non-positive → 1; tariff opt-out → 1;
// and the tariff's cap (param "surge_max", e.g. 1.08 to stay inside a regulated band).
func effectiveSurge(t Tariff, surge decimal.Decimal) decimal.Decimal {
	one := decimal.NewFromInt(1)
	if !t.SurgeEnabled || !surge.IsPositive() {
		return one
	}
	if limit, ok := t.Params["surge_max"]; ok && limit.IsPositive() && surge.GreaterThan(limit) {
		return decimal.Max(one, limit)
	}
	return surge
}
