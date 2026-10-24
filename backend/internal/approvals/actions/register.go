package actions

import (
	"github.com/kalen-limited/hauler-rider/backend/internal/approvals"
	"github.com/kalen-limited/hauler-rider/backend/internal/branding"
	"github.com/kalen-limited/hauler-rider/backend/internal/compliance"
	"github.com/kalen-limited/hauler-rider/backend/internal/identity"
	"github.com/kalen-limited/hauler-rider/backend/internal/payouts"
	"github.com/kalen-limited/hauler-rider/backend/internal/privacy"
	"github.com/kalen-limited/hauler-rider/backend/internal/profiles"
	"github.com/kalen-limited/hauler-rider/backend/internal/refunds"
)

// Modules are the services the actions call; a nil module registers no actions.
type Modules struct {
	Profiles   *profiles.Service
	Refunds    *refunds.Service
	Payouts    *payouts.Service
	Privacy    *privacy.Service
	Compliance *compliance.Service
	Identity   *identity.Service
	Branding   *branding.Service
}

// Register adds the Platform's approvable actions to a catalogue.
func Register(c *approvals.Catalogue, m Modules) {
	if m.Profiles != nil {
		registerPricing(c, m.Profiles)
	}
	if m.Refunds != nil {
		c.Register(refundAction(m.Refunds))
	}
	if m.Payouts != nil {
		c.Register(payoutAction(m.Payouts))
	}
	if m.Privacy != nil {
		c.Register(privacyAction(m.Privacy))
	}
	if m.Compliance != nil {
		c.Register(regulatorAction(m.Compliance))
	}
	if m.Identity != nil {
		c.Register(staffRolesAction(m.Identity))
	}
	if m.Branding != nil {
		c.Register(brandingAction(m.Branding))
	}
}
