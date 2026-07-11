// Package ingest keeps the database in sync with NRB by polling on an
// interval, with retry/backoff and operational metrics.
package ingest

import (
	"context"
	"expvar"
	"log/slog"
	"time"

	"github.com/prashantkoirala465/nepapi/internal/nrb"
)

// Exposed via the admin server's /debug/vars for monitoring; a poller
// that has stopped succeeding is the failure mode that otherwise goes
// unnoticed for days.
var (
	pollSuccess     = expvar.NewInt("nepapi_poll_success_total")
	pollFailure     = expvar.NewInt("nepapi_poll_failure_total")
	pollLastSuccess = expvar.NewString("nepapi_poll_last_success")
)

// Fetcher is the slice of nrb.Client the poller needs.
type Fetcher interface {
	RatesRange(ctx context.Context, from, to time.Time) ([]nrb.DayRates, error)
}

// Storer is the slice of store.Store the poller needs.
type Storer interface {
	UpsertDayRates(ctx context.Context, day nrb.DayRates) error
}

type Poller struct {
	Fetcher Fetcher
	Storer  Storer
	Log     *slog.Logger

	// Interval between polls; NRB publishes once daily but revises
	// same-day values, so the default is hourly.
	Interval time.Duration
	// Lookback covers restarts across unpublished days.
	Lookback time.Duration
	// RetryBase is the first retry delay; each attempt multiplies it
	// by 5 (1s, 5s, 25s with the default). Tests shrink it.
	RetryBase time.Duration
}

const retryAttempts = 3

func (p *Poller) interval() time.Duration {
	if p.Interval == 0 {
		return time.Hour
	}
	return p.Interval
}

func (p *Poller) lookback() time.Duration {
	if p.Lookback == 0 {
		return 3 * 24 * time.Hour
	}
	return p.Lookback
}

func (p *Poller) retryBase() time.Duration {
	if p.RetryBase == 0 {
		return time.Second
	}
	return p.RetryBase
}

// Run polls immediately, then on every interval tick until ctx is done.
func (p *Poller) Run(ctx context.Context) {
	p.pollOnce(ctx)
	ticker := time.NewTicker(p.interval())
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.pollOnce(ctx)
		}
	}
}

func (p *Poller) pollOnce(ctx context.Context) {
	to := time.Now()
	from := to.Add(-p.lookback())

	days, err := p.fetchWithRetry(ctx, from, to)
	if err != nil {
		pollFailure.Add(1)
		p.Log.Error("poll: fetching rates", "err", err)
		return
	}
	for _, d := range days {
		if err := p.Storer.UpsertDayRates(ctx, d); err != nil {
			pollFailure.Add(1)
			p.Log.Error("poll: storing rates", "date", d.Date.Format("2006-01-02"), "err", err)
			return
		}
	}
	pollSuccess.Add(1)
	pollLastSuccess.Set(time.Now().UTC().Format(time.RFC3339))
	p.Log.Info("poll: rates refreshed", "days", len(days))
}

func (p *Poller) fetchWithRetry(ctx context.Context, from, to time.Time) ([]nrb.DayRates, error) {
	var lastErr error
	delay := p.retryBase()
	for attempt := 1; attempt <= retryAttempts; attempt++ {
		days, err := p.Fetcher.RatesRange(ctx, from, to)
		if err == nil {
			return days, nil
		}
		lastErr = err
		if attempt < retryAttempts {
			p.Log.Warn("poll: fetch failed, retrying", "attempt", attempt, "delay", delay, "err", err)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
			delay *= 5
		}
	}
	return nil, lastErr
}
