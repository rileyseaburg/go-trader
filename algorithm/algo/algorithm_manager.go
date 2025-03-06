package algo

import (
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/rileyseaburg/go-trader/types"
)

// AlgorithmManager manages the trading algorithms and their integration with Claude
type AlgorithmManager struct {
	algorithms    map[AlgorithmType]Algorithm
	configs       map[AlgorithmType]AlgorithmConfig
	defaultConfig AlgorithmConfig
	mu            sync.RWMutex
}

// NewAlgorithmManager creates a new algorithm manager
func NewAlgorithmManager() *AlgorithmManager {
	// Create default configuration
	defaultConfig := AlgorithmConfig{
		RiskAversion:      2.0,
		MaxPositionWeight: 0.3,
		MinPositionWeight: 0.01,
		TargetReturn:      0.1, // 10% target annual return
		HistoricalDays:    30,
		AdditionalParams: map[string]float64{
			"view_confidence": 0.5,
			"min_sharpe":      0.5,
		},
	}

	return &AlgorithmManager{
		algorithms:    make(map[AlgorithmType]Algorithm),
		configs:       make(map[AlgorithmType]AlgorithmConfig),
		defaultConfig: defaultConfig,
	}
}

// RegisterAlgorithm registers an algorithm with the manager
func (am *AlgorithmManager) RegisterAlgorithm(alg Algorithm) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	algType := alg.Type()

	// Check if already registered
	if _, exists := am.algorithms[algType]; exists {
		return fmt.Errorf("algorithm type %s already registered", algType)
	}

	// Register the algorithm
	am.algorithms[algType] = alg

	// Set default config
	am.configs[algType] = am.defaultConfig

	// Configure the algorithm
	if err := alg.Configure(am.defaultConfig); err != nil {
		delete(am.algorithms, algType)
		delete(am.configs, algType)
		return fmt.Errorf("failed to configure algorithm %s: %w", alg.Name(), err)
	}

	log.Printf("Registered algorithm: %s (%s)", alg.Name(), algType)
	return nil
}

// GetAlgorithm returns an algorithm by type
func (am *AlgorithmManager) GetAlgorithm(algType AlgorithmType) (Algorithm, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	alg, exists := am.algorithms[algType]
	if !exists {
		return nil, fmt.Errorf("algorithm type %s not registered", algType)
	}

	return alg, nil
}

// GetAvailableAlgorithms returns all registered algorithms
func (am *AlgorithmManager) GetAvailableAlgorithms() []Algorithm {
	am.mu.RLock()
	defer am.mu.RUnlock()

	algorithms := make([]Algorithm, 0, len(am.algorithms))
	for _, alg := range am.algorithms {
		algorithms = append(algorithms, alg)
	}

	return algorithms
}

// ConfigureAlgorithm configures an algorithm with custom parameters
func (am *AlgorithmManager) ConfigureAlgorithm(algType AlgorithmType, config AlgorithmConfig) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	alg, exists := am.algorithms[algType]
	if !exists {
		return fmt.Errorf("algorithm type %s not registered", algType)
	}

	// Store the configuration
	am.configs[algType] = config

	// Configure the algorithm
	return alg.Configure(config)
}

// GetAlgorithmConfig returns the current configuration for an algorithm
func (am *AlgorithmManager) GetAlgorithmConfig(algType AlgorithmType) (AlgorithmConfig, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	config, exists := am.configs[algType]
	if !exists {
		return AlgorithmConfig{}, fmt.Errorf("algorithm type %s not registered", algType)
	}

	return config, nil
}

// ProcessWithAlgorithm processes market data with a specific algorithm
func (am *AlgorithmManager) ProcessWithAlgorithm(algType AlgorithmType, symbol string, data *types.MarketData, historicalData []types.MarketData) (*AlgorithmResult, error) {
	am.mu.RLock()
	alg, exists := am.algorithms[algType]
	am.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("algorithm type %s not registered", algType)
	}

	// Process the data with the algorithm
	return alg.Process(symbol, data, historicalData)
}

// ProcessWithAllAlgorithms processes market data with all registered algorithms and combines the results
func (am *AlgorithmManager) ProcessWithAllAlgorithms(symbol string, data *types.MarketData, historicalData []types.MarketData) (*AlgorithmResult, error) {
	am.mu.RLock()
	algorithms := make([]Algorithm, 0, len(am.algorithms))
	for _, alg := range am.algorithms {
		algorithms = append(algorithms, alg)
	}
	am.mu.RUnlock()

	if len(algorithms) == 0 {
		return nil, fmt.Errorf("no algorithms registered")
	}

	// Process with each algorithm
	results := make([]*AlgorithmResult, 0, len(algorithms))
	for _, alg := range algorithms {
		result, err := alg.Process(symbol, data, historicalData)
		if err != nil {
			log.Printf("Warning: Algorithm %s failed to process data: %v", alg.Name(), err)
			continue
		}
		results = append(results, result)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("all algorithms failed to process data")
	}

	// Combine the results (weighted by confidence)
	return am.combineResults(symbol, results)
}

