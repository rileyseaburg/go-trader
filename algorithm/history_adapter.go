package algorithm

import (
	"errors"
	"fmt"
	"log"

	"github.com/rileyseaburg/go-trader/types"
)

// The functions in this file provide compatibility between the old API in algorithm.go
// and the new implementation in history.go

// GetHistoricalData provides backward compatibility with the handler API
func (a *TradingAlgorithm) GetHistoricalData(request types.HistoricalDataRequest) (*types.HistoricalData, error) {
	log.Printf("GetHistoricalData: converting request for %s", request.Symbol)

	// Validate request
	if request.Symbol == "" {
		return nil, errors.New("symbol is required")
	}
	if request.StartDate.After(request.EndDate) {
		return nil, errors.New("start date must be before end date")
	}

	// Create history request
	historyRequest := HistoryRequest{
		Symbol:    request.Symbol,
		StartDate: request.StartDate,
		EndDate:   request.EndDate,
	}

	// Call the new implementation
	barHistory, err := a.GetBarHistory(historyRequest)
	if err != nil {
		return nil, err
	}

	// Convert to old format for API compatibility
	return convertBarHistoryToHistoricalData(barHistory), nil
}

// convertBarHistoryToHistoricalData converts from BarHistory to the algorithm's
// expected format with Data points
func convertBarHistoryToHistoricalData(data BarHistory) *types.HistoricalData {
	// Create data points from bars
	points := make([]types.HistoricalDataPoint, len(data.Bars))
	for i, bar := range data.Bars {
		points[i] = types.HistoricalDataPoint{
			Symbol:    bar.Symbol,
			Timestamp: bar.Timestamp,
			Open:      bar.Open,
			High:      bar.High,
			Low:       bar.Low,
			Close:     bar.Close,
			Volume:    bar.Volume,
		}
	}

	// Return in the format expected by handlers
	return &types.HistoricalData{
		Symbol:    data.Symbol,
		TimeFrame: data.TimeFrame,
		StartDate: data.StartDate,
		EndDate:   data.EndDate,
		Data:      points,
	}
}

// AnalyzeHistoricalData provides backward compatibility with the handler API
func (a *TradingAlgorithm) AnalyzeHistoricalData(data *types.HistoricalData) *types.HistoricalDataAnalysis {
	log.Printf("AnalyzeHistoricalData: analyzing data for %s", data.Symbol)

	// Convert to BarHistory
	barData := make([]BarData, len(data.Data))
	for i, point := range data.Data {
		barData[i] = BarData{
			Symbol:    point.Symbol,
			Timestamp: point.Timestamp,
			Open:      point.Open,
			High:      point.High,
			Low:       point.Low,
			Close:     point.Close,
			Volume:    point.Volume,
		}
	}

	barHistory := BarHistory{
		Symbol:    data.Symbol,
		TimeFrame: data.TimeFrame,
		StartDate: data.StartDate,
		EndDate:   data.EndDate,
		Bars:      barData,
	}

	// Call the new analysis function
	analysis := a.AnalyzeBarHistory(barHistory)

	// Create the result in the old format
	return &types.HistoricalDataAnalysis{
		Symbol:    analysis.Symbol,
		TimeFrame: analysis.TimeFrame,
		StartDate: analysis.StartDate,
		EndDate:   analysis.EndDate,
		Indicators: map[string]interface{}{
			"trend_direction": analysis.TrendDirection,
			"trend_strength":  analysis.TrendStrength,
			"volatility":      analysis.Volatility,
		},
		Stats: map[string]float64{
			"avg_volume":        analysis.AverageVolume,
			"avg_price":         analysis.AveragePrice,
			"min_price":         analysis.MinPrice,
			"max_price":         analysis.MaxPrice,
			"price_range":       analysis.PriceRange,
			"percentage_change": analysis.PercentageChange,
		},
		Recommendations: []string{
			analysis.TrendDirection + " trend detected with " +
				fmt.Sprintf("%.2f%%", analysis.Volatility) + " volatility",
		},
	}
}
