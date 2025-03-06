package algorithm

import (
	"fmt"
	"log"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata"
)

// BarData represents a single historical price bar
type BarData struct {
	Symbol    string    `json:"symbol"`
	Timestamp time.Time `json:"timestamp"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	Volume    int64     `json:"volume"`
	VWAP      float64   `json:"vwap,omitempty"`
}

// BarHistory represents a collection of historical bars
type BarHistory struct {
	Symbol    string    `json:"symbol"`
	TimeFrame string    `json:"timeframe"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	Bars      []BarData `json:"bars"`
}

// HistoryRequest represents a request for historical data
type HistoryRequest struct {
	Symbol    string    `json:"symbol"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	TimeFrame string    `json:"timeframe"`
}

// BarAnalysis represents an analysis of historical data
type BarAnalysis struct {
	Symbol           string    `json:"symbol"`
	TimeFrame        string    `json:"timeframe"`
	StartDate        time.Time `json:"start_date"`
	EndDate          time.Time `json:"end_date"`
	BarCount         int       `json:"bar_count"`
	AverageVolume    float64   `json:"avg_volume"`
	AveragePrice     float64   `json:"avg_price"`
	MinPrice         float64   `json:"min_price"`
	MaxPrice         float64   `json:"max_price"`
	PriceRange       float64   `json:"price_range"`
	Volatility       float64   `json:"volatility"`
	TrendDirection   string    `json:"trend_direction"`
	TrendStrength    float64   `json:"trend_strength"`
	PercentageChange float64   `json:"percentage_change"`
	RecentVolume     float64   `json:"recent_volume"`
	RecentVolatility float64   `json:"recent_volatility"`
}

// GetBarHistory fetches historical data for a symbol using bars
func (a *TradingAlgorithm) GetBarHistory(request HistoryRequest) (BarHistory, error) {
	// Validate the timeframe
	timeframe, err := parseTimeFrame(request.TimeFrame)
	if err != nil {
		return BarHistory{}, err
	}

	// Fetch the historical bars from Alpaca
	bars, err := a.mdClient.GetBars(
		request.Symbol,
		marketdata.GetBarsRequest{
			TimeFrame: timeframe,
			Start:     request.StartDate,
			End:       request.EndDate,
		},
	)
	if err != nil {
		return BarHistory{}, fmt.Errorf("failed to fetch historical data: %w", err)
	}

	// Convert Alpaca bars to our BarData format
	historicalBars := make([]BarData, len(bars))
	for i, bar := range bars {
		historicalBars[i] = BarData{
			Symbol:    request.Symbol,
			Timestamp: bar.Timestamp,
			Open:      bar.Open,
			High:      bar.High,
			Low:       bar.Low,
			Close:     bar.Close,
			Volume:    int64(bar.Volume),
			VWAP:      bar.VWAP,
		}
	}

	log.Printf("Fetched %d historical bars for %s from %s to %s with timeframe %s",
		len(historicalBars), request.Symbol, request.StartDate.Format("2006-01-02"),
		request.EndDate.Format("2006-01-02"), request.TimeFrame)

	// Create and return the BarHistory
	return BarHistory{
		Symbol:    request.Symbol,
		TimeFrame: request.TimeFrame,
		StartDate: request.StartDate,
		EndDate:   request.EndDate,
		Bars:      historicalBars,
	}, nil
}

// AnalyzeBarHistory performs analysis on historical data
func (a *TradingAlgorithm) AnalyzeBarHistory(data BarHistory) BarAnalysis {
	// Initialize analysis
	analysis := BarAnalysis{
		Symbol:    data.Symbol,
		TimeFrame: data.TimeFrame,
		StartDate: data.StartDate,
		EndDate:   data.EndDate,
		BarCount:  len(data.Bars),
	}

	if len(data.Bars) == 0 {
		log.Printf("No bars to analyze for %s", data.Symbol)
		return analysis
	}

	// Calculate basic metrics
	var sumVolume, sumClose, sumSquaredReturns float64
	minPrice := data.Bars[0].Low
	maxPrice := data.Bars[0].High

	// Calculate daily returns for volatility
	returns := make([]float64, len(data.Bars)-1)

	for i, bar := range data.Bars {
		// Update min and max prices
		if bar.Low < minPrice {
			minPrice = bar.Low
		}
		if bar.High > maxPrice {
			maxPrice = bar.High
		}

		// Sum for averages
		sumVolume += float64(bar.Volume)
		sumClose += bar.Close

		// Calculate returns (for volatility)
		if i > 0 {
			prevClose := data.Bars[i-1].Close
			if prevClose > 0 {
				dailyReturn := (bar.Close - prevClose) / prevClose
				returns[i-1] = dailyReturn
				sumSquaredReturns += dailyReturn * dailyReturn
			}
		}
	}

	// Calculate averages
	analysis.AverageVolume = sumVolume / float64(len(data.Bars))
	analysis.AveragePrice = sumClose / float64(len(data.Bars))
	analysis.MinPrice = minPrice
	analysis.MaxPrice = maxPrice
	analysis.PriceRange = maxPrice - minPrice

	// Calculate volatility (standard deviation of returns)
	if len(returns) > 0 {
		analysis.Volatility = (sumSquaredReturns / float64(len(returns))) * 100 // as percentage
	}

	// Determine trend
	firstPrice := data.Bars[0].Close
	lastPrice := data.Bars[len(data.Bars)-1].Close
	priceChange := lastPrice - firstPrice
	analysis.PercentageChange = (priceChange / firstPrice) * 100

	if priceChange > 0 {
		analysis.TrendDirection = "up"
		analysis.TrendStrength = analysis.PercentageChange
	} else if priceChange < 0 {
		analysis.TrendDirection = "down"
		analysis.TrendStrength = -analysis.PercentageChange
	} else {
		analysis.TrendDirection = "neutral"
		analysis.TrendStrength = 0
	}

	// Calculate recent metrics (last 10 bars or entire dataset if smaller)
	recentBarCount := min(10, len(data.Bars))
	recentBars := data.Bars[len(data.Bars)-recentBarCount:]

	var recentVolumeSum float64
	var recentReturnsSum float64
	for i, bar := range recentBars {
		recentVolumeSum += float64(bar.Volume)

		if i > 0 {
			prevClose := recentBars[i-1].Close
			if prevClose > 0 {
				dailyReturn := (bar.Close - prevClose) / prevClose
				recentReturnsSum += dailyReturn * dailyReturn
			}
		}
	}

	analysis.RecentVolume = recentVolumeSum / float64(recentBarCount)
	if recentBarCount > 1 {
		analysis.RecentVolatility = (recentReturnsSum / float64(recentBarCount-1)) * 100
	}

	log.Printf("Analyzed historical data for %s: trend=%s, strength=%.2f%%, volatility=%.2f%%",
		data.Symbol, analysis.TrendDirection, analysis.TrendStrength, analysis.Volatility)

	return analysis
}

// parseTimeFrame converts a string timeframe (e.g., "1D") to Alpaca's TimeFrame type
func parseTimeFrame(timeframe string) (marketdata.TimeFrame, error) {
	switch timeframe {
	case "1Min":
		return marketdata.OneMin, nil
	case "5Min":
		// Use static construction for 5 minute bars
		// For newer Alpaca SDK versions
		return marketdata.NewTimeFrame(5, marketdata.Min), nil
	case "15Min":
		// Use static construction for 15 minute bars
		// For newer Alpaca SDK versions
		return marketdata.NewTimeFrame(15, marketdata.Min), nil
	case "1H":
		// This uses the built-in constant for 1 hour
		return marketdata.OneHour, nil
	case "1D":
		return marketdata.OneDay, nil
	default:
		// Default to 1 day if not specified correctly
		log.Printf("Unrecognized timeframe: %s, defaulting to 1D", timeframe)
		return marketdata.OneDay, nil
	}
}

// Helper function to get minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
