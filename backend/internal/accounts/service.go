package accounts

import (
	"cmp"
	"context"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// rateScale is the decimals an effective rate is reported with (0.0180 = 1.80 %).
const rateScale = 4

// Service is the Accounts page use cases for one brand.
type Service struct {
	store  store
	ledger Ledger
	now    func() time.Time
}

// New wires the service.
func New(pool *pgxpool.Pool, brandID string, ledger Ledger) *Service {
	return &Service{store: store{pool: pool, brandID: brandID}, ledger: ledger, now: time.Now}
}

// List is every account in one currency with the period's movements (one query).
func (s *Service) List(ctx context.Context, currency string, p Period) (*List, error) {
	raw, fares, trips, err := s.store.list(ctx, currency, p)
	if err != nil {
		return nil, err
	}
	out := &List{Currency: currency, From: p.From, To: p.To, Fares: fares, Trips: trips, Accounts: []Account{}}
	seen := map[string]bool{}
	for _, r := range raw {
		a := toAccount(r, currency)
		seen[a.Key] = true
		out.Accounts = append(out.Accounts, a)
	}
	for _, kind := range fixedKinds {
		if !seen[kind] {
			out.Accounts = append(out.Accounts, emptyAccount(kind, currency))
		}
	}
	slices.SortStableFunc(out.Accounts, byPageOrder)
	return out, nil
}

// byPageOrder orders lines by kind, then payees by what we owe them, then name.
func byPageOrder(a, b Account) int {
	if c := cmp.Compare(info(a.Kind).order, info(b.Kind).order); c != 0 {
		return c
	}
	if c := b.Balance.Cmp(a.Balance); c != 0 {
		return c
	}
	return cmp.Compare(deref(a.Name), deref(b.Name))
}

// emptyAccount is a line for a kind with no ledger account yet.
func emptyAccount(kind, currency string) Account {
	k := info(kind)
	a := Account{Key: kind, Kind: kind, Currency: currency, Position: k.position, Settleable: k.settleable}
	if k.perHolder {
		zero := decimal.Zero
		a.OwedToUs, a.WeOwe = &zero, &zero
	}
	return a
}

// toAccount reads one raw line in the account's own direction.
func toAccount(r rawLine, currency string) Account {
	kind := *r.kind
	k := info(kind)
	sign := decimal.NewFromInt(int64(k.sign))
	a := emptyAccount(kind, currency)
	a.Key, a.PayeeID, a.Name, a.PayeeKind = KeyOf(kind, deref(r.payee)), r.payee, r.name, r.payeeKind
	a.Balance = val(r.bal).Mul(sign)
	a.Added, a.Reduced = val(r.deb), val(r.cred)
	if k.sign < 0 {
		a.Added, a.Reduced = a.Reduced, a.Added
	}
	if k.perHolder {
		pos, neg := val(r.pos), val(r.neg) // debit balances are owed to us, credit balances owed by us
		a.OwedToUs, a.WeOwe = &pos, &neg
	}
	a.Entries, a.Holders, a.LastAt = intVal(r.entries), intVal(r.holders), r.lastAt
	if k.fareShare && val(r.base).IsPositive() {
		base, on := val(r.base), val(r.accrued)
		rate := on.Div(base).Round(rateScale)
		a.FareBase, a.OnFares, a.Rate = &base, &on, &rate
	}
	return a
}

// Entries is one page of an account's entries in the period, newest first,
// amounts in the account's direction (one query).
func (s *Service) Entries(ctx context.Context, key, currency string, p Period, limit, offset int) (*Page, error) {
	q, err := s.entryQuery(key, currency, p, limit, offset)
	if err != nil {
		return nil, err
	}
	list, total, err := s.store.entries(ctx, q)
	if err != nil {
		return nil, err
	}
	orient(list, info(q.kind).sign)
	return &Page{Key: key, Entries: list, Total: total, Offset: q.offset, Limit: q.limit}, nil
}

// entryQuery checks the key and bounds the page.
func (s *Service) entryQuery(key, currency string, p Period, limit, offset int) (entryQuery, error) {
	kind, payee, err := ParseKey(key)
	if err != nil {
		return entryQuery{}, err
	}
	if limit <= 0 || limit > EntriesPageMax {
		limit = EntriesPageDefault
	}
	return entryQuery{kind: kind, payee: payee, currency: currency, period: p, limit: limit, offset: max(offset, 0),
		perHolder: info(kind).perHolder}, nil
}

// orient flips amounts of credit-normal accounts so + always grows the balance.
func orient(list []Entry, sign int) {
	if sign > 0 {
		return
	}
	for i := range list {
		list[i].Amount, list[i].Running = list[i].Amount.Neg(), list[i].Running.Neg()
	}
}

// Export is the period's entries of an account for the CSV file, oldest first
// and in the account's direction (one query, at most ExportMaxRows).
func (s *Service) Export(ctx context.Context, key, currency string, p Period) ([]Entry, error) {
	q, err := s.entryQuery(key, currency, p, EntriesPageMax, 0)
	if err != nil {
		return nil, err
	}
	q.limit = ExportMaxRows
	list, _, err := s.store.entries(ctx, q)
	if err != nil {
		return nil, err
	}
	orient(list, info(q.kind).sign)
	slices.Reverse(list)
	return list, nil
}

// Owed is what riders are owed in one currency: unpaid refunds oldest first
// and riders by total owed (two queries), for the rider-refunds payment.
func (s *Service) Owed(ctx context.Context, currency string) (*Owed, error) {
	refunds, err := s.store.owedRefunds(ctx, nil, refundFilter{currency: currency}, false)
	if err != nil {
		return nil, err
	}
	riders, err := s.store.owedRiders(ctx, currency)
	if err != nil {
		return nil, err
	}
	return &Owed{Currency: currency, Refunds: refunds, Riders: riders}, nil
}

func val(d *decimal.Decimal) decimal.Decimal {
	if d == nil {
		return decimal.Zero
	}
	return *d
}

func intVal(n *int) int {
	if n == nil {
		return 0
	}
	return *n
}
