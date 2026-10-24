package presence

import (
	"context"
	"strconv"

	"github.com/redis/go-redis/v9"

	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
)

// Trip queue fields of the presence hash. Dispatch reads them while matching,
// so a finishing driver is found without touching Postgres:
//
//	finishing  "1" while the current trip is in a finishing status
//	drop_lat   where the current trip ends (distance to the next pickup is
//	drop_lng   measured from here)
//	queued     the next trip offered to or accepted by the driver meanwhile
const (
	fieldFinishing = "finishing"
	fieldDropLat   = "drop_lat"
	fieldDropLng   = "drop_lng"
	fieldQueued    = "queued"
	flagOn         = "1"
)

// OfflineHook sees a driver's last presence record when they go offline (or
// are swept as stale): dispatch releases their queued trip back to matching.
type OfflineHook func(ctx context.Context, d Driver)

// OnOffline adds a hook run after a driver's record is removed.
func (s *Service) OnOffline(h OfflineHook) { s.onOffline = append(s.onOffline, h) }

// OnlineHook runs after a driver goes online, before the record is returned:
// dispatch puts back a trip the driver is still on (going online starts IDLE).
type OnlineHook func(ctx context.Context, driverID string)

// OnOnline adds a hook run when a driver goes online.
func (s *Service) OnOnline(h OnlineHook) { s.onOnline = append(s.onOnline, h) }

// clearedQueueFields resets the queue fields (going online, a new state).
func clearedQueueFields() map[string]any {
	return map[string]any{fieldFinishing: "", fieldDropLat: "", fieldDropLng: "", fieldQueued: ""}
}

// queueFromHash fills the queue fields of a record read from Redis.
func queueFromHash(d *Driver, m map[string]string) {
	d.Finishing = m[fieldFinishing] == flagOn
	d.QueuedTripID = m[fieldQueued]
	lat, errLat := strconv.ParseFloat(m[fieldDropLat], 64)
	lng, errLng := strconv.ParseFloat(m[fieldDropLng], 64)
	if errLat == nil && errLng == nil {
		d.Dropoff = &routing.Point{Lat: lat, Lng: lng}
	}
}

// The scripts below compare and set in one step, so a late answer about an old
// trip never overwrites the driver's newer state.

// setFinishingScript: KEYS[1] record; ARGV trip, lat, lng.
var setFinishingScript = redis.NewScript(`
if redis.call('HGET', KEYS[1], 'trip') ~= ARGV[1] or redis.call('HGET', KEYS[1], 'state') ~= 'ON_TRIP' then return 0 end
redis.call('HSET', KEYS[1], 'finishing', '1', 'drop_lat', ARGV[2], 'drop_lng', ARGV[3])
return 1`)

// setQueuedScript: KEYS[1] record; ARGV trip. Only one queued trip at a time.
var setQueuedScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 0 then return 0 end
local q = redis.call('HGET', KEYS[1], 'queued')
if q and q ~= '' and q ~= ARGV[1] then return 0 end
redis.call('HSET', KEYS[1], 'queued', ARGV[1])
return 1`)

// releaseOfferScript: KEYS[1] record; ARGV trip. A queued trip is dropped from
// the queue; a plain offer returns the driver to IDLE; anything else is left.
var releaseOfferScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 0 then return 0 end
if redis.call('HGET', KEYS[1], 'queued') == ARGV[1] then
  redis.call('HSET', KEYS[1], 'queued', '')
  return 1
end
if redis.call('HGET', KEYS[1], 'trip') == ARGV[1] and redis.call('HGET', KEYS[1], 'state') == 'OFFERED' then
  redis.call('HSET', KEYS[1], 'state', 'IDLE', 'trip', '', 'rider', '', 'finishing', '', 'drop_lat', '', 'drop_lng', '', 'queued', '')
  return 2
end
return 0`)

// promoteScript: KEYS[1] record; ARGV from-trip, to-trip, rider, state.
var promoteScript = redis.NewScript(`
if redis.call('HGET', KEYS[1], 'trip') ~= ARGV[1] then return 0 end
redis.call('HSET', KEYS[1], 'state', ARGV[4], 'trip', ARGV[2], 'rider', ARGV[3], 'finishing', '', 'drop_lat', '', 'drop_lng', '', 'queued', '')
return 1`)

// SetFinishing marks the driver's current trip as finishing at drop. Ignored
// when the record moved on to another trip or the driver is offline.
func (s *Service) SetFinishing(ctx context.Context, driverID, tripID string, drop routing.Point) error {
	return setFinishingScript.Run(ctx, s.r, []string{s.key(driverID)}, tripID,
		strconv.FormatFloat(drop.Lat, 'f', -1, 64), strconv.FormatFloat(drop.Lng, 'f', -1, 64)).Err()
}

// SetQueued records the next trip offered to (or accepted by) a finishing
// driver. False when another trip is already queued or the driver is offline.
func (s *Service) SetQueued(ctx context.Context, driverID, tripID string) (bool, error) {
	n, err := setQueuedScript.Run(ctx, s.r, []string{s.key(driverID)}, tripID).Int()
	return n == 1, err
}

// ReleaseOffer frees the driver from an offer or queued trip that ended
// (declined, expired, cancelled): a queued trip leaves the queue and the
// current trip is untouched; a plain offer returns the driver to IDLE.
func (s *Service) ReleaseOffer(ctx context.Context, driverID, tripID string) error {
	return releaseOfferScript.Run(ctx, s.r, []string{s.key(driverID)}, tripID).Err()
}

// Promote moves the driver from the trip that ended to the next one (ON_TRIP
// for an accepted trip, OFFERED for an offer still open), clearing the queue.
// Ignored when the record no longer points at fromTrip.
func (s *Service) Promote(ctx context.Context, driverID, fromTrip, toTrip, riderID string, st State) error {
	return promoteScript.Run(ctx, s.r, []string{s.key(driverID)}, fromTrip, toTrip, riderID, string(st)).Err()
}
