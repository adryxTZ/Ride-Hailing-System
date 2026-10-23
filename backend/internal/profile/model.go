package profile

import (
	"net/http"
	"time"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

type Profile struct {
	ID            string    `json:"user_id"`
	Phone         string    `json:"phone"`
	FullName      *string   `json:"full_name"`
	Email         *string   `json:"email"`
	EmailVerified bool      `json:"email_verified"`
	Locale        *string   `json:"locale"`
	PhotoURL      *string   `json:"photo_url"`
	Roles         []string  `json:"roles"`
	CreatedAt     time.Time `json:"created_at"`
}

// ProfileUpdate carries only the fields a client may change; nil = untouched.
type ProfileUpdate struct {
	FullName *string `json:"full_name"`
	Email    *string `json:"email"`
	Locale   *string `json:"locale"`
}

type Place struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"` // HOME | WORK | OTHER
	Label     string    `json:"label"`
	Address   string    `json:"address"`
	Lat       float64   `json:"lat"`
	Lng       float64   `json:"lng"`
	UseCount  int       `json:"use_count"`
	UpdatedAt time.Time `json:"updated_at"`
}

type PlaceInput struct {
	Kind    string  `json:"kind"`
	Label   string  `json:"label"`
	Address string  `json:"address"`
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
}

var (
	ErrPlaceNotFound = apperr.New("PLACE_NOT_FOUND", http.StatusNotFound, "")
	ErrEmailTaken    = apperr.New("EMAIL_TAKEN", http.StatusConflict, "")
)
