package cets

import (
	"strings"

	"github.com/kalen-limited/hauler-rider/backend/internal/providers/regulator"
)

// CeTS accepts exactly these vehicle types (vehicleTypeEnum). The value comes
// from the vehicle class's regulator type; names are never guessed.
var vehicleTypes = map[string]bool{"CAR": true, "MOTORCYCLE": true, "TRICYCLE": true}

// Rating bounds (CeTS: "Integer, in range 0-5"; 0 = not rated).
const (
	minRating = 0
	maxRating = 5
)

// codeInvalidRecord marks a rejection raised locally, before sending, because
// our record cannot be expressed in CeTS terms (staff must fix the data).
const codeInvalidRecord = "INVALID_RECORD"

// driverBody is the driver registration (API doc §4). Field order is CeTS's.
type driverBody struct {
	DriverName          *string `json:"driverName"`
	DriverEmail         *string `json:"driverEmail"`
	DriverPhone         *string `json:"driverPhone"`
	DriverLicenseNumber *string `json:"driverLicenseNumber"`
	LicenceCategory     *string `json:"licenceCategory"`
}

// vehicleBody is the vehicle registration (API doc §3).
type vehicleBody struct {
	VehicleRegistrationNumber *string `json:"vehicleRegistrationNumber"`
	VehicleModel              *string `json:"vehicleModel"`
	VehicleTypeEnum           string  `json:"vehicleTypeEnum"`
	VehicleRegistrationDate   *string `json:"vehicleRegistrationDate"`
}

// tripBody is a completed trip (API doc §5): the fifteen fields, verbatim.
type tripBody struct {
	TripID                    string  `json:"tripId"`
	OriginCoordinates         *string `json:"originCoordinates"`
	EndCoordinates            *string `json:"endCoordinates"`
	StartTime                 *string `json:"startTime"`
	EndTime                   *string `json:"endTime"`
	TotalFareAmount           *int64  `json:"totalFareAmount"`
	TripDistance              int     `json:"tripDistance"` // metres
	Rating                    int     `json:"rating"`
	DriverEarning             int64   `json:"driverEarning"`
	DriverLicenseNumber       *string `json:"driverLicenseNumber"`
	VehicleRegistrationNumber *string `json:"vehicleRegistrationNumber"`
	PassengerName             *string `json:"passengerName"`
	PassengerPhone            *string `json:"passengerPhone"`
	PickupPoint               *string `json:"pickupPoint"`
	DropOffPoint              *string `json:"dropOffPoint"`
}

// reportBody is the daily summary (API doc §6).
type reportBody struct {
	TotalTrip       int    `json:"totalTrip"`
	TotalPaidAmount int64  `json:"totalPaidAmount"`
	ReportDate      string `json:"reportDate"`
}

// driverPayload builds the driver registration body.
func driverPayload(d regulator.DriverRecord) driverBody {
	return driverBody{
		DriverName:          text(d.Name),
		DriverEmail:         text(d.Email),
		DriverPhone:         text(d.Phone),
		DriverLicenseNumber: text(d.LicenceNumber),
		LicenceCategory:     text(strings.ToUpper(d.LicenceClass)),
	}
}

// vehiclePayload builds the vehicle registration body.
func vehiclePayload(v regulator.VehicleRecord) (vehicleBody, error) {
	kind := strings.ToUpper(strings.TrimSpace(v.VehicleType))
	if !vehicleTypes[kind] {
		return vehicleBody{}, invalid("vehicle type " + v.VehicleType + " is not CAR, MOTORCYCLE or TRICYCLE")
	}
	body := vehicleBody{
		VehicleRegistrationNumber: plate(v.Plate),
		VehicleModel:              text(v.Make + " " + v.Model),
		VehicleTypeEnum:           kind,
	}
	if v.RegisteredOn != nil {
		loc, err := location(v.Timezone)
		if err != nil {
			return vehicleBody{}, invalid(err.Error())
		}
		body.VehicleRegistrationDate = localDate(v.RegisteredOn, loc)
	}
	return body, nil
}

// tripPayload builds the completed-trip body.
func tripPayload(t regulator.TripRecord) (tripBody, error) {
	loc, err := location(t.Timezone)
	if err != nil {
		return tripBody{}, invalid(err.Error())
	}
	return tripBody{
		TripID:                    t.TripID,
		OriginCoordinates:         coords(t.Pickup),
		EndCoordinates:            coords(t.Dropoff),
		StartTime:                 localDateTime(t.StartedAt, loc),
		EndTime:                   localDateTime(t.EndedAt, loc),
		TotalFareAmount:           positiveAmount(t.Fare),
		TripDistance:              t.DistanceM,
		Rating:                    clampRating(t.Rating),
		DriverEarning:             wholeAmount(t.DriverEarning),
		DriverLicenseNumber:       text(t.Driver.LicenceNumber),
		VehicleRegistrationNumber: plate(t.Vehicle.Plate),
		PassengerName:             text(t.PassengerName),
		PassengerPhone:            text(t.PassengerPhone),
		PickupPoint:               text(t.PickupAddress),
		DropOffPoint:              text(t.DropoffAddress),
	}, nil
}

// reportPayload builds the daily summary body.
func reportPayload(r regulator.DailyReport) (reportBody, error) {
	loc, err := location(r.Timezone)
	if err != nil {
		return reportBody{}, invalid(err.Error())
	}
	return reportBody{
		TotalTrip:       r.Trips,
		TotalPaidAmount: wholeAmount(r.Paid),
		ReportDate:      r.Date.In(loc).Format(dateLayout),
	}, nil
}

// clampRating keeps a rating inside CeTS's 0–5 range.
func clampRating(r int) int {
	return max(minRating, min(maxRating, r))
}

// invalid is a FIX_DATA rejection raised before anything is sent.
func invalid(msg string) *regulator.Rejection {
	return &regulator.Rejection{Code: codeInvalidRecord, Message: msg, Class: regulator.ClassFixData}
}
