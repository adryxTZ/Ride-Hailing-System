package trips

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/catalog"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
)

// DispatchWindow is how long matching runs before NO_DRIVER_FOUND (per-service
// overrides come with the dispatch module).
const DispatchWindow = 3 * time.Minute

// Hook lets other modules react inside the booking transaction (dispatch
// enqueues its saga here). Nil hooks are skipped.
type Hook func(ctx context.Context, tx pgx.Tx, trip *Trip) error

type Service struct {
	store     store
	catalog   *catalog.Catalog
	now       func() time.Time
	onCreated Hook
	onChanged []func(ctx context.Context, t *Trip)
	// onCompletedTx run inside every completing transaction (ledger posting, receipt).
	onCompletedTx []CompletedTxHook
	// onFareDue runs after a ride ends and its fare is due (automatic prompt).
	onFareDue FareDueHook
	// prompts reads the latest fare prompt for read models.
	prompts PromptSource
	fees    FeeFunc
	// onInsertedTx run when a trip is booked (pricing record pin).
	onInsertedTx []InsertedTxHook
	// dues returns what a rider owes from earlier cancellations (nil = none carried over).
	dues func(ctx context.Context, riderID string) (decimal.Decimal, error)
}

// FeeFunc splits a trip's fare (total, of which bookingFee) under the pricing
// terms pinned on that trip (read in tx) and returns the breakdown to store.
// Provided by the money wiring; trips knows nothing about profiles or rates.
type FeeFunc func(ctx context.Context, tx pgx.Tx, tripID string, total, bookingFee decimal.Decimal, currency string) (json.RawMessage, error)

// SetFees registers how fares are split (nil = trips carry no breakdown).
func (s *Service) SetFees(f FeeFunc) { s.fees = f }

// InsertedTxHook runs right after a booking's trip row is written, in the same
// transaction (pricing pins the quote's terms here); an error aborts the booking.
type InsertedTxHook func(ctx context.Context, tx pgx.Tx, tripID, quoteID, vehicleClass string) error

// OnInsertedTx registers a hook run when a trip is booked, before its fare is split.
func (s *Service) OnInsertedTx(h InsertedTxHook) { s.onInsertedTx = append(s.onInsertedTx, h) }

// storeFees computes and saves the split for a fare; failures are logged, never block a trip.
func (s *Service) storeFees(ctx context.Context, tx pgx.Tx, tripID string, total, bookingFee decimal.Decimal, currency string) {
	if s.fees == nil {
		return
	}
	raw, err := s.fees(ctx, tx, tripID, total, bookingFee, currency)
	if err != nil {
		slog.Warn("fare split failed", "trip", tripID, "err", err)
		return
	}
	if _, err := tx.Exec(ctx, `UPDATE trips SET fees=$2 WHERE id=$1`, tripID, raw); err != nil {
		slog.Warn("save fare split failed", "trip", tripID, "err", err)
	}
}

// bookingFeeOf reads the BOOKING_FEE line from a quote option.
func bookingFeeOf(option json.RawMessage) decimal.Decimal {
	var o struct {
		Lines []struct {
			Code   string          `json:"code"`
			Amount decimal.Decimal `json:"amount"`
		} `json:"lines"`
	}
	_ = json.Unmarshal(option, &o)
	for _, l := range o.Lines {
		if l.Code == "BOOKING_FEE" {
			return l.Amount
		}
	}
	return decimal.Zero
}

func New(pool *pgxpool.Pool, brandID string, cat *catalog.Catalog) *Service {
	return &Service{store: store{pool: pool, brandID: brandID}, catalog: cat, now: time.Now}
}

// OnCreated registers the hook run after a trip is inserted (same transaction).
func (s *Service) OnCreated(h Hook) { s.onCreated = h }

// quoteOption mirrors pricing.Option on the wire.
type quoteOption struct {
	VehicleClass string          `json:"vehicle_class"`
	Available    bool            `json:"available"`
	Fare         decimal.Decimal `json:"fare"`
	Currency     string          `json:"currency"`
	Source       string          `json:"pricing_source"`
	FixedRouteID string          `json:"fixed_route_id"`
}

