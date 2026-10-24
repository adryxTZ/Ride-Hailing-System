package dispatch

import (
	"net/http"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
)

const (
	TickInterval  = time.Second
	OfferTTL      = 20 * time.Second
	BatchLimit    = 50              // waiting trips considered per tick
	CandidateCap  = 10              // nearest drivers kept per trip
	leaderTTL     = 3 * time.Second // Redis lock so one process dispatches per brand
	cityKmh       = 18.0            // pickup ETA until routed ETAs arrive
	roadFactor    = 1.3             // straight line → road distance
	maxPickupKm   = 6.0             // never offer a pickup further than this (v0, per-service later)
	minEtaSeconds = 60
)

// ringSteps widens the H3 search until drivers are found (res 9 ≈ 0.35 km per ring).
var ringSteps = []int{2, 4, 8, 16}

// Offer is what a driver sees and answers.
type Offer struct {
	TripID         string          `json:"trip_id"`
	ExpiresAt      time.Time       `json:"expires_at"`
	TTLSeconds     int             `json:"ttl_s"`
	Service        string          `json:"service"`
	VehicleClass   string          `json:"vehicle_class"`
	Pickup         OfferStop       `json:"pickup"`
	Dropoff        *OfferStop      `json:"dropoff,omitempty"`
	Fare           decimal.Decimal `json:"fare"`
	Currency       string          `json:"currency"`
	PaymentMethod  string          `json:"payment_method"`
	RiderName      *string         `json:"rider_name,omitempty"`
	PickupDistance int             `json:"pickup_distance_m"`
	PickupEtaS     int             `json:"pickup_eta_s"`
	TripDistanceM  *int            `json:"trip_distance_m,omitempty"`
	StopsCount     int             `json:"stops_count"` // stops between pickup and drop-off
	// Queued: the driver is finishing a trip; this one starts after it ends
	// (the app labels it "Next trip"). PickupEtaS includes the finishing time.
	Queued bool `json:"queued"`
}

type OfferStop struct {
	Location routing.Point `json:"location"`
	Address  *string       `json:"address,omitempty"`
}

var (
	ErrOfferNotFound = apperr.New("OFFER_NOT_FOUND", http.StatusNotFound, "")
	ErrOfferExpired  = apperr.New("OFFER_EXPIRED", http.StatusConflict, "")
)

// etaSeconds is the v0 pickup estimate from straight-line distance.
func etaSeconds(distanceM float64) int {
	s := int(distanceM * roadFactor / 1000 / cityKmh * 3600)
	if s < minEtaSeconds {
		return minEtaSeconds
	}
	return s
}
