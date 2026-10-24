package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kalen-limited/hauler-rider/backend/internal/approvals"
	"github.com/kalen-limited/hauler-rider/backend/internal/compliance"
	"github.com/kalen-limited/hauler-rider/backend/internal/identity"
	"github.com/kalen-limited/hauler-rider/backend/internal/privacy"
)

func privacyAction(svc *privacy.Service) approvals.Action {
	return approvals.Action{Key: PrivacyDelete, Group: groupTrust, Permission: identity.PermPrivacyHandle, Scoped: true,
		Label:       "Approve an account deletion",
		Description: "Anonymise a rider's or driver's account at their request (cannot be undone).",
		Describe:    describePrivacy(svc), Execute: executePrivacy(svc)}
}

func describePrivacy(svc *privacy.Service) approvals.Describer {
	return func(ctx context.Context, raw json.RawMessage) (approvals.Summary, error) {
		in, err := decode[PrivacyPayload](raw)
		if err != nil {
			return approvals.Summary{}, err
		}
		r, err := svc.Get(ctx, in.RequestID)
		if err != nil {
			return approvals.Summary{}, err
		}
		if r.Status != privacy.StatusRequested || r.Kind != privacy.KindDelete {
			return approvals.Summary{}, privacy.ErrNotOpen
		}
		who := r.UserPhone
		if r.UserName != nil && *r.UserName != "" {
			who = *r.UserName + " · " + r.UserPhone
		}
		return approvals.Summary{Title: "Delete the account of " + who,
			Changes: []approvals.Change{{Field: "account", Before: "active (" + strings.ToLower(r.App) + ")", After: "anonymised"}}}, nil
	}
}

func executePrivacy(svc *privacy.Service) approvals.Executor {
	return func(ctx context.Context, x approvals.Execution) (any, error) {
		in, err := decode[PrivacyPayload](x.Payload)
		if err != nil {
			return nil, err
		}
		return svc.Approve(ctx, x.RequesterID, in.RequestID, x.Reason)
	}
}

func regulatorAction(svc *compliance.Service) approvals.Action {
	return approvals.Action{Key: RegulatorSettings, Group: groupTrust, Permission: identity.PermComplianceManage, Scoped: true,
		Label:       "Change regulator switches",
		Description: "Turn a country's regulator reporting on or off, or make it fail closed.",
		Describe:    describeRegulator(svc), Execute: executeRegulator(svc)}
}

// switches is a country's regulator settings as a diff compares them.
type switches struct {
	Enabled    bool `json:"enabled"`
	FailClosed bool `json:"fail_closed"`
}

func describeRegulator(svc *compliance.Service) approvals.Describer {
	return func(ctx context.Context, raw json.RawMessage) (approvals.Summary, error) {
		in, err := decode[RegulatorPayload](raw)
		if err != nil {
			return approvals.Summary{}, err
		}
		list, err := svc.Statuses(ctx, in.Country)
		if err != nil {
			return approvals.Summary{}, err
		}
		if len(list) == 0 {
			return approvals.Summary{}, compliance.ErrNoRegulator
		}
		before := switches{Enabled: list[0].Enabled, FailClosed: list[0].FailClosed}
		after := before
		if in.Enabled != nil {
			after.Enabled = *in.Enabled
		}
		if in.FailClosed != nil {
			after.FailClosed = *in.FailClosed
		}
		return approvals.Summary{Title: fmt.Sprintf("Change the %s regulator switches in %s", list[0].Regulator, in.Country),
			Changes: approvals.Diff(before, after)}, nil
	}
}

func executeRegulator(svc *compliance.Service) approvals.Executor {
	return func(ctx context.Context, x approvals.Execution) (any, error) {
		in, err := decode[RegulatorPayload](x.Payload)
		if err != nil {
			return nil, err
		}
		return svc.SetSettings(ctx, x.RequesterID, in.Country, in.Enabled, in.FailClosed, x.Reason)
	}
}
