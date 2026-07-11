package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/prashantkoirala465/nepapi/internal/nrb"
)

// newTestStore connects to the database in NEPAPI_TEST_DATABASE_URL and
// applies migrations. Tests are skipped when the variable is unset so
// `go test ./...` stays green without a database.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	url := os.Getenv("NEPAPI_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("NEPAPI_TEST_DATABASE_URL not set; skipping integration test")
	}
	s, err := New(context.Background(), url)
	if err != nil {
		t.Fatalf("connecting to test database: %v", err)
	}
	t.Cleanup(s.Close)
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("migrating: %v", err)
	}
	if _, err := s.pool.Exec(context.Background(), `TRUNCATE forex_rates`); err != nil {
		t.Fatalf("truncating: %v", err)
	}
	return s
}

func day(dateStr string, rates ...nrb.Rate) nrb.DayRates {
	d, _ := time.Parse("2006-01-02", dateStr)
	return nrb.DayRates{Date: d, PublishedOn: dateStr + " 00:00:03", Rates: rates}
}

func TestIntegrationUpsertAndLatest(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	usd := nrb.Rate{ISO3: "USD", Name: "U.S. Dollar", Unit: 1, Buy: "152.33", Sell: "152.93"}
	inr := nrb.Rate{ISO3: "INR", Name: "Indian Rupee", Unit: 100, Buy: "160.00", Sell: "160.15"}

	if err := s.UpsertDayRates(ctx, day("2026-07-10", usd, inr)); err != nil {
		t.Fatalf("upserting day 1: %v", err)
	}
	usd2 := usd
	usd2.Buy, usd2.Sell = "152.40", "153.00"
	if err := s.UpsertDayRates(ctx, day("2026-07-11", usd2)); err != nil {
		t.Fatalf("upserting day 2: %v", err)
	}

	latest, err := s.LatestRates(ctx)
	if err != nil {
		t.Fatalf("LatestRates: %v", err)
	}
	if len(latest) != 1 {
		t.Fatalf("latest has %d rates, want 1 (only USD on newest date)", len(latest))
	}
	// trim_scale drops trailing zeros: "152.40" comes back as "152.4".
	if latest[0].ISO3 != "USD" || latest[0].Buy != "152.4" || latest[0].Sell != "153" {
		t.Errorf("unexpected latest rate: %+v", latest[0])
	}
}

func TestIntegrationUpsertRevisesSameDay(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	orig := nrb.Rate{ISO3: "USD", Name: "U.S. Dollar", Unit: 1, Buy: "152.33", Sell: "152.93"}
	if err := s.UpsertDayRates(ctx, day("2026-07-10", orig)); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	revised := orig
	revised.Buy = "152.55"
	if err := s.UpsertDayRates(ctx, day("2026-07-10", revised)); err != nil {
		t.Fatalf("revised upsert: %v", err)
	}

	latest, err := s.LatestRates(ctx)
	if err != nil {
		t.Fatalf("LatestRates: %v", err)
	}
	if len(latest) != 1 || latest[0].Buy != "152.55" {
		t.Fatalf("revision not applied, got %+v", latest)
	}
}

func TestIntegrationRatesRange(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	usd := nrb.Rate{ISO3: "USD", Name: "U.S. Dollar", Unit: 1, Buy: "152.33", Sell: "152.93"}
	inr := nrb.Rate{ISO3: "INR", Name: "Indian Rupee", Unit: 100, Buy: "160.00", Sell: "160.15"}
	for _, d := range []string{"2026-07-08", "2026-07-09", "2026-07-10"} {
		if err := s.UpsertDayRates(ctx, day(d, usd, inr)); err != nil {
			t.Fatalf("upserting %s: %v", d, err)
		}
	}

	from, _ := time.Parse("2006-01-02", "2026-07-09")
	to, _ := time.Parse("2006-01-02", "2026-07-10")

	all, err := s.RatesRange(ctx, from, to, "")
	if err != nil {
		t.Fatalf("RatesRange all: %v", err)
	}
	if len(all) != 4 {
		t.Errorf("got %d rows, want 4 (2 days x 2 currencies)", len(all))
	}

	usdOnly, err := s.RatesRange(ctx, from, to, "USD")
	if err != nil {
		t.Fatalf("RatesRange USD: %v", err)
	}
	if len(usdOnly) != 2 {
		t.Errorf("got %d USD rows, want 2", len(usdOnly))
	}
	for _, r := range usdOnly {
		if r.ISO3 != "USD" {
			t.Errorf("currency filter leaked: %+v", r)
		}
	}
}
