package identity

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/phone"
)

// StaffRolePrefix marks staff roles inside the token's roles claim
// ("staff:Finance Officer"), next to the account roles (RIDER, DRIVER, STAFF).
const StaffRolePrefix = "staff:"

// EnsureUser finds the account for a phone or creates it, grants `role`, and
// sets the name when given. Used by operator commands and staff tools.
func (s *Service) EnsureUser(ctx context.Context, rawPhone, role, fullName string) (*User, error) {
	normalised, err := phone.Normalize(rawPhone, s.country)
	if err != nil {
		return nil, ErrPhoneInvalid
	}
	var user *User
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		u, err := s.store.findUserByPhone(ctx, tx, s.brandID, normalised)
		if err != nil {
			return err
		}
		if u == nil {
			if u, err = s.store.createUser(ctx, tx, s.brandID, normalised, s.locale, role); err != nil {
				return err
			}
		} else if err := s.store.grantRole(ctx, tx, u.ID, role); err != nil {
			return err
		}
		if name := strings.TrimSpace(fullName); name != "" {
			if _, err := tx.Exec(ctx, `UPDATE users SET full_name=$2, updated_at=now() WHERE id=$1`, u.ID, name); err != nil {
				return err
			}
		}
		user = u
		return nil
	})
	return user, err
}

// AddStaff creates (or updates) a dashboard account with the given staff roles.
func (s *Service) AddStaff(ctx context.Context, rawPhone, fullName string, roles []string) (*User, error) {
	u, err := s.EnsureUser(ctx, rawPhone, RoleStaff, fullName)
	if err != nil {
		return nil, err
	}
	for _, r := range roles {
		if _, err := s.pool.Exec(ctx, `INSERT INTO staff_roles (user_id, role) VALUES ($1, $2) ON CONFLICT DO NOTHING`, u.ID, strings.TrimSpace(r)); err != nil {
			return nil, err
		}
	}
	return u, nil
}

// staffClaims is what a dashboard token carries beyond the account roles: the
// staff roles ("staff:Finance"), their permissions ("perm:fees.edit") and the
// countries/regions the person is limited to ("scope:TZ", "scope:TZ/<area>").
func (s *Service) staffClaims(ctx context.Context, tx pgx.Tx, userID string) ([]string, error) {
	rows, err := tx.Query(ctx, `SELECT role FROM staff_roles WHERE user_id=$1 ORDER BY role`, userID)
	if err != nil {
		return nil, err
	}
	roles, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	perms, err := s.permissionsOf(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	scopes, err := s.store.accessOf(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(roles)+len(perms)+len(scopes))
	for _, r := range roles {
		out = append(out, StaffRolePrefix+r)
	}
	for _, p := range perms {
		out = append(out, PermissionPrefix+p)
	}
	for _, sc := range scopes {
		out = append(out, ScopePrefix+sc)
	}
	return out, nil
}
