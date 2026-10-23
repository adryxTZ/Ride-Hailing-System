package compliance

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"
)

// Document expiry rules.
const (
	DocumentSweepInterval = time.Hour
	// reminderHour is the local hour from which reminders go out (not at night).
	reminderHour = 8
)

// ReminderDays are the days before expiry a driver is reminded, tightest last.
var ReminderDays = []int{30, 7, 1}

// DocumentHolds holds or frees a driver over expired documents (the holds module).
type DocumentHolds func(ctx context.Context, driverID string, expired []string) error

// Reminder tells a driver a document is about to expire (push and SMS).
type Reminder func(ctx context.Context, driverID, phone, text string) error

// trackedDoc is a current document of an approved driver whose type tracks expiry.
type trackedDoc struct {
	ID, DriverID, Phone, Kind, Country string
	Label                              []byte
	ExpiresOn                          *time.Time
	Blocks, HasRenewal                 bool
}

// ReminderDue is the reminder threshold to send now, or 0: the tightest of
// ReminderDays at or above daysLeft that was not sent yet.
func ReminderDue(daysLeft int, sent []int) int {
	if daysLeft < 0 {
		return 0
	}
	due := 0
	for _, d := range ReminderDays {
		if daysLeft <= d && !containsInt(sent, d) {
			due = d
		}
	}
	return due
}

func containsInt(v []int, x int) bool {
	for _, y := range v {
		if y == x {
			return true
		}
	}
	return false
}

// SweepDocuments reminds drivers of documents about to expire and holds those
// whose blocking documents expired (frees them once renewed). Hourly.
func (s *Service) SweepDocuments(ctx context.Context) error {
	docs, err := s.store.trackedDocuments(ctx)
	if err != nil {
		return err
	}
	byDriver := map[string][]trackedDoc{}
	for _, d := range docs {
		byDriver[d.DriverID] = append(byDriver[d.DriverID], d)
	}
	for driverID, ds := range byDriver {
		if err := s.evaluateDriver(ctx, driverID, ds); err != nil {
			slog.Warn("compliance: document check failed", "driver", driverID, "err", err)
		}
	}
	return s.releaseCleared(ctx, byDriver)
}

// EvaluateDriverDocuments re-checks one driver at once (after a renewal is approved or staff fix a date).
func (s *Service) EvaluateDriverDocuments(ctx context.Context, driverID string) {
	docs, err := s.store.trackedDocumentsOf(ctx, driverID)
	if err == nil {
		err = s.evaluateDriver(ctx, driverID, docs)
	}
	if err != nil {
		slog.Warn("compliance: document check failed", "driver", driverID, "err", err)
	}
}

// evaluateDriver applies the hold rule and sends at most one due reminder per document.
func (s *Service) evaluateDriver(ctx context.Context, driverID string, docs []trackedDoc) error {
	var expired []string
	for _, d := range docs {
		left, ok := s.daysLeft(d)
		if !ok {
			continue
		}
		if left < 0 && d.Blocks {
			expired = append(expired, d.Kind)
		}
		if err := s.remind(ctx, d, left); err != nil {
			slog.Warn("compliance: reminder not sent", "document", d.ID, "err", err)
		}
	}
	sort.Strings(expired)
	if s.documentHolds == nil {
		return nil
	}
	return s.documentHolds(ctx, driverID, expired)
}

// releaseCleared frees held drivers who no longer have any tracked document (e.g. a type turned off).
func (s *Service) releaseCleared(ctx context.Context, seen map[string][]trackedDoc) error {
	held, err := s.store.documentHeldDrivers(ctx)
	if err != nil || s.documentHolds == nil {
		return err
	}
	for _, id := range held {
		if _, ok := seen[id]; !ok {
			if err := s.documentHolds(ctx, id, nil); err != nil {
				slog.Warn("compliance: document hold not lifted", "driver", id, "err", err)
			}
		}
	}
	return nil
}

// daysLeft counts whole days from the country's local today to the expiry date.
func (s *Service) daysLeft(d trackedDoc) (int, bool) {
	if d.ExpiresOn == nil {
		return 0, false
	}
	today := s.localDay(d.Country)
	exp := time.Date(d.ExpiresOn.Year(), d.ExpiresOn.Month(), d.ExpiresOn.Day(), 0, 0, 0, 0, time.UTC)
	return int(exp.Sub(today).Hours() / 24), true
}

func (s *Service) localDay(country string) time.Time {
	tz := "UTC"
	for _, c := range s.countries {
		if c.Code == country {
			tz = c.Timezone
		}
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	n := s.now().In(loc)
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
}

// remind sends the due reminder once (recorded before sending); a pending renewal suppresses reminders.
func (s *Service) remind(ctx context.Context, d trackedDoc, left int) error {
	if s.reminder == nil || d.HasRenewal || s.localHour(d.Country) < reminderHour {
		return nil
	}
	sent, err := s.store.remindersSent(ctx, d.ID)
	if err != nil {
		return err
	}
	due := ReminderDue(left, sent)
	if due == 0 {
		return nil
	}
	first, err := s.store.markReminder(ctx, d.ID, due)
	if err != nil || !first {
		return err
	}
	return s.reminder(ctx, d.DriverID, d.Phone, reminderText(labelEN(d.Label), left))
}

func (s *Service) localHour(country string) int {
	for _, c := range s.countries {
		if c.Code == country {
			if loc, err := time.LoadLocation(c.Timezone); err == nil {
				return s.now().In(loc).Hour()
			}
		}
	}
	return s.now().UTC().Hour()
}

func reminderText(label string, left int) string {
	switch left {
	case 0:
		return fmt.Sprintf("Your %s expires today. Upload the renewed one in the app to keep driving.", label)
	case 1:
		return fmt.Sprintf("Your %s expires tomorrow. Upload the renewed one in the app to keep driving.", label)
	}
	return fmt.Sprintf("Your %s expires in %d days. Upload the renewed one in the app before then.", label, left)
}
