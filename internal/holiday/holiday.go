// Package holiday serves Nepal's national public holidays from a
// hand-curated, per-BS-year embedded dataset. See data/ATTRIBUTION.md
// for provenance and curation rules.
package holiday

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/prashantkoirala465/nepapi/internal/calendar"
)

//go:embed data/*.json
var dataFS embed.FS

// Holiday is one public holiday with both calendar representations.
type Holiday struct {
	BS      calendar.Date `json:"bs"`
	AD      string        `json:"ad"`
	Weekday string        `json:"weekday"`
	Name    string        `json:"name"`
	NameNe  string        `json:"name_ne"`
	Note    string        `json:"note,omitempty"`
}

var byYear map[int][]Holiday

type rawEntry struct {
	BS     string `json:"bs"`
	Name   string `json:"name"`
	NameNe string `json:"name_ne"`
	Note   string `json:"note"`
}

func init() {
	byYear = make(map[int][]Holiday)

	entries, err := dataFS.ReadDir("data")
	if err != nil {
		panic(fmt.Sprintf("holiday: reading embedded data: %v", err))
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		year, err := strconv.Atoi(strings.TrimSuffix(e.Name(), ".json"))
		if err != nil {
			panic(fmt.Sprintf("holiday: data file %q is not <bs-year>.json", e.Name()))
		}

		raw, err := dataFS.ReadFile("data/" + e.Name())
		if err != nil {
			panic(fmt.Sprintf("holiday: reading %s: %v", e.Name(), err))
		}
		var list []rawEntry
		if err := json.Unmarshal(raw, &list); err != nil {
			panic(fmt.Sprintf("holiday: parsing %s: %v", e.Name(), err))
		}

		hs := make([]Holiday, 0, len(list))
		for _, r := range list {
			var y, m, d int
			if _, err := fmt.Sscanf(r.BS, "%d-%d-%d", &y, &m, &d); err != nil {
				panic(fmt.Sprintf("holiday: bad bs date %q in %s", r.BS, e.Name()))
			}
			if y != year {
				panic(fmt.Sprintf("holiday: entry %q filed under year %d", r.BS, year))
			}
			bs := calendar.Date{Year: y, Month: m, Day: d}
			ad, err := calendar.ToAD(bs)
			if err != nil {
				panic(fmt.Sprintf("holiday: %q does not convert: %v", r.BS, err))
			}
			hs = append(hs, Holiday{
				BS:      bs,
				AD:      ad.Format("2006-01-02"),
				Weekday: ad.Weekday().String(),
				Name:    r.Name,
				NameNe:  r.NameNe,
				Note:    r.Note,
			})
		}
		sort.SliceStable(hs, func(i, j int) bool {
			a, b := hs[i].BS, hs[j].BS
			if a.Month != b.Month {
				return a.Month < b.Month
			}
			return a.Day < b.Day
		})
		byYear[year] = hs
	}
}

// Years returns the BS years with data, ascending.
func Years() []int {
	ys := make([]int, 0, len(byYear))
	for y := range byYear {
		ys = append(ys, y)
	}
	sort.Ints(ys)
	return ys
}

// ForYear returns the holidays of one BS year in date order; ok is
// false when the year has no dataset.
func ForYear(bsYear int) (hs []Holiday, ok bool) {
	hs, ok = byYear[bsYear]
	return hs, ok
}

// On returns the holidays falling on one Gregorian day (usually zero or
// one; two when observances coincide).
func On(t time.Time) []Holiday {
	bs, err := calendar.FromAD(t)
	if err != nil {
		return nil
	}
	var out []Holiday
	for _, h := range byYear[bs.Year] {
		if h.BS == bs {
			out = append(out, h)
		}
	}
	return out
}
