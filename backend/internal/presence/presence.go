package presence

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/uber/h3-go/v4"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
)

const (
	Resolution   = 9 // H3 cell kept on each driver (analytics, surge, live map)
	Staleness    = 120 * time.Second
	ringSpacingM = 350.0 // distance per res-9 ring; callers still speak in rings
	// overfetch: candidates read per wanted result, to survive class/state filters.
	overfetch = 3
)

type State string

const (
	Idle    State = "IDLE"
	Offered State = "OFFERED"
	OnTrip  State = "ON_TRIP"
)

// Driver is a presence record as read back from Redis.
type Driver struct {
	ID        string        `json:"driver_id"`
	Location  routing.Point `json:"location"`
	Cell      string        `json:"h3"`
	Heading   float64       `json:"heading"`
	SpeedMps  float64       `json:"speed_mps"`
	State     State         `json:"state"`
	Services  []string      `json:"services"`
	Classes   []string      `json:"vehicle_classes"`
	VehicleID string        `json:"vehicle_id,omitempty"`
	Areas     []string      `json:"areas,omitempty"`
	TripID    string        `json:"trip_id,omitempty"`
	RiderID   string        `json:"rider_id,omitempty"`
	At        time.Time     `json:"at"`
}

// Capabilities declared when going online.
type Capabilities struct {
	Services  []string
	Classes   []string
	VehicleID string
	Areas     []string // operating area ids the driver may take trips in
}

type Ping struct {
	Location routing.Point
	Heading  float64
	SpeedMps float64
	At       time.Time
	Mocked   bool // the phone reports the location as mocked (Android)
}

// PingHook sees each accepted ping with the driver's previous position (nil
// when going online): the fraud check. It must be fast and never fail the ping.
type PingHook func(ctx context.Context, driverID string, prev *Ping, cur Ping)

var ErrNotOnline = apperr.New("DRIVER_NOT_ONLINE", http.StatusConflict, "")

type Service struct {
	r      *redis.Client
	brand  string
	now    func() time.Time
	onPing PingHook
}

// OnPing sets the hook every accepted ping goes through.
func (s *Service) OnPing(h PingHook) { s.onPing = h }

func New(r *redis.Client, brand string) *Service { return &Service{r: r, brand: brand, now: time.Now} }

func (s *Service) key(driverID string) string   { return "presence:" + s.brand + ":" + driverID }
func (s *Service) geoKey(service string) string { return "geo:" + s.brand + ":" + service }
func (s *Service) seenKey() string              { return "seen:" + s.brand } // zset driver → last ping

// Online creates the presence record (fresh immediately) and indexes the cell.
func (s *Service) Online(ctx context.Context, driverID string, p Ping, caps Capabilities) (*Driver, error) {
	d, err := s.write(ctx, driverID, p, &caps, nil)
	if err == nil && s.onPing != nil {
		s.onPing(ctx, driverID, nil, s.stamped(p))
	}
	return d, err
}

// Update refreshes position and freshness; requires an existing record.
// The previous position is read in the same call that checks the record exists.
func (s *Service) Update(ctx context.Context, driverID string, p Ping) (*Driver, error) {
	vals, err := s.r.HMGet(ctx, s.key(driverID), "lat", "lng", "at").Result()
	if err != nil {
		return nil, err
	}
	prev, ok := previous(vals)
	if !ok {
		return nil, ErrNotOnline
	}
	d, err := s.write(ctx, driverID, p, nil, nil)
	if err == nil && s.onPing != nil {
		s.onPing(ctx, driverID, prev, s.stamped(p))
	}
	return d, err
}

// stamped fills a ping's time with now when the client sent none.
func (s *Service) stamped(p Ping) Ping {
	if p.At.IsZero() {
		p.At = s.now()
	}
	return p
}

// previous turns HMGET lat, lng, at into a ping; ok is false when the record is missing.
func previous(vals []any) (*Ping, bool) {
	if len(vals) != 3 || vals[0] == nil || vals[1] == nil || vals[2] == nil {
		return nil, false
	}
	num := func(v any) float64 { s, _ := v.(string); f, _ := strconv.ParseFloat(s, 64); return f }
	return &Ping{Location: routing.Point{Lat: num(vals[0]), Lng: num(vals[1])}, At: time.Unix(int64(num(vals[2])), 0)}, true
}

