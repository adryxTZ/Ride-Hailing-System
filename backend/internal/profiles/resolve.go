package profiles

import (
	"sort"
	"time"

	"github.com/kalen-limited/hauler-rider/backend/internal/geofences"
)

// Snapshot is everything resolution needs, held in memory (built by snapshot.go).
type Snapshot struct {
	Profiles map[string]SnapProfile
	// Versions per profile, newest effective first (past ones and up to 90 days ahead).
	Versions map[string][]Version
	// Open maps "<kind>/<scope id>" to its open assignment.
	Open map[string]Scope
	// Payees maps payee id to name, for records.
	Payees map[string]string
	// ConfigVersion is the pricing config version the snapshot was built at.
	ConfigVersion int64
}

// SnapProfile is a profile's resolution-relevant state.
type SnapProfile struct {
	ID          string
	Country     string
	Name        string
	Status      string
	ActiveFrom  *time.Time
	ActiveUntil *time.Time
}

// Input is where and when a trip is priced. Areas come from the areas module;
// geofence hits from the geofence index (the service fills them from Where).
type Input struct {
	Country  string
	AreaID   string
	AreaName string
	At       time.Time
	// PickupFences are the districts, zones and places containing the pickup.
	PickupFences []ScopeRef
	// Dropoff (optional): its area and geofences, and the zones the route crosses.
	HasDropoff    bool
	DropAreaID    string
	DropAreaName  string
	DropoffFences []ScopeRef
	CrossedZones  []ScopeRef
	// Where the trip goes; the service turns these into the geofence lists above.
	Pickup  *geofences.Point
	Dropoff *geofences.Point
	Route   []geofences.Point
}

// SourcedRule is a charge rule with the profile it came from and where it matched.
type SourcedRule struct {
	ChargeRule
	ProfileID string
	Version   int
	MatchedAt string // PICKUP | DROPOFF | ROUTE
	PayeeName string // pinned with the charge, so a later rename leaves records as they were
}

// Ref names a profile version.
type Ref struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	VersionID   string `json:"version_id"`
	Version     int    `json:"version"`
	ContentHash string `json:"content_hash,omitempty"`
}

// ScopeRef names where a profile applied.
type ScopeRef struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Candidate is one profile considered at a point, and why it won or lost.
type Candidate struct {
	Scope    ScopeRef  `json:"scope"`
	Profile  Ref       `json:"profile"`
	Priority int       `json:"priority"`
	Outcome  string    `json:"outcome"` // WON | LOWER_PRECEDENCE | INACTIVE | DRAFT | OUT_OF_WINDOW | NO_VERSION_IN_FORCE | TIE_BROKEN
	since    time.Time // the assignment's start: the older one wins a tie
}

// Trace is the resolution evidence kept on every pricing record.
type Trace struct {
	Point      string      `json:"point"`
	At         time.Time   `json:"at"`
	Candidates []Candidate `json:"candidates"`
	WinnerRule string      `json:"winner_rule"`
	Default    Ref         `json:"default"`
	// Dropoff is the drop-off's candidates and winner (charges only); nil without a drop-off.
	Dropoff *Trace `json:"dropoff,omitempty"`
	// Crossed lists the zones the quoted route passes through whose profiles added charges.
	Crossed []Candidate `json:"crossed,omitempty"`
}

// Resolution is the profile that prices a trip, its merged rules and pickup charges.
type Resolution struct {
	Profile Ref           `json:"profile"`
	Scope   ScopeRef      `json:"scope"`
	Default Ref           `json:"default"`
	Terms   Effective     `json:"terms"`
	Charges []SourcedRule `json:"-"` // rules that can apply: the pickup's, the drop-off's, crossed zones', in that order
	Trace   Trace         `json:"trace"`
	Config  Config        `json:"config"`
}

// Config is the configuration a resolution was made with.
type Config struct {
	PricingVersion  int64 `json:"pricing_version"`
	ResolverVersion int   `json:"resolver_version"`
}

// Outcomes.
const (
	OutcomeWon        = "WON"
	OutcomeLower      = "LOWER_PRECEDENCE"
	OutcomeInactive   = "INACTIVE"
	OutcomeDraft      = "DRAFT"
	OutcomeOutWindow  = "OUT_OF_WINDOW"
	OutcomeNoVersion  = "NO_VERSION_IN_FORCE"
	OutcomeTieBroken  = "TIE_BROKEN"
	winnerRuleDefault = "COUNTRY_DEFAULT"
	winnerRuleScope   = "HIGHEST_PRECEDENCE"
)

