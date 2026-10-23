package httpx

import (
	_ "embed"
	"html/template"
	"net/http"

	"github.com/go-chi/chi/v5"
)

//go:embed pages/share.html
var sharePageHTML string

var sharePage = template.Must(template.New("share").Parse(sharePageHTML))

// sharePageData fills the live-trip page with the brand's name and colours.
type sharePageData struct {
	Brand, Primary, OnPrimary, Background, Text, Muted string
	DataURL                                            string
}

// mountSharePage serves /share/{token}: a public page that follows the trip
// live (map, driver, vehicle) from /v1/share/{token}. No sign-in.
func mountSharePage(r chi.Router, d Deps) {
	if d.Safety == nil {
		return
	}
	r.Get("/share/{token}", func(w http.ResponseWriter, req *http.Request) {
		data := sharePageData{DataURL: "/v1/share/" + chi.URLParam(req, "token")}
		if d.Brand != nil {
			data.Brand = d.Brand.DisplayName
		}
		if d.Theme != nil {
			p := d.Theme.Palette
			data.Primary, data.OnPrimary, data.Background, data.Text, data.Muted = p.Primary, p.OnPrimary, p.Background, p.Text, p.Muted
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = sharePage.Execute(w, data)
	})
}
