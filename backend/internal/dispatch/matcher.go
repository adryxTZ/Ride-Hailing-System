package dispatch

import "sort"

// candidate is one driver who could serve one trip.
type candidate struct {
	TripID    string
	DriverID  string
	DistanceM float64
	EtaS      int
}

// assign picks at most one driver per trip and one trip per driver, cheapest
// pickup ETA first across the whole batch (greedy batched assignment). Ties go
// to the trip that has waited longest, which is the order trips arrive in.
func assign(tripOrder []string, cands []candidate) []candidate {
	waitRank := make(map[string]int, len(tripOrder))
	for i, id := range tripOrder {
		waitRank[id] = i
	}
	sorted := append([]candidate(nil), cands...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].EtaS != sorted[j].EtaS {
			return sorted[i].EtaS < sorted[j].EtaS
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
