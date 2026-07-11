package api

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/prashantkoirala465/nepapi/internal/calendar"
)

// bsDateResponse is the BS half of a conversion response.
type bsDateResponse struct {
	Year        int    `json:"year"`
	Month       int    `json:"month"`
	Day         int    `json:"day"`
	MonthName   string `json:"month_name"`
	MonthNameNe string `json:"month_name_ne"`
}

type convertResponse struct {
	BS      bsDateResponse `json:"bs"`
	AD      string         `json:"ad"`
	Weekday string         `json:"weekday"`
}

// handleCalendarConvert converts between AD and BS dates:
//
//	GET /v1/calendar/convert?date=2026-07-11            (AD → BS, default)
//	GET /v1/calendar/convert?date=2083-03-27&from=bs    (BS → AD)
//
// Conversion is pure table lookup — no database involved.
func (s *Server) handleCalendarConvert(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	from := q.Get("from")
	if from == "" {
		from = "ad"
	}
	if from != "ad" && from != "bs" {
		writeError(w, http.StatusBadRequest, "invalid 'from' (want ad or bs)")
		return
	}

	dateStr := q.Get("date")
	var (
		bs calendar.Date
		ad time.Time
	)
	switch from {
	case "ad":
		t, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid or missing 'date' (want YYYY-MM-DD)")
			return
		}
		ad = t
		bs, err = calendar.FromAD(t)
		if err != nil {
			writeCalendarError(w, err)
			return
		}
	case "bs":
		var y, m, d int
		if _, err := fmt.Sscanf(dateStr, "%d-%d-%d", &y, &m, &d); err != nil || len(dateStr) != 10 {
			writeError(w, http.StatusBadRequest, "invalid or missing 'date' (want YYYY-MM-DD)")
			return
		}
		bs = calendar.Date{Year: y, Month: m, Day: d}
		t, err := calendar.ToAD(bs)
		if err != nil {
			writeCalendarError(w, err)
			return
		}
		ad = t
	}

	writeJSON(w, http.StatusOK, convertResponse{
		BS: bsDateResponse{
			Year:        bs.Year,
			Month:       bs.Month,
			Day:         bs.Day,
			MonthName:   bs.MonthName(),
			MonthNameNe: bs.MonthNameNepali(),
		},
		AD:      ad.Format("2006-01-02"),
		Weekday: ad.Weekday().String(),
	})
}

// writeCalendarError maps conversion failures to 400s with the
// package's message, which already explains the supported range or the
// invalid component.
func writeCalendarError(w http.ResponseWriter, err error) {
	var re *calendar.RangeError
	if errors.As(err, &re) {
		writeError(w, http.StatusBadRequest, re.Error())
		return
	}
	writeError(w, http.StatusBadRequest, err.Error())
}
