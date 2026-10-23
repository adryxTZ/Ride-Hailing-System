package pdf

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// xrefEntryLen is the fixed length of one cross-reference entry.
const xrefEntryLen = 20

// render builds a small two-element document and returns its bytes.
func render(t *testing.T, pages int, info Info) []byte {
	t.Helper()
	doc := New(A4)
	doc.SetInfo(info)
	for i := 0; i < pages; i++ {
		p := doc.AddPage()
		p.Text(40, 60, 18, Bold, fmt.Sprintf("Page %d (one) \\ two", i+1))
		p.TextRight(555, 60, 12, Regular, "TZS 12,500")
		p.Line(40, 80, 555, 80, 0.5)
		p.Rect(40, 100, 200, 20, Gray(0.9))
	}
	data, err := doc.Bytes()
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	return data
}

// checkStructure verifies header, EOF marker, xref layout and that every
// xref offset points at the matching "N 0 obj" line.
func checkStructure(t *testing.T, data []byte) int {
	t.Helper()
	if !bytes.HasPrefix(data, []byte("%PDF-1.4\n%")) {
		t.Fatalf("missing header: %q", data[:12])
	}
	if !bytes.HasSuffix(data, []byte("%%EOF\n")) {
		t.Fatalf("missing %%%%EOF")
	}
	start := startXref(t, data)
	if !bytes.HasPrefix(data[start:], []byte("xref\n")) {
		t.Fatalf("startxref %d does not point at xref", start)
	}
	var first, count int
	if _, err := fmt.Sscanf(string(data[start:]), "xref\n%d %d\n", &first, &count); err != nil {
		t.Fatalf("xref subsection header: %v", err)
	}
	entries := data[bytes.IndexByte(data[start+len("xref\n"):], '\n')+start+len("xref\n")+1:]
	for n := 0; n < count; n++ {
		checkEntry(t, data, entries[n*xrefEntryLen:(n+1)*xrefEntryLen], n)
	}
	if !bytes.Contains(data, []byte(fmt.Sprintf("/Size %d ", count))) {
		t.Fatalf("trailer /Size does not equal %d", count)
	}
	return count
}

// startXref parses the offset after the startxref keyword.
func startXref(t *testing.T, data []byte) int {
	t.Helper()
	m := regexp.MustCompile(`startxref\n(\d+)\n%%EOF\n$`).FindSubmatch(data)
	if m == nil {
		t.Fatal("no startxref")
	}
	off, _ := strconv.Atoi(string(m[1]))
	return off
}

// checkEntry validates one 20-byte xref entry for object n.
func checkEntry(t *testing.T, data, entry []byte, n int) {
	t.Helper()
	if len(entry) != xrefEntryLen || !bytes.HasSuffix(entry, []byte(" \n")) {
		t.Fatalf("entry %d malformed: %q", n, entry)
	}
	if n == 0 {
		if string(entry) != freeEntry {
			t.Fatalf("entry 0 not free: %q", entry)
		}
		return
	}
	off, err := strconv.Atoi(string(entry[:10]))
	if err != nil {
		t.Fatalf("entry %d offset: %v", n, err)
	}
	want := fmt.Sprintf("%d 0 obj\n", n)
	if !bytes.HasPrefix(data[off:], []byte(want)) {
		t.Fatalf("entry %d -> %q, want %q", n, data[off:off+len(want)], want)
	}
}

func TestStructureSinglePage(t *testing.T) {
	data := render(t, 1, Info{})
	// catalog, pages, 2 fonts, page, content (+ free entry 0) = 7
	if got := checkStructure(t, data); got != 7 {
		t.Fatalf("xref size = %d, want 7", got)
	}
	if bytes.Contains(data, []byte("/Info")) {
		t.Fatal("empty Info must be omitted")
	}
}

func TestMultiPage(t *testing.T) {
	data := render(t, 3, Info{Title: "Risiti"})
	if got := checkStructure(t, data); got != 12 {
		t.Fatalf("xref size = %d, want 12", got)
	}
	if !bytes.Contains(data, []byte("/Kids [5 0 R 7 0 R 9 0 R] /Count 3")) {
		t.Fatal("page tree does not list three pages in order")
	}
	if !bytes.Contains(data, []byte("/Info 11 0 R")) {
		t.Fatal("trailer missing /Info")
	}
}

func TestStreamLengthMatches(t *testing.T) {
	data := render(t, 2, Info{})
	re := regexp.MustCompile(`<< /Length (\d+) >>\nstream\n`)
	for _, loc := range re.FindAllSubmatchIndex(data, -1) {
		n, _ := strconv.Atoi(string(data[loc[2]:loc[3]]))
		if !bytes.HasPrefix(data[loc[1]+n:], []byte("\nendstream")) {
			t.Fatalf("stream /Length %d does not end at endstream", n)
		}
	}
}

func TestDeterministic(t *testing.T) {
	info := Info{Title: "Receipt", Author: "Ops", Subject: "Trip", Creator: "backend",
		CreatedAt: time.Date(2026, 10, 23, 9, 5, 7, 0, time.FixedZone("EAT", 3*3600))}
	a, b := render(t, 2, info), render(t, 2, info)
	if !bytes.Equal(a, b) {
		t.Fatal("two renders differ")
	}
	if !bytes.Contains(a, []byte("/CreationDate (D:20261023060507Z)")) {
		t.Fatal("CreationDate not formatted in UTC")
	}
	if !bytes.Contains(a, []byte("/Title <FEFF0052006500630065006900700074>")) {
		t.Fatal("Title not encoded as UTF-16BE")
	}
}

func TestWriteToMatchesBytes(t *testing.T) {
	doc := New(A4)
	doc.AddPage().Text(10, 10, 10, Regular, "x")
	var buf bytes.Buffer
	n, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := doc.Bytes()
	if n != int64(len(data)) || !bytes.Equal(buf.Bytes(), data) {
		t.Fatal("WriteTo and Bytes disagree")
	}
}

func TestNoPages(t *testing.T) {
	if _, err := New(A4).Bytes(); !errors.Is(err, ErrNoPages) {
		t.Fatalf("err = %v, want ErrNoPages", err)
	}
}

// TestQPDFCheck runs qpdf --check when qpdf is installed.
func TestQPDFCheck(t *testing.T) {
	qpdf, err := exec.LookPath("qpdf")
	if err != nil {
		t.Skip("qpdf not installed")
	}
	path := filepath.Join(t.TempDir(), "check.pdf")
	writeFile(t, path, render(t, 2, Info{Title: "Check"}))
	out, err := exec.Command(qpdf, "--check", path).CombinedOutput()
	if err != nil || strings.Contains(string(out), "WARNING") {
		t.Fatalf("qpdf --check: %v\n%s", err, out)
	}
}
