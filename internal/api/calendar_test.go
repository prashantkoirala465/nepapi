package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func decodeConvert(t *testing.T, body []byte) convertResponse {
	t.Helper()
	var resp convertResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decoding response: %v (body: %s)", err, body)
	}
	return resp
}

func TestCalendarConvertADToBS(t *testing.T) {
	// 2000-01-01 AD = 2056-09-17 BS (from the medic corpus)
	rec := get(t, testServer(&fakeForex{}), "/v1/calendar/convert?date=2000-01-01")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	resp := decodeConvert(t, rec.Body.Bytes())
	if resp.BS.Year != 2056 || resp.BS.Month != 9 || resp.BS.Day != 17 {
		t.Errorf("BS = %d-%d-%d, want 2056-9-17", resp.BS.Year, resp.BS.Month, resp.BS.Day)
	}
	if resp.BS.MonthName != "Poush" {
		t.Errorf("month name = %q, want Poush", resp.BS.MonthName)
	}
	if resp.AD != "2000-01-01" || resp.Weekday != "Saturday" {
		t.Errorf("ad/weekday = %s/%s, want 2000-01-01/Saturday", resp.AD, resp.Weekday)
	}
}

func TestCalendarConvertBSToAD(t *testing.T) {
	rec := get(t, testServer(&fakeForex{}), "/v1/calendar/convert?date=2056-09-17&from=bs")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	resp := decodeConvert(t, rec.Body.Bytes())
	if resp.AD != "2000-01-01" {
		t.Errorf("AD = %s, want 2000-01-01", resp.AD)
	}
}

func TestCalendarConvertRoundTripsEpoch(t *testing.T) {
	rec := get(t, testServer(&fakeForex{}), "/v1/calendar/convert?date=1913-04-13")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	resp := decodeConvert(t, rec.Body.Bytes())
	if resp.BS.Year != 1970 || resp.BS.Month != 1 || resp.BS.Day != 1 {
		t.Errorf("BS = %d-%d-%d, want 1970-1-1", resp.BS.Year, resp.BS.Month, resp.BS.Day)
	}
}

func TestCalendarConvertValidation(t *testing.T) {
	cases := []struct {
		name string
		path string
	}{
		{"missing date", "/v1/calendar/convert"},
		{"garbage date", "/v1/calendar/convert?date=notadate"},
		{"bad from", "/v1/calendar/convert?date=2026-07-11&from=julian"},
		{"AD before epoch", "/v1/calendar/convert?date=1900-01-01"},
		{"AD after range", "/v1/calendar/convert?date=2040-01-01"},
		{"BS year out of range", "/v1/calendar/convert?date=1950-01-01&from=bs"},
		{"BS impossible day", "/v1/calendar/convert?date=2080-01-32&from=bs"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := get(t, testServer(&fakeForex{}), tc.path)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400; body: %s", rec.Code, rec.Body)
			}
		})
	}
}
