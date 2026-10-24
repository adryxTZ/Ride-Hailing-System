package approvals

import (
	"testing"

	"github.com/shopspring/decimal"
)

func dec(s string) *decimal.Decimal {
	d := decimal.RequireFromString(s)
	return &d
}

func TestConditionsMatch(t *testing.T) {
	c := Conditions{AmountMin: dec("10000"), Countries: []string{"TZ"}}
	for name, tc := range map[string]struct {
		country string
		amount  *decimal.Decimal
		want    bool
	}{
		"at the threshold":   {"TZ", dec("10000"), true},
		"above":              {"tz", dec("25000"), true},
		"below":              {"TZ", dec("9999.99"), false},
		"other country":      {"KE", dec("50000"), false},
		"no amount":          {"TZ", nil, false},
		"no country at all":  {"", dec("50000"), false},
		"empty matches all?": {"TZ", dec("10000"), true},
	} {
		if got := c.Matches(tc.country, tc.amount); got != tc.want {
			t.Errorf("%s: got %v", name, got)
		}
	}
	if !(Conditions{}).Matches("", nil) {
		t.Error("no conditions must match everything")
	}
	if (Conditions{AmountMax: dec("100")}).Matches("TZ", dec("101")) {
		t.Error("amount_max is inclusive and 101 is above it")
	}
}

func TestStepAnyCountsApprovals(t *testing.T) {
	s := Step{Mode: ModeAny, Roles: []string{"Finance", "Operations"}, Approvals: 2}
	fin := Grants{Roles: []string{"Finance"}}
	if as := s.approveAs(fin, nil); as != "Finance" {
		t.Fatalf("finance counts as %q", as)
	}
	if s.done([]string{"Finance"}) {
		t.Fatal("one of two approvals is not enough")
	}
	if !s.done([]string{"Finance", "Operations"}) {
		t.Fatal("two approvals meet the step")
	}
	if s.approveAs(Grants{Roles: []string{"Support"}}, nil) != "" || s.mayAct(Grants{Roles: []string{"Support"}}) {
		t.Fatal("a role not listed cannot act")
	}
}

func TestStepAllNeedsEveryRole(t *testing.T) {
	s := Step{Mode: ModeAll, Roles: []string{"Finance", "Operations"}, Approvals: 1}
	both := Grants{Roles: []string{"Finance", "Operations"}}
	if as := s.approveAs(both, []string{"Finance"}); as != "Operations" {
		t.Fatalf("the uncovered role is taken, got %q", as)
	}
	if s.approveAs(Grants{Roles: []string{"Finance"}}, []string{"Finance"}) != "" {
		t.Fatal("a covered role cannot be covered twice")
	}
	if s.done([]string{"Finance"}) || !s.done([]string{"Finance", "Operations"}) {
		t.Fatal("ALL is met only when every role approved")
	}
}

func TestStepPermissionAndSuperAdmin(t *testing.T) {
	s := Step{Mode: ModeAny, Permission: "trips.refund", Approvals: 1}
	if !s.mayAct(Grants{Permissions: []string{"*"}}) {
		t.Fatal("* holds every permission")
	}
	if s.approveAs(Grants{Permissions: []string{"trips.refund"}}, nil) != "perm:trips.refund" {
		t.Fatal("the permission member is perm:<key>")
	}
	if s.mayAct(Grants{Roles: []string{"Super Admin"}}) {
		t.Fatal("a role name alone does not give a permission")
	}
}

func TestValidateDefinition(t *testing.T) {
	refund := Action{Key: "x", Condition: ConditionAmount, Scoped: true}
	branding := Action{Key: "y"}
	r := Rules{Roles: []string{"Finance"}, Permissions: []string{"trips.refund"}, Countries: []string{"TZ"}}
	ok := Definition{ExpireHours: 24, Conditions: Conditions{AmountMin: dec("1")}, Steps: []Step{{Name: "Finance", Mode: ModeAny, Roles: []string{"Finance"}, Approvals: 1}}}
	if p := ok.validate(refund, true, r); len(p) != 0 {
		t.Fatalf("valid definition refused: %v", p)
	}
	if p := ok.validate(branding, true, r); len(p) == 0 {
		t.Fatal("an amount condition on an action without amounts must be refused")
	}
	empty := Definition{ExpireHours: 24, Steps: []Step{}}
	if p := empty.validate(refund, true, r); len(p) == 0 {
		t.Fatal("an enabled workflow without steps must be refused")
	}
	if p := empty.validate(refund, false, r); len(p) != 0 {
		t.Fatalf("a disabled workflow may have no steps: %v", p)
	}
	bad := Definition{ExpireHours: 0, Conditions: Conditions{Countries: []string{"ZZ"}},
		Steps: []Step{{Name: "", Mode: "SOME", Roles: []string{"Nobody", "Nobody"}, Approvals: 11}}}
	if p := bad.validate(refund, true, r); len(p) < 6 {
		t.Fatalf("expected every problem listed, got %v", p)
	}
}

func TestDiff(t *testing.T) {
	type terms struct {
		Fee   *decimal.Decimal `json:"fee"`
		Rate  string           `json:"rate"`
		Tags  []string         `json:"tags"`
		Keeps int              `json:"keeps"`
	}
	got := Diff(terms{Rate: "0.1", Tags: []string{"a"}, Keeps: 1}, terms{Fee: dec("500"), Rate: "0.12", Tags: []string{"a", "b"}, Keeps: 1})
	want := map[string][2]string{"fee": {emptyValue, "500"}, "rate": {"0.1", "0.12"}, "tags": {"a", "a, b"}}
	if len(got) != len(want) {
		t.Fatalf("changes: %+v", got)
	}
	for _, c := range got {
		if w, ok := want[c.Field]; !ok || w[0] != c.Before || w[1] != c.After {
			t.Errorf("unexpected change %+v", c)
		}
	}
}
