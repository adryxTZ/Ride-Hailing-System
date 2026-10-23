package trips

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// The queries behind PersonStats, TopRoutes, LikelyHome and LastPlace. col is
// always "rider_id" or "driver_id" (from partyColumn), never input.

func (s store) personStats(ctx context.Context, col, userID string) (*PersonStats, error) {
	var p PersonStats
	var currency *string
	err := s.pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE status <> 'SCHEDULED'),
		  count(*) FILTER (WHERE status = 'COMPLETED'),
		  count(*) FILTER (WHERE status::text = ANY($3)),
		  count(*) FILTER (WHERE status = 'SCHEDULED'),
		  count(*) FILTER (WHERE status = 'CANCELLED_BY_RIDER'),
		  count(*) FILTER (WHERE status = 'CANCELLED_BY_DRIVER'),
		  count(*) FILTER (WHERE status = 'CANCELLED_BY_STAFF'),
		  count(*) FILTER (WHERE status IN ('NO_DRIVER_FOUND','EXPIRED')),
		  mode() WITHIN GROUP (ORDER BY currency),
		  coalesce(sum(final_fare) FILTER (WHERE status = 'COMPLETED'), 0),
		  round(coalesce(avg(final_fare) FILTER (WHERE status = 'COMPLETED'), 0)),
		  coalesce(sum(route_distance_m) FILTER (WHERE status = 'COMPLETED'), 0),
		  count(*) FILTER (WHERE status = 'COMPLETED' AND payment_method = 'CASH'),
		  count(*) FILTER (WHERE status = 'COMPLETED' AND payment_method <> 'CASH'),
		  count(*) FILTER (WHERE payment_status = 'FAILED'),
		  avg(extract(epoch FROM arrived_pickup_at - accepted_at)) FILTER (WHERE arrived_pickup_at > accepted_at),
		  avg(extract(epoch FROM completed_at - in_transit_at)) FILTER (WHERE status = 'COMPLETED' AND completed_at > in_transit_at),
		  min(requested_at) FILTER (WHERE status <> 'SCHEDULED'), max(requested_at) FILTER (WHERE status <> 'SCHEDULED')
		FROM trips WHERE brand_id = $1 AND `+col+` = $2`, s.brandID, userID, statusGroups["active"]).
		Scan(&p.Total, &p.Completed, &p.Active, &p.Scheduled, &p.CancelledByRider, &p.CancelledByDrv, &p.CancelledByStaff, &p.NoDriver,
			&currency, &p.FaresTotal, &p.AvgFare, &p.DistanceM, &p.PaidCash, &p.PaidMobile, &p.UnpaidFares,
			&p.AvgPickupS, &p.AvgRideS, &p.FirstTripAt, &p.LastTripAt)
	if currency != nil {
		p.Currency = *currency
	}
	return &p, err
}

// stopEnds is the first and last stop of each recent completed trip of the person.
const stopEnds = `
	SELECT t.completed_at, t.requested_at,
	  p.address AS pa, ST_Y(p.location::geometry) AS plat, ST_X(p.location::geometry) AS plng,
	  d.address AS da, ST_Y(d.location::geometry) AS dlat, ST_X(d.location::geometry) AS dlng
	FROM trips t
	JOIN LATERAL (SELECT address, location FROM trip_stops WHERE trip_id = t.id ORDER BY seq LIMIT 1) p ON true
	JOIN LATERAL (SELECT address, location FROM trip_stops WHERE trip_id = t.id ORDER BY seq DESC LIMIT 1) d ON true
	WHERE t.brand_id = $1 AND t.status = 'COMPLETED' AND t.%s = $2`

func (s store) topRoutes(ctx context.Context, col, userID string, limit int) ([]Route, error) {
	rows, err := s.pool.Query(ctx, `
		WITH legs AS (`+fmt.Sprintf(stopEnds, col)+` ORDER BY t.completed_at DESC LIMIT $3)
		SELECT (array_agg(pa ORDER BY completed_at DESC) FILTER (WHERE pa IS NOT NULL))[1], avg(plat), avg(plng),
		  (array_agg(da ORDER BY completed_at DESC) FILTER (WHERE da IS NOT NULL))[1], avg(dlat), avg(dlng), count(*), max(completed_at)
		FROM legs
		GROUP BY round(plat::numeric, $4), round(plng::numeric, $4), round(dlat::numeric, $4), round(dlng::numeric, $4)
		ORDER BY count(*) DESC, max(completed_at) DESC LIMIT $5`,
		s.brandID, userID, routeSampleTrips, routeGridDecimals, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Route{}
	for rows.Next() {
		var r Route
		if err := rows.Scan(&r.From.Address, &r.From.Lat, &r.From.Lng, &r.To.Address, &r.To.Lat, &r.To.Lng, &r.Trips, &r.LastAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s store) likelyHome(ctx context.Context, col, userID, tz string, since time.Time) (*Place, error) {
	var p Place
	err := s.pool.QueryRow(ctx, `
		WITH legs AS (`+fmt.Sprintf(stopEnds, col)+` AND t.requested_at >= $3 ORDER BY t.completed_at DESC LIMIT $4),
		pts AS (
		  SELECT pa AS a, plat AS lat, plng AS lng FROM legs WHERE extract(hour FROM requested_at AT TIME ZONE $5) < $6
		  UNION ALL
		  SELECT da, dlat, dlng FROM legs WHERE extract(hour FROM completed_at AT TIME ZONE $5) >= $7
		)
		SELECT (array_agg(a) FILTER (WHERE a IS NOT NULL))[1], avg(lat), avg(lng), count(*)
		FROM pts GROUP BY round(lat::numeric, $8), round(lng::numeric, $8)
		HAVING count(*) >= $9 ORDER BY count(*) DESC LIMIT 1`,
		s.brandID, userID, since, routeSampleTrips, tz, homeMorningHour, homeEveningHour, routeGridDecimals, homeMinSightings).
		Scan(&p.Address, &p.Lat, &p.Lng, &p.Sightings)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return &p, err
}

func (s store) lastPlace(ctx context.Context, col, userID string) (*Place, error) {
	var p Place
	err := s.pool.QueryRow(ctx, `
		SELECT st.address, ST_Y(st.location::geometry), ST_X(st.location::geometry), st.arrived_at, t.code
		FROM trips t JOIN trip_stops st ON st.trip_id = t.id
		WHERE t.brand_id = $1 AND t.`+col+` = $2 AND st.arrived_at IS NOT NULL
		ORDER BY st.arrived_at DESC LIMIT 1`, s.brandID, userID).
		Scan(&p.Address, &p.Lat, &p.Lng, &p.At, &p.TripCode)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return &p, err
}

func (s store) personCountries(ctx context.Context, col, userID string) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT DISTINCT country_code FROM trips WHERE brand_id = $1 AND `+col+` = $2`, s.brandID, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}
