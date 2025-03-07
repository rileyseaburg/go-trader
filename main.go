package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rileyseaburg/go-trader/types"

	// Ensure algorithm package is imported first, before algorithm/algo,
	// to avoid any import conflict or shadowing issues
	"github.com/rileyseaburg/go-trader/algorithm"
	"github.com/rileyseaburg/go-trader/algorithm/algo"
	"github.com/rileyseaburg/go-trader/claude"
	"github.com/rileyseaburg/go-trader/notification"
	"github.com/rileyseaburg/go-trader/ticker"

	"github.com/alpacahq/alpaca-trade-api-go/v3/alpaca"
	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata"
	"github.com/joho/godotenv"
	"github.com/shopspring/decimal"
)

// convertHistoricalDataToMarketData converts from types.HistoricalData to []types.MarketData
func convertHistoricalDataToMarketData(data *types.HistoricalData) []types.MarketData {
	if data == nil || len(data.Data) == 0 {
		return []types.MarketData{}
	}

	result := make([]types.MarketData, len(data.Data))
	for i, point := range data.Data {
		result[i] = types.MarketData{
			Symbol:    point.Symbol,
			Price:     point.Close,           // Using Close price as the current price
			High24h:   point.High,            // Using daily high
			Low24h:    point.Low,             // Using daily low
			Volume24h: float64(point.Volume), // Converting volume
			Change24h: 0,                     // Not available in historical data
		}
	}
	return result
}

const (
	defaultPort      = "8080"
	defaultSymbols   = "AAPL,MSFT,TSLA"
	paperTradingURL  = "https://paper-api.alpaca.markets"
	liveTradingURL   = "https://api.alpaca.markets"
	paperKeyPrefix   = "PK" // Paper API keys usually start with PK
	liveKeyPrefix    = "AK" // Live API keys usually start with AK
	maxNotifications = 100  // Maximum notifications to store
)
const dataDir = "./data" // Directory for storing persistent data like ticker baskets

// PriceTracker tracks previous prices for market event detection
type PriceTracker struct {
	prices map[string]float64
	mu     sync.RWMutex
}

// NewPriceTracker creates a new price tracker
func NewPriceTracker() *PriceTracker {
	return &PriceTracker{
		prices: make(map[string]float64),
	}
}

// GetPreviousPrice returns the previous price for a symbol
func (pt *PriceTracker) GetPreviousPrice(symbol string) float64 {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	return pt.prices[symbol]
}

// UpdatePrice updates the price for a symbol
func (pt *PriceTracker) UpdatePrice(symbol string, price float64) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.prices[symbol] = price
}

