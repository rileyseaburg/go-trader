package cartography

import (
	"context"
	"sort"
	"sync"
	"time"
)

// DataSignal is a single quantitative input — one FRED series reduced to a
// value, a threshold, and a triggered/clear status the algorithm can act on.
type DataSignal struct {
	Source      string    `json:"source"`      // e.g. "FRED:UNRATE"
	Name        string    `json:"name"`        // human-readable
	Value       float64   `json:"value"`       // current observation
	Threshold   float64   `json:"threshold"`   // trigger level
	Triggered   bool      `json:"triggered"`   // is the signal hot?
	AsOf        time.Time `json:"as_of"`       // observation date
	Description string    `json:"description"` // what the value means
}

// DataFeed is the composed real-data picture: each individual signal plus
// the multiplier they jointly imply.
type DataFeed struct {
	Signals     []DataSignal `json:"signals"`
	Triggers    []string     `json:"triggers"`     // names of fired signals
	Multiplier  float64      `json:"multiplier"`   // composed risk scalar
	Warnings    []string     `json:"warnings"`     // partial-fetch problems
	LastFetched time.Time    `json:"last_fetched"` // cache timestamp
}

// signalHaircut maps a triggered signal to the multiplicative haircut it
// applies to the position-size scalar. Intentionally conservative — the
// purpose is to *cap* risk when reality is hostile, not to chase.
//
// Sahm 0.40        — historically near-perfect recession trigger
// HY stress 0.50   — credit dislocation always precedes equity drawdowns
// Yield inv 0.85   — leading, not coincident; light haircut
// NFCI tight 0.85  — tightening conditions; light haircut
//
// Two small haircuts compound (0.85 × 0.85 ≈ 0.72), which matches how
// these signals tend to fire in clusters before something breaks.
func signalHaircut(name string) float64 {
	switch name {
	case "Sahm Recession Indicator":
		return 0.40
	case "High-Yield Credit Spread (OAS)":
		return 0.50
	case "10Y-2Y Treasury Spread":
		return 0.85
	case "Financial Conditions (NFCI)":
		return 0.85
	default:
		return 1.0
	}
}

// composeMultiplier reduces the active signals to a single risk scalar by
// multiplying their haircuts together. Returns (multiplier, names of
// triggered signals).
func composeMultiplier(signals []DataSignal) (float64, []string) {
	mult := 1.0
	var fired []string
	for _, s := range signals {
		if s.Triggered {
			mult *= signalHaircut(s.Name)
			fired = append(fired, s.Name)
		}
	}
	sort.Strings(fired)
	return mult, fired
}

// FeedCache keeps a cached snapshot of the FRED feed and refreshes it on
// a schedule rather than on every request — FRED has rate limits and the
// underlying data only updates daily.
type FeedCache struct {
	client  *FREDClient
	mu      sync.RWMutex
	current *DataFeed
	maxAge  time.Duration
}

// NewFeedCache builds a cache. maxAge of 0 defaults to 6 hours.
func NewFeedCache(client *FREDClient, maxAge time.Duration) *FeedCache {
	if maxAge <= 0 {
		maxAge = 6 * time.Hour
	}
	return &FeedCache{client: client, maxAge: maxAge}
}

// Get returns the cached feed (may be nil if never fetched). It does not
// trigger a refresh — the caller schedules that separately.
func (c *FeedCache) Get() *DataFeed {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.current
}

// Refresh pulls a fresh snapshot from FRED and stores it. Safe to call
// concurrently — the last writer wins. Returns the new feed plus any
// fetch error (the cache is not updated on hard error).
func (c *FeedCache) Refresh(ctx context.Context) (*DataFeed, error) {
	feed, err := c.client.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.current = feed
	c.mu.Unlock()
	return feed, nil
}

// AppliedMultiplier picks the more cautious of the formula multiplier and
// the data multiplier. The formula provides a slow-moving prior; the feed
// provides a coincident veto. min() means the feed can shrink risk when
// data disagrees with the model, but cannot enlarge it past what the
// formula already allows.
func AppliedMultiplier(formula float64, feed *DataFeed) float64 {
	if feed == nil {
		return formula
	}
	if feed.Multiplier < formula {
		return feed.Multiplier
	}
	return formula
}
