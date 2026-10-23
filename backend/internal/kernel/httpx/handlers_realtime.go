package httpx

import (
	"github.com/go-chi/chi/v5"
)

// mountRealtime registers GET /v1/ws. Auth happens in the first frame (never in the URL).
func mountRealtime(r chi.Router, d Deps) {
	if d.Hub == nil {
		return
	}
	verify := func(token string) (string, error) {
		c, err := d.Keys.Verify(token)
		if err != nil || c.Brand != d.Brand.Code {
			return "", err
		}
		return c.Subject, nil
	}
	r.Get("/ws", d.Hub.Handler(verify, d.CORS))
}