// dropoffConditions are the rules a drop-off's profile adds.
var dropoffConditions = map[string]bool{"DROPOFF": true, "EITHER": true}

// Resolve picks the profile that prices a trip at the pickup: every scope that
// contains the point is a candidate; ineligible ones are marked with why; the
// eligible one with the highest precedence (place > zone > district > region >
// country), then priority, then the older assignment, wins. Its rules are merged
// over the country default's. Charges also come from the drop-off's winner and
// from zones the quoted route crosses; a charge code applies once (first source wins).
func Resolve(s *Snapshot, in Input) (*Resolution, error) {
	def, defVersion, err := s.defaultAt(in.Country, in.At)
	if err != nil {
		return nil, err
	}
	scopes := append([]ScopeRef{{Kind: ScopeCountry, ID: in.Country, Name: in.Country}}, areaScope(in.AreaID, in.AreaName)...)
	cands, winner := s.rank(append(scopes, in.PickupFences...), in.At)
	res := &Resolution{Default: def, Trace: Trace{Point: "pickup", At: in.At, Candidates: cands, Default: def},
		Config: Config{PricingVersion: s.ConfigVersion, ResolverVersion: ResolverVersion}}
	if winner == nil {
		winner = &cands[0] // the default is always eligible (validated on save)
	}
	v, _ := s.versionAt(winner.Profile.ID, in.At)
	res.Profile, res.Scope = winner.Profile, winner.Scope
	res.Terms = Merge(defVersion.Terms, v.Terms)
	res.Trace.WinnerRule = winnerRuleScope
	if winner.Scope.Kind == ScopeCountry {
		res.Trace.WinnerRule = winnerRuleDefault
	}
	dropWinner := s.dropoff(in, res)
	res.Charges = pickupRules(v, winner.Profile, dropWinner)
	if dropWinner != nil {
		dv, _ := s.versionAt(dropWinner.Profile.ID, in.At)
		res.Charges = append(res.Charges, sourced(dv, dropWinner.Profile, "DROPOFF", dropoffConditions)...)
	}
	res.Charges = append(res.Charges, s.crossedRules(in, res)...)
	for i := range res.Charges {
		res.Charges[i].PayeeName = s.Payees[res.Charges[i].PayeeID]
	}
	return res, nil
}

func areaScope(id, name string) []ScopeRef {
	if id == "" {
		return nil
	}
	return []ScopeRef{{Kind: ScopeArea, ID: id, Name: name}}
}

// defaultAt is the country default's profile and version in force at `at`.
func (s *Snapshot) defaultAt(country string, at time.Time) (Ref, Version, error) {
	a, ok := s.Open[ScopeCountry+"/"+country]
	if !ok {
		return Ref{}, Version{}, ErrNoDefault
	}
	v, ok := s.versionAt(a.ProfileID, at)
	if !ok {
		return Ref{}, Version{}, ErrNoDefault
	}
	return Ref{ID: a.ProfileID, Name: s.Profiles[a.ProfileID].Name, VersionID: v.ID, Version: v.Version, ContentHash: v.ContentHash}, v, nil
}

// dropoff ranks the scopes at the drop-off and records them in the trace.
func (s *Snapshot) dropoff(in Input, res *Resolution) *Candidate {
	if !in.HasDropoff {
		return nil
	}
	scopes := append([]ScopeRef{{Kind: ScopeCountry, ID: in.Country, Name: in.Country}}, areaScope(in.DropAreaID, in.DropAreaName)...)
	cands, winner := s.rank(append(scopes, in.DropoffFences...), in.At)
	res.Trace.Dropoff = &Trace{Point: "dropoff", At: in.At, Candidates: cands, Default: res.Default}
	return winner
}

// pickupRules are the pickup winner's PICKUP and EITHER rules, and its BOTH
// rules when the drop-off resolves to the same profile.
func pickupRules(v Version, winner Ref, drop *Candidate) []SourcedRule {
	conds := map[string]bool{"PICKUP": true, "EITHER": true, "BOTH": drop != nil && drop.Profile.ID == winner.ID}
	return sourced(v, winner, "PICKUP", conds)
}

