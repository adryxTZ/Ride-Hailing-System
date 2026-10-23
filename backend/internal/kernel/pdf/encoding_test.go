package pdf

import (
	"bytes"
	"testing"
)

func TestToWinAnsi(t *testing.T) {
	cases := []struct {
		in   string
		want []byte
	}{
		{"Hello", []byte("Hello")},
		{"é", []byte{0xE9}},
		{"€", []byte{0x80}},
		{"–", []byte{0x96}},
		{"—", []byte{0x97}},
		{"“ok”", []byte{0x93, 'o', 'k', 0x94}},
		{"ő", []byte{'?'}},
		{"日本", []byte{'?', '?'}},
		{"a\tb", []byte{'a', '?', 'b'}},
		{"\xff", []byte{'?'}},
		{"Malipo: TZS 1 500 · Asante", []byte("Malipo: TZS 1 500 \xb7 Asante")},
	}
	for _, c := range cases {
		if got := toWinAnsi(c.in); !bytes.Equal(got, c.want) {
			t.Errorf("toWinAnsi(%q) = % x, want % x", c.in, got, c.want)
		}
	}
}

func TestEscapeLiteral(t *testing.T) {
	got := string(escapeLiteral([]byte(`a(b)c\d`)))
	if want := `a\(b\)c\\d`; got != want {
		t.Fatalf("escapeLiteral = %q, want %q", got, want)
	}
}

func TestTextIsEscapedInContent(t *testing.T) {
	doc := New(A4)
	doc.AddPage().Text(10, 20, 10, Regular, `Fare (est.) \ é`)
	data, err := doc.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	want := []byte("(Fare \\(est.\\) \\\\ \xe9) Tj")
	if !bytes.Contains(data, want) {
		t.Fatalf("content missing escaped text %q", want)
	}
}
