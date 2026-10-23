// Package scope names the operating scope a staff request looks at: a country,
// and optionally one area in it. Modules filter by it; the admin picks it.
package scope

// Scope is a country code and an optional area id ("" = the whole country).
type Scope struct {
	Country string
	AreaID  string
}
