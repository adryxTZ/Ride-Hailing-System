package areas

import (
	"context"

	"github.com/kalen-limited/hauler-rider/backend/internal/audit"
)

// Auditor records staff decisions in the same transaction as the change (audit.Record).
type Auditor func(ctx context.Context, db audit.Execer, brandID, actor, action, target, reason string, data any) error