// SetState moves a driver between IDLE / OFFERED / ON_TRIP (dispatch calls this).
// tripID and riderID are kept while OFFERED/ON_TRIP so GPS pings can be relayed
// to the rider straight from Redis. A missing record (driver offline) is left alone.
func (s *Service) SetState(ctx context.Context, driverID string, st State, tripID, riderID string) error {
	n, err := s.r.Exists(ctx, s.key(driverID)).Result()
	if err != nil || n == 0 {
		return err
	}
	return s.r.HSet(ctx, s.key(driverID), map[string]any{"state": string(st), "trip": tripID, "rider": riderID}).Err()
}

// Offline removes the driver from presence and its cell.
func (s *Service) Offline(ctx context.Context, driverID string) error {
	services, _ := s.r.HGet(ctx, s.key(driverID), "services").Result()
	pipe := s.r.TxPipeline()
	pipe.Del(ctx, s.key(driverID))
	pipe.ZRem(ctx, s.seenKey(), driverID)
	for _, svc := range split(services) {
		pipe.ZRem(ctx, s.geoKey(svc), driverID)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// Get returns the record, or nil when the driver is not online.
func (s *Service) Get(ctx context.Context, driverID string) (*Driver, error) {
	m, err := s.r.HGetAll(ctx, s.key(driverID)).Result()
	if err != nil {
		return nil, err
	}
	if len(m) == 0 {
		return nil, nil
	}
	return fromHash(driverID, m), nil
}

// Nearby returns fresh drivers offering `service` within k res-9 rings (≈ k ×
// 350 m) of a point, nearest first, optionally of one vehicle class. One
// GEOSEARCH on the service's geo index (Redis does the radius and the sort)
// plus one pipelined read of the few nearest records — cost grows with the
// results asked for, not with how many drivers are online.
func (s *Service) Nearby(ctx context.Context, at routing.Point, k int, service, class, areaID string, limit int) ([]Driver, error) {
	if service == "" {
		return []Driver{}, nil // every caller searches within a service
	}
	if limit <= 0 {
		limit = 50
	}
	ids, err := s.r.GeoSearch(ctx, s.geoKey(service), &redis.GeoSearchQuery{
		Longitude: at.Lng, Latitude: at.Lat, Radius: float64(k) * ringSpacingM, RadiusUnit: "m",
		Sort: "ASC", Count: limit * overfetch,
	}).Result()
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []Driver{}, nil
	}
	out, err := s.load(ctx, ids, func(d *Driver) bool {
		return s.now().Sub(d.At) <= Staleness && (class == "" || contains(d.Classes, class)) && (areaID == "" || len(d.Areas) == 0 || contains(d.Areas, areaID))
	})
	if err != nil {
		return nil, err
	}
	sortByDistance(out, at)
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// load reads driver hashes in one pipeline and keeps those passing keep.
func (s *Service) load(ctx context.Context, ids []string, keep func(*Driver) bool) ([]Driver, error) {
	pipe := s.r.Pipeline()
	cmds := make([]*redis.MapStringStringCmd, len(ids))
	for i, id := range ids {
		cmds[i] = pipe.HGetAll(ctx, s.key(id))
	}
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, err
	}
	out := []Driver{}
	for i, id := range ids {
		if m := cmds[i].Val(); len(m) > 0 {
			if d := fromHash(id, m); keep(d) {
				out = append(out, *d)
			}
		}
	}
	return out, nil
}

// GC removes drivers whose last ping is older than the staleness window from
// every index (the dispatcher runs it every GCInterval). Searches already skip
// them; this keeps the indexes small.
func (s *Service) GC(ctx context.Context) (int, error) {
	maxScore := strconv.FormatInt(s.now().Add(-Staleness).Unix(), 10)
	stale, err := s.r.ZRangeByScore(ctx, s.seenKey(), &redis.ZRangeBy{Min: "-inf", Max: maxScore}).Result()
	if err != nil || len(stale) == 0 {
		return 0, err
	}
	for _, id := range stale {
		if err := s.Offline(ctx, id); err != nil {
			return 0, err
		}
	}
	return len(stale), nil
}

// GCInterval is how often stale index entries are swept.
const GCInterval = 30 * time.Second

func (s *Service) write(ctx context.Context, driverID string, p Ping, caps *Capabilities, _ *State) (*Driver, error) {
	if p.At.IsZero() {
		p.At = s.now()
	}
	c, err := h3.LatLngToCell(h3.LatLng{Lat: p.Location.Lat, Lng: p.Location.Lng}, Resolution)
	if err != nil {
		return nil, fmt.Errorf("h3 cell: %w", err)
	}
	cell := c.String()
	services := ""
	if caps != nil {
		services = strings.Join(caps.Services, ",")
	} else {
		services, _ = s.r.HGet(ctx, s.key(driverID), "services").Result()
	}
	fields := map[string]any{
		"lat": p.Location.Lat, "lng": p.Location.Lng, "h3": cell, "heading": p.Heading, "speed": p.SpeedMps, "at": p.At.Unix(),
	}
	if caps != nil {
		fields["services"] = strings.Join(caps.Services, ",")
		fields["classes"] = strings.Join(caps.Classes, ",")
		fields["vehicle"] = caps.VehicleID
		fields["areas"] = strings.Join(caps.Areas, ",")
		fields["state"] = string(Idle)
		fields["trip"] = ""
		fields["rider"] = ""
	}
	pipe := s.r.TxPipeline()
	pipe.HSet(ctx, s.key(driverID), fields)
	pipe.Expire(ctx, s.key(driverID), 2*Staleness)
	for _, svc := range split(services) {
		pipe.GeoAdd(ctx, s.geoKey(svc), &redis.GeoLocation{Name: driverID, Longitude: p.Location.Lng, Latitude: p.Location.Lat})
	}
	pipe.ZAdd(ctx, s.seenKey(), redis.Z{Score: float64(p.At.Unix()), Member: driverID})
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, err
	}
	return s.Get(ctx, driverID)
}

