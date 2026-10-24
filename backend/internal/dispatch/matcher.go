package dispatch

import "sort"

// candidate is one driver who could serve one trip. Queued candidates are
// finishing another trip: EtaS includes the finishing allowance.
type candidate struct {
	TripID    string
	DriverID  string
	DistanceM float64
	EtaS      int
	Queued    bool
}

// rank orders candidates: pickup ETA, with finishing drivers behind idle ones
// that are about as close (queue.go).
func (c candidate) rank() int {
	if c.Queued {
		return c.EtaS + queuedRankPenaltyS
	}
	return c.EtaS
}

// assign picks at most one driver per trip and one trip per driver, best rank
// (pickup ETA) first across the whole batch (greedy batched assignment). Ties
// go to the trip that has waited longest, which is the order trips arrive in.
func assign(tripOrder []string, cands []candidate) []candidate {
	waitRank := make(map[string]int, len(tripOrder))
	for i, id := range tripOrder {
		waitRank[id] = i
	}
	sorted := append([]candidate(nil), cands...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if ri, rj := sorted[i].rank(), sorted[j].rank(); ri != rj {
			return ri < rj
		}
		return waitRank[sorted[i].TripID] < waitRank[sorted[j].TripID]
	})
	usedTrip, usedDriver := map[string]bool{}, map[string]bool{}
	var out []candidate
	for _, c := range sorted {
		if usedTrip[c.TripID] || usedDriver[c.DriverID] {
			continue
		}
		usedTrip[c.TripID], usedDriver[c.DriverID] = true, true
		out = append(out, c)
	}
	return out
}
