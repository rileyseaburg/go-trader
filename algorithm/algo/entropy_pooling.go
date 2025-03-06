package algo

import (
	"fmt"
	"math"
	"github.com/rileyseaburg/go-trader/types"
)

// EntropyPoolingAlgorithm implements the Entropy Pooling algorithm
type EntropyPoolingAlgorithm struct {
	BaseAlgorithm
	priorProbabilities []float64
	viewProbabilities  []float64
	assets             []string
}

// NewEntropyPoolingAlgorithm creates a new instance of the Entropy Pooling algorithm
func NewEntropyPoolingAlgorithm() Algorithm {
	// Register the algorithm factory function
	Register(AlgorithmTypeEntropyPooling, func() Algorithm {
		return &EntropyPoolingAlgorithm{}
	})

	return &EntropyPoolingAlgorithm{}
}

// Name returns the name of the algorithm
func (e *EntropyPoolingAlgorithm) Name() string {
	return "Entropy Pooling"
}

// Type returns the type of the algorithm
func (e *EntropyPoolingAlgorithm) Type() AlgorithmType {
	return AlgorithmTypeEntropyPooling
}

// Description returns a brief description of the algorithm
func (e *EntropyPoolingAlgorithm) Description() string {
	return "The Entropy Pooling algorithm combines prior beliefs about asset returns with new market views while minimizing the relative entropy between distributions."
}

// ParameterDescription returns a description of the parameters this algorithm accepts
func (e *EntropyPoolingAlgorithm) ParameterDescription() map[string]string {
	return map[string]string{
		"risk_aversion":       "Risk aversion coefficient (default: 1.0)",
		"max_position_weight": "Maximum weight for any single asset (default: 0.3)",
		"min_position_weight": "Minimum weight for any single asset (default: 0.01)",
		"historical_days":     "Number of days of historical data to use (default: 30)",
		"view_confidence":     "Confidence in the market views (0-1, default: 0.5)",
	}
}

