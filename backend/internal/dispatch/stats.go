package dispatch

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// OfferStats is how a driver answers offers: counts by outcome and how fast
// they respond (offered → accepted or declined; timeouts are not responses).
type OfferStats struct {
	Offered        int        `json:"offered"`
	Accepted       int        `json:"accepted"`
	Declined       int        `json:"declined"`
	TimedOut       int        `json:"timed_out"`
	AcceptanceRate *float64   `json:"acceptance_rate"` // accepted ÷ answered-or-expired offers
	AvgResponseS   *float64   `json:"avg_response_s"`
	LastOfferAt    *time.Time `json:"last_offer_at"`
}

// OfferStats reads a driver's offer history (staff driver page).
func (s *Service) OfferStats(ctx context.Context, driverID string) (*OfferStats, error) {
	return s.store.offerStats(ctx, driverID)
}

func (s store) offerStats(ctx context.Context, driverID string) (*OfferStats, error) {
	var o OfferStats
	err := s.pool.QueryRow(ctx, `
		SELECT count(*),
		  count(*) FILTER (WHERE outcome = 'ACCEPTED'),
		  count(*) FILTER (WHERE outcome = 'DECLINED'),
		  count(*) FILTER (WHERE outcome = 'TIMED_OUT'),
		  avg(extract(epoch FROM responded_at - offered_at)) FILTER (WHERE outcome IN ('ACCEPTED','DECLINED') AND responded_at IS NOT NULL),
		  max(offered_at)
		FROM trip_offers WHERE driver_id = $1`, driverID).
		Scan(&o.Offered, &o.Accepted, &o.Declined, &o.TimedOut, &o.AvgResponseS, &o.LastOfferAt)
	if decided := o.Accepted + o.Declined + o.TimedOut; decided > 0 {
		r := float64(o.Accepted) / float64(decided)
		o.AcceptanceRate = &r
	}
	return &o, err
}

// OfferDay is one local day of a driver's offer answers.
type OfferDay struct {
	Day          string   `json:"day"` // YYYY-MM-DD
	Offered      int      `json:"offered"`
	Accepted     int      `json:"accepted"`
	Declined     int      `json:"declined"`
	TimedOut     int      `json:"timed_out"`
	AvgResponseS *float64 `json:"avg_response_s"`
}

// OfferSeries is a driver's offers per local day since `since` (days with none are zero).
func (s *Service) OfferSeries(ctx context.Context, driverID, tz string, since time.Time) ([]OfferDay, error) {
	rows, err := s.store.pool.Query(ctx, `
		WITH d AS (
		  SELECT generate_series(($2::timestamptz AT TIME ZONE $3)::date, (now() AT TIME ZONE $3)::date, interval '1 day')::date AS day
		), o AS (
		  SELECT (offered_at AT TIME ZONE $3)::date AS day, outcome, responded_at - offered_at AS took
		  FROM trip_offers WHERE driver_id = $1 AND offered_at >= $2
		)
		SELECT d.day::text, count(o.day), count(o.day) FILTER (WHERE o.outcome = 'ACCEPTED'),
		  count(o.day) FILTER (WHERE o.outcome = 'DECLINED'), count(o.day) FILTER (WHERE o.outcome = 'TIMED_OUT'),
		  avg(extract(epoch FROM o.took)) FILTER (WHERE o.outcome IN ('ACCEPTED','DECLINED') AND o.took IS NOT NULL)
		FROM d LEFT JOIN o ON o.day = d.day GROUP BY d.day ORDER BY d.day`, driverID, since, tz)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[OfferDay])
}
