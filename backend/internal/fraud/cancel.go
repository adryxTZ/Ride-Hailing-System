package fraud

import "math"

// CancelCounts are one person's numbers over the cancellation window.
type CancelCounts struct {
	SubjectKind   string
	SubjectID     string
	Accepted      int // driver: trips accepted; rider: trips that got a driver
	Cancels       int // driver: cancels after accepting (not no-shows); rider: cancels after a driver was assigned
	Arrivals      int // driver: arrivals at the pickup
	NoShows       int // driver: rider no-show cancels
	CancelTripIDs []string
}

// RateFinding is a rate over its threshold.
type RateFinding struct {
	Rule      string   `json:"rule"`
	Count     int      `json:"count"`
	Of        int      `json:"of"`
	Rate      float64  `json:"rate"`
	Threshold float64  `json:"threshold"`
	TripIDs   []string `json:"trip_ids"`
}

// CancelFindings returns the rates at or over their thresholds, with enough sample.
func CancelFindings(c CancelCounts) []RateFinding {
	var out []RateFinding
	add := func(rule string, n, of, min int, threshold float64) {
		if of < min || of == 0 {
			return
		}
		rate := float64(n) / float64(of)
		if rate >= threshold {
			out = append(out, RateFinding{Rule: rule, Count: n, Of: of, Rate: math.Round(rate*1000) / 1000, Threshold: threshold, TripIDs: c.CancelTripIDs})
		}
	}
	if c.SubjectKind == SubjectDriver {
		add(RuleDriverCancelRate, c.Cancels, c.Accepted, DriverCancelMinSample, DriverCancelRateFlag)
		add(RuleDriverNoShowRate, c.NoShows, c.Arrivals, NoShowMinSample, NoShowRateFlag)
	} else {
		add(RuleRiderCancelRate, c.Cancels, c.Accepted, RiderCancelMinSample, RiderCancelRateFlag)
	}
	return out
}
