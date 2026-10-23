package compliance

import "testing"

func TestReminderDue(t *testing.T) {
	cases := []struct {
		left int
		sent []int
		want int
	}{
		{45, nil, 0},
		{30, nil, 30},
		{20, []int{30}, 0},
		{7, []int{30}, 7},
		{5, nil, 7}, // a late start still sends the tightest one due, once
		{1, []int{30, 7}, 1},
		{0, []int{30, 7}, 1},
		{0, []int{30, 7, 1}, 0},
		{-2, nil, 0}, // expired: the hold speaks, not a reminder
	}
	for _, c := range cases {
		if got := ReminderDue(c.left, c.sent); got != c.want {
			t.Errorf("left %d sent %v: got %d want %d", c.left, c.sent, got, c.want)
		}
	}
}