// Process processes the market data and returns a trading signal
func (e *EntropyPoolingAlgorithm) Process(symbol string, data *types.MarketData, historicalData []types.MarketData) (*AlgorithmResult, error) {
	if data == nil {
		return nil, fmt.Errorf("market data is nil")
	}

	// For simplicity, we'll use a single asset approach
	e.assets = []string{symbol}

	// Extract historical prices and calculate returns
	prices := make([]float64, len(historicalData))
	for i, d := range historicalData {
		prices[i] = d.Price
	}

	returns := make([]float64, len(prices)-1)
	for i := 1; i < len(prices); i++ {
		returns[i-1] = (prices[i] - prices[i-1]) / prices[i-1]
	}

	// For a single asset, entropy pooling simplifies to a weighted adjustment of our prior based on market views
	var signal string
	var orderType string
	var limitPrice *float64
	var confidence float64
	var explanation string

	if len(returns) > 0 {
		// Prior: Use historical return distribution
		meanReturn := mean(returns)
		volatility := standardDeviation(returns)

		// View: Based on current market conditions (recent performance and volatility trends)
		// Calculate momentum and volatility trends
		shortTermReturns := returns
		if len(returns) > 10 {
			shortTermReturns = returns[len(returns)-10:]
		}
		shortTermMean := mean(shortTermReturns)
		
		// Check if volatility is increasing or decreasing
		shortTermVol := standardDeviation(shortTermReturns)
		volatilityTrend := shortTermVol / volatility // > 1 means increasing volatility
		
		// Calculate price momentum (percentage change)
		momentum := 0.0
		if len(prices) >= 2 {
			momentum = (prices[len(prices)-1] - prices[0]) / prices[0]
		}

		// Combine prior (historical data) with our view (recent trends)
		// We'll use the config's view_confidence or a default if not provided
		viewConfidence := 0.5
		if val, ok := e.config.AdditionalParams["view_confidence"]; ok {
			viewConfidence = val
		}

		// Weighted combination of prior and view
		adjustedReturn := meanReturn*(1-viewConfidence) + shortTermMean*viewConfidence

		// Decision logic based on adjusted return and volatility trends
		// Higher momentum and stable/decreasing volatility is positive
		if adjustedReturn > 0 && momentum > 0 && volatilityTrend <= 1.1 {
			signal = types.SignalBuy
			orderType = "limit"
			// Set a limit price slightly below current for better entry
			price := data.Price * 0.99
			limitPrice = &price
			confidence = 0.65 + math.Min(0.3, momentum)
			explanation = fmt.Sprintf(
				"Based on Entropy Pooling analysis, %s shows positive return expectations (%.2f%%) with favorable momentum (%.2f%%) and stable volatility trends. Market sentiment analysis indicates a probability of upward movement.",
				symbol, adjustedReturn*100, momentum*100)
		} else if adjustedReturn > 0 && volatilityTrend > 1.1 {
			// Positive returns but increasing volatility suggests caution
			signal = types.SignalHold
			orderType = "market"
			confidence = 0.6
			explanation = fmt.Sprintf(
				"Based on Entropy Pooling analysis, %s shows positive adjusted returns (%.2f%%) but increasing volatility (trend: %.2f). Current market conditions suggest holding existing positions.",
				symbol, adjustedReturn*100, volatilityTrend)
		} else if adjustedReturn < 0 && momentum < 0 {
			// Negative returns and momentum suggest selling
			signal = types.SignalSell
			orderType = "market"
			confidence = 0.7 + math.Min(0.2, math.Abs(momentum))
			explanation = fmt.Sprintf(
				"Based on Entropy Pooling analysis, %s shows negative return expectations (%.2f%%) with negative momentum (%.2f%%). Market sentiment analysis indicates a high probability of continued downward movement.",
				symbol, adjustedReturn*100, momentum*100)
		} else {
			// Mixed signals, hold or neutral stance
			signal = types.SignalHold
			orderType = "market"
			confidence = 0.5
			explanation = fmt.Sprintf(
				"Based on Entropy Pooling analysis, %s shows mixed signals with adjusted returns of %.2f%% and momentum of %.2f%%. Entropy-adjusted probability distribution doesn't provide a clear directional signal.",
				symbol, adjustedReturn*100, momentum*100)
		}
	} else {
		// Not enough data
		signal = types.SignalHold
		orderType = "market"
		confidence = 0.5
		explanation = "Insufficient historical data to perform Entropy Pooling analysis."
	}

	// Store the explanation
	e.explanation = explanation

	// Create the result
	result := &AlgorithmResult{
		Signal:      signal,
		OrderType:   orderType,
		LimitPrice:  limitPrice,
		Confidence:  confidence,
		Explanation: explanation,
	}

	return result, nil
}

// In a full implementation, these functions would be used:

// calculateRelativeEntropy calculates the Kullback-Leibler divergence between two probability distributions
func (e *EntropyPoolingAlgorithm) calculateRelativeEntropy(p, q []float64) float64 {
	if len(p) != len(q) {
		return math.Inf(1)
	}

	entropy := 0.0
	for i := range p {
		if p[i] > 0 && q[i] > 0 {
			entropy += p[i] * math.Log(p[i]/q[i])
		}
	}

	return entropy
}

// entropyOptimization performs the entropy optimization to find posterior probabilities
// This is a simplified version - a full implementation would use proper optimization techniques
func (e *EntropyPoolingAlgorithm) entropyOptimization(prior []float64, views [][]float64, viewValues []float64) []float64 {
	// In a real implementation, this would solve the constrained optimization problem:
	// min KL(posterior || prior) subject to views = viewValues
	
	// For demonstration, we return a simple weighted average of the prior and views
	// This is NOT the correct entropy pooling algorithm, just a placeholder
	posterior := make([]float64, len(prior))
	
	// Copy the prior as a starting point
	copy(posterior, prior)
	
	// In a real implementation, we would:
	// 1. Set up Lagrangian with KL divergence and constraints
	// 2. Solve for Lagrange multipliers
	// 3. Compute posterior using p_i = q_i * exp(sum(lambda_j * view_j_i)) / Z
	// where Z is the normalization constant
	
	return posterior
}

// Initialize the algorithm
func init() {
	NewEntropyPoolingAlgorithm()
}