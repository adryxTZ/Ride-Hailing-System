package compliance

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/providers/regulator"
)

// errMissing means the subject of a submission no longer exists or is incomplete.
var errMissing = errors.New("compliance: subject not found")

// driverRecord reads a driver as the regulator registers them (one query).
func (s store) driverRecord(ctx context.Context, driverID, tz string) (regulator.DriverRecord, error) {
	d := regulator.DriverRecord{DriverID: driverID, Timezone: tz}
	var name, email, licence, class *string
	err := s.pool.QueryRow(ctx, `SELECT u.full_name, u.phone, u.email::text, p.licence_number, p.licence_class, p.country_code
		FROM users u JOIN driver_profiles p ON p.user_id=u.id WHERE u.id=$1 AND u.brand_id=$2`, driverID, s.brandID).
		Scan(&name, &d.Phone, &email, &licence, &class, &d.Country)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, errMissing
	}
	d.Name, d.Email, d.LicenceNumber, d.LicenceClass = deref(name), deref(email), deref(licence), deref(class)
	return d, err
}

// vehicleRecord reads a vehicle with the regulator type of its class (one query).
func (s store) vehicleRecord(ctx context.Context, vehicleID, tz string) (regulator.VehicleRecord, error) {
	v := regulator.VehicleRecord{VehicleID: vehicleID, Timezone: tz}
	var driverID, make_, model, vtype, licence, country *string
	var created time.Time
	err := s.pool.QueryRow(ctx, `SELECT v.driver_id, v.plate, v.make, v.model, c.regulator_type, p.licence_number, p.country_code, v.created_at
		FROM vehicles v LEFT JOIN services s ON s.key=v.service_key AND s.brand_id=v.brand_id
		LEFT JOIN vehicle_classes c ON c.service_id=s.id AND c.key=v.class_key
		LEFT JOIN driver_profiles p ON p.user_id=v.driver_id
		WHERE v.id=$1 AND v.brand_id=$2`, vehicleID, s.brandID).Scan(&driverID, &v.Plate, &make_, &model, &vtype, &licence, &country, &created)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, errMissing
	}
	v.DriverID, v.Make, v.Model, v.VehicleType, v.DriverLicence, v.Country = deref(driverID), deref(make_), deref(model), deref(vtype), deref(licence), deref(country)
	v.RegisteredOn = &created
	return v, err
}

// tripRow is what one query returns for a trip submission.
type tripRow struct {
	code, riderPhone, country, currency string
	riderName, driverID, vehicleID      *string
	contacts, fees                      []byte
	start, end                          *time.Time
	distance                            *int
	fare                                decimal.Decimal
	rating                              int
	pickup, dropoff                     regulator.Point
	pickupAddr, dropoffAddr             *string
}

// tripRecord reads a completed trip with its people, vehicle, stops, fare and rating.
func (s store) tripRecord(ctx context.Context, tripID, tz string) (regulator.TripRecord, error) {
	var r tripRow
	err := s.pool.QueryRow(ctx, `SELECT t.code, ru.phone, ru.full_name, t.country_code, t.currency, t.driver_id,
		  (SELECT id FROM vehicles WHERE driver_id=t.driver_id ORDER BY created_at DESC LIMIT 1),
		  t.contacts, t.fees, coalesce(t.in_transit_at, t.accepted_at), t.completed_at, t.route_distance_m, coalesce(t.final_fare, t.estimated_fare),
		  coalesce((SELECT score FROM trip_ratings WHERE trip_id=t.id AND ratee_id=t.driver_id LIMIT 1), 0),
		  ST_Y(pk.location::geometry), ST_X(pk.location::geometry), ST_Y(dp.location::geometry), ST_X(dp.location::geometry), pk.address, dp.address
		FROM trips t JOIN users ru ON ru.id=t.rider_id
		JOIN trip_stops pk ON pk.trip_id=t.id AND pk.kind='PICKUP' JOIN trip_stops dp ON dp.trip_id=t.id AND dp.kind='DROPOFF'
		WHERE t.id=$1 AND t.brand_id=$2 AND t.status='COMPLETED'`, tripID, s.brandID).
		Scan(&r.code, &r.riderPhone, &r.riderName, &r.country, &r.currency, &r.driverID, &r.vehicleID, &r.contacts, &r.fees,
			&r.start, &r.end, &r.distance, &r.fare, &r.rating, &r.pickup.Lat, &r.pickup.Lng, &r.dropoff.Lat, &r.dropoff.Lng, &r.pickupAddr, &r.dropoffAddr)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (r.driverID == nil || r.vehicleID == nil || r.end == nil)) {
		return regulator.TripRecord{}, errMissing
	}
	if err != nil {
		return regulator.TripRecord{}, err
	}
	return s.assembleTrip(ctx, tripID, tz, r)
}

func (s store) assembleTrip(ctx context.Context, tripID, tz string, r tripRow) (regulator.TripRecord, error) {
	d, err := s.driverRecord(ctx, *r.driverID, tz)
	if err != nil {
		return regulator.TripRecord{}, err
	}
	v, err := s.vehicleRecord(ctx, *r.vehicleID, tz)
	if err != nil {
		return regulator.TripRecord{}, err
	}
	t := regulator.TripRecord{TripID: tripID, Code: r.code, Driver: d, Vehicle: v, Pickup: r.pickup, Dropoff: r.dropoff,
		PickupAddress: deref(r.pickupAddr), DropoffAddress: deref(r.dropoffAddr), EndedAt: *r.end, Fare: r.fare,
		Rating: r.rating, Currency: r.currency, Timezone: tz, PassengerName: deref(r.riderName), PassengerPhone: r.riderPhone}
	if r.start != nil {
		t.StartedAt = *r.start
	}
	if r.distance != nil {
		t.DistanceM = *r.distance
	}
	passenger(&t, r.contacts)
	var fees struct {
		DriverEarnings decimal.Decimal `json:"driver_earnings"`
	}
	_ = json.Unmarshal(r.fees, &fees)
	t.DriverEarning = fees.DriverEarnings
	return t, nil
}

// passenger is the "who is riding" contact when the rider booked for someone else.
func passenger(t *regulator.TripRecord, contacts []byte) {
	var c struct {
		Pickup *struct {
			Name  string `json:"name"`
			Phone string `json:"phone"`
		} `json:"pickup"`
	}
	if json.Unmarshal(contacts, &c) == nil && c.Pickup != nil && c.Pickup.Phone != "" {
		t.PassengerName, t.PassengerPhone = c.Pickup.Name, c.Pickup.Phone
	}
}

// dayTotals counts a country's completed trips (of services that submit trips) in a local day.
func (s store) dayTotals(ctx context.Context, country, regulatorName string, from, to time.Time) (int, decimal.Decimal, error) {
	var n int
	var sum decimal.Decimal
	err := s.pool.QueryRow(ctx, `SELECT count(*), coalesce(sum(coalesce(t.final_fare, t.estimated_fare)), 0)
		FROM trips t JOIN services s ON s.key=t.service_key AND s.brand_id=t.brand_id
		WHERE t.brand_id=$1 AND t.country_code=$2 AND t.status='COMPLETED' AND t.completed_at >= $3 AND t.completed_at < $4
		  AND coalesce((s.compliance -> $5 ->> 'submit_trips')::boolean, false)`, s.brandID, country, from, to, regulatorName).Scan(&n, &sum)
	return n, sum, err
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
