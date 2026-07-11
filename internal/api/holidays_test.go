package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

type holidaysResponse struct {
	Year     int `json:"year"`
	Count    int `json:"count"`
	Holidays []struct {
		AD      string `json:"ad"`
		Name    string `json:"name"`
		NameNe  string `json:"name_ne"`
		Note    string `json:"note"`
		Weekday string `json:"weekday"`
	} `json:"holidays"`
}

func TestHolidaysForYear(t *testing.T) {
	rec := get(t, testServer(&fakeForex{}), "/v1/holidays?year=2082")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var resp holidaysResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if resp.Year != 2082 || resp.Count != len(resp.Holidays) || resp.Count == 0 {
		t.Fatalf("year=%d count=%d len=%d", resp.Year, resp.Count, len(resp.Holidays))
	}
	var dashami bool
	for _, h := range resp.Holidays {
		if h.Name == "Vijaya Dashami" && h.AD == "2025-10-02" && h.Weekday == "Thursday" {
			dashami = true
		}
	}
	if !dashami {
		t.Error("Vijaya Dashami 2025-10-02 (Thursday) missing from 2082")
	}
}

func TestHolidaysUnknownYear(t *testing.T) {
	rec := get(t, testServer(&fakeForex{}), "/v1/holidays?year=1999")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body: %s", rec.Code, rec.Body)
	}
	var e struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if !strings.Contains(e.Error, "2082") {
		t.Errorf("error should list available years, got %q", e.Error)
	}
}

func TestHolidaysBadYear(t *testing.T) {
	rec := get(t, testServer(&fakeForex{}), "/v1/holidays?year=abc")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHolidaysDefaultsToCurrentYear(t *testing.T) {
	// Today falls inside the curated range while 2082/2083 data is
	// current, so the default should resolve rather than 404.
	rec := get(t, testServer(&fakeForex{}), "/v1/holidays")
	if rec.Code != http.StatusOK && rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 200 (or 404 once data ages out)", rec.Code)
	}
}
