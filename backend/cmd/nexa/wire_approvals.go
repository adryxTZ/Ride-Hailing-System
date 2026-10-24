package main

import (
	"context"

	"github.com/riverqueue/river"

	"github.com/kalen-limited/hauler-rider/backend/internal/approvals"
	"github.com/kalen-limited/hauler-rider/backend/internal/approvals/actions"
	"github.com/kalen-limited/hauler-rider/backend/internal/identity"
)

// registerApprovalWorkers adds the periodic expiry of overdue approval requests.
func registerApprovalWorkers(workers *river.Workers) (*approvals.ExpireWorker, []*river.PeriodicJob) {
	w := &approvals.ExpireWorker{}
	river.AddWorker(workers, w)
	return w, []*river.PeriodicJob{
		river.NewPeriodicJob(river.PeriodicInterval(approvals.ExpireEvery), func() (river.JobArgs, *river.InsertOpts) {
			return approvals.ExpireArgs{}, nil
		}, &river.PeriodicJobOpts{ID: "approvals.expire", RunOnStart: true}),
	}
}

// startApprovals builds the approval workflows with the Platform's catalogue
// of approvable actions (only those whose module is wired).
func startApprovals(bs *bootstrap, brandID string, w *approvals.ExpireWorker, ident *identity.Service, m actions.Modules) *approvals.Service {
	cat := approvals.NewCatalogue()
	m.Identity = ident
	actions.Register(cat, m)
	svc := approvals.New(approvals.Deps{Pool: bs.pool, BrandID: brandID, Catalogue: cat,
		Staff: approvalStaff{ident: ident}, Rules: approvalRules(bs, ident)})
	w.Svc = svc
	return svc
}

// approvalStaff adapts identity to what approvals needs.
type approvalStaff struct{ ident *identity.Service }

// Grants are the person's current roles and permissions.
func (a approvalStaff) Grants(ctx context.Context, userID string) (approvals.Grants, error) {
	roles, perms, err := a.ident.StaffGrants(ctx, userID)
	return approvals.Grants{Roles: roles, Permissions: perms}, err
}

// Names maps user ids to display names.
func (a approvalStaff) Names(ctx context.Context, ids []string) (map[string]string, error) {
	return a.ident.Names(ctx, ids)
}

// approvalRules lists what workflow steps and conditions may name.
func approvalRules(bs *bootstrap, ident *identity.Service) approvals.RuleSource {
	return func(ctx context.Context) (approvals.Rules, error) {
		roles, err := ident.RoleNames(ctx)
		if err != nil {
			return approvals.Rules{}, err
		}
		r := approvals.Rules{Roles: roles}
		for _, p := range identity.Permissions {
			r.Permissions = append(r.Permissions, p.Key)
		}
		for _, c := range bs.brand.Countries {
			r.Countries = append(r.Countries, c.Code)
		}
		return r, nil
	}
}
