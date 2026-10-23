package regulator

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// Regulator registers drivers and vehicles and receives trips and daily reports.
// Implementations must not retry internally: the compliance sweeper owns retries.
type Regulator interface {
	// Name is the regulator key stored on submissions (e.g. "latra").
	Name() string
	RegisterDriver(ctx context.Context, d DriverRecord) (Ack, error)
	RegisterVehicle(ctx context.Context, v VehicleRecord) (Ack, error)
	SubmitTrip(ctx context.Context, t TripRecord) (Ack, error)
	SubmitDailyReport(ctx context.Context, r DailyReport) (Ack, error)
}

// Ack is the regulator's acceptance.
type Ack struct {
	Reference string // the regulator's reference number, when it gives one
	Code      string // the regulator's own success code
}

// Point is a location (GeoJSON order kept by name).
type Point struct {
	Lat float64
	Lng float64
}

// DriverRecord is a driver as the regulator registers them.
type DriverRecord struct {
	DriverID      string
	Name          string
	Phone         string // E.164
	Email         string
	LicenceNumber string
	LicenceClass  string
	Country       string
	Timezone      string // IANA, for local times
}

// VehicleRecord is a vehicle as the regulator registers it.
type VehicleRecord struct {
	VehicleID     string
	DriverID      string
	Plate         string
	Make, Model   string
	VehicleType   string // regulator vehicle type from the vehicle class (e.g. CAR, MOTORCYCLE, TRICYCLE)
	RegisteredOn  *time.Time
	Country       string
	Timezone      string
	DriverLicence string
}

// TripRecord is a completed trip as the regulator receives it.
type TripRecord struct {
	TripID         string
	Code           string
	Driver         DriverRecord
	Vehicle        VehicleRecord
	PassengerName  string
	PassengerPhone string
	Pickup         Point
	Dropoff        Point
	PickupAddress  string
	DropoffAddress string
	StartedAt      time.Time
	EndedAt        time.Time
	DistanceM      int
	Fare           decimal.Decimal // what the rider paid
	DriverEarning  decimal.Decimal
	Rating         int // rider's rating of the driver, 0 = none
	Currency       string
	Timezone       string
}

// DailyReport is one country day: completed trips and what riders paid.
type DailyReport struct {
	Country  string
	Date     time.Time // the local calendar day (midnight in Timezone)
	Timezone string
	Trips    int
	Paid     decimal.Decimal
	Currency string
}

// Rejection classes: what the sweeper does next.
const (
	ClassRetry        = "RETRY"        // try again on the ladder
	ClassFixData      = "FIX_DATA"     // our data is wrong; retry after staff fix it (still on the ladder)
	ClassNotNeeded    = "NOT_NEEDED"   // nothing to send (e.g. a zero-trip day): close locally
	ClassPrerequisite = "PREREQUISITE" // register the driver and vehicle again first
)

// Rejection is a definite answer from the regulator other than acceptance.
type Rejection struct {
	Code    string
	Message string
	Class   string
}

func (r *Rejection) Error() string {
	return fmt.Sprintf("regulator rejected (%s, %s): %s", r.Code, r.Class, r.Message)
}

// Errors adapters return.
var (
	ErrNotConfigured = errors.New("regulator: adapter not configured")
	ErrUnavailable   = errors.New("regulator: unavailable")
)

// AsRejection returns the rejection inside err, if any.
func AsRejection(err error) (*Rejection, bool) {
	var r *Rejection
	ok := errors.As(err, &r)
	return r, ok
}
