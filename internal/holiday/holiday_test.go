package holiday

import (
	"testing"
	"time"

	"github.com/prashantkoirala465/nepapi/internal/calendar"
)

// The init function already panics on unparseable files, entries filed
// under the wrong year, and BS dates that don't convert — so much of
// the dataset's validity is enforced by the package loading at all.

func TestYearsPresent(t *testing.T) {
	ys := Years()
	if len(ys) < 2 {
		t.Fatalf("expected at least 2 years of data, got %v", ys)
	}
	for i := 1; i < len(ys); i++ {
		if ys[i] <= ys[i-1] {
			t.Errorf("Years() not ascending: %v", ys)
		}
	}
}

func TestForYearKnownAnchors(t *testing.T) {
	hs, ok := ForYear(2082)
	if !ok {
		t.Fatal("no data for 2082")
	}

	anchors := map[string]string{ // name -> expected AD date
		"Vijaya Dashami":   "2025-10-02",
		"Constitution Day": "2025-09-19",
		"Republic Day":     "2025-05-29",
		"Buddha Jayanti":   "2025-05-12",
		"Maha Shivaratri":  "2026-02-15",
	}
	found := map[string]string{}
	for _, h := range hs {
		found[h.Name] = h.AD
	}
	for name, want := range anchors {
		if got, ok := found[name]; !ok {
			t.Errorf("2082 missing %q", name)
		} else if got != want {
			t.Errorf("%s = %s, want %s", name, got, want)
		}
	}
}

func TestForYearSorted(t *testing.T) {
	for _, y := range Years() {
		hs, _ := ForYear(y)
		for i := 1; i < len(hs); i++ {
			prev, cur := hs[i-1].BS, hs[i].BS
			if cur.Month < prev.Month || (cur.Month == prev.Month && cur.Day < prev.Day) {
				t.Errorf("year %d not sorted at %v -> %v", y, prev, cur)
			}
		}
	}
}

func TestForYearUnknown(t *testing.T) {
	if _, ok := ForYear(1999); ok {
		t.Error("ForYear(1999) reported data")
	}
}

func TestOn(t *testing.T) {
	// 2025-10-02 is Vijaya Dashami 2082.
	hs := On(time.Date(2025, time.October, 2, 0, 0, 0, 0, time.UTC))
	if len(hs) != 1 || hs[0].Name != "Vijaya Dashami" {
		t.Errorf("On(2025-10-02) = %+v, want Vijaya Dashami", hs)
	}

	// 2026-05-01 is both Workers' Day and Buddha Jayanti in 2083.
	hs = On(time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC))
	if len(hs) != 2 {
		t.Errorf("On(2026-05-01) = %d holidays, want 2 (Workers' Day + Buddha Jayanti)", len(hs))
	}

	// A regular working day.
	if hs := On(time.Date(2025, time.June, 18, 0, 0, 0, 0, time.UTC)); len(hs) != 0 {
		t.Errorf("On(2025-06-18) = %+v, want none", hs)
	}
}

func TestWeekdayConsistency(t *testing.T) {
	for _, y := range Years() {
		hs, _ := ForYear(y)
		for _, h := range hs {
			ad, err := calendar.ToAD(h.BS)
			if err != nil {
				t.Fatalf("%v: %v", h.BS, err)
			}
			if ad.Weekday().String() != h.Weekday {
				t.Errorf("%v: weekday %s, calendar says %s", h.BS, h.Weekday, ad.Weekday())
			}
		}
	}
}
