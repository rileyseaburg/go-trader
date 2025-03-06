package algo

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"github.com/rileyseaburg/go-trader/types"
)

// ClaudeAlgorithmAdapter adapts our algorithms for use with Claude
type ClaudeAlgorithmAdapter struct {
	manager *AlgorithmManager
}

// NewClaudeAlgorithmAdapter creates a new adapter for Claude
func NewClaudeAlgorithmAdapter() *ClaudeAlgorithmAdapter {
	manager := NewAlgorithmManager()
	
	// Register all available algorithms
	for _, algType := range GetRegisteredAlgorithms() {
		alg, err := Create(algType)
		if err != nil {
			// Log error but continue
			fmt.Printf("Error creating algorithm %s: %v\n", algType, err)
			continue
		}
		
		err = manager.RegisterAlgorithm(alg)
		if err != nil {
			fmt.Printf("Error registering algorithm %s: %v\n", algType, err)
		}
	}
	
	return &ClaudeAlgorithmAdapter{
		manager: manager,
	}
}

// GenerateTradeSignal generates a trade signal for a symbol using algorithmic analysis
func (c *ClaudeAlgorithmAdapter) GenerateTradeSignal(symbol string, currentData *types.MarketData, historicalData []types.MarketData) (*types.TradeSignal, error) {
	// Process with all algorithms and combine results
	result, err := c.manager.ProcessWithAllAlgorithms(symbol, currentData, historicalData)
	if err != nil {
		return nil, fmt.Errorf("error processing algorithms: %w", err)
	}
	
	// Convert to trade signal
	signal := c.manager.GetTradeSignal(symbol, result)
	return signal, nil
}

// GenerateDetailedAnalysis generates a detailed analysis of a symbol using the specified algorithms
func (c *ClaudeAlgorithmAdapter) GenerateDetailedAnalysis(symbol string, currentData *types.MarketData, historicalData []types.MarketData, algorithmTypes []AlgorithmType) (map[string]interface{}, error) {
	results := make(map[string]interface{})
	
	// Process with each specified algorithm
	for _, algType := range algorithmTypes {
		result, err := c.manager.ProcessWithAlgorithm(algType, symbol, currentData, historicalData)
		if err != nil {
			results[string(algType)] = map[string]interface{}{
				"error": err.Error(),
			}
			continue
		}
		
		// Add result to the analysis
		results[string(algType)] = map[string]interface{}{
			"signal":      result.Signal,
			"order_type":  result.OrderType,
			"limit_price": result.LimitPrice,
			"confidence":  result.Confidence,
			"explanation": result.Explanation,
		}
	}
	
	// Add combined result
	combinedResult, err := c.manager.ProcessWithAllAlgorithms(symbol, currentData, historicalData)
	if err == nil {
		results["combined"] = map[string]interface{}{
			"signal":      combinedResult.Signal,
			"order_type":  combinedResult.OrderType,
			"limit_price": combinedResult.LimitPrice,
			"confidence":  combinedResult.Confidence,
			"explanation": combinedResult.Explanation,
		}
	}
	
	return results, nil
}

// ParseClaudeResponse parses a response from Claude into a trade signal
func (c *ClaudeAlgorithmAdapter) ParseClaudeResponse(response string) (*types.TradeSignal, error) {
	// Look for JSON in the response
	jsonStart := strings.Index(response, "{")
	jsonEnd := strings.LastIndex(response, "}")
	
	if jsonStart == -1 || jsonEnd == -1 || jsonEnd <= jsonStart {
		return nil, fmt.Errorf("no valid JSON found in Claude response")
	}
	
	jsonStr := response[jsonStart : jsonEnd+1]
	
	// Parse the JSON
	var signalData struct {
		Symbol     string   `json:"symbol"`
		Signal     string   `json:"signal"`
		OrderType  string   `json:"order_type"`
		LimitPrice *float64 `json:"limit_price"`
		Reasoning  string   `json:"reasoning"`
		Confidence float64  `json:"confidence"`
	}
	
	err := json.Unmarshal([]byte(jsonStr), &signalData)
	if err != nil {
		return nil, fmt.Errorf("error parsing JSON from Claude response: %w", err)
	}
	
	// Validate signal
	if signalData.Signal != types.SignalBuy && 
	   signalData.Signal != types.SignalSell && 
	   signalData.Signal != types.SignalHold {
		return nil, fmt.Errorf("invalid signal type: %s", signalData.Signal)
	}
	
	// Create trade signal
	signal := &types.TradeSignal{
		Symbol:     signalData.Symbol,
		Signal:     signalData.Signal,
		OrderType:  signalData.OrderType,
		LimitPrice: signalData.LimitPrice,
		Timestamp:  time.Now(),
		Reasoning:  signalData.Reasoning,
		Confidence: &signalData.Confidence,
	}
	
	return signal, nil
}

// ValidateAlgorithmResponse checks if Claude's algorithm response matches our expected algorithms
func (c *ClaudeAlgorithmAdapter) ValidateAlgorithmResponse(response string) bool {
	// Check for mentions of our implemented algorithms
	algorithmMentions := []string{
		"Hierarchical Risk Parity", "HRP",
		"Mean-Variance Optimization", "MVO",
		"Entropy Pooling",
	}
	
	for _, mention := range algorithmMentions {
		if strings.Contains(response, mention) {
			return true
		}
	}
	
	// Look for statistical terms that our algorithms use
	technicalTerms := []string{
		"Sharpe ratio", "volatility", "correlation", 
		"covariance", "risk-adjusted return", "risk aversion",
		"diversification", "clustering", "portfolio optimization",
	}
	
	termCount := 0
	for _, term := range technicalTerms {
		if strings.Contains(response, term) {
			termCount++
		}
	}
	
	// If it mentions at least 3 technical terms, it's probably valid
	return termCount >= 3
}

// GetAlgorithmsDescription returns a description of all available algorithms
func (c *ClaudeAlgorithmAdapter) GetAlgorithmsDescription() string {
	algorithms := c.manager.GetAvailableAlgorithms()
	
	var description strings.Builder
	description.WriteString("Available Trading Algorithms:\n\n")
	
	for _, alg := range algorithms {
		description.WriteString(fmt.Sprintf("## %s\n", alg.Name()))
		description.WriteString(fmt.Sprintf("%s\n\n", alg.Description()))
		
		// Parameters
		description.WriteString("Parameters:\n")
		for param, desc := range alg.ParameterDescription() {
			description.WriteString(fmt.Sprintf("- %s: %s\n", param, desc))
		}
		description.WriteString("\n")
	}
	
	return description.String()
}

// ConfigureAlgorithm configures an algorithm with custom parameters
func (c *ClaudeAlgorithmAdapter) ConfigureAlgorithm(algType AlgorithmType, config AlgorithmConfig) error {
	return c.manager.ConfigureAlgorithm(algType, config)
}