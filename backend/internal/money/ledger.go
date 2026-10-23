package money

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// Account kinds (ledger_accounts.kind).
const (
	DriverWallet      = "DRIVER_WALLET"
	PlatformRevenue   = "PLATFORM_REVENUE"
	VATPayable        = "VAT_PAYABLE"
	LevyPayable       = "LEVY_PAYABLE"
	CashHeldByDrivers = "CASH_HELD_BY_DRIVERS"
	RiderDues         = "RIDER_DUES"       // per rider: positive = the rider owes the platform
	GatewayClearing   = "GATEWAY_CLEARING" // collected at the payment gateway, not yet swept to the bank
	ChargesPayable    = "CHARGES_PAYABLE"  // per payee: extra charges owed to a third party (airport, council …)
	genesisHash       = "genesis"
	txTripCompleted   = "trip.completed"
)

// Leg is one side of a transaction. Amount: + debit, − credit.
// For a DRIVER_WALLET a debit means the driver owes the platform.
type Leg struct {
	Kind    string
	OwnerID string // driver id for DRIVER_WALLET, empty for platform accounts
	Amount  decimal.Decimal
}

// Ledger posts balanced, hash-chained transactions.
type Ledger struct {
	pool    *pgxpool.Pool
	brandID string
	// onDriverWallet runs after commit for every driver whose wallet a Post moved
	// (debt holds re-evaluate from it).
	onDriverWallet []func(ctx context.Context, driverID string)
}

// OnDriverWallet registers a listener called after a posted transaction moved a driver's wallet.
func (l *Ledger) OnDriverWallet(fn func(ctx context.Context, driverID string)) {
	l.onDriverWallet = append(l.onDriverWallet, fn)
}

func NewLedger(pool *pgxpool.Pool, brandID string) *Ledger {
	return &Ledger{pool: pool, brandID: brandID}
}

var ErrUnbalanced = errors.New("money: transaction legs do not sum to zero")

// Post records one transaction in its own database transaction. Posting the
// same (kind, reference) twice is a no-op, so callers can retry safely. Returns
// false when it was already posted. Wallet listeners run after commit.
func (l *Ledger) Post(ctx context.Context, kind, reference, memo, currency string, legs []Leg) (bool, error) {
	posted := false
	err := pgx.BeginFunc(ctx, l.pool, func(tx pgx.Tx) error {
		var err error
		posted, err = l.PostTx(ctx, tx, kind, reference, memo, currency, legs)
		return err
	})
	if err == nil && posted {
		l.notifyWallets(ctx, legs)
	}
	return posted, err
}

// PostTx records one transaction inside the caller's database transaction, so a
// money movement and the state change that caused it commit together. The caller
// is responsible for anything that must happen after commit.
func (l *Ledger) PostTx(ctx context.Context, tx pgx.Tx, kind, reference, memo, currency string, legs []Leg) (bool, error) {
	sum := decimal.Zero
	for _, g := range legs {
		sum = sum.Add(g.Amount)
	}
	if !sum.IsZero() {
		return false, ErrUnbalanced
	}
	// One writer at a time per brand keeps the hash chain linear.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('ledger:' || $1))`, l.brandID); err != nil {
		return false, err
	}
	var txID string
	err := tx.QueryRow(ctx, `INSERT INTO ledger_transactions (brand_id, kind, reference, memo) VALUES ($1,$2,$3,$4)
		ON CONFLICT (brand_id, kind, reference) DO NOTHING RETURNING id`, l.brandID, kind, reference, memo).Scan(&txID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil // already posted
	}
	if err != nil {
		return false, err
	}
	return true, l.writeLegs(ctx, tx, txID, currency, legs)
}

