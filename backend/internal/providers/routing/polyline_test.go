package routing

import (
	"math"
	"testing"
)

func TestDecodePolyline(t *testing.T) {
	// Google's documented example.
	got := DecodePolyline("_p~iF~ps|U_ulLnnqC_mqNvxq`@")
	want := []Point{{38.5, -120.2}, {40.7, -120.95}, {43.252, -126.453}}
	if len(got) != len(want) {
		t.Fatalf("decoded %d points", len(got))
	}
	for i := range want {
		if math.Abs(got[i].Lat-want[i].Lat) > 1e-9 || math.Abs(got[i].Lng-want[i].Lng) > 1e-9 {
			t.Fatalf("point %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}
