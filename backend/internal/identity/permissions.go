package identity

// Permissions a staff role can hold. Routes check these, never role names, so a
// Super Admin can build any role from them. "*" (Super Admin) holds them all.
const (
	PermAll              = "*"
	PermTripsCancel      = "trips.cancel"
	PermRidersBlock      = "riders.block"
	PermDriversReview    = "drivers.review"
	PermPricingEdit      = "pricing.edit"
	PermAreasEdit        = "areas.edit"
	PermFeesEdit         = "fees.edit"
	PermFinanceView      = "finance.view"
	PermHoldsManage      = "holds.manage"
	PermPayoutsApprove   = "payouts.approve"
	PermComplianceManage = "compliance.manage"
	PermSafetyResolve    = "safety.resolve"
	PermStaffManage      = "staff.manage"
	PermBrandingManage   = "branding.manage"
)

// PermissionPrefix marks permissions inside the token's roles claim ("perm:pricing.edit").
const PermissionPrefix = "perm:"

// Permission describes one permission for the roles editor.
type Permission struct {
	Key   string `json:"key"`
	Group string `json:"group"`
	Label string `json:"label"`
}

// Permissions is the catalogue, in the order the roles editor shows it.
var Permissions = []Permission{
	{PermTripsCancel, "Operations", "End trips on behalf of riders or drivers"},
	{PermRidersBlock, "Operations", "Block and unblock riders"},
	{PermDriversReview, "Operations", "Review driver documents, approve, reject, suspend; set driver regions"},
	{PermSafetyResolve, "Operations", "Close safety (SOS) alerts"},
	{PermPricingEdit, "Pricing", "Change vehicle type prices and fixed fares"},
	{PermAreasEdit, "Pricing", "Create regions and mark them on the map"},
	{PermFeesEdit, "Pricing", "Publish fees and tax rules"},
	{PermFinanceView, "Money", "See finance: ledger, balances, pay-ins"},
	{PermHoldsManage, "Money", "Place and release driver holds"},
	{PermPayoutsApprove, "Money", "Approve or reject driver cash-outs"},
	{PermComplianceManage, "Trust", "Regulator submissions and switches, driver documents"},
	{PermStaffManage, "Staff", "Manage staff, roles, sessions and account status"},
	{PermBrandingManage, "Settings", "Change the brand name, contacts, colours, logos and icons"},
}

// knownPermission reports whether a key is in the catalogue.
func knownPermission(key string) bool {
	for _, p := range Permissions {
		if p.Key == key {
			return true
		}
	}
	return false
}
