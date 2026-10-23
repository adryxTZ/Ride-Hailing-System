package pdf

import (
	"reflect"
	"strings"
	"testing"
)

func TestWrapFitsWidth(t *testing.T) {
	text := "Asante kwa kusafiri nasi. Tafadhali hifadhi risiti hii kwa kumbukumbu zako " +
		"na wasiliana nasi kama una swali lolote kuhusu safari yako."
	const maxWidth = 150.0
	lines := Wrap(10, Regular, text, maxWidth)
	if len(lines) < 3 {
		t.Fatalf("expected several lines, got %q", lines)
	}
	for _, l := range lines {
		if w := TextWidth(10, Regular, l); w > maxWidth {
			t.Errorf("line %q is %.2fpt, wider than %.0f", l, w, maxWidth)
		}
	}
	if strings.Join(lines, " ") != strings.Join(strings.Fields(text), " ") {
		t.Fatal("wrapping lost or reordered words")
	}
}

func TestWrapNewlinesAndShortText(t *testing.T) {
	got := Wrap(10, Regular, "one two\n\nthree", 500)
	want := []string{"one two", "", "three"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Wrap = %q, want %q", got, want)
	}
}

func TestWrapSplitsLongWord(t *testing.T) {
	word := strings.Repeat("W", 20)
	lines := Wrap(10, Bold, word, 40)
	if len(lines) < 2 || strings.Join(lines, "") != word {
		t.Fatalf("long word split wrong: %q", lines)
	}
	for _, l := range lines {
		if TextWidth(10, Bold, l) > 40 {
			t.Fatalf("piece %q too wide", l)
		}
	}
}

func TestWrapTinyWidthProgresses(t *testing.T) {
	lines := Wrap(10, Regular, "ab", 1)
	if !reflect.DeepEqual(lines, []string{"a", "b"}) {
		t.Fatalf("Wrap = %q", lines)
	}
}
