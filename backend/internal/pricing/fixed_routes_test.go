package pricing

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
)

func TestFixedRouteMatching(t *testing.T) {
	kariakoo := routing.Point{Lat: -6.8181, Lng: 39.2745}
	goba := routing.Point{Lat: -6.7420, Lng: 39.1690}
	routes := []FixedRoute{{ServiceKey: "taxi", Name: "K-G", Origin: kariakoo, OriginRadiusM: 1500,
		Destination: goba, DestinationRadiusM: 2000, Bidirectional: true, VehicleClasses: []string{"standard"},
		Price: decimal.NewFromInt(60000)}}
	now := time.Now()
	near := routing.Point{Lat: -6.8190, Lng: 39.2760} // ~200 m from Kariakoo
	if r, dir := matchFixedRoute(routes, "taxi", near, goba, "standard", now); r == nil || dir != Forward {
		t.Fatal("forward direction should match")
	}
	if r, dir := matchFixedRoute(routes, "taxi", goba, near, "standard", now); r == nil || dir != Reverse {
		t.Fatal("reverse direction should match when bidirectional")
	}
	if r, _ := matchFixedRoute(routes, "taxi", near, goba, "tricycle_general", now); r != nil {
		t.Fatal("class not in the route's list must not match")
	}
	far := routing.Point{Lat: -6.90, Lng: 39.30}
	if r, _ := matchFixedRoute(routes, "taxi", far, goba, "standard", now); r != nil {
		t.Fatal("pickup outside the origin radius must not match")
	}
	if r, _ := matchFixedRoute(routes, "other_service", near, goba, "standard", now); r != nil {
		t.Fatal("other service must not match")
	}
}

func TestAsymmetricPrices(t *testing.T) {
	rev := decimal.NewFromInt(2000000)
	r := FixedRoute{Price: decimal.NewFromInt(3000000), ReversePrice: &rev}
	if r.PriceFor(Forward).String() != "3000000" || r.PriceFor(Reverse).String() != "2000000" {
		t.Fatal("direction-specific prices")
	}
	r.ReversePrice = nil
	if r.PriceFor(Reverse).String() != "3000000" {
		t.Fatal("reverse falls back to the forward price")
	}
}

func TestRoundedFare(t *testing.T) {
	o := rounded(Option{Fare: decimal.RequireFromString("6411.23"), Available: true}, decimal.NewFromInt(50))
	if !o.Fare.Equal(decimal.NewFromInt(6400)) || len(o.Lines) != 1 || !o.Lines[0].Amount.Equal(decimal.RequireFromString("-11.23")) {
		t.Fatalf("got %s with %v", o.Fare, o.Lines)
	}
	if o := rounded(Option{Fare: decimal.NewFromInt(4000), Available: true}, decimal.NewFromInt(50)); len(o.Lines) != 0 {
		t.Fatal("an already round fare gets no line")
	}
}
