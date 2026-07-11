package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prashantkoirala465/nepapi/internal/store"
)

type fakeForex struct {
	latest []store.ForexRate
	ranged []store.ForexRate
	err    error

	gotFrom, gotTo time.Time
	gotISO3        string
}

func (f *fakeForex) LatestRates(ctx context.Context) ([]store.ForexRate, error) {
	return f.latest, f.err
}

func (f *fakeForex) RatesRange(ctx context.Context, from, to time.Time, iso3 string) ([]store.ForexRate, error) {
	f.gotFrom, f.gotTo, f.gotISO3 = from, to, iso3
	return f.ranged, f.err
}

type fakePinger struct{ err error }

func (p *fakePinger) Ping(ctx context.Context) error { return p.err }

func testServer(f *fakeForex) http.Handler {
	return testServerCfg(Config{}, f)
}

func testServerCfg(cfg Config, f *fakeForex) http.Handler {
	return NewServer(cfg, f, &fakePinger{}, slog.New(slog.DiscardHandler)).Handler()
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.RemoteAddr = "192.0.2.1:1234"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHealth(t *testing.T) {
	rec := get(t, testServer(&fakeForex{}), "/v1/health")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestReady(t *testing.T) {
	rec := get(t, testServer(&fakeForex{}), "/v1/ready")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestReadyDatabaseDown(t *testing.T) {
	h := NewServer(Config{}, &fakeForex{}, &fakePinger{err: errors.New("connection refused")}, slog.New(slog.DiscardHandler)).Handler()
	rec := get(t, h, "/v1/ready")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

func TestForexLatest(t *testing.T) {
	date := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	f := &fakeForex{latest: []store.ForexRate{
		{Date: date, ISO3: "USD", Name: "U.S. Dollar", Unit: 1, Buy: "152.33", Sell: "152.93"},
	}}
	rec := get(t, testServer(f), "/v1/forex/latest")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var body struct {
		Date  string            `json:"date"`
		Rates []store.ForexRate `json:"rates"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding body: %v", err)
	}
	if body.Date != "2026-07-10" || len(body.Rates) != 1 || body.Rates[0].ISO3 != "USD" {
		t.Errorf("unexpected body: %+v", body)
	}
}

func TestForexLatestEmpty(t *testing.T) {
	rec := get(t, testServer(&fakeForex{}), "/v1/forex/latest")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestForexLatestStoreError(t *testing.T) {
	rec := get(t, testServer(&fakeForex{err: errors.New("db down")}), "/v1/forex/latest")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestForexRatesValidation(t *testing.T) {
	cases := []struct {
		name string
		path string
		want int
	}{
		{"missing from", "/v1/forex/rates?to=2026-07-10", http.StatusBadRequest},
		{"bad from", "/v1/forex/rates?from=notadate&to=2026-07-10", http.StatusBadRequest},
		{"to before from", "/v1/forex/rates?from=2026-07-10&to=2026-07-01", http.StatusBadRequest},
		{"range too large", "/v1/forex/rates?from=2020-01-01&to=2026-07-10", http.StatusBadRequest},
		{"valid", "/v1/forex/rates?from=2026-07-01&to=2026-07-10", http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := get(t, testServer(&fakeForex{}), tc.path)
			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d; body: %s", rec.Code, tc.want, rec.Body)
			}
		})
	}
}

func TestForexRatesPassesCurrencyFilter(t *testing.T) {
	f := &fakeForex{}
	get(t, testServer(f), "/v1/forex/rates?from=2026-07-01&to=2026-07-10&currency=USD")
	if f.gotISO3 != "USD" {
		t.Errorf("currency filter = %q, want USD", f.gotISO3)
	}
	if f.gotFrom.Format("2006-01-02") != "2026-07-01" {
		t.Errorf("from = %v", f.gotFrom)
	}
}

func TestRateLimitKicksIn(t *testing.T) {
	h := testServerCfg(Config{RateRPS: 1, RateBurst: 5}, &fakeForex{})
	var limited bool
	for i := 0; i < 10; i++ {
		rec := get(t, h, "/v1/health")
		if rec.Code == http.StatusTooManyRequests {
			limited = true
			break
		}
	}
	if !limited {
		t.Error("rate limiter never returned 429 after 10 rapid requests with burst 5")
	}
}

func TestRateLimitIgnoresSpoofedForwardedFor(t *testing.T) {
	// TrustProxy off: rotating X-Forwarded-For must NOT reset the
	// bucket, since all requests share the same RemoteAddr.
	h := testServerCfg(Config{RateRPS: 1, RateBurst: 5}, &fakeForex{})
	var limited bool
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
		req.RemoteAddr = "192.0.2.1:1234"
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("10.0.0.%d", i))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code == http.StatusTooManyRequests {
			limited = true
			break
		}
	}
	if !limited {
		t.Error("spoofed X-Forwarded-For bypassed the rate limiter with TrustProxy off")
	}
}

func TestRateLimitHonorsForwardedForBehindProxy(t *testing.T) {
	// TrustProxy on: distinct client IPs in X-Forwarded-For get
	// separate buckets even through one proxy connection.
	h := testServerCfg(Config{TrustProxy: true, RateRPS: 1, RateBurst: 5}, &fakeForex{})
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
		req.RemoteAddr = "127.0.0.1:9999" // the proxy
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("10.0.0.%d", i))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code == http.StatusTooManyRequests {
			t.Fatalf("request %d limited despite unique forwarded client IPs", i)
		}
	}
}

func TestCORSHeader(t *testing.T) {
	rec := get(t, testServer(&fakeForex{}), "/v1/health")
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q, want *", got)
	}
}
