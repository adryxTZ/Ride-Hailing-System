package httpx

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/identity"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// mountAuth registers sign-in, token refresh, logout and key discovery.
func mountAuth(r chi.Router, d Deps) {
	h := authHandlers{deps: d}
	r.Get("/auth/jwks", h.jwks)
	r.Post("/auth/otp/request", h.requestOTP)
	r.Post("/auth/otp/verify", h.verifyOTP)
	r.Post("/auth/staff/login", h.staffLogin)
	r.Post("/auth/refresh", h.refresh)
	r.Post("/auth/logout", h.logout)
}

type authHandlers struct{ deps Deps }

type otpRequestBody struct {
	Phone string `json:"phone"`
	App   string `json:"app"`
}

type otpVerifyBody struct {
	Phone string `json:"phone"`
	Code  string `json:"code"`
	App   string `json:"app"`
}

type refreshBody struct {
	RefreshToken string `json:"refresh_token"`
	App          string `json:"app"`
}

// jwks publishes the public signing keys.
func (h authHandlers) jwks(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.deps.Keys.JWKS())
}

// requestOTP sends a one-time code. Returns 202; in development it may echo the code.
func (h authHandlers) requestOTP(w http.ResponseWriter, req *http.Request) {
	var body otpRequestBody
	if !decode(w, req, &body) || !requireFields(w, req, map[string]string{"phone": body.Phone, "app": body.App}) {
		return
	}
	code, err := h.deps.Identity.RequestOTP(req.Context(), body.Phone, identity.App(body.App))
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	resp := map[string]any{"status": "sent"}
	if code != "" {
		resp["dev_code"] = code
	}
	writeJSON(w, http.StatusAccepted, resp)
}

// verifyOTP exchanges a valid code for a token pair.
func (h authHandlers) verifyOTP(w http.ResponseWriter, req *http.Request) {
	var body otpVerifyBody
	if !decode(w, req, &body) || !requireFields(w, req, map[string]string{"phone": body.Phone, "code": body.Code, "app": body.App}) {
		return
	}
	pair, err := h.deps.Identity.VerifyOTP(req.Context(), body.Phone, body.Code, identity.App(body.App))
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, pair)
}

// clientIP is the caller's address (the RealIP middleware honours X-Forwarded-For).
func clientIP(req *http.Request) string {
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return req.RemoteAddr
	}
	return host
}

type staffLoginBody struct {
	Email    string                  `json:"email"`
	Password string                  `json:"password"`
	Location *identity.LoginLocation `json:"location"` // optional: the browser's position, with the person's permission
}

// staffLogin signs dashboard staff in with email and password.
func (h authHandlers) staffLogin(w http.ResponseWriter, req *http.Request) {
	var body staffLoginBody
	if !decode(w, req, &body) || !requireFields(w, req, map[string]string{"email": body.Email, "password": body.Password}) {
		return
	}
	pair, err := h.deps.Identity.LoginWithPassword(req.Context(), body.Email, body.Password,
		identity.LoginContext{IP: clientIP(req), UserAgent: req.UserAgent(), Location: body.Location})
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, pair)
}

// refresh rotates the refresh token and returns a new pair.
func (h authHandlers) refresh(w http.ResponseWriter, req *http.Request) {
	var body refreshBody
	if !decode(w, req, &body) || !requireFields(w, req, map[string]string{"refresh_token": body.RefreshToken, "app": body.App}) {
		return
	}
	pair, err := h.deps.Identity.Refresh(req.Context(), body.RefreshToken, identity.App(body.App))
	if err != nil {
		writeDomainError(w, req, err)
		return
	}
	writeJSON(w, http.StatusOK, pair)
}

// logout revokes the whole session family of the given refresh token.
func (h authHandlers) logout(w http.ResponseWriter, req *http.Request) {
	var body refreshBody
	if !decode(w, req, &body) || !requireFields(w, req, map[string]string{"refresh_token": body.RefreshToken}) {
		return
	}
	if err := h.deps.Identity.Logout(req.Context(), body.RefreshToken); err != nil {
		writeDomainError(w, req, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- shared request helpers (transport only) ------------------------------

// decode parses a JSON body; on failure it writes VALIDATION_FAILED and returns false.
func decode(w http.ResponseWriter, req *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, req.Body, 1<<20)).Decode(v); err != nil {
		writeError(w, req, apperr.Validation, map[string]any{"reason": "malformed JSON body"})
		return false
	}
	return true
}

// requireFields rejects empty required fields with a per-field details list.
func requireFields(w http.ResponseWriter, req *http.Request, fields map[string]string) bool {
	var missing []map[string]string
	for name, val := range fields {
		if val == "" {
			missing = append(missing, map[string]string{"path": name, "code": "REQUIRED"})
		}
	}
	if len(missing) > 0 {
		writeError(w, req, apperr.Validation, map[string]any{"fields": missing})
		return false
	}
	return true
}

// writeDomainError maps a module error to the envelope; unknown errors become INTERNAL.
func writeDomainError(w http.ResponseWriter, req *http.Request, err error) {
	var e *apperr.E
	if errors.As(err, &e) {
		writeError(w, req, e, e.Details)
		return
	}
	logUnexpected(req, err)
	writeError(w, req, apperr.Internal, nil)
}
