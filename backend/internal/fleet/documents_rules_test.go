package fleet

import (
	"errors"
	"testing"
	"time"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

var today = time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)

func str(s string) *string { return &s }

func TestExpiryState(t *testing.T) {
	cases := []struct {
		tracked bool
		on      *string
		want    string
		left    int
	}{
		{false, str("2026-10-01"), ExpiryNotTracked, 0},
		{true, nil, ExpiryUnknown, 0},
		{true, str("2026-10-23"), ExpiryExpiring, 0}, // valid through its date
		{true, str("2026-10-22"), ExpiryExpired, -1},
		{true, str("2026-11-22"), ExpiryExpiring, 30},
		{true, str("2026-11-23"), ExpiryValid, 31},
	}
	for _, c := range cases {
		got, left := ExpiryState(c.tracked, c.on, today)
		if got != c.want || (left != nil && *left != c.left) {
			t.Errorf("%v %v: got %s %v", c.tracked, c.on, got, left)
		}
	}
}

func TestUploadExpiryRules(t *testing.T) {
	licence := &DocumentType{TracksExpiry: true, ExpiryRequired: true}
	police := &DocumentType{TracksExpiry: true}
	photo := &DocumentType{}
	if _, err := checkExpiry(licence, nil, today); !isField(err, "REQUIRED") {
		t.Fatalf("required: %v", err)
	}
	if _, err := checkExpiry(licence, str("2026-10-22"), today); !isField(err, "PAST") {
		t.Fatalf("past: %v", err)
	}
	if _, err := checkExpiry(licence, str("2037-01-01"), today); !isField(err, "TOO_FAR") {
		t.Fatalf("too far: %v", err)
	}
	if v, err := checkExpiry(police, nil, today); err != nil || v != nil {
		t.Fatal("optional expiry may be left out")
	}
	if v, _ := checkExpiry(photo, str("2030-01-01"), today); v != nil {
		t.Fatal("kinds that do not track expiry drop the date")
	}
}

func isField(err error, code string) bool {
	var e *apperr.E
	if !errors.As(err, &e) {
		return false
	}
	fields, _ := e.Details["fields"].([]map[string]string)
	return len(fields) == 1 && fields[0]["code"] == code
}

func TestChecklistFollowsTheCountrysChoices(t *testing.T) {
	types := []DocumentType{
		{Kind: "LICENCE", Owner: OwnerDriver, Requirement: RequireMandatory},
		{Kind: "POLICE_CLEARANCE", Owner: OwnerDriver, Requirement: RequireOptional},
		{Kind: "INSPECTION", Owner: OwnerVehicle, Requirement: RequireOff},
	}
	name, lic, nid := "A", "1", "2"
	reqs, missing := checklist(types, Profile{FullName: &name, LicenceNumber: &lic, NationalID: &nid}, &Vehicle{}, nil)
	if len(reqs) != 2 || !reqs[0].Mandatory || reqs[1].Mandatory {
		t.Fatalf("OFF is hidden, optional shown: %+v", reqs)
	}
	if len(missing) != 1 || missing[0] != "LICENCE" {
		t.Fatalf("only mandatory documents block: %v", missing)
	}
}
