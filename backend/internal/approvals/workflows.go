package approvals

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/audit"
)

// ActionWorkflow is one catalogue action with its current workflow (the editor's row).
type ActionWorkflow struct {
	Action   Action   `json:"action"`
	Workflow Workflow `json:"workflow"`
}

// WorkflowDetail is one action's workflow with its saved history.
type WorkflowDetail struct {
	ActionWorkflow
	History []WorkflowVersion `json:"history"`
}

// emptyWorkflow is the never-saved, disabled workflow of an action.
func emptyWorkflow(action string) Workflow {
	return Workflow{Action: action, Definition: Definition{Steps: []Step{}, ExpireHours: DefaultExpireHours}}
}

// Workflows lists every approvable action with its workflow (off when never saved).
func (s *Service) Workflows(ctx context.Context) ([]ActionWorkflow, error) {
	all, _, err := s.flows.Get(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ActionWorkflow, 0, len(s.cat.order))
	for _, a := range s.cat.List() {
		w, ok := all[a.Key]
		if !ok {
			w = emptyWorkflow(a.Key)
		}
		out = append(out, ActionWorkflow{Action: a, Workflow: w})
	}
	return out, nil
}

// WorkflowOf returns one action's workflow and history.
func (s *Service) WorkflowOf(ctx context.Context, action string) (*WorkflowDetail, error) {
	a, ok := s.cat.Get(action)
	if !ok {
		return nil, ErrUnknownAction
	}
	all, err := s.store.workflows(ctx) // fresh: the editor shows what was just saved
	if err != nil {
		return nil, err
	}
	w, ok := all[action]
	if !ok {
		w = emptyWorkflow(action)
	}
	h, err := s.store.history(ctx, action)
	if err != nil {
		return nil, err
	}
	return &WorkflowDetail{ActionWorkflow: ActionWorkflow{Action: a, Workflow: w}, History: h}, nil
}

// SaveWorkflow validates and saves the next version of an action's workflow
// (audited). Pending requests keep the version they started under.
func (s *Service) SaveWorkflow(ctx context.Context, staffID, action string, in WorkflowInput) (*WorkflowDetail, error) {
	a, ok := s.cat.Get(action)
	if !ok {
		return nil, ErrUnknownAction
	}
	if in.Reason = strings.TrimSpace(in.Reason); in.Reason == "" {
		return nil, ErrReasonRequired
	}
	if in.Definition.Steps == nil {
		in.Definition.Steps = []Step{}
	}
	in.Definition.normalise()
	rules, err := s.rules(ctx)
	if err != nil {
		return nil, err
	}
	if problems := in.Definition.validate(a, in.Enabled, rules); len(problems) > 0 {
		return nil, ErrWorkflowInvalid.WithDetails(map[string]any{"problems": problems})
	}
	err = pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		version, err := s.store.saveWorkflow(ctx, tx, action, staffID, in)
		if err != nil {
			return err
		}
		if err := audit.Record(ctx, tx, s.store.brandID, staffID, "approval.workflow", "approval_workflow:"+action, in.Reason,
			map[string]any{"version": version, "enabled": in.Enabled, "definition": in.Definition}); err != nil {
			return err
		}
		return s.store.bump(ctx, tx)
	})
	if err != nil {
		return nil, err
	}
	s.flows.Stale()
	return s.WorkflowOf(ctx, action)
}
