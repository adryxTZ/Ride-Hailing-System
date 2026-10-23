package trips

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

// Chart windows for the person pages.
const (
	DefaultSeriesDays = 90
	MaxSeriesDays     = 365
	hoursInDay        = 24
)

// DayCount is one local day of a person's trips.
type DayCount struct {
	Day       string          `json:"day"` // YYYY-MM-DD in the person's country
	Requested int             `json:"requested"`
	Completed int             `json:"completed"`
	Cancelled int             `json:"cancelled"`
	Fares     decimal.Decimal `json:"fares"` // completed fares that day
}

// ClassCount is completed trips in one vehicle class.
type ClassCount struct {
	VehicleClass string `json:"vehicle_class"`
	Trips        int    `json:"trips"`
}

// PersonSeries is a person's trips over time, by hour of day and by vehicle class.
type PersonSeries struct {
	Days    int             `json:"days"`
	Daily   []DayCount      `json:"daily"`
	ByHour  [hoursInDay]int `json:"by_hour"` // trips requested per local hour, 0 = midnight
	ByClass []ClassCount    `json:"by_class"`
}

// ClampDays keeps a requested chart window within bounds.
func ClampDays(days int) int {
	if days <= 0 {
		return DefaultSeriesDays
	}
	return min(days, MaxSeriesDays)
}

// PersonSeries reads a person's last `days` days of trips in timezone tz.
func (s *Service) PersonSeries(ctx context.Context, role, userID, tz string, days int) (*PersonSeries, error) {
	col, ok := partyColumn(role)
	if !ok {
		return nil, ErrTripNotFound
	}
	days = ClampDays(days)
	since := s.now().AddDate(0, 0, -(days - 1))
	out := &PersonSeries{Days: days}
	var err error
	if out.Daily, err = s.store.daily(ctx, col, userID, tz, since); err != nil {
		return nil, err
	}
	if out.ByHour, err = s.store.byHour(ctx, col, userID, tz, since); err != nil {
		return nil, err
	}
	out.ByClass, err = s.store.byClass(ctx, col, userID, since)
	return out, err
}

// daily zero-fills every local day from since to today.
func (s store) daily(ctx context.Context, col, userID, tz string, since time.Time) ([]DayCount, error) {
	rows, err := s.pool.Query(ctx, `
		WITH d AS (
		  SELECT generate_series(($3::timestamptz AT TIME ZONE $4)::date, (now() AT TIME ZONE $4)::date, interval '1 day')::date AS day
		), t AS (
		  SELECT (requested_at AT TIME ZONE $4)::date AS day, status, final_fare FROM trips
		  WHERE brand_id = $1 AND `+col+` = $2 AND requested_at >= $3 AND status <> 'SCHEDULED'
		)
		SELECT d.day::text, count(t.day), count(t.day) FILTER (WHERE t.status = 'COMPLETED'),
		  count(t.day) FILTER (WHERE t.status::text LIKE 'CANCELLED%'),
		  coalesce(sum(t.final_fare) FILTER (WHERE t.status = 'COMPLETED'), 0)
		FROM d LEFT JOIN t ON t.day = d.day GROUP BY d.day ORDER BY d.day`, s.brandID, userID, since, tz)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[DayCount])
}

func (s store) byHour(ctx context.Context, col, userID, tz string, since time.Time) ([hoursInDay]int, error) {
	var out [hoursInDay]int
	rows, err := s.pool.Query(ctx, `
		SELECT extract(hour FROM requested_at AT TIME ZONE $4)::int, count(*) FROM trips
		WHERE brand_id = $1 AND `+col+` = $2 AND requested_at >= $3 AND status <> 'SCHEDULED' GROUP BY 1`, s.brandID, userID, since, tz)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var h, n int
		if err := rows.Scan(&h, &n); err != nil {
			return out, err
		}
		if h >= 0 && h < hoursInDay {
			out[h] = n
		}
	}
	return out, rows.Err()
}

func (s store) byClass(ctx context.Context, col, userID string, since time.Time) ([]ClassCount, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT vehicle_class_key, count(*) FROM trips
		WHERE brand_id = $1 AND `+col+` = $2 AND requested_at >= $3 AND status = 'COMPLETED'
		GROUP BY 1 ORDER BY 2 DESC`, s.brandID, userID, since)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[ClassCount])
}