// combineResults combines multiple algorithm results into a single result
func (am *AlgorithmManager) combineResults(symbol string, results []*AlgorithmResult) (*AlgorithmResult, error) {
	if len(results) == 0 {
		return nil, fmt.Errorf("no results to combine")
	}

	if len(results) == 1 {
		return results[0], nil
	}

	// Count signals by type, weighted by confidence
	signalWeights := make(map[string]float64)
	totalBuyWeight := 0.0
	totalSellWeight := 0.0
	totalConfidence := 0.0

	for _, result := range results {
		if result.Signal == types.SignalBuy {
			totalBuyWeight += result.Confidence
		} else if result.Signal == types.SignalSell {
			totalSellWeight += result.Confidence
		}

		signalWeights[result.Signal] += result.Confidence
		totalConfidence += result.Confidence
	}

	// Normalize weights
	if totalConfidence > 0 {
		for signal := range signalWeights {
			signalWeights[signal] /= totalConfidence
		}
	}

	// Determine the combined signal (highest weighted signal)
	var combinedSignal string
	var maxWeight float64
	for signal, weight := range signalWeights {
		if weight > maxWeight {
			maxWeight = weight
			combinedSignal = signal
		}
	}

	// If buy and sell weights are close, default to hold
	if math.Abs(totalBuyWeight-totalSellWeight) < 0.2*totalConfidence &&
		(combinedSignal == types.SignalBuy || combinedSignal == types.SignalSell) {
		combinedSignal = types.SignalHold
		maxWeight = (totalBuyWeight + totalSellWeight) / 2
	}

	// Determine order type and limit price
	// For buy/sell signals, use limit orders if the confidence is high
	orderType := "market"
	var limitPrice *float64

	if combinedSignal == types.SignalBuy && maxWeight > 0.7 {
		orderType = "limit"
		// Find the average limit price from buy signals
		var totalPrice float64
		var count int
		for _, result := range results {
			if result.Signal == types.SignalBuy && result.LimitPrice != nil {
				totalPrice += *result.LimitPrice
				count++
			}
		}
		if count > 0 {
			avgPrice := totalPrice / float64(count)
			limitPrice = &avgPrice
		}
	}

	// Combine explanations
	combinedExplanation := fmt.Sprintf("Combined analysis from %d algorithms:\n", len(results))
	for _, result := range results {
		algorithmName := "Unknown" // In a real implementation, you'd get this from the algorithm
		for _, alg := range am.GetAvailableAlgorithms() {
			if alg.Explain() == result.Explanation {
				algorithmName = alg.Name()
				break
			}
		}
		combinedExplanation += fmt.Sprintf("- %s (%.0f%% confidence): %s\n",
			algorithmName, result.Confidence*100, result.Signal)
	}

	combinedExplanation += fmt.Sprintf("\nFinal recommendation: %s with %.0f%% confidence.\n",
		combinedSignal, maxWeight*100)

	if combinedSignal == types.SignalBuy {
		combinedExplanation += "The algorithms suggest a potential upside based on favorable risk-adjusted returns and market conditions."
	} else if combinedSignal == types.SignalSell {
		combinedExplanation += "The algorithms indicate negative momentum and unfavorable risk-return profile, suggesting a downside risk."
	} else {
		combinedExplanation += "The algorithms show mixed or neutral signals, suggesting maintaining current positions."
	}

	// Create the combined result
	combinedResult := &AlgorithmResult{
		Signal:      combinedSignal,
		OrderType:   orderType,
		LimitPrice:  limitPrice,
		Confidence:  maxWeight,
		Explanation: combinedExplanation,
	}

	return combinedResult, nil
}

// GetTradeSignal converts an AlgorithmResult to a TradeSignal
func (am *AlgorithmManager) GetTradeSignal(symbol string, result *AlgorithmResult) *types.TradeSignal {
	if result == nil {
		return nil
	}

	// Create trade signal
	now := time.Now()
	signal := &types.TradeSignal{
		Symbol:     symbol,
		Signal:     result.Signal,
		OrderType:  result.OrderType,
		LimitPrice: result.LimitPrice,
		Timestamp:  now,
		Reasoning:  result.Explanation,
		Confidence: &result.Confidence,
	}

	return signal
}

// Init initializes algorithms
func init() {
	// The algorithm factories are initialized in their respective files
	// This ensures that all algorithms are registered when this package is imported
}
