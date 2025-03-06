package algorithm

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
)

// AlgorithmHandler implements HTTP handlers for algorithm API endpoints
type AlgorithmHandler struct {
	algorithm *TradingAlgorithm
}

// NewAlgorithmHandler creates a new algorithm handler
func NewAlgorithmHandler(algorithm *TradingAlgorithm) *AlgorithmHandler {
	return &AlgorithmHandler{
		algorithm: algorithm,
	}
}

// RegisterRoutes registers algorithm routes with the provided HTTP mux
func (h *AlgorithmHandler) RegisterRoutes(mux *http.ServeMux) {
	// GET /api/algorithm/status - Get algorithm status
	mux.HandleFunc("/api/algorithm/status", h.handleGetStatus)

	// POST /api/algorithm/start - Start algorithm
	mux.HandleFunc("/api/algorithm/start", h.handleStartAlgorithm)

	// POST /api/algorithm/stop - Stop algorithm
	mux.HandleFunc("/api/algorithm/stop", h.handleStopAlgorithm)

	// GET /api/algorithm/historical - Get historical data
	mux.HandleFunc("/api/algorithm/historical", h.handleGetHistoricalData)

	// GET /api/algorithm/analyze - Analyze historical data
	mux.HandleFunc("/api/algorithm/analyze", h.handleAnalyzeData)

	// GET /api/algorithm/recommendations - Get ticker recommendations
	mux.HandleFunc("/api/algorithm/recommendations", h.handleGetRecommendations)
}

// handleGetStatus handles GET requests to /api/algorithm/status
func (h *AlgorithmHandler) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	// Set common headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Handle OPTIONS for CORS
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get algorithm status
	status := h.algorithm.GetStatus()

	// Return status
	if err := json.NewEncoder(w).Encode(status); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		log.Printf("Error encoding algorithm status: %v", err)
		return
	}
}

