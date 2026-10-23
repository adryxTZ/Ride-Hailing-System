package cets

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kalen-limited/hauler-rider/backend/internal/providers/regulator"
)

// stub answers every request with status and body, recording the last request.
type stub struct {
	status int
	body   string
	path   string
	auth   string
	sent   string
}

func (s *stub) server(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		s.path, s.auth, s.sent = r.URL.Path, r.Header.Get("Authorization"), string(b)
		w.WriteHeader(s.status)
		_, _ = io.WriteString(w, s.body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestSuccessReturnsReference(t *testing.T) {
	s := &stub{status: 200, body: `{"status":"MS","trxnId":null,"statusCode":5000,"message":"Successful","data":{"tripRef":"t-1","latraReferenceNumber":"M57WEES"}}`}
	c := newClient(t, s.server(t).URL+"/api/")
	ack, err := c.SubmitTrip(context.Background(), sampleTrip())
	if err != nil || ack.Reference != "M57WEES" || ack.Code != "5000" {
		t.Fatalf("ack %+v err %v", ack, err)
	}
	if s.path != "/api/rideHailing/submitTrip" || !strings.HasPrefix(s.auth, "Bearer ") {
		t.Fatalf("path %q auth %q", s.path, s.auth)
	}
	_, claims := verifyJWT(t, strings.TrimPrefix(s.auth, "Bearer "), &key(t).PublicKey)
	if claims.Aud != "cets" || claims.Exp-claims.Iat != 300 {
		t.Fatalf("claims %+v", claims)
	}
}

func TestPaths(t *testing.T) {
	s := &stub{status: 200, body: `{"statusCode":5000,"data":{}}`}
	c := newClient(t, s.server(t).URL)
	ctx := context.Background()
	calls := map[string]func() error{
		"/rideDriver/submitDriver":   func() error { _, e := c.RegisterDriver(ctx, sampleDriver()); return e },
		"/rideVehicle/submitVehicle": func() error { _, e := c.RegisterVehicle(ctx, sampleVehicle()); return e },
		"/rideReport/submitReport":   func() error { _, e := c.SubmitDailyReport(ctx, sampleReport()); return e },
	}
	for want, call := range calls {
		if err := call(); err != nil || s.path != want {
			t.Errorf("%s: path %q err %v", want, s.path, err)
		}
	}
}

func TestRejectionClasses(t *testing.T) {
	cases := []struct {
		name, body, code, class string
		report                  bool
	}{
		{"2xx 5001", `{"statusCode":5001,"message":"Failed","data":{}}`, "5001", regulator.ClassRetry, false},
		{"5016 report", `{"statusCode":5016,"message":"No trips"}`, "5016", regulator.ClassNotNeeded, true},
		{"5016 trip", `{"statusCode":5016,"message":"x"}`, "5016", regulator.ClassRetry, false},
		{"5018", `{"statusCode":5018,"message":"Driver not found"}`, "5018", regulator.ClassPrerequisite, false},
		{"5019 string code", `{"statusCode":"5019","message":"Vehicle not found"}`, "5019", regulator.ClassPrerequisite, false},
	}
	for _, tc := range cases {
		s := &stub{status: 200, body: tc.body}
		c := newClient(t, s.server(t).URL)
		var err error
		if tc.report {
			_, err = c.SubmitDailyReport(context.Background(), sampleReport())
		} else {
			_, err = c.SubmitTrip(context.Background(), sampleTrip())
		}
		r, ok := regulator.AsRejection(err)
		if !ok || r.Code != tc.code || r.Class != tc.class {
			t.Errorf("%s: got %v", tc.name, err)
			continue
		}
		if tc.class == regulator.ClassPrerequisite && r.Message != tc.body {
			t.Errorf("%s: raw body not kept: %q", tc.name, r.Message)
		}
	}
}

func TestUnavailable(t *testing.T) {
	for name, s := range map[string]*stub{
		"500":         {status: 500, body: `{"statusCode":5000}`},
		"not json":    {status: 200, body: `<html>gateway</html>`},
		"no envelope": {status: 200, body: `{"ok":true}`},
	} {
		c := newClient(t, s.server(t).URL)
		if _, err := c.RegisterDriver(context.Background(), sampleDriver()); !errors.Is(err, regulator.ErrUnavailable) {
			t.Errorf("%s: want ErrUnavailable, got %v", name, err)
		}
	}
}

func TestTimeoutIsUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	t.Cleanup(srv.Close)
	cfg := testConfig(t, srv.URL)
	cfg.Timeout = 50 * time.Millisecond
	c, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.SubmitDailyReport(context.Background(), sampleReport()); !errors.Is(err, regulator.ErrUnavailable) {
		t.Fatalf("want ErrUnavailable, got %v", err)
	}
}

func TestInvalidRecordIsFixDataAndNotSent(t *testing.T) {
	s := &stub{status: 200, body: `{"statusCode":5000}`}
	c := newClient(t, s.server(t).URL)
	v := sampleVehicle()
	v.VehicleType = "Boda"
	_, err := c.RegisterVehicle(context.Background(), v)
	tr := sampleTrip()
	tr.Timezone = ""
	_, err2 := c.SubmitTrip(context.Background(), tr)
	for _, e := range []error{err, err2} {
		if r, ok := regulator.AsRejection(e); !ok || r.Class != regulator.ClassFixData {
			t.Errorf("want FIX_DATA, got %v", e)
		}
	}
	if s.path != "" {
		t.Fatalf("an invalid record was sent to %s", s.path)
	}
}
