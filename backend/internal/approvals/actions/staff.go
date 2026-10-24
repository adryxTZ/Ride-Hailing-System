package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/kalen-limited/hauler-rider/backend/internal/approvals"
	"github.com/kalen-limited/hauler-rider/backend/internal/branding"
	"github.com/kalen-limited/hauler-rider/backend/internal/identity"
)

func staffRolesAction(svc *identity.Service) approvals.Action {
	return approvals.Action{Key: StaffRoles, Group: groupStaff, Permission: identity.PermStaffManage,
		Label:       "Change a staff member's roles",
		Description: "Grant or remove dashboard roles (removing every role removes access).",
		Describe:    describeStaffRoles(svc), Execute: executeStaffRoles(svc)}
}

// SameRoles reports whether two role lists hold the same names (order and
// spacing ignored): a role edit that keeps them is not a role change.
func SameRoles(a, b []string) bool {
	clean := func(xs []string) []string {
		out := []string{}
		for _, x := range xs {
			if x = strings.TrimSpace(x); x != "" && !slices.Contains(out, x) {
				out = append(out, x)
			}
		}
		slices.Sort(out)
		return out
	}
	return slices.Equal(clean(a), clean(b))
}

func describeStaffRoles(svc *identity.Service) approvals.Describer {
	return func(ctx context.Context, raw json.RawMessage) (approvals.Summary, error) {
		in, err := decode[StaffRolesPayload](raw)
		if err != nil {
			return approvals.Summary{}, err
		}
		cur, err := svc.StaffRolesByPhone(ctx, in.Phone)
		if err != nil {
			return approvals.Summary{}, err
		}
		who := strings.TrimSpace(in.FullName)
		if who == "" {
			who = cur.Name
		}
		if who == "" {
			who = in.Phone
		}
		after := slices.Clone(in.Roles)
		slices.Sort(after)
		return approvals.Summary{Title: fmt.Sprintf("Change the roles of %s", who),
			Changes: []approvals.Change{{Field: "roles", Before: strings.Join(cur.Roles, ", "), After: strings.Join(after, ", ")}}}, nil
	}
}

func executeStaffRoles(svc *identity.Service) approvals.Executor {
	return func(ctx context.Context, x approvals.Execution) (any, error) {
		in, err := decode[StaffRolesPayload](x.Payload)
		if err != nil {
			return nil, err
		}
		u, err := svc.SetStaffRoles(ctx, x.RequesterID, in.Phone, in.FullName, in.Roles)
		if err != nil {
			return nil, err
		}
		return map[string]any{"user_id": u.ID, "roles": in.Roles}, nil
	}
}

func brandingAction(svc *branding.Service) approvals.Action {
	return approvals.Action{Key: BrandingPublish, Group: groupSettings, Permission: identity.PermBrandingManage,
		Label:       "Publish branding",
		Description: "Make a theme draft the live look of every app, receipt and email.",
		Describe:    describeBranding(svc), Execute: executeBranding(svc)}
}

// themeLook is what a theme changes (compared field by field).
type themeLook struct {
	Palette    any               `json:"palette"`
	Dark       any               `json:"dark"`
	Typography map[string]string `json:"typography"`
	Shape      map[string]int    `json:"shape"`
	Assets     map[string]string `json:"assets"`
}

func lookOf(t *branding.ThemeVersion) themeLook {
	if t == nil {
		return themeLook{}
	}
	return themeLook{Palette: t.Palette, Dark: t.Dark, Typography: t.Typography, Shape: t.Shape, Assets: t.Assets}
}

func describeBranding(svc *branding.Service) approvals.Describer {
	return func(ctx context.Context, raw json.RawMessage) (approvals.Summary, error) {
		in, err := decode[BrandingPayload](raw)
		if err != nil {
			return approvals.Summary{}, err
		}
		draft, err := svc.Get(ctx, in.ThemeID)
		if err != nil {
			return approvals.Summary{}, err
		}
		if draft.Status != branding.StatusDraft {
			return approvals.Summary{}, branding.ErrNotDraft
		}
		versions, err := svc.Versions(ctx)
		if err != nil {
			return approvals.Summary{}, err
		}
		var live *branding.ThemeVersion
		for i := range versions {
			if versions[i].Status == branding.StatusPublished {
				live = &versions[i]
			}
		}
		return approvals.Summary{Title: fmt.Sprintf("Publish the theme %q (version %d)", draft.Name, draft.Version),
			Changes: approvals.Diff(lookOf(live), lookOf(draft))}, nil
	}
}

func executeBranding(svc *branding.Service) approvals.Executor {
	return func(ctx context.Context, x approvals.Execution) (any, error) {
		in, err := decode[BrandingPayload](x.Payload)
		if err != nil {
			return nil, err
		}
		return svc.Publish(ctx, x.RequesterID, in.ThemeID, x.Reason)
	}
}
