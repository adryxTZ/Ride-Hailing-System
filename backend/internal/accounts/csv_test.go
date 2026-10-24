package accounts

import (
	"bytes"
	"encoding/csv"
	"testing"
	"time"
)

func TestSafeCellEscapesFormulas(t *testing.T) {
	for in, want := range map[string]string{
		"=HYPERLINK(\"x\")": "'=HYPERLINK(\"x\")",
		"+1":                "'+1",
		"-2+3":              "'-2+3",
		"@SUM(A1)":          "'@SUM(A1)",
		"\t=1":              "'\t=1",
		"Cash trip":         "Cash trip",
		"":                  "",
	} {
		if got := safeCell(in); got != want {
			t.Errorf("%q: got %q", in, got)
		}
	}
}

func TestWriteCSVKeepsAmountsNumericAndTextSafe(t *testing.T) {
	code, holder := "=CMD", "@evil"
	list := []Entry{{At: time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC), Kind: "finance.settlement", Memo: "-paid",
		Amount: d("-1500.5"), Running: d("300"), TripCode: &code, Holder: &holder, Reference: "ref"}}
	var buf bytes.Buffer
	if err := WriteCSV(&buf, "TZS", list); err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(&buf).ReadAll()
	if err != nil || len(rows) != 2 {
		t.Fatalf("rows %v %v", rows, err)
	}
	r := rows[1]
	if r[1] != "'=CMD" || r[3] != "'-paid" || r[4] != "'@evil" {
		t.Errorf("text cells not escaped: %q", r)
	}
	if r[6] != "-1500.50" || r[7] != "300.00" {
		t.Errorf("amounts must stay numbers: %q", r)
	}
}