func main() {
	// Load .env file
	// Define global API key variables that will be used throughout the application
	var alpacaAPIKey, alpacaSecretKey string

	if err := godotenv.Load(); err != nil {
		// If .env file doesn't exist, log a warning but continue
		if !errors.Is(err, os.ErrNotExist) {
			log.Printf("Warning: Error loading .env file: %v", err)
		}
	}

	// Parse command line arguments
	port := flag.String("port", defaultPort, "Port to listen on")
	symbols := flag.String("symbols", defaultSymbols, "Comma-separated list of ticker symbols")
	usePaperTrading := flag.Bool("paper", true, "Use paper trading (true) or live trading (false)")

	// Add flags for API keys that can be used instead of environment variables
	alpacaKey := flag.String("alpaca-key", "", "Alpaca API key (overrides env var)")
	alpacaSecret := flag.String("alpaca-secret", "", "Alpaca secret key (overrides env var)")

	// Log to verify that the environment variables are being loaded
	log.Printf("DEBUG: Checking for Alpaca API Keys in environment...")
	flag.Parse()

	// Get appropriate API keys from environment based on trading mode

	if *usePaperTrading {
		alpacaAPIKey = os.Getenv("PAPER_ALPACA_API_KEY")
		// Check for both correct spelling and potential typo
		alpacaSecretKey = os.Getenv("PAPER_ALPACA_SECRET_KEY")
		if alpacaSecretKey == "" {
			alpacaSecretKey = os.Getenv("PAPAER_ALPACA_SECRET_KEY") // Handle potential typo
		}
	} else {
		alpacaAPIKey = os.Getenv("LIVE_ALPACA_API_KEY")
		alpacaSecretKey = os.Getenv("LIVE_ALPACA_SECRET_KEY")
	}

	// Override with command line flags if provided
	if *alpacaKey != "" {
		alpacaAPIKey = *alpacaKey
		log.Printf("Using Alpaca API Key from command line")
	}
	if *alpacaSecret != "" {
		alpacaSecretKey = *alpacaSecret
		log.Printf("Using Alpaca Secret Key from command line")
	}

	// Log the key being used (first few characters only)
	if alpacaAPIKey != "" {
		log.Printf("DEBUG: Using Alpaca API Key: %s...", alpacaAPIKey[:5]+"...")
	}

	// Validate required API keys
	if alpacaAPIKey == "" || alpacaSecretKey == "" {
		if *usePaperTrading {
			log.Fatal("PAPER_ALPACA_API_KEY and PAPER_ALPACA_SECRET_KEY environment variables are required for paper trading")
		} else {
			log.Fatal("LIVE_ALPACA_API_KEY and LIVE_ALPACA_SECRET_KEY environment variables are required for live trading")
		}
	}

	var baseURL string
	if *usePaperTrading {
		baseURL = paperTradingURL
		log.Println("Using PAPER trading environment")
	} else {
		// Safety check: Only allow live trading if the API key has the correct prefix
		if !strings.HasPrefix(alpacaAPIKey, liveKeyPrefix) {
			log.Println("WARNING: Cannot use live trading - live API keys not detected (keys should start with AK)")
			log.Println("Falling back to paper trading mode")
			*usePaperTrading = true
			baseURL = paperTradingURL
		} else {
			baseURL = liveTradingURL
			log.Println("Using LIVE trading environment")
		}
	}

	// Split symbols into a slice
	symbolsSlice := strings.Split(*symbols, ",")
	for i, s := range symbolsSlice {
		symbolsSlice[i] = strings.TrimSpace(s)
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize Alpaca clients
	client := alpaca.NewClient(alpaca.ClientOpts{
		APIKey:    alpacaAPIKey,
		APISecret: alpacaSecretKey,
		BaseURL:   baseURL,
	})

	mdClient := marketdata.NewClient(marketdata.ClientOpts{
		APIKey:    alpacaAPIKey,
		APISecret: alpacaSecretKey,
	})

	// Initialize tading algorithm
	// Create Claude WebSocket adapter for communication with the Next.js frontend
	claudeAdapter := claude.NewWebSocketAdapterWrapper("http://localhost:3000")

	// Adapt Claude adapter to the algorithm's ClaudeClientInterface
	adaptedClaudeAdapter := &adaptedClaudeClient{claudeAdapter}

	// Initialize algorithm with the Claude adapter
	tradingAlgorithm := algorithm.NewTradingAlgorithm(ctx, adaptedClaudeAdapter, client, mdClient)

	// Initialize basket manager
	basketManager, err := ticker.NewBasketManager(dataDir)
	if err != nil {
		log.Fatalf("Failed to initialize basket manager: %v", err)
	}

	// Initialize notification manager
	notificationService := notification.NewNotificationManager(maxNotifications)

	// Create system startup notification
	log.Println("Initializing system with notification service")
	notificationService.AddNotification(notification.CreateSystemAlertNotification("System Started", "Trading system successfully initialized", nil))

	// Initialize price tracker
	priceTracker := NewPriceTracker()

	// Start the ticker server
	// Get API keys for ticker server
	var tickerAPIKey, tickerAPISecret string
	if *usePaperTrading {
		tickerAPIKey = alpacaAPIKey
		tickerAPISecret = alpacaSecretKey
	} else {
		tickerAPIKey = alpacaAPIKey
		tickerAPISecret = alpacaSecretKey
	}

	// Create ticker server with the correct API keys
	tickerServer := ticker.NewTickerServer(ctx, *usePaperTrading, tickerAPIKey, tickerAPISecret)
	if err := tickerServer.Start(); err != nil {
		log.Fatalf("Failed to start ticker server: %v", err)
	}

	// Set the initial symbols
	if err := tickerServer.UpdateSymbols(symbolsSlice); err != nil {
		log.Fatalf("Failed to set initial symbols: %v", err)
	}

	// Start the trading algorithm
	// Initialize but don't enable automatic trading - only symbols will be processed
	// when explicitly triggered from the frontend UI
	tradingAlgorithm.Start(symbolsSlice)
	log.Println("Trading algorithm initialized but not auto-running - waiting for UI trigger")

	// Register signal callback for notifications
	tradingAlgorithm.RegisterSignalCallback(func(signal *algorithm.TradeSignal) {
		// Convert signal priority based on type
		var priority notification.NotificationPriority
		if signal.Signal == algorithm.SignalBuy || signal.Signal == algorithm.SignalSell {
			priority = notification.PriorityHigh
		} else {
			priority = notification.PriorityMedium
		}

		// Create notification for the signal
		notif := notification.CreateSignalGeneratedNotification(
			signal.Symbol, signal.Signal, signal.Reasoning, priority, nil)
		notificationService.AddNotification(notif)
	})

	// Set up market data handler to forward data from ticker to algorithm
	tickerServer.SetDataHandler(func(symbol string, trade ticker.TickerData) {
		tradingAlgorithm.UpdateMarketData(
			symbol,
			trade.Trade.Price,
			trade.Trade.Price*1.05,         // Placeholder for 24h high
			trade.Trade.Price*0.95,         // Placeholder for 24h low
			float64(trade.Trade.Size)*1000, // Convert uint32 to float64
			0,                              // Placeholder for 24h change percentage
		)

		// Process the symbol to generate trading signals
		// Only process if explicitly triggered by UI (don't auto-process for all data updates)
		go func(s string) {
			// Market data is updated, but signals are only generated
			// when requested from the frontend to avoid excessive Claude API calls
			// if err := tradingAlgorithm.ProcessSymbol(s); err != nil {
			//    log.Printf("Error processing symbol %s: %v", s, err)
			// }
		}(symbol)

		// Create market event notification for significant price changes (>2%)
		currentPrice := trade.Trade.Price
		prevPrice := priceTracker.GetPreviousPrice(symbol)

		if prevPrice > 0 {
			priceChange := (currentPrice - prevPrice) / prevPrice
			if priceChange > 0.02 {
				notif := notification.CreateMarketEventNotification(
					symbol,
					"Significant Price Increase",
					fmt.Sprintf("%s price increased by %.2f%% to %s",
						symbol,
						priceChange*100,
						fmt.Sprintf("$%.2f", currentPrice)),
				)
				notificationService.AddNotification(notif)
			} else if priceChange < -0.02 {
				notif := notification.CreateMarketEventNotification(
					symbol,
					"Significant Price Decrease",
					fmt.Sprintf("%s price decreased by %.2f%% to %s",
						symbol,
						-priceChange*100,
						fmt.Sprintf("$%.2f", currentPrice)),
				)
				notificationService.AddNotification(notif)
			}
		}

		// Update the previous price for next comparison
		priceTracker.UpdatePrice(symbol, currentPrice)
	})

	// Set up HTTP handlers, passing API keys for order handlers to use
	setupHTTPHandlers(client, tradingAlgorithm, tickerServer, basketManager, notificationService,
		alpacaAPIKey, alpacaSecretKey)

	log.Printf("Starting HTTP server on port %s", *port)
	if err := http.ListenAndServe(":"+*port, nil); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}

type SignalGeneratorFunc func(string) (*algorithm.TradeSignal, error)

// adaptedClaudeClient adapts the claude.WebSocketAdapterWrapper to the algorithm.ClaudeClientInterface
type adaptedClaudeClient struct {
	*claude.WebSocketAdapterWrapper
}

// GenerateTradeSignal adapts the method signature
func (a *adaptedClaudeClient) GenerateTradeSignal(symbol string, marketData algorithm.MarketData, portfolioData algorithm.PortfolioData) (*algorithm.TradeSignal, error) {
	// Convert algorithm types to claude types
	claudeMarketData := claude.AlgorithmMarketData{
		Price:     marketData.Price,
		High24h:   marketData.High24h,
		Low24h:    marketData.Low24h,
		Volume24h: marketData.Volume24h,
		Change24h: marketData.Change24h,
	}

	claudePortfolioData := claude.AlgorithmPortfolioData{
		Balance:     portfolioData.Balance,
		TotalValue:  portfolioData.TotalValue,
		DailyPnL:    portfolioData.DailyPnL,
		DailyReturn: portfolioData.DailyReturn,
		Positions:   make(map[string]claude.AlgorithmPositionData),
	}

	// Convert positions from algorithm to claude types
	for symbol, pos := range portfolioData.Positions {
		claudePortfolioData.Positions[symbol] = claude.AlgorithmPositionData{
			Symbol:    pos.Symbol,
			Quantity:  pos.Quantity,
			AvgPrice:  pos.AvgPrice,
			MarketVal: pos.MarketVal,
			Profit:    pos.Profit,
			Return:    pos.Return,
		}
	}

	// Call the original GenerateTradeSignal method
	claudeSignal, err := a.WebSocketAdapterWrapper.GenerateTradeSignal(symbol, claudeMarketData, claudePortfolioData)
	if err != nil {
		return nil, err
	}

	// Convert claude types to algorithm types
	signal := &algorithm.TradeSignal{
		Symbol:    claudeSignal.Symbol,
		Signal:    claudeSignal.Signal,
		OrderType: claudeSignal.OrderType,
		Timestamp: claudeSignal.Timestamp,
		Reasoning: claudeSignal.Reasoning,
	}

	if claudeSignal.LimitPrice != nil {
		limitPrice := *claudeSignal.LimitPrice
		signal.LimitPrice = &limitPrice
	}

	if claudeSignal.Confidence != nil {
		confidence := *claudeSignal.Confidence
		signal.Confidence = &confidence
	}

	return signal, nil
}

func setupHTTPHandlers(client *alpaca.Client, tradingAlgo *algorithm.TradingAlgorithm, tickerServer *ticker.TickerServer,
	basketManager *ticker.BasketManager, notificationManager *notification.NotificationManager, apiKey, apiSecret string) {
	// Create a registry for the Lopez de Prado algorithms
	var algoRegistry = make(map[string]interface{})

	// Create notification handler to register routes
	notificationHandler := notification.NewNotificationHandler(notificationManager)

	// Function to generate signal without execution
	generateSignalWithoutExecution := func(algo *algorithm.TradingAlgorithm, symbol string) (*algorithm.TradeSignal, error) {
		// Simply delegate to the algorithm's existing signal generator
		// Get the current signal for the symbol - assuming GetSignal is the right method
		signal := algo.GetSignal(symbol)
		if signal == nil {
			return nil, fmt.Errorf("no signal available for %s", symbol)
		}
		return signal, nil
	}

	corsMiddleware := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next(w, r)
		}
	}

	// Account Handler
	http.HandleFunc("/api/account", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		acct, err := client.GetAccount()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(acct)
	}))

	// Positions Handler
	http.HandleFunc("/api/positions", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		positions, err := client.GetPositions()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(positions)
	}))

	// Orders Handler
	http.HandleFunc("/api/orders", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		orders, err := client.GetOrders(alpaca.GetOrdersRequest{
			Status: "all",
			Limit:  100,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(orders)
	}))

	// Tickers Handler - GET current tickers, POST to update
	http.HandleFunc("/api/tickers", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			// Get current symbols
			symbols := tickerServer.GetSymbols()

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"symbols": symbols,
			})
			return
		}

		if r.Method == http.MethodPost {
			// Update symbols
			var request struct {
				Symbols []string `json:"symbols"`
			}

			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				http.Error(w, "Invalid request body", http.StatusBadRequest)
				return
			}

			if err := tickerServer.UpdateSymbols(request.Symbols); err != nil {
				http.Error(w, fmt.Sprintf("Failed to update symbols: %v", err), http.StatusInternalServerError)
				return
			}

			// Also update the algorithm's symbols
			if err := tradingAlgo.Start(request.Symbols); err != nil {
				http.Error(w, fmt.Sprintf("Failed to update algorithm symbols: %v", err), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"message": "Symbols updated successfully",
				"symbols": request.Symbols,
			})
			return
		}

		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}))

	// Trading Signals Handler
	http.HandleFunc("/api/signals", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		// Get symbol from query string
		symbol := r.URL.Query().Get("symbol")

		var signals interface{}
		if symbol != "" {
			// Get signal for specific symbol
			signal := tradingAlgo.GetSignal(symbol)
			if signal == nil {
				http.Error(w, "No signal available for symbol", http.StatusNotFound)
				return
			}
			signals = signal
		} else {
			// Get all signals
			signals = tradingAlgo.GetAllSignals()
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(signals)
	}))

	// Ticker Recommendations Handler
	http.HandleFunc("/api/recommendations", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Get sector from query string, default to empty (which gives general recommendations)
		sector := r.URL.Query().Get("sector")

		// Get max results parameter, default to 5
		maxResults := 5
		if maxParam := r.URL.Query().Get("max"); maxParam != "" {
			if max, err := strconv.Atoi(maxParam); err == nil && max > 0 {
				maxResults = max
			}
		}

		// Get recommendations
		recommendations, err := tradingAlgo.RecommendTickers(sector, maxResults)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get recommendations: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(recommendations)
	}))

	// Risk Parameters Handler
	http.HandleFunc("/api/risk-parameters", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			// Get current risk parameters
			params := tradingAlgo.GetRiskParameters()

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(params)
			return
		}

		if r.Method == http.MethodPost {
			// Update risk parameters
			var request map[string]interface{}

			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				http.Error(w, "Invalid request body", http.StatusBadRequest)
				return
			}

			if err := tradingAlgo.UpdateRiskParameters(request); err != nil {
				http.Error(w, fmt.Sprintf("Failed to update risk parameters: %v", err), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"message":    "Risk parameters updated successfully",
				"parameters": request,
			})
			return
		}

		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}))

	// Baskets Handler - List and Create
	http.HandleFunc("/api/baskets", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			// List all baskets
			baskets := basketManager.ListBaskets()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(baskets)
			return
		}

		if r.Method == http.MethodPost {
			// Create a new basket
			var basket ticker.TickerBasket
			if err := json.NewDecoder(r.Body).Decode(&basket); err != nil {
				http.Error(w, "Invalid request body", http.StatusBadRequest)
				return
			}

			// Set created/updated timestamps
			now := time.Now().Format(time.RFC3339)
			basket.CreatedAt = now
			basket.UpdatedAt = now

			if err := basketManager.SaveBasket(&basket); err != nil {
				http.Error(w, fmt.Sprintf("Failed to save basket: %v", err), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(basket)
			return
		}

		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}))

	// Individual Basket Handler - Get, Update, Delete
	http.HandleFunc("/api/baskets/", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		// Extract path to determine if it's for a specific basket or symbols
		path := strings.TrimPrefix(r.URL.Path, "/api/baskets/")
		parts := strings.Split(path, "/")

		// Handle /api/baskets/{id}/symbols endpoint
		if len(parts) == 2 && parts[1] == "symbols" {
			basketID := parts[0]

			if r.Method == http.MethodPost {
				// Add symbol to basket
				var request struct {
					Symbol string `json:"symbol"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					http.Error(w, "Invalid request body", http.StatusBadRequest)
					return
				}

				if err := basketManager.AddSymbolToBasket(basketID, request.Symbol); err != nil {
					http.Error(w, fmt.Sprintf("Failed to add symbol to basket: %v", err), http.StatusInternalServerError)
					return
				}

				// Get updated basket
				basket, err := basketManager.GetBasket(basketID)
				if err != nil {
					http.Error(w, fmt.Sprintf("Failed to get updated basket: %v", err), http.StatusInternalServerError)
					return
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(basket)
				return
			}

			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Handle /api/baskets/{id} endpoint
		basketID := parts[0]
		if basketID == "" {
			http.Error(w, "Invalid basket ID", http.StatusBadRequest)
			return
		}

		if r.Method == http.MethodGet {
			// Get basket details
			basket, err := basketManager.GetBasket(basketID)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to get basket: %v", err), http.StatusNotFound)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(basket)
			return
		}

		if r.Method == http.MethodPut {
			// Update basket
			var basket ticker.TickerBasket
			if err := json.NewDecoder(r.Body).Decode(&basket); err != nil {
				http.Error(w, "Invalid request body", http.StatusBadRequest)
				return
			}

			// Set updated timestamp
			basket.UpdatedAt = time.Now().Format(time.RFC3339)

			if err := basketManager.SaveBasket(&basket); err != nil {
				http.Error(w, fmt.Sprintf("Failed to update basket: %v", err), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(basket)
			return
		}

		if r.Method == http.MethodDelete {
			// Delete basket
			if err := basketManager.DeleteBasket(basketID); err != nil {
				http.Error(w, fmt.Sprintf("Failed to delete basket: %v", err), http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"message": "Basket deleted successfully"})
			return
		}

		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}))

	// Trade Basket Handler - Trade all symbols in a basket
	http.HandleFunc("/api/baskets/trade/", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		basketID := strings.TrimPrefix(r.URL.Path, "/api/baskets/trade/")
		if basketID == "" {
			http.Error(w, "Invalid basket ID", http.StatusBadRequest)
			return
		}

		if r.Method == http.MethodPost {
			// Get the basket
			basket, err := basketManager.GetBasket(basketID)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to get basket: %v", err), http.StatusNotFound)
				return
			}

			// Check if basket has symbols
			if len(basket.Symbols) == 0 {
				http.Error(w, "Basket has no symbols", http.StatusBadRequest)
				return
			}

			// Update the ticker server with the basket symbols
			if err := tickerServer.UpdateSymbols(basket.Symbols); err != nil {
				http.Error(w, fmt.Sprintf("Failed to update symbols: %v", err), http.StatusInternalServerError)
				return
			}

			// Also update the trading algorithm
			if err := tradingAlgo.Start(basket.Symbols); err != nil {
				http.Error(w, fmt.Sprintf("Failed to update algorithm symbols: %v", err), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"message": fmt.Sprintf("Started trading %d symbols from basket '%s'", len(basket.Symbols), basket.Name),
				"symbols": basket.Symbols,
			})
			return
		}

		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}))

	// Historical Data and Analysis Handler
	http.HandleFunc("/api/historical", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			// Get query parameters
			symbol := r.URL.Query().Get("symbol")
			if symbol == "" {
				http.Error(w, "Symbol is required", http.StatusBadRequest)
				return
			}

			timeFrame := r.URL.Query().Get("timeframe")
			if timeFrame == "" {
				timeFrame = "1D" // Default to daily
			}

			// Parse dates if provided
			var startDate, endDate time.Time
			startStr := r.URL.Query().Get("start")
			endStr := r.URL.Query().Get("end")

			if startStr != "" {
				var err error
				startDate, err = time.Parse("2006-01-02", startStr)
				if err != nil {
					http.Error(w, "Invalid start date format. Use YYYY-MM-DD", http.StatusBadRequest)
					return
				}
			} else {
				// Default to 30 days ago
				startDate = time.Now().AddDate(0, 0, -30)
			}

			if endStr != "" {
				var err error
				endDate, err = time.Parse("2006-01-02", endStr)
				if err != nil {
					http.Error(w, "Invalid end date format. Use YYYY-MM-DD", http.StatusBadRequest)
					return
				}
			} else {
				// Default to today
				endDate = time.Now()
			}

			// Create request 
			request := types.HistoricalDataRequest{
				Symbol:    symbol,
				StartDate: startDate,
				EndDate:   endDate,
				TimeFrame: timeFrame,
			}

			// Get historical data
			data, err := tradingAlgo.GetHistoricalData(request)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to get historical data: %v", err), http.StatusInternalServerError)
				return
			}

			// Get analysis if requested
			analyze := r.URL.Query().Get("analyze")
			if analyze == "true" {
				analysis := tradingAlgo.AnalyzeHistoricalData(data)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(analysis)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(data)
			return
		}

		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}))

	// Claude WebSocket endpoint for streaming responses
	// Note: This route is already registered by claudeHandler.RegisterRoutes in the main function
	// The duplicate registration was causing a panic:
	// "panic: pattern "/ws/claude" conflicts with pattern "/ws/claude""
	// claudeHandler handles this endpoint correctly

	// Signal generation with manual approval
	http.HandleFunc("/api/signals/generate", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		symbol := r.URL.Query().Get("symbol")
		if symbol == "" {
			http.Error(w, "Symbol is required", http.StatusBadRequest)
			return
		}

		// Generate the signal but don't execute it yet
		signal, err := generateSignalWithoutExecution(tradingAlgo, symbol)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to generate signal: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(signal)
	}))

	// Execute trade endpoint - receives signals from the frontend AI integration
	http.HandleFunc("/api/executeTrade", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		log.Printf("Received request to execute trade from frontend")
		var request struct {
			Symbol     string  `json:"symbol"`
			Signal     string  `json:"signal"`
			OrderType  string  `json:"order_type"`
			LimitPrice float64 `json:"limit_price,omitempty"`
			Reasoning  string  `json:"reasoning,omitempty"`
			Confidence float64 `json:"confidence,omitempty"`
		}

		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
			return
		}

		// Validate the request
		if request.Symbol == "" || request.Signal == "" || request.OrderType == "" {
			http.Error(w, "Missing required fields: symbol, signal, and order_type are required", http.StatusBadRequest)
			return
		}

		// Create a pointer to the limit price
		var limitPricePtr *float64
		if request.LimitPrice > 0 {
			limitPrice := request.LimitPrice
			limitPricePtr = &limitPrice
		}

		// Convert the request to a trade signal
		signal := &algorithm.TradeSignal{
			Symbol:     request.Symbol,
			Signal:     request.Signal,
			OrderType:  request.OrderType,
			LimitPrice: limitPricePtr,
			Timestamp:  time.Now(),
			Reasoning:  request.Reasoning,
		}

		// Add confidence if provided
		if request.Confidence > 0 {
			confidenceVal := request.Confidence
			signal.Confidence = &confidenceVal
			log.Printf("Signal confidence: %.2f", *signal.Confidence)
		}

		// Log the signal
		log.Printf("Received trade signal: %+v", signal)

		// Execute the trade based on the signal
		var result string
		var err error

		// Execute different actions based on the signal type
		switch signal.Signal {
		case "buy":
			result, err = executeBuyOrder(client, signal, apiKey, apiSecret)
		case "sell":
			result, err = executeSellOrder(client, signal, apiKey, apiSecret)
		case "hold":
			result = "No trade executed for hold signal"
			err = nil
		default:
			err = fmt.Errorf("invalid signal type: %s", signal.Signal)
		}

		if err != nil {
			// Return error as JSON instead of plain text
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":   fmt.Sprintf("Error executing trade: %v", err),
				"success": false,
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": result,

			"symbol":     signal.Symbol,
			"signal":     signal.Signal,
			"confidence": signal.Confidence,
			"reasoning":  signal.Reasoning,
			"order_id":   fmt.Sprintf("ord_%s", time.Now().Format("20060102150405")),
			"timestamp":  time.Now().Format(time.RFC3339),
		})
	}))

	// Reject signal (don't execute)
	http.HandleFunc("/api/signals/reject", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var request struct {
			Symbol string `json:"symbol"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": fmt.Sprintf("Signal for %s rejected", request.Symbol),
		})
	}))

	// Lopez de Prado Algorithms API
	http.HandleFunc("/api/algorithms/metadata", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Define metadata for all Lopez de Prado algorithms
		metadata := []map[string]interface{}{
			{
				"type":        "fractional_diff",
				"name":        "Fractional Differentiation",
				"description": "Makes time series stationary while preserving memory",
				"parameters": map[string]string{
					"d_value":     "Differencing parameter (0-1), typically 0.3-0.5",
					"threshold":   "Threshold for FFD method weight truncation",
					"window_size": "Window size for fixed-width window method",
				},
				"defaults": map[string]interface{}{
					"d_value":     0.4,
					"threshold":   1e-4,
					"window_size": 20,
				},
			},
			{
				"type":        "triple_barrier",
				"name":        "Triple Barrier Method",
				"description": "Implements realistic profit-taking, stop-loss, and time exit conditions",
				"parameters": map[string]string{
					"profit_taking":       "Multiple of volatility for profit target",
					"stop_loss":           "Multiple of volatility for stop loss",
					"time_horizon":        "Days for time barrier",
					"volatility_lookback": "Lookback window for volatility estimation",
				},
				"defaults": map[string]interface{}{
					"profit_taking":       2.0,
					"stop_loss":           1.0,
					"time_horizon":        5,
					"volatility_lookback": 20,
				},
			},
			{
				"type":        "meta_labeling",
				"name":        "Meta-Labeling",
				"description": "Secondary model to filter primary trading signals",
				"parameters": map[string]string{
					"confidence_threshold":    "Minimum confidence to execute trade",
					"use_price_features":      "Whether to use price-based features",
					"use_volume_features":     "Whether to use volume-based features",
					"use_volatility_features": "Whether to use volatility-based features",
					"use_technical_features":  "Whether to use technical indicators",
				},
				"defaults": map[string]interface{}{
					"confidence_threshold":    0.6,
					"use_price_features":      1.0,
					"use_volume_features":     1.0,
					"use_volatility_features": 1.0,
					"use_technical_features":  1.0,
				},
			},
			{
				"type":        "purged_cv",
				"name":        "Purged Cross-Validation",
				"description": "Prevents information leakage in model validation",
				"parameters": map[string]string{
					"num_folds":   "Number of folds for cross-validation",
					"embargo_pct": "Percentage of observations to embargo",
					"test_size":   "Size of test set as percentage of total",
				},
				"defaults": map[string]interface{}{
					"num_folds":   5,
					"embargo_pct": 0.01,
					"test_size":   0.3,
				},
			},
			{
				"type":        "position_sizing",
				"name":        "Advanced Position Sizing",
				"description": "Dynamic allocation based on confidence and volatility",
				"parameters": map[string]string{
					"max_size":           "Maximum position size as percentage of capital",
					"risk_fraction":      "Kelly fraction (typically 0.2-0.5)",
					"use_vol_adjustment": "Whether to adjust for volatility",
					"vol_lookback":       "Volatility lookback period",
					"max_drawdown":       "Maximum acceptable drawdown",
				},
				"defaults": map[string]interface{}{
					"max_size":           0.2,
					"risk_fraction":      0.3,
					"use_vol_adjustment": 1.0,
					"vol_lookback":       20,
					"max_drawdown":       0.1,
				},
			},
			{
				"type":        "sequential_bootstrap",
				"name":        "Sequential Bootstrap",
				"description": "Creates unbiased training sets by addressing overlapping returns",
				"parameters": map[string]string{
					"sample_pct":        "Percentage of data to include in each bootstrap",
					"num_bootstraps":    "Number of bootstrap samples to create",
					"sequential_blocks": "Whether to use sequential blocks (true) or random sampling (false)",
				},
				"defaults": map[string]interface{}{
					"sample_pct":        0.5,
					"num_bootstraps":    100,
					"sequential_blocks": true,
				},
			},
		}

		// Return metadata
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(metadata)
	}))

	// Configure an algorithm
	http.HandleFunc("/api/algorithms/configure", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse request body
		var req struct {
			Type       string                 `json:"type"`
			Parameters map[string]interface{} `json:"parameters"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			log.Printf("Error decoding algorithm configuration request: %v", err)
			return
		}

		// Create algorithm config
		params := make(map[string]float64)
		for k, v := range req.Parameters {
			// Convert to float64
			switch val := v.(type) {
			case float64:
				params[k] = val
			case int:
				params[k] = float64(val)
			case string:
				// Try to parse as float
				floatVal, err := strconv.ParseFloat(val, 64)
				if err != nil {
					http.Error(w, fmt.Sprintf("Invalid parameter value for %s: %v", k, v), http.StatusBadRequest)
					return
				}
				params[k] = floatVal
			default:
				http.Error(w, fmt.Sprintf("Invalid parameter type for %s: %T", k, v), http.StatusBadRequest)
				return
			}
		}

		// Create algorithm
		algType := algo.AlgorithmType(req.Type)
		algorithm, err := algo.Create(algType)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to create algorithm: %v", err), http.StatusBadRequest)
			log.Printf("Error creating algorithm: %v", err)
			return
		}

		// Configure algorithm
		config := algo.AlgorithmConfig{
			AdditionalParams: params,
		}
		if err := algorithm.Configure(config); err != nil {
			http.Error(w, fmt.Sprintf("Failed to configure algorithm: %v", err), http.StatusBadRequest)
			log.Printf("Error configuring algorithm: %v", err)
			return
		}

		// Register the algorithm for future use
		algoRegistry[req.Type] = algorithm

		// Return success
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "success",
			"message": fmt.Sprintf("Algorithm %s configured successfully", req.Type),
			"type":    req.Type,
		})
	}))

	// Execute an algorithm
	http.HandleFunc("/api/algorithms/execute", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse request body
		var req struct {
			Type   string `json:"type"`
			Symbol string `json:"symbol"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			log.Printf("Error decoding algorithm execution request: %v", err)
			return
		}

		// Get the algorithm
		algorithm, exists := algoRegistry[req.Type]
		if !exists {
			http.Error(w, fmt.Sprintf("Algorithm of type %s not found. Configure it first.", req.Type), http.StatusBadRequest)
			return
		}

		// Get current market data
		marketData := tradingAlgo.GetMarketData(req.Symbol)
		if marketData.Price == 0 {
			http.Error(w, fmt.Sprintf("No market data available for symbol %s", req.Symbol), http.StatusBadRequest)
			return
		}

		// Get historical data
		request := types.HistoricalDataRequest{
			Symbol:    req.Symbol,                     // Symbol to get data for
			StartDate: time.Now().AddDate(0, 0, -30),  // Last 30 days
			EndDate:   time.Now(),                     // Current time
			TimeFrame: "1D",                           // Daily timeframe
		}

		// Use the correctly imported algorithm package and function
		historicalData, err := tradingAlgo.GetHistoricalDataV2(request)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get historical data: %v", err), http.StatusInternalServerError)
			log.Printf("Error getting historical data: %v", err)
			return
		}

		// Convert market data to types.MarketData
		typesMarketData := &types.MarketData{
			Symbol:    marketData.Symbol,
			Price:     marketData.Price,
			High24h:   marketData.High24h,
			Low24h:    marketData.Low24h,
			Volume24h: marketData.Volume24h,
			Change24h: marketData.Change24h,
		}

		// Execute the algorithm
		var result *algo.AlgorithmResult
		var algErr error

		// Type assertion to the correct algorithm type
		switch a := algorithm.(type) {
		case *algo.FractionalDiffAlgorithm:
			result, algErr = a.Process(req.Symbol, typesMarketData, convertHistoricalDataToMarketData(historicalData))
		case *algo.TripleBarrierAlgorithm:
			result, algErr = a.Process(req.Symbol, typesMarketData, convertHistoricalDataToMarketData(historicalData))
		case *algo.MetaLabelingAlgorithm:
			result, algErr = a.Process(req.Symbol, typesMarketData, convertHistoricalDataToMarketData(historicalData))
		case *algo.PurgedCVAlgorithm:
			result, algErr = a.Process(req.Symbol, typesMarketData, convertHistoricalDataToMarketData(historicalData))
		case *algo.PositionSizingAlgorithm:
			result, algErr = a.Process(req.Symbol, typesMarketData, convertHistoricalDataToMarketData(historicalData))
		case *algo.SequentialBootstrapAlgorithm:
			result, algErr = a.Process(req.Symbol, typesMarketData, convertHistoricalDataToMarketData(historicalData))
		default:
			http.Error(w, fmt.Sprintf("Unsupported algorithm type: %T", a), http.StatusBadRequest)
			return
		}

		if algErr != nil {
			http.Error(w, fmt.Sprintf("Failed to execute algorithm: %v", algErr), http.StatusInternalServerError)
			log.Printf("Error executing algorithm: %v", algErr)
			return
		}

		// Return the result
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":      "success",
			"type":        req.Type,
			"symbol":      req.Symbol,
			"signal":      result.Signal,
			"order_type":  result.OrderType,
			"confidence":  result.Confidence,
			"explanation": result.Explanation,
		})
	}))

	// Toggle manual control setting
	http.HandleFunc("/api/settings/manual-control", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var request struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Update application settings - in a real app, this would update a settings store
		log.Printf("Setting manual trading control to: %v", request.Enabled)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": fmt.Sprintf("Manual control set to %v", request.Enabled),
			"enabled": request.Enabled,
		})
	}))

	// Register notification routes
	notificationHandler.RegisterRoutes(http.DefaultServeMux)

	// Static File Server - Must be last to avoid conflicts with API routes
	fs := http.FileServer(http.Dir("."))
	http.Handle("/", fs)
}

