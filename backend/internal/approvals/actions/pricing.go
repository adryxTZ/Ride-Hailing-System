package actions

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kalen-limited/hauler-rider/backend/internal/approvals"
	"github.com/kalen-limited/hauler-rider/backend/internal/identity"
	"github.com/kalen-limited/hauler-rider/backend/internal/profiles"
)

// registerPricing adds the four pricing-profile actions.
func registerPricing(c *approvals.Catalogue, p *profiles.Service) {
	c.Register(approvals.Action{Key: ProfilePublish, Group: groupPricing, Permission: identity.PermFeesEdit, Scoped: true,
		Label:       "Publish a pricing profile version",
		Description: "A new set of commission, fees and charges for a profile, from its effective date.",
		Describe:    describeProfileVersion(p), Execute: executeProfileVersion(p)})
	c.Register(approvals.Action{Key: ProfileStatus, Group: groupPricing, Permission: identity.PermFeesEdit, Scoped: true,
		Label:       "Change a pricing profile's status",
		Description: "Switch a profile active or inactive.",
		Describe:    describeProfileStatus(p), Execute: executeProfileStatus(p)})
	c.Register(approvals.Action{Key: ProfileDefault, Group: groupPricing, Permission: identity.PermFeesEdit, Scoped: true,
		Label:       "Make a pricing profile the default",
		Description: "The profile becomes the country's default terms.",
		Describe:    describeProfileDefault(p), Execute: executeProfileDefault(p)})
	c.Register(approvals.Action{Key: ProfileAssign, Group: groupPricing, Permission: identity.PermFeesEdit, Scoped: true,
		Label:       "Assign a pricing profile",
		Description: "Apply a profile to a region, district, zone or place.",
		Describe:    describeProfileAssign(p), Execute: executeProfileAssign(p)})
}

// versionContent is what a version changes (compared field by field).
type versionContent struct {
	Terms   profiles.Terms        `json:"terms"`
	Charges []profiles.ChargeRule `json:"charges"`
}

// currentContent is the profile's version in force now (empty when it has none).
func currentContent(p *profiles.Profile) versionContent {
	if p.CurrentVersion == nil {
		return versionContent{}
	}
	for _, v := range p.Versions {
		if v.ID == p.CurrentVersion.ID {
			return versionContent{Terms: v.Terms, Charges: v.Charges}
		}
	}
	return versionContent{}
}

func describeProfileVersion(svc *profiles.Service) approvals.Describer {
	return func(ctx context.Context, raw json.RawMessage) (approvals.Summary, error) {
		in, err := decode[ProfileVersionPayload](raw)
		if err != nil {
			return approvals.Summary{}, err
		}
		p, err := svc.Get(ctx, in.ProfileID)
		if err != nil {
			return approvals.Summary{}, err
		}
		changes := approvals.Diff(currentContent(p), versionContent{Terms: in.Version.Terms, Charges: in.Version.Charges})
		if in.Version.EffectiveFrom != nil {
			changes = append(changes, approvals.Change{Field: "effective_from", Before: "", After: in.Version.EffectiveFrom.Format("2006-01-02 15:04")})
		}
		return approvals.Summary{Title: fmt.Sprintf("Publish a new version of %s (%s)", p.Name, p.Country), Changes: changes}, nil
	}
}

func executeProfileVersion(svc *profiles.Service) approvals.Executor {
	return func(ctx context.Context, x approvals.Execution) (any, error) {
		in, err := decode[ProfileVersionPayload](x.Payload)
		if err != nil {
			return nil, err
		}
		in.Version.Reason = x.Reason
		return svc.Publish(ctx, x.RequesterID, in.ProfileID, in.Version)
	}
}

func describeProfileStatus(svc *profiles.Service) approvals.Describer {
	return func(ctx context.Context, raw json.RawMessage) (approvals.Summary, error) {
		in, err := decode[ProfileStatusPayload](raw)
		if err != nil {
			return approvals.Summary{}, err
		}
		p, err := svc.Get(ctx, in.ProfileID)
		if err != nil {
			return approvals.Summary{}, err
		}
		return approvals.Summary{Title: fmt.Sprintf("Set %s (%s) to %s", p.Name, p.Country, in.Status),
			Changes: []approvals.Change{{Field: "status", Before: p.Status, After: in.Status}}}, nil
	}
}

func executeProfileStatus(svc *profiles.Service) approvals.Executor {
	return func(ctx context.Context, x approvals.Execution) (any, error) {
		in, err := decode[ProfileStatusPayload](x.Payload)
		if err != nil {
			return nil, err
		}
		return svc.SetStatus(ctx, x.RequesterID, in.ProfileID, in.Status, x.Reason)
	}
}

func describeProfileDefault(svc *profiles.Service) approvals.Describer {
	return func(ctx context.Context, raw json.RawMessage) (approvals.Summary, error) {
		in, err := decode[ProfileDefaultPayload](raw)
		if err != nil {
			return approvals.Summary{}, err
		}
		p, err := svc.Get(ctx, in.ProfileID)
		if err != nil {
			return approvals.Summary{}, err
		}
		return approvals.Summary{Title: fmt.Sprintf("Make %s the default profile of %s", p.Name, p.Country),
			Changes: []approvals.Change{{Field: "is_default", Before: fmt.Sprint(p.IsDefault), After: "true"}}}, nil
	}
}

func executeProfileDefault(svc *profiles.Service) approvals.Executor {
	return func(ctx context.Context, x approvals.Execution) (any, error) {
		in, err := decode[ProfileDefaultPayload](x.Payload)
		if err != nil {
			return nil, err
		}
		return svc.MakeDefault(ctx, x.RequesterID, in.ProfileID, x.Reason)
	}
}

func describeProfileAssign(svc *profiles.Service) approvals.Describer {
	return func(ctx context.Context, raw json.RawMessage) (approvals.Summary, error) {
		in, err := decode[ProfileAssignPayload](raw)
		if err != nil {
			return approvals.Summary{}, err
		}
		p, err := svc.Get(ctx, in.ProfileID)
		if err != nil {
			return approvals.Summary{}, err
		}
		return approvals.Summary{Title: fmt.Sprintf("Assign %s (%s) to a %s", p.Name, p.Country, in.Assign.Kind),
			Changes: []approvals.Change{
				{Field: "scope", Before: "", After: in.Assign.Kind + " " + in.Assign.ScopeID},
				{Field: "priority", Before: "", After: fmt.Sprint(in.Assign.Priority)},
			}}, nil
	}
}

func executeProfileAssign(svc *profiles.Service) approvals.Executor {
	return func(ctx context.Context, x approvals.Execution) (any, error) {
		in, err := decode[ProfileAssignPayload](x.Payload)
		if err != nil {
			return nil, err
		}
		in.Assign.Reason = x.Reason
		return svc.Assign(ctx, x.RequesterID, in.ProfileID, in.Assign)
	}
}