// writeLegs appends the entries of one transaction to the hash chain.
func (l *Ledger) writeLegs(ctx context.Context, tx pgx.Tx, txID, currency string, legs []Leg) error {
	prev, err := l.lastHash(ctx, tx)
	if err != nil {
		return err
	}
	for _, g := range legs {
		if g.Amount.IsZero() {
			continue
		}
		acct, err := l.account(ctx, tx, g.Kind, g.OwnerID, currency)
		if err != nil {
			return err
		}
		h := entryHash(prev, txID, acct, g.Amount)
		if _, err := tx.Exec(ctx, `INSERT INTO ledger_entries (transaction_id, account_id, amount, prev_hash, hash) VALUES ($1,$2,$3,$4,$5)`,
			txID, acct, g.Amount, prev, h); err != nil {
			return err
		}
		prev = h
	}
	return nil
}

// notifyWallets tells wallet listeners about every driver a transaction touched.
func (l *Ledger) notifyWallets(ctx context.Context, legs []Leg) {
	for _, g := range legs {
		if g.Kind != DriverWallet || g.OwnerID == "" || g.Amount.IsZero() {
			continue
		}
		for _, fn := range l.onDriverWallet {
			fn(ctx, g.OwnerID)
		}
	}
}

// PostCashTripCompleted records a trip the driver was paid for in cash: the
// driver holds the whole fare, so the platform's share, VAT and levy become the
// driver's debt. Trips paid through the gateway post with PostTripPaidTx instead.
func (l *Ledger) PostCashTripCompleted(ctx context.Context, tripID, driverID string, b Breakdown) (bool, error) {
	posted := false
	err := pgx.BeginFunc(ctx, l.pool, func(tx pgx.Tx) error {
		var err error
		posted, err = l.PostCashTripCompletedTx(ctx, tx, tripID, driverID, b)
		return err
	})
	if err == nil && posted {
		l.WalletMoved(ctx, driverID)
	}
	return posted, err
}

// PostCashTripCompletedTx is PostCashTripCompleted inside the transaction that
// completes the trip, so the driver's debt can never be missed (a missed debt
// would let the driver cash out the platform's share). Callers tell wallet
// listeners after commit.
func (l *Ledger) PostCashTripCompletedTx(ctx context.Context, tx pgx.Tx, tripID, driverID string, b Breakdown) (bool, error) {
	owed := b.Total.Sub(b.DriverEarnings) // platform net + VAT + levy + third-party charges
	legs := append([]Leg{
		{Kind: DriverWallet, OwnerID: driverID, Amount: owed},
		{Kind: PlatformRevenue, Amount: b.PlatformNet.Neg()},
		{Kind: VATPayable, Amount: b.VAT.Neg()},
		{Kind: LevyPayable, Amount: b.Levy.Neg()},
	}, thirdPartyLegs(b)...)
	memo := fmt.Sprintf("Cash trip: fare %s, you keep %s, platform share %s", b.Total, b.DriverEarnings, owed)
	return l.PostTx(ctx, tx, txTripCompleted, tripID, memo, b.Currency, legs)
}

// thirdPartyLegs credits each payee's CHARGES_PAYABLE account with its charges.
func thirdPartyLegs(b Breakdown) []Leg {
	var legs []Leg
	for _, c := range b.Charges {
		if c.Recipient == ChargeToThirdParty && c.Amount.IsPositive() {
			legs = append(legs, Leg{Kind: ChargesPayable, OwnerID: c.PayeeID, Amount: c.Amount.Neg()})
		}
	}
	return legs
}

// WalletEntry is one line in a driver's wallet history.
type WalletEntry struct {
	At        time.Time       `json:"at"`
	Kind      string          `json:"kind"`
	Reference string          `json:"reference"`
	Memo      string          `json:"memo"`
	Amount    decimal.Decimal `json:"amount"` // as the driver sees it: − = you owe, + = owed to you
}

// Wallet is a driver's balance (− = owes the platform) and recent history.
type Wallet struct {
	Currency string          `json:"currency"`
	Balance  decimal.Decimal `json:"balance"`
	Entries  []WalletEntry   `json:"entries"`
}

