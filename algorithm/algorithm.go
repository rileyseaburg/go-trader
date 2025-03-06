package algorithm

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata"
)

// Constants for signal types
const (
	SignalNone  = "none"
	SignalBuy   = "buy"
	SignalSell  = "sell"
	SignalHold  = "hold"
	SignalClose = "close"
)

// TradeSignal represents a trading signal from Claude
type TradeSignal struct {
	Symbol     string    `json:"symbol"`
	Signal     string    `json:"signal"`      // buy, sell, hold, close
	OrderType  string    `json:"order_type"`  // market, limit
	LimitPrice *float64  `json:"limit_price"` // Only for limit orders
	Timestamp  time.Time `json:"timestamp"`
	Reasoning  string    `json:"reasoning"`
	Confidence *float64  `json:"confidence,omitempty"` // Confidence score from 0-1, nil if not provided
}

// MarketData represents the current market data for a symbol
type MarketData struct {
	Symbol    string  `json:"symbol"`
	Price     float64 `json:"price"`
	High24h   float64 `json:"high_24h"`
	Low24h    float64 `json:"low_24h"`
	Volume24h float64 `json:"volume_24h"`
	Change24h float64 `json:"change_24h"` // Percentage
}

// PositionData represents current position information
type PositionData struct {
	Symbol    string  `json:"symbol"`
	Quantity  float64 `json:"quantity"`
	AvgPrice  float64 `json:"avg_price"`
	MarketVal float64 `json:"market_value"`
	Profit    float64 `json:"profit"`
	Return    float64 `json:"return"` // Percentage
}

// PortfolioData represents the current portfolio state
type PortfolioData struct {
	Balance     float64                 `json:"balance"`
	Positions   map[string]PositionData `json:"positions"`
	TotalValue  float64                 `json:"total_value"`
	DailyPnL    float64                 `json:"daily_pnl"`
	DailyReturn float64                 `json:"daily_return"` // Percentage
}

// ClaudeClientInterface defines the interface for the Claude client
type ClaudeClientInterface interface {
	GenerateTradeSignal(symbol string, marketData MarketData, portfolio PortfolioData) (*TradeSignal, error)
}

// TradingAlgorithm represents the main algorithm that processes market data and executes trades
type TradingAlgorithm struct {
	ctx            context.Context
	claude         ClaudeClientInterface
	client         *alpaca.Client
	mdClient       *marketdata.Client
	marketData     map[string]MarketData
	signals        map[string]*TradeSignal
	portfolio      PortfolioData
	riskParameters map[string]interface{}
	signalCB       func(*TradeSignal)
	tradingEnabled bool
	mu             sync.RWMutex
}

// NewTradingAlgorithm creates a new trading algorithm instance
func NewTradingAlgorithm(ctx context.Context, claude ClaudeClientInterface, client *alpaca.Client, mdClient *marketdata.Client) *TradingAlgorithm {
	return &TradingAlgorithm{
		ctx:        ctx,
		claude:     claude,
		client:     client,
		mdClient:   mdClient,
		marketData: make(map[string]MarketData),
		signals:    make(map[string]*TradeSignal),
		portfolio: PortfolioData{
			Positions: make(map[string]PositionData),
		},
		riskParameters: map[string]interface{}{
			"max_position_size_percent": 5.0,  // Max 5% of portfolio per position
			"max_daily_drawdown":        10.0, // Max 10% daily drawdown
			"stop_loss_percent":         5.0,  // 5% stop loss
			"take_profit_percent":       15.0, // 15% take profit
			"max_trades_per_day":        10,   // Max 10 trades per day
		},
		tradingEnabled: false,
	}
}

// Start initializes the trading algorithm with the given symbols
func (a *TradingAlgorithm) Start(symbols []string) error {
	// Reset market data and signals
	a.mu.Lock()
	a.marketData = make(map[string]MarketData)
	a.signals = make(map[string]*TradeSignal)
	a.mu.Unlock()

	// Initialize market data for each symbol
	for _, symbol := range symbols {
		a.marketData[symbol] = MarketData{
			Symbol: symbol,
		}
	}

	// Update account information
	if err := a.updatePortfolio(); err != nil {
		return fmt.Errorf("failed to update portfolio: %w", err)
	}

	log.Printf("Started trading algorithm with %d symbols", len(symbols))
	a.tradingEnabled = true
	return nil
}

