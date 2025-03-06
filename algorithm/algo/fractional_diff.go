package algo

import (
	"errors"
	"fmt"
	"math"

	"github.com/rileyseaburg/go-trader/types"
)

// init registers the Fractional Differentiation algorithm with the factory
func init() {
	Register(AlgorithmTypeFractionalDiff, func() Algorithm {
		return &FractionalDiffAlgorithm{
			BaseAlgorithm: BaseAlgorithm{},
		}
	})
}

// FractionalDiffAlgorithm implements fractional differentiation techniques from
// de Prado's "Advances in Financial Machine Learning" (2018)
type FractionalDiffAlgorithm struct {
	BaseAlgorithm
	d             float64 // Differencing parameter (typically 0-1)
	threshold     float64 // Minimum weight threshold
	windowSize    int     // Fixed window size (for fixed window approach)
	useFixedWidth bool    // Whether to use fixed width window or FFD
}

// Name returns the name of the algorithm
func (f *FractionalDiffAlgorithm) Name() string {
	return "Fractional Differentiation"
}

// Type returns the type of the algorithm
func (f *FractionalDiffAlgorithm) Type() AlgorithmType {
	return AlgorithmTypeFractionalDiff
}

// Description returns a brief description of the algorithm
func (f *FractionalDiffAlgorithm) Description() string {
	return "Performs fractional differentiation to make financial time series stationary while preserving memory"
}

// ParameterDescription returns a description of the parameters
func (f *FractionalDiffAlgorithm) ParameterDescription() map[string]string {
	return map[string]string{
		"d":              "Differencing parameter, between 0 and 1 (default: 0.5)",
		"threshold":      "Minimum weight threshold for FFD method (default: 1e-5)",
		"window_size":    "Fixed window size for fixed window method (default: 10)",
		"use_fixed_width": "Whether to use fixed width window (1) or FFD (0) (default: 0)",
	}
}

// Configure configures the algorithm with the given parameters
func (f *FractionalDiffAlgorithm) Configure(config AlgorithmConfig) error {
	if err := f.BaseAlgorithm.Configure(config); err != nil {
		return err
	}

	// Set default values
	f.d = 0.5
	f.threshold = 1e-5
	f.windowSize = 10
	f.useFixedWidth = false

	// Override with provided values
	if val, ok := config.AdditionalParams["d"]; ok {
		if val < 0 || val > 1 {
			return errors.New("d must be between 0 and 1")
		}
		f.d = val
	}

	if val, ok := config.AdditionalParams["threshold"]; ok {
		if val <= 0 {
			return errors.New("threshold must be positive")
		}
		f.threshold = val
	}

	if val, ok := config.AdditionalParams["window_size"]; ok {
		if val < 1 {
			return errors.New("window_size must be at least 1")
		}
		f.windowSize = int(val)
	}

	if val, ok := config.AdditionalParams["use_fixed_width"]; ok {
		f.useFixedWidth = val > 0.5
	}

	return nil
}

// Process processes the market data and returns a processed dataset
// This method implements the Algorithm interface but since Fractional Differentiation
// is mainly a data transformation technique, it returns a modified version of the input
func (f *FractionalDiffAlgorithm) Process(
	symbol string,
	currentData *types.MarketData,
	historicalData []types.MarketData,
) (*AlgorithmResult, error) {
	if len(historicalData) < 2 {
		return nil, errors.New("insufficient historical data for fractional differentiation")
	}

	// Extract price series
	prices := make([]float64, len(historicalData))
	for i, data := range historicalData {
		prices[i] = data.Price
	}

	// Apply fractional differentiation
	var diffPrices []float64
	var err error

	if f.useFixedWidth {
		diffPrices, err = FixedWidthFractionalDiff(prices, f.d, f.windowSize)
	} else {
		diffPrices, err = FFD(prices, f.d, f.threshold)
	}

	if err != nil {
		return nil, err
	}

	// Generate explanation
	f.explanation = "Fractional differentiation applied to price series "
	if f.useFixedWidth {
		f.explanation += fmt.Sprintf("using fixed-width window approach with window size %d", f.windowSize)
	} else {
		f.explanation += fmt.Sprintf("using FFD method with weight threshold %.5f", f.threshold)
	}
	f.explanation += fmt.Sprintf(" and d=%.2f", f.d)
	
	// Store the differenced prices for potential use by other algorithms
	// In a real implementation, we'd want to make this available to other components
	// For now we just print the first few values
	if len(diffPrices) > 0 {
		f.explanation += fmt.Sprintf("\nFirst few differenced values: [%.4f", diffPrices[0])
		for i := 1; i < min(5, len(diffPrices)); i++ {
			f.explanation += fmt.Sprintf(", %.4f", diffPrices[i])
		}
		f.explanation += "]"
	}

	// For pure data transformation algorithms, we just return a "hold" signal
	// The transformed data itself can be used by other algorithms
	return &AlgorithmResult{
		Signal:      "hold",
		OrderType:   "none",
		Confidence:  0.5,
		Explanation: f.explanation,
	}, nil
}

