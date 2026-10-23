package profile

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type store struct{ pool *pgxpool.Pool }

func (s store) get(ctx context.Context, userID string) (*Profile, error) {
	var p Profile
	var photoKey *string
	var verified *string
	err := s.pool.QueryRow(ctx, `
		SELECT u.id, u.phone, u.full_name, u.email, u.email_verified_at::text, u.locale, u.photo_key, u.created_at,
		       COALESCE(array_agg(r.role::text) FILTER (WHERE r.role IS NOT NULL), '{}')
		FROM users u LEFT JOIN user_roles r ON r.user_id = u.id
		WHERE u.id=$1 AND u.deleted_at IS NULL GROUP BY u.id`, userID).
		Scan(&p.ID, &p.Phone, &p.FullName, &p.Email, &verified, &p.Locale, &photoKey, &p.CreatedAt, &p.Roles)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.EmailVerified = verified != nil
	p.PhotoURL = photoKey // resolved to a URL by the object-store provider once uploads exist
	return &p, nil
}

func (s store) update(ctx context.Context, userID string, u ProfileUpdate) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE users SET
		  full_name = COALESCE($2, full_name),
		  email = COALESCE($3, email),
		  email_verified_at = CASE WHEN $3 IS NOT NULL AND $3 IS DISTINCT FROM email THEN NULL ELSE email_verified_at END,
		  locale = COALESCE($4, locale),
		  updated_at = now()
		WHERE id=$1`, userID, u.FullName, u.Email, u.Locale)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrEmailTaken
	}
	return err
}

func (s store) listPlaces(ctx context.Context, userID string) ([]Place, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, kind::text, label, address, ST_Y(location::geometry), ST_X(location::geometry), use_count, updated_at
		FROM saved_places WHERE user_id=$1
		ORDER BY CASE kind WHEN 'HOME' THEN 0 WHEN 'WORK' THEN 1 ELSE 2 END, use_count DESC, label`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Place{}
	for rows.Next() {
		var p Place
		if err := rows.Scan(&p.ID, &p.Kind, &p.Label, &p.Address, &p.Lat, &p.Lng, &p.UseCount, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// upsertPlace replaces HOME/WORK in place (one each) and appends OTHER places.
func (s store) upsertPlace(ctx context.Context, userID string, in PlaceInput) (*Place, error) {
	point := fmt.Sprintf("SRID=4326;POINT(%f %f)", in.Lng, in.Lat)
	var p Place
	var row pgx.Row
	if in.Kind == "HOME" || in.Kind == "WORK" {
		row = s.pool.QueryRow(ctx, `
			INSERT INTO saved_places (user_id, kind, label, address, location) VALUES ($1,$2,$3,$4,ST_GeogFromText($5))
			ON CONFLICT (user_id) WHERE kind = $2::place_kind DO UPDATE SET label=EXCLUDED.label, address=EXCLUDED.address, location=EXCLUDED.location, updated_at=now()
			RETURNING id, kind::text, label, address, ST_Y(location::geometry), ST_X(location::geometry), use_count, updated_at`,
			userID, in.Kind, in.Label, in.Address, point)
	} else {
		row = s.pool.QueryRow(ctx, `
			INSERT INTO saved_places (user_id, kind, label, address, location) VALUES ($1,'OTHER',$2,$3,ST_GeogFromText($4))
			RETURNING id, kind::text, label, address, ST_Y(location::geometry), ST_X(location::geometry), use_count, updated_at`,
			userID, in.Label, in.Address, point)
	}
	if err := row.Scan(&p.ID, &p.Kind, &p.Label, &p.Address, &p.Lat, &p.Lng, &p.UseCount, &p.UpdatedAt); err != nil {
		return nil, err
	}
	return &p, nil
}

func (s store) deletePlace(ctx context.Context, userID, placeID string) (bool, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM saved_places WHERE id=$1 AND user_id=$2`, placeID, userID)
	return tag.RowsAffected() > 0, err
}