// DriverWallet returns the driver's balance and last entries.
func (l *Ledger) DriverWallet(ctx context.Context, driverID string, limit int) (Wallet, error) {
	w := Wallet{Balance: decimal.Zero, Entries: []WalletEntry{}}
	rows, err := l.pool.Query(ctx, `
		SELECT a.currency, t.created_at, t.kind, t.reference, coalesce(t.memo,''), e.amount
		FROM ledger_entries e JOIN ledger_accounts a ON a.id = e.account_id JOIN ledger_transactions t ON t.id = e.transaction_id
		WHERE a.brand_id=$1 AND a.kind='DRIVER_WALLET' AND a.owner_id=$2
		ORDER BY e.seq DESC LIMIT $3`, l.brandID, driverID, limit)
	if err != nil {
		return w, err
	}
	defer rows.Close()
	for rows.Next() {
		var e WalletEntry
		if err := rows.Scan(&w.Currency, &e.At, &e.Kind, &e.Reference, &e.Memo, &e.Amount); err != nil {
			return w, err
		}
		e.Amount = e.Amount.Neg() // debit on the receivable = money the driver owes
		w.Entries = append(w.Entries, e)
	}
	if err := rows.Err(); err != nil {
		return w, err
	}
	err = l.pool.QueryRow(ctx, `SELECT coalesce(-sum(e.amount),0) FROM ledger_entries e JOIN ledger_accounts a ON a.id=e.account_id
		WHERE a.brand_id=$1 AND a.kind='DRIVER_WALLET' AND a.owner_id=$2`, l.brandID, driverID).Scan(&w.Balance)
	return w, err
}

// Verify recomputes the whole hash chain; it returns the first broken entry, or 0.
func (l *Ledger) Verify(ctx context.Context) (int64, int, error) {
	rows, err := l.pool.Query(ctx, `SELECT e.seq, e.transaction_id, e.account_id, e.amount, e.prev_hash, e.hash
		FROM ledger_entries e JOIN ledger_accounts a ON a.id=e.account_id WHERE a.brand_id=$1 ORDER BY e.seq`, l.brandID)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()
	prev, n := genesisHash, 0
	for rows.Next() {
		var seq int64
		var txID, acct, prevHash, hash string
		var amount decimal.Decimal
		if err := rows.Scan(&seq, &txID, &acct, &amount, &prevHash, &hash); err != nil {
			return 0, n, err
		}
		if prevHash != prev || entryHash(prev, txID, acct, amount) != hash {
			return seq, n, nil
		}
		prev, n = hash, n+1
	}
	return 0, n, rows.Err()
}

func (l *Ledger) lastHash(ctx context.Context, tx pgx.Tx) (string, error) {
	var h string
	err := tx.QueryRow(ctx, `SELECT e.hash FROM ledger_entries e JOIN ledger_accounts a ON a.id=e.account_id
		WHERE a.brand_id=$1 ORDER BY e.seq DESC LIMIT 1`, l.brandID).Scan(&h)
	if errors.Is(err, pgx.ErrNoRows) {
		return genesisHash, nil
	}
	return h, err
}

// account finds or creates the ledger account for (kind, owner, currency).
func (l *Ledger) account(ctx context.Context, tx pgx.Tx, kind, owner, currency string) (string, error) {
	var ownerArg any
	if owner != "" {
		ownerArg = owner
	}
	var id string
	err := tx.QueryRow(ctx, `INSERT INTO ledger_accounts (brand_id, kind, owner_id, currency) VALUES ($1,$2::account_kind,$3,$4)
		ON CONFLICT (brand_id, kind, owner_id, currency) DO UPDATE SET kind=EXCLUDED.kind RETURNING id`,
		l.brandID, kind, ownerArg, currency).Scan(&id)
	return id, err
}

// entryHash chains an entry to the one before it.
func entryHash(prev, txID, accountID string, amount decimal.Decimal) string {
	sum := sha256.Sum256([]byte(prev + "|" + txID + "|" + accountID + "|" + amount.StringFixed(2)))
	return hex.EncodeToString(sum[:])
}
