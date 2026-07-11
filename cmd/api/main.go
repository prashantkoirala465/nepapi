// nepapi API server: serves stored forex data and keeps it fresh by
// polling NRB hourly for the current day's publication.
package main

import (
	"context"
	"errors"
	"expvar"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prashantkoirala465/nepapi/internal/api"
	"github.com/prashantkoirala465/nepapi/internal/ingest"
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

	poller := &ingest.Poller{
		Fetcher: nrb.NewClient(""),
		Storer:  st,
		Log:     log,
	}
	go poller.Run(ctx)

	go serveAdmin(ctx, log)

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

// serveAdmin exposes expvar metrics (poll counters, memstats) on a
// localhost-only listener, kept off the public port on purpose.
func serveAdmin(ctx context.Context, log *slog.Logger) {
	port := os.Getenv("ADMIN_PORT")
	if port == "" {
		port = "8081"
	}
	mux := http.NewServeMux()
	mux.Handle("GET /debug/vars", expvar.Handler())

	srv := &http.Server{
		Addr:              "127.0.0.1:" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()
	log.Info("admin listening", "addr", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("admin server", "err", err)
	}
}
