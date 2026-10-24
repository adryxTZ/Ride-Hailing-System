package accounts

import (
	"strings"

	"github.com/google/uuid"

	"github.com/kalen-limited/hauler-rider/backend/internal/money"
)

// keySep separates a kind from its payee in an account key.
const keySep = ":"

// kindInfo is how one ledger account kind reads on the Accounts page.
type kindInfo struct {
	position   string
	sign       int  // +1: a debit grows the balance (owed to us); −1: a credit does (we owe, earned)
	perHolder  bool // one ledger account per driver or rider, shown as one line
	settleable bool // takes "Record payment"
	fareShare  bool // takes a share of fares: report the fare base and effective rate
	order      int  // line order on the page
}

// kinds lists every account kind the page shows, in page order. Charge payees
// get one line each (CHARGES_PAYABLE:<payee>).
var kinds = map[string]kindInfo{
	money.PlatformRevenue: {position: PositionEarned, sign: -1, fareShare: true, order: 1},
	money.VATPayable:      {position: PositionWeOwe, sign: -1, settleable: true, fareShare: true, order: 2},
	money.LevyPayable:     {position: PositionWeOwe, sign: -1, settleable: true, fareShare: true, order: 3},
	money.ChargesPayable:  {position: PositionWeOwe, sign: -1, settleable: true, fareShare: true, order: 4},
	money.DriverWallet:    {position: PositionOwedToUs, sign: 1, perHolder: true, order: 5},
	money.RiderDues:       {position: PositionOwedToUs, sign: 1, perHolder: true, order: 6},
	money.RiderRefunds:    {position: PositionWeOwe, sign: -1, perHolder: true, settleable: true, order: 7},
	money.GatewayClearing: {position: PositionHeld, sign: 1, order: 8},
	money.PlatformBank:    {position: PositionBank, sign: 1, order: 9},
}

// fixedKinds are always listed, even before their first entry.
var fixedKinds = []string{
	money.PlatformRevenue, money.VATPayable, money.LevyPayable, money.DriverWallet,
	money.RiderDues, money.RiderRefunds, money.GatewayClearing, money.PlatformBank,
}

// unknownKind reads a kind the page does not know as a plain debit balance.
var unknownKind = kindInfo{position: PositionOwedToUs, sign: 1, order: 99}

// info returns how a kind reads.
func info(kind string) kindInfo {
	if k, ok := kinds[kind]; ok {
		return k
	}
	return unknownKind
}

// KeyOf is an account's key: its kind, plus the payee for a charge payee.
func KeyOf(kind, payeeID string) string {
	if kind == money.ChargesPayable && payeeID != "" {
		return kind + keySep + payeeID
	}
	return kind
}

// ParseKey splits a key into kind and payee; unknown kinds and a payee line
// without a valid payee id are not found.
func ParseKey(key string) (kind, payeeID string, err error) {
	kind, payeeID, _ = strings.Cut(key, keySep)
	if _, ok := kinds[kind]; !ok {
		return "", "", ErrAccountNotFound
	}
	if (kind == money.ChargesPayable) != (payeeID != "") {
		return "", "", ErrAccountNotFound
	}
	if payeeID != "" {
		if _, perr := uuid.Parse(payeeID); perr != nil {
			return "", "", ErrAccountNotFound
		}
	}
	return kind, payeeID, nil
}
