package people

import (
	"context"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/dispatch"
	"github.com/kalen-limited/hauler-rider/backend/internal/holds"
	"github.com/kalen-limited/hauler-rider/backend/internal/identity"
	"github.com/kalen-limited/hauler-rider/backend/internal/messaging"
	"github.com/kalen-limited/hauler-rider/backend/internal/money"
	"github.com/kalen-limited/hauler-rider/backend/internal/payments"
	"github.com/kalen-limited/hauler-rider/backend/internal/payouts"
	"github.com/kalen-limited/hauler-rider/backend/internal/presence"
	"github.com/kalen-limited/hauler-rider/backend/internal/profile"
	"github.com/kalen-limited/hauler-rider/backend/internal/safety"
	"github.com/kalen-limited/hauler-rider/backend/internal/trips"
)

// Accounts reads a person's account (identity).
type Accounts interface {
	PersonAccount(ctx context.Context, userID, role string) (*identity.PersonAccount, error)
}

// Trips reads a person's trip figures (trips).
type Trips interface {
	PersonStats(ctx context.Context, role, userID string) (*trips.PersonStats, error)
	TopRoutes(ctx context.Context, role, userID string) ([]trips.Route, error)
	PersonSeries(ctx context.Context, role, userID, tz string, days int) (*trips.PersonSeries, error)
	LikelyHome(ctx context.Context, role, userID, tz string) (*trips.Place, error)
	LastPlace(ctx context.Context, role, userID string) (*trips.Place, error)
	Countries(ctx context.Context, role, userID string) ([]string, error)
}

// Offers reads how a driver answers offers (dispatch).
type Offers interface {
	OfferStats(ctx context.Context, driverID string) (*dispatch.OfferStats, error)
	OfferSeries(ctx context.Context, driverID, tz string, since time.Time) ([]dispatch.OfferDay, error)
}

// Ratings reads ratings both ways (messaging).
type Ratings interface {
	PersonRatings(ctx context.Context, userID string) (*messaging.PersonRatings, error)
}

// Alerts reads SOS on a person's trips (safety).
type Alerts interface {
	AlertsFor(ctx context.Context, userID string) ([]safety.Alert, error)
}

// SavedPlaces reads saved places (profile).
type SavedPlaces interface {
	ListPlaces(ctx context.Context, userID string) ([]profile.Place, error)
}

// Live reads a driver's live position (presence, Redis).
type Live interface {
	Get(ctx context.Context, driverID string) (*presence.Driver, error)
}

// Money reads balances (money ledger).
type Money interface {
	DriverWallet(ctx context.Context, driverID string, limit int) (money.Wallet, error)
	RiderDues(ctx context.Context, riderID string) (decimal.Decimal, error)
}

// Holds reads a driver's holds (holds).
type Holds interface {
	Status(ctx context.Context, driverID string) (holds.Status, error)
}

// Payouts reads a driver's cash-outs (payouts).
type Payouts interface {
	List(ctx context.Context, driverID string) ([]payouts.Payout, error)
}

// Payins reads a driver's pay-ins (payments).
type Payins interface {
	List(ctx context.Context, status, driverID, currency, purpose string, limit int) ([]payments.AdminPayin, error)
}

// Deps are the ports; Offers, Live, Holds, Payouts and Payins may be nil (feature off).
type Deps struct {
	Accounts Accounts
	Trips    Trips
	Offers   Offers
	Ratings  Ratings
	Alerts   Alerts
	Places   SavedPlaces
	Live     Live
	Money    Money
	Holds    Holds
	Payouts  Payouts
	Payins   Payins
	// Timezone returns a country's IANA timezone (for the likely-home guess).
	Timezone func(country string) string
	// DriverCountry returns the country a driver registered in ("" if none).
	DriverCountry func(ctx context.Context, driverID string) (string, error)
}
