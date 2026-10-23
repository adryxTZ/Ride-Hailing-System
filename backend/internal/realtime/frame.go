package realtime

import (
	"encoding/json"
	"time"
)

// Frame types sent by the server. One type per meaning — no aliases.
const (
	TypeReady          = "ready"
	TypePong           = "pong"
	TypeTripUpdated    = "trip.updated"
	TypeOfferNew       = "offer.new"
	TypeOfferClosed    = "offer.closed"
	TypeDriverLocation = "driver.location"
	TypeWalletUpdated  = "wallet.updated" // a driver's balance, holds or pay-in changed
	TypeError          = "error"
)

// Frame is the envelope on the wire.
type Frame struct {
	Type string          `json:"type"`
	TS   time.Time       `json:"ts"`
	Data json.RawMessage `json:"data,omitempty"`
}

// NewFrame marshals data into a frame stamped now.
func NewFrame(typ string, data any) Frame {
	f := Frame{Type: typ, TS: time.Now().UTC()}
	if data != nil {
		f.Data, _ = json.Marshal(data)
	}
	return f
}

// clientFrame is what clients send.
type clientFrame struct {
	Type  string `json:"type"`
	Token string `json:"token,omitempty"`
}
