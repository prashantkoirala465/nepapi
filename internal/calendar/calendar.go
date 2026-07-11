// Package calendar converts between Bikram Sambat (the official Nepali
// calendar) and Gregorian (AD) dates.
//
// BS months have no arithmetic rule — their lengths (29-32 days) come
// from astronomical calculation and are published per year, so
// conversion is table-driven: an embedded dataset of month lengths for
// 1970-2090 BS anchored at 1970-01-01 BS = 1913-04-13 AD. See
// data/ATTRIBUTION.md for the dataset's provenance.
package calendar

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"
)

//go:embed data/days_in_month.json
var daysInMonthJSON []byte

const (
	// EpochYearBS is the first year in the dataset; 1 Baishakh of that
	// year corresponds to EpochAD.
	EpochYearBS = 1970
)

// EpochAD is the Gregorian date of 1970-01-01 BS.
var EpochAD = time.Date(1913, time.April, 13, 0, 0, 0, 0, time.UTC)

var (
	// daysInMonth[y-EpochYearBS][m-1] is the length of month m in BS year y.
	daysInMonth [][12]int
	// yearStart[y-EpochYearBS] is the day offset from EpochAD to 1 Baishakh
	// of BS year y; the final extra entry marks the end of the last year.
	yearStart []int
)

func init() {
	var raw map[string][12]int
	if err := json.Unmarshal(daysInMonthJSON, &raw); err != nil {
		panic(fmt.Sprintf("calendar: corrupt embedded dataset: %v", err))
	}

	years := make([]int, 0, len(raw))
	for y := range raw {
		n, err := strconv.Atoi(y)
		if err != nil {
			panic(fmt.Sprintf("calendar: bad year key %q", y))
		}
		years = append(years, n)
	}
	sort.Ints(years)
	for i, y := range years {
		if y != EpochYearBS+i {
			panic(fmt.Sprintf("calendar: dataset has a gap at BS year %d", y))
		}
	}

	daysInMonth = make([][12]int, len(years))
	yearStart = make([]int, len(years)+1)
	offset := 0
	for i, y := range years {
		daysInMonth[i] = raw[strconv.Itoa(y)]
		yearStart[i] = offset
		for _, d := range daysInMonth[i] {
			offset += d
		}
	}
	yearStart[len(years)] = offset
}

// MaxYearBS is the last fully supported BS year.
func MaxYearBS() int { return EpochYearBS + len(daysInMonth) - 1 }

// monthNames are the romanized and Devanagari names of BS months 1-12.
var monthNames = [12][2]string{
	{"Baishakh", "वैशाख"}, {"Jestha", "जेठ"}, {"Ashadh", "असार"},
	{"Shrawan", "साउन"}, {"Bhadra", "भदौ"}, {"Ashwin", "असोज"},
	{"Kartik", "कात्तिक"}, {"Mangsir", "मंसिर"}, {"Poush", "पुस"},
	{"Magh", "माघ"}, {"Falgun", "फागुन"}, {"Chaitra", "चैत"},
}

// Date is a Bikram Sambat calendar date.
type Date struct {
	Year  int `json:"year"`
	Month int `json:"month"`
	Day   int `json:"day"`
}

// MonthName returns the romanized month name (e.g. "Ashadh").
func (d Date) MonthName() string { return monthNames[d.Month-1][0] }

// MonthNameNepali returns the Devanagari month name (e.g. "असार").
func (d Date) MonthNameNepali() string { return monthNames[d.Month-1][1] }

func (d Date) String() string {
	return fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Day)
}

// RangeError reports a date outside the dataset's coverage.
type RangeError struct{ msg string }

func (e *RangeError) Error() string { return e.msg }

func rangeErrorf(format string, args ...any) error {
	return &RangeError{msg: fmt.Sprintf(format, args...)}
}

// Validate checks that d is a real date within the supported range.
func (d Date) Validate() error {
	if d.Year < EpochYearBS || d.Year > MaxYearBS() {
		return rangeErrorf("calendar: BS year %d out of supported range %d..%d", d.Year, EpochYearBS, MaxYearBS())
	}
	if d.Month < 1 || d.Month > 12 {
		return fmt.Errorf("calendar: BS month %d out of range 1..12", d.Month)
	}
	if max := daysInMonth[d.Year-EpochYearBS][d.Month-1]; d.Day < 1 || d.Day > max {
		return fmt.Errorf("calendar: BS day %d out of range 1..%d for %s %d", d.Day, max, d.MonthName(), d.Year)
	}
	return nil
}

// ToAD converts a BS date to its Gregorian equivalent (UTC midnight).
func ToAD(d Date) (time.Time, error) {
	if err := d.Validate(); err != nil {
		return time.Time{}, err
	}
	yi := d.Year - EpochYearBS
	offset := yearStart[yi]
	for m := 0; m < d.Month-1; m++ {
		offset += daysInMonth[yi][m]
	}
	offset += d.Day - 1
	return EpochAD.AddDate(0, 0, offset), nil
}

// FromAD converts a Gregorian date to its BS equivalent. Only the
// year/month/day of t are considered.
func FromAD(t time.Time) (Date, error) {
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	offset := int(day.Sub(EpochAD).Hours() / 24)
	if offset < 0 || offset >= yearStart[len(yearStart)-1] {
		last, _ := ToAD(Date{MaxYearBS(), 12, daysInMonth[len(daysInMonth)-1][11]})
		return Date{}, rangeErrorf("calendar: AD date %s out of supported range %s..%s",
			day.Format("2006-01-02"), EpochAD.Format("2006-01-02"), last.Format("2006-01-02"))
	}

	yi := sort.Search(len(yearStart), func(i int) bool { return yearStart[i] > offset }) - 1
	rem := offset - yearStart[yi]
	month := 0
	for rem >= daysInMonth[yi][month] {
		rem -= daysInMonth[yi][month]
		month++
	}
	return Date{Year: EpochYearBS + yi, Month: month + 1, Day: rem + 1}, nil
}
