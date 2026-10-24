package profiles

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
)

// snapshotAt rebuilds a country's snapshot as it was at `at` from the
// append-only history (five queries; admin only, never on the quote path).
func (s store) snapshotAt(ctx context.Context, country string, at time.Time) (*Snapshot, error) {
	snap := &Snapshot{Profiles: map[string]SnapProfile{}, Versions: map[string][]Version{}, Open: map[string]Scope{}}
	ps, err := s.profiles(ctx, country)
	if err != nil {
		return nil, err
	}
	later, err := s.eventsAfter(ctx, country, at)
	if err != nil {
		return nil, err
	}
	for _, p := range ps {
		cur := SnapProfile{ID: p.ID, Country: p.Country, Name: p.Name, Status: p.Status, ActiveFrom: p.ActiveFrom, ActiveUntil: p.ActiveUntil}
		snap.Profiles[p.ID] = rewind(cur, later[p.ID])
	}
	vs, err := s.versions(ctx, s.pool, `p.country_code=$2 AND v.id IN (SELECT DISTINCT ON (profile_id) id FROM pricing_profile_versions
		WHERE effective_from <= $3 ORDER BY profile_id, effective_from DESC, version DESC)`, country, at)
	if err != nil {
		return nil, err
	}
	for _, v := range vs {
		snap.Versions[v.ProfileID] = append(snap.Versions[v.ProfileID], v)
	}
	open, err := s.assignments(ctx, s.pool, `a.country_code=$2 AND a.valid_from <= $3 AND (a.valid_to IS NULL OR a.valid_to > $3)`, country, at)
	if err != nil {
		return nil, err
	}
	for _, a := range open {
		snap.Open[a.Kind+"/"+a.ID] = a
	}
	snap.Payees, err = s.payeeNames(ctx)
	return snap, err
}

// eventsAfter reads the status, window and name changes made after `at`, newest first, per profile.
func (s store) eventsAfter(ctx context.Context, country string, at time.Time) (map[string][]Event, error) {
	rows, err := s.pool.Query(ctx, `SELECT e.profile_id::text, e.at, e.kind, e.data FROM pricing_profile_events e
		JOIN pricing_profiles p ON p.id = e.profile_id
		WHERE e.brand_id=$1 AND p.country_code=$2 AND e.at > $3 AND e.kind = ANY($4)
		ORDER BY e.at DESC, e.id DESC`, s.brandID, country, at, []string{EventActivated, EventDeactivated, EventWindowChanged, EventRenamed})
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]Event{}
	for rows.Next() {
		var id string
		var e Event
		if err := rows.Scan(&id, &e.At, &e.Kind, &e.Data); err != nil {
			return nil, err
		}
		out[id] = append(out[id], e)
	}
	return out, rows.Err()
}

// rewind undoes, newest first, the changes made to a profile after a point in
// time: each event carries what was there before it.
func rewind(p SnapProfile, newestFirst []Event) SnapProfile {
	for _, e := range newestFirst {
		switch e.Kind {
		case EventActivated, EventDeactivated:
			if from, ok := e.Data["from"].(string); ok && from != "" {
				p.Status = from
			}
		case EventRenamed:
			if from, ok := e.Data["from"].(string); ok && from != "" {
				p.Name = from
			}
		case EventWindowChanged:
			if old, ok := e.Data["old"].(map[string]any); ok {
				p.ActiveFrom, p.ActiveUntil = timeOf(old["from"]), timeOf(old["until"])
			}
		}
	}
	return p
}

// timeOf reads a JSON time (RFC 3339 string or null).
func timeOf(v any) *time.Time {
	s, ok := v.(string)
	if !ok || s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return nil
	}
	return &t
}

// areasChangedSince reports whether any of the regions was edited after `at`.
func (s store) areasChangedSince(ctx context.Context, ids []string, at time.Time) (bool, error) {
	var changed bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM areas WHERE id::text = ANY($1) AND updated_at > $2)`, ids, at).Scan(&changed)
	return changed, err
}

// timeline reads assignments overlapping [From, To] from the named view.
func (s store) timeline(ctx context.Context, q HistoryQuery) ([]TimelineEntry, error) {
	rows, err := s.pool.Query(ctx, `SELECT assignment_id::text, profile_id::text, profile_name, profile_status, scope_kind, scope_id, scope_name,
		priority, valid_from, valid_to, reason, close_reason, assigned_by_name, closed_by_name
		FROM pricing_assignment_timeline
		WHERE brand_id=$1 AND country_code=$2 AND ($3='' OR scope_kind=$3) AND ($4='' OR scope_id=$4)
		  AND valid_from <= $6 AND (valid_to IS NULL OR valid_to >= $5)
		ORDER BY scope_kind, scope_name, valid_from DESC`, s.brandID, q.Country, q.ScopeKind, q.ScopeID, q.From, q.To)
	if err != nil {
		return nil, err
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByPos[TimelineEntry])
	if out == nil {
		out = []TimelineEntry{}
	}
	return out, err
}

// timelineEvents reads the events of the given profiles in [From, To] (newest first, bounded).
func (s store) timelineEvents(ctx context.Context, q HistoryQuery, profileIDs []string) ([]TimelineEvent, error) {
	out := []TimelineEvent{}
	if len(profileIDs) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx, `SELECT e.at, e.kind, e.version, e.assignment_id::text, e.actor_id::text, u.full_name, e.reason, e.data,
		e.profile_id::text, p.name
		FROM pricing_profile_events e JOIN pricing_profiles p ON p.id=e.profile_id LEFT JOIN users u ON u.id=e.actor_id
		WHERE e.brand_id=$1 AND e.profile_id::text = ANY($2) AND e.at BETWEEN $3 AND $4
		ORDER BY e.at DESC, e.id DESC LIMIT $5`, s.brandID, profileIDs, q.From, q.To, historyEventsLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var e TimelineEvent
		if err := rows.Scan(&e.At, &e.Kind, &e.Version, &e.AssignmentID, &e.ActorID, &e.ActorName, &e.Reason, &e.Data, &e.ProfileID, &e.ProfileName); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// finalRecords streams finalised records of a country in [from, to] (uses trip_pricing_records_final_idx).
func (s store) finalRecords(ctx context.Context, country string, from, to time.Time, emit func(RecordRow) error) error {
	rows, err := s.pool.Query(ctx, `SELECT r.trip_id::text, r.finalized_at, coalesce(r.country_code,''), r.scope_kind, coalesce(r.scope_name, r.scope_id),
		r.profile_id::text, coalesce(p.name,''), r.profile_version, coalesce(r.default_version_id::text,''), r.source,
		jsonb_array_length(r.charges), coalesce(r.final,'{}'), coalesce(r.record_hash,'')
		FROM trip_pricing_records r LEFT JOIN pricing_profiles p ON p.id=r.profile_id
		WHERE r.brand_id=$1 AND r.country_code=$2 AND r.stage='FINAL' AND r.finalized_at >= $3 AND r.finalized_at < $4
		ORDER BY r.finalized_at DESC`, s.brandID, country, from, to)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var r RecordRow
		var final []byte
		if err := rows.Scan(&r.TripID, &r.FinalizedAt, &r.Country, &r.ScopeKind, &r.ScopeName, &r.ProfileID, &r.ProfileName, &r.ProfileVersion,
			&r.DefaultVersion, &r.Source, &r.Charges, &final, &r.RecordHash); err != nil {
			return err
		}
		r.Final = json.RawMessage(final)
		if err := emit(r); err != nil {
			return err
		}
	}
	return rows.Err()
}
