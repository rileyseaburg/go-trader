package ticker

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata"

)

// TickerData represents the current market data for a ticker
type TickerData struct {
	Symbol      string           `json:"symbol"`
	Trade       *marketdata.Trade `json:"trade"`
	Quote       *marketdata.Quote `json:"quote"`
	Bar         *marketdata.Bar  `json:"bar,omitempty"`
	LastUpdated time.Time        `json:"last_updated"`
}

// TickerDataHandler is a function that handles ticker data
type TickerDataHandler func(symbol string, data TickerData)

// TickerServer manages connections to Alpaca market data streaming API
type TickerServer struct {
	mdClient     *marketdata.Client
	symbols      []string
	symbolsMutex sync.RWMutex
	dataHandler  TickerDataHandler
	ctx          context.Context
	cancel       context.CancelFunc
	
	// Keep track of last data received
	lastData     map[string]TickerData
	dataMutex    sync.RWMutex
}

// NewTickerServer creates a new ticker server
func NewTickerServer(ctx context.Context, isPaperTrading bool, apiKey, apiSecret string) *TickerServer {
	// Create child context so we can cancel it independently
	childCtx, cancel := context.WithCancel(ctx)
	
	// Create market data client
	mdClient := marketdata.NewClient(marketdata.ClientOpts{
		APIKey:    apiKey,
		APISecret: apiSecret,
	})

	return &TickerServer{
		mdClient:    mdClient,
		symbols:     []string{},
		ctx:         childCtx,
		cancel:      cancel,
		lastData:    make(map[string]TickerData),
	}
}

// Start initializes the ticker server
func (ts *TickerServer) Start() error {
	log.Println("Ticker server started successfully")
	
	// Start polling for data
	go ts.pollForData()
	
	return nil
}

// Stop shuts down the ticker server
func (ts *TickerServer) Stop() {
	ts.cancel()
	log.Println("Ticker server stopped")
}

// pollForData polls for market data for the subscribed symbols
func (ts *TickerServer) pollForData() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ts.ctx.Done():
			return
		case <-ticker.C:
			ts.updateMarketData()
		}
	}
}

// updateMarketData fetches latest market data for all symbols
func (ts *TickerServer) updateMarketData() {
	ts.symbolsMutex.RLock()
	symbols := make([]string, len(ts.symbols))
	copy(symbols, ts.symbols)
	ts.symbolsMutex.RUnlock()

	if len(symbols) == 0 {
		return
	}

	// Get latest quotes for all symbols
	for _, symbol := range symbols {
		// Get quote
		quote, err := ts.mdClient.GetLatestQuote(symbol, marketdata.GetLatestQuoteRequest{})
		if err != nil {
			log.Printf("Error getting quote for %s: %v", symbol, err)
			continue
		}

		// Get trade
		trade, err := ts.mdClient.GetLatestTrade(symbol, marketdata.GetLatestTradeRequest{})
		if err != nil {
			log.Printf("Error getting trade for %s: %v", symbol, err)
			continue
		}

		// Create ticker data
		data := TickerData{
			Symbol:      symbol,
			Trade:       trade,
			Quote:       quote,
			LastUpdated: time.Now(),
		}

		// Get bars
		now := time.Now()
		oneHourAgo := now.Add(-1 * time.Hour)
		bars, err := ts.mdClient.GetBars(symbol, marketdata.GetBarsRequest{
			TimeFrame: marketdata.OneMin,
			Start:     oneHourAgo,
			End: 	  now,
		})
		
		if err == nil && len(bars) > 0 {
			data.Bar = &bars[0]
		}

		// Store data
		ts.dataMutex.Lock()
		ts.lastData[symbol] = data
		ts.dataMutex.Unlock()

		// Notify handler
		if ts.dataHandler != nil {
			go ts.dataHandler(symbol, data)
		}
	}
}

// UpdateSymbols updates the symbols to subscribe to
func (ts *TickerServer) UpdateSymbols(symbols []string) error {
	ts.symbolsMutex.Lock()
	defer ts.symbolsMutex.Unlock()

	// Update symbols list
	ts.symbols = make([]string, len(symbols))
	copy(ts.symbols, symbols)
	
	log.Printf("Updated ticker symbols: %v", ts.symbols)
	return nil
}

// GetSymbols returns the current list of symbols
func (ts *TickerServer) GetSymbols() []string {
	ts.symbolsMutex.RLock()
	defer ts.symbolsMutex.RUnlock()

	symbols := make([]string, len(ts.symbols))
	copy(symbols, ts.symbols)
	return symbols
}

// SetDataHandler sets the handler for ticker data
func (ts *TickerServer) SetDataHandler(handler TickerDataHandler) {
	ts.dataHandler = handler
}

// GetLastData returns the last data for a symbol
func (ts *TickerServer) GetLastData(symbol string) (TickerData, error) {
	ts.dataMutex.RLock()
	defer ts.dataMutex.RUnlock()

	data, ok := ts.lastData[symbol]
	if !ok {
		return TickerData{}, fmt.Errorf("no data available for symbol: %s", symbol)
	}

	return data, nil
}

// GetAllLastData returns all last data
func (ts *TickerServer) GetAllLastData() map[string]TickerData {
	ts.dataMutex.RLock()
	defer ts.dataMutex.RUnlock()

	// Create a copy to avoid race conditions
	result := make(map[string]TickerData, len(ts.lastData))
	for k, v := range ts.lastData {
		result[k] = v
	}

	return result
}
