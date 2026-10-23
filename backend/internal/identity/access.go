package identity

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/audit"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// ScopePrefix marks a country/region limit inside the token's roles claim
// ("scope:TZ" for a whole country, "scope:TZ/<area id>" for one region).
const ScopePrefix = "scope:"

// Access is one country, or one region of it, a staff member may work in.
type Access struct {
	Country string `json:"country"`
	AreaID  string `json:"area_id,omitempty"` // "" = the whole country
}

// Session is one sign-in (one refresh-token family) as staff see it.
type Session struct {
	ID         string         `json:"id"`
	App        string         `json:"app"`
	IP         *string        `json:"ip"`
	Device     *string        `json:"device"`
	UserAgent  *string        `json:"user_agent"`
	Location   *LoginLocation `json:"location"`
	StartedAt  time.Time      `json:"started_at"`
	LastUsedAt time.Time      `json:"last_used_at"`
	EndedAt    *time.Time     `json:"ended_at"`
	EndReason  *string        `json:"end_reason"`
	Active     bool           `json:"active"`
}

// StaffDetail is one staff member with roles, limits and sign-in sessions.
type StaffDetail struct {
	StaffRow
	Access      []Access   `json:"access"`
	Permissions []string   `json:"permissions"`
	Sessions    []Session  `json:"sessions"`
	LastSignIn  *time.Time `json:"last_sign_in"`
}

// Errors for staff management.
var (
	ErrStaffNotFound   = apperr.New("STAFF_NOT_FOUND", http.StatusNotFound, "")
	ErrSessionNotFound = apperr.New("SESSION_NOT_FOUND", http.StatusNotFound, "")
	ErrNotYourself     = apperr.New("CANNOT_CHANGE_YOURSELF", http.StatusConflict, "")
)

// sessionLimit is how many recent sessions the staff page shows.
const sessionLimit = 30

// accessOf lists a person's country/region limits as token scopes.
func (s store) accessOf(ctx context.Context, q pgx.Tx, userID string) ([]string, error) {
	rows, err := q.Query(ctx, `SELECT country_code || coalesce('/' || area_id::text, '') FROM staff_access WHERE user_id=$1 ORDER BY 1`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// StaffDetail returns one staff member with roles, permissions, limits and sessions.
func (s *Service) StaffDetail(ctx context.Context, userID string) (*StaffDetail, error) {
	var d StaffDetail
	err := s.pool.QueryRow(ctx, `
		SELECT u.id, u.full_name, u.phone, u.email::text, u.status::text, '{STAFF}'::text[], u.created_at,
		  COALESCE((SELECT array_agg(sr.role ORDER BY sr.role) FROM staff_roles sr WHERE sr.user_id=u.id), '{}'),
		  (SELECT max(started_at) FROM sessions WHERE user_id=u.id AND app='admin')
		FROM users u JOIN user_roles r ON r.user_id=u.id AND r.role='STAFF' WHERE u.id=$1 AND u.brand_id=$2`, userID, s.brandID).
		Scan(&d.ID, &d.Name, &d.Phone, &d.Email, &d.Status, &d.Roles, &d.CreatedAt, &d.StaffRoles, &d.LastSignIn)
	if err == pgx.ErrNoRows {
		return nil, ErrStaffNotFound
	}
	if err != nil {
		return nil, err
	}
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if d.Permissions, err = s.permissionsOf(ctx, tx, userID); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT country_code, coalesce(area_id::text,'') FROM staff_access WHERE user_id=$1 ORDER BY 1, 2`, userID)
		if err != nil {
			return err
		}
		if d.Access, err = pgx.CollectRows(rows, pgx.RowToStructByPos[Access]); err != nil {
			return err
		}
		d.Sessions, err = s.sessionsOf(ctx, tx, userID)
		return err
	})
	return &d, err
}

func (s *Service) sessionsOf(ctx context.Context, tx pgx.Tx, userID string) ([]Session, error) {
	rows, err := tx.Query(ctx, `
		SELECT s.family_id, s.app, s.ip, s.device, s.user_agent, s.location, s.started_at, s.last_used_at, s.ended_at, s.end_reason,
		  s.ended_at IS NULL AND EXISTS (SELECT 1 FROM refresh_tokens rt WHERE rt.family_id=s.family_id AND rt.revoked_at IS NULL AND rt.expires_at > now())
		FROM sessions s WHERE s.user_id=$1 ORDER BY s.started_at DESC LIMIT $2`, userID, sessionLimit)
	if err != nil {
		return nil, err
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Session])
	if out == nil {
		out = []Session{}
	}
	return out, err
}

// EndSessions ends one session (sessionID) or every open one ("") of a person,
// audited. Their dashboard tokens stop working on the next request.
func (s *Service) EndSessions(ctx context.Context, actorID, userID, sessionID, reason string) error {
	if strings.TrimSpace(reason) == "" {
		return ErrReasonRequired
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE sessions SET ended_at=now(), ended_by=$3, end_reason=$4
			WHERE user_id=$1 AND ended_at IS NULL AND ($2 = '' OR family_id=$2::uuid)`, userID, sessionID, actorID, reason)
		if err != nil {
			return err
		}
		if sessionID != "" && tag.RowsAffected() == 0 {
			return ErrSessionNotFound
		}
		if _, err := tx.Exec(ctx, `UPDATE refresh_tokens SET revoked_at=now() WHERE user_id=$1 AND revoked_at IS NULL AND ($2 = '' OR family_id=$2::uuid)`, userID, sessionID); err != nil {
			return err
		}
		action := "staff.sessions.end_all"
		if sessionID != "" {
			action = "staff.session.end"
		}
		return audit.Record(ctx, tx, s.brandID, actorID, action, "user:"+userID, reason, map[string]any{"session": sessionID})
	})
}

