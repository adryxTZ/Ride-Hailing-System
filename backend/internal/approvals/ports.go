package approvals

import "context"

// Staff is what this module needs from the identity module.
type Staff interface {
	// Grants are a person's current staff roles and permissions (fresh, not the token's).
	Grants(ctx context.Context, userID string) (Grants, error)
	// Names maps user ids to display names in one query.
	Names(ctx context.Context, ids []string) (map[string]string, error)
}

// RuleSource lists what workflow definitions may refer to: roles, permissions, countries.
type RuleSource func(ctx context.Context) (Rules, error)
