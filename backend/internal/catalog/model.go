package catalog

import "encoding/json"

// Localized is a map of language → text, e.g. {"en": "...", "sw": "..."}.
type Localized map[string]string

type Service struct {
	Key            string          `json:"key"`
	Category       string          `json:"category"`
	Name           Localized       `json:"name"`
	Description    Localized       `json:"description,omitempty"`
	Status         string          `json:"status"`
	SortOrder      int             `json:"sort_order"`
	Icon           string          `json:"icon,omitempty"`
	Lifecycle      json.RawMessage `json:"lifecycle"`
	RequestSchema  json.RawMessage `json:"request_schema"`
	Dispatch       json.RawMessage `json:"dispatch,omitempty"`
	Compliance     json.RawMessage `json:"compliance,omitempty"`
	VehicleClasses []VehicleClass  `json:"vehicle_classes"`
}

type VehicleClass struct {
	Key           string    `json:"key"`
	Name          Localized `json:"name"`
	Description   Localized `json:"description,omitempty"`
	Icon          string    `json:"icon,omitempty"`
	SortOrder     int       `json:"sort_order"`
	Seats         *int      `json:"seats,omitempty"`
	RegulatorType string    `json:"regulator_type,omitempty"`
	Status        string    `json:"status"`
}

// Catalogue is the whole per-brand catalogue as served to clients.
type Catalogue struct {
	Services []Service `json:"services"`
}
