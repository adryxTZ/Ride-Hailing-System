package surge

import "testing"

func TestMultiplier(t *testing.T) {
	cases := []struct {
		demand, supply int
		want           string
	}{
		{1, 0, "1"},    // a single request never surges
		{3, 5, "1"},    // more drivers than requests
		{4, 2, "1.25"}, // 2 requests per driver
		{10, 0, "2"},   // capped
	}
	for _, c := range cases {
		if got := multiplierFor(c.demand, c.supply); got.String() != c.want {
			t.Errorf("demand %d supply %d: got %s want %s", c.demand, c.supply, got, c.want)
		}
	}
}
