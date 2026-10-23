package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/riverqueue/river"
	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/holds"
	"github.com/kalen-limited/hauler-rider/backend/internal/identity"
	"github.com/kalen-limited/hauler-rider/backend/internal/money"
	"github.com/kalen-limited/hauler-rider/backend/internal/payments"
	"github.com/kalen-limited/hauler-rider/backend/internal/payouts"
	"github.com/kalen-limited/hauler-rider/backend/internal/presence"
	"github.com/kalen-limited/hauler-rider/backend/internal/pricing"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/payment"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/payment/fake"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/payment/selcom"
	"github.com/kalen-limited/hauler-rider/backend/internal/realtime"
	"github.com/kalen-limited/hauler-rider/backend/internal/trips"
)

// moneyWorkers are registered before the River client exists and get their
// services once those are built (the client is needed to build them).
type moneyWorkers struct {
	holdSweep       *holds.SweepWorker
	check           *payments.CheckWorker
	reconcile       *payments.ReconcileWorker
	payoutSend      *payouts.SendWorker
	payoutCheck     *payouts.CheckWorker
	payoutReconcile *payouts.ReconcileWorker
}

// registerMoneyWorkers adds the hold sweep and pay-in jobs to the worker set
// and schedules the periodic ones.
func registerMoneyWorkers(workers *river.Workers) (moneyWorkers, []*river.PeriodicJob) {
	mw := moneyWorkers{holdSweep: &holds.SweepWorker{}, check: &payments.CheckWorker{}, reconcile: &payments.ReconcileWorker{},
		payoutSend: &payouts.SendWorker{}, payoutCheck: &payouts.CheckWorker{}, payoutReconcile: &payouts.ReconcileWorker{}}
	river.AddWorker(workers, mw.holdSweep)
	river.AddWorker(workers, mw.check)
	river.AddWorker(workers, mw.reconcile)
	river.AddWorker(workers, mw.payoutSend)
	river.AddWorker(workers, mw.payoutCheck)
	river.AddWorker(workers, mw.payoutReconcile)
	periodic := []*river.PeriodicJob{
		river.NewPeriodicJob(river.PeriodicInterval(holds.SweepInterval), func() (river.JobArgs, *river.InsertOpts) {
			return holds.SweepArgs{}, nil
		}, &river.PeriodicJobOpts{ID: "holds.sweep", RunOnStart: true}),
		river.NewPeriodicJob(river.PeriodicInterval(payments.ReconcileInterval), func() (river.JobArgs, *river.InsertOpts) {
			return payments.ReconcileArgs{}, nil
		}, &river.PeriodicJobOpts{ID: "payments.reconcile", RunOnStart: true}),
		river.NewPeriodicJob(river.PeriodicInterval(payouts.ReconcileInterval), func() (river.JobArgs, *river.InsertOpts) {
			return payouts.ReconcileArgs{}, nil
		}, &river.PeriodicJobOpts{ID: "payouts.reconcile", RunOnStart: true}),
	}
	return mw, periodic
}

// paymentGateways builds the collection and payout gateways of each country
// from its brand entry (countries[].payment_gateway): every country pays
// through its own provider, and one without a configured provider simply has
// no mobile-money payments (cash still works). NEXA_PAYMENT_GATEWAY overrides
// every country for development; only that override may name the fake gateway,
// which reports money that never moved.
func paymentGateways(bs *bootstrap) (*payment.Gateways, *payment.Payouters, error) {
	collect, payout := map[string]payment.Gateway{}, map[string]payment.Payouter{}
	built := map[string]gatewayPair{}
	for _, c := range bs.brand.Countries {
		name := bs.cfg.PaymentGateway
		if name == "" {
			name = c.PaymentGateway
		}
		pair, ok := built[name]
		if !ok {
			var err error
			if pair, err = buildGateway(bs, name); err != nil {
				if bs.cfg.PaymentGateway != "" {
					return nil, nil, err // an explicit override must work
				}
				slog.Warn("no mobile-money payments in this country", "country", c.Code, "gateway", name, "reason", err)
			}
			built[name] = pair
		}
		if pair.collect != nil {
			collect[c.Currency] = pair.collect
		}
		if pair.payout != nil {
			payout[c.Currency] = pair.payout
		}
	}
	return payment.NewGateways(collect), payment.NewPayouters(payout), nil
}

// gatewayPair is one provider's collection and payout sides (payouts may be off).
type gatewayPair struct {
	collect payment.Gateway
	payout  payment.Payouter
}

