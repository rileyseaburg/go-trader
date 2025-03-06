package algorithm

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/rileyseaburg/go-trader/algorithm/algo"
	"github.com/rileyseaburg/go-trader/types"
)

// AlgorithmExecutionRequest represents a request to execute an algorithm
type AlgorithmExecutionRequest struct {
	AlgorithmType algo.AlgorithmType   `json:"algorithm_type"`
	Symbol        string               `json:"symbol"`
	Config        algo.AlgorithmConfig `json:"config"`
}

// AlgorithmExecutionResponse represents the response from algorithm execution
type AlgorithmExecutionResponse struct {
	Success bool                  `json:"success"`
	Result  *algo.AlgorithmResult `json:"result,omitempty"`
	Error   string                `json:"error,omitempty"`
}

// AlgorithmMetadata represents metadata about an algorithm
type AlgorithmMetadata struct {
	Name                  string            `json:"name"`
	Description           string            `json:"description"`
	ParameterDescriptions map[string]string `json:"parameter_descriptions"`
}

// HandleAlgorithmExecution handles requests to execute an algorithm
func HandleAlgorithmExecution(w http.ResponseWriter, r *http.Request) {
	// Only allow POST method
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req AlgorithmExecutionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request format: %v", err), http.StatusBadRequest)
		return
	}

	// Validate algorithm type
	algorithm, err := algo.Create(req.AlgorithmType)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid algorithm type: %v", err), http.StatusBadRequest)
		return
	}

	// Configure the algorithm
	if err := algorithm.Configure(req.Config); err != nil {
		http.Error(w, fmt.Sprintf("Error configuring algorithm: %v", err), http.StatusBadRequest)
		return
	}

	// Get market data for the symbol
	marketData, historicalData, err := getMarketDataForSymbol(req.Symbol)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error getting market data: %v", err), http.StatusInternalServerError)
		return
	}

	// Process the data with the algorithm
	result, err := algorithm.Process(req.Symbol, marketData, historicalData)
	if err != nil {
		log.Printf("Error processing algorithm: %v", err)

		response := AlgorithmExecutionResponse{
			Success: false,
			Error:   fmt.Sprintf("Error processing algorithm: %v", err),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Return the result
	response := AlgorithmExecutionResponse{
		Success: true,
		Result:  result,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleAlgorithmMetadata handles requests for algorithm metadata
func HandleAlgorithmMetadata(w http.ResponseWriter, r *http.Request) {
	// Only allow GET method
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if a specific algorithm was requested
	path := r.URL.Path
	parts := strings.Split(path, "/")

	// If path has format /api/algorithm/metadata/{algo_type}
	if len(parts) > 4 && parts[3] == "metadata" {
		algorithmType := algo.AlgorithmType(parts[4])

		// Get metadata for specific algorithm
		metadata, err := getAlgorithmMetadata(algorithmType)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error: %v", err), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(metadata)
		return
	}

	// Otherwise return metadata for all algorithms
	allMetadata := make(map[algo.AlgorithmType]AlgorithmMetadata)

	for _, algType := range algo.GetRegisteredAlgorithms() {
		metadata, err := getAlgorithmMetadata(algType)
		if err != nil {
			log.Printf("Error getting metadata for %s: %v", algType, err)
			continue
		}

		allMetadata[algType] = metadata
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(allMetadata)
}

// getAlgorithmMetadata retrieves metadata for a specific algorithm
func getAlgorithmMetadata(algorithmType algo.AlgorithmType) (AlgorithmMetadata, error) {
	algorithm, err := algo.Create(algorithmType)
	if err != nil {
		return AlgorithmMetadata{}, fmt.Errorf("invalid algorithm type: %v", err)
	}

	return AlgorithmMetadata{
		Name:                  algorithm.Name(),
		Description:           algorithm.Description(),
		ParameterDescriptions: algorithm.ParameterDescription(),
	}, nil
}

// getMarketDataForSymbol retrieves current and historical market data for a symbol
// This is a placeholder - in a real implementation, this would fetch data from a market data provider
func getMarketDataForSymbol(symbol string) (*types.MarketData, []types.MarketData, error) {
	// This is a simplified implementation that returns dummy data
	// In a real system, you would fetch this data from your market data source

	// Current market data
	current := &types.MarketData{
		Symbol:    symbol,
		Price:     100.0,   // Dummy price
		High24h:   105.0,   // Dummy high
		Low24h:    95.0,    // Dummy low
		Volume24h: 1000000, // Dummy volume
		Change24h: 1.5,     // Dummy change percentage
	}

	// Historical market data (last 30 days)
	historical := make([]types.MarketData, 30)
	basePrice := 100.0

	for i := 0; i < 30; i++ {
		// Generate some variation in price
		priceFactor := 0.95 + float64(i%10)/100.0
		price := basePrice * priceFactor

		historical[29-i] = types.MarketData{
			Symbol:    symbol,
			Price:     price,
			High24h:   price * 1.05,
			Low24h:    price * 0.95,
			Volume24h: 900000 + float64(i*1000),
			Change24h: -1.0 + float64(i%5),
		}
	}

	return current, historical, nil
}
