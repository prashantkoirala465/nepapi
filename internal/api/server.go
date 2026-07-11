// Package api implements nepapi's HTTP layer.
package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/time/rate"

	"github.com/prashantkoirala465/nepapi/internal/store"
)

// maxRangeDays caps /v1/forex/rates queries so one request can't ask for
// decades of rows.
const maxRangeDays = 366

// queryTimeout bounds each handler's database work; a slow query
// shouldn't hold a connection for as long as the client cares to wait.
const queryTimeout = 5 * time.Second

// ForexReader is the read surface handlers need; *store.Store satisfies
// it, tests use a fake.
type ForexReader interface {
	LatestRates(ctx context.Context) ([]store.ForexRate, error)
	RatesRange(ctx context.Context, from, to time.Time, iso3 string) ([]store.ForexRate, error)
}

// Pinger reports whether the backing database is reachable.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Config carries deployment-specific server settings.
type Config struct {
	// TrustProxy enables X-Forwarded-For for client identification.
	// Only set when running behind a reverse proxy you control.
	TrustProxy bool
	// RateRPS/RateBurst shape the per-IP token bucket. Zero values
	// fall back to 2 req/s with bursts of 30.
	RateRPS   float64
	RateBurst int
}

func (c *Config) applyDefaults() {
	if c.RateRPS == 0 {
		c.RateRPS = 2
	}
	if c.RateBurst == 0 {
		c.RateBurst = 30
	}
}

type Server struct {
	cfg     Config
	forex   ForexReader
	db      Pinger
	log     *slog.Logger
	mux     *http.ServeMux
	limiter *ipLimiter
}

func NewServer(cfg Config, forex ForexReader, db Pinger, log *slog.Logger) *Server {
	cfg.applyDefaults()
	s := &Server{
		cfg:     cfg,
		forex:   forex,
		db:      db,
		log:     log,
		mux:     http.NewServeMux(),
		limiter: newIPLimiter(rate.Limit(cfg.RateRPS), cfg.RateBurst),
	}
	for _, rt := range s.routes() {
		s.mux.HandleFunc(rt.method+" "+rt.path, rt.handler)
	}
	return s
}

type route struct {
	method  string
	path    string
	handler http.HandlerFunc
}

// routes is the single source of truth for the API surface; the OpenAPI
// drift test compares it against openapi.yaml.
func (s *Server) routes() []route {
	return []route{
		{http.MethodGet, "/v1/health", s.handleHealth},
		{http.MethodGet, "/v1/ready", s.handleReady},
		{http.MethodGet, "/v1/forex/latest", s.handleForexLatest},
		{http.MethodGet, "/v1/forex/rates", s.handleForexRates},
		{http.MethodGet, "/v1/calendar/convert", s.handleCalendarConvert},
		{http.MethodGet, "/v1/openapi.yaml", s.handleOpenAPISpec},
		{http.MethodGet, "/docs", s.handleDocs},
	}
}

// Close releases background resources (the limiter's sweep goroutine).
func (s *Server) Close() { s.limiter.Stop() }

// Handler returns the full middleware-wrapped handler.
func (s *Server) Handler() http.Handler {
	var h http.Handler = s.mux
	h = s.rateLimit(h)
	h = cors(h)
	h = s.requestLog(h)
	return h
}

// handleHealth is a liveness probe: the process is up and serving.
// It deliberately checks nothing else.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleReady is a readiness probe: 200 only if the database answers.
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.db.Ping(ctx); err != nil {
		s.log.Error("readiness check failed", "err", err)
		writeError(w, http.StatusServiceUnavailable, "database unreachable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) handleForexLatest(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), queryTimeout)
	defer cancel()
	rates, err := s.forex.LatestRates(ctx)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	if len(rates) == 0 {
		writeError(w, http.StatusNotFound, "no rates available yet")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"date":  rates[0].Date.Format("2006-01-02"),
		"rates": rates,
	})
}

func (s *Server) handleForexRates(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	from, err := time.Parse("2006-01-02", q.Get("from"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid or missing 'from' (want YYYY-MM-DD)")
		return
	}
	to, err := time.Parse("2006-01-02", q.Get("to"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid or missing 'to' (want YYYY-MM-DD)")
		return
	}
	if to.Before(from) {
		writeError(w, http.StatusBadRequest, "'to' is before 'from'")
		return
	}
	if to.Sub(from) > maxRangeDays*24*time.Hour {
		writeError(w, http.StatusBadRequest, "range too large (max 366 days)")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), queryTimeout)
	defer cancel()
	rates, err := s.forex.RatesRange(ctx, from, to, q.Get("currency"))
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"from":  from.Format("2006-01-02"),
		"to":    to.Format("2006-01-02"),
		"count": len(rates),
		"rates": rates,
	})
}

func (s *Server) serverError(w http.ResponseWriter, r *http.Request, err error) {
	s.log.Error("handler error", "path", r.URL.Path, "err", err)
	writeError(w, http.StatusInternalServerError, "internal error")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
