package actions

import (
	"testing"

	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/approvals"
)

func TestSameRoles(t *testing.T) {
	if !SameRoles([]string{"Finance", " Operations "}, []string{"Operations", "Finance"}) {
		t.Error("order and spacing must not matter")
	}
	if SameRoles([]string{"Finance"}, []string{"Finance", "Support"}) {
		t.Error("an added role is a change")
	}
	if !SameRoles(nil, []string{" "}) {
		t.Error("no roles equals blank roles")
	}
}

func TestWhole(t *testing.T) {
	for in, want := range map[string]string{"1000": "1,000", "999.6": "1,000", "15000.25": "15,000", "-1234567": "-1,234,567", "12": "12"} {
		if got := whole(decimal.RequireFromString(in)); got != want {
			t.Errorf("%s: got %s", in, got)
		}
	}
}

// TestRegisterWithoutModules registers nothing when no module is wired.
func TestRegisterWithoutModules(t *testing.T) {
	c := approvals.NewCatalogue()
	Register(c, Modules{})
	if len(c.Keys()) != 0 {
		t.Fatalf("got %v", c.Keys())
	}
}
