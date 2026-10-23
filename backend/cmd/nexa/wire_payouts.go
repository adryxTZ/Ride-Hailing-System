package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/kalen-limited/hauler-rider/backend/internal/holds"
	"github.com/kalen-limited/hauler-rider/backend/internal/identity"
	"github.com/kalen-limited/hauler-rider/backend/internal/money"
	"github.com/kalen-limited/hauler-rider/backend/internal/payouts"
	"github.com/kalen-limited/hauler-rider/backend/internal/pricing"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/payment"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/payment/selcom"
)

// selcomPayouts is Selcom's Business API payout client, or nil when its
// credentials are not set (payouts are then off in Selcom countries).
func selcomPayouts(bs *bootstrap) (payment.Payouter, error) {
	if bs.cfg.SelcomPayoutBaseURL == "" && bs.cfg.SelcomPayoutAPIKey == "" && bs.cfg.SelcomPayoutKeyFile == "" {
		slog.Warn("selcom payouts are off: set NEXA_SELCOM_PAYOUT_BASE_URL, _API_KEY and _KEY_FILE to pay drivers")
		return nil, nil
	}
	pem, err := os.ReadFile(bs.cfg.SelcomPayoutKeyFile)
	if err != nil {
		return nil, fmt.Errorf("payout key: %w", err)
	}
	p, err := selcom.NewPayouts(selcom.PayoutConfig{BaseURL: bs.cfg.SelcomPayoutBaseURL, APIKey: bs.cfg.SelcomPayoutAPIKey, PrivateKeyPEM: pem})
	if errors.Is(err, payment.ErrNotConfigured) {
		return nil, fmt.Errorf("payouts: %w (set NEXA_SELCOM_PAYOUT_BASE_URL, _API_KEY and _KEY_FILE together)", err)
	}
	return p, err
}

// newPayouts builds the payouts service: rules from the fee policy in force,
// staff holds block cash-out, money goes to the account phone only.
func newPayouts(bs *bootstrap, brandID string, gws *payment.Payouters, jobs payouts.JobEnqueuer, ledger *money.Ledger,
	prices *pricing.Service, h *holds.Service, ident *identity.Service, notify payouts.Notifier) *payouts.Service {
	rules := func(ctx context.Context) (payouts.Rules, error) {
		fp, err := prices.FeePolicy(ctx)
		return payouts.Rules{Min: fp.PayoutMinAmount, AutoApprove: fp.PayoutAutoApprove}, err
	}
	blocked := func(ctx context.Context, driverID string) (bool, error) {
		active, err := h.Active(ctx, driverID)
		for _, a := range active {
			if a.Kind == holds.KindManual {
				return true, err
			}
		}
		return false, err
	}
	person := func(ctx context.Context, driverID string) (string, string, error) {
		phone, err := ident.Phone(ctx, driverID)
		if err != nil {
			return "", "", err
		}
		names, _ := ident.Names(ctx, []string{driverID})
		return phone, names[driverID], nil
	}
	return payouts.New(payouts.Deps{Pool: bs.pool, BrandID: brandID, Gateways: gws, Ledger: ledger, Rules: rules,
		Blocked: blocked, Person: person, Jobs: jobs, Notify: notify})
}
