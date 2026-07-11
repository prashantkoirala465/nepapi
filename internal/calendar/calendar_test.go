package calendar

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

// TestAgainstFullCorpus validates both conversion directions against
// 30,572 known BS→AD pairs from medic/bikram-sambat (production
// healthcare software). Every pair is checked both ways.
func TestAgainstFullCorpus(t *testing.T) {
	raw, err := os.ReadFile("testdata/toGreg.json")
	if err != nil {
		t.Fatalf("reading corpus: %v", err)
	}
	var pairs []struct {
		BS   [3]int `json:"bs"`
		Greg [3]int `json:"expectedGreg"`
	}
	if err := json.Unmarshal(raw, &pairs); err != nil {
		t.Fatalf("parsing corpus: %v", err)
	}
	if len(pairs) < 30000 {
		t.Fatalf("corpus suspiciously small: %d pairs", len(pairs))
	}

	for _, p := range pairs {
		bs := Date{Year: p.BS[0], Month: p.BS[1], Day: p.BS[2]}
		wantAD := time.Date(p.Greg[0], time.Month(p.Greg[1]), p.Greg[2], 0, 0, 0, 0, time.UTC)

		gotAD, err := ToAD(bs)
		if err != nil {
			t.Fatalf("ToAD(%v): %v", bs, err)
		}
		if !gotAD.Equal(wantAD) {
			t.Fatalf("ToAD(%v) = %s, want %s", bs, gotAD.Format("2006-01-02"), wantAD.Format("2006-01-02"))
		}

		gotBS, err := FromAD(wantAD)
		if err != nil {
			t.Fatalf("FromAD(%s): %v", wantAD.Format("2006-01-02"), err)
		}
		if gotBS != bs {
			t.Fatalf("FromAD(%s) = %v, want %v", wantAD.Format("2006-01-02"), gotBS, bs)
		}
	}
}

func TestEpoch(t *testing.T) {
	got, err := ToAD(Date{1970, 1, 1})
	if err != nil {
		t.Fatalf("ToAD(epoch): %v", err)
	}
	if !got.Equal(EpochAD) {
		t.Errorf("ToAD(1970-01-01 BS) = %s, want %s", got.Format("2006-01-02"), EpochAD.Format("2006-01-02"))
	}
}

func TestValidateRejectsBadDates(t *testing.T) {
	cases := []struct {
		name string
		d    Date
	}{
		{"year too early", Date{1969, 1, 1}},
		{"year too late", Date{2091, 1, 1}},
		{"month zero", Date{2080, 0, 1}},
		{"month thirteen", Date{2080, 13, 1}},
		{"day zero", Date{2080, 1, 0}},
		{"day beyond month length", Date{2080, 1, 32}}, // Baishakh 2080 has 31 days
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.d.Validate(); err == nil {
				t.Errorf("Validate(%v) = nil, want error", tc.d)
			}
		})
	}
}

func TestFromADRejectsOutOfRange(t *testing.T) {
	for _, d := range []time.Time{
		time.Date(1913, time.April, 12, 0, 0, 0, 0, time.UTC), // day before epoch
		time.Date(2035, time.January, 1, 0, 0, 0, 0, time.UTC),
	} {
		if _, err := FromAD(d); err == nil {
			t.Errorf("FromAD(%s) = nil error, want out-of-range", d.Format("2006-01-02"))
		}
	}
}

func TestMonthNames(t *testing.T) {
	d := Date{2080, 3, 15}
	if d.MonthName() != "Ashadh" || d.MonthNameNepali() != "असार" {
		t.Errorf("month 3 names = %q/%q, want Ashadh/असार", d.MonthName(), d.MonthNameNepali())
	}
}

func BenchmarkFromAD(b *testing.B) {
	d := time.Date(2026, time.July, 11, 0, 0, 0, 0, time.UTC)
	for i := 0; i < b.N; i++ {
		if _, err := FromAD(d); err != nil {
			b.Fatal(err)
		}
	}
}
