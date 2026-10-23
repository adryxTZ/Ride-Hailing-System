// Package safety keeps riders safe during a trip: trusted contacts, a public
// live link to share the trip, and SOS alerts that text the trusted contacts
// and land in the staff queue.
//
//   - model.go     TrustedContact, PublicTrip, Alert, errors
//   - ports.go     what safety needs: trip lookups, the driver's live position, SMS
//   - store.go     queries (contacts, share tokens, alerts)
//   - service.go   Contacts / SaveContacts, ShareLink / PublicView, SOS, staff alert queue
//   - person.go    AlertsFor: SOS on one person's trips (staff rider and driver pages)
package safety
