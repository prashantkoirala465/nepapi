// curate-holidays is a maintainer tool for building the holiday
// dataset. It reads scraped hamropatro month files (S4NKALP/
// nepali-calendar-api layout) from a directory, drops Saturdays (the
// weekly holiday), and prints the remaining holiday-flagged days with
// their AD date and festival text for manual curation. The output is
// NOT the dataset — every entry gets verified and named by hand before
// landing in internal/holiday/data.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/prashantkoirala465/nepapi/internal/calendar"
)

type monthFile struct {
	Days []struct {
		N string `json:"n"` // BS day in Devanagari digits; empty = grid padding
		F string `json:"f"` // festival text
		H bool   `json:"h"` // holiday flag (includes Saturdays)
	} `json:"days"`
}

var devanagariDigits = map[rune]int{
	'०': 0, '१': 1, '२': 2, '३': 3, '४': 4,
	'५': 5, '६': 6, '७': 7, '८': 8, '९': 9,
}

func parseDevanagari(s string) (int, bool) {
	n := 0
	for _, r := range s {
		d, ok := devanagariDigits[r]
		if !ok {
			return 0, false
		}
		n = n*10 + d
	}
	return n, s != ""
}

func main() {
	dir := flag.String("dir", "", "directory with <bsyear>-<month>.json files")
	year := flag.Int("year", 0, "BS year to extract")
	flag.Parse()
	if *dir == "" || *year == 0 {
		log.Fatal("usage: curate-holidays -dir <path> -year <bs-year>")
	}

	for m := 1; m <= 12; m++ {
		raw, err := os.ReadFile(filepath.Join(*dir, fmt.Sprintf("%d-%d.json", *year, m)))
		if err != nil {
			log.Fatalf("reading month %d: %v", m, err)
		}
		var mf monthFile
		if err := json.Unmarshal(raw, &mf); err != nil {
			log.Fatalf("parsing month %d: %v", m, err)
		}
		for _, d := range mf.Days {
			day, ok := parseDevanagari(d.N)
			if !ok || !d.H {
				continue
			}
			bs := calendar.Date{Year: *year, Month: m, Day: day}
			ad, err := calendar.ToAD(bs)
			if err != nil {
				log.Fatalf("converting %v: %v", bs, err)
			}
			// Every Saturday carries the holiday flag (weekly holiday);
			// only skip the ones with no festival attached. Saturdays
			// WITH festival text may be real holidays — verify by hand.
			if ad.Weekday() == time.Saturday && d.F == "" {
				continue
			}
			fmt.Printf("%s\t%s\t%s\t%s\n", bs, ad.Format("2006-01-02"), ad.Weekday(), d.F)
		}
	}
}
