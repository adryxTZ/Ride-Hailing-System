package safety

import (
	"context"
	"time"

	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
)

// TripFacts is what safety reads about a trip.
type TripFacts struct {
	ID, Code, Status, RiderID string
	DriverID                  string // "" before assignment
	RiderName, DriverName     string
	Vehicle                   string // shown to people following the trip
	Pickup, Dropoff           *Place
	EndedAt                   *time.Time
	Terminal                  bool
}

// Trips looks trips up and records events on them.
type Trips interface {
	Facts(ctx context.Context, tripID string) (*TripFacts, error)
	RecordEvent(ctx context.Context, tripID, typ, actorKind string, actorID *string, payload any) error
}

// DriverLocator returns a driver's live position (nil when not online).
type DriverLocator func(ctx context.Context, driverID string) *routing.Point

// Texter sends an SMS (the SMS provider).
type Texter interface {
	Send(ctx context.Context, to, body string) error
}
