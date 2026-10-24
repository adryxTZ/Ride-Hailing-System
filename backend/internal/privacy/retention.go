package privacy

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
)

// Rule is one retention rule: data older than Days is deleted or stripped by Run.
type Rule struct {
	Name string
	Days int
	Run  func(ctx context.Context, cutoff time.Time) (int, error)
}

// defaultDays are the retention periods when the brand file sets none (spec §5).
var defaultDays = map[string]int{
	"otp_codes": 1, "sessions": 30, "trip_messages": 180, "share_tokens": 1, "gps_evidence": 90,
	"fraud_flags_dismissed": 365, "devices_installs": 365, "regulator_payloads": 90,
	"kyc_rejected": 180, "kyc_closed": 730, "documents_archived": 730, "privacy_exports": 7,
}

// RuleNames lists the known rules (brand-file validation).
func RuleNames() map[string]int { return defaultDays }

// retentionRules builds the registry with the brand's day overrides.
func (s *Service) retentionRules(overrides map[string]int) []Rule {
	days := func(name string) int {
		if d, ok := overrides[name]; ok {
			return d
		}
		return defaultDays[name]
	}
	sqlRule := func(name, sql string) Rule {
		return Rule{Name: name, Days: days(name), Run: func(ctx context.Context, cutoff time.Time) (int, error) { return s.batched(ctx, sql, cutoff) }}
	}
	return []Rule{
		sqlRule("otp_codes", `DELETE FROM otp_codes WHERE ctid IN (SELECT ctid FROM otp_codes WHERE brand_id=$3 AND expires_at < $1 LIMIT $2)`),
		sqlRule("sessions", `WITH gone AS (DELETE FROM refresh_tokens WHERE id IN (SELECT r.id FROM refresh_tokens r JOIN users u ON u.id=r.user_id WHERE u.brand_id=$3 AND coalesce(r.revoked_at, r.expires_at) < $1 LIMIT $2) RETURNING 1) SELECT count(*) FROM gone`),
		sqlRule("trip_messages", `DELETE FROM trip_messages WHERE id IN (SELECT m.id FROM trip_messages m JOIN trips t ON t.id=m.trip_id
			WHERE m.brand_id=$3 AND coalesce(t.completed_at, t.cancelled_at) < $1 LIMIT $2)`),
		sqlRule("share_tokens", `UPDATE trips SET share_token=NULL WHERE id IN (SELECT id FROM trips WHERE brand_id=$3 AND share_token IS NOT NULL
			AND coalesce(completed_at, cancelled_at) < $1 LIMIT $2)`),
		sqlRule("gps_evidence", `UPDATE fraud_flags SET evidence = (evidence - 'pairs') || '{"pairs_removed": true}' WHERE id IN (SELECT id FROM fraud_flags
			WHERE brand_id=$3 AND rule='GPS_SPOOF' AND evidence ? 'pairs' AND opened_at < $1 LIMIT $2)`),
		sqlRule("fraud_flags_dismissed", `DELETE FROM fraud_flags WHERE id IN (SELECT id FROM fraud_flags WHERE brand_id=$3 AND status='DISMISSED' AND reviewed_at < $1 LIMIT $2)`),
		sqlRule("devices_installs", `WITH d AS (DELETE FROM devices WHERE id IN (SELECT d.id FROM devices d JOIN users u ON u.id=d.user_id WHERE u.brand_id=$3 AND d.last_seen_at < $1 LIMIT $2) RETURNING 1),
			i AS (DELETE FROM user_installs WHERE (user_id, install_id) IN (SELECT i.user_id, i.install_id FROM user_installs i JOIN users u ON u.id=i.user_id WHERE u.brand_id=$3 AND i.last_seen_at < $1 LIMIT $2) RETURNING 1)
			SELECT (SELECT count(*) FROM d) + (SELECT count(*) FROM i)`),
		sqlRule("regulator_payloads", `UPDATE regulator_submissions SET payload = '{"removed": true}' WHERE id IN (SELECT id FROM regulator_submissions
			WHERE brand_id=$3 AND status='ACCEPTED' AND accepted_at < $1 AND payload IS NOT NULL AND NOT payload ? 'removed' LIMIT $2)`),
		s.fileRule("kyc_rejected", days("kyc_rejected"), `SELECT d.id, d.file_key FROM documents d JOIN driver_profiles p ON p.user_id=d.owner_id
			WHERE d.brand_id=$3 AND d.purged_at IS NULL AND p.status IN ('REJECTED','ONBOARDING') AND p.updated_at < $1 AND d.created_at < $1 LIMIT $2`),
		s.fileRule("kyc_closed", days("kyc_closed"), `SELECT d.id, d.file_key FROM documents d JOIN users u ON u.id=d.owner_id
			WHERE d.brand_id=$3 AND d.purged_at IS NULL AND u.status='DELETED' AND u.deleted_at < $1 LIMIT $2`),
		s.fileRule("documents_archived", days("documents_archived"), `SELECT id, file_key FROM documents WHERE brand_id=$3 AND purged_at IS NULL AND archived_at < $1 LIMIT $2`),
		{Name: "privacy_exports", Days: days("privacy_exports"), Run: s.purgeExports},
	}
}

