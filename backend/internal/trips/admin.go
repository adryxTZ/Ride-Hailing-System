package trips

import (
	"context"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/scope"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/audit"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// CancelledByStaff ends a trip from the operations desk.
const CancelledByStaff Status = "CANCELLED_BY_STAFF"

// Admin list bounds.
const (
	AdminPageSize    = 50
	AdminMaxPageSize = 200
	adminEventsLimit = 200
)

// statusGroups are the filters the admin offers.
var statusGroups = map[string][]string{
	"active":    {"REQUESTED", "OFFERED", "ACCEPTED", "EN_ROUTE_TO_PICKUP", "ARRIVED_AT_PICKUP", "IN_TRANSIT", "ARRIVED_AT_DROPOFF", "AWAITING_PAYMENT"},
	"scheduled": {"SCHEDULED"},
	"completed": {"COMPLETED"},
	"cancelled": {"CANCELLED_BY_RIDER", "CANCELLED_BY_DRIVER", "CANCELLED_BY_STAFF", "NO_DRIVER_FOUND", "EXPIRED"},
}

// AdminFilter narrows the staff trip list.
type AdminFilter struct {
	Scope   scope.Scope
	Group   string // active | scheduled | completed | cancelled | "" (all)
	Q       string // trip code, rider/driver name or phone, stop address
	Service string
	// RiderID / DriverID narrow the list to one person's trips (their page).
	RiderID  string
	DriverID string
	From     *time.Time
	To       *time.Time
	Limit    int
	Offset   int
}

// TripPage is one page of the staff trip list.
type TripPage struct {
	Data  []Trip `json:"data"`
	Total int    `json:"total"`
}

// TripDetail is everything staff see about one trip.
type TripDetail struct {
	Trip
	Events []Event `json:"events"`
}

// Stats are the operations numbers since a moment (usually today's midnight).
type Stats struct {
	Since          time.Time       `json:"since"`
	Requested      int             `json:"requested"`
	Completed      int             `json:"completed"`
	Cancelled      int             `json:"cancelled"`
	NoDriver       int             `json:"no_driver"`
	Active         int             `json:"active"`
	Scheduled      int             `json:"scheduled"`
	GrossFares     decimal.Decimal `json:"gross_fares"`
	AvgFare        decimal.Decimal `json:"avg_fare"`
	CompletionRate float64         `json:"completion_rate"` // completed ÷ (completed + cancelled + no driver)
	ByHour         []HourCount     `json:"by_hour"`
	Previous       PeriodTotals    `json:"previous"` // the same window one day earlier, for "vs yesterday"
}

// PeriodTotals is the headline counts for one time window.
type PeriodTotals struct {
	Requested  int             `json:"requested"`
	Completed  int             `json:"completed"`
	GrossFares decimal.Decimal `json:"gross_fares"`
}

// comparisonOffset shifts the stats window back to compare with the same hours yesterday.
const comparisonOffset = 24 * time.Hour

// HourCount is trips requested in one hour.
type HourCount struct {
	Hour      time.Time `json:"hour"`
	Requested int       `json:"requested"`
	Completed int       `json:"completed"`
}

// AdminList is the staff trip list, newest first.
func (s *Service) AdminList(ctx context.Context, f AdminFilter) (*TripPage, error) {
	if f.Limit <= 0 || f.Limit > AdminMaxPageSize {
		f.Limit = AdminPageSize
	}
	if !validIDs(f.RiderID, f.DriverID) {
		return &TripPage{Data: []Trip{}}, nil
	}
	where, args := adminWhere(f, s.store.brandID)
	var total int
	if err := s.store.pool.QueryRow(ctx, `SELECT count(*)`+tripFrom+`WHERE `+where, args...).Scan(&total); err != nil {
		return nil, err
	}
	args = append(args, f.Limit, max(0, f.Offset))
	rows, err := s.store.pool.Query(ctx, `SELECT `+tripColumns+tripFrom+`WHERE `+where+
		` ORDER BY t.created_at DESC LIMIT $`+itoa(len(args)-1)+` OFFSET $`+itoa(len(args)), args...)
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
	if len(ids) > 0 {
		if err := s.store.attachStops(ctx, out, index, ids); err != nil {
			return nil, err
		}
	}
	return &TripPage{Data: out, Total: total}, nil
}

// validIDs reports whether each non-empty person filter is a UUID (anything
// else matches no trip, rather than failing the query).
func validIDs(ids ...string) bool {
	for _, id := range ids {
		if _, err := uuid.Parse(id); id != "" && err != nil {
			return false
		}
	}
	return true
}

// adminWhere builds the filter; args[0] is always the brand.
func adminWhere(f AdminFilter, brandID string) (string, []any) {
	conds := []string{"t.brand_id=$1"}
	args := []any{brandID}
	add := func(cond string, v any) {
		args = append(args, v)
		conds = append(conds, strings.ReplaceAll(cond, "?", "$"+itoa(len(args))))
	}
	if g, ok := statusGroups[f.Group]; ok {
		add("t.status::text = ANY(?)", g)
	}
	if q := strings.TrimSpace(f.Q); q != "" {
		add("(t.code ILIKE '%'||?||'%' OR ru.full_name ILIKE '%'||?||'%' OR ru.phone LIKE '%'||?||'%' OR du.full_name ILIKE '%'||?||'%' OR du.phone LIKE '%'||?||'%'"+
			" OR EXISTS (SELECT 1 FROM trip_stops ts WHERE ts.trip_id = t.id AND ts.address ILIKE '%'||?||'%'))", q)
	}
	if f.Service != "" {
		add("t.service_key = ?", f.Service)
	}
	if f.RiderID != "" {
		add("t.rider_id = ?::uuid", f.RiderID)
	}
	if f.DriverID != "" {
		add("t.driver_id = ?::uuid", f.DriverID)
	}
	if f.Scope.Country != "" {
		add("t.country_code = ?", f.Scope.Country)
	}
	if f.Scope.AreaID != "" {
		add("t.area_id = ?::uuid", f.Scope.AreaID)
	}
	if f.From != nil {
		add("t.created_at >= ?", *f.From)
	}
	if f.To != nil {
		add("t.created_at < ?", *f.To)
	}
	return strings.Join(conds, " AND "), args
}

// AdminGet returns a trip with its full event log, for staff.
func (s *Service) AdminGet(ctx context.Context, id string) (*TripDetail, error) {
	t, err := s.store.get(ctx, s.store.pool, id, false)
	if err != nil {
		return nil, err
	}
	if t == nil || !strings.EqualFold(t.ID, id) {
		return nil, ErrTripNotFound
	}
	ev, err := s.store.events(ctx, id, 0)
	if err != nil {
		return nil, err
	}
	if len(ev) > adminEventsLimit {
		ev = ev[:adminEventsLimit]
	}
	return &TripDetail{Trip: *t, Events: ev}, nil
}

// AdminCancel ends a trip from the operations desk (any status before it ends), audited.
func (s *Service) AdminCancel(ctx context.Context, staffID, id, reason string) (*Trip, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, apperr.Validation.WithDetails(map[string]any{"fields": []map[string]string{{"path": "reason", "code": "REQUIRED"}}})
	}
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		t, err := s.store.get(ctx, tx, id, true)
		if err != nil {
			return err
		}
		if t == nil {
			return ErrTripNotFound
		}
		if IsTerminal(t.Status) {
			return ErrInvalidTransition
		}
		if err := s.store.setStatus(ctx, tx, id, CancelledByStaff, "cancelled_at", `cancelled_by='STAFF', cancel_reason=$3`, reason); err != nil {
			return err
		}
		if _, err := s.store.appendEvent(ctx, tx, id, "trip.cancelled_by_staff", "STAFF", &staffID, map[string]any{"status": CancelledByStaff, "reason": reason}); err != nil {
			return err
		}
		return audit.Record(ctx, tx, s.store.brandID, staffID, "trip.cancel", "trip:"+id, reason, map[string]any{"from": t.Status})
	})
	if err != nil {
		return nil, err
	}
	s.Notify(ctx, id)
	return s.store.get(ctx, s.store.pool, id, false)
}

