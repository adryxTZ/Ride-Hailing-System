package compliance

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/kalen-limited/hauler-rider/backend/internal/providers/regulator"
)

func TestLadder(t *testing.T) {
	now := time.Unix(0, 0)
	want := []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute, 3 * time.Hour, 24 * time.Hour, 24 * time.Hour, 24 * time.Hour}
	for i, w := range want {
		if got := NextAttempt(i+1, now).Sub(now); got != w {
			t.Errorf("after %d failures: got %s want %s", i+1, got, w)
		}
	}
}

func TestShouldSubmit(t *testing.T) {
	taxi := json.RawMessage(`{"latra": {"submit_trips": true}}`)
	parcel := json.RawMessage(`{"latra": {"submit_trips": false}}`)
	cases := []struct {
		name, regulator string
		rules           json.RawMessage
		want            bool
	}{
		{"TZ taxi", "latra", taxi, true},
		{"TZ parcel", "latra", parcel, false},
		{"KE has none", "none", taxi, false},
		{"no rules", "latra", nil, false},
	}
	for _, c := range cases {
		if got := ShouldSubmit(c.regulator, c.rules); got != c.want {
			t.Errorf("%s: got %v", c.name, got)
		}
	}
}

func TestOutcomes(t *testing.T) {
	if o := outcomeOf(regulator.Ack{Code: "5000"}, nil); o.status != StatusAccepted {
		t.Fatal(o)
	}
	if o := outcomeOf(regulator.Ack{}, &regulator.Rejection{Code: "5016", Class: regulator.ClassNotNeeded}); o.status != StatusSkipped || o.retry {
		t.Fatal(o)
	}
	if o := outcomeOf(regulator.Ack{}, &regulator.Rejection{Code: "5018", Class: regulator.ClassPrerequisite}); !o.prerequisite || !o.retry {
		t.Fatal(o)
	}
	if o := outcomeOf(regulator.Ack{}, errors.New("timeout")); o.status != StatusPending || !o.retry {
		t.Fatal(o)
	}
}
