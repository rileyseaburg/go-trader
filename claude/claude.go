package claude

import (
	"net/http"
	"log"
	"sync"
	"time"
)

// Client is a client for the Claude API
type Client struct {
	apiKey      string
	apiURL      string
	httpClient  *http.Client
	signals     map[string]*TradeSignal
	signalMutex sync.RWMutex
	callback    StreamCallback
	adapter     SignalAdapter
}

// MarketData represents market data for a symbol
type MarketData struct {
	Symbol    string  `json:"symbol"`
	Price     float64 `json:"price"`
	High24h   float64 `json:"high_24h"`
	Low24h    float64 `json:"low_24h"`
	Volume24h float64 `json:"volume_24h"`
	Change24h float64 `json:"change_24h"` // Percentage
}

// TradeSignal represents a trading signal with reasoning
type TradeSignal struct {
	Symbol     string    `json:"symbol"`
	Signal     string    `json:"signal"`      // buy, sell, hold
	OrderType  string    `json:"order_type"`  // market, limit
	LimitPrice *float64  `json:"limit_price"` // nil for market orders
	Timestamp  time.Time `json:"timestamp"`
	Reasoning  string    `json:"reasoning"`   // explanation for the decision
	Margin     float64   `json:"margin"`      // leverage margin
}

// PositionData represents a trading position
type PositionData struct {
	Symbol    string  `json:"symbol"`
	Quantity  float64 `json:"quantity"`
	AvgPrice  float64 `json:"avg_price"`
	MarketVal float64 `json:"market_val"`
	Profit    float64 `json:"profit"`
	Return    float64 `json:"return"`
}

// PortfolioData represents a trading portfolio
type PortfolioData struct {
	Balance     float64                 `json:"balance"`
	TotalValue  float64                 `json:"total_value"`
	DailyPnL    float64                 `json:"daily_pnl"`
	DailyReturn float64                 `json:"daily_return"`
	Positions   map[string]PositionData `json:"positions"`
}

// StreamOptions provides configuration for streaming responses
type StreamOptions struct {
	Enabled    bool `json:"enabled"`
	ChunkSize  int  `json:"chunk_size"`
	DelayMs    int  `json:"delay_ms"`
	UseRealAPI bool `json:"use_real_api"`
}

// StreamCallback is a function called for each chunk of a streaming response
type StreamCallback func(symbol string, chunk string)

// SignalAdapter is an interface for generating trading signals
type SignalAdapter interface {
	GenerateTradeSignal(symbol string, marketData MarketData, portfolioData PortfolioData) (*TradeSignal, error)
	GenerateSignal(symbol string) (*TradeSignal, error)
}

// NewClient creates a new Claude client
func NewClient(apiKey, apiURL string, adapter SignalAdapter) *Client {
	if adapter == nil {
		adapter = NewWebSocketAdapter("http://localhost:3000")
	}
	
	return &Client{
		apiKey:     apiKey,
		apiURL:     apiURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		signals:    make(map[string]*TradeSignal),
		adapter:    adapter,
	}
}

// RegisterStreamCallback registers a callback for streaming responses
func (c *Client) RegisterStreamCallback(callback StreamCallback) {
	c.callback = callback
}

// GenerateSignal generates a trading signal for a symbol
func (c *Client) GenerateSignal(symbol string) (*TradeSignal, error) {
	log.Printf("[DEBUG] GenerateSignal called for symbol: %s", symbol)

	// Check if we have a cached signal
	c.signalMutex.RLock()
	cachedSignal, exists := c.signals[symbol]
	c.signalMutex.RUnlock()

	// If we have a recent cached signal (less than 10 minutes old), return it
	if exists && time.Since(cachedSignal.Timestamp) < 10*time.Minute {
		log.Printf("[DEBUG] Using cached signal for %s (age: %v)", symbol, time.Since(cachedSignal.Timestamp))
		return cachedSignal, nil
	}

	log.Printf("[INFO] No recent cached signal found, generating new signal from algorithm for %s", symbol)
	
	// Use the adapter to generate a real signal
	signal, err := c.adapter.GenerateSignal(symbol)
	if err != nil {
		log.Printf("[ERROR] Error generating signal via adapter: %v", err)
		log.Printf("[ERROR] Failed to generate signal for %s: %v", symbol, err)
		return nil, err
	}

	// Cache the signal
	c.signalMutex.Lock()
	c.signals[symbol] = signal
	c.signalMutex.Unlock()
	log.Printf("[DEBUG] Generated and cached algorithmic signal for %s: %s", symbol, signal.Signal)

	return signal, nil
}

// GenerateTradeSignal generates a trading signal with full market data
func (c *Client) GenerateTradeSignal(symbol string, marketData MarketData, portfolioData PortfolioData, opts *StreamOptions) (*TradeSignal, error) {
	// Enhanced version of GenerateSignal that takes additional context
	log.Printf("[DEBUG] GenerateTradeSignal called for %s with market data: price=%.2f, change=%.2f%%", 
		symbol, marketData.Price, marketData.Change24h)
	
	log.Printf("[INFO] Generating real trading signal using algorithm analysis for %s", symbol)
	// If real API is specified, use the adapter directly
	if opts != nil && opts.UseRealAPI {
		log.Printf("[DEBUG] Using real API for %s", symbol)
		signal, err := c.adapter.GenerateTradeSignal(symbol, marketData, portfolioData)
		if err != nil {
			log.Printf("[ERROR] Error calling real API: %v", err)
			return nil, err
		}
		
		// Cache the signal
		c.signalMutex.Lock()
		c.signals[symbol] = signal
		c.signalMutex.Unlock()
		return signal, nil
	}
	
	// Otherwise use the basic implementation which will still use the adapter
	return c.GenerateSignal(symbol)
}

// GetCachedSignal returns a cached signal if available
func (c *Client) GetCachedSignal(symbol string) *TradeSignal {
	c.signalMutex.RLock()
	defer c.signalMutex.RUnlock()
	return c.signals[symbol]
}