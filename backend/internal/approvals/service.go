package approvals

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kalen-limited/hauler-rider/backend/internal/audit"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/cache"
)

// workflowCheckTTL bounds how long a workflow change made in another process
// takes to reach this one (every gated write checks the cached workflow).
const workflowCheckTTL = 5 * time.Second

// Service owns workflows and approval requests.
type Service struct {
	store store
	cat   *Catalogue
	staff Staff
	rules RuleSource
	flows *cache.Versioned[map[string]Workflow]
	now   func() time.Time
}

// Deps is everything the service needs.
type Deps struct {
	Pool      *pgxpool.Pool
	BrandID   string
	Catalogue *Catalogue
	Staff     Staff
	Rules     RuleSource
}

// New builds the service.
func New(d Deps) *Service {
	s := &Service{store: store{pool: d.Pool, brandID: d.BrandID}, cat: d.Catalogue, staff: d.Staff, rules: d.Rules, now: time.Now}
	s.flows = cache.New(configScope, s.store, s.store.workflows, workflowCheckTTL)
	return s
}

// Catalogue is the registered actions.
func (s *Service) Catalogue() *Catalogue { return s.cat }

// workflow is an action's current workflow from the cache (zero value = never saved, off).
func (s *Service) workflow(ctx context.Context, action string) (Workflow, error) {
	all, _, err := s.flows.Get(ctx)
	if err != nil {
		return Workflow{}, err
	}
	return all[action], nil
}

// Enabled reports from the cache whether an action's workflow is on, so a
// gate can skip extra lookups (e.g. reading the payout) when it is off.
func (s *Service) Enabled(ctx context.Context, action string) bool {
	w, err := s.workflow(ctx, action)
	return err != nil || (w.Enabled && len(w.Definition.Steps) > 0) // unsure: let Submit decide
}

// Submit offers a change to its action's workflow. nil = the workflow is off
// or its conditions do not match: the caller applies the change itself. A
// request means the change is held for approval (the same pending change
// asked again returns the same request).
func (s *Service) Submit(ctx context.Context, sub Submission) (*Request, error) {
	a, ok := s.cat.Get(sub.Action)
	if !ok {
		return nil, fmt.Errorf("approvals: action %q is not registered", sub.Action)
	}
	w, err := s.workflow(ctx, sub.Action)
	if err != nil {
		return nil, err
	}
	if !w.Enabled || len(w.Definition.Steps) == 0 || !w.Definition.Conditions.Matches(sub.Country, sub.Amount) {
		return nil, nil
	}
	if strings.TrimSpace(sub.Reason) == "" {
		return nil, ErrReasonRequired
	}
	n, err := s.prepare(ctx, a, w, sub)
	if err != nil {
		return nil, err
	}
	var id string
	err = pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		var created bool
		if id, created, err = s.store.insertRequest(ctx, tx, n); err != nil || !created {
			return err
		}
		return audit.Record(ctx, tx, s.store.brandID, sub.RequesterID, "approval.requested", "approval:"+id, sub.Reason,
			map[string]any{"action": sub.Action, "target": sub.Target, "workflow_version": w.Version})
	})
	if err != nil {
		return nil, err
	}
	return s.store.request(ctx, s.store.pool, id, false)
}

// prepare encodes the payload, builds the readable summary and the expiry.
func (s *Service) prepare(ctx context.Context, a Action, w Workflow, sub Submission) (newRequest, error) {
	sub.Reason = strings.TrimSpace(sub.Reason)
	sub.Country = strings.ToUpper(sub.Country)
	payload, err := json.Marshal(sub.Payload)
	if err != nil {
		return newRequest{}, err
	}
	summary := Summary{Title: a.Label, Changes: []Change{}}
	if a.Describe != nil {
		if summary, err = a.Describe(ctx, payload); err != nil {
			return newRequest{}, err
		}
	}
	sum := sha256.Sum256(append([]byte(sub.Action+"\n"), payload...))
	hours := w.Definition.ExpireHours
	if hours == 0 {
		hours = DefaultExpireHours
	}
	return newRequest{sub: sub, payload: payload, hash: hex.EncodeToString(sum[:]), summary: summary, workflow: w,
		expiresAt: s.now().Add(time.Duration(hours) * time.Hour)}, nil
}
