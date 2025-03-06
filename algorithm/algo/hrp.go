package algo

import (
	"fmt"
	"math"
	"sort"

	"github.com/rileyseaburg/go-trader/types"
)

// HRPAlgorithm implements the Hierarchical Risk Parity algorithm
type HRPAlgorithm struct {
	BaseAlgorithm
	correlationMatrix [][]float64
	distanceMatrix    [][]float64
	assetOrder        []int
	assetWeights      []float64
	assets            []string
}

// NewHRPAlgorithm creates a new instance of the HRP algorithm
func NewHRPAlgorithm() Algorithm {
	// Register the algorithm factory function
	Register(AlgorithmTypeHRP, func() Algorithm {
		return &HRPAlgorithm{}
	})

	return &HRPAlgorithm{}
}

// Name returns the name of the algorithm
func (h *HRPAlgorithm) Name() string {
	return "Hierarchical Risk Parity"
}

// Type returns the type of the algorithm
func (h *HRPAlgorithm) Type() AlgorithmType {
	return AlgorithmTypeHRP
}

// Description returns a brief description of the algorithm
func (h *HRPAlgorithm) Description() string {
	return "The Hierarchical Risk Parity (HRP) algorithm creates diversified portfolios by using hierarchical clustering to identify similar assets and allocate weights using an inverse-variance approach."
}

// ParameterDescription returns a description of the parameters this algorithm accepts
func (h *HRPAlgorithm) ParameterDescription() map[string]string {
	return map[string]string{
		"risk_aversion":       "Risk aversion coefficient (default: 1.0)",
		"max_position_weight": "Maximum weight for any single asset (default: 0.3)",
		"min_position_weight": "Minimum weight for any single asset (default: 0.01)",
		"historical_days":     "Number of days of historical data to use (default: 30)",
	}
}

// Process processes the market data and returns a trading signal
func (h *HRPAlgorithm) Process(symbol string, data *types.MarketData, historicalData []types.MarketData) (*AlgorithmResult, error) {
	if data == nil {
		return nil, fmt.Errorf("market data is nil")
	}

	// In a real implementation, we would use multiple assets
	// For this simplified version, we'll just use the single asset and make a decision based on recent trends
	h.assets = []string{symbol}

	// Extract historical prices
	prices := make([]float64, len(historicalData))
	for i, d := range historicalData {
		prices[i] = d.Price
	}

	// Calculate returns
	returns := make([]float64, len(prices)-1)
	for i := 1; i < len(prices); i++ {
		returns[i-1] = (prices[i] - prices[i-1]) / prices[i-1]
	}

	// For a single asset, we don't need clustering
	// We'll determine signal based on recent trend and volatility
	var signal string
	var orderType string
	var limitPrice *float64
	var confidence float64
	var explanation string

	if len(returns) > 0 {
		// Calculate mean return and volatility
		meanReturn := mean(returns)
		volatility := standardDeviation(returns)

		// Calculate Sharpe ratio (simplified)
		sharpeRatio := meanReturn / volatility

		// Decision logic based on Sharpe ratio
		if sharpeRatio > 0.5 {
			signal = types.SignalBuy
			orderType = "limit"
			// Set limit price slightly below current price
			price := data.Price * 0.99
			limitPrice = &price
			confidence = 0.7 + math.Min(0.3, sharpeRatio/10)
			explanation = fmt.Sprintf("Based on HRP analysis, symbol %s shows strong risk-adjusted returns (Sharpe: %.2f) with moderate volatility (%.2f%%). The positive trend suggests continued upward movement.",
				symbol, sharpeRatio, volatility*100)
		} else if sharpeRatio > 0 {
			signal = types.SignalHold
			orderType = "market"
			confidence = 0.6
			explanation = fmt.Sprintf("Based on HRP analysis, symbol %s shows positive but weak risk-adjusted returns (Sharpe: %.2f). Recommend holding current positions.",
				symbol, sharpeRatio)
		} else {
			signal = types.SignalSell
			orderType = "market"
			confidence = 0.6 + math.Min(0.3, math.Abs(sharpeRatio)/5)
			explanation = fmt.Sprintf("Based on HRP analysis, symbol %s shows negative risk-adjusted returns (Sharpe: %.2f) with volatility of %.2f%%. The negative trend suggests downward movement.",
				symbol, sharpeRatio, volatility*100)
		}
	} else {
		// Not enough data
		signal = types.SignalHold
		orderType = "market"
		confidence = 0.5
		explanation = "Insufficient historical data to make a reliable prediction using HRP."
	}

	// Store the explanation
	h.explanation = explanation

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

// mean calculates the arithmetic mean of a slice of float64 values
func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sum := 0.0
	for _, v := range values {
		sum += v
	}

	return sum / float64(len(values))
}

