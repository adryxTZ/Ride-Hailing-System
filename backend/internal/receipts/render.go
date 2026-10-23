package receipts

import (
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/pdf"
)

// Layout of the A4 receipt, in points.
const (
	margin    = 48.0
	bandH     = 72.0
	titleSize = 22.0
	bodySize  = 10.5
	smallSize = 8.5
	totalSize = 13.0
	rowStep   = 18.0
	gap       = 12.0
	ruleW     = 0.6
	labelW    = 110.0 // label column of the trip details
	noteWidth = 400.0
)

// Ink levels (no brand colours: the receipt prints well in black and white).
var (
	bandInk  = pdf.Gray(0.12)
	mutedInk = pdf.Gray(0.42)
	ruleInk  = pdf.Gray(0.75)
)

// Render draws the receipt as an A4 PDF in a language. Same receipt and
// language, same bytes (the creation date is the issue date).
func Render(r *Receipt, lang string) ([]byte, error) {
	t := text[langOf(lang)]
	doc := pdf.New(pdf.A4)
	doc.SetInfo(pdf.Info{Title: t["title"] + " " + r.Number, Author: r.Issuer.Name, Creator: r.Issuer.Name, CreatedAt: r.IssuedAt})
	p := doc.AddPage()
	header(p, r, t)
	y := details(p, r, t, bandH+2*gap+rowStep)
	y = lines(p, r, langOf(lang), y+gap)
	y = totals(p, r, t, y)
	footer(p, r, t, y+2*gap)
	return doc.Bytes()
}

func header(p *pdf.Page, r *Receipt, t map[string]string) {
	right := p.Width() - margin
	p.Rect(0, 0, p.Width(), bandH, bandInk)
	p.SetColor(pdf.White)
	p.Text(margin, 44, titleSize, pdf.Bold, r.Issuer.Name)
	p.TextRight(right, 36, bodySize, pdf.Bold, t["title"])
	p.TextRight(right, 52, bodySize, pdf.Regular, t["number"]+" "+r.Number)
	p.SetColor(pdf.Black)
}

// details prints the trip facts as label/value rows and returns the next y.
func details(p *pdf.Page, r *Receipt, t map[string]string, y float64) float64 {
	rows := [][2]string{
		{t["issued"], localTime(r.IssuedAt, r.Issuer.Timezone)},
		{t["trip"], r.Trip.Code},
		{t["from"], r.Trip.Pickup},
		{t["to"], r.Trip.Dropoff},
		{t["driver"], r.Driver.Name},
	}
	if r.Driver.Plate != "" {
		rows = append(rows, [2]string{t["vehicle"], r.Driver.Plate})
	}
	if r.Trip.DistanceM != nil {
		rows = append(rows, [2]string{t["distance"], fmt.Sprintf("%.1f km", float64(*r.Trip.DistanceM)/1000)})
	}
	for _, row := range rows {
		p.SetColor(mutedInk)
		p.Text(margin, y, bodySize, pdf.Regular, row[0])
		p.SetColor(pdf.Black)
		for i, part := range pdf.Wrap(bodySize, pdf.Regular, row[1], p.Width()-2*margin-labelW) {
			p.Text(margin+labelW, y+float64(i)*rowStep, bodySize, pdf.Regular, part)
			if i > 0 {
				y += rowStep
			}
		}
		y += rowStep
	}
	return y
}

// lines prints the fare lines and returns the next y.
func lines(p *pdf.Page, r *Receipt, lang string, y float64) float64 {
	right := p.Width() - margin
	rule(p, y-rowStep+4)
	for _, l := range r.Lines {
		p.Text(margin, y, bodySize, pdf.Regular, l.Label[lang])
		p.TextRight(right, y, bodySize, pdf.Regular, money(r.Currency, l.Amount))
		y += rowStep
	}
	rule(p, y-rowStep+6)
	return y + gap/2
}

// totals prints the total, the included taxes and how it was paid.
func totals(p *pdf.Page, r *Receipt, t map[string]string, y float64) float64 {
	right := p.Width() - margin
	p.Text(margin, y, totalSize, pdf.Bold, t["total"])
	p.TextRight(right, y, totalSize, pdf.Bold, money(r.Currency, r.Total))
	y += rowStep + 4
	p.SetColor(mutedInk)
	for _, tax := range []struct {
		label  string
		amount decimal.Decimal
	}{{t["vat"], r.Taxes.VAT}, {t["levy"], r.Taxes.Levy}} {
		if tax.amount.IsPositive() {
			p.Text(margin, y, smallSize, pdf.Regular, tax.label)
			p.TextRight(right, y, smallSize, pdf.Regular, r.Currency+" "+tax.amount.StringFixed(2))
			y += rowStep - 4
		}
	}
	paid := t["paid_cash"]
	if r.Payment.Method == "MOBILE_MONEY" && r.Payment.Status == "COLLECTED" {
		paid = t["paid_mobile"]
	}
	p.SetColor(pdf.Black)
	p.Text(margin, y+gap/2, bodySize, pdf.Bold, paid)
	return y + gap/2 + rowStep
}

func footer(p *pdf.Page, r *Receipt, t map[string]string, y float64) {
	p.SetColor(mutedInk)
	for _, line := range pdf.Wrap(smallSize, pdf.Regular, t["footer"], noteWidth) {
		p.Text(margin, y, smallSize, pdf.Regular, line)
		y += rowStep - 6
	}
	contact := strings.TrimSpace(strings.Join(nonEmpty(r.Issuer.SupportPhone, r.Issuer.SupportEmail), " · "))
	if contact != "" {
		p.Text(margin, y+4, smallSize, pdf.Regular, t["support"]+": "+contact)
	}
	if r.Issuer.LegalEntity != "" {
		p.TextCenter(p.Width()/2, p.Height()-margin/2, smallSize, pdf.Regular, r.Issuer.LegalEntity)
	}
	p.SetColor(pdf.Black)
}

// localTime is the issue time in the issuer's timezone, e.g. "23 Oct 2026, 10:30 EAT".
func localTime(at time.Time, tz string) string {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	return at.In(loc).Format("02 Jan 2006, 15:04 MST")
}

func rule(p *pdf.Page, y float64) {
	p.SetColor(ruleInk)
	p.Line(margin, y, p.Width()-margin, y, ruleW)
	p.SetColor(pdf.Black)
}

// money formats an amount with thousands separators and no decimals ("TZS 12,500");
// receipts show whole currency units like the apps do.
func money(currency string, v decimal.Decimal) string {
	n := v.Round(0).IntPart()
	sign := ""
	if n < 0 {
		sign, n = "-", -n
	}
	s := fmt.Sprintf("%d", n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return currency + " " + sign + s
}

func nonEmpty(v ...string) []string {
	out := v[:0:0]
	for _, s := range v {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
