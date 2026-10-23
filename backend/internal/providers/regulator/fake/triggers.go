package fake

import (
	"strings"

	"github.com/kalen-limited/hauler-rider/backend/internal/providers/regulator"
)

// Data triggers, so end-to-end runs can force outcomes through real records.
const (
	TriggerFailPlate   = "TFAIL"      // plate prefix: rejected 5001
	TriggerDownPlate   = "TDOWN"      // plate prefix: unavailable
	TriggerFailLicence = "9999999999" // licence number: rejected 5001
)

// driverTrigger checks a driver's licence.
func driverTrigger(d regulator.DriverRecord) error {
	return licenceTrigger(d.LicenceNumber)
}

// vehicleTrigger checks a vehicle's plate, then its driver's licence.
func vehicleTrigger(v regulator.VehicleRecord) error {
	if err := plateTrigger(v.Plate); err != nil {
		return err
	}
	return licenceTrigger(v.DriverLicence)
}

// tripTrigger checks the trip's vehicle plate, then the driver's licence.
func tripTrigger(t regulator.TripRecord) error {
	if err := plateTrigger(t.Vehicle.Plate); err != nil {
		return err
	}
	return licenceTrigger(t.Driver.LicenceNumber)
}

// plateTrigger matches the plate prefixes; spacing and case are ignored.
func plateTrigger(plate string) error {
	p := strings.ToUpper(strings.ReplaceAll(plate, " ", ""))
	switch {
	case strings.HasPrefix(p, TriggerDownPlate):
		return regulator.ErrUnavailable
	case strings.HasPrefix(p, TriggerFailPlate):
		return rejected("fake regulator: plate trigger " + TriggerFailPlate)
	}
	return nil
}

// licenceTrigger matches the failing licence number.
func licenceTrigger(licence string) error {
	if strings.TrimSpace(licence) == TriggerFailLicence {
		return rejected("fake regulator: licence trigger " + TriggerFailLicence)
	}
	return nil
}
