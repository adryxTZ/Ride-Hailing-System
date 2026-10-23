package theme

import "testing"

func TestLoadPresetsAndDerive(t *testing.T) {
	p, err := LoadPresets("../../../seed")
	if err != nil {
		t.Fatalf("load presets: %v", err)
	}
	if len(p.List()) < 19 {
		t.Fatalf("expected at least 19 presets, got %d", len(p.List()))
	}
	for _, pr := range p.List() {
		th, ok := p.Resolve(pr.Key)
		if !ok {
			t.Fatalf("resolve %s", pr.Key)
		}
		pal := th.Palette
		if c := Contrast(pal.Text, pal.Background); c < 4.5 {
			t.Errorf("%s: body text contrast %.2f < 4.5", pr.Key, c)
		}
		if c := Contrast(pal.OnPrimary, pal.Primary); c < 4.5 {
			t.Errorf("%s: on-primary contrast %.2f < 4.5", pr.Key, c)
		}
		if c := Contrast(pal.OnAccent, pal.Accent); c < 3.0 {
			t.Errorf("%s: on-accent contrast %.2f < 3.0", pr.Key, c)
		}
	}
}

func TestContrastBlackWhite(t *testing.T) {
	if c := Contrast("#000000", "#FFFFFF"); c < 20.9 || c > 21.1 {
		t.Fatalf("expected 21, got %.2f", c)
	}
}
