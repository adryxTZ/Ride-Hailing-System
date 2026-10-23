package dispatch

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type store struct{ pool *pgxpool.Pool }

// excluded returns the drivers already asked for each trip (they are not asked again).
func (s store) excluded(ctx context.Context, tripIDs []string) (map[string]map[string]bool, error) {
	out := map[string]map[string]bool{}
	if len(tripIDs) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx, `SELECT trip_id, driver_id FROM trip_offers WHERE trip_id = ANY($1::uuid[])`, tripIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var trip, driver string
		if err := rows.Scan(&trip, &driver); err != nil {
			return nil, err
		}
		if out[trip] == nil {
			out[trip] = map[string]bool{}
		}
		out[trip][driver] = true
	}
	return out, rows.Err()
}

// insertOffer records a new open offer; false if the driver or trip already has one.
func (s store) insertOffer(ctx context.Context, tx pgx.Tx, c candidate, expires time.Time) (bool, error) {
	tag, err := tx.Exec(ctx, `
		INSERT INTO trip_offers (trip_id, driver_id, wave, distance_m, eta_s, expires_at)
		VALUES ($1, $2, (SELECT COALESCE(max(wave),0)+1 FROM trip_offers WHERE trip_id=$1), $3, $4, $5)
		ON CONFLICT DO NOTHING`, c.TripID, c.DriverID, int(c.DistanceM), c.EtaS, expires)
	return err == nil && tag.RowsAffected() == 1, err
}

// openOffer is an offer still waiting for the driver's answer.
type openOffer struct {
	TripID    string
	DriverID  string
	DistanceM int
	EtaS      int
	ExpiresAt time.Time
}

// lockOpen locks the driver's open offer for this trip (accept/decline).
func (s store) lockOpen(ctx context.Context, tx pgx.Tx, tripID, driverID string) (*openOffer, error) {
	var o openOffer
	err := tx.QueryRow(ctx, `SELECT trip_id, driver_id, distance_m, eta_s, expires_at FROM trip_offers
		WHERE trip_id=$1 AND driver_id=$2 AND outcome='OFFERED' FOR UPDATE`, tripID, driverID).
		Scan(&o.TripID, &o.DriverID, &o.DistanceM, &o.EtaS, &o.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &o, err
}

// openForDriver returns the driver's current open offer, if any (app resume).
func (s store) openForDriver(ctx context.Context, driverID string) (*openOffer, error) {
	var o openOffer
	err := s.pool.QueryRow(ctx, `SELECT trip_id, driver_id, distance_m, eta_s, expires_at FROM trip_offers
		WHERE driver_id=$1 AND outcome='OFFERED' AND expires_at > now()`, driverID).
		Scan(&o.TripID, &o.DriverID, &o.DistanceM, &o.EtaS, &o.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &o, err
}

func (s store) close(ctx context.Context, tx pgx.Tx, tripID, driverID, outcome string) error {
	_, err := tx.Exec(ctx, `UPDATE trip_offers SET outcome=$3::offer_outcome, responded_at=now() WHERE trip_id=$1 AND driver_id=$2 AND outcome='OFFERED'`,
		tripID, driverID, outcome)
	return err
}

// expired closes offers past their deadline and returns them.
func (s store) expired(ctx context.Context, tx pgx.Tx) ([]openOffer, error) {
	rows, err := tx.Query(ctx, `UPDATE trip_offers SET outcome='TIMED_OUT', responded_at=now()
		WHERE outcome='OFFERED' AND expires_at <= now() RETURNING trip_id, driver_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []openOffer
	for rows.Next() {
		var o openOffer
		if err := rows.Scan(&o.TripID, &o.DriverID); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// cancelForTrip closes any open offer on a trip that ended; returns the driver it was with.
func (s store) cancelForTrip(ctx context.Context, tripID string) (string, error) {
	var driver string
	err := s.pool.QueryRow(ctx, `UPDATE trip_offers SET outcome='CANCELLED', responded_at=now()
		WHERE trip_id=$1 AND outcome='OFFERED' RETURNING driver_id`, tripID).Scan(&driver)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return driver, err
}