// Create books a trip from a quote option. Everything the trip needs is
// copied from the quote so a later tariff change cannot alter this booking.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Trip, error) {
	if err := validateCreate(in); err != nil {
		return nil, err
	}
	status, windowEnds, err := s.bookingTiming(in.ScheduledFor)
	if err != nil {
		return nil, err
	}
	var trip *Trip
	err = pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		q, err := s.store.quote(ctx, tx, in.QuoteID)
		if err != nil {
			return err
		}
		if q == nil || (q.UserID != nil && *q.UserID != in.RiderID) {
			return ErrQuoteNotFound
		}
		if s.now().After(q.ExpiresAt) {
			return ErrQuoteExpired
		}
		opt, raw, err := pickOption(q.Options, in.VehicleClass)
		if err != nil {
			return err
		}
		svc, err := s.catalog.Find(ctx, q.ServiceKey)
		if err != nil {
			return err
		}
		if svc == nil {
			return apperr.NotFound
		}
		var lc Lifecycle
		_ = json.Unmarshal(svc.Lifecycle, &lc)
		var route struct {
			DistanceM int `json:"distance_m"`
			DurationS int `json:"duration_s"`
		}
		_ = json.Unmarshal(q.Route, &route)

		if err := checkContacts(lc, in.Contacts); err != nil {
			return err
		}
		stops := buildStops(q, in.Stops)
		var fixedRoute *string
		if opt.FixedRouteID != "" {
			fixedRoute = &opt.FixedRouteID
		}
		id, _, err := s.store.insert(ctx, tx, insertTrip{
			ServiceKey: q.ServiceKey, VehicleClass: opt.VehicleClass, RiderID: in.RiderID, Lifecycle: lc,
			Attributes: q.Attributes, QuoteOption: raw, QuoteID: q.ID, PricingSource: opt.Source, FixedRouteID: fixedRoute,
			Surge: q.Surge, EstimatedFare: opt.Fare, Currency: opt.Currency, PaymentMethod: in.PaymentMethod, Country: q.Country, AreaID: q.AreaID,
			RouteDistanceM: nonZero(route.DistanceM), RouteDurationS: nonZero(route.DurationS), Notes: in.Notes,
			DispatchWindowEnds: windowEnds, Stops: stops, Status: status, ScheduledFor: in.ScheduledFor,
		})
		if err != nil {
			return err
		}
		for _, h := range s.onInsertedTx {
			if err := h(ctx, tx, id, q.ID, opt.VehicleClass); err != nil {
				return err
			}
		}
		s.storeFees(ctx, tx, id, opt.Fare, bookingFeeOf(raw), opt.Currency)
		if err := s.addDues(ctx, tx, id, in.RiderID); err != nil {
			return err
		}
		if contacts, _ := json.Marshal(in.Contacts); len(contacts) > 2 {
			if _, err := tx.Exec(ctx, `UPDATE trips SET contacts=$2 WHERE id=$1`, id, contacts); err != nil {
				return err
			}
		}
		event := "trip.requested"
		if status == Scheduled {
			event = "trip.scheduled"
		}
		if _, err := tx.Exec(ctx, `INSERT INTO trip_events (trip_id, seq, type, actor_kind, actor_id, payload) VALUES ($1, 1, $4, 'RIDER', $2, $3)`,
			id, in.RiderID, mustJSON(map[string]any{"service": q.ServiceKey, "vehicle_class": opt.VehicleClass, "estimated_fare": opt.Fare, "scheduled_for": in.ScheduledFor}), event); err != nil {
			return err
		}
		trip, err = s.store.get(ctx, tx, id, false)
		if err != nil {
			return err
		}
		if s.onCreated != nil {
			return s.onCreated(ctx, tx, trip)
		}
		return nil
	})
	if err == nil && trip != nil {
		s.Notify(ctx, trip.ID)
	}
	return trip, err
}

