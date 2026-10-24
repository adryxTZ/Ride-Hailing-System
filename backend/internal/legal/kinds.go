package legal

import "strings"

// KindInfo describes a document kind: which apps have it, whether people must
// accept it (publishing a new version then asks them again) and its URL slug.
type KindInfo struct {
	Kind    string
	Slug    string
	Apps    []string
	Consent bool
	Title   Text // shown when a version has no title in a language
}

// Kinds is the catalogue, in the order the Legal page lists them.
var Kinds = []KindInfo{
	{Kind: KindPrivacyNotice, Slug: "privacy-notice", Apps: []string{AppRider, AppDriver}, Consent: true,
		Title: Text{"en": "Privacy Notice", "sw": "Taarifa ya Faragha"}},
	{Kind: KindTerms, Slug: "terms", Apps: []string{AppRider, AppDriver}, Consent: true,
		Title: Text{"en": "Terms of Use", "sw": "Masharti ya Matumizi"}},
	{Kind: KindDriverAgreement, Slug: "driver-agreement", Apps: []string{AppDriver}, Consent: true,
		Title: Text{"en": "Driver Agreement", "sw": "Mkataba wa Dereva"}},
}

// Apps lists the apps in display order.
var Apps = []string{AppRider, AppDriver}

// Info returns a kind's description.
func Info(kind string) (KindInfo, bool) {
	for _, k := range Kinds {
		if k.Kind == kind {
			return k, true
		}
	}
	return KindInfo{}, false
}

// Offered reports whether an app has a document of this kind.
func Offered(app, kind string) bool {
	k, ok := Info(kind)
	if !ok {
		return false
	}
	for _, a := range k.Apps {
		if a == app {
			return true
		}
	}
	return false
}

// ParseApp accepts "rider", "RIDER" and the like.
func ParseApp(s string) (string, bool) {
	app := strings.ToUpper(strings.TrimSpace(s))
	return app, app == AppRider || app == AppDriver
}

// ParseKind accepts the kind ("PRIVACY_NOTICE") or its slug ("privacy-notice").
func ParseKind(s string) (string, bool) {
	kind := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(s), "-", "_"))
	_, ok := Info(kind)
	return kind, ok
}

// PagePath is a published document's public page, relative to the API base
// ("/legal/rider/privacy-notice?lang=en"); clients prefix the base they call.
func PagePath(app, kind, lang string) string {
	k, _ := Info(kind)
	return "/legal/" + strings.ToLower(app) + "/" + k.Slug + "?lang=" + lang
}