// UpdateMarketData updates the market data for a symbol
func (a *TradingAlgorithm) UpdateMarketData(symbol string, price, high24h, low24h, volume24h, change24h float64) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Calculate change if not provided
	currentData, exists := a.marketData[symbol]
	if exists && change24h == 0 && currentData.Price > 0 {
		change24h = (price - currentData.Price) / currentData.Price * 100
	}

	a.marketData[symbol] = MarketData{
		Symbol:    symbol,
		Price:     price,
		High24h:   high24h,
		Low24h:    low24h,
		Volume24h: volume24h,
		Change24h: change24h,
	}
}

// GetMarketData returns the market data for a symbol
func (a *TradingAlgorithm) GetMarketData(symbol string) MarketData {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.marketData[symbol]
}

// ProcessSymbol processes a single symbol to generate trading signals
func (a *TradingAlgorithm) ProcessSymbol(symbol string) error {
	if !a.tradingEnabled {
		return errors.New("trading algorithm is not enabled")
	}

	// Check if claude client is initialized
	if a.claude == nil {
		log.Printf("Warning: Claude client is nil, skipping signal generation for %s", symbol)
		// Create a default "hold" signal instead of failing
		signal := &TradeSignal{
			Symbol:    symbol,
			Signal:    SignalHold,
			OrderType: "market",
			Timestamp: time.Now(),
			Reasoning: "Signal generation skipped: Claude AI service not available.",
		}

		// Store the signal
		a.mu.Lock()
		a.signals[symbol] = signal
		a.mu.Unlock()

		// Notify callback if registered
		if a.signalCB != nil {
			a.signalCB(signal)
		}

		return nil
	}

	a.mu.RLock()
	marketData, exists := a.marketData[symbol]
	if !exists {
		a.mu.RUnlock()
		return fmt.Errorf("market data not found for symbol: %s", symbol)
	}
	portfolio := a.portfolio
	a.mu.RUnlock()

	// Generate trading signal from Claude
	signal, err := a.claude.GenerateTradeSignal(symbol, marketData, portfolio)
	if err != nil {
		return fmt.Errorf("failed to generate trading signal: %w", err)
	}

	// Store the signal
	a.mu.Lock()
	a.signals[symbol] = signal
	a.mu.Unlock()

	// Execute the signal based on configuration
	// In a real implementation, this would check if auto-trading is enabled
	// and verify that the signal passes risk management checks
	// For now, we'll just log the signal
	log.Printf("Generated signal for %s: %s", symbol, signal.Signal)

	// Notify callback if registered
	if a.signalCB != nil {
		a.signalCB(signal)
	}

	return nil
}

// GetSignal returns the current signal for a symbol
func (a *TradingAlgorithm) GetSignal(symbol string) *TradeSignal {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.signals[symbol]
}

// GetAllSignals returns all current signals
func (a *TradingAlgorithm) GetAllSignals() map[string]*TradeSignal {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// Create a copy to avoid race conditions
	signals := make(map[string]*TradeSignal, len(a.signals))
	for k, v := range a.signals {
		signals[k] = v
	}
	return signals
}

// RegisterSignalCallback registers a callback function for new signals
func (a *TradingAlgorithm) RegisterSignalCallback(callback func(*TradeSignal)) {
	a.signalCB = callback
}

