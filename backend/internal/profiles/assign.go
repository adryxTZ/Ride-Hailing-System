package profiles

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/geofences"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// AssignInput puts a profile on a scope.
type AssignInput struct {
	Kind     string `json:"scope_kind"`
	ScopeID  string `json:"scope_id"`
	Priority int    `json:"priority"`
	Reason   string `json:"reason"`
}

// assignableKinds are the scope kinds a profile can be assigned to now (COUNTRY goes through MakeDefault).
var assignableKinds = map[string]bool{ScopeArea: true, ScopeDistrict: true, ScopeZone: true, ScopePlace: true}

// SetStatus switches a profile ACTIVE or INACTIVE (the default is always active).
func (s *Service) SetStatus(ctx context.Context, staffID, id, status, reason string) (*Profile, error) {
	p, err := s.store.profile(ctx, s.store.pool, id, false)
	if err != nil {
		return nil, err
	}
	err = s.write(ctx, p.Country, reason, func(tx pgx.Tx) error {
		cur, err := s.store.profile(ctx, tx, id, true)
		if err != nil {
			return err
		}
		if err := s.checkStatusChange(ctx, tx, cur, status); err != nil {
			return err
		}
		if status == StatusActive {
			if err := s.checkOwnTies(ctx, tx, cur); err != nil {
				return err
			}
			if err := s.checkLivePayees(ctx, tx, cur); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE pricing_profiles SET status=$2, updated_at=now() WHERE id=$1`, id, status); err != nil {
			return err
		}
		kind := EventActivated
		if status == StatusInactive {
			kind = EventDeactivated
		}
		if err := s.store.event(ctx, tx, id, kind, staffID, reason, nil, nil, map[string]any{"from": cur.Status}); err != nil {
			return err
		}
		return s.record(ctx, tx, staffID, "pricing.profile.status", id, reason, map[string]any{"status": status})
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

// checkLivePayees refuses switching on a profile whose current charges name a payee that was switched off since.
func (s *Service) checkLivePayees(ctx context.Context, tx pgx.Tx, p Profile) error {
	var off int
	err := tx.QueryRow(ctx, `SELECT count(*) FROM pricing_profile_charges c JOIN charge_payees y ON y.id=c.payee_id
		WHERE c.version_id = (SELECT id FROM pricing_profile_versions WHERE profile_id=$1 AND effective_from <= now() ORDER BY effective_from DESC LIMIT 1)
		  AND y.status <> 'ACTIVE'`, p.ID).Scan(&off)
	if err == nil && off > 0 {
		return ErrPayeeInvalid
	}
	return err
}

// checkOwnTies runs the tie check for every open geofence assignment of a profile being switched on.
func (s *Service) checkOwnTies(ctx context.Context, tx pgx.Tx, p Profile) error {
	open, err := s.store.assignments(ctx, tx, `a.valid_to IS NULL AND a.profile_id = $2`, p.ID)
	if err != nil {
		return err
	}
	for _, a := range open {
		if err := s.checkTies(ctx, tx, p.Country, a.Kind, a.ID, a.Priority, p.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) checkStatusChange(ctx context.Context, tx pgx.Tx, cur Profile, status string) error {
	if status != StatusActive && status != StatusInactive {
		return apperr.Validation.WithDetails(map[string]any{"fields": []map[string]string{{"path": "status", "code": "NOT_ALLOWED"}}})
	}
	if cur.Status == status {
		return ErrStatusUnchanged
	}
	if status == StatusInactive {
		if def, err := s.isDefault(ctx, tx, cur.ID); err != nil || def {
			return firstErr(err, ErrIsDefault)
		}
		return nil
	}
	var versions int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM pricing_profile_versions WHERE profile_id=$1`, cur.ID).Scan(&versions); err != nil {
		return err
	}
	if versions == 0 {
		return ErrNotPublished
	}
	return nil
}

// Assign opens an assignment of the profile to a scope (one open profile per scope).
func (s *Service) Assign(ctx context.Context, staffID, id string, in AssignInput) (*Scope, error) {
	if !assignableKinds[in.Kind] {
		return nil, ErrScopeKindUnavailable
	}
	canonical, err := uuid.Parse(in.ScopeID)
	if err != nil {
		return nil, ErrScopeNotFound
	}
	in.ScopeID = canonical.String() // the snapshot and the open-scope index compare text
	if in.Priority < PriorityMin || in.Priority > PriorityMax {
		return nil, apperr.Validation.WithDetails(map[string]any{"fields": []map[string]string{{"path": "priority", "code": "OUT_OF_RANGE"}}})
	}
	p, err := s.store.profile(ctx, s.store.pool, id, false)
	if err != nil {
		return nil, err
	}
	if err := s.checkScope(ctx, p, in); err != nil {
		return nil, err
	}
	var assignment string
	err = s.write(ctx, p.Country, in.Reason, func(tx pgx.Tx) error {
		if def, err := s.isDefault(ctx, tx, id); err != nil || def {
			return firstErr(err, ErrIsDefault)
		}
		if err := s.scopeFree(ctx, tx, in); err != nil {
			return err
		}
		if p.Status == StatusActive {
			if err := s.checkTies(ctx, tx, p.Country, in.Kind, in.ScopeID, in.Priority, p.ID); err != nil {
				return err
			}
		}
		if err := tx.QueryRow(ctx, `INSERT INTO pricing_profile_assignments (brand_id, country_code, profile_id, scope_kind, scope_id, priority, assigned_by, reason)
			VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,'')::uuid,$8) RETURNING id`, s.store.brandID, p.Country, id, in.Kind, in.ScopeID, in.Priority, staffID, in.Reason).Scan(&assignment); err != nil {
			return err
		}
		if err := s.store.event(ctx, tx, id, EventAssigned, staffID, in.Reason, nil, &assignment, map[string]any{"scope_kind": in.Kind, "scope_id": in.ScopeID}); err != nil {
			return err
		}
		return s.record(ctx, tx, staffID, "pricing.profile.assign", id, in.Reason, in)
	})
	if err != nil {
		return nil, err
	}
	scopes, err := s.store.assignments(ctx, s.store.pool, `a.id = $2`, assignment)
	if err != nil || len(scopes) == 0 {
		return nil, err
	}
	return &scopes[0], nil
}

// checkScope confirms the scope (a region, or an active geofence of that kind) exists in the profile's country.
func (s *Service) checkScope(ctx context.Context, p Profile, in AssignInput) error {
	country, ok, err := s.scopeCountry(ctx, in)
	if err != nil {
		return err
	}
	if !ok {
		return ErrScopeNotFound
	}
	if country != p.Country {
		return ErrScopeCountryMismatch
	}
	return nil
}

func (s *Service) scopeCountry(ctx context.Context, in AssignInput) (string, bool, error) {
	if in.Kind == ScopeArea {
		country, _, ok, err := s.areas.AreaInfo(ctx, in.ScopeID)
		return country, ok, err
	}
	if s.fences == nil {
		return "", false, ErrScopeKindUnavailable
	}
	country, _, kind, ok, err := s.fences.Info(ctx, in.ScopeID)
	return country, ok && kind == in.Kind, err
}

// checkTies refuses an active profile on a geofence that overlaps another of
// the same kind with the same priority whose profile is active too (precedence
// could not decide between them). Regions never overlap and need no check.
func (s *Service) checkTies(ctx context.Context, tx pgx.Tx, country, kind, scopeID string, priority int, exceptProfile string) error {
	if kind == ScopeArea || kind == ScopeCountry || s.fences == nil {
		return nil
	}
	overlaps, err := s.fences.Overlaps(ctx, country, scopeID)
	if err != nil {
		return err
	}
	var others []string
	for _, o := range overlaps {
		other := o.A
		if other.ID == scopeID {
			other = o.B
		}
		if other.Kind == kind {
			others = append(others, other.ID)
		}
	}
	if len(others) == 0 {
		return nil
	}
	rows, err := tx.Query(ctx, `SELECT p.id, p.name, a.scope_id FROM pricing_profile_assignments a JOIN pricing_profiles p ON p.id=a.profile_id
		WHERE a.brand_id=$1 AND a.valid_to IS NULL AND a.scope_kind=$2 AND a.scope_id = ANY($3) AND a.priority=$4 AND p.status='ACTIVE' AND p.id::text <> $5`,
		s.store.brandID, kind, others, priority, exceptProfile)
	if err != nil {
		return err
	}
	defer rows.Close()
	var conflicts []map[string]string
	for rows.Next() {
		var id, name, scope string
		if err := rows.Scan(&id, &name, &scope); err != nil {
			return err
		}
		conflicts = append(conflicts, map[string]string{"profile_id": id, "profile": name, "scope_id": scope})
	}
	if len(conflicts) > 0 {
		return ErrTie.WithDetails(map[string]any{"conflicts": conflicts})
	}
	return rows.Err()
}

// OpenOnScopes names the profile open on each of the given scopes (geofence pages).
func (s *Service) OpenOnScopes(ctx context.Context, scopeIDs []string) (map[string]geofences.Assigned, error) {
	rows, err := s.store.pool.Query(ctx, `SELECT a.scope_id, p.id, p.name, p.status FROM pricing_profile_assignments a JOIN pricing_profiles p ON p.id=a.profile_id
		WHERE a.brand_id=$1 AND a.valid_to IS NULL AND a.scope_id = ANY($2)`, s.store.brandID, scopeIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]geofences.Assigned{}
	for rows.Next() {
		var scope string
		var a geofences.Assigned
		if err := rows.Scan(&scope, &a.ID, &a.Name, &a.Status); err != nil {
			return nil, err
		}
		out[scope] = a
	}
	return out, rows.Err()
}

// scopeFree refuses a scope that already has an open profile (close it first).
func (s *Service) scopeFree(ctx context.Context, tx pgx.Tx, in AssignInput) error {
	var holder, name string
	err := tx.QueryRow(ctx, `SELECT p.id, p.name FROM pricing_profile_assignments a JOIN pricing_profiles p ON p.id=a.profile_id
		WHERE a.brand_id=$1 AND a.scope_kind=$2 AND a.scope_id=$3 AND a.valid_to IS NULL`, s.store.brandID, in.Kind, in.ScopeID).Scan(&holder, &name)
	if err == pgx.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	return ErrScopeAssigned.WithDetails(map[string]any{"profile": map[string]string{"id": holder, "name": name}})
}

// Unassign closes an assignment (rows are closed, never deleted; the default is only replaced).
func (s *Service) Unassign(ctx context.Context, staffID, id, assignmentID, reason string) error {
	p, err := s.store.profile(ctx, s.store.pool, id, false)
	if err != nil {
		return err
	}
	return s.write(ctx, p.Country, reason, func(tx pgx.Tx) error {
		var kind string
		var open bool
		err := tx.QueryRow(ctx, `SELECT scope_kind, valid_to IS NULL FROM pricing_profile_assignments WHERE id::text=$1 AND profile_id=$2 FOR UPDATE`, assignmentID, id).Scan(&kind, &open)
		if err == pgx.ErrNoRows {
			return ErrScopeNotFound
		}
		if err != nil {
			return err
		}
		if !open {
			return ErrAssignmentClosed
		}
		if kind == ScopeCountry {
			return ErrIsDefault
		}
		if err := closeAssignment(ctx, tx, assignmentID, staffID, reason); err != nil {
			return err
		}
		if err := s.store.event(ctx, tx, id, EventUnassigned, staffID, reason, nil, &assignmentID, nil); err != nil {
			return err
		}
		return s.record(ctx, tx, staffID, "pricing.profile.unassign", id, reason, map[string]any{"assignment": assignmentID})
	})
}

func closeAssignment(ctx context.Context, tx pgx.Tx, assignmentID, staffID, reason string) error {
	// clock_timestamp: a transaction that waited on the pricing lock must not close before a newer start.
	_, err := tx.Exec(ctx, `UPDATE pricing_profile_assignments SET valid_to=greatest(clock_timestamp(), valid_from), closed_by=NULLIF($2,'')::uuid, close_reason=$3 WHERE id::text=$1`, assignmentID, staffID, reason)
	return err
}

// MakeDefault makes an active, complete, unscoped profile its country's default,
// closing the old default's assignment in the same transaction.
func (s *Service) MakeDefault(ctx context.Context, staffID, id, reason string) (*Profile, error) {
	p, err := s.store.profile(ctx, s.store.pool, id, false)
	if err != nil {
		return nil, err
	}
	err = s.write(ctx, p.Country, reason, func(tx pgx.Tx) error {
		cur, err := s.store.profile(ctx, tx, id, true)
		if err != nil {
			return err
		}
		if err := s.checkCanBeDefault(ctx, tx, cur); err != nil {
			return err
		}
		return s.swapDefault(ctx, tx, staffID, cur, reason)
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

func (s *Service) checkCanBeDefault(ctx context.Context, tx pgx.Tx, p Profile) error {
	if def, err := s.isDefault(ctx, tx, p.ID); err != nil || def {
		return firstErr(err, ErrStatusUnchanged)
	}
	if p.Status != StatusActive {
		return ErrNotActive
	}
	if p.ActiveFrom != nil || p.ActiveUntil != nil {
		return ErrHasWindow
	}
	var scopes int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM pricing_profile_assignments WHERE profile_id=$1 AND valid_to IS NULL`, p.ID).Scan(&scopes); err != nil {
		return err
	}
	if scopes > 0 {
		return ErrHasScopes
	}
	vs, err := s.store.versions(ctx, tx, `v.profile_id = $2 AND v.effective_from <= now()`, p.ID)
	if err != nil {
		return err
	}
	if len(vs) == 0 {
		return ErrNotPublished
	}
	if m := missing(vs[0].Terms); len(m) > 0 {
		return ErrIncomplete.WithDetails(map[string]any{"fields": m})
	}
	return nil
}

func (s *Service) swapDefault(ctx context.Context, tx pgx.Tx, staffID string, p Profile, reason string) error {
	var oldID, oldAssignment string
	err := tx.QueryRow(ctx, `SELECT profile_id, id FROM pricing_profile_assignments WHERE brand_id=$1 AND scope_kind='COUNTRY' AND scope_id=$2 AND valid_to IS NULL FOR UPDATE`,
		s.store.brandID, p.Country).Scan(&oldID, &oldAssignment)
	if err != nil && err != pgx.ErrNoRows {
		return err
	}
	if oldAssignment != "" {
		if err := closeAssignment(ctx, tx, oldAssignment, staffID, reason); err != nil {
			return err
		}
		if err := s.store.event(ctx, tx, oldID, EventDefaultReplaced, staffID, reason, nil, &oldAssignment, map[string]any{"by": p.ID}); err != nil {
			return err
		}
	}
	var assignment string
	if err := tx.QueryRow(ctx, `INSERT INTO pricing_profile_assignments (brand_id, country_code, profile_id, scope_kind, scope_id, assigned_by, reason)
		VALUES ($1,$2,$3,'COUNTRY',$6,NULLIF($4,'')::uuid,$5) RETURNING id`, s.store.brandID, p.Country, p.ID, staffID, reason, p.Country).Scan(&assignment); err != nil {
		return err
	}
	if err := s.store.event(ctx, tx, p.ID, EventMadeDefault, staffID, reason, nil, &assignment, map[string]any{"replaced": oldID}); err != nil {
		return err
	}
	return s.record(ctx, tx, staffID, "pricing.profile.make_default", p.ID, reason, map[string]any{"replaced": oldID})
}

func firstErr(err, otherwise error) error {
	if err != nil {
		return err
	}
	return otherwise
}
