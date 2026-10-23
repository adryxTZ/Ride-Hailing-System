package identity

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kalen-limited/hauler-rider/backend/internal/audit"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// Role is a named set of permissions staff can hold.
type Role struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Permissions []string  `json:"permissions"`
	System      bool      `json:"system"` // Super Admin: cannot be edited or deleted
	Members     int       `json:"members"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// RoleInput is what a Super Admin sends to create or change a role.
type RoleInput struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
	Reason      string   `json:"reason"`
}

// Role errors.
var (
	ErrRoleNotFound   = apperr.New("ROLE_NOT_FOUND", http.StatusNotFound, "")
	ErrRoleLocked     = apperr.New("ROLE_LOCKED", http.StatusConflict, "")
	ErrRoleInUse      = apperr.New("ROLE_IN_USE", http.StatusConflict, "")
	ErrRoleNameTaken  = apperr.New("ROLE_NAME_TAKEN", http.StatusConflict, "")
	ErrRoleUnknown    = apperr.New("ROLE_UNKNOWN", http.StatusBadRequest, "")
	ErrReasonRequired = apperr.New("REASON_REQUIRED", http.StatusBadRequest, "")
)

const roleColumns = `r.id, r.name, r.description, r.permissions, r.system,
	(SELECT count(*) FROM staff_roles sr JOIN users u ON u.id=sr.user_id WHERE sr.role=r.name AND u.brand_id=r.brand_id), r.updated_at`

// ListRoles returns every role with how many people hold it.
func (s *Service) ListRoles(ctx context.Context) ([]Role, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+roleColumns+` FROM roles r WHERE r.brand_id=$1 ORDER BY r.system DESC, r.name`, s.brandID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Role])
}

// validateRole checks a role's name and that every permission is known.
func validateRole(in RoleInput) error {
	if strings.TrimSpace(in.Reason) == "" {
		return ErrReasonRequired
	}
	if strings.TrimSpace(in.Name) == "" || len(in.Name) > 60 {
		return apperr.Validation.WithDetails(map[string]any{"fields": []string{"name"}})
	}
	for _, p := range in.Permissions {
		if !knownPermission(p) {
			return apperr.Validation.WithDetails(map[string]any{"fields": []string{"permissions"}, "unknown": p})
		}
	}
	return nil
}

// CreateRole adds a role (audited).
func (s *Service) CreateRole(ctx context.Context, actorID string, in RoleInput) (*Role, error) {
	if err := validateRole(in); err != nil {
		return nil, err
	}
	var id string
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `INSERT INTO roles (brand_id, name, description, permissions) VALUES ($1,$2,$3,$4) RETURNING id`,
			s.brandID, strings.TrimSpace(in.Name), strings.TrimSpace(in.Description), uniq(in.Permissions)).Scan(&id)
		if isUnique(err) {
			return ErrRoleNameTaken
		}
		if err != nil {
			return err
		}
		return audit.Record(ctx, tx, s.brandID, actorID, "role.create", "role:"+id, in.Reason, in)
	})
	if err != nil {
		return nil, err
	}
	return s.role(ctx, id)
}

// UpdateRole renames a role or changes its permissions (audited). People holding
// it get the new permissions at their next token refresh (within 15 minutes).
func (s *Service) UpdateRole(ctx context.Context, actorID, id string, in RoleInput) (*Role, error) {
	if err := validateRole(in); err != nil {
		return nil, err
	}
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var oldName string
		var system bool
		err := tx.QueryRow(ctx, `SELECT name, system FROM roles WHERE id=$1 AND brand_id=$2 FOR UPDATE`, id, s.brandID).Scan(&oldName, &system)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrRoleNotFound
		}
		if err != nil {
			return err
		}
		if system {
			return ErrRoleLocked
		}
		name := strings.TrimSpace(in.Name)
		_, err = tx.Exec(ctx, `UPDATE roles SET name=$2, description=$3, permissions=$4, updated_at=now() WHERE id=$1`, id, name, strings.TrimSpace(in.Description), uniq(in.Permissions))
		if isUnique(err) {
			return ErrRoleNameTaken
		}
		if err != nil {
			return err
		}
		if name != oldName { // people keep the role through a rename
			if _, err := tx.Exec(ctx, `UPDATE staff_roles sr SET role=$2 FROM users u WHERE u.id=sr.user_id AND u.brand_id=$3 AND sr.role=$1`, oldName, name, s.brandID); err != nil {
				return err
			}
		}
		return audit.Record(ctx, tx, s.brandID, actorID, "role.update", "role:"+id, in.Reason, in)
	})
	if err != nil {
		return nil, err
	}
	return s.role(ctx, id)
}

// DeleteRole removes a role nobody holds (audited).
func (s *Service) DeleteRole(ctx context.Context, actorID, id, reason string) error {
	if strings.TrimSpace(reason) == "" {
		return ErrReasonRequired
	}
	r, err := s.role(ctx, id)
	if err != nil {
		return err
	}
	if r.System {
		return ErrRoleLocked
	}
	if r.Members > 0 {
		return ErrRoleInUse.WithDetails(map[string]any{"members": r.Members})
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM roles WHERE id=$1 AND brand_id=$2`, id, s.brandID); err != nil {
			return err
		}
		return audit.Record(ctx, tx, s.brandID, actorID, "role.delete", "role:"+id, reason, map[string]any{"name": r.Name})
	})
}

func (s *Service) role(ctx context.Context, id string) (*Role, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+roleColumns+` FROM roles r WHERE r.id=$1 AND r.brand_id=$2`, id, s.brandID)
	if err != nil {
		return nil, err
	}
	r, err := pgx.CollectOneRow(rows, pgx.RowToStructByPos[Role])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRoleNotFound
	}
	return &r, err
}

// roleNamesExist checks every name is a role of the brand.
func (s *Service) roleNamesExist(ctx context.Context, tx pgx.Tx, names []string) error {
	if len(names) == 0 {
		return nil
	}
	var n int
	if err := tx.QueryRow(ctx, `SELECT count(DISTINCT name) FROM roles WHERE brand_id=$1 AND name = ANY($2)`, s.brandID, uniq(names)).Scan(&n); err != nil {
		return err
	}
	if n != len(uniq(names)) {
		return ErrRoleUnknown
	}
	return nil
}

// permissionsOf is every permission a person holds through their staff roles.
func (s *Service) permissionsOf(ctx context.Context, tx pgx.Tx, userID string) ([]string, error) {
	rows, err := tx.Query(ctx, `SELECT DISTINCT unnest(r.permissions) FROM staff_roles sr JOIN roles r ON r.name=sr.role AND r.brand_id=$2 WHERE sr.user_id=$1 ORDER BY 1`, userID, s.brandID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

func uniq(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, v := range in {
		if v = strings.TrimSpace(v); v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

func isUnique(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
