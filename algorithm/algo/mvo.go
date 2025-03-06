package algo

import (
	"fmt"
	"math"

	"github.com/rileyseaburg/go-trader/types"
)

// MVOAlgorithm implements the Mean-Variance Optimization algorithm
type MVOAlgorithm struct {
	BaseAlgorithm
	expectedReturns   []float64
	covarianceMatrix  [][]float64
	optimalWeights    []float64
	assets            []string
}

// NewMVOAlgorithm creates a new instance of the MVO algorithm
func NewMVOAlgorithm() Algorithm {
	// Register the algorithm factory function
	Register(AlgorithmTypeMVO, func() Algorithm {
		return &MVOAlgorithm{}
	})

	return &MVOAlgorithm{}
}

// Name returns the name of the algorithm
func (m *MVOAlgorithm) Name() string {
	return "Mean-Variance Optimization"
}

// Type returns the type of the algorithm
func (m *MVOAlgorithm) Type() AlgorithmType {
	return AlgorithmTypeMVO
}

// Description returns a brief description of the algorithm
func (m *MVOAlgorithm) Description() string {
	return "The Mean-Variance Optimization algorithm creates optimal portfolios by finding the balance between expected return and risk, aiming to maximize the Sharpe ratio."
}

// ParameterDescription returns a description of the parameters this algorithm accepts
func (m *MVOAlgorithm) ParameterDescription() map[string]string {
	return map[string]string{
		"risk_aversion":       "Risk aversion coefficient (default: 2.0)",
		"max_position_weight": "Maximum weight for any single asset (default: 0.3)",
		"min_position_weight": "Minimum weight for any single asset (default: 0.01)",
		"historical_days":     "Number of days of historical data to use (default: 30)",
		"min_sharpe":          "Minimum Sharpe ratio for buy signal (default: 0.5)",
	}
}