// batched runs a bounded statement until it touches nothing (rows per pass ≤ RetentionBatch).
// Retention may strip and delete reviewed fraud flags; the guard trigger checks nexa.retention.
func (s *Service) batched(ctx context.Context, sql string, cutoff time.Time) (int, error) {
	total := 0
	for {
		var n int64
		err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `SET LOCAL nexa.retention = 'on'`); err != nil {
				return err
			}
			if isSelectCount(sql) {
				return tx.QueryRow(ctx, sql, cutoff, RetentionBatch, s.store.brandID).Scan(&n)
			}
			tag, err := tx.Exec(ctx, sql, cutoff, RetentionBatch, s.store.brandID)
			n = tag.RowsAffected()
			return err
		})
		if err != nil {
			return total, err
		}
		total += int(n)
		if n < RetentionBatch {
			return total, nil
		}
	}
}

func isSelectCount(sql string) bool { return len(sql) > 4 && sql[:4] == "WITH" }

// fileRule deletes document files past their period and marks the rows purged (the row stays).
func (s *Service) fileRule(name string, days int, pick string) Rule {
	return Rule{Name: name, Days: days, Run: func(ctx context.Context, cutoff time.Time) (int, error) {
		if s.d.Writer == nil {
			return 0, nil
		}
		total := 0
		for {
			rows, err := s.store.pickFiles(ctx, pick, cutoff, RetentionBatch)
			if err != nil || len(rows) == 0 {
				return total, err
			}
			for _, r := range rows {
				if err := s.d.Writer.Delete(ctx, r.key); err != nil {
					return total, err
				}
				if err := s.store.markPurged(ctx, r.id); err != nil {
					return total, err
				}
				total++
			}
			if len(rows) < RetentionBatch {
				return total, nil
			}
		}
	}}
}

// purgeExports deletes export files older than the period and closes their requests.
func (s *Service) purgeExports(ctx context.Context, cutoff time.Time) (int, error) {
	if s.d.Writer == nil {
		return 0, nil
	}
	rows, err := s.store.oldExports(ctx, cutoff, RetentionBatch)
	if err != nil {
		return 0, err
	}
	done := 0
	for _, r := range rows { // one file that cannot be deleted does not stop the others
		if err := s.d.Writer.Delete(ctx, r.key); err != nil {
			slog.Warn("retention: delete export", "request", r.id, "err", err)
			continue
		}
		if err := s.store.closeExport(ctx, r.id); err != nil {
			return done, err
		}
		done++
	}
	return done, nil
}

// RunRetention runs every rule once per local day, after RetentionHour.
func (s *Service) RunRetention(ctx context.Context) error {
	local := s.now().In(s.d.Location)
	if local.Hour() < RetentionHour {
		return nil
	}
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	for _, r := range s.rules {
		claimed, err := s.store.claimRun(ctx, r.Name, day)
		if err != nil {
			return err
		}
		if !claimed {
			continue
		}
		n, runErr := r.Run(ctx, s.now().Add(-time.Duration(r.Days)*24*time.Hour))
		if runErr != nil {
			slog.Error("retention rule failed", "rule", r.Name, "err", runErr)
		}
		if err := s.store.finishRun(ctx, r.Name, day, n, runErr); err != nil {
			return fmt.Errorf("retention %s: %w", r.Name, err)
		}
	}
	return nil
}

// Retention lists the rules with their last run (admin).
func (s *Service) Retention(ctx context.Context) ([]RetentionStatus, error) {
	last, err := s.store.lastRuns(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]RetentionStatus, 0, len(s.rules))
	for _, r := range s.rules {
		st := last[r.Name]
		st.Name, st.Days = r.Name, r.Days
		out = append(out, st)
	}
	return out, nil
}
