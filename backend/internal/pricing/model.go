package pricing

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
	"github.com/kalen-limited/hauler-rider/backend/internal/pricing/engine"
	"github.com/kalen-limited/hauler-rider/backend/internal/profiles"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
)

// QuoteTTL is how long a quote can be booked.
const QuoteTTL = 3 * time.Minute

// Request is the quote input from the rider.
type Request struct {
	ServiceKey string          `json:"service"`
	Pickup     routing.Point   `json:"pickup"`
	Dropoff    *routing.Point  `json:"dropoff,omitempty"`
	Waypoints  []routing.Point `json:"waypoints,omitempty"` // stops between pickup and drop-off, in order
	Attributes json.RawMessage `json:"attributes"`
	UserID     string          `json:"-"`
}

// Option is the price for one vehicle class.
type Option struct {
	VehicleClass string          `json:"vehicle_class"`
	Fare         decimal.Decimal `json:"fare"`
	Currency     string          `json:"currency"`
	Lines        []engine.Line   `json:"lines"`
	Available    bool            `json:"available"`
	Reason       string          `json:"unavailable_reason,omitempty"`
	Source       string          `json:"pricing_source"` // METERED | FIXED_ROUTE
	RouteName    string          `json:"fixed_route_name,omitempty"`
	FixedRouteID string          `json:"fixed_route_id,omitempty"`
	// Waiting terms at the pickup, snapshotted so the trip charges what was quoted.
	WaitingGraceS     int             `json:"waiting_grace_s,omitempty"`
	WaitingRatePerMin decimal.Decimal `json:"waiting_rate_per_min,omitempty"`
	WaitingCap        decimal.Decimal `json:"waiting_cap,omitempty"` // 0 = uncapped
	// Cancellation terms snapshotted for the trip (see FeePolicy).
	CancellationFee   decimal.Decimal `json:"cancellation_fee,omitempty"`
	CancelFreeS       int             `json:"cancel_free_s,omitempty"`
	NoShowAfterS      int             `json:"no_show_after_s,omitempty"`
	CancelDriverShare decimal.Decimal `json:"cancel_driver_share,omitempty"`
}

// Quote is what the rider gets back and what a booking later references.
type Quote struct {
	ID             string          `json:"quote_id"`
	ServiceKey     string          `json:"service"`
	Country        string          `json:"country"`
	Area           AreaRef         `json:"area"`
	Route          routing.Route   `json:"route"`
	Surge          decimal.Decimal `json:"surge_multiplier"`
	Options        []Option        `json:"options"`
	ExpiresAt      time.Time       `json:"expires_at"`
	PricingVersion int64           `json:"-"`
	// pricing is the profile resolution and per-class pins stored on the quote (never sent to riders).
	pricing profiles.QuotePricing
}

// FeePolicy is the platform money policy in force for one scope (a country, or an area within it).
type FeePolicy struct {
	Country        string
	AreaID         string // "" for the country default
	Version        int
	CommissionRate decimal.Decimal
	VATRate        decimal.Decimal
	VATBase        string
	LevyRate       decimal.Decimal
	LevyBorneBy    string
	BookingFee     decimal.Decimal
	// WaitingCapRate caps the pickup waiting fee as a share of the fare (0 = no cap).
	WaitingCapRate decimal.Decimal
	// FareRounding rounds every quoted fare to this step (e.g. 50); 0 = none.
	FareRounding decimal.Decimal
	// Cancellation terms: free cancel window after acceptance, wait before a
	// no-show may be declared, and the driver's share of a cancellation fee.
	CancelFreeS       int
	NoShowAfterS      int
	CancelDriverShare decimal.Decimal
	// Driver debt rules (account rules, read from the policy in force):
	// a hold starts at DebtHardLimit, or after DebtGraceDays at or above DebtSoftLimit;
	// PayinMinAmount is the smallest mobile-money payment accepted.
	DebtSoftLimit  decimal.Decimal
	DebtHardLimit  decimal.Decimal
	DebtGraceDays  int
	PayinMinAmount decimal.Decimal
	// Payout rules: the smallest cash-out a driver may request, and whether
	// requests are paid without a Finance approval.
	PayoutMinAmount   decimal.Decimal
	PayoutAutoApprove bool
}

// Error codes owned by this module.
var (
	ErrUnknownService         = apperr.New("SERVICE_UNKNOWN", http.StatusNotFound, "")
	ErrAttributesInvalid      = apperr.New("VALIDATION_FAILED", http.StatusBadRequest, "")
	ErrNoTariff               = apperr.New("PRICING_NOT_CONFIGURED", http.StatusConflict, "")
	ErrRoutingFailed          = apperr.New("ROUTING_FAILED", http.StatusBadGateway, "")
	ErrDropoffRequired        = apperr.New("DROPOFF_REQUIRED", http.StatusBadRequest, "")
	ErrTooManyStops           = apperr.New("TOO_MANY_STOPS", http.StatusBadRequest, "")
	ErrCrossBorder            = apperr.New("CROSS_BORDER_NOT_SUPPORTED", http.StatusConflict, "")
	ErrCountryDefaultRequired = apperr.New("COUNTRY_DEFAULT_REQUIRED", http.StatusConflict, "")
	ErrPolicyFieldCountryOnly = apperr.New("POLICY_FIELD_COUNTRY_ONLY", http.StatusConflict, "")
)
