package engine

import (
	"encoding/json"

	"github.com/shopspring/decimal"
)

// BracketMeter is the reference system's V1 port: tiered distance/duration brackets, surge,
// floor, waiting, loaders, approach fee, booking fee. Parameters live in the
// tariff's bracket tables rather than Params.
type BracketMeter struct{}

func init() { Register(BracketMeter{}) }

func (BracketMeter) Key() string  { return DefaultKey }
func (BracketMeter) Name() string { return "Bracket meter (distance/duration tiers)" }
func (BracketMeter) SettingsSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{},"x-ui":{"uses":"brackets","note":"Rates come from the distance and duration bracket tables of the tariff."}}`)
}
func (BracketMeter) Quote(t Tariff, in QuoteInput) (decimal.Decimal, []Line) { return Quote(t, in) }
func (BracketMeter) Bill(t Tariff, in BillInput) (decimal.Decimal, []Line)   { return Bill(t, in) }