// crossedRules are the ROUTE_CROSSES rules of active profiles on crossed zones.
func (s *Snapshot) crossedRules(in Input, res *Resolution) []SourcedRule {
	var out []SourcedRule
	for _, z := range in.CrossedZones {
		cands, winner := s.rank([]ScopeRef{z}, in.At)
		res.Trace.Crossed = append(res.Trace.Crossed, cands...)
		if winner == nil {
			continue
		}
		v, _ := s.versionAt(winner.Profile.ID, in.At)
		out = append(out, sourced(v, winner.Profile, "ROUTE", map[string]bool{"ROUTE_CROSSES": true})...)
	}
	return out
}

func sourced(v Version, p Ref, at string, conds map[string]bool) []SourcedRule {
	var out []SourcedRule
	for _, r := range v.Charges {
		if conds[r.AppliesWhen] {
			out = append(out, SourcedRule{ChargeRule: r, ProfileID: p.ID, Version: p.Version, MatchedAt: at})
		}
	}
	return out
}

// rank marks each candidate scope's outcome and returns the winner (nil if none eligible).
func (s *Snapshot) rank(scopes []ScopeRef, at time.Time) ([]Candidate, *Candidate) {
	var cands []Candidate
	for _, sc := range scopes {
		a, ok := s.Open[sc.Kind+"/"+sc.ID]
		if !ok {
			continue
		}
		p := s.Profiles[a.ProfileID]
		c := Candidate{Scope: sc, Priority: a.Priority, Profile: Ref{ID: p.ID, Name: p.Name}, since: a.ValidFrom}
		c.Outcome = s.eligibility(p, at)
		if v, ok := s.versionAt(p.ID, at); ok {
			c.Profile.VersionID, c.Profile.Version, c.Profile.ContentHash = v.ID, v.Version, v.ContentHash
		}
		cands = append(cands, c)
	}
	var eligible []int
	for i, c := range cands {
		if c.Outcome == "" {
			eligible = append(eligible, i)
		}
	}
	sort.SliceStable(eligible, func(a, b int) bool {
		x, y := cands[eligible[a]], cands[eligible[b]]
		if Precedence[x.Scope.Kind] != Precedence[y.Scope.Kind] {
			return Precedence[x.Scope.Kind] > Precedence[y.Scope.Kind]
		}
		if x.Priority != y.Priority {
			return x.Priority > y.Priority
		}
		return x.since.Before(y.since)
	})
	for rank, i := range eligible {
		cands[i].Outcome = OutcomeLower
		if rank == 0 {
			cands[i].Outcome = OutcomeWon
		} else if sameRank(cands[eligible[0]], cands[i]) {
			cands[i].Outcome = OutcomeTieBroken // recorded in the trace; save-time checks keep this from happening
		}
	}
	if len(eligible) == 0 {
		return cands, nil
	}
	w := cands[eligible[0]]
	return cands, &w
}

func sameRank(a, b Candidate) bool {
	return Precedence[a.Scope.Kind] == Precedence[b.Scope.Kind] && a.Priority == b.Priority
}

// eligibility is "" when the profile can price at `at`, else why not.
func (s *Snapshot) eligibility(p SnapProfile, at time.Time) string {
	switch {
	case p.Status == StatusDraft:
		return OutcomeDraft
	case p.Status != StatusActive:
		return OutcomeInactive
	case (p.ActiveFrom != nil && at.Before(*p.ActiveFrom)) || (p.ActiveUntil != nil && !at.Before(*p.ActiveUntil)):
		return OutcomeOutWindow
	}
	if _, ok := s.versionAt(p.ID, at); !ok {
		return OutcomeNoVersion
	}
	return ""
}

// versionAt is the profile's version in force at `at` (newest effective_from ≤ at).
func (s *Snapshot) versionAt(profileID string, at time.Time) (Version, bool) {
	for _, v := range s.Versions[profileID] {
		if !v.EffectiveFrom.After(at) {
			return v, true
		}
	}
	return Version{}, false
}

// DefaultTerms returns the country default's merged rules at `at` (debt, payout and fee rules).
func DefaultTerms(s *Snapshot, country string, at time.Time) (Effective, Ref, error) {
	a, ok := s.Open[ScopeCountry+"/"+country]
	if !ok {
		return Effective{}, Ref{}, ErrNoDefault
	}
	v, ok := s.versionAt(a.ProfileID, at)
	if !ok {
		return Effective{}, Ref{}, ErrNoDefault
	}
	return effectiveOf(v.Terms), Ref{ID: a.ProfileID, Name: s.Profiles[a.ProfileID].Name, VersionID: v.ID, Version: v.Version}, nil
}