// GetWeights calculates the weights (w) of the series (window) based on the differencing parameter d
// This corresponds to Snippet 5.1 in the book
func GetWeights(d float64, size int) []float64 {
	weights := make([]float64, size)
	weights[0] = 1.0 // w[0] = 1
	
	// Calculate weights recursively: w[k] = -w[k-1] * (d - k + 1) / k
	for k := 1; k < size; k++ {
		weights[k] = weights[k-1] * (-d + float64(k) - 1) / float64(k)
	}
	
	return weights
}

// FixedWidthFractionalDiff implements the fixed-width window method for fractional differentiation
// This corresponds to Snippet 5.4 in the book
func FixedWidthFractionalDiff(series []float64, d float64, windowSize int) ([]float64, error) {
	if windowSize < 1 {
		return nil, errors.New("window size must be at least 1")
	}
	
	if windowSize > len(series) {
		return nil, errors.New("window size cannot exceed series length")
	}
	
	// Get weights for the window
	weights := GetWeights(d, windowSize)
	
	// Calculate the fractionally differenced series
	result := make([]float64, len(series)-windowSize+1)
	
	for i := windowSize - 1; i < len(series); i++ {
		// Apply convolution for a window of the series with the weights
		var dot float64
		for j := 0; j < windowSize; j++ {
			dot += weights[j] * series[i-j]
		}
		result[i-windowSize+1] = dot
	}
	
	return result, nil
}

// FFD implements the Full Fractional Differencing method for fractional differentiation
// This corresponds to Snippet 5.5 in the book
func FFD(series []float64, d float64, threshold float64) ([]float64, error) {
	if len(series) == 0 {
		return nil, errors.New("series cannot be empty")
	}
	
	if threshold <= 0 {
		return nil, errors.New("threshold must be positive")
	}
	
	// Step 1: Compute weights until they fall below the threshold
	var weights []float64
	w := 1.0
	weights = append(weights, w)
	
	for k := 1; math.Abs(w) > threshold; k++ {
		w = w * (-d + float64(k) - 1) / float64(k)
		weights = append(weights, w)
		
		// Guard against extremely long weight computation
		if k > 100000 {
			return nil, errors.New("weight computation did not converge below threshold")
		}
	}
	
	// Step 2: Apply the weights to the series
	width := len(weights)
	result := make([]float64, len(series))
	
	// Initialize the first window where we don't have enough data for full computation
	for i := 0; i < len(series); i++ {
		// We can only use weights up to the current index
		maxWeight := int(math.Min(float64(i+1), float64(width)))
		
		// Apply available weights
		var dot float64
		for j := 0; j < maxWeight; j++ {
			dot += weights[j] * series[i-j]
		}
		result[i] = dot
	}
	
	return result, nil
}

// HelperFunctions for fractional differentiation

// IsStationary checks if a time series is stationary using ADF test
// This is a simplified placeholder - a proper implementation would use statistical tests
func IsStationary(series []float64) bool {
	if len(series) < 10 {
		return false // Not enough data to determine
	}
	
	// Compute mean and standard deviation
	avg := meanFD(series)
	
	// Check if the series is roughly constant (very simplified test)
	deviation := stdDevFD(series)
	
	// This is a highly simplified check - in production code you'd use proper statistical tests
	return deviation < 0.1 * math.Abs(avg)
}

// FindOptimalD finds the optimal differencing parameter d that makes the series stationary
// while preserving the maximum amount of memory
func FindOptimalD(series []float64, dVals []float64, threshold float64) (float64, error) {
	if len(series) < 10 {
		return 0, errors.New("series too short to determine optimal d")
	}
	
	if len(dVals) == 0 {
		// Default range of d values to test
		dVals = []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0}
	}
	
	// Try different d values and find the smallest one that makes the series stationary
	for _, d := range dVals {
		diffSeries, err := FFD(series, d, threshold)
		if err != nil {
			continue
		}
		
		if IsStationary(diffSeries) {
			return d, nil
		}
	}
	
	return 1.0, errors.New("could not find optimal d; returning maximum value")
}

// meanFD calculates the average of a slice of float64 values
// Using FD suffix to avoid conflicts with other functions
func meanFD(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	
	return sum / float64(len(values))
}

// stdDevFD calculates the standard deviation of a slice of float64 values
// Using FD suffix to avoid conflicts with other functions
func stdDevFD(values []float64) float64 {
	if len(values) <= 1 {
		return 0
	}
	
	avg := meanFD(values)
	sum := 0.0
	
	for _, v := range values {
		sum += math.Pow(v-avg, 2)
	}
	
	return math.Sqrt(sum / float64(len(values)-1))
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}