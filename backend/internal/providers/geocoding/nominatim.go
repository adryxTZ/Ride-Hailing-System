package geocoding

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
)

// Nominatim searches OpenStreetMap. The public server allows ~1 request/second
// with an identifying User-Agent, so requests are spaced out here and the
// Cached wrapper absorbs repeats. Production: self-host or switch to Google.
type Nominatim struct {
	BaseURL   string
	UserAgent string
	Country   string // ISO code, e.g. "tz": results stay in the brand's country
	Language  string // e.g. "en"
	client    *http.Client

	mu   sync.Mutex
	last time.Time
}

const (
	nominatimTimeout = 6 * time.Second
	nominatimSpacing = time.Second // public usage policy
	biasDegrees      = 0.35        // ~40 km box around the user to rank nearby places first
)

func NewNominatim(baseURL, userAgent, country, language string) *Nominatim {
	return &Nominatim{BaseURL: baseURL, UserAgent: userAgent, Country: strings.ToLower(country), Language: language,
		client: &http.Client{Timeout: nominatimTimeout}}
}

type nominatimResult struct {
	Lat         string `json:"lat"`
	Lon         string `json:"lon"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

func (n *Nominatim) Search(ctx context.Context, q string, near *routing.Point, limit int) ([]Place, error) {
	v := url.Values{"q": {q}, "format": {"jsonv2"}, "limit": {fmt.Sprint(limit)}, "addressdetails": {"0"}}
	if n.Country != "" {
		v.Set("countrycodes", n.Country)
	}
	if n.Language != "" {
		v.Set("accept-language", n.Language)
	}
	if near != nil {
		v.Set("viewbox", fmt.Sprintf("%f,%f,%f,%f", near.Lng-biasDegrees, near.Lat+biasDegrees, near.Lng+biasDegrees, near.Lat-biasDegrees))
	}
	var rows []nominatimResult
	if err := n.get(ctx, "/search?"+v.Encode(), &rows); err != nil {
		return nil, err
	}
	out := make([]Place, 0, len(rows))
	for _, r := range rows {
		out = append(out, toPlace(r))
	}
	return out, nil
}

func (n *Nominatim) Reverse(ctx context.Context, at routing.Point) (*Place, error) {
	v := url.Values{"lat": {fmt.Sprint(at.Lat)}, "lon": {fmt.Sprint(at.Lng)}, "format": {"jsonv2"}, "zoom": {"17"}}
	if n.Language != "" {
		v.Set("accept-language", n.Language)
	}
	var r nominatimResult
	if err := n.get(ctx, "/reverse?"+v.Encode(), &r); err != nil {
		return nil, err
	}
	if r.DisplayName == "" {
		return nil, nil
	}
	p := toPlace(r)
	p.Location = at // keep the exact point the user is standing on
	return &p, nil
}

func (n *Nominatim) get(ctx context.Context, path string, into any) error {
	n.wait()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, n.BaseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", n.UserAgent)
	res, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("nominatim: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("nominatim: HTTP %d", res.StatusCode)
	}
	return json.NewDecoder(res.Body).Decode(into)
}

// wait spaces requests to respect the public server's one-per-second policy.
func (n *Nominatim) wait() {
	n.mu.Lock()
	defer n.mu.Unlock()
	if d := nominatimSpacing - time.Since(n.last); d > 0 {
		time.Sleep(d)
	}
	n.last = time.Now()
}

// toPlace splits "Name, street, area, city, …" into a title and a short address.
func toPlace(r nominatimResult) Place {
	var lat, lng float64
	_, _ = fmt.Sscan(r.Lat, &lat)
	_, _ = fmt.Sscan(r.Lon, &lng)
	parts := strings.Split(r.DisplayName, ", ")
	name := r.Name
	if name == "" && len(parts) > 0 {
		name, parts = parts[0], parts[1:]
	} else if len(parts) > 0 && parts[0] == name {
		parts = parts[1:]
	}
	const addressParts = 3
	if len(parts) > addressParts {
		parts = parts[:addressParts]
	}
	return Place{Name: name, Address: strings.Join(parts, ", "), Location: routing.Point{Lat: lat, Lng: lng}, Provider: "nominatim"}
}
