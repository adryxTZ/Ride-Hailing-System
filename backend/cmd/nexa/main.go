// Command nexa is the Platform's single backend binary. The name is an internal
// codename only: everything user-facing (brand name, colours, logos) is data.
//
// Subcommands:
//
//	serve    run HTTP/WS and/or job workers (see --roles)
//	migrate  apply schema + job-queue migrations
//	seed     load reference data (brand, theme presets)
//	version  print the build version
package main

import (
	"fmt"
	"os"
)

// Version is injected at build time (-ldflags "-X main.Version=...").
var Version = "dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "serve":
		err = runServe(os.Args[2:])
	case "migrate":
		err = runMigrate()
	case "seed":
		err = runSeed(os.Args[2:])
	case "version":
		fmt.Println(Version)
	case "staff":
		err = runStaff(os.Args[2:])
	case "driver":
		err = runDriver(os.Args[2:])
	case "verify-ledger":
		err = runVerifyLedger()
	case "keys":
		err = fmt.Errorf("%q: not implemented yet", os.Args[1])
	case "help", "-h", "--help":
		usage()
	default:
		usage()
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "nexa:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage: nexa <command> [flags]

  serve [--roles=api,realtime,worker,scheduler]   run the service with the given roles
  migrate                                          apply database migrations (schema + job queue)
  seed [brand|presets|catalog|pricing|all]         load reference data (default: all)
  verify-ledger                                    recompute and verify the ledger hash chain
  staff add <phone> --name N --roles R1,R2         create a dashboard (staff) account
  driver provision <phone> --service S --class C   approve a driver + vehicle without documents (test/operator)
  keys <rotate>                                    manage signing keys
  version                                          print the build version`)
}
