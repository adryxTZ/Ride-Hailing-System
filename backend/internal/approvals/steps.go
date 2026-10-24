package approvals

import (
	"slices"
	"strings"
)

// permAll is the Super Admin permission that covers every other.
const permAll = "*"

// permMember marks a step member that is a permission rather than a role.
const permMember = "perm:"

// members are what a step asks for: each listed role, then the permission.
func (s Step) members() []string {
	out := slices.Clone(s.Roles)
	if s.Permission != "" {
		out = append(out, permMember+s.Permission)
	}
	return out
}

// holds reports whether a person has a member (a role, or a permission via "*").
func (g Grants) holds(member string) bool {
	if perm, ok := strings.CutPrefix(member, permMember); ok {
		return slices.Contains(g.Permissions, perm) || slices.Contains(g.Permissions, permAll)
	}
	return slices.Contains(g.Roles, member)
}

// mayAct reports whether a person may approve or reject on this step at all.
func (s Step) mayAct(g Grants) bool {
	return slices.ContainsFunc(s.members(), g.holds)
}

// approveAs is the member a new approval counts as, given the members already
// counted on this step; "" = the person cannot add an approval here (no
// matching role, or in ALL mode every role they hold is already covered).
func (s Step) approveAs(g Grants, counted []string) string {
	for _, m := range s.members() {
		if !g.holds(m) {
			continue
		}
		if s.Mode == ModeAll && slices.Contains(counted, m) {
			continue
		}
		return m
	}
	return ""
}

// done reports whether the approvals counted so far meet the step.
func (s Step) done(counted []string) bool {
	if s.Mode == ModeAll {
		for _, m := range s.members() {
			if !slices.Contains(counted, m) {
				return false
			}
		}
		return true
	}
	return len(counted) >= s.Approvals
}

// countedOn returns the members the approvals on one step counted as.
func countedOn(ds []Decision, step int) []string {
	var out []string
	for _, d := range ds {
		if d.Step == step && d.Decision == DecisionApprove {
			out = append(out, d.AsRole)
		}
	}
	return out
}

// decided reports whether a person already decided this request (any step).
func decided(ds []Decision, personID string) bool {
	return slices.ContainsFunc(ds, func(d Decision) bool { return d.Approver.ID == personID })
}

// canDecide is the inbox flag: the viewer is not the requester, has not
// decided yet, and could approve or reject the current step.
func (r *Request) canDecide(viewerID string, g Grants) bool {
	if r.Status != StatusPending || r.Requester.ID == viewerID || decided(r.Decisions, viewerID) {
		return false
	}
	if r.CurrentStep >= len(r.Workflow.Steps) {
		return false
	}
	return r.Workflow.Steps[r.CurrentStep].mayAct(g)
}
