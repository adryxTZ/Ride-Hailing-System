package trips

import "encoding/json"

// feesView keeps the parts of a fare split the apps show: what the rider paid
// for (booking fee, VAT, levy inside the total) and what the driver earns. The
// pricing profile, payees and platform net stay internal (staff see them on the
// trip's pricing record).
func feesView(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var v struct {
		Currency       string          `json:"currency"`
		Total          json.RawMessage `json:"total"`
		BookingFee     json.RawMessage `json:"booking_fee"`
		RideFare       json.RawMessage `json:"ride_fare"`
		Commission     json.RawMessage `json:"commission"`
		VAT            json.RawMessage `json:"vat"`
		Levy           json.RawMessage `json:"levy"`
		LevyBorneBy    string          `json:"levy_borne_by"`
		DriverEarnings json.RawMessage `json:"driver_earnings"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil
	}
	out, _ := json.Marshal(v)
	return out
}
