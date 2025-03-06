package claude

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocketAdapter implements a Claude client that communicates with the Next.js frontend via WebSockets
// with fallback to HTTP when WebSockets aren't available
type WebSocketAdapter struct {
	serverURL   string
	conn        *websocket.Conn
	mutex       sync.Mutex
	isConnected bool
	pendingReqs map[string]chan *TradeSignal
	httpClient  *http.Client
	reqIDMutex  sync.Mutex
	reqIDCount  int
}

// WSSignalRequest represents a WebSocket request for a signal
type WSSignalRequest struct {
	ID            string        `json:"id"`
	Action        string        `json:"action"`
	Symbol        string        `json:"symbol"`
	MarketData    MarketData    `json:"marketData"`
	PortfolioData PortfolioData `json:"portfolioData"`
}

// WSSignalResponse represents a WebSocket response with a signal
type WSSignalResponse struct {
	ID      string      `json:"id"`
	Status  string      `json:"status"`
	Signal  TradeSignal `json:"signal,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// NewWebSocketAdapter creates a new WebSocket adapter
func NewWebSocketAdapter(serverURL string) *WebSocketAdapter {
	return &WebSocketAdapter{
		serverURL:   serverURL,
		pendingReqs: make(map[string]chan *TradeSignal),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConnsPerHost: 20,
			},
		},
	}
}

// Connect establishes a WebSocket connection to the Next.js server
// or sets up for HTTP fallback if WebSockets aren't available
func (a *WebSocketAdapter) Connect() error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if a.isConnected {
		return nil
	}

	// For Next.js App Router, we'll use HTTP instead of WebSockets
	// as WebSocket support in App Router is limited
	a.isConnected = true
	log.Printf("Using HTTP fallback mode for WebSocket communication with %s", a.serverURL)
	return nil
}

// GenerateTradeSignal sends a signal request via HTTP (fallback for WebSocket)
func (a *WebSocketAdapter) GenerateTradeSignal(symbol string, marketData MarketData, portfolio PortfolioData) (*TradeSignal, error) {
	// Ensure connection is established
	if err := a.Connect(); err != nil {
		log.Printf("Warning: Failed to connect to server: %v", err)
		// Return a default hold signal as fallback
		return &TradeSignal{
			Symbol:    symbol,
			Signal:    "hold",
			OrderType: "market",
			Timestamp: time.Now(),
			Reasoning: "Connection to Next.js failed. Using default hold signal.",
			Margin:    1.0,
		}, nil
	}

	// Generate unique request ID
	a.reqIDMutex.Lock()
	a.reqIDCount++
	reqID := fmt.Sprintf("req_%d", a.reqIDCount)
	a.reqIDMutex.Unlock()

	// Create request
	request := WSSignalRequest{
		ID:            reqID,
		Action:        "generateSignal",
		Symbol:        symbol,
		MarketData:    marketData,
		PortfolioData: portfolio,
	}

	// Marshal request
	reqData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}

	// Parse the URL for the API endpoint
	u, err := url.Parse(a.serverURL)
	if err != nil {
		return nil, fmt.Errorf("invalid server URL: %w", err)
	}

	// Ensure the URL has a scheme and build the API URL
	var apiURL string
	if !strings.HasPrefix(a.serverURL, "http") {
		apiURL = fmt.Sprintf("http://%s/api/ws/claude", u.Host)
	} else {
		apiURL = fmt.Sprintf("%s/api/ws/claude", a.serverURL)
	}

	// Execute the HTTP POST request
	resp, err := a.httpClient.Post(apiURL, "application/json", strings.NewReader(string(reqData)))
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Parse the response
	var response WSSignalResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	// Handle error responses
	if response.Status == "error" {
		return nil, fmt.Errorf("server returned error: %s", response.Error)
	}

	return &response.Signal, nil
}

// Disconnect closes any connection
func (a *WebSocketAdapter) Disconnect() error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// Mark as disconnected
	a.isConnected = false
	return nil
}

// For compatibility with the ClaudeClientInterface
func (a *WebSocketAdapter) GenerateSignal(symbol string) (*TradeSignal, error) {
	// Create dummy market data and portfolio for the call
	marketData := MarketData{
		Symbol:    symbol,
		Price:     100.0,
		High24h:   105.0,
		Low24h:    95.0,
		Volume24h: 10000.0,
		Change24h: 0.5,
	}

	portfolioData := PortfolioData{
		Balance:     10000.0,
		TotalValue:  15000.0,
		DailyPnL:    500.0,
		DailyReturn: 3.33,
		Positions:   make(map[string]PositionData),
	}

	return a.GenerateTradeSignal(symbol, marketData, portfolioData)
}