// Get returns a trip the caller participates in.
func (s *Service) Get(ctx context.Context, tripID, userID string) (*Trip, error) {
	t, err := s.store.get(ctx, s.store.pool, tripID, false)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, ErrTripNotFound
	}
	if t.RiderID != userID && (t.DriverID == nil || *t.DriverID != userID) {
		return nil, ErrNotParticipant
	}
	return s.withPrompt(ctx, t), nil
}

// Active returns the rider's in-progress trip, or nil.
func (s *Service) Active(ctx context.Context, riderID string) (*Trip, error) {
	t, err := s.store.activeForRider(ctx, riderID)
	return s.withPrompt(ctx, t), err
}

// List returns the rider's recent trips (history).
func (s *Service) List(ctx context.Context, riderID string) ([]Trip, error) {
	return s.store.listForRider(ctx, riderID, 50)
}

// ListForDriver returns the driver's recent trips (history, earnings).
func (s *Service) ListForDriver(ctx context.Context, driverID string) ([]Trip, error) {
	return s.store.listForDriver(ctx, driverID, 100)
}

// Events returns the trip's event log after a sequence number (WS replay).
func (s *Service) Events(ctx context.Context, tripID, userID string, afterSeq int) ([]Event, error) {
	if _, err := s.Get(ctx, tripID, userID); err != nil {
		return nil, err
	}
	return s.store.events(ctx, tripID, afterSeq)
}

// CancelByRider cancels while the rider still may (before the service starts).
func (s *Service) CancelByRider(ctx context.Context, tripID, riderID, reason string, note *string) (*Trip, error) {
	var out *Trip
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		t, err := s.store.get(ctx, tx, tripID, true)
		if err != nil {
			return err
		}
		if t == nil {
			return ErrTripNotFound
		}
		if t.RiderID != riderID {
			return ErrNotParticipant
		}
		if IsTerminal(t.Status) {
			return ErrInvalidTransition
		}
		if !RiderMayCancel(t.Status) {
			return ErrCancelTooLate
		}
		fee := cancelFeeAt(t, s.now())
		if err := s.store.setStatus(ctx, tx, tripID, CancelledByRider, "cancelled_at",
			`cancelled_by='RIDER', cancel_reason=$3, cancel_note=$4, cancellation_fee=NULLIF($5::numeric, 0)`, reason, note, fee); err != nil {
			return err
		}
		if _, err := s.store.appendEvent(ctx, tx, tripID, "trip.cancelled", "RIDER", &riderID, map[string]any{"status": CancelledByRider, "reason": reason, "fee": fee}); err != nil {
			return err
		}
		out, err = s.store.get(ctx, tx, tripID, false)
		return err
	})
	if err == nil && out != nil {
		s.Notify(ctx, out.ID)
	}
	return out, err
}

// SetDues connects what riders owe from earlier cancellations (the ledger);
// without it nothing is carried over.
func (s *Service) SetDues(f func(ctx context.Context, riderID string) (decimal.Decimal, error)) {
	s.dues = f
}

// addDues puts what the rider owes from earlier cancellations on this trip.
func (s *Service) addDues(ctx context.Context, tx pgx.Tx, tripID, riderID string) error {
	if s.dues == nil {
		return nil
	}
	owed, err := s.dues(ctx, riderID)
	if err != nil || !owed.IsPositive() {
		return err
	}
	adj, _ := json.Marshal(duesAdjustment(owed))
	_, err = tx.Exec(ctx, `UPDATE trips SET adjustments=$2, final_fare=estimated_fare + $3 WHERE id=$1`, tripID, adj, owed)
	return err
}

// --- helpers ---------------------------------------------------------------

// bookingTiming decides whether a booking is matched now or waits as SCHEDULED,
// and until when matching may run.
func (s *Service) bookingTiming(at *time.Time) (Status, time.Time, error) {
	now := s.now()
	if at == nil {
		return Requested, now.Add(DispatchWindow), nil
	}
	if at.Before(now.Add(ScheduleMinLead)) || at.After(now.Add(ScheduleMaxAhead)) {
		return "", time.Time{}, ErrScheduleRange.WithDetails(map[string]any{
			"min_lead_minutes": int(ScheduleMinLead.Minutes()), "max_ahead_days": int(ScheduleMaxAhead.Hours() / 24)})
	}
	return Scheduled, at.Add(DispatchWindow), nil
}