// executeBuyOrder executes a buy order using the Alpaca API
func executeBuyOrder(client *alpaca.Client, signal *algorithm.TradeSignal, apiKey, apiSecret string) (string, error) {
	log.Printf("Starting executeBuyOrder for symbol: %s", signal.Symbol)
	// Create order request
	// Initialize order request with only required fields to avoid potential API issues
	orderRequest := alpaca.PlaceOrderRequest{}

	// Set basic order properties
	orderRequest.Symbol = signal.Symbol
	orderRequest.Side = alpaca.Side("buy")
	orderRequest.Type = alpaca.OrderType(strings.ToLower(signal.OrderType))
	orderRequest.TimeInForce = alpaca.Day

	// Set PositionIntent explicitly to prevent 422 API error
	// Use correct PositionIntent value from Alpaca SDK
	log.Printf("DEBUG: Setting PositionIntent to BuyToOpen (value: %s)", string(alpaca.BuyToOpen))
	orderRequest.PositionIntent = alpaca.BuyToOpen

	// Log Alpaca client version info if available
	log.Printf("DEBUG: Alpaca API Version Info - Using SDK go/v3")

	// Qty will be set later after position sizing

	// Calculate position size (simple example - in a real system this would use risk management)
	// Get account information
	account, err := client.GetAccount()
	log.Printf("GetAccount call result: %v", err)
	if err != nil {
		return "", fmt.Errorf("failed to get account info: %w", err)
	}

	log.Printf("Account cash available: %s", account.Cash)
	// Simple position sizing: use 5% of available cash
	cashAvailable, _ := account.Cash.Float64()
	positionSize := cashAvailable * 0.05

	// Get latest quote for the symbol
	// Using marketdata client instead of direct client for quotes
	mdClient := marketdata.NewClient(marketdata.ClientOpts{
		APIKey:    apiKey,
		APISecret: apiSecret,
	})
	log.Printf("Created marketdata client for quote lookup")

	// Get the quote
	quote, err := mdClient.GetLatestQuote(signal.Symbol, marketdata.GetLatestQuoteRequest{})
	log.Printf("GetLatestQuote call result for %s: %v", signal.Symbol, err)
	if err != nil {
		return "", fmt.Errorf("failed to get quote for %s: %w", signal.Symbol, err)
	}

	// Calculate number of shares
	latestPrice := quote.BidPrice
	log.Printf("Latest price for %s: %v", signal.Symbol, latestPrice)
	if latestPrice == 0 {

		return "", fmt.Errorf("invalid price (0) for %s", signal.Symbol)
	}

	shares := positionSize / float64(latestPrice)
	// Convert to decimal format for Alpaca API
	qtyValue := fmt.Sprintf("%.0f", shares) // Round to whole shares
	qtyDecimal, _ := decimal.NewFromString(qtyValue)
	orderRequest.Qty = &qtyDecimal

	// For limit orders, set the limit price
	var priceDecimal decimal.Decimal
	marketPrice := float64(latestPrice)

	if strings.ToLower(signal.OrderType) == "limit" {
		if signal.LimitPrice == nil || *signal.LimitPrice <= 0 {
			// If no limit price provided, use current price
			priceDecimal = decimal.NewFromFloat(marketPrice)
		} else {
			proposedPrice := *signal.LimitPrice

			// Validate that the limit price is within a reasonable range of current market price
			// For buy orders, limit price should normally be at or below market price
			minReasonablePrice := marketPrice * 0.70 // 30% below market price (allowing more room for strategy-based limit orders)
			maxReasonablePrice := marketPrice * 1.05 // 5% above market price

			if proposedPrice < minReasonablePrice || proposedPrice > maxReasonablePrice {
				log.Printf("WARNING: Proposed limit price ($%.2f) for %s is outside reasonable range of market price ($%.2f)",
					proposedPrice, signal.Symbol, marketPrice)
				log.Printf("Adjusting limit price to 99%% of market price")
				proposedPrice = marketPrice * 0.99 // 1% below market price as default fallback
			}

			priceDecimal = decimal.NewFromFloat(proposedPrice)
			// Round to 2 decimal places to avoid sub-penny increments
			priceDecimal = priceDecimal.Round(2)
		}
		orderRequest.LimitPrice = &priceDecimal
	}

	// Place the order
	log.Printf("Attempting to place order: %+v", orderRequest)
	order, err := client.PlaceOrder(orderRequest)
	if err != nil {
		log.Printf("Full request details: %#v", orderRequest)
	}
	log.Printf("PlaceOrder call result: %v", err)

	if err != nil {
		log.Printf("Error details: %#v", err)
		return "", fmt.Errorf("failed to place buy order: %w", err)
	}
	log.Printf("Order placed successfully: %+v", order)

	return fmt.Sprintf("Buy order placed for %s shares of %s at %s", orderRequest.Qty.String(), signal.Symbol, order.FilledAvgPrice), nil
}

