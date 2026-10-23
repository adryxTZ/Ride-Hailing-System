package engine

import (
	"encoding/json"

	"github.com/shopspring/decimal"
)

// PerKmMin is the classic taxi meter: base fare + per-km + per-minute, surged,
// floored at the minimum fare. Waiting at pickup/dropoff beyond grace is billed
// at the tariff's waiting rate like every engine.
type PerKmMin struct{}

func init() { Register(PerKmMin{}) }

func (PerKmMin) Key() string  { return "per_km_min" }
func (PerKmMin) Name() string { return "Per km + per minute (taxi meter)" }
func (PerKmMin) SettingsSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","required":["base_fare","per_km","per_min"],"properties":{
		"base_fare":{"type":"number","minimum":0,"x-ui":{"label":{"en":"Base fare","sw":"Nauli ya msingi"},"order":1}},
		"per_km":{"type":"number","minimum":0,"x-ui":{"label":{"en":"Per kilometre","sw":"Kwa kilomita"},"order":2}},
		"per_min":{"type":"number","minimum":0,"x-ui":{"label":{"en":"Per minute","sw":"Kwa dakika"},"order":3}}}}`)
}

func (e PerKmMin) routeFare(t Tariff, distanceM, durationS, surge decimal.Decimal) decimal.Decimal {
	km := distanceM.Div(decimal.NewFromInt(1000))
	minutes := durationS.Div(decimal.NewFromInt(60))
	fare := param(t, "base_fare", decimal.Zero).
		Add(param(t, "per_km", decimal.Zero).Mul(km)).
		Add(param(t, "per_min", decimal.Zero).Mul(minutes))
	fare = fare.Mul(effectiveSurge(t, surge))
	if t.MinimumFare.IsPositive() && fare.LessThan(t.MinimumFare) {
		fare = t.MinimumFare
	}
	return fare.Round(2)
}

func (e PerKmMin) Quote(t Tariff, in QuoteInput) (decimal.Decimal, []Line) {
	fare := e.routeFare(t, in.DistanceM, in.DurationS, in.Surge)
	unsurged := e.routeFare(t, in.DistanceM, in.DurationS, decimal.NewFromInt(1))
	lines := baseAndSurge(fare, unsurged, in.DistanceM, in.DurationS, effectiveSurge(t, in.Surge))
	return addOns(t, fare, lines, in.Loaders, in.BookingFee)
}

func (e PerKmMin) Bill(t Tariff, in BillInput) (decimal.Decimal, []Line) {
	fare := e.routeFare(t, in.DistanceM, in.DurationS, in.Surge)
	unsurged := e.routeFare(t, in.DistanceM, in.DurationS, decimal.NewFromInt(1))
	lines := baseAndSurge(fare, unsurged, in.DistanceM, in.DurationS, effectiveSurge(t, in.Surge))
	total := fare
	if w := waitingCharge(t, in); w.IsPositive() {
		lines = append(lines, Line{Code: LineWaiting, Amount: w})
		total = total.Add(w)
	}
	return addOns(t, total, lines, in.Loaders, in.BookingFee)
}

// baseAndSurge splits a surged fare into the BASE_FARE and SURGE lines.
func baseAndSurge(fare, unsurged, distanceM, durationS, surge decimal.Decimal) []Line {
	surgeAmount := decimal.Max(decimal.Zero, fare.Sub(unsurged))
	lines := []Line{{Code: LineBaseFare, Amount: fare.Sub(surgeAmount), Meta: map[string]any{
		"distance_m": distanceM.IntPart(), "duration_s": durationS.IntPart()}}}
	if surgeAmount.IsPositive() {
		lines = append(lines, Line{Code: LineSurge, Amount: surgeAmount, Meta: map[string]any{"multiplier": surge.String()}})
	}
	return lines
}

// waitingCharge bills time beyond the loading/unloading grace at the waiting rate.
func waitingCharge(t Tariff, in BillInput) decimal.Decimal {
	waitS := max(0, in.LoadingTakenS-t.LoadingGraceS) + max(0, in.UnloadingTakenS-t.UnloadingGraceS)
	return decimal.NewFromInt(int64(waitS)).Div(decimal.NewFromInt(60)).Mul(t.WaitingRatePerMin).Round(2)
}
