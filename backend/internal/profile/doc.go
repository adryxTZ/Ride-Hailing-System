// Package profile owns what a signed-in person can see and edit about
// themselves: name, email, locale, photo, and saved places. Transport DTOs
// live in kernel/httpx; this package exposes typed use cases.
//
//   - model.go    Profile, Place
//   - store.go    queries
//   - service.go  use cases: Get, Update, ListPlaces, SavePlace, DeletePlace
package profile
