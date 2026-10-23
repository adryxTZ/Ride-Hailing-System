package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/brand"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/theme"
)

func TestAppConfigAndErrors(t *testing.T) {
	presets, err := theme.LoadPresets("../../../seed")
	if err != nil {
		t.Fatal(err)
	}
	th, _ := presets.Resolve("deep-navy")
	h := New(Deps{Version: "test", Brand: &brand.Brand{Code: "demo", DisplayName: "Demo", Country: "TZ", Currency: "TZS", DefaultLocale: "sw"}, Theme: th, Presets: presets, MinApp: "1.0.0"})

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/app/config", nil))
	if rr.Code != 200 || rr.Header().Get("ETag") == "" {
		t.Fatalf("app config: %d etag=%q", rr.Code, rr.Header().Get("ETag"))
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/app/config", nil)
	req.Header.Set("If-None-Match", rr.Header().Get("ETag"))
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req)
	if rr2.Code != http.StatusNotModified {
		t.Fatalf("expected 304, got %d", rr2.Code)
	}
	rr3 := httptest.NewRecorder()
	h.ServeHTTP(rr3, httptest.NewRequest(http.MethodGet, "/v1/nope", nil))
	if rr3.Code != 404 || !containsAll(rr3.Body.String(), `"code":"NOT_FOUND"`, `"request_id"`) {
		t.Fatalf("error envelope: %d %s", rr3.Code, rr3.Body.String())
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !contains(s, sub) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
