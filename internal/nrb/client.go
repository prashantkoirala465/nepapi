// Package nrb fetches foreign exchange rates from Nepal Rastra Bank's
// public forex API (https://www.nrb.org.np/api/forex/v1).
package nrb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const DefaultBaseURL = "https://www.nrb.org.np/api/forex/v1"

// perPage is the page size requested from the NRB API. One page covers
// well over a month of daily publications.
const perPage = 100

type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// Rate is a single currency's buy/sell quote for one day.
type Rate struct {
	ISO3 string
	Name string
	Unit int
	Buy  float64
	Sell float64
}

// DayRates is the full set of published rates for one date.
type DayRates struct {
	Date        time.Time
	PublishedOn string
	Rates       []Rate
}

type apiResponse struct {
	Status struct {
		Code int `json:"code"`
	} `json:"status"`
	Data struct {
		Payload []struct {
			Date        string `json:"date"`
			PublishedOn string `json:"published_on"`
			Rates       []struct {
				Currency struct {
					ISO3 string `json:"iso3"`
					Name string `json:"name"`
					Unit int    `json:"unit"`
				} `json:"currency"`
				Buy  string `json:"buy"`
				Sell string `json:"sell"`
			} `json:"rates"`
		} `json:"payload"`
	} `json:"data"`
}

// RatesRange returns published rates for every day in [from, to],
// following pagination until the API returns an empty page. Days the
// bank did not publish (none observed in practice) are simply absent.
func (c *Client) RatesRange(ctx context.Context, from, to time.Time) ([]DayRates, error) {
	var all []DayRates
	for page := 1; ; page++ {
		days, err := c.fetchPage(ctx, from, to, page)
		if err != nil {
			return nil, err
		}
		if len(days) == 0 {
			break
		}
		all = append(all, days...)
		if len(days) < perPage {
			break
		}
	}
	return all, nil
}

func (c *Client) fetchPage(ctx context.Context, from, to time.Time, page int) ([]DayRates, error) {
	q := url.Values{
		"from":     {from.Format("2006-01-02")},
		"to":       {to.Format("2006-01-02")},
		"page":     {strconv.Itoa(page)},
		"per_page": {strconv.Itoa(perPage)},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/rates?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nrb: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nrb: unexpected status %d", resp.StatusCode)
	}

	var body apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("nrb: decoding response: %w", err)
	}
	if body.Status.Code != http.StatusOK {
		return nil, fmt.Errorf("nrb: api status code %d", body.Status.Code)
	}

	days := make([]DayRates, 0, len(body.Data.Payload))
	for _, p := range body.Data.Payload {
		date, err := time.Parse("2006-01-02", p.Date)
		if err != nil {
			return nil, fmt.Errorf("nrb: bad date %q: %w", p.Date, err)
		}
		day := DayRates{Date: date, PublishedOn: p.PublishedOn}
		for _, r := range p.Rates {
			buy, err := strconv.ParseFloat(r.Buy, 64)
			if err != nil {
				return nil, fmt.Errorf("nrb: bad buy rate %q for %s on %s: %w", r.Buy, r.Currency.ISO3, p.Date, err)
			}
			sell, err := strconv.ParseFloat(r.Sell, 64)
			if err != nil {
				return nil, fmt.Errorf("nrb: bad sell rate %q for %s on %s: %w", r.Sell, r.Currency.ISO3, p.Date, err)
			}
			day.Rates = append(day.Rates, Rate{
				ISO3: r.Currency.ISO3,
				Name: r.Currency.Name,
				Unit: r.Currency.Unit,
				Buy:  buy,
				Sell: sell,
			})
		}
		days = append(days, day)
	}
	return days, nil
}
