package fraud

// TripParties is what the self-booking check needs about a completed trip.
type TripParties struct {
	DriverPhone    string
	PickupPhone    string   // the passenger or sender named on the trip, if any
	ReceiverPhone  string   // the parcel receiver, if any
	SharedInstalls []string // app installs both the rider and the driver signed in from
}

// Signal is one reason to think a driver booked their own ride.
type Signal struct {
	Code     string `json:"code"` // SHARED_INSTALL | PASSENGER_IS_DRIVER
	Severity string `json:"severity"`
	Detail   string `json:"detail,omitempty"`
}

// Signal codes.
const (
	SignalSharedInstall     = "SHARED_INSTALL"
	SignalPassengerIsDriver = "PASSENGER_IS_DRIVER"
)

// SelfBookingSignals lists why a trip looks self-booked (empty when it does not).
func SelfBookingSignals(p TripParties) []Signal {
	var out []Signal
	if len(p.SharedInstalls) > 0 {
		out = append(out, Signal{Code: SignalSharedInstall, Severity: SeverityHigh})
	}
	if p.DriverPhone != "" && (p.PickupPhone == p.DriverPhone || p.ReceiverPhone == p.DriverPhone) {
		detail := "pickup contact"
		if p.ReceiverPhone == p.DriverPhone {
			detail = "receiver"
		}
		out = append(out, Signal{Code: SignalPassengerIsDriver, Severity: SeverityMedium, Detail: detail})
	}
	return out
}

// worstSeverity is the highest severity among signals.
func worstSeverity(signals []Signal) string {
	worst := SeverityLow
	for _, s := range signals {
		if s.Severity == SeverityHigh {
			return SeverityHigh
		}
		if s.Severity == SeverityMedium {
			worst = SeverityMedium
		}
	}
	return worst
}
