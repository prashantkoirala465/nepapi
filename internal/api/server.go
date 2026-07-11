// Package api implements nepapi's HTTP layer.
package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/prashantkoirala465/nepapi/internal/store"
)

// maxRangeDays caps /v1/forex/rates queries so one request can't ask for
// decades of rows.
const maxRangeDays = 366

// ForexReader is the read surface handlers need; *store.Store satisfies
// it, tests use a fake.
type ForexReader interface {
	LatestRates(ctx context.Context) ([]store.ForexRate, error)
	RatesRange(ctx context.Context, from, to time.Time, iso3 string) ([]store.ForexRate, error)
}

type Server struct {
	forex ForexReader
	log   *slog.Logger
	mux   *http.ServeMux
}

func NewServer(forex ForexReader, log *slog.Logger) *Server {
	s := &Server{forex: forex, log: log, mux: http.NewServeMux()}
	s.mux.HandleFunc("GET /v1/health", s.handleHealth)
	s.mux.HandleFunc("GET /v1/forex/latest", s.handleForexLatest)
	s.mux.HandleFunc("GET /v1/forex/rates", s.handleForexRates)
	return s
}

// Handler returns the full middleware-wrapped handler.
func (s *Server) Handler() http.Handler {
	var h http.Handler = s.mux
	h = rateLimit(h)
	h = cors(h)
	h = requestLog(s.log, h)
	return h
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleForexLatest(w http.ResponseWriter, r *http.Request) {
	rates, err := s.forex.LatestRates(r.Context())
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

	rates, err := s.forex.RatesRange(r.Context(), from, to, q.Get("currency"))
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
