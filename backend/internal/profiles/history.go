package profiles

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// History bounds.
const (
	RecordsExportMaxDays = 31
	HistoryMaxDays       = 366
	SimulateMaxOffset    = 7 * 24 * time.Hour
	// historicAfter: a time older than this is answered from history, not from the live snapshot.
	historicAfter      = time.Minute
	historyEventsLimit = 500
)

// History error codes.
var (
	ErrAsOfInFuture    = apperr.New("AS_OF_IN_FUTURE", http.StatusBadRequest, "")
	ErrRangeTooLarge   = apperr.New("RANGE_TOO_LARGE", http.StatusBadRequest, "")
	ErrRangeInvalid    = apperr.New("RANGE_INVALID", http.StatusBadRequest, "")
	ErrSimulateAtRange = apperr.New("SIMULATE_AT_OUT_OF_RANGE", http.StatusBadRequest, "")
)

// AsOfCharge is a charge rule that would match, with the profile it comes from.
type AsOfCharge struct {
	ChargeRule
	ProfileID string `json:"profile_id"`
	Version   int    `json:"version"`
	MatchedAt string `json:"matched_at"`
}

// AsOfResult answers "which profile priced this point at that time".
type AsOfResult struct {
	At      time.Time    `json:"at"`
	Profile Ref          `json:"profile"`
	Scope   ScopeRef     `json:"scope"`
	Default Ref          `json:"default_profile"`
	Terms   Effective    `json:"terms"`
	Charges []AsOfCharge `json:"charges"`
	Trace   Trace        `json:"resolution"`
	// Approximate is true when a region's shape changed after `at` (regions use today's cells).
	Approximate bool     `json:"approximate"`
	Notes       []string `json:"notes"`
}

// AsOf resolves a point as it would have been priced at in.At, from history:
// the assignments open then, each profile's status and window then, the
// versions in force then and the geofence shapes saved by then. It runs the
// same Resolve as live quotes.
func (s *Service) AsOf(ctx context.Context, in Input) (*AsOfResult, error) {
	if in.At.IsZero() || in.At.After(s.now()) {
		return nil, ErrAsOfInFuture
	}
	res, err := s.resolveAt(ctx, in, true, "")
	if err != nil {
		return nil, err
	}
	out := &AsOfResult{At: in.At, Profile: res.Profile, Scope: res.Scope, Default: res.Default, Terms: res.Terms, Trace: res.Trace,
		Charges: make([]AsOfCharge, 0, len(res.Charges)), Notes: []string{}}
	for _, c := range res.Charges {
		out.Charges = append(out.Charges, AsOfCharge{ChargeRule: c.ChargeRule, ProfileID: c.ProfileID, Version: c.Version, MatchedAt: c.MatchedAt})
	}
	if out.Approximate, err = s.store.areasChangedSince(ctx, []string{in.AreaID, in.DropAreaID}, in.At); err != nil {
		return nil, err
	}
	if out.Approximate {
		out.Notes = append(out.Notes, "REGION_SHAPE_CHANGED")
	}
	return out, nil
}

// Preview resolves a trip like a quote at in.At (within ±7 days), optionally
// letting a DRAFT or INACTIVE profile take part as if it were active. Nothing
// is saved. A past time is answered from history.
func (s *Service) Preview(ctx context.Context, in Input, previewProfileID string) (*Resolution, error) {
	now := s.now()
	if in.At.IsZero() {
		in.At = now
	}
	if in.At.Before(now.Add(-SimulateMaxOffset)) || in.At.After(now.Add(SimulateMaxOffset)) {
		return nil, ErrSimulateAtRange
	}
	return s.resolveAt(ctx, in, in.At.Before(now.Add(-historicAfter)), previewProfileID)
}

// resolveAt resolves with the snapshot and geofence index rebuilt from history
// at in.At (historic), or with the live ones (which hold future versions too).
func (s *Service) resolveAt(ctx context.Context, in Input, historic bool, previewID string) (*Resolution, error) {
	in.HasDropoff = in.HasDropoff || in.Dropoff != nil
	var snap *Snapshot
	var err error
	if historic {
		if snap, err = s.store.snapshotAt(ctx, in.Country, in.At); err != nil {
			return nil, err
		}
		if s.fences != nil {
			ix, err := s.fences.IndexAt(ctx, in.Country, in.At)
			if err != nil {
				return nil, err
			}
			locateIn(ix, &in)
		}
	} else {
		if snap, err = s.Snapshot(ctx); err != nil {
			return nil, err
		}
		if err := s.locate(ctx, &in); err != nil {
			return nil, err
		}
	}
	if previewID != "" {
		if snap, err = withPreview(snap, previewID, in.Country); err != nil {
			return nil, err
		}
	}
	return Resolve(snap, in)
}

