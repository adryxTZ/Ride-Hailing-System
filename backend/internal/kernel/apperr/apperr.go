// Package apperr defines the Platform's error envelope: a stable machine-readable
// code plus a message localised by the HTTP layer. Apps key on Code, never on text.
package apperr

import "net/http"

type E struct {
	Code    string         `json:"code"`
	HTTP    int            `json:"-"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

func (e *E) Error() string { return e.Code }

// Is matches errors by code, so a copy carrying details still equals its base
// error (errors.Is(err, ErrBelowMinimum) holds after WithDetails).
func (e *E) Is(target error) bool {
	t, ok := target.(*E)
	return ok && t.Code == e.Code
}

func New(code string, status int, msg string) *E { return &E{Code: code, HTTP: status, Message: msg} }

// WithDetails returns a copy carrying structured details for the envelope.
func (e *E) WithDetails(d map[string]any) *E {
	c := *e
	c.Details = d
	return &c
}

var (
	NotFound        = New("NOT_FOUND", http.StatusNotFound, "Resource not found.")
	Validation      = New("VALIDATION_FAILED", http.StatusBadRequest, "Request is invalid.")
	Unauthenticated = New("UNAUTHENTICATED", http.StatusUnauthorized, "Sign in required.")
	Forbidden       = New("FORBIDDEN", http.StatusForbidden, "Not allowed.")
	NotReady        = New("NOT_READY", http.StatusServiceUnavailable, "Service is starting.")
	Internal        = New("INTERNAL", http.StatusInternalServerError, "Something went wrong.")
)
