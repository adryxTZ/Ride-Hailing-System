package pdf

import (
	"os"
	"testing"
	"time"
)

// sampleEnv names the env var that, when set to a file path, makes
// TestRenderSample write a receipt-like PDF there for visual inspection.
const sampleEnv = "PDF_SAMPLE_OUT"

// Sample layout constants (points).
const (
	sampleMargin   = 48.0
	sampleTitle    = 20.0
	sampleBody     = 10.5
	sampleSmall    = 8.5
	sampleRowStep  = 18.0
	sampleBandH    = 64.0
	sampleRuleW    = 0.6
	sampleNoteWrap = 360.0
)

// writeFile writes data to path or fails the test.
func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestRenderSample renders a bilingual receipt; it always builds the
// document (exercising the API) and only writes it when sampleEnv is set.
func TestRenderSample(t *testing.T) {
	doc := New(A4)
	doc.SetInfo(Info{Title: "Receipt 2026-000123", Creator: "pdf sample",
		CreatedAt: time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)})
	drawSampleReceipt(doc.AddPage())
	data, err := doc.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	checkStructure(t, data)
	if out := os.Getenv(sampleEnv); out != "" {
		writeFile(t, out, data)
	}
}

// drawSampleReceipt lays out a header band, line items, total and a note.
func drawSampleReceipt(p *Page) {
	right := p.Width() - sampleMargin
	p.Rect(0, 0, p.Width(), sampleBandH, RGB(0.31, 0.01, 0.75))
	p.SetColor(White)
	p.Text(sampleMargin, 40, sampleTitle, Bold, "Receipt / Risiti")
	p.TextRight(right, 40, sampleBody, Regular, "No. 2026-000123")
	p.SetColor(Black)
	y := 100.0
	rows := [][2]string{
		{"Date / Tarehe", "23 Oct 2026, 15:00"},
		{"From / Kutoka", "Kariakoo Market (Soko)"},
		{"To / Kwenda", "Julius Nyerere Int'l Airport – Terminal 3"},
		{"Ride type / Aina", "Boda"},
		{"Base fare / Nauli ya msingi", "TZS 2,000"},
		{"Distance 12.4 km × 650", "TZS 8,060"},
		{"Waiting / Kusubiri", "TZS 2,440"},
	}
	for _, r := range rows {
		p.Text(sampleMargin, y, sampleBody, Regular, r[0])
		p.TextRight(right, y, sampleBody, Regular, r[1])
		y += sampleRowStep
	}
	p.SetColor(Gray(0.6))
	p.Line(sampleMargin, y-6, right, y-6, sampleRuleW)
	p.SetColor(Black)
	y += sampleRowStep / 2
	p.Text(sampleMargin, y, sampleBody+2, Bold, "Total / Jumla")
	p.TextRight(right, y, sampleBody+2, Bold, "TZS 12,500")
	drawSampleNote(p, y+2*sampleRowStep)
}

// drawSampleNote writes a wrapped Swahili/English footer with odd glyphs.
func drawSampleNote(p *Page, y float64) {
	note := "Asante kwa kusafiri nasi! Thank you for riding with us. Prices include " +
		"VAT (18%) where applicable; refunds \\ disputes: within 7 days. Ünïcödé test: " +
		"é € — ő 日本."
	p.SetColor(Gray(0.35))
	for _, line := range Wrap(sampleSmall, Regular, note, sampleNoteWrap) {
		p.Text(sampleMargin, y, sampleSmall, Regular, line)
		y += sampleSmall * 1.4
	}
	p.TextCenter(p.Width()/2, p.Height()-sampleMargin, sampleSmall, Regular, "Page 1 of 1")
}
