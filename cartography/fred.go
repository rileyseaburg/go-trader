package cartography

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// FRED (Federal Reserve Economic Data) integration — pulls four canonical
// series and reduces them to a small set of binary recession/stress signals
// that override the formula's regime when reality disagrees with the model.
//
//   UNRATE        monthly unemployment rate           → Sahm rule
//   T10Y2Y        10Y minus 2Y Treasury spread, daily → yield-curve inversion
//   NFCI          Chicago Fed financial conditions    → credit/liquidity stress
//   BAMLH0A0HYM2  ICE BofA high-yield OAS, daily      → corporate stress

const fredAPIBase = "https://api.stlouisfed.org/fred/series/observations"

type fredObservation struct {
	Date  string `json:"date"`
	Value string `json:"value"` // FRED returns "." for missing
}

type fredResponse struct {
	Observations []fredObservation `json:"observations"`
}

// FREDClient hits the FRED API. Set APIKey from the FRED_API_KEY env var.
type FREDClient struct {
	APIKey string
	HTTP   *http.Client
}

// NewFREDClient builds a client with a sensible default HTTP timeout.
func NewFREDClient(apiKey string) *FREDClient {
	return &FREDClient{
		APIKey: apiKey,
		HTTP:   &http.Client{Timeout: 15 * time.Second},
	}
}

// fetch pulls the most recent `limit` observations of seriesID, sorted
// newest-first.
func (c *FREDClient) fetch(ctx context.Context, seriesID string, limit int) ([]fredObservation, error) {
	if c == nil || c.APIKey == "" {
		return nil, fmt.Errorf("FRED API key not configured")
	}
	q := url.Values{}
	q.Set("series_id", seriesID)
	q.Set("api_key", c.APIKey)
	q.Set("file_type", "json")
	q.Set("sort_order", "desc")
	q.Set("limit", strconv.Itoa(limit))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fredAPIBase+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("FRED %s: %w", seriesID, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("FRED %s: HTTP %d", seriesID, resp.StatusCode)
	}
	var fr fredResponse
	if err := json.NewDecoder(resp.Body).Decode(&fr); err != nil {
		return nil, fmt.Errorf("FRED %s decode: %w", seriesID, err)
	}
	return fr.Observations, nil
}

// parseObs converts FRED observations to (values, dates), dropping "."
// missing markers. Output preserves desc-by-date order.
func parseObs(obs []fredObservation) ([]float64, []time.Time) {
	vals := make([]float64, 0, len(obs))
	dates := make([]time.Time, 0, len(obs))
	for _, o := range obs {
		if o.Value == "." || o.Value == "" {
			continue
		}
		v, err := strconv.ParseFloat(o.Value, 64)
		if err != nil {
			continue
		}
		d, err := time.Parse("2006-01-02", o.Date)
		if err != nil {
			continue
		}
		vals = append(vals, v)
		dates = append(dates, d)
	}
	return vals, dates
}

// computeSahm implements the Sahm Recession Indicator:
//   sahm = (current 3-month MA of UNRATE) - (lowest 3-month MA over prior 12 months)
// triggers at >= 0.50. Authoritative through the post-WWII record.
func computeSahm(obs []fredObservation) *DataSignal {
	vals, dates := parseObs(obs)
	if len(vals) < 14 {
		return nil
	}
	current := (vals[0] + vals[1] + vals[2]) / 3.0
	minMMA := math.Inf(1)
	// Window i=1..12: 3MMA at offset i uses vals[i], vals[i+1], vals[i+2].
	for i := 1; i <= 12 && i+2 < len(vals); i++ {
		mma := (vals[i] + vals[i+1] + vals[i+2]) / 3.0
		if mma < minMMA {
			minMMA = mma
		}
	}
	if math.IsInf(minMMA, 1) {
		return nil
	}
	delta := current - minMMA
	return &DataSignal{
		Source:    "FRED:UNRATE",
		Name:      "Sahm Recession Indicator",
		Value:     round2(delta),
		Threshold: 0.50,
		Triggered: delta >= 0.50,
		AsOf:      dates[0],
		Description: fmt.Sprintf("3M unemployment %.2f%% vs trailing-12mo min %.2f%% (Δ %+.2f, triggers ≥ +0.50)",
			current, minMMA, delta),
	}
}

