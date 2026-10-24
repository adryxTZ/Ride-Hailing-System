package safety

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/scope"
)

type store struct {
	pool    *pgxpool.Pool
	brandID string
}

func (s store) contacts(ctx context.Context, userID string) ([]TrustedContact, error) {
	rows, err := s.pool.Query(ctx, `SELECT name, phone FROM trusted_contacts WHERE user_id=$1 ORDER BY created_at`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[TrustedContact])
}

// replaceContacts swaps the whole list in one transaction.
func (s store) replaceContacts(ctx context.Context, userID string, list []TrustedContact) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM trusted_contacts WHERE user_id=$1`, userID); err != nil {
			return err
		}
		for _, c := range list {
			if _, err := tx.Exec(ctx, `INSERT INTO trusted_contacts (user_id, name, phone) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`, userID, c.Name, c.Phone); err != nil {
				return err
			}
		}
		return nil
	})
}

// shareToken returns the trip's token, setting `fresh` when none exists yet.
func (s store) shareToken(ctx context.Context, tripID, fresh string) (string, error) {
	var token string
	err := s.pool.QueryRow(ctx, `
		UPDATE trips SET share_token = COALESCE(share_token, $2) WHERE id=$1 AND brand_id=$3 RETURNING share_token`,
		tripID, fresh, s.brandID).Scan(&token)
	return token, err
}

func (s store) tripByToken(ctx context.Context, token string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `SELECT id FROM trips WHERE share_token=$1 AND brand_id=$2`, token, s.brandID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return id, err
}

func (s store) insertAlert(ctx context.Context, tripID, userID string, in SOSInput, alerted int) (string, error) {
	var lat, lng *float64
	if in.Location != nil {
		lat, lng = &in.Location.Lat, &in.Location.Lng
	}
	var id string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO sos_alerts (brand_id, trip_id, user_id, lat, lng, contacts_alerted) VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		s.brandID, tripID, userID, lat, lng, alerted).Scan(&id)
	return id, err
}

const alertColumns = `a.id, a.trip_id, t.code, a.user_id, u.full_name, u.phone, a.lat, a.lng, a.contacts_alerted, a.created_at, a.resolved_at, a.resolution`

func (s store) alerts(ctx context.Context, openOnly bool, sc scope.Scope, limit int) ([]Alert, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+alertColumns+` FROM sos_alerts a JOIN trips t ON t.id=a.trip_id JOIN users u ON u.id=a.user_id
		WHERE a.brand_id=$1 AND (NOT $2 OR a.resolved_at IS NULL)
		  AND ($4 = '' OR t.country_code = $4) AND ($5 = '' OR t.area_id = NULLIF($5,'')::uuid)
		ORDER BY a.created_at DESC LIMIT $3`, s.brandID, openOnly, limit, sc.Country, sc.AreaID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Alert])
}

func (s store) resolve(ctx context.Context, id, staffID, resolution string) (bool, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE sos_alerts SET resolved_at=now(), resolved_by=$2, resolution=$3 WHERE id=$1 AND brand_id=$4 AND resolved_at IS NULL`,
		id, staffID, resolution, s.brandID)
	return tag.RowsAffected() > 0, err
}

// alertScope is where an alert's trip happened (staff limited to a country or region).
func (s store) alertScope(ctx context.Context, id string) (country, areaID string, err error) {
	err = s.pool.QueryRow(ctx, `SELECT coalesce(t.country_code,''), coalesce(t.area_id::text,'') FROM sos_alerts a JOIN trips t ON t.id=a.trip_id
		WHERE a.id::text=$1 AND a.brand_id=$2`, id, s.brandID).Scan(&country, &areaID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", apperr.NotFound
	}
	return country, areaID, err
}
