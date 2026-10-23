// Package people builds the staff pages for one rider and one driver: who
// they are, what they ride or drive, where they go, how they rate and are
// rated, complaints and SOS, money, sessions and staff notes.
//
// It owns no tables. Every figure comes from the module that owns it through
// a port (trips, dispatch, messaging, safety, identity, profile, money,
// payouts, payments, holds, presence), fetched in parallel, so a page costs
// one round of queries and no module reads another's tables.
//
//   - model.go    Rider and Driver (the two pages), Charts, Location
//   - ports.go    what this package needs from the other modules
//   - service.go  Rider, Driver, Charts: parallel reads, the staff scope check, the live location
package people