// executeSellOrder executes a sell order using the Alpaca API
func executeSellOrder(client *alpaca.Client, signal *algorithm.TradeSignal, apiKey, apiSecret string) (string, error) {
	// Check if we have a position in this symbol
	position, err := client.GetPosition(signal.Symbol)
	if err != nil {
		// If no position, return an error
		return "", fmt.Errorf("no position found for %s: %w", signal.Symbol, err)
	}

	// Initialize order request with only required fields to avoid potential API issues
	orderRequest := alpaca.PlaceOrderRequest{}

	// Get position quantity to sell
	qtyDecimal := position.Qty // Already a decimal in the Alpaca API

	// Set basic order properties
	orderRequest.Symbol = signal.Symbol
	orderRequest.Qty = &qtyDecimal // Sell entire position
	orderRequest.Side = alpaca.Side("sell")
	orderRequest.Type = alpaca.OrderType(strings.ToLower(signal.OrderType))
	orderRequest.TimeInForce = alpaca.Day

	// Set PositionIntent explicitly to prevent 422 API error
	// Use correct PositionIntent value from Alpaca SDK
	log.Printf("DEBUG: Setting PositionIntent to SellToClose (value: %s)", string(alpaca.SellToClose))
	orderRequest.PositionIntent = alpaca.SellToClose

	// Log Alpaca client version info if available
	log.Printf("DEBUG: Alpaca API Version Info - Using SDK go/v3")

	// For limit orders, set the limit price
	if strings.ToLower(signal.OrderType) == "limit" {
		if signal.LimitPrice == nil || *signal.LimitPrice <= 0 {
			// Using marketdata client for quote
			mdClient := marketdata.NewClient(marketdata.ClientOpts{
				APIKey:    apiKey,
				APISecret: apiSecret,
			})

			// Get latest quote
			askQuote, err := mdClient.GetLatestQuote(signal.Symbol, marketdata.GetLatestQuoteRequest{})
			if err != nil {

				return "", fmt.Errorf("failed to get quote for %s: %w", signal.Symbol, err)
			}

			marketPrice := float64(askQuote.AskPrice)
			priceDecimal := decimal.NewFromFloat(marketPrice)
			orderRequest.LimitPrice = &priceDecimal
			// Round to 2 decimal places to avoid sub-penny increments
			*orderRequest.LimitPrice = orderRequest.LimitPrice.Round(2)
		} else {
			// Using marketdata client for quote to get current price for validation
			mdClient := marketdata.NewClient(marketdata.ClientOpts{
				APIKey:    apiKey,
				APISecret: apiSecret,
			})
			askQuote, err := mdClient.GetLatestQuote(signal.Symbol, marketdata.GetLatestQuoteRequest{})
			if err == nil { // Only validate if we can get the current price
				marketPrice := float64(askQuote.AskPrice)
				proposedPrice := *signal.LimitPrice
				if proposedPrice < marketPrice*0.70 || proposedPrice > marketPrice*1.30 {
					log.Printf("WARNING: Proposed sell limit price ($%.2f) for %s is outside reasonable range of market price ($%.2f)",
						proposedPrice, signal.Symbol, marketPrice)
					log.Printf("Adjusting limit price to 101%% of market price")
					*signal.LimitPrice = marketPrice * 1.01 // 1% above market price as default fallback
				}
			}
			priceDecimal := decimal.NewFromFloat(*signal.LimitPrice)
			// Round to 2 decimal places to avoid sub-penny increments
			priceDecimal = priceDecimal.Round(2)
			orderRequest.LimitPrice = &priceDecimal
		}
	}

	// Place the order
	order, err := client.PlaceOrder(orderRequest)
	if err != nil {
		return "", fmt.Errorf("failed to place sell order: %w", err)
	}

	return fmt.Sprintf("Sell order placed for %s shares of %s at %s", qtyDecimal.String(), signal.Symbol, order.FilledAvgPrice), nil
}
