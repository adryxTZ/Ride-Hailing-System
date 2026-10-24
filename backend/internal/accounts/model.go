package accounts

import (
	"net/http"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// Positions say which way an account's balance points, in the words the page uses.
const (
	PositionEarned   = "EARNED"     // platform revenue
	PositionWeOwe    = "WE_OWE"     // VAT, levy, payees, rider refunds
	PositionOwedToUs = "OWED_TO_US" // rider dues; drivers net (with the split)
	PositionHeld     = "HELD"       // collected at the gateway, not yet in the bank
	PositionBank     = "BANK"       // the platform's bank: negative = net paid out
)

// Payment methods staff can record.
var Methods = []string{"BANK_TRANSFER", "MOBILE_MONEY", "CASH", "CHEQUE", "OTHER"}

// methodWords is how a method reads in the ledger memo.
var methodWords = map[string]string{
	"BANK_TRANSFER": "bank transfer", "MOBILE_MONEY": "mobile money", "CASH": "cash", "CHEQUE": "cheque", "OTHER": "other means",
}

// Limits.
const (
	EntriesPageDefault = 50
	EntriesPageMax     = 200
	ExportMaxRows      = 50_000 // one CSV; a longer period is exported in parts
	referenceMaxLen    = 120
	reasonMaxLen       = 1000
	keyMaxLen          = 200
	// paidOnSlack lets a payment be dated "today" in a timezone ahead of the server's.
	paidOnSlack = 24 * time.Hour
)

// Account is one party's line on the Accounts page.
type Account struct {
	Key       string          `json:"key"` // "VAT_PAYABLE" or "CHARGES_PAYABLE:<payee id>"
	Kind      string          `json:"kind"`
	PayeeID   *string         `json:"payee_id,omitempty"`
	Name      *string         `json:"name,omitempty"`       // the payee's name
	PayeeKind *string         `json:"payee_kind,omitempty"` // AIRPORT, MUNICIPALITY …
	Currency  string          `json:"currency"`
	Position  string          `json:"position"`
	Balance   decimal.Decimal `json:"balance"` // in the position's direction: what we owe, what is owed to us, what we earned
	// OwedToUs and WeOwe split accounts kept per person (drivers, riders): the
	// sum of balances owed to us and owed by us; Balance is their difference.
	OwedToUs *decimal.Decimal `json:"owed_to_us,omitempty"`
	WeOwe    *decimal.Decimal `json:"we_owe,omitempty"`
	Holders  int              `json:"holders"`             // ledger accounts behind the line (drivers, riders …)
	Added    decimal.Decimal  `json:"added"`               // the period's movements that grew the balance
	Reduced  decimal.Decimal  `json:"reduced"`             // the period's movements that shrank it (payments, clearing)
	FareBase *decimal.Decimal `json:"fare_base,omitempty"` // fare-share parties: fares of the period's trips it took a share of
	OnFares  *decimal.Decimal `json:"on_fares,omitempty"`  // its share of those fares
	Rate     *decimal.Decimal `json:"rate,omitempty"`      // OnFares / FareBase
	Entries  int              `json:"entries"`
	LastAt   *time.Time       `json:"last_at,omitempty"`
	// Settleable accounts take "Record payment".
	Settleable bool `json:"settleable"`
}

// List is the Accounts page: every account in one currency for a period.
type List struct {
	Currency string          `json:"currency"`
	From     time.Time       `json:"from"`
	To       time.Time       `json:"to"`    // exclusive
	Fares    decimal.Decimal `json:"fares"` // all fares of trips completed in the period
	Trips    int             `json:"trips"`
	Accounts []Account       `json:"accounts"`
}

// Entry is one ledger movement on an account.
type Entry struct {
	Seq       int64           `json:"seq"`
	At        time.Time       `json:"at"`
	Kind      string          `json:"kind"` // ledger transaction kind (trip.completed, finance.settlement …)
	Reference string          `json:"reference"`
	Memo      string          `json:"memo"`
	Amount    decimal.Decimal `json:"amount"`  // in the account's direction: + grew the balance
	Running   decimal.Decimal `json:"running"` // the balance after this entry
	TripID    *string         `json:"trip_id,omitempty"`
	TripCode  *string         `json:"trip_code,omitempty"`
	HolderID  *string         `json:"holder_id,omitempty"` // the driver or rider of a per-person account
	Holder    *string         `json:"holder,omitempty"`    // their name or phone
}

// Page is one page of an account's entries (the account's line is on the list).
type Page struct {
	Key     string  `json:"key"`
	Entries []Entry `json:"entries"`
	Total   int     `json:"total"`
	Offset  int     `json:"offset"`
	Limit   int     `json:"limit"`
}

// Period bounds a query: From inclusive, To exclusive.
type Period struct{ From, To time.Time }

// PaymentInput is "Record payment" on a payable account.
type PaymentInput struct {
	Key       string // the account key
	Currency  string // the scope's currency
	Country   string // the scope's country: a payee must be one of its payees
	Amount    decimal.Decimal
	PaidOn    string // YYYY-MM-DD
	Method    string
	Reference string
	Reason    string
	RefundIDs []string // RIDER_REFUNDS: pay these refunds (the amount must match)
	RiderID   string   // RIDER_REFUNDS: pay down one rider's balance
	IdemKey   string
	StaffID   string
}

// Payment is a recorded payment.
type Payment struct {
	ID        string          `json:"id"`
	Key       string          `json:"key"`
	Kind      string          `json:"kind"`
	PayeeID   *string         `json:"payee_id,omitempty"`
	RiderID   *string         `json:"rider_id,omitempty"`
	Currency  string          `json:"currency"`
	Amount    decimal.Decimal `json:"amount"`
	PaidOn    string          `json:"paid_on"`
	Method    string          `json:"method"`
	Reference string          `json:"reference"`
	Reason    string          `json:"reason"`
	RefundIDs []string        `json:"refund_ids"`
	CreatedAt time.Time       `json:"created_at"`
	Replayed  bool            `json:"replayed"` // a repeated Idempotency-Key: nothing new was posted
}

// OwedRefund is one rider refund Finance has not paid yet.
type OwedRefund struct {
	ID        string          `json:"id"`
	TripID    string          `json:"trip_id"`
	TripCode  string          `json:"trip_code"`
	RiderID   string          `json:"rider_id"`
	Rider     string          `json:"rider"`
	Amount    decimal.Decimal `json:"amount"`
	Reason    string          `json:"reason"`
	CreatedAt time.Time       `json:"created_at"`
}

// OwedRider is what one rider is owed in total (refunds and late fare money).
type OwedRider struct {
	RiderID string          `json:"rider_id"`
	Rider   string          `json:"rider"`
	Owed    decimal.Decimal `json:"owed"`
}

// Owed is what riders are owed, for the rider-refunds payment dialog.
type Owed struct {
	Currency string       `json:"currency"`
	Refunds  []OwedRefund `json:"refunds"`
	Riders   []OwedRider  `json:"riders"`
}

// Errors owned by this module.
var (
	ErrAccountNotFound = apperr.New("ACCOUNT_NOT_FOUND", http.StatusNotFound, "")
	ErrNotSettleable   = apperr.New("ACCOUNT_NOT_SETTLEABLE", http.StatusConflict, "")
	ErrInvalid         = apperr.New("VALIDATION_FAILED", http.StatusBadRequest, "")
	ErrKeyRequired     = apperr.New("IDEMPOTENCY_KEY_REQUIRED", http.StatusBadRequest, "")
	ErrKeyReused       = apperr.New("IDEMPOTENCY_KEY_REUSED", http.StatusConflict, "")
	ErrOverpay         = apperr.New("PAYMENT_MORE_THAN_OWED", http.StatusConflict, "")
	ErrAmountMismatch  = apperr.New("PAYMENT_AMOUNT_MISMATCH", http.StatusConflict, "")
	ErrRefundNotOwed   = apperr.New("REFUND_NOT_OWED", http.StatusConflict, "")
)
