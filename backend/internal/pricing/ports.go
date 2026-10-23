package pricing

import (
	"context"

	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
)

// AreaRef names the operating area a point falls in.
type AreaRef struct {
	ID      string `json:"id"`
	Code    string `json:"code"`
	Name    string `json:"name"`
	Country string `json:"country"`
}

// AreaRule is what pricing needs to know about an area.
type AreaRule struct {
	Ref          AreaRef
	SurgeEnabled bool
	SurgeMax     *decimal.Decimal // nil = no area cap (tariff caps still apply)
}

// AreaResolver finds the serviceable area of a point (OUTSIDE_SERVICE_AREA when none)
// and the area, if any, a point falls in whatever its status.
type AreaResolver interface {
	Serviceable(ctx context.Context, p routing.Point) (AreaRule, error)
	AreaOf(ctx context.Context, p routing.Point) (AreaRef, bool, error)
}
