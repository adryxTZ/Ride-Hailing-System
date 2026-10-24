package approvals

import (
	"slices"
	"strconv"
	"strings"

	"github.com/shopspring/decimal"
)

// Expiry bounds for a request, in hours.
const (
	DefaultExpireHours = 72
	MinExpireHours     = 1
	MaxExpireHours     = 720 // 30 days
)

// Step limits.
const (
	maxSteps          = 10
	maxStepApprovals  = 10
	maxStepNameLength = 80
)

// Matches reports whether a workflow's conditions cover a submission.
// An amount condition never matches a submission without an amount, and a
// country condition never matches one without a country.
func (c Conditions) Matches(country string, amount *decimal.Decimal) bool {
	if c.AmountMin != nil && (amount == nil || amount.LessThan(*c.AmountMin)) {
		return false
	}
	if c.AmountMax != nil && (amount == nil || amount.GreaterThan(*c.AmountMax)) {
		return false
	}
	if len(c.Countries) > 0 && !slices.Contains(c.Countries, strings.ToUpper(country)) {
		return false
	}
	return true
}

// normalise trims names, upper-cases modes and countries, and fills defaults.
func (d *Definition) normalise() {
	if d.ExpireHours == 0 {
		d.ExpireHours = DefaultExpireHours
	}
	for i := range d.Conditions.Countries {
		d.Conditions.Countries[i] = strings.ToUpper(strings.TrimSpace(d.Conditions.Countries[i]))
	}
	for i := range d.Steps {
		s := &d.Steps[i]
		s.Name = strings.TrimSpace(s.Name)
		s.Mode = strings.ToUpper(strings.TrimSpace(s.Mode))
		s.Permission = strings.TrimSpace(s.Permission)
		if s.Mode == "" {
			s.Mode = ModeAny
		}
		if s.Approvals == 0 {
			s.Approvals = 1
		}
		for j := range s.Roles {
			s.Roles[j] = strings.TrimSpace(s.Roles[j])
		}
	}
}

// Rules the editor's choices are checked against.
type Rules struct {
	Roles       []string // role names that exist
	Permissions []string // permission keys that exist
	Countries   []string // the brand's country codes
}

// validate lists every problem with a definition for an action ("" = none).
// An enabled workflow needs at least one step.
func (d Definition) validate(a Action, enabled bool, r Rules) []string {
	var out []string
	if d.ExpireHours < MinExpireHours || d.ExpireHours > MaxExpireHours {
		out = append(out, "expire_hours must be between 1 and 720")
	}
	out = append(out, d.Conditions.validate(a, r)...)
	if enabled && len(d.Steps) == 0 {
		out = append(out, "an enabled workflow needs at least one step")
	}
	if len(d.Steps) > maxSteps {
		out = append(out, "at most 10 steps")
	}
	for i, s := range d.Steps {
		out = append(out, s.validate(i, r)...)
	}
	return out
}

func (c Conditions) validate(a Action, r Rules) []string {
	var out []string
	if (c.AmountMin != nil || c.AmountMax != nil) && a.Condition != ConditionAmount {
		out = append(out, "this action has no amount to compare")
	}
	if c.AmountMin != nil && c.AmountMax != nil && c.AmountMin.GreaterThan(*c.AmountMax) {
		out = append(out, "amount_min is above amount_max")
	}
	if (c.AmountMin != nil && c.AmountMin.IsNegative()) || (c.AmountMax != nil && c.AmountMax.IsNegative()) {
		out = append(out, "amounts cannot be negative")
	}
	if len(c.Countries) > 0 && !a.Scoped {
		out = append(out, "this action is not tied to a country")
	}
	for _, code := range c.Countries {
		if !slices.Contains(r.Countries, code) {
			out = append(out, "unknown country "+code)
		}
	}
	return out
}

func (s Step) validate(i int, r Rules) []string {
	var out []string
	at := "step " + strconv.Itoa(i+1) + ": "
	if s.Name == "" || len(s.Name) > maxStepNameLength {
		out = append(out, at+"needs a name (up to 80 characters)")
	}
	if s.Mode != ModeAny && s.Mode != ModeAll {
		out = append(out, at+"mode must be ANY or ALL")
	}
	if len(s.Roles) == 0 && s.Permission == "" {
		out = append(out, at+"choose at least one role or a permission")
	}
	if s.Approvals < 1 || s.Approvals > maxStepApprovals {
		out = append(out, at+"approvals must be between 1 and 10")
	}
	for _, role := range s.Roles {
		if !slices.Contains(r.Roles, role) {
			out = append(out, at+"unknown role "+role)
		}
	}
	if s.Permission != "" && !slices.Contains(r.Permissions, s.Permission) {
		out = append(out, at+"unknown permission "+s.Permission)
	}
	if hasDuplicates(s.Roles) {
		out = append(out, at+"a role is listed twice")
	}
	return out
}

func hasDuplicates(xs []string) bool {
	seen := map[string]bool{}
	for _, x := range xs {
		if seen[x] {
			return true
		}
		seen[x] = true
	}
	return false
}
