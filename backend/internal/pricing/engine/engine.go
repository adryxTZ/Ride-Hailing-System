package engine

import (
	"encoding/json"
	"sort"

	"github.com/shopspring/decimal"
)

// Engine is one pricing module. The admin chooses an engine per tariff and
// fills in the parameters its SettingsSchema describes; the quote and bill
// paths never know which engine is behind a tariff.
type Engine interface {
	Key() string
	Name() string
	// SettingsSchema is a JSON Schema (with x-ui hints) for Tariff.Params, so
	// the admin can render a form for this engine without engine-specific code.
	SettingsSchema() json.RawMessage
	Quote(t Tariff, in QuoteInput) (total decimal.Decimal, lines []Line)
	Bill(t Tariff, in BillInput) (total decimal.Decimal, lines []Line)
}

// Description is what the admin lists.
type Description struct {
	Key            string          `json:"key"`
	Name           string          `json:"name"`
	SettingsSchema json.RawMessage `json:"settings_schema"`
}

var registry = map[string]Engine{}

// Register adds an engine; called from each engine file's init.
func Register(e Engine) { registry[e.Key()] = e }

// Get returns the engine for a key, or nil.
func Get(key string) Engine { return registry[key] }

// DefaultKey is the engine used when a tariff names none (the reference-system port).
const DefaultKey = "bracket_meter"

// Describe lists every registered engine, sorted by key.
func Describe() []Description {
	out := make([]Description, 0, len(registry))
	for _, e := range registry {
		out = append(out, Description{Key: e.Key(), Name: e.Name(), SettingsSchema: e.SettingsSchema()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// param reads a decimal parameter with a default.
func param(t Tariff, key string, def decimal.Decimal) decimal.Decimal {
	if v, ok := t.Params[key]; ok {
		return v
	}
	return def
}

// addOns appends loaders and booking fee the same way for every engine, so a
// tariff switching engines never changes how extras are charged.
func addOns(t Tariff, total decimal.Decimal, lines []Line, loaders int, bookingFee decimal.Decimal) (decimal.Decimal, []Line) {
	if lc := LoaderCost(t, loaders); lc.IsPositive() {
		lines = append(lines, Line{Code: LineLoaders, Amount: lc, Meta: map[string]any{"count": loaders}})
		total = total.Add(lc)
	}
	if bookingFee.IsPositive() {
		lines = append(lines, Line{Code: LineBookingFee, Amount: bookingFee})
		total = total.Add(bookingFee)
	}
	return total.Round(2), lines
}
