package nrb

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const fixturePage1 = `{
  "status": {"code": 200},
  "errors": {"validation": null},
  "data": {
    "payload": [
      {
        "date": "2026-07-10",
        "published_on": "2026-07-10 00:00:03",
        "rates": [
          {"currency": {"iso3": "USD", "name": "U.S. Dollar", "unit": 1}, "buy": "152.33", "sell": "152.93"},
          {"currency": {"iso3": "INR", "name": "Indian Rupee", "unit": 100}, "buy": "160.00", "sell": "160.15"}
        ]
      },
      {
        "date": "2026-07-11",
        "published_on": "2026-07-11 00:00:02",
        "rates": [
          {"currency": {"iso3": "USD", "name": "U.S. Dollar", "unit": 1}, "buy": "152.40", "sell": "153.00"}
        ]
      }
    ]
  }
}`

const fixtureEmpty = `{"status": {"code": 200}, "errors": {"validation": null}, "data": {"payload": []}}`

func TestRatesRange(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("from"); got != "2026-07-10" {
			t.Errorf("from = %q, want 2026-07-10", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, fixturePage1)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	from := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC)

	days, err := c.RatesRange(context.Background(), from, to)
	if err != nil {
		t.Fatalf("RatesRange: %v", err)
	}
	if len(days) != 2 {
		t.Fatalf("got %d days, want 2", len(days))
	}
	if !days[0].Date.Equal(from) {
		t.Errorf("day 0 date = %v, want %v", days[0].Date, from)
	}
	usd := days[0].Rates[0]
	if usd.ISO3 != "USD" || usd.Buy != 152.33 || usd.Sell != 152.93 || usd.Unit != 1 {
		t.Errorf("unexpected USD rate: %+v", usd)
	}
	inr := days[0].Rates[1]
	if inr.Unit != 100 {
		t.Errorf("INR unit = %d, want 100 (NRB quotes INR per 100)", inr.Unit)
	}
}

func TestRatesRangeEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, fixtureEmpty)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	days, err := c.RatesRange(context.Background(), time.Now(), time.Now())
	if err != nil {
		t.Fatalf("RatesRange: %v", err)
	}
	if len(days) != 0 {
		t.Fatalf("got %d days, want 0", len(days))
	}
}

func TestRatesRangeServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if _, err := c.RatesRange(context.Background(), time.Now(), time.Now()); err == nil {
		t.Fatal("expected error on 500 response, got nil")
	}
}

func TestRatesRangeBadRateValue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"status":{"code":200},"data":{"payload":[{"date":"2026-07-10","published_on":"x","rates":[{"currency":{"iso3":"USD","name":"U.S. Dollar","unit":1},"buy":"not-a-number","sell":"1.0"}]}]}}`)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if _, err := c.RatesRange(context.Background(), time.Now(), time.Now()); err == nil {
		t.Fatal("expected parse error, got nil")
	}
}