func fromHash(id string, m map[string]string) *Driver {
	f := func(k string) float64 { v, _ := strconv.ParseFloat(m[k], 64); return v }
	at, _ := strconv.ParseInt(m["at"], 10, 64)
	d := &Driver{ID: id, Location: routing.Point{Lat: f("lat"), Lng: f("lng")}, Cell: m["h3"], Heading: f("heading"), SpeedMps: f("speed"),
		State: State(m["state"]), VehicleID: m["vehicle"], TripID: m["trip"], RiderID: m["rider"], At: time.Unix(at, 0)}
	d.Services = split(m["services"])
	d.Classes = split(m["classes"])
	d.Areas = split(m["areas"])
	return d
}

func split(s string) []string {
	if s == "" {
		return []string{}
	}
	return strings.Split(s, ",")
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func sortByDistance(list []Driver, at routing.Point) {
	for i := 1; i < len(list); i++ { // small lists; insertion sort keeps it dependency-free
		for j := i; j > 0 && routing.HaversineM(at, list[j].Location) < routing.HaversineM(at, list[j-1].Location); j-- {
			list[j], list[j-1] = list[j-1], list[j]
		}
	}
}

// String helper for logs.
func (d Driver) String() string {
	return fmt.Sprintf("%s@%s %s", d.ID, d.Cell, d.State)
}

// Fresh lists every driver seen within the staleness window (the live map),
// most recently seen first, up to limit. One ZRANGE plus one pipelined read.
func (s *Service) Fresh(ctx context.Context, limit int) ([]Driver, error) {
	minScore := strconv.FormatInt(s.now().Add(-Staleness).Unix(), 10)
	ids, err := s.r.ZRevRangeByScore(ctx, s.seenKey(), &redis.ZRangeBy{Min: minScore, Max: "+inf", Count: int64(limit)}).Result()
	if err != nil || len(ids) == 0 {
		return []Driver{}, err
	}
	return s.load(ctx, ids, func(*Driver) bool { return true })
}
