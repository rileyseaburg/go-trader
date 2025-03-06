package algorithm

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// TradeState represents the state of trading for a single symbol
type TradeState struct {
	Symbol       string      `json:"symbol"`
	LastUpdate   time.Time   `json:"last_update"`
	PendingOrder interface{} `json:"pending_order,omitempty"`
}

// AlgorithmStatus represents the current status of the trading algorithm
type AlgorithmStatus struct {
	IsRunning              bool                    `json:"is_running"`
	ActiveSymbols          []string                `json:"active_symbols"`
	LastSignals            map[string]*TradeSignal `json:"last_signals"`
	RiskParameters         map[string]interface{}  `json:"risk_parameters"`
	TradesExecutedToday    int                     `json:"trades_executed_today"`
	TradesExecutedThisWeek int                     `json:"trades_executed_this_week"`
	StartTime              time.Time               `json:"start_time,omitempty"`
	Version                string                  `json:"version"`
}

// These fields should be added to TradingAlgorithm in a full implementation
// For now, we'll use these global variables for demonstration
var (
	globalStatesMutex       sync.RWMutex
	globalStates            = make(map[string]*TradeState)
	globalTradingCountMutex sync.RWMutex
	globalTradeCount        = make(map[string]int)
)

// GetStatus returns the current status of the trading algorithm
func (a *TradingAlgorithm) GetStatus() *AlgorithmStatus {
	// Use our existing GetAllSignals method
	signals := a.GetAllSignals()

	// Get active symbols from our marketData map
	a.mu.RLock()
	symbols := make([]string, 0, len(a.marketData))
	for symbol := range a.marketData {
		symbols = append(symbols, symbol)
	}
	a.mu.RUnlock()

	// For trade counts, we'll use the global variables for now
	// In a real implementation, these would be fields in TradingAlgorithm
	globalTradingCountMutex.RLock()
	today := time.Now().Format("2006-01-02")
	thisWeek := time.Now().Format("2006-W02")

	dailyKey := "daily:" + today
	weeklyKey := "weekly:" + thisWeek

	tradesExecutedToday := globalTradeCount[dailyKey]
	tradesExecutedThisWeek := globalTradeCount[weeklyKey]
	globalTradingCountMutex.RUnlock()

	return &AlgorithmStatus{
		IsRunning:              a.tradingEnabled,
		ActiveSymbols:          symbols,
		LastSignals:            signals,
		RiskParameters:         a.GetRiskParameters(),
		TradesExecutedToday:    tradesExecutedToday,
		TradesExecutedThisWeek: tradesExecutedThisWeek,
		Version:                "1.0.0",
	}
}

// Stop stops the trading algorithm
func (a *TradingAlgorithm) Stop() error {
	log.Printf("Stopping trading algorithm")

	// Set trading enabled to false
	a.mu.Lock()
	wasEnabled := a.tradingEnabled
	a.tradingEnabled = false
	a.mu.Unlock()

	if !wasEnabled {
		log.Printf("Trading algorithm was already stopped")
		return nil
	}

	// In a full implementation, we would cancel pending orders
	// For now, just log that we're stopping
	log.Printf("Trading algorithm stopped successfully")
	return nil
}

// UpdateStatus updates the trading algorithm status with new symbols
func (a *TradingAlgorithm) UpdateStatus(symbols []string) error {
	if len(symbols) == 0 {
		return fmt.Errorf("no symbols provided")
	}

	// Update our existing marketData map
	a.mu.Lock()
	for _, symbol := range symbols {
		if _, exists := a.marketData[symbol]; !exists {
			a.marketData[symbol] = MarketData{
				Symbol: symbol,
			}
			log.Printf("Added symbol %s to trading algorithm", symbol)
		}
	}
	a.mu.Unlock()

	return nil
}