// handleStartAlgorithm handles POST requests to /api/algorithm/start
func (h *AlgorithmHandler) handleStartAlgorithm(w http.ResponseWriter, r *http.Request) {
	// Set common headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Handle OPTIONS for CORS
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req struct {
		Symbols []string `json:"symbols"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		log.Printf("Error decoding start algorithm request: %v", err)
		return
	}

	// Start algorithm
	if err := h.algorithm.Start(req.Symbols); err != nil {
		http.Error(w, fmt.Sprintf("Failed to start algorithm: %v", err), http.StatusInternalServerError)
		log.Printf("Error starting algorithm: %v", err)
		return
	}

	// Return success
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Algorithm started successfully",
		"symbols": req.Symbols,
	}); err != nil {
		log.Printf("Error encoding success response: %v", err)
	}
}

// handleStopAlgorithm handles POST requests to /api/algorithm/stop
func (h *AlgorithmHandler) handleStopAlgorithm(w http.ResponseWriter, r *http.Request) {
	// Set common headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Handle OPTIONS for CORS
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Stop algorithm
	if err := h.algorithm.Stop(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to stop algorithm: %v", err), http.StatusInternalServerError)
		log.Printf("Error stopping algorithm: %v", err)
		return
	}

	// Return success
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Algorithm stopped successfully",
	}); err != nil {
		log.Printf("Error encoding success response: %v", err)
	}
}

// handleGetHistoricalData handles GET requests to /api/algorithm/historical
func (h *AlgorithmHandler) handleGetHistoricalData(w http.ResponseWriter, r *http.Request) {
	// Set common headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Handle OPTIONS for CORS
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		http.Error(w, "Symbol parameter is required", http.StatusBadRequest)
		return
	}

	timeFrame := r.URL.Query().Get("timeframe")
	if timeFrame == "" {
		timeFrame = "1D" // Default to daily
	}

	// Parse start and end dates
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")

	var start, end time.Time
	var err error

	if startStr != "" {
		start, err = time.Parse(time.RFC3339, startStr)
		if err != nil {
			http.Error(w, "Invalid start date format. Use RFC3339 format.", http.StatusBadRequest)
			return
		}
	} else {
		// Default to 30 days ago
		start = time.Now().AddDate(0, 0, -30)
	}

	if endStr != "" {
		end, err = time.Parse(time.RFC3339, endStr)
		if err != nil {
			http.Error(w, "Invalid end date format. Use RFC3339 format.", http.StatusBadRequest)
			return
		}
	} else {
		// Default to now
		end = time.Now()
	}

	// Create historical data request
	request := HistoricalDataRequest{
		Symbol:    symbol,
		StartDate: start,
		EndDate:   end,
		TimeFrame: timeFrame,
	}

	// Get historical data
	data, err := h.algorithm.GetHistoricalDataV2(request)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get historical data: %v", err), http.StatusInternalServerError)
		log.Printf("Error getting historical data: %v", err)
		return
	}

	// Return historical data
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		log.Printf("Error encoding historical data: %v", err)
		return
	}
}

// handleAnalyzeData handles GET requests to /api/algorithm/analyze
func (h *AlgorithmHandler) handleAnalyzeData(w http.ResponseWriter, r *http.Request) {
	// Set common headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Handle OPTIONS for CORS
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters (same as historical data endpoint)
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		http.Error(w, "Symbol parameter is required", http.StatusBadRequest)
		return
	}

	timeFrame := r.URL.Query().Get("timeframe")
	if timeFrame == "" {
		timeFrame = "1D" // Default to daily
	}

	// Parse start and end dates
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")

	var start, end time.Time
	var err error

	if startStr != "" {
		start, err = time.Parse(time.RFC3339, startStr)
		if err != nil {
			http.Error(w, "Invalid start date format. Use RFC3339 format.", http.StatusBadRequest)
			return
		}
	} else {
		// Default to 30 days ago
		start = time.Now().AddDate(0, 0, -30)
	}

	if endStr != "" {
		end, err = time.Parse(time.RFC3339, endStr)
		if err != nil {
			http.Error(w, "Invalid end date format. Use RFC3339 format.", http.StatusBadRequest)
			return
		}
	} else {
		// Default to now
		end = time.Now()
	}

	// First, get historical data
	request := HistoricalDataRequest{
		Symbol:    symbol,
		StartDate: start,
		EndDate:   end,
		TimeFrame: timeFrame,
	}

	data, err := h.algorithm.GetHistoricalDataV2(request)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get historical data: %v", err), http.StatusInternalServerError)
		log.Printf("Error getting historical data: %v", err)
		return
	}

	// Then, analyze the data
	analysis := h.algorithm.AnalyzeHistoricalDataV2(data)

	// Return analysis
	if err := json.NewEncoder(w).Encode(analysis); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		log.Printf("Error encoding analysis: %v", err)
		return
	}
}

// handleGetRecommendations handles GET requests to /api/algorithm/recommendations
func (h *AlgorithmHandler) handleGetRecommendations(w http.ResponseWriter, r *http.Request) {
	// Set common headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Handle OPTIONS for CORS
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters
	sector := r.URL.Query().Get("sector")
	maxResultsStr := r.URL.Query().Get("max_results")

	maxResults := 10 // Default max results
	if maxResultsStr != "" {
		maxResultsParsed, err := strconv.Atoi(maxResultsStr)
		if err == nil && maxResultsParsed > 0 {
			maxResults = maxResultsParsed
		}
	}

	// Get recommendations
	recommendations, err := h.algorithm.RecommendTickersV2(sector, maxResults)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get recommendations: %v", err), http.StatusInternalServerError)
		log.Printf("Error getting recommendations: %v", err)
		return
	}

	// Return recommendations
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"sector":          sector,
		"recommendations": recommendations,
	}); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		log.Printf("Error encoding recommendations: %v", err)
		return
	}
}