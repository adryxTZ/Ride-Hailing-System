package httpx

import "github.com/kalen-limited/hauler-rider/backend/internal/kernel/theme"

// Transport DTOs. These mirror api/openapi.yaml and are the only shapes the
// HTTP layer writes; domain models never leak onto the wire directly.

type healthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

type listResponse[T any] struct {
	Data []T `json:"data"`
}

type brandResponse struct {
	Code         string `json:"code"`
	Name         string `json:"name"`
	LegalEntity  string `json:"legal_entity,omitempty"`
	Country      string `json:"country"`
	Currency     string `json:"currency"`
	Locale       string `json:"locale"`
	SupportPhone string `json:"support_phone,omitempty"`
	SupportEmail string `json:"support_email,omitempty"`
	// EmergencyPhone is offered by the Safety screen (country emergency number).
	EmergencyPhone string `json:"emergency_phone,omitempty"`
}

type appConfigResponse struct {
	Brand         brandResponse   `json:"brand"`
	Theme         *theme.Theme    `json:"theme"`
	Features      map[string]bool `json:"features"`
	MinAppVersion string          `json:"min_app_version"`
}

type errorResponse struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
	RequestID string         `json:"request_id"`
}

func toAnySlice[T any](in []T) []any {
	out := make([]any, len(in))
	for i, v := range in {
		out[i] = v
	}
	return out
}
