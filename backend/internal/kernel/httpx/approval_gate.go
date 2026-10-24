package httpx

import (
	"net/http"
	"slices"

	"github.com/kalen-limited/hauler-rider/backend/internal/approvals"
)

// approvalAccepted is the 202 body of a gated endpoint whose change now waits
// for approval: no error, and the request the client can link to.
type approvalAccepted struct {
	ApprovalRequest *approvals.Request `json:"approval_request"`
}

// sentForApproval is the approval gate every gated handler calls after its
// own checks and before applying the change. When the action's workflow is
// on and its conditions match, the change is stored as an approval request
// and 202 {approval_request} is written: the handler must stop (true). false
// means apply the change now, exactly as before (also when approvals are off).
func sentForApproval(w http.ResponseWriter, req *http.Request, d Deps, sub approvals.Submission) bool {
	if d.Approvals == nil {
		return false
	}
	sub.RequesterID = claimsFrom(req.Context()).Subject
	r, err := d.Approvals.Submit(req.Context(), sub)
	if err != nil {
		writeDomainError(w, req, err)
		return true
	}
	if r == nil {
		return false
	}
	writeJSON(w, http.StatusAccepted, approvalAccepted{ApprovalRequest: r})
	return true
}

// deref reads an optional string ("" when absent).
func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// approvalViewer is the signed-in staff member as the approvals module sees
// them: the countries they are limited to (a region limit counts as its country).
func approvalViewer(req *http.Request) approvals.Viewer {
	c := claimsFrom(req.Context())
	v := approvals.Viewer{ID: c.Subject}
	for _, a := range staffAccess(c) {
		if !slices.Contains(v.Countries, a.Country) {
			v.Countries = append(v.Countries, a.Country)
		}
	}
	return v
}
