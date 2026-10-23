package identity

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// Clock lets tests control time.
type Clock interface{ Now() time.Time }

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// TokenIssuer signs access tokens (implemented by kernel/auth.KeySet).
type TokenIssuer interface {
	Issue(subject, brand, app, session string, roles []string, now time.Time) (string, error)
}

// JobEnqueuer inserts a job in the caller's transaction (implemented by the
// River client). Keeping it an interface lets unit tests skip the queue.
type JobEnqueuer interface {
	InsertTx(ctx context.Context, tx pgx.Tx, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}
