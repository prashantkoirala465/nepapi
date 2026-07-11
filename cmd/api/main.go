// nepapi API server: serves stored forex data and keeps it fresh by
// polling NRB hourly for the current day's publication.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prashantkoirala465/nepapi/internal/api"
	"github.com/prashantkoirala465/nepapi/internal/nrb"
	"github.com/prashantkoirala465/nepapi/internal/store"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := run(log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := store.New(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer st.Close()

	if err := st.Migrate(ctx); err != nil {
		return err
	}
	log.Info("migrations applied")

	client := nrb.NewClient("")
	go pollLoop(ctx, log, client, st)

	apiServer := api.NewServer(api.Config{
		TrustProxy: os.Getenv("TRUST_PROXY") == "true",
	}, st, st, log)
	defer apiServer.Close()

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           apiServer.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	log.Info("listening", "port", port)

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

// pollLoop fetches the last 3 days immediately (covers restarts across
// unpublished days), then re-fetches hourly. NRB publishes once daily
// just after midnight NPT; hourly polling also picks up same-day
// revisions.
func pollLoop(ctx context.Context, log *slog.Logger, client *nrb.Client, st *store.Store) {
	poll := func() {
		to := time.Now()
		from := to.AddDate(0, 0, -3)
		days, err := client.RatesRange(ctx, from, to)
		if err != nil {
			log.Error("poll: fetching rates", "err", err)
			return
		}
		for _, d := range days {
			if err := st.UpsertDayRates(ctx, d); err != nil {
				log.Error("poll: storing rates", "date", d.Date.Format("2006-01-02"), "err", err)
				return
			}
		}
		log.Info("poll: rates refreshed", "days", len(days))
	}

	poll()
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			poll()
		}
	}
}
