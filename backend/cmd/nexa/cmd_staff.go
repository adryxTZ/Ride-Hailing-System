package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/kalen-limited/hauler-rider/backend/internal/catalog"
	"github.com/kalen-limited/hauler-rider/backend/internal/fleet"
	"github.com/kalen-limited/hauler-rider/backend/internal/identity"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/auth"
)

// runStaff: nexa staff add <phone> --name "…" --roles "Super Admin,Finance Officer" --email a@b.c --password secret
func runStaff(args []string) error {
	if len(args) < 2 || args[0] != "add" {
		return errors.New(`usage: nexa staff add <phone> --name "Full Name" --roles "Super Admin" --email you@example.com --password secret`)
	}
	fs := flag.NewFlagSet("staff add", flag.ContinueOnError)
	name := fs.String("name", "", "full name")
	roles := fs.String("roles", "Super Admin", "comma-separated staff roles")
	email := fs.String("email", "", "dashboard sign-in email")
	password := fs.String("password", "", "dashboard password (at least 6 characters)")
	if err := fs.Parse(args[2:]); err != nil {
		return err
	}
	ctx := context.Background()
	ident, _, err := identityForCLI(ctx)
	if err != nil {
		return err
	}
	u, err := ident.AddStaff(ctx, args[1], *name, strings.Split(*roles, ","))
	if err != nil {
		return err
	}
	if *email != "" || *password != "" {
		if err := ident.SetStaffCredentials(ctx, u.ID, *email, *password); err != nil {
			return err
		}
	}
	fmt.Printf("staff account ready: %s (%s) roles=%s email=%s\n", u.Phone, u.ID, *roles, *email)
	return nil
}

// runDriver: nexa driver provision <phone> --service taxi --class standard --plate T123ABC --name "…"
// Creates an APPROVED driver and vehicle without documents (test/operator tooling).
func runDriver(args []string) error {
	if len(args) < 2 || args[0] != "provision" {
		return errors.New("usage: nexa driver provision <phone> --service <key> --class <key> --plate <plate> [--name ...]")
	}
	fs := flag.NewFlagSet("driver provision", flag.ContinueOnError)
	service := fs.String("service", "taxi", "service key")
	class := fs.String("class", "standard", "vehicle class key")
	plate := fs.String("plate", "", "number plate")
	name := fs.String("name", "Test Driver", "full name")
	if err := fs.Parse(args[2:]); err != nil {
		return err
	}
	ctx := context.Background()
	ident, bs, err := identityForCLI(ctx)
	if err != nil {
		return err
	}
	u, err := ident.EnsureUser(ctx, args[1], identity.RoleDriver, *name)
	if err != nil {
		return err
	}
	brandID, _ := bs.brandID(ctx)
	svc := fleet.New(bs.pool, brandID, catalog.New(bs.pool, brandID), fileStore(bs), nil)
	if *plate == "" {
		*plate = "TST" + strings.ToUpper(u.ID[:4])
	}
	ob, err := svc.Provision(ctx, nil, u.ID,
		fleet.ProfileInput{FullName: *name, LicenceNumber: testLicence(u.ID), NationalID: "TEST-" + u.ID[:8]},
		fleet.VehicleInput{ServiceKey: *service, ClassKey: *class, Plate: *plate})
	if err != nil {
		return err
	}
	fmt.Printf("driver %s approved: %s %s/%s (%s)\n", u.Phone, ob.Vehicle.Plate, ob.Vehicle.ServiceKey, ob.Vehicle.ClassKey, ob.Profile.Status)
	return nil
}

// identityForCLI builds the identity service for operator commands (no token issuing needed).
func identityForCLI(ctx context.Context) (*identity.Service, *bootstrap, error) {
	bs, err := load(ctx, true)
	if err != nil {
		return nil, nil, err
	}
	brandID, err := bs.brandID(ctx)
	if err != nil {
		return nil, nil, err
	}
	keys, err := auth.LoadOrGenerate(bs.cfg.JWTKeysDir)
	if err != nil {
		return nil, nil, err
	}
	return identity.New(bs.pool, keys, nil, brandID, bs.brand.Code, bs.brand.Country, bs.brand.DefaultLocale), bs, nil
}

// testLicence is a 10-digit licence number derived from the user id, so a
// provisioned test driver passes countries' licence rules (e.g. Tanzania's).
func testLicence(userID string) string {
	digits := make([]byte, 0, testLicenceLen)
	for i := 0; len(digits) < testLicenceLen; i++ {
		digits = append(digits, '0'+userID[i%len(userID)]%10)
	}
	return string(digits)
}

// testLicenceLen is the length of a generated test licence number.
const testLicenceLen = 10
