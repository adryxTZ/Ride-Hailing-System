package trips

import (
	"context"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/scope"
	"time"

	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
)

// WaitingRequest is a ride looking for a driver, for the live map.
type WaitingRequest struct {
	TripID       string        `json:"trip_id"`
	Code         string        `json:"code"`
	Service      string        `json:"service"`
	VehicleClass string        `json:"vehicle_class"`
	Pickup       routing.Point `json:"pickup"`
	WaitingS     int           `json:"waiting_s"`
}

// liveRequestLimit bounds the live map's request layer.
const liveRequestLimit = 500

// Waiting lists rides currently looking for a driver, oldest first.
func (s *Service) Waiting(ctx context.Context, sc scope.Scope) ([]WaitingRequest, error) {
	rows, err := s.store.pool.Query(ctx, `
		SELECT t.id, t.code, t.service_key, t.vehicle_class_key, ST_Y(st.location::geometry), ST_X(st.location::geometry), t.requested_at
		FROM trips t JOIN trip_stops st ON st.trip_id=t.id AND st.kind='PICKUP'
		WHERE t.brand_id=$1 AND t.status IN ('REQUESTED','OFFERED')
		  AND ($3 = '' OR t.country_code = $3) AND ($4 = '' OR t.area_id = NULLIF($4,'')::uuid)
		ORDER BY t.requested_at LIMIT $2`, s.store.brandID, liveRequestLimit, sc.Country, sc.AreaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WaitingRequest{}
	for rows.Next() {
		var w WaitingRequest
		var at time.Time
		if err := rows.Scan(&w.TripID, &w.Code, &w.Service, &w.VehicleClass, &w.Pickup.Lat, &w.Pickup.Lng, &at); err != nil {
			return nil, err
		}
		w.WaitingS = int(s.now().Sub(at).Seconds())
		out = append(out, w)
	}
	return out, rows.Err()
}
