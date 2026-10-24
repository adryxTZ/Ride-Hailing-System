package httpx

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/theme"
	"github.com/kalen-limited/hauler-rider/backend/internal/legal"
)

//go:embed pages/legal.html
var legalPageHTML string

var legalPage = template.Must(template.New("legal").Parse(legalPageHTML))

// legalCacheS: a published text never changes; a new version shows within five minutes.
const legalCacheS = "public, max-age=300"

// legalPageCSP allows no script, frame or form at all; only the inline styles.
const legalPageCSP = "default-src 'none'; style-src 'unsafe-inline'; img-src https: data:; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"

// legalLabels are the page's two fixed words per language.
var legalLabels = map[string][2]string{
	"en": {"Version", "Effective"},
	"sw": {"Toleo", "Linaanza kutumika"},
}

// legalPageData fills pages/legal.html.
type legalPageData struct {
	Lang, Title, Brand, EffectiveDate, VersionLabel, EffectiveLabel, Version string
	Primary, Background, Surface, Text, Muted, Border                        string
	Body                                                                     template.HTML
}

// writeLegalPage renders a document page with an ETag, a short public cache
// and a policy that runs no script. The body is the legal module's sanitised HTML.
func writeLegalPage(w http.ResponseWriter, req *http.Request, d Deps, p legal.Page) {
	labels, ok := legalLabels[p.Lang]
	if !ok {
		labels = legalLabels["en"]
	}
	pal := legalPalette(req, d)
	data := legalPageData{Lang: p.Lang, Title: p.Title, Brand: p.Brand, EffectiveDate: p.EffectiveDate, VersionLabel: labels[0],
		EffectiveLabel: labels[1], Version: strconv.Itoa(p.Version), Primary: pal.Primary, Background: pal.Background, Surface: pal.Surface,
		Text: pal.Text, Muted: pal.Muted, Border: pal.Border, Body: template.HTML(p.HTML)} // #nosec G203 -- legal.Markdown escapes every character it does not turn into its own tags
	var buf bytes.Buffer
	if err := legalPage.Execute(&buf, data); err != nil {
		logUnexpected(req, err)
		writeError(w, req, legal.ErrNotFound, nil)
		return
	}
	sum := sha256.Sum256(buf.Bytes())
	etag := `"` + hex.EncodeToString(sum[:8]) + `"`
	h := w.Header()
	h.Set("ETag", etag)
	h.Set("Cache-Control", legalCacheS)
	if req.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Content-Security-Policy", legalPageCSP)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
	_, _ = w.Write(buf.Bytes())
}

// legalPalette is the published theme's colours (the brand file's preset until one is published).
func legalPalette(req *http.Request, d Deps) theme.Palette {
	if d.Branding != nil {
		if live, err := d.Branding.Published(req.Context()); err == nil && live.Theme != nil {
			return live.Theme.Palette
		}
	}
	if d.Theme != nil {
		return d.Theme.Palette
	}
	return theme.Palette{}
}

// apiLink makes a link stored relative to the API ("/legal/rider/terms?lang=en")
// absolute: NEXA_PUBLIC_API_URL, or the address the caller used (development).
func apiLink(req *http.Request, d Deps, u string) string {
	if !strings.HasPrefix(u, "/") {
		return u
	}
	if d.PublicAPIURL != "" {
		return strings.TrimRight(d.PublicAPIURL, "/") + u
	}
	return publicBase(req, "") + "/v1" + u
}
