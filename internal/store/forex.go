package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/prashantkoirala465/nepapi/internal/nrb"
)

// ForexRate is one currency's stored quote for one date. Buy and Sell
// are decimal strings (e.g. "152.33"): Postgres numeric in, exact
// digits out — floats never touch money on the way through.
type ForexRate struct {
	Date time.Time `json:"date"`
	ISO3 string    `json:"iso3"`
	Name string    `json:"name"`
	Unit int       `json:"unit"`
	Buy  string    `json:"buy"`
	Sell string    `json:"sell"`
}

// UpsertDayRates stores all rates for one published day, replacing any
// previously fetched values (NRB occasionally revises same-day rates).
// All rows go in one batched round-trip inside one transaction — a
// year's backfill is ~7000 rows and per-row round-trips add up.
func (s *Store) UpsertDayRates(ctx context.Context, day nrb.DayRates) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("store: beginning tx: %w", err)
	}
	defer tx.Rollback(ctx)

	batch := &pgx.Batch{}
	for _, r := range day.Rates {
		batch.Queue(`
			INSERT INTO forex_rates (date, currency_iso3, currency_name, unit, buy, sell, published_on)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (date, currency_iso3) DO UPDATE SET
				currency_name = EXCLUDED.currency_name,
				unit = EXCLUDED.unit,
				buy = EXCLUDED.buy,
				sell = EXCLUDED.sell,
				published_on = EXCLUDED.published_on,
				fetched_at = now()`,
			day.Date, r.ISO3, r.Name, r.Unit, r.Buy, r.Sell, parsePublishedOn(day.PublishedOn),
		)
	}

	br := tx.SendBatch(ctx, batch)
	for _, r := range day.Rates {
		if _, err := br.Exec(); err != nil {
			br.Close()
			return fmt.Errorf("store: upserting %s %s: %w", day.Date.Format("2006-01-02"), r.ISO3, err)
		}
	}
	if err := br.Close(); err != nil {
		return fmt.Errorf("store: closing batch: %w", err)
	}
	return tx.Commit(ctx)
}

// parsePublishedOn converts NRB's "2006-01-02 15:04:05" local timestamps.
// Returns nil (SQL NULL) if the value doesn't parse.
func parsePublishedOn(v string) *time.Time {
	loc, err := time.LoadLocation("Asia/Kathmandu")
	if err != nil {
		loc = time.UTC
	}
	t, err := time.ParseInLocation("2006-01-02 15:04:05", v, loc)
	if err != nil {
		return nil
	}
	return &t
}

// LatestRates returns all rates for the most recent stored date.
func (s *Store) LatestRates(ctx context.Context) ([]ForexRate, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT date, currency_iso3, currency_name, unit,
		       trim_scale(buy)::text, trim_scale(sell)::text
		FROM forex_rates
		WHERE date = (SELECT max(date) FROM forex_rates)
		ORDER BY currency_iso3`)
	if err != nil {
		return nil, fmt.Errorf("store: querying latest rates: %w", err)
	}
	defer rows.Close()
	return scanRates(rows)
}

// RatesRange returns rates within [from, to], optionally filtered to one
// currency (ISO3), ordered by date then currency.
func (s *Store) RatesRange(ctx context.Context, from, to time.Time, iso3 string) ([]ForexRate, error) {
	query := `
		SELECT date, currency_iso3, currency_name, unit,
		       trim_scale(buy)::text, trim_scale(sell)::text
		FROM forex_rates
		WHERE date BETWEEN $1 AND $2`
	args := []any{from, to}
	if iso3 != "" {
		query += ` AND currency_iso3 = $3`
		args = append(args, iso3)
	}
	query += ` ORDER BY date, currency_iso3`

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: querying rates range: %w", err)
	}
	defer rows.Close()
	return scanRates(rows)
}

type pgxRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

func scanRates(rows pgxRows) ([]ForexRate, error) {
	var out []ForexRate
	for rows.Next() {
		var r ForexRate
		if err := rows.Scan(&r.Date, &r.ISO3, &r.Name, &r.Unit, &r.Buy, &r.Sell); err != nil {
			return nil, fmt.Errorf("store: scanning rate: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterating rates: %w", err)
	}
	return out, nil
}
