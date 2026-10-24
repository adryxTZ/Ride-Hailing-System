package actions

import (
	"encoding/json"

	"github.com/shopspring/decimal"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
	"github.com/kalen-limited/hauler-rider/backend/internal/profiles"
)

// Action keys (stable: stored on workflows and requests).
const (
	ProfilePublish    = "pricing.profile_publish"
	ProfileStatus     = "pricing.profile_status"
	ProfileDefault    = "pricing.profile_default"
	ProfileAssign     = "pricing.profile_assign"
	TripRefund        = "trips.refund"
	PayoutApprove     = "payouts.approve"
	PrivacyDelete     = "privacy.delete"
	RegulatorSettings = "compliance.regulator"
	StaffRoles        = "staff.roles"
	BrandingPublish   = "branding.publish"
)

// Catalogue groups (the editor groups actions by these).
const (
	groupPricing  = "Pricing"
	groupMoney    = "Money"
	groupTrust    = "Trust"
	groupStaff    = "Staff"
	groupSettings = "Settings"
)

// ProfileVersionPayload publishes a new version of a pricing profile.
type ProfileVersionPayload struct {
	ProfileID string                `json:"profile_id"`
	Version   profiles.VersionInput `json:"version"`
}

// ProfileStatusPayload switches a profile ACTIVE or INACTIVE.
type ProfileStatusPayload struct {
	ProfileID string `json:"profile_id"`
	Status    string `json:"status"`
}

// ProfileDefaultPayload makes a profile its country's default.
type ProfileDefaultPayload struct {
	ProfileID string `json:"profile_id"`
}

// ProfileAssignPayload assigns a profile to a region, district, zone or place.
type ProfileAssignPayload struct {
	ProfileID string               `json:"profile_id"`
	Assign    profiles.AssignInput `json:"assign"`
}

// RefundPayload refunds part or all of a completed trip.
type RefundPayload struct {
	TripID  string          `json:"trip_id"`
	Amount  decimal.Decimal `json:"amount"`
	BorneBy string          `json:"borne_by"`
}

// PayoutPayload approves a driver cash-out.
type PayoutPayload struct {
	PayoutID string `json:"payout_id"`
}

// PrivacyPayload approves an account deletion request.
type PrivacyPayload struct {
	RequestID string `json:"request_id"`
}

// RegulatorPayload changes a country's regulator switches.
type RegulatorPayload struct {
	Country    string `json:"country"`
	Enabled    *bool  `json:"enabled,omitempty"`
	FailClosed *bool  `json:"fail_closed,omitempty"`
}

// StaffRolesPayload sets a person's staff roles (no password is ever stored here).
type StaffRolesPayload struct {
	Phone    string   `json:"phone"`
	FullName string   `json:"full_name"`
	Roles    []string `json:"roles"`
}

// BrandingPayload publishes a theme draft.
type BrandingPayload struct {
	ThemeID string `json:"theme_id"`
}

// errPayload is returned when a stored payload cannot be read.
var errPayload = apperr.Validation

// decode reads a stored payload into its type.
func decode[T any](raw json.RawMessage) (T, error) {
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		return v, errPayload
	}
	return v, nil
}
