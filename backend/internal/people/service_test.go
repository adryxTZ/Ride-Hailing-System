package people

import (
	"context"
	"errors"
	"testing"

	"github.com/kalen-limited/hauler-rider/backend/internal/trips"
)

// fakeTrips answers Countries only; the scope check needs nothing else.
type fakeTrips struct {
	Trips
	countries []string
}

func (f fakeTrips) Countries(context.Context, string, string) ([]string, error) {
	return f.countries, nil
}

func only(code string) Allowed { return func(c string) bool { return c == code } }

func TestCheckScope(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name      string
		role      string
		home      string
		countries []string
		allowed   Allowed
		want      error
	}{
		{"unlimited staff see everyone", trips.AsRider, "", nil, nil, nil},
		{"rider with trips in the staff country", trips.AsRider, "", []string{"KE", "TZ"}, only("TZ"), nil},
		{"rider only elsewhere is hidden", trips.AsRider, "", []string{"KE"}, only("TZ"), ErrOutOfScope},
		{"rider with no trips is hidden from limited staff", trips.AsRider, "", nil, only("TZ"), ErrOutOfScope},
		{"driver registered in the staff country", trips.AsDriver, "TZ", nil, only("TZ"), nil},
		{"driver registered elsewhere but drove here", trips.AsDriver, "KE", []string{"TZ"}, only("TZ"), nil},
		{"driver registered and driving elsewhere", trips.AsDriver, "KE", []string{"KE"}, only("TZ"), ErrOutOfScope},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := New(Deps{
				Trips:         fakeTrips{countries: c.countries},
				DriverCountry: func(context.Context, string) (string, error) { return c.home, nil },
			})
			if err := s.Check(ctx, c.role, "id", c.allowed); !errors.Is(err, c.want) {
				t.Fatalf("Check = %v, want %v", err, c.want)
			}
		})
	}
}
