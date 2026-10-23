package fleet

// checklist marks the country's document types done and lists what still
// blocks submission: only MANDATORY types block; OPTIONAL ones are shown; OFF
// ones are not asked for.
func checklist(types []DocumentType, p Profile, v *Vehicle, docs []Document) ([]Requirement, []string) {
	have := map[string]bool{}
	for _, d := range docs {
		if d.Status != ReviewRejected {
			have[d.Kind] = true
		}
	}
	missing := []string{}
	if p.FullName == nil || p.LicenceNumber == nil || p.NationalID == nil {
		missing = append(missing, "profile")
	}
	if v == nil {
		missing = append(missing, "vehicle")
	}
	reqs := []Requirement{}
	for _, t := range types {
		if t.Requirement == RequireOff {
			continue
		}
		r := Requirement{Kind: t.Kind, Owner: t.Owner, Label: t.Label, Done: have[t.Kind], Mandatory: t.Requirement == RequireMandatory,
			TracksExpiry: t.TracksExpiry, ExpiryRequired: t.ExpiryRequired}
		reqs = append(reqs, r)
		if r.Mandatory && !r.Done {
			missing = append(missing, r.Kind)
		}
	}
	return reqs, missing
}

// editable: a driver changes details while onboarding or after a rejection.
func editable(status string) bool {
	return status == StatusOnboarding || status == StatusRejected
}