// standardDeviation calculates the standard deviation of a slice of float64 values
func standardDeviation(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}

	m := mean(values)
	var variance float64

	for _, v := range values {
		variance += math.Pow(v-m, 2)
	}

	variance /= float64(len(values) - 1)
	return math.Sqrt(variance)
}

// In a full implementation, the following functions would be used for multiple assets:

// calculateCorrelationMatrix calculates the correlation matrix from asset returns
func (h *HRPAlgorithm) calculateCorrelationMatrix(returns [][]float64) [][]float64 {
	n := len(returns)
	correlations := make([][]float64, n)

	for i := range correlations {
		correlations[i] = make([]float64, n)
		correlations[i][i] = 1 // Diagonal is 1
	}

	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			// Calculate correlation between assets i and j
			corr := correlation(returns[i], returns[j])
			correlations[i][j] = corr
			correlations[j][i] = corr // Matrix is symmetric
		}
	}

	return correlations
}

// correlation calculates the Pearson correlation coefficient between two slices
func correlation(x, y []float64) float64 {
	if len(x) != len(y) || len(x) == 0 {
		return 0
	}

	n := float64(len(x))
	sumX, sumY := 0.0, 0.0
	sumXY := 0.0
	sumX2, sumY2 := 0.0, 0.0

	for i := 0; i < len(x); i++ {
		sumX += x[i]
		sumY += y[i]
		sumXY += x[i] * y[i]
		sumX2 += x[i] * x[i]
		sumY2 += y[i] * y[i]
	}

	numerator := n*sumXY - sumX*sumY
	denominator := math.Sqrt((n*sumX2 - sumX*sumX) * (n*sumY2 - sumY*sumY))

	if denominator == 0 {
		return 0
	}

	return numerator / denominator
}

// calculateDistanceMatrix converts a correlation matrix to a distance matrix
func (h *HRPAlgorithm) calculateDistanceMatrix(correlations [][]float64) [][]float64 {
	n := len(correlations)
	distances := make([][]float64, n)

	for i := range distances {
		distances[i] = make([]float64, n)
	}

	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			// Convert correlation to distance: sqrt(0.5 * (1 - correlation))
			distances[i][j] = math.Sqrt(0.5 * (1 - correlations[i][j]))
		}
	}

	return distances
}

// hierarchicalClustering performs hierarchical clustering on the distance matrix
// This is a simplified version - a real implementation would use a proper clustering algorithm
func (h *HRPAlgorithm) hierarchicalClustering(distances [][]float64) []int {
	n := len(distances)
	order := make([]int, n)
	for i := range order {
		order[i] = i
	}

	// Simple heuristic for ordering: sort by total distance
	distanceSum := make([]float64, n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			distanceSum[i] += distances[i][j]
		}
	}

	indices := make([]int, n)
	for i := range indices {
		indices[i] = i
	}

	sort.Slice(indices, func(i, j int) bool {
		return distanceSum[indices[i]] < distanceSum[indices[j]]
	})

	return indices
}

// allocateWeights allocates weights based on hierarchical clustering
func (h *HRPAlgorithm) allocateWeights(returns [][]float64, order []int) []float64 {
	n := len(returns)
	weights := make([]float64, n)

	// Calculate variances
	variances := make([]float64, n)
	for i := 0; i < n; i++ {
		variances[i] = standardDeviation(returns[i]) * standardDeviation(returns[i])
		if variances[i] == 0 {
			variances[i] = 1e-8 // Avoid division by zero
		}
	}

	// Inverse-variance weighting
	totalWeight := 0.0
	for i := 0; i < n; i++ {
		idx := order[i]
		weights[idx] = 1.0 / variances[idx]
		totalWeight += weights[idx]
	}

	// Normalize weights
	for i := range weights {
		weights[i] /= totalWeight
	}

	return weights
}

// Initialize the algorithm
func init() {
	NewHRPAlgorithm()
}
