// Command load measures the hot API paths on a running server.
//
//	go run ./tools/load -drivers 3000 -c 64 -d 10s
//
// It seeds N synthetic online drivers around Dar es Salaam straight into Redis
// presence (ids "load-*", removed at the end), signs in the two OTP test numbers,
// then drives each endpoint with C concurrent clients for D and prints
// throughput and latency percentiles. Development tool; never point it at production.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/kalen-limited/hauler-rider/backend/internal/presence"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/routing"
)

var (
	api      = flag.String("api", "http://localhost:8080/v1", "API base URL")
	redisURL = flag.String("redis", "redis://localhost:6380/0", "Redis URL (for seeding drivers)")
	brand    = flag.String("brand", "nexa", "brand code")
	drivers  = flag.Int("drivers", 3000, "synthetic online drivers to seed")
	conc     = flag.Int("c", 64, "concurrent clients per endpoint")
	dur      = flag.Duration("d", 10*time.Second, "duration per endpoint")
)

var dar = routing.Point{Lat: -6.80, Lng: 39.25}

func main() {
	flag.Parse()
	ctx := context.Background()
	opt, err := redis.ParseURL(*redisURL)
	must(err)
	r := redis.NewClient(opt)
	p := presence.New(r, *brand)
	ids := seed(ctx, p)
	defer func() {
		for _, id := range ids {
			_ = p.Offline(ctx, id)
		}
		fmt.Printf("removed %d synthetic drivers\n", len(ids))
	}()
	rider := token("+255626512332", "rider")
	driver := token("+255715123702", "driver")
	_, _ = call("PUT", "/driver/status", driver, map[string]any{"online": true, "location": dar, "services": []string{"taxi"}, "vehicle_classes": []string{"standard"}})
	defer func() { _, _ = call("PUT", "/driver/status", driver, map[string]any{"online": false}) }()

	fmt.Printf("%-28s %9s %8s %8s %8s %7s\n", "endpoint", "req/s", "p50", "p95", "p99", "errors")
	run("GET /services (cached)", func() (int, error) { return call("GET", "/services", rider, nil) })
	run("GET /drivers/nearby", func() (int, error) {
		at := jitter(dar, 0.03)
		return call("GET", fmt.Sprintf("/drivers/nearby?lat=%f&lng=%f&service=taxi", at.Lat, at.Lng), rider, nil)
	})
	run("POST /driver/location", func() (int, error) {
		return call("POST", "/driver/location", driver, map[string]any{"location": jitter(dar, 0.01), "heading": 90, "speed_mps": 8})
	})
	run("GET /trips/active", func() (int, error) { return call("GET", "/trips/active", rider, nil) })
	run("POST /quotes (route cached)", func() (int, error) {
		return call("POST", "/quotes", rider, map[string]any{"service": "taxi", "pickup": map[string]float64{"lat": -6.7764, "lng": 39.229}, "dropoff": map[string]float64{"lat": -6.7724, "lng": 39.2181}})
	})
}

func seed(ctx context.Context, p *presence.Service) []string {
	ids := make([]string, *drivers)
	for i := range ids {
		ids[i] = fmt.Sprintf("load-%05d", i)
		_, err := p.Online(ctx, ids[i], presence.Ping{Location: jitter(dar, 0.08)}, presence.Capabilities{Services: []string{"taxi"}, Classes: []string{"standard"}})
		must(err)
	}
	fmt.Printf("seeded %d synthetic drivers within ~9 km of Dar es Salaam\n", len(ids))
	return ids
}

func run(name string, fn func() (int, error)) {
	var mu sync.Mutex
	var lat []time.Duration
	var errs atomic.Int64
	deadline := time.Now().Add(*dur)
	var wg sync.WaitGroup
	for range *conc {
		wg.Add(1)
		go func() {
			defer wg.Done()
			local := make([]time.Duration, 0, 4096)
			for time.Now().Before(deadline) {
				t := time.Now()
				status, err := fn()
				local = append(local, time.Since(t))
				if err != nil || status >= 400 {
					errs.Add(1)
				}
			}
			mu.Lock()
			lat = append(lat, local...)
			mu.Unlock()
		}()
	}
	wg.Wait()
	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })
	pct := func(q float64) time.Duration { return lat[int(q*float64(len(lat)-1))] }
	fmt.Printf("%-28s %9.0f %8s %8s %8s %7d\n", name, float64(len(lat))/dur.Seconds(), ms(pct(0.5)), ms(pct(0.95)), ms(pct(0.99)), errs.Load())
}

var client = &http.Client{Transport: &http.Transport{MaxIdleConnsPerHost: 512}, Timeout: 10 * time.Second}

func call(method, path, token string, body any) (int, error) {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req, _ := http.NewRequest(method, *api+path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = res.Body.Close() }()
	_, _ = bytes.NewBuffer(nil).ReadFrom(res.Body)
	return res.StatusCode, nil
}

func token(phone, app string) string {
	_, _ = call("POST", "/auth/otp/request", "", map[string]string{"phone": phone, "app": app})
	var buf bytes.Buffer
	_ = json.NewEncoder(&buf).Encode(map[string]string{"phone": phone, "code": "0000", "app": app})
	res, err := client.Post(*api+"/auth/otp/verify", "application/json", &buf)
	must(err)
	defer func() { _ = res.Body.Close() }()
	var out struct {
		AccessToken string `json:"access_token"`
	}
	must(json.NewDecoder(res.Body).Decode(&out))
	return out.AccessToken
}

func jitter(p routing.Point, deg float64) routing.Point {
	return routing.Point{Lat: p.Lat + (rand.Float64()*2-1)*deg, Lng: p.Lng + (rand.Float64()*2-1)*deg}
}

func ms(d time.Duration) string { return fmt.Sprintf("%.1fms", float64(d.Microseconds())/1000) }

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
