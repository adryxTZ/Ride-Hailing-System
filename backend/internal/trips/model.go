package trips

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
)

type Status string

const (
	Scheduled         Status = "SCHEDULED" // booked ahead; matching starts ScheduleDispatchLead before pickup
	Requested         Status = "REQUESTED"
	Offered           Status = "OFFERED"
	Accepted          Status = "ACCEPTED"
	EnRouteToPickup   Status = "EN_ROUTE_TO_PICKUP"
	ArrivedAtPickup   Status = "ARRIVED_AT_PICKUP"
	InTransit         Status = "IN_TRANSIT"
	ArrivedAtDropoff  Status = "ARRIVED_AT_DROPOFF"
	AwaitingPayment   Status = "AWAITING_PAYMENT"
	Completed         Status = "COMPLETED"
	CancelledByRider  Status = "CANCELLED_BY_RIDER"
	CancelledByDriver Status = "CANCELLED_BY_DRIVER"
	NoDriverFound     Status = "NO_DRIVER_FOUND"
	Expired           Status = "EXPIRED"
)

// Booking ahead: how far ahead a ride may be scheduled, and when matching starts.
const (
	ScheduleMinLead      = 30 * time.Minute
	ScheduleMaxAhead     = 7 * 24 * time.Hour
	ScheduleDispatchLead = 15 * time.Minute
)

// Active statuses block a second booking for the same rider.
var ActiveStatuses = []Status{Requested, Offered, Accepted, EnRouteToPickup, ArrivedAtPickup, InTransit, ArrivedAtDropoff, AwaitingPayment}

// Lifecycle is the service's profile snapshot a trip carries.
type Lifecycle struct {
	Profile       string `json:"profile"` // pickup_dropoff
	PaymentAtEnd  bool   `json:"payment_at_end"`
	ArrivalRadius int    `json:"arrival_radius_m"`
	// PickupContact "when_not_here": the app asks who is riding (or handing over the
	// package) when the pickup is not where the requester stands.
	PickupContact string `json:"pickup_contact,omitempty"`
	// DropoffContact "required": package delivery needs the receiver's name and phone.
	DropoffContact string `json:"dropoff_contact,omitempty"`
}

type Stop struct {
	Seq       int           `json:"seq"`
	Kind      string        `json:"kind"`
	Location  routing.Point `json:"location"`
	Address   *string       `json:"address,omitempty"`
	ArrivedAt *time.Time    `json:"arrived_at,omitempty"`
}

type Trip struct {
	ID             string                `json:"id"`
	Code           string                `json:"code"`
	Country        string                `json:"country"`
	Area           *AreaRef              `json:"area"`
	ServiceKey     string                `json:"service"`
	VehicleClass   string                `json:"vehicle_class"`
	RiderID        string                `json:"rider_id"`
	DriverID       *string               `json:"driver_id,omitempty"`
	Status         Status                `json:"status"`
	Lifecycle      Lifecycle             `json:"lifecycle"`
	Attributes     json.RawMessage       `json:"attributes"`
	QuoteOption    json.RawMessage       `json:"quote_option"`
	PricingSource  string                `json:"pricing_source"`
	EstimatedFare  decimal.Decimal       `json:"estimated_fare"`
	FinalFare      *decimal.Decimal      `json:"final_fare,omitempty"`
	Currency       string                `json:"currency"`
	PaymentMethod  string                `json:"payment_method"`
	RouteDistanceM *int                  `json:"route_distance_m,omitempty"`
	RouteDurationS *int                  `json:"route_duration_s,omitempty"`
	Notes          *string               `json:"notes,omitempty"`
	Stops          []Stop                `json:"stops"`
	Seq            int                   `json:"seq"`
	AllowedNext    []Status              `json:"allowed_transitions"`
	RequestedAt    time.Time             `json:"requested_at"`
	ScheduledFor   *time.Time            `json:"scheduled_for,omitempty"`
	Timestamps     map[string]*time.Time `json:"timestamps"`
	CancelReason   *string               `json:"cancel_reason,omitempty"`
	// Contacts: the passenger or sender at the pickup, the receiver at the drop-off.
	Contacts Contacts `json:"contacts"`
	// CancellationFee is what a late cancel or no-show cost on this trip.
	CancellationFee *decimal.Decimal `json:"cancellation_fee,omitempty"`
	// CancelFeeNow is what the rider would pay to cancel right now (computed on read).
	CancelFeeNow decimal.Decimal `json:"cancel_fee_now"`
	// Adjustments are charges added during the ride (pickup waiting), included in FinalFare.
	Adjustments []Adjustment `json:"adjustments"`
	// Fees is the full fare split (money.Breakdown), internal: it names the
	// pricing profile, payees and platform shares. Apps and staff see FeesView.
	Fees json.RawMessage `json:"-"`
	// FeesView is the split as the apps show it (booking fee, VAT, levy, the
	// driver's earnings and the commission); filled from Fees on read.
	FeesView json.RawMessage `json:"fees,omitempty"`
	Rider    *Party          `json:"rider,omitempty"`
	Driver   *Party          `json:"driver,omitempty"`
	// PaymentStatus is where the fare stands once the ride ended (nil before).
	PaymentStatus *string `json:"payment_status,omitempty"`
	// PaymentRef is the gateway row collecting the fare (internal).
	PaymentRef *string `json:"-"`
	// Payment is the fare as both apps show it (filled on read; the API layer adds the open prompt).
	Payment    *TripPayment `json:"payment,omitempty"`
	cashChosen bool
	// DriverLocation is filled by the API layer from live presence (Redis), never stored.
	DriverLocation *DriverLocation `json:"driver_location,omitempty"`
	// Trip queue (queue.go): Queued is true while the assigned driver is still
	// finishing the trip ahead (QueuedAfter, internal); QueuedPickupEtaS is the
	// pickup estimate including that finishing time.
	Queued           bool    `json:"queued"`
	QueuedPickupEtaS *int    `json:"queued_pickup_eta_s,omitempty"`
	QueuedAfter      *string `json:"-"`
	queuedPickupBy   *time.Time
	queuePromotedAt  *time.Time
}