// buildGateway builds a provider by name; credentials missing is an error the caller decides on.
func buildGateway(bs *bootstrap, name string) (gatewayPair, error) {
	switch name {
	case "", "none":
		return gatewayPair{}, fmt.Errorf("no payment gateway named")
	case selcom.Name:
		c, err := selcom.New(selcom.Config{BaseURL: bs.cfg.SelcomBaseURL, APIKey: bs.cfg.SelcomAPIKey, APISecret: bs.cfg.SelcomAPISecret,
			Vendor: bs.cfg.SelcomVendor, BuyerName: bs.brand.DisplayName, BuyerEmail: bs.brand.SupportEmail})
		if err != nil {
			return gatewayPair{}, fmt.Errorf("selcom: %w (set NEXA_SELCOM_BASE_URL, _API_KEY, _API_SECRET, _VENDOR)", err)
		}
		po, err := selcomPayouts(bs)
		return gatewayPair{collect: c, payout: po}, err
	case fake.Name:
		if bs.cfg.PaymentGateway != fake.Name {
			return gatewayPair{}, fmt.Errorf("the fake gateway runs only when NEXA_PAYMENT_GATEWAY=fake")
		}
		delay, err := time.ParseDuration(bs.cfg.FakeGatewayDelay)
		if err != nil {
			return gatewayPair{}, fmt.Errorf("NEXA_FAKE_GATEWAY_DELAY: %w", err)
		}
		slog.Warn("payment gateway is the FAKE adapter: no real money moves; never use in production", "delay", delay)
		f := fake.New(delay)
		return gatewayPair{collect: f, payout: f}, nil
	}
	return gatewayPair{}, fmt.Errorf("unknown payment gateway %q", name)
}

// startMoney wires debt holds and pay-ins: holds re-evaluate whenever a wallet
// moves or a trip ends, pay-ins post to the ledger, and the driver's app hears
// about every change over the websocket.
func startMoney(bs *bootstrap, brandID string, mw moneyWorkers, jobs payments.JobEnqueuer, ledger *money.Ledger, prices *pricing.Service,
	p *presence.Service, hub *realtime.Hub, t *trips.Service, ident *identity.Service) (*holds.Service, *payments.Service, *payouts.Service, error) {
	gws, payouters, err := paymentGateways(bs)
	if err != nil {
		return nil, nil, nil, err
	}
	limits := func(ctx context.Context) (holds.Limits, error) {
		fp, err := prices.FeePolicy(ctx)
		return holds.Limits{Soft: fp.DebtSoftLimit, Hard: fp.DebtHardLimit, GraceDays: fp.DebtGraceDays, MinPayin: fp.PayinMinAmount}, err
	}
	var holdSvc *holds.Service
	var paySvc *payments.Service
	var payoutSvc *payouts.Service
	notify := func(ctx context.Context, driverID string) {
		if hub == nil {
			return
		}
		hub.Publish(ctx, driverID, realtime.NewFrame(realtime.TypeWalletUpdated, walletFrame(ctx, ledger, holdSvc, paySvc, payoutSvc, driverID)))
	}
	var pres holds.Presence
	if p != nil {
		pres = p
	}
	holdSvc = holds.New(bs.pool, brandID, ledger, limits, pres, notify)
	callback := ""
	if bs.cfg.PaymentWebhookURL != "" {
		callback = bs.cfg.PaymentWebhookURL + "/v1/payments/webhooks"
	}
	paySvc = payments.New(payments.Deps{Pool: bs.pool, BrandID: brandID, Country: bs.brand.Country, Gateways: gws, Debts: ledger, Ledger: ledger,
		Holds: holdSvc, Limits: limits, Jobs: jobs, Notify: notify, CallbackURL: callback,
		Fares: fareTrips{t: t}, FareLedger: fareLedger{l: ledger}})
	wireFares(t, paySvc)
	mw.holdSweep.Svc, mw.check.Svc, mw.reconcile.Svc = holdSvc, paySvc, paySvc
	payoutSvc = newPayouts(bs, brandID, payouters, jobs, ledger, prices, holdSvc, ident, notify)
	mw.payoutSend.Svc, mw.payoutCheck.Svc, mw.payoutReconcile.Svc = payoutSvc, payoutSvc, payoutSvc

	ledger.OnDriverWallet(func(ctx context.Context, driverID string) {
		if _, err := holdSvc.Evaluate(ctx, driverID); err != nil {
			slog.Warn("holds: evaluation after wallet movement failed", "driver", driverID, "err", err)
		}
	})
	// A driver held mid-trip finishes it; once the trip ends they go offline.
	t.OnChanged(func(ctx context.Context, trip *trips.Trip) {
		if trip.DriverID != nil && trips.IsTerminal(trip.Status) {
			if err := holdSvc.Enforce(ctx, *trip.DriverID); err != nil {
				slog.Warn("holds: enforce after trip failed", "driver", *trip.DriverID, "err", err)
			}
		}
	})
	return holdSvc, paySvc, payoutSvc, nil
}

// walletFrame is the wallet.updated payload: balance, debt, holds and any open pay-in.
func walletFrame(ctx context.Context, ledger *money.Ledger, h *holds.Service, p *payments.Service, po *payouts.Service, driverID string) map[string]any {
	out := map[string]any{"holds": []holds.Hold{}}
	if w, err := ledger.DriverWallet(ctx, driverID, 0); err == nil {
		out["balance"] = w.Balance
		out["debt"] = decimal.Max(w.Balance.Neg(), decimal.Zero)
	}
	if hs, err := h.Active(ctx, driverID); err == nil {
		out["holds"] = hs
	}
	if pp, err := p.Pending(ctx, driverID); err == nil {
		out["payin"] = pp
	}
	if w, err := po.WalletOf(ctx, driverID); err == nil {
		out["payouts"] = w
	}
	return out
}
