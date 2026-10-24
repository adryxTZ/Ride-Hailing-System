package fraud

import (
	"math"
	"time"
)

// Ping is one driver position as the fraud check sees it.
type Ping struct {
	Lat    float64   `json:"lat"`
	Lng    float64   `json:"lng"`
	At     time.Time `json:"at"`
	Mocked bool      `json:"mocked,omitempty"`
}

// Strike is the verdict on one ping.
type Strike struct {
	Hit      bool
	Mocked   bool    // the phone said the location is mocked: flag at once
	SpeedMps float64 // the speed implied by the move (0 when not computed)
	JumpM    float64
}

// CheckPing compares a ping with the previous one (nil for the first). The
// phone's mock flag is always a strike; otherwise only a move of at least
// MinJumpM, at least MinGapS apart, faster than MaxPlausibleSpeedMps strikes.
func CheckPing(prev *Ping, cur Ping) Strike {
	if cur.Mocked {
		return Strike{Hit: true, Mocked: true}
	}
	if prev == nil {
		return Strike{}
	}
	gap := cur.At.Sub(prev.At).Seconds()
	jump := haversineM(prev.Lat, prev.Lng, cur.Lat, cur.Lng)
	if gap < MinGapS || jump < MinJumpM {
		return Strike{JumpM: jump}
	}
	speed := jump / gap
	return Strike{Hit: speed > MaxPlausibleSpeedMps, SpeedMps: speed, JumpM: jump}
}

const earthRadiusM = 6_371_000.0

func haversineM(lat1, lng1, lat2, lng2 float64) float64 {
	rad := math.Pi / 180
	dLat, dLng := (lat2-lat1)*rad, (lng2-lng1)*rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * earthRadiusM * math.Asin(math.Sqrt(a))
}
