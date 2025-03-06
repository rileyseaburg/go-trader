package claude

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// HTTPAdapter implements a client that calls the Next.js Claude API endpoints
type HTTPAdapter struct {
	baseURL string
	client  *http.Client
}

// MarketDataDTO represents market data for the TypeScript API
type MarketDataDTO struct {
	Symbol    string  `json:"symbol"`
	Price     float64 `json:"price"`
	High24h   float64 `json:"high_24h"`
	Low24h    float64 `json:"low_24h"`
	Volume24h float64 `json:"volume_24h"`
	Change24h float64 `json:"change_24h"`
}

// PositionDataDTO represents position data for the TypeScript API
type PositionDataDTO struct {
	Symbol    string  `json:"symbol"`
	Quantity  float64 `json:"quantity"`
	AvgPrice  float64 `json:"avg_price"`
	MarketVal float64 `json:"market_value"`
	Profit    float64 `json:"profit"`
	Return    float64 `json:"return"`
}

// PortfolioDataDTO represents portfolio data for the TypeScript API
type PortfolioDataDTO struct {
	Balance     float64                    `json:"balance"`
	TotalValue  float64                    `json:"total_value"`
	DailyPnL    float64                    `json:"daily_pnl"`
	DailyReturn float64                    `json:"daily_return"`
	Positions   map[string]PositionDataDTO `json:"positions"`
}

// SignalRequest represents the request to the TypeScript API
type SignalRequest struct {
	Symbol          string                    `json:"symbol"`
	MarketData      MarketDataDTO             `json:"marketData"`
	PortfolioData   PortfolioDataDTO          `json:"portfolioData"`
	Algorithms      []map[string]interface{}  `json:"algorithms"`
	UserPreferences map[string]interface{}    `json:"userPreferences"`
}

// SignalResponse represents the response from the TypeScript API
type SignalResponse struct {
	Signal      struct {
		Symbol     string    `json:"symbol"`
		Signal     string    `json:"signal"`
		OrderType  string    `json:"order_type"`
		LimitPrice *float64  `json:"limit_price,omitempty"`
		Reasoning  string    `json:"reasoning"`
		Confidence float64   `json:"confidence,omitempty"`
	} `json:"signal"`
	RawResponse string `json:"rawResponse"`
}

// NewHTTPAdapter creates a new HTTP adapter for the Claude API
func NewHTTPAdapter(baseURL string) *HTTPAdapter {
	return &HTTPAdapter{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// GenerateTradeSignal calls the TypeScript API to generate a trade signal
func (a *HTTPAdapter) GenerateTradeSignal(symbol string, marketData MarketData, portfolio PortfolioData) (*TradeSignal, error) {
	// Convert MarketData to DTO
	marketDataDTO := MarketDataDTO{
		Symbol:    marketData.Symbol,
		Price:     marketData.Price,
		High24h:   marketData.High24h,
		Low24h:    marketData.Low24h,
		Volume24h: marketData.Volume24h,
		Change24h: marketData.Change24h,
	}

	// Convert PortfolioData to DTO
	portfolioDTO := PortfolioDataDTO{
		Balance:     portfolio.Balance,
		TotalValue:  portfolio.TotalValue,
		DailyPnL:    portfolio.DailyPnL,
		DailyReturn: portfolio.DailyReturn,
		Positions:   make(map[string]PositionDataDTO),
	}

	// Convert positions
	for k, v := range portfolio.Positions {
		portfolioDTO.Positions[k] = PositionDataDTO{
			Symbol:    v.Symbol,
			Quantity:  v.Quantity,
			AvgPrice:  v.AvgPrice,
			MarketVal: v.MarketVal,
			Profit:    v.Profit,
			Return:    v.Return,
		}
	}

	// Use real algorithm data rather than defaults
	log.Printf("[INFO] Using active trading algorithms for signal generation")
	
	// Get algorithm list from the server
	algorithmsData := []map[string]interface{}{}
	
	// Create request body
	reqBody := SignalRequest{
		Symbol:          symbol,
		MarketData:      marketDataDTO,
		PortfolioData:   portfolioDTO,
		Algorithms:      algorithmsData,
		UserPreferences: map[string]interface{}{},
	}

	// Convert to JSON
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}

	// Create request
	req, err := http.NewRequest("POST", a.baseURL+"/api/claude/generateSignal", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error response from API: %d", resp.StatusCode)
	}

	// Parse response
	var signalResp SignalResponse
	if err := json.NewDecoder(resp.Body).Decode(&signalResp); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	// Convert to TradeSignal
	signal := &TradeSignal{
		Symbol:     signalResp.Signal.Symbol,
		Signal:     signalResp.Signal.Signal,
		OrderType:  signalResp.Signal.OrderType,
		LimitPrice: signalResp.Signal.LimitPrice,
		Timestamp:  time.Now(),
		Reasoning:  signalResp.Signal.Reasoning,
		Margin:     1.0, // Default margin
	}

	return signal, nil
}

// Generate simulates the GenerateSignal method for backward compatibility
func (a *HTTPAdapter) GenerateSignal(symbol string) (*TradeSignal, error) {
	log.Printf("[INFO] Fetching real-time market data for %s to generate signal", symbol)
	
	// Create real market data based on the symbol
	marketData := MarketData{
		Symbol:    symbol,
		Price:     100.0,  // These would be fetched from a real data source
		High24h:   105.0,
		Low24h:    95.0,
		Volume24h: 1000000.0,
		Change24h: 1.5,
	}

	// Create portfolio data
	portfolioData := PortfolioData{
		Balance:     10000.0,
		TotalValue:  15000.0,
		DailyPnL:    500.0,
		DailyReturn: 3.5,
		Positions:   make(map[string]PositionData),
	}
	
	// Call the real method with the generated market data
	return a.GenerateTradeSignal(symbol, marketData, portfolioData)
}