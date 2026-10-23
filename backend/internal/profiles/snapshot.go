package profiles

import (
	"context"
	"time"
)

// loadSnapshot reads profiles, versions in force or due within the horizon,
// open assignments and payee names: five queries, run only when the pricing
// config version moves.
func (s store) loadSnapshot(ctx context.Context, now time.Time) (*Snapshot, error) {
	ver, err := s.Version(ctx, "pricing")
	if err != nil {
		return nil, err
	}
	snap := &Snapshot{Profiles: map[string]SnapProfile{}, Versions: map[string][]Version{}, Open: map[string]Scope{}, ConfigVersion: ver}
	ps, err := s.profiles(ctx, "")
	if err != nil {
		return nil, err
	}
	for _, p := range ps {
		snap.Profiles[p.ID] = SnapProfile{ID: p.ID, Country: p.Country, Name: p.Name, Status: p.Status, ActiveFrom: p.ActiveFrom, ActiveUntil: p.ActiveUntil}
	}
	vs, err := s.versions(ctx, s.pool, liveVersions, now, snapshotHorizon(now))
	if err != nil {
		return nil, err
	}
	for _, v := range vs {
		snap.Versions[v.ProfileID] = append(snap.Versions[v.ProfileID], v)
	}
	open, err := s.assignments(ctx, s.pool, `a.valid_to IS NULL`)
	if err != nil {
		return nil, err
	}
	for _, a := range open {
		snap.Open[a.Kind+"/"+a.ID] = a
	}
	snap.Payees, err = s.payeeNames(ctx)
	return snap, err
}
