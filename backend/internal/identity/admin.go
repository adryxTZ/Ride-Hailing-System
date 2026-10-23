package identity

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/audit"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/phone"
)

// Admin page bounds.
const (
	adminPageSize = 50
	adminMaxPage  = 200
)

// UserRow is one person in a staff list.
type UserRow struct {
	ID        string    `json:"id"`
	Name      *string   `json:"full_name,omitempty"`
	Phone     string    `json:"phone"`
	Email     *string   `json:"email,omitempty"`
	Status    string    `json:"status"`
	Roles     []string  `json:"roles"`
	CreatedAt time.Time `json:"created_at"`
}

// UserPage is one page of users.
type UserPage struct {
	Data  []UserRow `json:"data"`
	Total int       `json:"total"`
}

// StaffRow is a dashboard account with its staff roles.
type StaffRow struct {
	UserRow
	StaffRoles []string `json:"staff_roles"`
}

// ListUsers lists people with a role (RIDER, DRIVER, STAFF), matching name, phone or email.
// With a country (and optional area), only people who booked a trip there are listed.
func (s *Service) ListUsers(ctx context.Context, role, q, country, areaID string, limit, offset int) (*UserPage, error) {
	if limit <= 0 || limit > adminMaxPage {
		limit = adminPageSize
	}
	const where = `u.brand_id=$1 AND u.deleted_at IS NULL AND EXISTS (SELECT 1 FROM user_roles r WHERE r.user_id=u.id AND r.role::text=$2)
		AND ($3='' OR u.full_name ILIKE '%'||$3||'%' OR u.phone LIKE '%'||$3||'%' OR u.email::text ILIKE '%'||$3||'%')
		AND ($4='' OR EXISTS (SELECT 1 FROM trips t WHERE t.rider_id=u.id AND t.country_code=$4 AND ($5='' OR t.area_id::text=$5)))`
	q = strings.TrimSpace(q)
	page := &UserPage{Data: []UserRow{}}
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM users u WHERE `+where, s.brandID, role, q, country, areaID).Scan(&page.Total); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT u.id, u.full_name, u.phone, u.email::text, u.status::text,
		  COALESCE((SELECT array_agg(r.role::text) FROM user_roles r WHERE r.user_id=u.id), '{}'), u.created_at
		FROM users u WHERE `+where+` ORDER BY u.created_at DESC LIMIT $6 OFFSET $7`, s.brandID, role, q, country, areaID, limit, max(0, offset))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var u UserRow
		if err := rows.Scan(&u.ID, &u.Name, &u.Phone, &u.Email, &u.Status, &u.Roles, &u.CreatedAt); err != nil {
			return nil, err
		}
		page.Data = append(page.Data, u)
	}
	return page, rows.Err()
}

// SetStatus blocks (DISABLED) or unblocks (ACTIVE) an account, audited. Blocking
// ends the person's sessions: refresh tokens are revoked at once.
func (s *Service) SetStatus(ctx context.Context, staffID, userID, status, reason string) error {
	if status != "ACTIVE" && status != "DISABLED" {
		return apperr.Validation
	}
	if strings.TrimSpace(reason) == "" {
		return apperr.Validation.WithDetails(map[string]any{"fields": []map[string]string{{"path": "reason", "code": "REQUIRED"}}})
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE users SET status=$2::user_status, updated_at=now() WHERE id=$1 AND brand_id=$3`, userID, status, s.brandID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return apperr.NotFound
		}
		if status == "DISABLED" {
			if _, err := tx.Exec(ctx, `UPDATE refresh_tokens SET revoked_at=now() WHERE user_id=$1 AND revoked_at IS NULL`, userID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE sessions SET ended_at=now(), ended_by=NULLIF($2,'')::uuid, end_reason=$3 WHERE user_id=$1 AND ended_at IS NULL`, userID, staffID, "Account deactivated"); err != nil {
				return err
			}
		}
		return audit.Record(ctx, tx, s.brandID, staffID, "user."+strings.ToLower(status), "user:"+userID, reason, nil)
	})
}

// ListStaff lists dashboard accounts with their staff roles.
func (s *Service) ListStaff(ctx context.Context) ([]StaffRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT u.id, u.full_name, u.phone, u.email::text, u.status::text, '{STAFF}'::text[], u.created_at,
		  COALESCE((SELECT array_agg(sr.role ORDER BY sr.role) FROM staff_roles sr WHERE sr.user_id=u.id), '{}')
		FROM users u JOIN user_roles r ON r.user_id=u.id AND r.role='STAFF'
		WHERE u.brand_id=$1 ORDER BY u.full_name NULLS LAST`, s.brandID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []StaffRow{}
	for rows.Next() {
		var r StaffRow
		if err := rows.Scan(&r.ID, &r.Name, &r.Phone, &r.Email, &r.Status, &r.Roles, &r.CreatedAt, &r.StaffRoles); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// SetStaffRoles gives a person dashboard access with exactly these roles (none = no access), audited.
func (s *Service) SetStaffRoles(ctx context.Context, actorID, rawPhone, name string, roles []string) (*User, error) {
	normalised, err := phone.Normalize(rawPhone, s.country)
	if err != nil {
		return nil, ErrPhoneInvalid
	}
	u, err := s.EnsureUser(ctx, normalised, RoleStaff, name)
	if err != nil {
		return nil, err
	}
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.roleNamesExist(ctx, tx, roles); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM staff_roles WHERE user_id=$1`, u.ID); err != nil {
			return err
		}
		for _, r := range roles {
			if r = strings.TrimSpace(r); r != "" {
				if _, err := tx.Exec(ctx, `INSERT INTO staff_roles (user_id, role) VALUES ($1, $2)`, u.ID, r); err != nil {
					return err
				}
			}
		}
		if len(roles) == 0 {
			if _, err := tx.Exec(ctx, `DELETE FROM user_roles WHERE user_id=$1 AND role='STAFF'`, u.ID); err != nil {
				return err
			}
		}
		return audit.Record(ctx, tx, s.brandID, actorID, "staff.roles", "user:"+u.ID, "", map[string]any{"roles": roles})
	})
	return u, err
}

// Names returns full names for user ids in one query (missing names are absent).
func (s *Service) Names(ctx context.Context, ids []string) (map[string]string, error) {
	out := map[string]string{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx, `SELECT id, coalesce(full_name, phone) FROM users WHERE id = ANY($1::uuid[])`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[id] = name
	}
	return out, rows.Err()
}

// Phone returns a user's phone number (E.164).
func (s *Service) Phone(ctx context.Context, userID string) (string, error) {
	var p string
	err := s.pool.QueryRow(ctx, `SELECT phone FROM users WHERE id=$1`, userID).Scan(&p)
	return p, err
}
