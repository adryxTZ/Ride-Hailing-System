package theme

import "testing"

func TestFromColorHunt(t *testing.T) {
	got, err := FromColorHunt("https://colorhunt.co/palette/0b244719376d576cbca5d7e8")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"#0B2447", "#19376D", "#576CBC", "#A5D7E8"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("colour %d = %s, want %s", i, got[i], want[i])
		}
	}
	for _, bad := range []string{"https://example.com/palette/0b244719376d576cbca5d7e8", "colorhunt.co/palette/0b2447", "nope"} {
		if _, err := FromColorHunt(bad); err == nil {
			t.Fatalf("%q accepted", bad)
		}
	}
}

func TestAssignRolesLightestIsBackground(t *testing.T) {
	r := AssignRoles([]string{"#112D4E", "#3F72AF", "#DBE2EF", "#F9F7F7"})
	if r["background"] != "#F9F7F7" || r["surface"] != "#DBE2EF" {
		t.Fatalf("background/surface = %s/%s", r["background"], r["surface"])
	}
	if r["accent"] != "#3F72AF" || r["primary"] != "#112D4E" {
		t.Fatalf("accent/primary = %s/%s", r["accent"], r["primary"])
	}
}

func TestAssignRolesDarkPaletteGetsWhitePage(t *testing.T) {
	r := AssignRoles([]string{"#070F2B", "#1B1A55", "#535C91", "#9290C3"})
	if r["background"] != "#FFFFFF" || r["surface"] != "#9290C3" {
		t.Fatalf("background/surface = %s/%s", r["background"], r["surface"])
	}
}

func TestEvaluateBlocksUnreadableAndLookalikes(t *testing.T) {
	good := Derive(map[string]string{"primary": "#0B2447", "accent": "#576CBC", "background": "#FFFFFF", "surface": "#F4F6FA"})
	if r := Evaluate(good, nil); !r.Pass {
		t.Fatalf("good palette failed: %+v", r.Checks)
	}
	same := Derive(map[string]string{"primary": "#0B2447", "accent": "#0C2548"})
	if Evaluate(same, nil).Pass {
		t.Fatal("primary and accent that look alike must fail")
	}
	grey := Derive(map[string]string{"background": "#FFFFFF", "text": "#BBBBBB"})
	if Evaluate(grey, nil).Pass {
		t.Fatal("light grey text on white must fail")
	}
}

func TestDeriveDarkIsDarkAndReadable(t *testing.T) {
	light := Derive(map[string]string{"primary": "#5003C0", "accent": "#00B384", "background": "#FFFFFF"})
	d := DeriveDark(light, nil)
	if relLum(d.Background) > 0.05 {
		t.Fatalf("dark background too light: %s", d.Background)
	}
	if Contrast(d.Text, d.Background) < MinTextContrast {
		t.Fatalf("dark text unreadable: %s on %s", d.Text, d.Background)
	}
}