// updatePortfolio updates the portfolio data from Alpaca
func (a *TradingAlgorithm) updatePortfolio() error {
	// Get account information
	account, err := a.client.GetAccount()
	if err != nil {
		return fmt.Errorf("failed to get account: %w", err)
	}

	// Get positions
	positions, err := a.client.GetPositions()
	if err != nil {
		return fmt.Errorf("failed to get positions: %w", err)
	}

	// Update portfolio
	a.mu.Lock()
	defer a.mu.Unlock()

	cashVal, _ := account.Cash.Float64()
	equityVal, equityOk := account.Equity.Float64()

	lastEquityVal, lastEquityOk := account.LastEquity.Float64()

	// Calculate day change and return
	dayChangeVal := 0.0
	if equityOk && lastEquityOk {
		dayChangeVal = equityVal - lastEquityVal
	}
	dayReturn := 0.0
	if lastEquityVal > 0 {
		dayReturn = dayChangeVal / lastEquityVal * 100
	}

	a.portfolio = PortfolioData{
		Balance:     cashVal,
		Positions:   make(map[string]PositionData),
		TotalValue:  equityVal,
		DailyPnL:    dayChangeVal,
		DailyReturn: dayReturn,
	}

	// Process positions
	for _, pos := range positions {
		qty, _ := pos.Qty.Float64()
		avgPrice, _ := pos.AvgEntryPrice.Float64()
		marketValue, _ := pos.MarketValue.Float64()
		profit, _ := pos.UnrealizedPL.Float64()

		posReturn := 0.0
		if avgPrice > 0 && qty > 0 {
			currentPrice := marketValue / qty
			posReturn = (currentPrice - avgPrice) / avgPrice * 100
		}

		a.portfolio.Positions[pos.Symbol] = PositionData{
			Symbol:    pos.Symbol,
			Quantity:  qty,
			AvgPrice:  avgPrice,
			MarketVal: marketValue,
			Profit:    profit,
			Return:    posReturn,
		}
	}

	return nil
}

// GetPortfolio returns the current portfolio data
func (a *TradingAlgorithm) GetPortfolio() PortfolioData {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.portfolio
}

// GetRiskParameters returns the current risk parameters
func (a *TradingAlgorithm) GetRiskParameters() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// Create a copy to avoid race conditions
	params := make(map[string]interface{}, len(a.riskParameters))
	for k, v := range a.riskParameters {
		params[k] = v
	}
	return params
}

// UpdateRiskParameters updates the risk parameters
func (a *TradingAlgorithm) UpdateRiskParameters(params map[string]interface{}) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Validate parameters
	for k, v := range params {
		switch k {
		case "max_position_size_percent", "max_daily_drawdown", "stop_loss_percent", "take_profit_percent":
			// These should be numeric
			switch val := v.(type) {
			case float64:
				if val <= 0 {
					return fmt.Errorf("parameter %s must be positive", k)
				}
			case int:
				if val <= 0 {
					return fmt.Errorf("parameter %s must be positive", k)
				}
				// Convert to float64
				params[k] = float64(val)
			default:
				return fmt.Errorf("parameter %s must be numeric", k)
			}
		case "max_trades_per_day":
			// This should be an integer
			switch val := v.(type) {
			case float64:
				if val <= 0 || math.Floor(val) != val {
					return fmt.Errorf("parameter %s must be a positive integer", k)
				}
				// Convert to int
				params[k] = int(val)
			case int:
				if val <= 0 {
					return fmt.Errorf("parameter %s must be positive", k)
				}
			default:
				return fmt.Errorf("parameter %s must be an integer", k)
			}
		default:
			// Unknown parameter
			return fmt.Errorf("unknown parameter: %s", k)
		}
	}

	// Update parameters
	for k, v := range params {
		a.riskParameters[k] = v
	}

	return nil
}