// Contact is a person at a stop who is not the account holder.
type Contact struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

// Contacts on a trip: Pickup is the passenger (or package sender) when it is not
// the rider; Dropoff is the package receiver.
type Contacts struct {
	Pickup  *Contact `json:"pickup,omitempty"`
	Dropoff *Contact `json:"dropoff,omitempty"`
}

// Party is the other side of the trip as each participant sees it.
type Party struct {
	ID    string  `json:"id"`
	Name  *string `json:"name,omitempty"`
	Phone string  `json:"phone"`
}

// DriverLocation is the assigned driver's last ping.
type DriverLocation struct {
	Location routing.Point `json:"location"`
	Heading  float64       `json:"heading"`
	At       time.Time     `json:"at"`
}

// OpenRequest is a trip waiting for a driver, as dispatch needs it.
type OpenRequest struct {
	TripID       string
	RiderID      string
	ServiceKey   string
	VehicleClass string
	AreaID       string // operating area of the pickup ("" for trips before areas existed)
	Country      string // the trip's country (dispatch settings are per country)
	Pickup       routing.Point
	RequestedAt  time.Time
}

type Event struct {
	Seq       int             `json:"seq"`
	Type      string          `json:"type"`
	ActorKind string          `json:"actor_kind"`
	Payload   json.RawMessage `json:"payload"`
	At        time.Time       `json:"at"`
}

// CreateInput is what the rider sends to book.
type CreateInput struct {
	QuoteID       string      `json:"quote_id"`
	Contacts      Contacts    `json:"contacts"`
	VehicleClass  string      `json:"vehicle_class"`
	PaymentMethod string      `json:"payment_method"`
	Stops         []StopInput `json:"stops"`
	Notes         *string     `json:"notes"`
	// ScheduledFor books the ride ahead (between ScheduleMinLead and ScheduleMaxAhead from now).
	ScheduledFor *time.Time `json:"scheduled_for"`
	RiderID      string     `json:"-"`
}

type StopInput struct {
	Kind    string  `json:"kind"`
	Address *string `json:"address"`
}

var (
	ErrQuoteNotFound     = apperr.New("QUOTE_NOT_FOUND", http.StatusNotFound, "")
	ErrQuoteExpired      = apperr.New("QUOTE_EXPIRED", http.StatusConflict, "")
	ErrOptionUnavailable = apperr.New("OPTION_UNAVAILABLE", http.StatusConflict, "")
	ErrTripAlreadyActive = apperr.New("TRIP_ALREADY_ACTIVE", http.StatusConflict, "")
	ErrTripNotFound      = apperr.New("TRIP_NOT_FOUND", http.StatusNotFound, "")
	ErrNotParticipant    = apperr.New("FORBIDDEN", http.StatusForbidden, "")
	ErrInvalidTransition = apperr.New("INVALID_TRANSITION", http.StatusConflict, "")
	ErrCancelTooLate     = apperr.New("CANCEL_TOO_LATE", http.StatusConflict, "")
	ErrNotAssigned       = apperr.New("TRIP_NOT_ASSIGNED_TO_YOU", http.StatusForbidden, "")
	ErrScheduleRange     = apperr.New("SCHEDULE_OUT_OF_RANGE", http.StatusBadRequest, "")
	ErrContactRequired   = apperr.New("CONTACT_REQUIRED", http.StatusBadRequest, "")
)

// AreaRef names the operating area a trip was booked in.
type AreaRef struct {
	ID   string `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}