// windowTotals fills the outcome counts and fares for trips requested in [from, to).
func (s *Service) windowTotals(ctx context.Context, from, to time.Time, sc scope.Scope, st *Stats) error {
	return s.store.pool.QueryRow(ctx, `
		SELECT count(*),
		  count(*) FILTER (WHERE status='COMPLETED'),
		  count(*) FILTER (WHERE status IN ('CANCELLED_BY_RIDER','CANCELLED_BY_DRIVER','CANCELLED_BY_STAFF')),
		  count(*) FILTER (WHERE status IN ('NO_DRIVER_FOUND','EXPIRED')),
		  coalesce(sum(coalesce(final_fare, estimated_fare)) FILTER (WHERE status='COMPLETED'), 0),
		  coalesce(avg(coalesce(final_fare, estimated_fare)) FILTER (WHERE status='COMPLETED'), 0)
		FROM trips WHERE brand_id=$1 AND requested_at >= $2 AND requested_at < $3 AND status <> 'SCHEDULED'
		  AND ($4 = '' OR country_code = $4) AND ($5 = '' OR area_id = NULLIF($5,'')::uuid)`, s.store.brandID, from, to, sc.Country, sc.AreaID).
		Scan(&st.Requested, &st.Completed, &st.Cancelled, &st.NoDriver, &st.GrossFares, &st.AvgFare)
}