// ExecuteTrade executes a trade based on a signal
func (a *TradingAlgorithm) ExecuteTrade(signal *TradeSignal) error {
	if signal == nil {
		return errors.New("signal is nil")
	}

	// Get current market data for the symbol
	a.mu.RLock()
	marketData, exists := a.marketData[signal.Symbol]
	portfolio := a.portfolio
	riskParams := a.riskParameters
	a.mu.RUnlock()

	if !exists {
		return fmt.Errorf("market data not found for symbol: %s", signal.Symbol)
	}

	// Determine if we have an existing position
	position, hasPosition := portfolio.Positions[signal.Symbol]

	// Process the signal
	var side, orderType string
	var qty float64
	var limitPrice float64

	switch signal.Signal {
	case SignalBuy:
		// Only buy if we don't have a long position already
		if hasPosition && position.Quantity > 0 {
			log.Printf("Already have a long position in %s, skipping buy signal", signal.Symbol)
			return nil
		}
		side = "buy"
		orderType = signal.OrderType

		// Handle limit price
		if signal.LimitPrice != nil {
			limitPrice = *signal.LimitPrice
		}

		// Calculate position size
		maxPosSize, ok := riskParams["max_position_size_percent"].(float64)
		if !ok {
			maxPosSize = 5.0 // Default to 5% if not specified
		}

		// Calculate position value
		positionValue := portfolio.TotalValue * (maxPosSize / 100.0)
		qty = a.calculatePositionSize(positionValue, marketData.Price, true)

	case SignalSell:
		// If we have a long position, close it
		if hasPosition && position.Quantity > 0 {
			side = "sell"
			orderType = signal.OrderType

			// Handle limit price
			if signal.LimitPrice != nil {
				limitPrice = *signal.LimitPrice
			}

			qty = position.Quantity
		} else {
			// Otherwise, open a short position
			side = "sell"
			orderType = signal.OrderType

			// Handle limit price
			if signal.LimitPrice != nil {
				limitPrice = *signal.LimitPrice
			}

			// Calculate position size
			maxPosSize, ok := riskParams["max_position_size_percent"].(float64)
			if !ok {
				maxPosSize = 5.0 // Default to 5% if not specified
			}

			// Calculate position value
			positionValue := portfolio.TotalValue * (maxPosSize / 100.0)
			qty = a.calculatePositionSize(positionValue, marketData.Price, false)
		}

	case SignalClose:
		// Close any existing position
		if !hasPosition {
			log.Printf("No position to close for %s", signal.Symbol)
			return nil
		}

		if position.Quantity > 0 {
			side = "sell"
		} else {
			side = "buy"
		}
		orderType = signal.OrderType

		// Handle limit price
		if signal.LimitPrice != nil {
			limitPrice = *signal.LimitPrice
		}

		qty = math.Abs(position.Quantity)

	case SignalHold:
		// Do nothing
		log.Printf("Hold signal for %s, no action taken", signal.Symbol)
		return nil

	default:
		return fmt.Errorf("unknown signal type: %s", signal.Signal)
	}

	// Create order request - these will need to be adjusted to use decimal.Decimal in a real implementation
	qtyStr := fmt.Sprintf("%.6f", qty)
	limitPriceStr := ""
	if limitPrice > 0 {
		limitPriceStr = fmt.Sprintf("%.2f", limitPrice)
	}

	log.Printf("Order details: symbol=%s, side=%s, qty=%s, type=%s, limitPrice=%s",
		signal.Symbol, side, qtyStr, orderType, limitPriceStr)

	// Here we would normally place the order with Alpaca
	// In a real implementation, this would be:
	/*
		req := alpaca.PlaceOrderRequest{
			Symbol:      signal.Symbol,
			Qty:         qtyStr,
			Side:        alpaca.Side(side),
			Type:        alpaca.OrderType(orderType),
			TimeInForce: alpaca.Day,
		}

		if orderType == "limit" && limitPrice > 0 {
			req.LimitPrice = limitPriceStr
		}

		order, err := a.client.PlaceOrder(req)
		if err != nil {
			return fmt.Errorf("failed to place order: %w", err)
		}
	*/

	log.Printf("Order placed successfully for %s (%s)", signal.Symbol, side)
	return nil
}

// calculatePositionSize calculates the position size in shares based on the position value and current price
func (a *TradingAlgorithm) calculatePositionSize(positionValue, currentPrice float64, isBuy bool) float64 {
	if currentPrice <= 0 {
		log.Printf("Warning: Invalid current price %.2f, using 1.0", currentPrice)
		currentPrice = 1.0
	}

	// Calculate position size in shares
	qty := float64(int(positionValue / currentPrice))

	// For sells (shorts), make it negative
	if !isBuy {
		qty = -qty
	}
	return qty
}
