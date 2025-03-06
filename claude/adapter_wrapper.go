package claude

// WebSocketAdapterWrapper wraps the WebSocketAdapter to implement the algorithm package interfaces
// without creating circular dependencies
type WebSocketAdapterWrapper struct {
	adapter *WebSocketAdapter
}

// NewWebSocketAdapterWrapper creates a new wrapper around a WebSocketAdapter
func NewWebSocketAdapterWrapper(serverURL string) *WebSocketAdapterWrapper {
	return &WebSocketAdapterWrapper{
		adapter: NewWebSocketAdapter(serverURL),
	}
}

// GenerateTradeSignal implements the algorithm.ClaudeClientInterface method
// with the exact signature the algorithm package expects
func (w *WebSocketAdapterWrapper) GenerateTradeSignal(
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
	
	// Call the adapter's method with claude types
	claudeSignal, err := w.adapter.GenerateTradeSignal(symbol, claudeMarketData, claudePortfolio)
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

// GenerateSignal implements the simpler interface for direct symbol queries
func (w *WebSocketAdapterWrapper) GenerateSignal(symbol string) (*AlgorithmTradeSignal, error) {
	// Create dummy market data and portfolio data
	marketData := AlgorithmMarketData{
		Symbol:    symbol,
		Price:     100.0,
		High24h:   105.0,
		Low24h:    95.0,
		Volume24h: 10000.0,
		Change24h: 0.5,
	}
	
	portfolioData := AlgorithmPortfolioData{
		Balance:     10000.0,
		TotalValue:  15000.0,
		DailyPnL:    500.0,
		DailyReturn: 3.33,
		Positions:   make(map[string]AlgorithmPositionData),
	}
	
	return w.GenerateTradeSignal(symbol, marketData, portfolioData)
}

// Disconnect closes the underlying WebSocket connection
func (w *WebSocketAdapterWrapper) Disconnect() error {
	return w.adapter.Disconnect()
}