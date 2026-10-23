package engine

import (
	"encoding/json"

	"github.com/shopspring/decimal"
)

// FlatRate charges a fixed price per trip for the vehicle class regardless of
// distance (e.g. a flat in-town fare), optionally surged.
type FlatRate struct{}

func init() { Register(FlatRate{}) }

func (FlatRate) Key() string  { return "flat_rate" }
func (FlatRate) Name() string { return "Flat rate per trip" }
func (FlatRate) SettingsSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","required":["price"],"properties":{
		"price":{"type":"number","minimum":0,"x-ui":{"label":{"en":"Price per trip","sw":"Bei kwa safari"},"order":1}}}}`)
}

func (e FlatRate) fare(t Tariff, surge decimal.Decimal) decimal.Decimal {
	return param(t, "price", decimal.Zero).Mul(effectiveSurge(t, surge)).Round(2)
}

func (e FlatRate) Quote(t Tariff, in QuoteInput) (decimal.Decimal, []Line) {
	fare := e.fare(t, in.Surge)
	lines := baseAndSurge(fare, e.fare(t, decimal.NewFromInt(1)), in.DistanceM, in.DurationS, effectiveSurge(t, in.Surge))
	return addOns(t, fare, lines, in.Loaders, in.BookingFee)
}

func (e FlatRate) Bill(t Tariff, in BillInput) (decimal.Decimal, []Line) {
	fare := e.fare(t, in.Surge)
	lines := baseAndSurge(fare, e.fare(t, decimal.NewFromInt(1)), in.DistanceM, in.DurationS, effectiveSurge(t, in.Surge))
	total := fare
	if w := waitingCharge(t, in); w.IsPositive() {
		lines = append(lines, Line{Code: LineWaiting, Amount: w})
		total = total.Add(w)
	}
	return addOns(t, total, lines, in.Loaders, in.BookingFee)
}
