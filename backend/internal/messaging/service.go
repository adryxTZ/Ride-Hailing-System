package messaging

import "github.com/jackc/pgx/v5/pgxpool"

// Service is the messaging use cases. Realtime and Pusher may be nil (tests,
// processes without Redis); messages are then only stored.
type Service struct {
	store    store
	trips    Trips
	realtime Realtime
	pusher   Pusher
}

// New wires the service.
func New(pool *pgxpool.Pool, brandID string, t Trips, rt Realtime, p Pusher) *Service {
	return &Service{store: store{pool: pool, brandID: brandID}, trips: t, realtime: rt, pusher: p}
}

// roleIn tells whether userID is the rider or the driver of the trip.
func roleIn(t *TripView, userID string) (string, bool) {
	switch userID {
	case t.RiderID:
		return RoleRider, true
	case t.DriverID:
		if t.DriverID != "" {
			return RoleDriver, true
		}
	}
	return "", false
}

// other returns the participant on the other side and the app they use.
func other(t *TripView, role string) (userID, app string) {
	if role == RoleRider {
		return t.DriverID, RoleDriver
	}
	return t.RiderID, RoleRider
}