// SetStaffAccess limits a person to countries/regions (empty = everywhere), audited.
// Takes effect at their next token refresh.
func (s *Service) SetStaffAccess(ctx context.Context, actorID, userID string, access []Access, reason string) error {
	if strings.TrimSpace(reason) == "" {
		return ErrReasonRequired
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM staff_access WHERE user_id=$1`, userID); err != nil {
			return err
		}
		for _, a := range access {
			country := strings.ToUpper(strings.TrimSpace(a.Country))
			if len(country) != 2 {
				return apperr.Validation.WithDetails(map[string]any{"fields": []string{"access.country"}})
			}
			if _, err := tx.Exec(ctx, `INSERT INTO staff_access (user_id, country_code, area_id)
				SELECT $1, $2, NULLIF($3,'')::uuid WHERE NULLIF($3,'') IS NULL OR EXISTS (SELECT 1 FROM areas WHERE id=NULLIF($3,'')::uuid AND country_code=$2)
				ON CONFLICT DO NOTHING`, userID, country, a.AreaID); err != nil {
				return err
			}
		}
		return audit.Record(ctx, tx, s.brandID, actorID, "staff.access", "user:"+userID, reason, map[string]any{"access": access})
	})
}

// SessionGate is what the API checks on every dashboard request: whether the
// account is still active and whether the token's session was ended.
type SessionGate struct {
	Active      bool
	EndedSince  map[string]bool // sessions ended (by id)
	RevokedFrom *time.Time      // tokens issued before this are void (account deactivated)
}

// GateFor reads the gate for a person in one query.
func (s *Service) GateFor(ctx context.Context, userID string) (SessionGate, error) {
	g := SessionGate{EndedSince: map[string]bool{}}
	var status string
	var ended []string
	err := s.pool.QueryRow(ctx, `SELECT u.status::text,
		  COALESCE((SELECT array_agg(family_id::text) FROM sessions WHERE user_id=u.id AND ended_at > now() - interval '1 hour'), '{}')
		FROM users u WHERE u.id=$1`, userID).Scan(&status, &ended)
	if err != nil {
		return g, err
	}
	g.Active = status == "ACTIVE"
	for _, id := range ended {
		g.EndedSince[id] = true
	}
	return g, nil
}
