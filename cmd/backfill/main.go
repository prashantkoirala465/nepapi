// backfill loads historical NRB forex rates into the database in
// month-sized chunks: go run ./cmd/backfill -from 2020-01-01 -to 2026-07-11
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/prashantkoirala465/nepapi/internal/nrb"
	"github.com/prashantkoirala465/nepapi/internal/store"
)

func main() {
	fromStr := flag.String("from", "", "start date (YYYY-MM-DD)")
	toStr := flag.String("to", "", "end date (YYYY-MM-DD)")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(log, *fromStr, *toStr); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger, fromStr, toStr string) error {
	from, err := time.Parse("2006-01-02", fromStr)
	if err != nil {
		return fmt.Errorf("invalid -from: %w", err)
	}
	to, err := time.Parse("2006-01-02", toStr)
	if err != nil {
		return fmt.Errorf("invalid -to: %w", err)
	}
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}

	ctx := context.Background()
	st, err := store.New(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer st.Close()
	if err := st.Migrate(ctx); err != nil {
		return err
	}

	client := nrb.NewClient("")
	total := 0
	for chunkStart := from; !chunkStart.After(to); chunkStart = chunkStart.AddDate(0, 1, 0) {
		chunkEnd := chunkStart.AddDate(0, 1, -1)
		if chunkEnd.After(to) {
			chunkEnd = to
		}
		days, err := client.RatesRange(ctx, chunkStart, chunkEnd)
		if err != nil {
			return fmt.Errorf("fetching %s..%s: %w", chunkStart.Format("2006-01-02"), chunkEnd.Format("2006-01-02"), err)
		}
		for _, d := range days {
			if err := st.UpsertDayRates(ctx, d); err != nil {
				return fmt.Errorf("storing %s: %w", d.Date.Format("2006-01-02"), err)
			}
		}
		total += len(days)
		log.Info("chunk done", "from", chunkStart.Format("2006-01-02"), "to", chunkEnd.Format("2006-01-02"), "days", len(days))
		// stay polite to NRB's servers
		time.Sleep(500 * time.Millisecond)
	}
	log.Info("backfill complete", "days", total)
	return nil
}
