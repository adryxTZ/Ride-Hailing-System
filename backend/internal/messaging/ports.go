package messaging

import "context"

// TripView is what messaging needs to know about a trip.
type TripView struct {
	ID       string
	Code     string
	Status   string
	RiderID  string
	DriverID string // "" before a driver is assigned
	Terminal bool
}

// Trips looks up a trip's participants and state.
type Trips interface {
	View(ctx context.Context, tripID string) (*TripView, error)
}

// Realtime delivers a frame to a user's open sockets (no-op when nil).
type Realtime interface {
	Send(ctx context.Context, userID, frameType string, data any)
}

// Notification is one push message.
type Notification struct {
	Title string
	Body  string
	// Data is delivered to the app untouched (e.g. {"type":"chat","trip_id":…}).
	Data map[string]string
	// Priority "high" wakes the phone (offers); "normal" otherwise.
	Priority string
	// Channel is the Android channel id ("offers", "trips", "chat").
	Channel string
}

// Pusher sends notifications to device tokens and reports tokens the provider
// says are no longer valid, so they can be removed.
type Pusher interface {
	Push(ctx context.Context, tokens []string, n Notification) (invalid []string, err error)
}
