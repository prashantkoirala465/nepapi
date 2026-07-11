package ingest

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/prashantkoirala465/nepapi/internal/nrb"
)

type fakeFetcher struct {
	failures int // fail this many calls before succeeding
	calls    int
	days     []nrb.DayRates
}

func (f *fakeFetcher) RatesRange(ctx context.Context, from, to time.Time) ([]nrb.DayRates, error) {
	f.calls++
	if f.calls <= f.failures {
		return nil, errors.New("nrb unreachable")
	}
	return f.days, nil
}

type fakeStorer struct {
	stored []nrb.DayRates
	err    error
}

func (s *fakeStorer) UpsertDayRates(ctx context.Context, day nrb.DayRates) error {
	if s.err != nil {
		return s.err
	}
	s.stored = append(s.stored, day)
	return nil
}

func testPoller(f Fetcher, s Storer) *Poller {
	return &Poller{
		Fetcher:   f,
		Storer:    s,
		Log:       slog.New(slog.DiscardHandler),
		RetryBase: time.Millisecond,
	}
}

func someDay() nrb.DayRates {
	d, _ := time.Parse("2006-01-02", "2026-07-11")
	return nrb.DayRates{Date: d, Rates: []nrb.Rate{{ISO3: "USD", Name: "U.S. Dollar", Unit: 1, Buy: "152.33", Sell: "152.93"}}}
}

func TestPollOnceStoresFetchedDays(t *testing.T) {
	f := &fakeFetcher{days: []nrb.DayRates{someDay()}}
	s := &fakeStorer{}
	testPoller(f, s).pollOnce(context.Background())

	if len(s.stored) != 1 {
		t.Fatalf("stored %d days, want 1", len(s.stored))
	}
	if f.calls != 1 {
		t.Errorf("fetcher called %d times, want 1", f.calls)
	}
}

func TestPollOnceRetriesTransientFailures(t *testing.T) {
	f := &fakeFetcher{failures: 2, days: []nrb.DayRates{someDay()}}
	s := &fakeStorer{}
	testPoller(f, s).pollOnce(context.Background())

	if f.calls != 3 {
		t.Errorf("fetcher called %d times, want 3 (2 failures + success)", f.calls)
	}
	if len(s.stored) != 1 {
		t.Fatalf("stored %d days, want 1 after retries", len(s.stored))
	}
}

func TestPollOnceGivesUpAfterMaxRetries(t *testing.T) {
	f := &fakeFetcher{failures: 10}
	s := &fakeStorer{}
	testPoller(f, s).pollOnce(context.Background())

	if f.calls != retryAttempts {
		t.Errorf("fetcher called %d times, want %d", f.calls, retryAttempts)
	}
	if len(s.stored) != 0 {
		t.Errorf("stored %d days, want 0 on total failure", len(s.stored))
	}
}

func TestPollOnceStopsOnStoreError(t *testing.T) {
	f := &fakeFetcher{days: []nrb.DayRates{someDay(), someDay()}}
	s := &fakeStorer{err: errors.New("db down")}
	testPoller(f, s).pollOnce(context.Background())

	if len(s.stored) != 0 {
		t.Errorf("stored %d days, want 0 when the store errors", len(s.stored))
	}
}

func TestRunRespectsContextCancellation(t *testing.T) {
	f := &fakeFetcher{days: []nrb.DayRates{someDay()}}
	s := &fakeStorer{}
	p := testPoller(f, s)
	p.Interval = time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		p.Run(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
}
