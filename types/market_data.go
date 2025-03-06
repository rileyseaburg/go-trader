package types

// MarketData represents the current market data for a symbol
type MarketData struct {
	Symbol    string  `json:"symbol"`
	Price     float64 `json:"price"`
	High24h   float64 `json:"high_24h"`
	Low24h    float64 `json:"low_24h"`
	Volume24h float64 `json:"volume_24h"`
	Change24h float64 `json:"change_24h"` // Percentage
}