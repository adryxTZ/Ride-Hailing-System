package identity

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/phone"
)

// StaffGrants are a person's current staff roles and the permissions they
// give, read from the database (approval decisions use these, not the token).
func (s *Service) StaffGrants(ctx context.Context, userID string) (roles, perms []string, err error) {
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT role FROM staff_roles WHERE user_id=$1 ORDER BY role`, userID)
		if err != nil {
			return err
		}
		if roles, err = pgx.CollectRows(rows, pgx.RowTo[string]); err != nil {
			return err
		}
		perms, err = s.permissionsOf(ctx, tx, userID)
		return err
	})
	return roles, perms, err
}

// StaffByPhone is the account behind a phone number with its staff roles;
// found=false when nobody has that number yet.
type StaffByPhone struct {
	UserID string
	Name   string
	Roles  []string
	Found  bool
}

// StaffRolesByPhone reads who a phone belongs to and their staff roles (the
// "before" of a role change).
func (s *Service) StaffRolesByPhone(ctx context.Context, rawPhone string) (StaffByPhone, error) {
	normalised, err := phone.Normalize(rawPhone, s.country)
	if err != nil {
		return StaffByPhone{}, ErrPhoneInvalid
	}
	var out StaffByPhone
	err = s.pool.QueryRow(ctx, `
		SELECT u.id, coalesce(u.full_name, ''),
		  coalesce((SELECT array_agg(sr.role ORDER BY sr.role) FROM staff_roles sr WHERE sr.user_id=u.id), '{}')
		FROM users u WHERE u.brand_id=$1 AND u.phone=$2`, s.brandID, normalised).Scan(&out.UserID, &out.Name, &out.Roles)
	if errors.Is(err, pgx.ErrNoRows) {
		return StaffByPhone{Roles: []string{}}, nil
	}
	out.Found = err == nil
	return out, err
}

// RoleNames lists the brand's role names (what approval steps may name).
func (s *Service) RoleNames(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT name FROM roles WHERE brand_id=$1 ORDER BY system DESC, name`, s.brandID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}
