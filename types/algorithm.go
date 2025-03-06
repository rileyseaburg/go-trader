package types

import (
	"time"
)

// HistoricalDataPoint represents a single data point in historical data
// Used in the V2 API (adapter.go) for backward compatibility
type HistoricalDataPoint struct {
	Symbol    string    `json:"symbol"`
	Timestamp time.Time `json:"timestamp"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	Volume    int64     `json:"volume"`
}

// HistoricalDataRequest represents a request for historical market data
// Used by HTTP API handlers for backward compatibility
type HistoricalDataRequest struct {
	Symbol    string    `json:"symbol"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	TimeFrame string    `json:"time_frame"` // e.g., "1D", "1H", "15Min"
}

// HistoricalData represents historical market data with DataPoints
// Used by adapter.go and handler code for backward compatibility
type HistoricalData struct {
	Symbol    string                `json:"symbol"`
	TimeFrame string                `json:"time_frame"`
	StartDate time.Time             `json:"start_date"`
	EndDate   time.Time             `json:"end_date"`
	Data      []HistoricalDataPoint `json:"data,omitempty"`
}

// HistoricalDataAnalysis represents analysis of historical market data
// Used by adapter.go for API responses for backward compatibility
type HistoricalDataAnalysis struct {
	Symbol          string                 `json:"symbol"`
	TimeFrame       string                 `json:"time_frame"`
	StartDate       time.Time              `json:"start_date"`
	EndDate         time.Time              `json:"end_date"`
	Indicators      map[string]interface{} `json:"indicators"`      // Technical indicators like RSI, MACD
	Stats           map[string]float64     `json:"stats"`           // Statistical metrics
	Recommendations []string               `json:"recommendations"` // Trading recommendations
}

// RecommendedTicker represents a ticker recommendation from the algorithm
type RecommendedTicker struct {
Symbol     string  `json:"symbol"`
Confidence float64 `json:"confidence"`
Reasoning  string  `json:"reasoning"`
}

