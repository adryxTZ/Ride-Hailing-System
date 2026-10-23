package safety

import (
	"net/http"
	"time"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
)

// MaxTrustedContacts is how many people a rider can trust.
const MaxTrustedContacts = 3

// ShareGrace keeps a share link alive this long after the trip ends.
const ShareGrace = time.Hour

// TrustedContact is someone the rider wants alerted on SOS.
type TrustedContact struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

// ShareLink is the public link to a live trip.
type ShareLink struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

// PublicTrip is what anyone with the share link sees: no phone numbers, no fares.
type PublicTrip struct {
	Code       string         `json:"code"`
	Status     string         `json:"status"`
	RiderName  string         `json:"rider_first_name"`
	DriverName string         `json:"driver_first_name,omitempty"`
	Vehicle    string         `json:"vehicle,omitempty"` // "Silver Toyota IST · T123ABC"
	Pickup     *Place         `json:"pickup,omitempty"`
	Dropoff    *Place         `json:"dropoff,omitempty"`
	Driver     *routing.Point `json:"driver_location,omitempty"`
	UpdatedAt  time.Time      `json:"updated_at"`
	Ended      bool           `json:"ended"`
}

// Place is a stop on the public view.
type Place struct {
	Location routing.Point `json:"location"`
	Address  string        `json:"address,omitempty"`
}

// Alert is an SOS raised on a trip.
type Alert struct {
	ID              string     `json:"id"`
	TripID          string     `json:"trip_id"`
	TripCode        string     `json:"trip_code"`
	UserID          string     `json:"user_id"`
	UserName        *string    `json:"user_name,omitempty"`
	UserPhone       string     `json:"user_phone"`
	Lat             *float64   `json:"lat,omitempty"`
	Lng             *float64   `json:"lng,omitempty"`
	ContactsAlerted int        `json:"contacts_alerted"`
	CreatedAt       time.Time  `json:"created_at"`
	ResolvedAt      *time.Time `json:"resolved_at,omitempty"`
	Resolution      *string    `json:"resolution,omitempty"`
}

// SOSInput is where the rider is when pressing SOS (optional).
type SOSInput struct {
	Location *routing.Point `json:"location"`
}

var (
	ErrTooManyContacts = apperr.New("TOO_MANY_CONTACTS", http.StatusBadRequest, "")
	ErrShareExpired    = apperr.New("SHARE_EXPIRED", http.StatusGone, "")
	ErrNotParticipant  = apperr.New("FORBIDDEN", http.StatusForbidden, "")
)
