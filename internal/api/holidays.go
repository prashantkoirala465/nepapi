package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prashantkoirala465/nepapi/internal/calendar"
	"github.com/prashantkoirala465/nepapi/internal/holiday"
)

// handleHolidays lists Nepal's national public holidays for one BS
// year:
//
//	GET /v1/holidays              (current BS year)
//	GET /v1/holidays?year=2082    (specific BS year)
//
// Pure embedded-data lookup — no database involved.
func (s *Server) handleHolidays(w http.ResponseWriter, r *http.Request) {
	yearStr := r.URL.Query().Get("year")

	var year int
	if yearStr == "" {
		bs, err := calendar.FromAD(time.Now())
		if err != nil {
			s.serverError(w, r, fmt.Errorf("today out of calendar range: %w", err))
			return
		}
		year = bs.Year
	} else {
		y, err := strconv.Atoi(yearStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid 'year' (want a BS year, e.g. 2082)")
			return
		}
		year = y
	}

	hs, ok := holiday.ForYear(year)
	if !ok {
		years := make([]string, 0)
		for _, y := range holiday.Years() {
			years = append(years, strconv.Itoa(y))
		}
		writeError(w, http.StatusNotFound,
			fmt.Sprintf("no holiday data for BS year %d (available: %s)", year, strings.Join(years, ", ")))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"year":     year,
		"count":    len(hs),
		"holidays": hs,
	})
}
