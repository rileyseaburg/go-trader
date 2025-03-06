package claude

import "time"

// AlgorithmMarketData represents market data with the same structure as algorithm.MarketData
type AlgorithmMarketData struct {
	Symbol    string  `json:"symbol"`
	Price     float64 `json:"price"`
	High24h   float64 `json:"high_24h"`
	Low24h    float64 `json:"low_24h"`
	Volume24h float64 `json:"volume_24h"`
	Change24h float64 `json:"change_24h"` // Percentage
}

// AlgorithmPositionData represents position data with the same structure as algorithm.PositionData
type AlgorithmPositionData struct {
	Symbol    string  `json:"symbol"`
	Quantity  float64 `json:"quantity"`
	AvgPrice  float64 `json:"avg_price"`
	MarketVal float64 `json:"market_value"`
	Profit    float64 `json:"profit"`
	Return    float64 `json:"return"` // Percentage
}

// AlgorithmPortfolioData represents portfolio data with the same structure as algorithm.PortfolioData
type AlgorithmPortfolioData struct {
	Balance     float64                            `json:"balance"`
	Positions   map[string]AlgorithmPositionData   `json:"positions"`
	TotalValue  float64                            `json:"total_value"`
	DailyPnL    float64                            `json:"daily_pnl"`
	DailyReturn float64                            `json:"daily_return"` // Percentage
}

// AlgorithmTradeSignal represents a trade signal with the same structure as algorithm.TradeSignal
type AlgorithmTradeSignal struct {
	Symbol     string    `json:"symbol"`
	Signal     string    `json:"signal"`      // buy, sell, hold, close
	OrderType  string    `json:"order_type"`  // market, limit
	LimitPrice *float64  `json:"limit_price"` // Only for limit orders
	Timestamp  time.Time `json:"timestamp"`
	Reasoning  string    `json:"reasoning"`
	Confidence *float64  `json:"confidence,omitempty"` // Confidence score from 0-1, nil if not provided
}

// GenerateTradeSignalForAlgorithm adapts the WebSocketAdapter for the algorithm package
func (a *WebSocketAdapter) GenerateTradeSignalForAlgorithm(
	symbol string, 
	marketData AlgorithmMarketData, 
	portfolio AlgorithmPortfolioData,
) (*AlgorithmTradeSignal, error) {
	// Convert from algorithm types to claude types
	claudeMarketData := MarketData{
		Symbol:    marketData.Symbol,
		Price:     marketData.Price,
		High24h:   marketData.High24h,
		Low24h:    marketData.Low24h,
		Volume24h: marketData.Volume24h,
		Change24h: marketData.Change24h,
	}
	
	claudePositions := make(map[string]PositionData)
	for symbol, pos := range portfolio.Positions {
		claudePositions[symbol] = PositionData{
			Symbol:    pos.Symbol,
			Quantity:  pos.Quantity,
			AvgPrice:  pos.AvgPrice,
			MarketVal: pos.MarketVal,
			Profit:    pos.Profit,
			Return:    pos.Return,
		}
	}
	
	claudePortfolio := PortfolioData{
		Balance:     portfolio.Balance,
		TotalValue:  portfolio.TotalValue,
		DailyPnL:    portfolio.DailyPnL,
		DailyReturn: portfolio.DailyReturn,
		Positions:   claudePositions,
	}
	
	// Call the original method with claude types
	claudeSignal, err := a.GenerateTradeSignal(symbol, claudeMarketData, claudePortfolio)
	if err != nil {
		return nil, err
	}
	
	// Convert claude.TradeSignal to AlgorithmTradeSignal
	var confidence *float64
	if claudeSignal.Margin != 0 {
		confidenceVal := claudeSignal.Margin
		confidence = &confidenceVal
	}
	
	return &AlgorithmTradeSignal{
		Symbol:     claudeSignal.Symbol,
		Signal:     claudeSignal.Signal,
		OrderType:  claudeSignal.OrderType,
		LimitPrice: claudeSignal.LimitPrice,
		Timestamp:  claudeSignal.Timestamp,
		Reasoning:  claudeSignal.Reasoning,
		Confidence: confidence,
	}, nil
}