// Process processes the market data and returns a trading signal
func (m *MVOAlgorithm) Process(symbol string, data *types.MarketData, historicalData []types.MarketData) (*AlgorithmResult, error) {
	if data == nil {
		return nil, fmt.Errorf("market data is nil")
	}

	// For simplicity with a single asset
	m.assets = []string{symbol}

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

	// For single asset, MVO simplifies to evaluating the risk-adjusted return
	var signal string
	var orderType string
	var limitPrice *float64
	var confidence float64
	var explanation string

	if len(returns) > 0 {
		// Calculate expected return (mean of historical returns)
		expectedReturn := mean(returns)
		
		// Calculate risk (standard deviation of returns)
		risk := standardDeviation(returns)
		
		// Calculate Sharpe ratio (we'll assume risk-free rate is 0 for simplicity)
		sharpeRatio := 0.0
		if risk > 0 {
			sharpeRatio = expectedReturn / risk
		}
		
		// Get risk aversion parameter
		riskAversion := 2.0 // default
		if val, ok := m.config.AdditionalParams["risk_aversion"]; ok {
			riskAversion = val
		}
		
		// Get minimum Sharpe ratio parameter
		minSharpe := 0.5 // default
		if val, ok := m.config.AdditionalParams["min_sharpe"]; ok {
			minSharpe = val
		}
		
		// Calculate utility (expected return - risk aversion * variance)
		utility := expectedReturn - (riskAversion * risk * risk / 2)
		
		// Decision logic based on Sharpe ratio and utility
		if sharpeRatio > minSharpe && utility > 0 {
			signal = types.SignalBuy
			orderType = "limit"
			// Set limit price slightly below current for better entry
			price := data.Price * 0.99
			limitPrice = &price
			confidence = 0.65 + math.Min(0.25, sharpeRatio/4)
			explanation = fmt.Sprintf(
				"Based on MVO analysis, %s shows a favorable risk-return profile with Sharpe ratio of %.2f and utility of %.4f. Expected return (%.2f%%) outweighs risk (%.2f%%) given risk aversion of %.1f.",
				symbol, sharpeRatio, utility, expectedReturn*100, risk*100, riskAversion)
		} else if sharpeRatio > 0 && utility > -0.001 {
			// Slightly positive or neutral utility suggests holding
			signal = types.SignalHold
			orderType = "market"
			confidence = 0.6
			explanation = fmt.Sprintf(
				"Based on MVO analysis, %s shows a moderate risk-return profile with Sharpe ratio of %.2f and utility of %.4f. Current position should be maintained.",
				symbol, sharpeRatio, utility)
		} else {
			// Negative utility or Sharpe ratio suggests selling
			signal = types.SignalSell
			orderType = "market"
			confidence = 0.7 - math.Min(0.2, sharpeRatio)
			explanation = fmt.Sprintf(
				"Based on MVO analysis, %s shows an unfavorable risk-return profile with Sharpe ratio of %.2f and utility of %.4f. Expected return (%.2f%%) does not compensate for risk (%.2f%%).",
				symbol, sharpeRatio, utility, expectedReturn*100, risk*100)
		}
	} else {
		// Not enough data
		signal = types.SignalHold
		orderType = "market"
		confidence = 0.5
		explanation = "Insufficient historical data to perform Mean-Variance Optimization analysis."
	}

	// Store the explanation
	m.explanation = explanation

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

// In a full portfolio implementation, these functions would be used:

// calculateExpectedReturns calculates the expected returns for each asset
func (m *MVOAlgorithm) calculateExpectedReturns(returns [][]float64) []float64 {
	n := len(returns)
	expectedReturns := make([]float64, n)
	
	for i := 0; i < n; i++ {
		expectedReturns[i] = mean(returns[i])
	}
	
	return expectedReturns
}

// calculateCovarianceMatrix calculates the covariance matrix for asset returns
func (m *MVOAlgorithm) calculateCovarianceMatrix(returns [][]float64) [][]float64 {
	n := len(returns)
	if n == 0 {
		return [][]float64{}
	}
	
	// Make sure all return series have the same length
	t := len(returns[0])
	for i := 1; i < n; i++ {
		if len(returns[i]) != t {
			// In practice, you'd handle this more gracefully
			return [][]float64{}
		}
	}
	
	// Calculate means
	means := make([]float64, n)
	for i := 0; i < n; i++ {
		means[i] = mean(returns[i])
	}
	
	// Initialize covariance matrix
	cov := make([][]float64, n)
	for i := range cov {
		cov[i] = make([]float64, n)
	}
	
	// Calculate covariances
	for i := 0; i < n; i++ {
		for j := i; j < n; j++ {
			// Covariance formula: E[(X-E[X])(Y-E[Y])]
			var sum float64
			for k := 0; k < t; k++ {
				sum += (returns[i][k] - means[i]) * (returns[j][k] - means[j])
			}
			
			covariance := sum / float64(t-1)
			cov[i][j] = covariance
			cov[j][i] = covariance // Matrix is symmetric
		}
	}
	
	return cov
}

// findOptimalPortfolio calculates the optimal portfolio weights
// This is a simplified version - a real implementation would use quadratic programming
func (m *MVOAlgorithm) findOptimalPortfolio(expectedReturns []float64, covMatrix [][]float64, riskAversion float64) []float64 {
	n := len(expectedReturns)
	if n == 0 || len(covMatrix) != n {
		return []float64{}
	}
	
	// In a real implementation, this would solve:
	// max w^T * μ - (λ/2) * w^T * Σ * w
	// subject to sum(w) = 1, w_i >= min_weight, w_i <= max_weight
	
	// For simplicity, we'll just return equal weights
	weights := make([]float64, n)
	for i := range weights {
		weights[i] = 1.0 / float64(n)
	}
	
	// A proper implementation would use a quadratic programming solver
	
	return weights
}

// calculatePortfolioStats calculates expected return and risk for a given portfolio
func (m *MVOAlgorithm) calculatePortfolioStats(weights []float64, expectedReturns []float64, covMatrix [][]float64) (float64, float64) {
	if len(weights) == 0 || len(weights) != len(expectedReturns) || len(covMatrix) != len(weights) {
		return 0, 0
	}
	
	// Calculate expected portfolio return
	portfolioReturn := 0.0
	for i, w := range weights {
		portfolioReturn += w * expectedReturns[i]
	}
	
	// Calculate portfolio variance
	portfolioVariance := 0.0
	for i, wi := range weights {
		for j, wj := range weights {
			portfolioVariance += wi * wj * covMatrix[i][j]
		}
	}
	
	// Return expected return and risk (standard deviation)
	return portfolioReturn, math.Sqrt(portfolioVariance)
}

// Initialize the algorithm
func init() {
	NewMVOAlgorithm()
}