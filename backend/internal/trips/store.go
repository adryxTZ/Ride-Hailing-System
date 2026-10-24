package trips

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

type store struct {
	pool    *pgxpool.Pool
	brandID string
}

// quoteRow is what booking needs from a stored quote.
type quoteRow struct {
	ID         string
	UserID     *string
	ServiceKey string
	Pickup     [2]float64 // lat, lng
	Dropoff    *[2]float64
	Attributes json.RawMessage
	Route      json.RawMessage
	Surge      decimal.Decimal
	Options    json.RawMessage
	ExpiresAt  time.Time
	Waypoints  []routing.Point
	Country    string
	AreaID     string
}

func (s store) quote(ctx context.Context, tx pgx.Tx, id string) (*quoteRow, error) {
	var q quoteRow
	var dLat, dLng *float64
	err := tx.QueryRow(ctx, `
		SELECT id, user_id, service_key, ST_Y(pickup::geometry), ST_X(pickup::geometry),
		       ST_Y(dropoff::geometry), ST_X(dropoff::geometry), attributes, route, surge, options, expires_at, waypoints,
		       coalesce(country_code, (SELECT country FROM brands WHERE id=$2)), coalesce(area_id::text, '')
		FROM quotes WHERE id=$1 AND brand_id=$2`, id, s.brandID).
		Scan(&q.ID, &q.UserID, &q.ServiceKey, &q.Pickup[0], &q.Pickup[1], &dLat, &dLng, &q.Attributes, &q.Route, &q.Surge, &q.Options, &q.ExpiresAt, &q.Waypoints,
			&q.Country, &q.AreaID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if dLat != nil && dLng != nil {
		q.Dropoff = &[2]float64{*dLat, *dLng}
	}
	return &q, nil
}

type insertTrip struct {
	ServiceKey, VehicleClass, RiderID string
	Lifecycle                         Lifecycle
	Attributes, QuoteOption           json.RawMessage
	QuoteID                           string
	PricingSource                     string
	FixedRouteID                      *string
	Surge, EstimatedFare              decimal.Decimal
	Currency, PaymentMethod           string
	RouteDistanceM, RouteDurationS    *int
	Notes                             *string
	DispatchWindowEnds                time.Time
	Status                            Status // Requested, or Scheduled when booked ahead
	ScheduledFor                      *time.Time
	Stops                             []Stop
	Country, AreaID                   string
}

// insert writes the trip, its stops and the first event in the caller's transaction.
func (s store) insert(ctx context.Context, tx pgx.Tx, in insertTrip) (string, string, error) {
	lc, _ := json.Marshal(in.Lifecycle)
	var id, code string
	err := tx.QueryRow(ctx, `
		INSERT INTO trips (brand_id, code, service_key, vehicle_class_key, rider_id, lifecycle, attributes, quote_id, quote_option,
		  pricing_source, fixed_route_id, surge_multiplier, estimated_fare, currency, payment_method, route_distance_m, route_duration_s,
		  notes, dispatch_window_ends_at, seq, status, scheduled_for, country_code, area_id)
		VALUES ($1, upper(substr(md5(nextval('trip_code_seq')::text), 1, 6)), $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, 1, $19::trip_status, $20, $21, NULLIF($22,'')::uuid)
		RETURNING id, code`,
		s.brandID, in.ServiceKey, in.VehicleClass, in.RiderID, lc, in.Attributes, in.QuoteID, in.QuoteOption,
		in.PricingSource, in.FixedRouteID, in.Surge, in.EstimatedFare, in.Currency, in.PaymentMethod, in.RouteDistanceM, in.RouteDurationS,
		in.Notes, in.DispatchWindowEnds, string(in.Status), in.ScheduledFor, in.Country, in.AreaID).Scan(&id, &code)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "trips_one_active_per_rider" {
		return "", "", ErrTripAlreadyActive
	}
	if err != nil {
		return "", "", err
	}
	for _, st := range in.Stops {
		if _, err := tx.Exec(ctx, `INSERT INTO trip_stops (trip_id, seq, kind, location, address) VALUES ($1,$2,$3,ST_GeogFromText($4),$5)`,
			id, st.Seq, st.Kind, fmt.Sprintf("SRID=4326;POINT(%f %f)", st.Location.Lng, st.Location.Lat), st.Address); err != nil {
			return "", "", err
		}
	}
	return id, code, nil
}

// appendEvent bumps the trip's seq and writes the event. Callers hold the trip row lock.
func (s store) appendEvent(ctx context.Context, tx pgx.Tx, tripID, typ, actorKind string, actorID *string, payload any) (int, error) {
	body, _ := json.Marshal(payload)
	if payload == nil {
		body = []byte("{}")
	}
	var seq int
	if err := tx.QueryRow(ctx, `UPDATE trips SET seq = seq + 1, updated_at = now() WHERE id=$1 RETURNING seq`, tripID).Scan(&seq); err != nil {
		return 0, err
	}
	_, err := tx.Exec(ctx, `INSERT INTO trip_events (trip_id, seq, type, actor_kind, actor_id, payload) VALUES ($1,$2,$3,$4,$5,$6)`,
		tripID, seq, typ, actorKind, actorID, body)
	return seq, err
}

const tripColumns = `
	t.id, t.code, t.service_key, t.vehicle_class_key, t.rider_id, t.driver_id, t.status::text, t.lifecycle, t.attributes, t.quote_option,
	t.pricing_source, t.estimated_fare, t.final_fare, t.currency, t.payment_method::text, t.route_distance_m, t.route_duration_s, t.notes, t.seq,
	t.requested_at, t.offered_at, t.accepted_at, t.en_route_at, t.arrived_pickup_at, t.in_transit_at,
	t.arrived_dropoff_at, t.awaiting_payment_at, t.completed_at, t.cancelled_at, t.cancel_reason,
	ru.full_name, ru.phone, du.full_name, du.phone, t.fees, t.contacts, t.scheduled_for, t.adjustments, t.cancellation_fee,
	t.country_code, ar.id, ar.code, ar.name, t.payment_status, t.payment_ref, t.cash_chosen,
	t.queued_after, t.queued_pickup_by, t.queue_promoted_at`

// tripFrom joins the people on the trip; every trip query selects from it.
const tripFrom = ` FROM trips t JOIN users ru ON ru.id = t.rider_id LEFT JOIN users du ON du.id = t.driver_id
	LEFT JOIN areas ar ON ar.id = t.area_id `

func scanTrip(row pgx.Row) (*Trip, error) {
	var t Trip
	var status string
	var lc json.RawMessage
	var ts [9]*time.Time
	var riderName, driverName, driverPhone *string
	var areaID, areaCode, areaName *string
	var contacts, adjustments []byte
	var riderPhone string
	err := row.Scan(&t.ID, &t.Code, &t.ServiceKey, &t.VehicleClass, &t.RiderID, &t.DriverID, &status, &lc, &t.Attributes, &t.QuoteOption,
		&t.PricingSource, &t.EstimatedFare, &t.FinalFare, &t.Currency, &t.PaymentMethod, &t.RouteDistanceM, &t.RouteDurationS, &t.Notes, &t.Seq,
		&t.RequestedAt, &ts[0], &ts[1], &ts[2], &ts[3], &ts[4], &ts[5], &ts[6], &ts[7], &ts[8], &t.CancelReason,
		&riderName, &riderPhone, &driverName, &driverPhone, &t.Fees, &contacts, &t.ScheduledFor, &adjustments, &t.CancellationFee,
		&t.Country, &areaID, &areaCode, &areaName, &t.PaymentStatus, &t.PaymentRef, &t.cashChosen,
		&t.QueuedAfter, &t.queuedPickupBy, &t.queuePromotedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if areaID != nil {
		t.Area = &AreaRef{ID: *areaID, Code: *areaCode, Name: *areaName}
	}
	t.Status = Status(status)
	_ = json.Unmarshal(lc, &t.Lifecycle)
	names := []string{"offered_at", "accepted_at", "en_route_at", "arrived_pickup_at", "in_transit_at",
		"arrived_dropoff_at", "awaiting_payment_at", "completed_at", "cancelled_at"}
	t.Timestamps = map[string]*time.Time{}
	for i, n := range names {
		if ts[i] != nil {
			t.Timestamps[n] = ts[i]
		}
	}
	_ = json.Unmarshal(contacts, &t.Contacts)
	t.Adjustments = []Adjustment{}
	_ = json.Unmarshal(adjustments, &t.Adjustments)
	t.Rider = &Party{ID: t.RiderID, Name: riderName, Phone: riderPhone}
	if t.DriverID != nil && driverPhone != nil {
		t.Driver = &Party{ID: *t.DriverID, Name: driverName, Phone: *driverPhone}
	}
	t.CancelFeeNow = cancelFeeAt(&t, time.Now()) // read model: what cancelling now would cost
	t.Payment = paymentOf(&t)
	t.AllowedNext = AllowedNext(t.Status, t.Lifecycle)
	if t.AllowedNext == nil {
		t.AllowedNext = []Status{}
	}
	t.FeesView = feesView(t.Fees)
	applyQueue(&t, time.Now())
	return &t, nil
}

// get loads a trip with its stops. With forUpdate the row is locked for a transition.
func (s store) get(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, id string, forUpdate bool) (*Trip, error) {
	lock := ""
	if forUpdate {
		lock = " FOR UPDATE OF t"
	}
	t, err := scanTrip(q.QueryRow(ctx, `SELECT `+tripColumns+tripFrom+`WHERE t.id=$1 AND t.brand_id=$2`+lock, id, s.brandID))
	if err != nil || t == nil {
		return t, err
	}
	rows, err := q.Query(ctx, `SELECT seq, kind::text, ST_Y(location::geometry), ST_X(location::geometry), address, arrived_at FROM trip_stops WHERE trip_id=$1 ORDER BY seq`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	t.Stops = []Stop{}
	for rows.Next() {
		var st Stop
		if err := rows.Scan(&st.Seq, &st.Kind, &st.Location.Lat, &st.Location.Lng, &st.Address, &st.ArrivedAt); err != nil {
			return nil, err
		}
		t.Stops = append(t.Stops, st)
	}
	return t, rows.Err()
}

func (s store) activeForRider(ctx context.Context, riderID string) (*Trip, error) {
	var id string
	err := s.pool.QueryRow(ctx, `SELECT id FROM trips WHERE rider_id=$1 AND brand_id=$2 AND status = ANY($3::trip_status[]) ORDER BY created_at DESC LIMIT 1`,
		riderID, s.brandID, statusStrings(ActiveStatuses)).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s.get(ctx, s.pool, id, false)
}

func (s store) events(ctx context.Context, tripID string, afterSeq int) ([]Event, error) {
	rows, err := s.pool.Query(ctx, `SELECT seq, type, actor_kind, payload, at FROM trip_events WHERE trip_id=$1 AND seq > $2 ORDER BY seq LIMIT 200`, tripID, afterSeq)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.Seq, &e.Type, &e.ActorKind, &e.Payload, &e.At); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s store) setStatus(ctx context.Context, tx pgx.Tx, tripID string, to Status, stampColumn string, extra string, args ...any) error {
	set := `status=$2, updated_at=now(), version=version+1`
	if stampColumn != "" {
		set += `, ` + stampColumn + `=now()`
	}
	if extra != "" {
		set += `, ` + extra
	}
	_, err := tx.Exec(ctx, `UPDATE trips SET `+set+` WHERE id=$1`, append([]any{tripID, string(to)}, args...)...)
	return err
}

func (s store) listForRider(ctx context.Context, riderID string, limit int) ([]Trip, error) {
	return s.list(ctx, `t.rider_id=$1`, riderID, limit)
}

func (s store) listForDriver(ctx context.Context, driverID string, limit int) ([]Trip, error) {
	return s.list(ctx, `t.driver_id=$1`, driverID, limit)
}

// list loads trips matching one condition, newest first, with their stops in a
// second query (two round trips in total, never one per trip).
func (s store) list(ctx context.Context, where string, arg any, limit int) ([]Trip, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+tripColumns+tripFrom+`WHERE `+where+` AND t.brand_id=$2 ORDER BY t.created_at DESC LIMIT $3`, arg, s.brandID, limit)
	if err != nil {
		return nil, err
	}
	out := []Trip{}
	index := map[string]int{}
	var ids []string
	for rows.Next() {
		t, err := scanTrip(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		t.Stops = []Stop{}
		index[t.ID] = len(out)
		ids = append(ids, t.ID)
		out = append(out, *t)
	}
	rows.Close()
	if err := rows.Err(); err != nil || len(ids) == 0 {
		return out, err
	}
	return out, s.attachStops(ctx, out, index, ids)
}

func (s store) attachStops(ctx context.Context, trips []Trip, index map[string]int, ids []string) error {
	rows, err := s.pool.Query(ctx, `SELECT trip_id, seq, kind::text, ST_Y(location::geometry), ST_X(location::geometry), address, arrived_at
		FROM trip_stops WHERE trip_id = ANY($1::uuid[]) ORDER BY trip_id, seq`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var tripID string
		var st Stop
		if err := rows.Scan(&tripID, &st.Seq, &st.Kind, &st.Location.Lat, &st.Location.Lng, &st.Address, &st.ArrivedAt); err != nil {
			return err
		}
		if i, ok := index[tripID]; ok {
			trips[i].Stops = append(trips[i].Stops, st)
		}
	}
	return rows.Err()
}

func statusStrings(in []Status) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = string(s)
	}
	return out
}

