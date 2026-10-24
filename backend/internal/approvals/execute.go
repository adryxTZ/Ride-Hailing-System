package approvals

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/audit"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// shortIDLength is how much of a request id the "(approved by …)" note shows.
const shortIDLength = 8

// maxExecutedReason is the longest reason modules accept (refunds cap at 1000).
const maxExecutedReason = 1000

// errUnavailable is stored when an approved action is no longer registered.
const errUnavailable = "APPROVAL_ACTION_UNAVAILABLE"

// execute runs an APPROVED request's action exactly once: the claim moves it
// to EXECUTING so a second caller does nothing. The outcome (result or error
// code) is stored and audited with the final approver as actor. It never
// fails the caller's request: a failure is recorded on the approval request.
func (s *Service) execute(ctx context.Context, id, finalApproverID string) {
	ctx = context.WithoutCancel(ctx) // the change must finish even if the approver's request ends
	claimed, err := s.store.claim(ctx, id)
	if err != nil || !claimed {
		if err != nil {
			slog.Error("approvals: claim", "request", id, "err", err)
		}
		return
	}
	r, err := s.store.request(ctx, s.store.pool, id, false)
	if err != nil {
		slog.Error("approvals: load approved request", "request", id, "err", err)
		return
	}
	result, errCode := s.run(ctx, r)
	s.record(ctx, r, finalApproverID, result, errCode)
}

// run calls the action's executor and returns its JSON result or an error code.
func (s *Service) run(ctx context.Context, r *Request) ([]byte, string) {
	a, ok := s.cat.Get(r.Action)
	if !ok {
		return nil, errUnavailable
	}
	out, err := a.Execute(ctx, Execution{RequestID: r.ID, RequesterID: r.Requester.ID, Reason: approvedReason(r), Payload: r.Payload})
	if err != nil {
		var e *apperr.E
		if errors.As(err, &e) {
			return nil, e.Code
		}
		slog.Error("approvals: executor failed", "request", r.ID, "action", r.Action, "err", err)
		return nil, apperr.Internal.Code
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return nil, apperr.Internal.Code
	}
	return raw, ""
}

// record stores the outcome and audits it.
func (s *Service) record(ctx context.Context, r *Request, actorID string, result []byte, errCode string) {
	status, action := StatusExecuted, "approval.executed"
	if errCode != "" {
		status, action = StatusFailed, "approval.failed"
	}
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		if err := s.store.finish(ctx, tx, r.ID, status, result, errCode); err != nil {
			return err
		}
		return audit.Record(ctx, tx, s.store.brandID, actorID, action, "approval:"+r.ID, approvedReason(r),
			map[string]any{"action": r.Action, "target": r.Target, "requester": r.Requester.ID, "approvers": approverIDs(r), "error": errCode})
	})
	if err != nil {
		slog.Error("approvals: record outcome", "request", r.ID, "err", err)
	}
}

// approvedReason is the requester's reason with who approved it.
func approvedReason(r *Request) string {
	var names []string
	for _, d := range r.Decisions {
		if d.Decision == DecisionApprove {
			names = append(names, d.Approver.Name)
		}
	}
	short := r.ID
	if len(short) > shortIDLength {
		short = short[:shortIDLength]
	}
	tail := fmt.Sprintf(" (approved by %s; approval %s)", strings.Join(names, ", "), short)
	base := r.Reason
	if room := maxExecutedReason - len(tail); len(base) > room && room > 0 {
		base = base[:room] // modules cap reasons; the approvers' names must survive
	}
	return base + tail
}

func approverIDs(r *Request) []string {
	var out []string
	for _, d := range r.Decisions {
		if d.Decision == DecisionApprove {
			out = append(out, d.Approver.ID)
		}
	}
	return out
}