// computeYieldCurve reads T10Y2Y. Negative = inverted (recession warning,
// historically 6–18 months lead).
func computeYieldCurve(obs []fredObservation) *DataSignal {
	vals, dates := parseObs(obs)
	if len(vals) == 0 {
		return nil
	}
	v := vals[0]
	state := "normal slope"
	if v < 0 {
		state = "INVERTED — recession-leading signal"
	} else if v < 0.25 {
		state = "near-flat"
	}
	return &DataSignal{
		Source:    "FRED:T10Y2Y",
		Name:      "10Y-2Y Treasury Spread",
		Value:     round2(v),
		Threshold: 0.0,
		Triggered: v < 0,
		AsOf:      dates[0],
		Description: fmt.Sprintf("10Y minus 2Y = %+.2f%% — %s", v, state),
	}
}

// computeNFCI reads the Chicago Fed National Financial Conditions Index.
// Mean = 0; positive = tighter than average. >0 already flags tightening.
func computeNFCI(obs []fredObservation) *DataSignal {
	vals, dates := parseObs(obs)
	if len(vals) == 0 {
		return nil
	}
	v := vals[0]
	state := "looser than average"
	if v > 0 {
		state = "tighter than average"
	}
	if v > 0.5 {
		state = "STRESSED — credit tightening"
	}
	return &DataSignal{
		Source:    "FRED:NFCI",
		Name:      "Financial Conditions (NFCI)",
		Value:     round2(v),
		Threshold: 0.0,
		Triggered: v > 0,
		AsOf:      dates[0],
		Description: fmt.Sprintf("NFCI = %+.2fσ — %s", v, state),
	}
}

// computeHYSpread reads the ICE BofA US High Yield OAS. Long-run median
// ~4–5%; sustained >7% historically marks credit-cycle stress.
func computeHYSpread(obs []fredObservation) *DataSignal {
	vals, dates := parseObs(obs)
	if len(vals) == 0 {
		return nil
	}
	v := vals[0]
	state := "calm"
	if v >= 5.5 {
		state = "elevated"
	}
	if v >= 7.0 {
		state = "STRESSED — credit-cycle warning"
	}
	return &DataSignal{
		Source:    "FRED:BAMLH0A0HYM2",
		Name:      "High-Yield Credit Spread (OAS)",
		Value:     round2(v),
		Threshold: 7.0,
		Triggered: v >= 7.0,
		AsOf:      dates[0],
		Description: fmt.Sprintf("HY OAS = %.2f%% — %s", v, state),
	}
}

// Snapshot fetches all four series and returns a composed feed. Failures of
// individual series are non-fatal — the snapshot returns the signals it
// could compute and surfaces the rest as a warning. Only a hard FRED error
// (no key, network down) returns an error.
func (c *FREDClient) Snapshot(ctx context.Context) (*DataFeed, error) {
	if c == nil || c.APIKey == "" {
		return nil, fmt.Errorf("FRED API key not configured")
	}

	feed := &DataFeed{LastFetched: time.Now()}

	// UNRATE: need 24 monthly obs for the trailing 12mo Sahm window.
	if obs, err := c.fetch(ctx, "UNRATE", 24); err == nil {
		if sig := computeSahm(obs); sig != nil {
			feed.Signals = append(feed.Signals, *sig)
		}
	} else {
		feed.Warnings = append(feed.Warnings, err.Error())
	}

	if obs, err := c.fetch(ctx, "T10Y2Y", 5); err == nil {
		if sig := computeYieldCurve(obs); sig != nil {
			feed.Signals = append(feed.Signals, *sig)
		}
	} else {
		feed.Warnings = append(feed.Warnings, err.Error())
	}

	if obs, err := c.fetch(ctx, "NFCI", 5); err == nil {
		if sig := computeNFCI(obs); sig != nil {
			feed.Signals = append(feed.Signals, *sig)
		}
	} else {
		feed.Warnings = append(feed.Warnings, err.Error())
	}

	if obs, err := c.fetch(ctx, "BAMLH0A0HYM2", 5); err == nil {
		if sig := computeHYSpread(obs); sig != nil {
			feed.Signals = append(feed.Signals, *sig)
		}
	} else {
		feed.Warnings = append(feed.Warnings, err.Error())
	}

	feed.Multiplier, feed.Triggers = composeMultiplier(feed.Signals)
	return feed, nil
}
