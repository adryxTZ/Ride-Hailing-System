package receipts

import "strings"

// Languages a receipt is written in.
const (
	LangEN = "en"
	LangSW = "sw"
)

// lineLabels names each fare line code in English and Swahili.
var lineLabels = map[string]map[string]string{
	"BASE_FARE":               {LangEN: "Trip fare", LangSW: "Nauli ya safari"},
	"SURGE":                   {LangEN: "Busy-time price", LangSW: "Bei ya wakati wa watu wengi"},
	"WAITING":                 {LangEN: "Waiting time", LangSW: "Muda wa kusubiri"},
	"PICKUP_APPROACH":         {LangEN: "Pickup distance", LangSW: "Umbali hadi kuchukuliwa"},
	"MINIMUM_FARE_ADJUSTMENT": {LangEN: "Minimum fare", LangSW: "Nauli ya chini kabisa"},
	"BOOKING_FEE":             {LangEN: "Booking fee", LangSW: "Ada ya kuagiza"},
	"FIXED_ROUTE":             {LangEN: "Fixed fare", LangSW: "Nauli maalum"},
	"ROUNDING":                {LangEN: "Rounding", LangSW: "Kuzungusha"},
	"PREVIOUS_CANCELLATION":   {LangEN: "Earlier cancellation fee", LangSW: "Ada ya kughairi awali"},
}

// labelFor names a line; unknown codes are shown as words ("NIGHT_FEE" → "Night fee").
func labelFor(code string) map[string]string {
	if l, ok := lineLabels[code]; ok {
		return l
	}
	words := strings.ReplaceAll(strings.ToLower(code), "_", " ")
	if words != "" {
		words = strings.ToUpper(words[:1]) + words[1:]
	}
	return map[string]string{LangEN: words, LangSW: words}
}

// text is the PDF wording in each language.
var text = map[string]map[string]string{
	LangEN: {
		"title": "Receipt", "number": "Receipt no.", "issued": "Issued", "trip": "Trip",
		"from": "From", "to": "To", "driver": "Driver", "vehicle": "Vehicle",
		"total": "Total paid", "vat": "Includes VAT", "levy": "Includes regulator levy",
		"paid_cash": "Paid in cash to the driver", "paid_mobile": "Paid by mobile money",
		"reference": "Reference", "distance": "Distance", "duration": "Duration",
		"footer":  "Thank you for riding with us. Questions about this trip? Contact support and quote the receipt number.",
		"support": "Support",
	},
	LangSW: {
		"title": "Risiti", "number": "Risiti na.", "issued": "Imetolewa", "trip": "Safari",
		"from": "Kutoka", "to": "Kwenda", "driver": "Dereva", "vehicle": "Gari",
		"total": "Jumla iliyolipwa", "vat": "Inajumuisha VAT", "levy": "Inajumuisha tozo ya mdhibiti",
		"paid_cash": "Imelipwa taslimu kwa dereva", "paid_mobile": "Imelipwa kwa pesa ya simu",
		"reference": "Kumbukumbu", "distance": "Umbali", "duration": "Muda",
		"footer":  "Asante kwa kusafiri nasi. Una swali kuhusu safari hii? Wasiliana na huduma kwa wateja ukitaja namba ya risiti.",
		"support": "Huduma kwa wateja",
	},
}

// langOf returns a supported language (English by default).
func langOf(lang string) string {
	if strings.HasPrefix(strings.ToLower(lang), LangSW) {
		return LangSW
	}
	return LangEN
}
