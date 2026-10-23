package fleet

import (
	"context"
	"strings"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/scope"

	"github.com/jackc/pgx/v5"
)

// Staff list page bounds.
const (
	DefaultPageSize = 50
	MaxPageSize     = 200
	auditPageSize   = 100
)

// DriverPage is one page of the staff driver list.
type DriverPage struct {
	Data   []DriverRow    `json:"data"`
	Total  int            `json:"total"`
	Counts map[string]int `json:"counts"`
}

// DriverDetail is the staff view of one driver: the checklist with viewable files and the decision history.
type DriverDetail struct {
	Onboarding
	History []AuditEntry `json:"history"`
}

// ListDrivers is the staff list (pending reviews first) for one country and,
// optionally, one area (drivers registered there). An empty scope lists everyone.
func (s *Service) ListDrivers(ctx context.Context, status, q string, sc scope.Scope, limit, offset int) (*DriverPage, error) {
	if limit <= 0 || limit > MaxPageSize {
		limit = DefaultPageSize
	}
	rows, total, err := s.store.listDrivers(ctx, strings.ToUpper(status), strings.TrimSpace(q), sc, limit, max(0, offset))
	if err != nil {
		return nil, err
	}
	counts, err := s.store.statusCounts(ctx, sc)
	if err != nil {
		return nil, err
	}
	return &DriverPage{Data: rows, Total: total, Counts: counts}, nil
}

// Detail returns a driver's application with short-lived links to each document.
func (s *Service) Detail(ctx context.Context, driverID string) (*DriverDetail, error) {
	ob, err := s.assemble(ctx, driverID)
	if err != nil {
		return nil, err
	}
	for i := range ob.Documents {
		if url, err := s.files.DownloadURL(ctx, ob.Documents[i].FileKey, viewTTL); err == nil {
			ob.Documents[i].URL = url
		}
	}
	history, err := s.store.auditFor(ctx, "driver:"+driverID, "", "", auditPageSize)
	if err != nil {
		return nil, err
	}
	return &DriverDetail{Onboarding: *ob, History: history}, nil
}

// Approve approves the driver, their vehicle and every pending document.
func (s *Service) Approve(ctx context.Context, staffID, driverID string) (*DriverDetail, error) {
	ob, err := s.assemble(ctx, driverID)
	if err != nil {
		return nil, err
	}
	if len(ob.Missing) > 0 {
		return nil, ErrIncomplete.WithDetails(map[string]any{"missing": ob.Missing})
	}
	err = pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		owners := []string{driverID, ob.Vehicle.ID}
		if err := s.store.approvePending(ctx, tx, owners, staffID); err != nil {
			return err
		}
		if err := s.store.setVehicleStatus(ctx, tx, ob.Vehicle.ID, ReviewApproved, nil); err != nil {
			return err
		}
		if err := s.store.setStatus(ctx, tx, driverID, StatusApproved, nil, &staffID); err != nil {
			return err
		}
		if err := s.approvedTx(ctx, tx, driverID, ob.Vehicle.ID); err != nil {
			return err
		}
		return s.store.audit(ctx, tx, staffID, "driver.approve", "driver:"+driverID, nil, map[string]string{"plate": ob.Vehicle.Plate})
	})
	if err != nil {
		return nil, err
	}
	return s.Detail(ctx, driverID)
}

// Reject sends the application back to the driver with a reason to fix.
func (s *Service) Reject(ctx context.Context, staffID, driverID, reason string) (*DriverDetail, error) {
	return s.decide(ctx, staffID, driverID, StatusRejected, "driver.reject", reason)
}

// Suspend stops an approved driver from working and takes them offline.
func (s *Service) Suspend(ctx context.Context, staffID, driverID, reason string) (*DriverDetail, error) {
	d, err := s.decide(ctx, staffID, driverID, StatusSuspended, "driver.suspend", reason)
	if err == nil && s.offline != nil {
		_ = s.offline(ctx, driverID)
	}
	return d, err
}

// Reinstate lifts a suspension.
func (s *Service) Reinstate(ctx context.Context, staffID, driverID, reason string) (*DriverDetail, error) {
	return s.decide(ctx, staffID, driverID, StatusApproved, "driver.reinstate", reason)
}

func (s *Service) decide(ctx context.Context, staffID, driverID, status, action, reason string) (*DriverDetail, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, ErrReasonRequired
	}
	p, err := s.store.profile(ctx, driverID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, ErrDriverNotFound
	}
	err = pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		if err := s.store.setStatus(ctx, tx, driverID, status, &reason, &staffID); err != nil {
			return err
		}
		return s.store.audit(ctx, tx, staffID, action, "driver:"+driverID, &reason, map[string]string{"from": p.Status})
	})
	if err != nil {
		return nil, err
	}
	return s.Detail(ctx, driverID)
}

// ReviewDocument approves or rejects one document (rejection needs a reason the driver sees).
func (s *Service) ReviewDocument(ctx context.Context, staffID, driverID, docID string, approve bool, reason string) (*DriverDetail, error) {
	status, why := ReviewApproved, (*string)(nil)
	if !approve {
		reason = strings.TrimSpace(reason)
		if reason == "" {
			return nil, ErrReasonRequired
		}
		status, why = ReviewRejected, &reason
	}
	err := pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		if _, err := s.store.reviewDocument(ctx, tx, docID, status, why, &staffID); err != nil {
			return err
		}
		if approve {
			if err := s.store.promoteRenewalTx(ctx, tx, docID); err != nil {
				return err
			}
		}
		return s.store.audit(ctx, tx, staffID, "document."+strings.ToLower(status), "driver:"+driverID, why, map[string]string{"document": docID})
	})
	if err != nil {
		return nil, err
	}
	s.documentsChanged(ctx, driverID)
	return s.Detail(ctx, driverID)
}

// Audit lists recent staff decisions (all targets when target is "").
func (s *Service) Audit(ctx context.Context, target, actionPrefix, actorID string, limit int) ([]AuditEntry, error) {
	if limit <= 0 || limit > MaxPageSize {
		limit = auditPageSize
	}
	return s.store.auditFor(ctx, target, actionPrefix, actorID, limit)
}
