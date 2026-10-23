package safety

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// personAlertsLimit bounds the SOS list on a person page.
const personAlertsLimit = 50

// AlertsFor lists SOS alerts on a person's trips: raised by them, or raised
// on trips they drove (staff rider and driver pages), newest first.
func (s *Service) AlertsFor(ctx context.Context, userID string) ([]Alert, error) {
	return s.store.alertsFor(ctx, userID, personAlertsLimit)
}

func (s store) alertsFor(ctx context.Context, userID string, limit int) ([]Alert, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+alertColumns+` FROM sos_alerts a JOIN trips t ON t.id=a.trip_id JOIN users u ON u.id=a.user_id
		WHERE a.brand_id=$1 AND (a.user_id=$2 OR t.driver_id=$2) ORDER BY a.created_at DESC LIMIT $3`, s.brandID, userID, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Alert])
}
