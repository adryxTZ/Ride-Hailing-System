package approvals

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/audit"
)

// maxNoteLength bounds an approver's note.
const maxNoteLength = 1000

// errExpiredNow is returned from a decision's transaction after it marked the
// request expired, so the expiry commits while the caller gets ErrExpired.
var errExpiredNow = errors.New("approval expired during decision")

// Approve adds the viewer's approval to the current step. When it completes
// the last step the action runs at once (as the requester of record) and the
// request comes back EXECUTED or FAILED.
func (s *Service) Approve(ctx context.Context, v Viewer, id, note string) (*Request, error) {
	note, err := checkNote(note, false)
	if err != nil {
		return nil, err
	}
	g, err := s.staff.Grants(ctx, v.ID)
	if err != nil {
		return nil, err
	}
	final := false
	err = s.decide(ctx, v, id, func(tx pgx.Tx, r *Request) error {
		step := r.Workflow.Steps[r.CurrentStep]
		counted := countedOn(r.Decisions, r.CurrentStep)
		as := step.approveAs(g, counted)
		if as == "" {
			return ErrNotApprover
		}
		if err := s.store.addDecision(ctx, tx, r.ID, r.CurrentStep, v.ID, DecisionApprove, as, note); err != nil {
			return err
		}
		next, status := r.CurrentStep, StatusPending
		if step.done(append(counted, as)) {
			next++
			if next == len(r.Workflow.Steps) {
				status, final = StatusApproved, true
			}
		}
		if err := s.store.advance(ctx, tx, r.ID, next, status); err != nil {
			return err
		}
		return audit.Record(ctx, tx, s.store.brandID, v.ID, "approval.approved", "approval:"+r.ID, note,
			map[string]any{"step": r.CurrentStep, "as": as, "final": final})
	})
	if err != nil {
		return nil, err
	}
	if final {
		s.execute(ctx, id, v.ID)
	}
	return s.Get(ctx, v, id)
}

// Reject ends a request on the current step; a note is required.
func (s *Service) Reject(ctx context.Context, v Viewer, id, note string) (*Request, error) {
	note, err := checkNote(note, true)
	if err != nil {
		return nil, err
	}
	g, err := s.staff.Grants(ctx, v.ID)
	if err != nil {
		return nil, err
	}
	err = s.decide(ctx, v, id, func(tx pgx.Tx, r *Request) error {
		if !r.Workflow.Steps[r.CurrentStep].mayAct(g) {
			return ErrNotApprover
		}
		if err := s.store.addDecision(ctx, tx, r.ID, r.CurrentStep, v.ID, DecisionReject, "", note); err != nil {
			return err
		}
		if err := s.store.advance(ctx, tx, r.ID, r.CurrentStep, StatusRejected); err != nil {
			return err
		}
		return audit.Record(ctx, tx, s.store.brandID, v.ID, "approval.rejected", "approval:"+r.ID, note, map[string]any{"step": r.CurrentStep})
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, v, id)
}

// decide runs a decision on a locked, pending, unexpired request that the
// viewer may see, did not ask for and has not decided yet.
func (s *Service) decide(ctx context.Context, v Viewer, id string, fn func(tx pgx.Tx, r *Request) error) error {
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		r, err := s.store.request(ctx, tx, id, true)
		if err != nil {
			return err
		}
		if !r.visibleTo(v) {
			return ErrNotFound
		}
		if r.Status != StatusPending || r.CurrentStep >= len(r.Workflow.Steps) {
			return ErrNotPending
		}
		if !r.ExpiresAt.After(s.now()) {
			return errExpiredNow
		}
		if r.Requester.ID == v.ID {
			return ErrSelfApproval
		}
		if decided(r.Decisions, v.ID) {
			return ErrAlreadyDecided
		}
		return fn(tx, r)
	})
	if errors.Is(err, errExpiredNow) {
		if xerr := s.ExpireDue(ctx); xerr != nil {
			return xerr
		}
		return ErrExpired
	}
	return err
}

// Cancel withdraws a pending request; only its requester may.
func (s *Service) Cancel(ctx context.Context, v Viewer, id, note string) (*Request, error) {
	note, err := checkNote(note, false)
	if err != nil {
		return nil, err
	}
	err = pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		r, err := s.store.request(ctx, tx, id, true)
		if err != nil {
			return err
		}
		if r.Requester.ID != v.ID {
			return ErrNotRequester
		}
		if r.Status != StatusPending {
			return ErrNotPending
		}
		if err := s.store.advance(ctx, tx, r.ID, r.CurrentStep, StatusCancelled); err != nil {
			return err
		}
		return audit.Record(ctx, tx, s.store.brandID, v.ID, "approval.cancelled", "approval:"+r.ID, note, nil)
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, v, id)
}

// checkNote trims a note, bounds it, and requires it when asked.
func checkNote(note string, required bool) (string, error) {
	note = strings.TrimSpace(note)
	if required && note == "" {
		return "", ErrNoteRequired
	}
	if len(note) > maxNoteLength {
		return "", ErrNoteRequired.WithDetails(map[string]any{"max_length": maxNoteLength})
	}
	return note, nil
}
