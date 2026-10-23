package compliance

import (
	"encoding/json"
	"time"

	"github.com/kalen-limited/hauler-rider/backend/internal/providers/regulator"
)

// NextAttempt is when to try again after `attempts` failed tries (1 = the first failure).
func NextAttempt(attempts int, now time.Time) time.Time {
	i := max(attempts-1, 0)
	if i >= len(RetryLadder) {
		i = len(RetryLadder) - 1
	}
	return now.Add(RetryLadder[i])
}

// ShouldSubmit reports whether a completed trip goes to the country's regulator:
// the country names one, and the service's compliance settings ask for trips
// ({"latra": {"submit_trips": true}}).
func ShouldSubmit(countryRegulator string, serviceCompliance json.RawMessage) bool {
	if countryRegulator == "" || countryRegulator == "none" {
		return false
	}
	var rules map[string]struct {
		SubmitTrips bool `json:"submit_trips"`
	}
	if json.Unmarshal(serviceCompliance, &rules) != nil {
		return false
	}
	return rules[countryRegulator].SubmitTrips
}

// outcome is what an answer from the regulator does to a submission.
type outcome struct {
	status       string // new status (PENDING stays pending)
	code, msg    string
	retry        bool // count an attempt and wait on the ladder
	prerequisite bool // register the driver and vehicle again first
}

// outcomeOf turns an adapter result into the submission's next state.
func outcomeOf(ack regulator.Ack, err error) outcome {
	if err == nil {
		return outcome{status: StatusAccepted, code: ack.Code}
	}
	r, ok := regulator.AsRejection(err)
	if !ok {
		return outcome{status: StatusPending, code: "UNAVAILABLE", msg: err.Error(), retry: true}
	}
	switch r.Class {
	case regulator.ClassNotNeeded:
		return outcome{status: StatusSkipped, code: r.Code, msg: r.Message}
	case regulator.ClassPrerequisite:
		return outcome{status: StatusPending, code: r.Code, msg: r.Message, retry: true, prerequisite: true}
	}
	return outcome{status: StatusPending, code: r.Code, msg: r.Message, retry: true}
}
