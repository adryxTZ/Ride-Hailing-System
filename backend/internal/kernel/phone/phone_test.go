package phone

import "testing"

func TestNormalizeTZ(t *testing.T) {
	cases := map[string]string{"0712345678": "+255712345678", "712345678": "+255712345678", "+255 712 345 678": "+255712345678", "255712345678": "+255712345678"}
	for in, want := range cases {
		got, err := Normalize(in, "TZ")
		if err != nil || got != want {
			t.Errorf("%q → %q (%v), want %q", in, got, err, want)
		}
	}
	if _, err := Normalize("12", "TZ"); err == nil {
		t.Error("expected error for short number")
	}
}