// withPreview copies the snapshot with one profile switched on and unbounded.
func withPreview(snap *Snapshot, id, country string) (*Snapshot, error) {
	p, ok := snap.Profiles[id]
	if !ok || p.Country != country {
		return nil, ErrProfileNotFound
	}
	cp := *snap
	cp.Profiles = make(map[string]SnapProfile, len(snap.Profiles))
	for k, v := range snap.Profiles {
		cp.Profiles[k] = v
	}
	p.Status, p.ActiveFrom, p.ActiveUntil = StatusActive, nil, nil
	cp.Profiles[id] = p
	return &cp, nil
}

// TimelineEntry is one assignment in a scope's history.
type TimelineEntry struct {
	AssignmentID  string     `json:"assignment_id"`
	ProfileID     string     `json:"profile_id"`
	ProfileName   string     `json:"profile_name"`
	ProfileStatus string     `json:"profile_status"`
	Kind          string     `json:"kind"`
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Priority      int        `json:"priority"`
	ValidFrom     time.Time  `json:"valid_from"`
	ValidTo       *time.Time `json:"valid_to"`
	Reason        string     `json:"reason"`
	CloseReason   *string    `json:"close_reason"`
	AssignedBy    *string    `json:"assigned_by"`
	ClosedBy      *string    `json:"closed_by"`
}

// TimelineEvent is a profile event with the profile it belongs to.
type TimelineEvent struct {
	Event
	ProfileID   string `json:"profile_id"`
	ProfileName string `json:"profile_name"`
}

// HistoryQuery filters the timeline (scope optional).
type HistoryQuery struct {
	Country   string
	ScopeKind string
	ScopeID   string
	From, To  time.Time
}

// Timeline is a country's (or one scope's) assignment history and profile events in a range.
type Timeline struct {
	Assignments []TimelineEntry `json:"assignments"`
	Events      []TimelineEvent `json:"events"`
}

// History returns which profile was assigned where, from when to when, who
// changed it and why, for assignments that overlap [From, To].
func (s *Service) History(ctx context.Context, q HistoryQuery) (*Timeline, error) {
	if err := checkRange(q.From, q.To, HistoryMaxDays); err != nil {
		return nil, err
	}
	as, err := s.store.timeline(ctx, q)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(as))
	seen := map[string]bool{}
	for _, a := range as {
		if !seen[a.ProfileID] {
			seen[a.ProfileID] = true
			ids = append(ids, a.ProfileID)
		}
	}
	evs, err := s.store.timelineEvents(ctx, q, ids)
	if err != nil {
		return nil, err
	}
	return &Timeline{Assignments: as, Events: evs}, nil
}

// checkRange refuses an empty, reversed or too long range.
func checkRange(from, to time.Time, maxDays int) error {
	if from.IsZero() || to.IsZero() || !to.After(from) {
		return ErrRangeInvalid
	}
	if to.Sub(from) > time.Duration(maxDays)*24*time.Hour {
		return ErrRangeTooLarge
	}
	return nil
}

// RecordRow is one finalised pricing record in the CSV export.
type RecordRow struct {
	TripID         string
	FinalizedAt    time.Time
	Country        string
	ScopeKind      string
	ScopeName      string
	ProfileID      string
	ProfileName    string
	ProfileVersion int
	DefaultVersion string
	Source         string
	Charges        int
	Final          json.RawMessage
	RecordHash     string
}

// ExportRecords streams a country's finalised records in [from, to] (≤ 31 days) to emit.
func (s *Service) ExportRecords(ctx context.Context, country string, from, to time.Time, emit func(RecordRow) error) error {
	if err := checkRange(from, to, RecordsExportMaxDays); err != nil {
		return err
	}
	return s.store.finalRecords(ctx, country, from, to, emit)
}
