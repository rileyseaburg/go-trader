package algorithm

import (
	"errors"
	"log"
	"time"
"github.com/rileyseaburg/go-trader/types"
"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata"



)

// This file provides adapter functions to handle incompatible data structures
// and act as wrappers for the HTTP API handlers

// GetHistoricalDataV2 provides a compatible wrapper for the algorithm handler
func (a *TradingAlgorithm) GetHistoricalDataV2(request types.HistoricalDataRequest) (*types.HistoricalData, error) {
	log.Printf("GetHistoricalDataV2: fetching data for %s", request.Symbol)

	// Validate request
	if request.Symbol == "" {
		return nil, errors.New("symbol is required")
	}
	if request.StartDate.After(request.EndDate) {
		return nil, errors.New("start date must be before end date")
	}

	// Use ParseTimeFrame from timeframe.go instead of the one in history.go
	timeframe, err := ParseTimeFrame(request.TimeFrame)
	if err != nil {
		return nil, err
	}

	// Get bars from Alpaca
	bars, err := a.mdClient.GetBars(request.Symbol,
		marketdata.GetBarsRequest{
			TimeFrame: timeframe,
			Start:     request.StartDate,
			End:       request.EndDate,
		})
	if err != nil {
		return nil, err
	}

	// Convert bars to historical data points
	data := make([]types.HistoricalDataPoint, len(bars))
	for i, bar := range bars {
		data[i] = types.HistoricalDataPoint{
			Symbol:    request.Symbol, // Alpaca bars don't have symbol
			Timestamp: bar.Timestamp,
			Open:      bar.Open,
			High:      bar.High,
			Low:       bar.Low,
			Close:     bar.Close,
			Volume:    int64(bar.Volume),
		}
	}

	// Create historical data
	historicalData := &types.HistoricalData{
		Symbol:    request.Symbol,
		TimeFrame: request.TimeFrame,
		StartDate: request.StartDate,
		EndDate:   request.EndDate,
		Data:      data,
	}

	log.Printf("Fetched %d data points for %s from %s to %s",
		len(data), request.Symbol, request.StartDate.Format("2006-01-02"),
		request.EndDate.Format("2006-01-02"))

	return historicalData, nil
}

// AnalyzeHistoricalDataV2 provides a compatible wrapper for the algorithm handler
func (a *TradingAlgorithm) AnalyzeHistoricalDataV2(data *types.HistoricalData) *types.HistoricalDataAnalysis {
	if data == nil {
		return &types.HistoricalDataAnalysis{
			Symbol:          "",
			TimeFrame:       "",
			StartDate:       time.Time{},
			EndDate:         time.Time{},
			Indicators:      map[string]interface{}{},
			Stats:           map[string]float64{},
			Recommendations: []string{"No data provided"},
		}
	}

	log.Printf("AnalyzeHistoricalDataV2: analyzing data for %s", data.Symbol)

	if len(data.Data) == 0 {
		return &types.HistoricalDataAnalysis{
			Symbol:          data.Symbol,
			TimeFrame:       data.TimeFrame,
			StartDate:       data.StartDate,
			EndDate:         data.EndDate,
			Indicators:      map[string]interface{}{},
			Stats:           map[string]float64{},
			Recommendations: []string{"Insufficient data for analysis"},
		}
	}

	// Calculate simple metrics directly
	var highestPrice, lowestPrice, sumClose, sumVolume float64
	if len(data.Data) > 0 {
		highestPrice = data.Data[0].High
		lowestPrice = data.Data[0].Low
	}

	// Calculate metrics
	for _, point := range data.Data {
		if point.High > highestPrice {
			highestPrice = point.High
		}
		if point.Low < lowestPrice {
			lowestPrice = point.Low
		}
		sumClose += point.Close
		sumVolume += float64(point.Volume)
	}

	// Calculate average price and volume
	avgPrice := sumClose / float64(len(data.Data))
	avgVolume := sumVolume / float64(len(data.Data))

	// Calculate trend
	priceChange := 0.0
	pctChange := 0.0
	trendDirection := "neutral"
	if len(data.Data) > 1 {
		firstClose := data.Data[0].Close
		lastClose := data.Data[len(data.Data)-1].Close
		priceChange = lastClose - firstClose
		if firstClose > 0 {
			pctChange = priceChange / firstClose * 100
		}

		if pctChange > 1.0 {
			trendDirection = "up"
		} else if pctChange < -1.0 {
			trendDirection = "down"
		}
	}

	// Generate recommendations
	var recommendations []string
	if trendDirection == "up" {
		recommendations = append(recommendations, "Consider long position based on uptrend")
	} else if trendDirection == "down" {
		recommendations = append(recommendations, "Consider short position based on downtrend")
	} else {
		recommendations = append(recommendations, "Market appears neutral, monitor for breakout")
	}

	// Create analysis object
	analysis := &types.HistoricalDataAnalysis{
		Symbol:    data.Symbol,
		TimeFrame: data.TimeFrame,
		StartDate: data.StartDate,
		EndDate:   data.EndDate,
		Indicators: map[string]interface{}{
			"trend_direction": trendDirection,
			"price_change":    priceChange,
		},
		Stats: map[string]float64{
			"high":       highestPrice,
			"low":        lowestPrice,
			"avg_price":  avgPrice,
			"avg_volume": avgVolume,
			"pct_change": pctChange,
		},
		Recommendations: recommendations,
	}

	return analysis
}

// RecommendTickersV2 converts from RecommendedTicker to []string for the V2 API
func (a *TradingAlgorithm) RecommendTickersV2(sector string, maxResults int) ([]string, error) {
	log.Printf("RecommendTickersV2: getting recommendations for %s sector", sector)

	// Convert old RecommendedTicker to types.RecommendedTicker
	recommendations, err := a.RecommendTickersList(sector, maxResults)
	if err != nil {
		  return nil, err
	}

	// Convert to string array for the API
	result := make([]string, len(recommendations))
	for i, rec := range recommendations {
		result[i] = rec.Symbol
	}

	return result, nil
}

// RecommendTickers provides backward compatibility with the main.go API
func (a *TradingAlgorithm) RecommendTickers(sector string, maxResults int) ([]*types.RecommendedTicker, error) {
	  log.Printf("Getting ticker recommendations for sector %s via recommendations module", sector)
	  oldRecs, err := a.RecommendTickersList(sector, maxResults)
  if err != nil {
    return nil, err
  }
  
  // Convert to new type
  recs := make([]*types.RecommendedTicker, len(oldRecs))
  for i, rec := range oldRecs {
    recs[i] = &types.RecommendedTicker{Symbol: rec.Symbol, Confidence: rec.Confidence, Reasoning: rec.Reasoning}
  }
  
  return recs, nil
}