// checkContacts enforces the service's contact rules (a package needs its
// receiver) and validates every contact given.
func checkContacts(lc Lifecycle, c Contacts) error {
	if lc.DropoffContact == "required" && c.Dropoff == nil {
		return ErrContactRequired.WithDetails(map[string]any{"fields": []map[string]string{{"path": "contacts.dropoff", "code": "REQUIRED"}}})
	}
	for path, ct := range map[string]*Contact{"contacts.pickup": c.Pickup, "contacts.dropoff": c.Dropoff} {
		if ct != nil && !contactValid(*ct) {
			return apperr.Validation.WithDetails(map[string]any{"fields": []map[string]string{{"path": path, "code": "INVALID"}}})
		}
	}
	return nil
}

// contactValid: a name and a phone with enough digits.
func contactValid(ct Contact) bool {
	digits := 0
	for _, r := range ct.Phone {
		if r >= '0' && r <= '9' {
			digits++
		}
	}
	return digits >= minPhoneDigits && strings.TrimSpace(ct.Name) != ""
}

const minPhoneDigits = 9

func validateCreate(in CreateInput) error {
	var fields []map[string]string
	if in.QuoteID == "" {
		fields = append(fields, map[string]string{"path": "quote_id", "code": "REQUIRED"})
	}
	if in.VehicleClass == "" {
		fields = append(fields, map[string]string{"path": "vehicle_class", "code": "REQUIRED"})
	}
	switch in.PaymentMethod {
	case MethodCash, MethodMobileMoney:
	case "CARD", "WALLET":
		return ErrMethodUnavailable
	case "":
		fields = append(fields, map[string]string{"path": "payment_method", "code": "REQUIRED"})
	default:
		fields = append(fields, map[string]string{"path": "payment_method", "code": "INVALID"})
	}
	if len(fields) > 0 {
		return apperr.Validation.WithDetails(map[string]any{"fields": fields})
	}
	return nil
}

func pickOption(raw json.RawMessage, class string) (quoteOption, json.RawMessage, error) {
	var options []json.RawMessage
	if err := json.Unmarshal(raw, &options); err != nil {
		return quoteOption{}, nil, err
	}
	for _, o := range options {
		var opt quoteOption
		if err := json.Unmarshal(o, &opt); err != nil {
			continue
		}
		if strings.EqualFold(opt.VehicleClass, class) {
			if !opt.Available {
				return quoteOption{}, nil, ErrOptionUnavailable
			}
			return opt, o, nil
		}
	}
	return quoteOption{}, nil, ErrOptionUnavailable
}

// buildStops turns the quote's pickup, stops and drop-off into stop rows in
// order, attaching the addresses the rider supplied (stops by position).
func buildStops(q *quoteRow, inputs []StopInput) []Stop {
	addr := func(kind string, nth int) *string {
		seen := 0
		for _, s := range inputs {
			if strings.EqualFold(s.Kind, kind) {
				if seen == nth {
					return s.Address
				}
				seen++
			}
		}
		return nil
	}
	stops := []Stop{{Seq: 1, Kind: "PICKUP", Location: routing.Point{Lat: q.Pickup[0], Lng: q.Pickup[1]}, Address: addr("PICKUP", 0)}}
	for i, w := range q.Waypoints {
		stops = append(stops, Stop{Seq: len(stops) + 1, Kind: "WAYPOINT", Location: w, Address: addr("WAYPOINT", i)})
	}
	if q.Dropoff != nil {
		stops = append(stops, Stop{Seq: len(stops) + 1, Kind: "DROPOFF", Location: routing.Point{Lat: q.Dropoff[0], Lng: q.Dropoff[1]}, Address: addr("DROPOFF", 0)})
	}
	return stops
}

func nonZero(v int) *int {
	if v == 0 {
		return nil
	}
	return &v
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
