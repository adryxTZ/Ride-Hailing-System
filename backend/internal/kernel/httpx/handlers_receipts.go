package httpx

import (
	"net"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
	"github.com/kalen-limited/hauler-rider/backend/internal/receipts"
)

// receiptCache lets the owner's app keep a receipt: it never changes once issued.
const receiptCache = "private, max-age=86400, immutable"

// errPublicURLMissing: links need NEXA_PUBLIC_API_URL outside a local machine.
var errPublicURLMissing = apperr.New("PUBLIC_URL_NOT_CONFIGURED", http.StatusServiceUnavailable, "")

// mountReceipts registers receipts: the trip's participants read the JSON and
// get a short-lived signed PDF link; the PDF itself is reached only through
// such a link (no session, so it opens in any browser or share sheet).
func mountReceipts(r chi.Router, d Deps) {
	if d.Receipts == nil {
		return
	}
	h := receiptHandlers{deps: d}
	r.Get("/receipts/{id}/pdf", h.pdf)
	r.Group(func(g chi.Router) {
		g.Use(requireAuth(d))
		g.Get("/trips/{id}/receipt", h.get(false))
		g.Get("/trips/{id}/receipt/link", h.link(false))
	})
	r.Group(func(g chi.Router) {
		g.Use(requireAuth(d), requireStaff())
		g.Get("/admin/trips/{id}/receipt", h.get(true))
		g.Get("/admin/trips/{id}/receipt/link", h.link(true))
	})
}

type receiptHandlers struct{ deps Deps }

func (h receiptHandlers) load(req *http.Request, staff bool) (*receipts.Receipt, error) {
	id := chi.URLParam(req, "id")
	if staff {
		if err := h.inScope(req, id); err != nil {
			return nil, err
		}
	}
	return h.deps.Receipts.ForTrip(req.Context(), id, claimsFrom(req.Context()).Subject, staff)
}

// inScope refuses staff limited to other countries or regions (a receipt carries
// addresses and the driver's name, so it follows the trip page's scope rule).
func (h receiptHandlers) inScope(req *http.Request, tripID string) error {
	t, err := h.deps.Trips.AdminGet(req.Context(), tripID)
	if err != nil {
		return err
	}
	area := ""
	if t.Area != nil {
		area = t.Area.ID
	}
	if !allowedScope(claimsFrom(req.Context()), t.Country, area) {
		return ErrScopeForbidden
	}
	return nil
}

func (h receiptHandlers) get(staff bool) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		r, err := h.load(req, staff)
		if err != nil {
			writeDomainError(w, req, err)
			return
		}
		writeJSONCached(w, req, r, receiptCache)
	}
}

func (h receiptHandlers) link(staff bool) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		r, err := h.load(req, staff)
		if err != nil {
			writeDomainError(w, req, err)
			return
		}
		lang := req.URL.Query().Get("lang")
		if lang == "" {
			lang = req.Header.Get("Accept-Language")
		}
		base, ok := h.publicBase(req)
		if !ok {
			writeError(w, req, errPublicURLMissing, nil)
			return
		}
		writeJSON(w, http.StatusOK, h.deps.Receipts.LinkFor(r, lang, base))
	}
}

// pdf serves the receipt PDF behind a signed link.
func (h receiptHandlers) pdf(w http.ResponseWriter, req *http.Request) {
	q := req.URL.Query()
	id := chi.URLParam(req, "id")
	body, err := h.deps.Receipts.PDF(req.Context(), id, q.Get("lang"), q.Get("exp"), q.Get("sig"))
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `inline; filename="receipt.pdf"`)
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

// publicBase is the API's public address for links: NEXA_PUBLIC_API_URL. Only on
// a local machine does it fall back to the request's host (development); a link
// is never built from a Host header a proxy passed through.
func (h receiptHandlers) publicBase(req *http.Request) (string, bool) {
	if h.deps.PublicAPIURL != "" {
		return strings.TrimRight(h.deps.PublicAPIURL, "/"), true
	}
	host := req.Host
	if name, _, err := net.SplitHostPort(host); err == nil {
		host = name
	}
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return "http://" + req.Host, true
	}
	return "", false
}
