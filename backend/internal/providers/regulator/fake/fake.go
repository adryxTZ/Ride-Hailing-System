package fake

import (
	"context"
	"crypto/rand"
	"strings"
	"sync"

	"github.com/kalen-limited/hauler-rider/backend/internal/providers/regulator"
)

// DefaultName is the regulator key the fake stands in for.
const DefaultName = "latra"

// Modes, from NEXA_FAKE_REGULATOR_MODE.
const (
	ModeOK     = "ok"     // every submission is accepted
	ModeReject = "reject" // every submission is rejected with code 5001
	ModeDown   = "down"   // every submission fails with regulator.ErrUnavailable
)

// Submission kinds, matching the compliance queue's kind column.
const (
	KindDriver      = "DRIVER"
	KindVehicle     = "VEHICLE"
	KindTrip        = "TRIP"
	KindDailyReport = "DAILY_REPORT"
)

// Regulator-side codes the fake answers with (the CeTS vocabulary).
const (
	CodeSuccess  = "5000"
	CodeRejected = "5001"
)

// referencePrefix and referenceLen shape fake acceptance references.
const (
	referencePrefix = "FAKE-"
	referenceLen    = 8
	referenceChars  = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
)

// Regulator is the fake adapter. The zero value is not usable; call New.
type Regulator struct {
	// RegulatorName is what Name returns; New sets DefaultName.
	RegulatorName string

	mode    string
	mu      sync.Mutex
	scripts map[string][]error
}

// New returns a fake in the given mode; an unknown or empty mode means ok.
func New(mode string) *Regulator {
	m := strings.ToLower(strings.TrimSpace(mode))
	if m != ModeReject && m != ModeDown {
		m = ModeOK
	}
	return &Regulator{RegulatorName: DefaultName, mode: m, scripts: map[string][]error{}}
}

// Mode returns the effective mode (ok, reject or down).
func (f *Regulator) Mode() string { return f.mode }

// Name implements regulator.Regulator.
func (f *Regulator) Name() string { return f.RegulatorName }

// RegisterDriver implements regulator.Regulator.
func (f *Regulator) RegisterDriver(_ context.Context, d regulator.DriverRecord) (regulator.Ack, error) {
	return f.answer(KindDriver, driverTrigger(d))
}

// RegisterVehicle implements regulator.Regulator.
func (f *Regulator) RegisterVehicle(_ context.Context, v regulator.VehicleRecord) (regulator.Ack, error) {
	return f.answer(KindVehicle, vehicleTrigger(v))
}

// SubmitTrip implements regulator.Regulator.
func (f *Regulator) SubmitTrip(_ context.Context, t regulator.TripRecord) (regulator.Ack, error) {
	return f.answer(KindTrip, tripTrigger(t))
}

// SubmitDailyReport implements regulator.Regulator.
func (f *Regulator) SubmitDailyReport(_ context.Context, _ regulator.DailyReport) (regulator.Ack, error) {
	return f.answer(KindDailyReport, nil)
}

// answer resolves one submission: script, then trigger, then mode.
func (f *Regulator) answer(kind string, trigger error) (regulator.Ack, error) {
	if scripted, ok := f.next(kind); ok {
		return result(scripted.err)
	}
	if trigger != nil {
		return regulator.Ack{}, trigger
	}
	switch f.mode {
	case ModeReject:
		return regulator.Ack{}, rejected("fake regulator in reject mode")
	case ModeDown:
		return regulator.Ack{}, regulator.ErrUnavailable
	}
	return accepted(), nil
}

// result turns a scripted outcome into the port's answer (nil = accepted).
func result(err error) (regulator.Ack, error) {
	if err != nil {
		return regulator.Ack{}, err
	}
	return accepted(), nil
}

// accepted is a successful acknowledgement with a fresh reference.
func accepted() regulator.Ack {
	return regulator.Ack{Reference: referencePrefix + randomRef(), Code: CodeSuccess}
}

// rejected is a 5001 rejection the sweeper retries.
func rejected(msg string) *regulator.Rejection {
	return &regulator.Rejection{Code: CodeRejected, Message: msg, Class: regulator.ClassRetry}
}

// randomRef returns referenceLen characters from an unambiguous alphabet.
func randomRef() string {
	buf := make([]byte, referenceLen)
	_, _ = rand.Read(buf) // crypto/rand.Read never fails on supported platforms
	for i, b := range buf {
		buf[i] = referenceChars[int(b)%len(referenceChars)]
	}
	return string(buf)
}
