package approvals

import (
	"context"
	"slices"
)

// Inbox page sizes.
const (
	defaultListLimit = 50
	maxListLimit     = 200
	// countScanLimit bounds the pending requests the nav badge looks at.
	countScanLimit = 200
)

// visibleTo reports whether a viewer may see a request: their own always;
// others when the viewer is not country-limited or the request is in one of
// their countries (a country-less request needs an unlimited viewer).
func (r *Request) visibleTo(v Viewer) bool {
	if r.Requester.ID == v.ID || len(v.Countries) == 0 {
		return true
	}
	return r.Country != nil && slices.Contains(v.Countries, *r.Country)
}

// label fills the action's display label from the catalogue.
func (s *Service) label(r *Request) {
	if a, ok := s.cat.Get(r.Action); ok {
		r.ActionLabel = a.Label
	} else {
		r.ActionLabel = r.Action
	}
}

// List returns one inbox box for a viewer, with can_decide set per request.
func (s *Service) List(ctx context.Context, v Viewer, box string, limit int) ([]Request, error) {
	if limit <= 0 || limit > maxListLimit {
		limit = defaultListLimit
	}
	if box != BoxMine && box != BoxDone {
		box = BoxPending
	}
	list, err := s.store.list(ctx, listFilter{box: box, viewerID: v.ID, countries: v.Countries, limit: limit})
	if err != nil {
		return nil, err
	}
	g, err := s.staff.Grants(ctx, v.ID)
	if err != nil {
		return nil, err
	}
	for i := range list {
		s.label(&list[i])
		list[i].CanDecide = list[i].canDecide(v.ID, g)
	}
	return list, nil
}

// Actionable counts the pending requests the viewer could decide now (the nav badge).
func (s *Service) Actionable(ctx context.Context, v Viewer) (int, error) {
	list, err := s.List(ctx, v, BoxPending, countScanLimit)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, r := range list {
		if r.CanDecide {
			n++
		}
	}
	return n, nil
}

// Get returns one request the viewer may see.
func (s *Service) Get(ctx context.Context, v Viewer, id string) (*Request, error) {
	r, err := s.store.request(ctx, s.store.pool, id, false)
	if err != nil {
		return nil, err
	}
	if !r.visibleTo(v) {
		return nil, ErrNotFound
	}
	g, err := s.staff.Grants(ctx, v.ID)
	if err != nil {
		return nil, err
	}
	s.label(r)
	r.CanDecide = r.canDecide(v.ID, g)
	return r, nil
}