// Stats counts today's trips by outcome, the fares and the hourly curve.
func (s *Service) Stats(ctx context.Context, since time.Time, sc scope.Scope) (*Stats, error) {
	st := &Stats{Since: since, ByHour: []HourCount{}}
	now := time.Now()
	if err := s.windowTotals(ctx, since, now, sc, st); err != nil {
		return nil, err
	}
	var prev Stats
	if err := s.windowTotals(ctx, since.Add(-comparisonOffset), now.Add(-comparisonOffset), sc, &prev); err != nil {
		return nil, err
	}
	st.Previous = PeriodTotals{Requested: prev.Requested, Completed: prev.Completed, GrossFares: prev.GrossFares}
	err := s.store.pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE status::text = ANY($2)), count(*) FILTER (WHERE status='SCHEDULED')
		FROM trips WHERE brand_id=$1 AND ($3 = '' OR country_code = $3) AND ($4 = '' OR area_id = NULLIF($4,'')::uuid)`, s.store.brandID, statusGroups["active"], sc.Country, sc.AreaID).Scan(&st.Active, &st.Scheduled)
	if err != nil {
		return nil, err
	}
	if ended := st.Completed + st.Cancelled + st.NoDriver; ended > 0 {
		st.CompletionRate = float64(st.Completed) / float64(ended)
	}
	st.AvgFare = st.AvgFare.Round(0)
	rows, err := s.store.pool.Query(ctx, `
		SELECT date_trunc('hour', requested_at) h, count(*), count(*) FILTER (WHERE status='COMPLETED')
		FROM trips WHERE brand_id=$1 AND requested_at >= $2 AND status <> 'SCHEDULED' AND ($3 = '' OR country_code = $3) AND ($4 = '' OR area_id = NULLIF($4,'')::uuid) GROUP BY h ORDER BY h`, s.store.brandID, since, sc.Country, sc.AreaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var h HourCount
		if err := rows.Scan(&h.Hour, &h.Requested, &h.Completed); err != nil {
			return nil, err
		}
		st.ByHour = append(st.ByHour, h)
	}
	return st, rows.Err()
}

func itoa(n int) string {
	const digits = "0123456789"
	if n < 10 {
		return digits[n : n+1]
	}
	return itoa(n/10) + digits[n%10:n%10+1]
}