func (s store) activeForDriver(ctx context.Context, driverID string) (*Trip, error) {
	var id string
	// The current trip only: a queued next trip waits until this one ends.
	err := s.pool.QueryRow(ctx, `SELECT id FROM trips WHERE driver_id=$1 AND brand_id=$2 AND status = ANY($3::trip_status[]) AND queued_after IS NULL
		ORDER BY created_at DESC LIMIT 1`, driverID, s.brandID, statusStrings(ActiveStatuses)).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s.get(ctx, s.pool, id, false)
}

// openRequests lists trips waiting for a driver with no open offer, oldest first.
func (s store) openRequests(ctx context.Context, limit int) ([]OpenRequest, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT t.id, t.rider_id, t.service_key, t.vehicle_class_key, coalesce(t.area_id::text, ''), t.country_code, ST_Y(st.location::geometry), ST_X(st.location::geometry), t.requested_at
		FROM trips t JOIN trip_stops st ON st.trip_id = t.id AND st.seq = 1
		WHERE t.brand_id=$1 AND t.status='REQUESTED' AND t.dispatch_window_ends_at > now()
		ORDER BY t.requested_at LIMIT $2`, s.brandID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []OpenRequest{}
	for rows.Next() {
		var r OpenRequest
		if err := rows.Scan(&r.TripID, &r.RiderID, &r.ServiceKey, &r.VehicleClass, &r.AreaID, &r.Country, &r.Pickup.Lat, &r.Pickup.Lng, &r.RequestedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
