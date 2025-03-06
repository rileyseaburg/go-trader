package algo

import (
	"errors"
	"fmt"
	"log"
	"math/rand"

	"github.com/rileyseaburg/go-trader/types"
	"gonum.org/v1/gonum/mat"
)

// init registers the algorithm with the factory
func init() {
	Register(AlgorithmTypeSequentialBootstrap, func() Algorithm {
		return &SequentialBootstrapAlgorithm{
			BaseAlgorithm: BaseAlgorithm{},
		}
	})
}

// SequentialBootstrapAlgorithm implements the Sequential Bootstrap algorithm from
// de Prado's "Advances in Financial Machine Learning" (2018)
type SequentialBootstrapAlgorithm struct {
	BaseAlgorithm
	lookbackPeriod      int     // Number of past observations to consider
	confidenceThreshold float64 // Threshold for signal generation
	useSequential       bool    // Whether to use sequential bootstrap (if false, uses standard bootstrap)
	sampleSize          int     // Number of samples to take in bootstrap
	lastSamples         []int   // Last bootstrap samples used
}

// Name returns the name of the algorithm
func (s *SequentialBootstrapAlgorithm) Name() string {
	return "Sequential Bootstrap"
}

// Type returns the type of the algorithm
func (s *SequentialBootstrapAlgorithm) Type() AlgorithmType {
	return AlgorithmTypeSequentialBootstrap
}

// Description returns a brief description of the algorithm
func (s *SequentialBootstrapAlgorithm) Description() string {
	return "Performs sequential bootstrap sampling to control for overlapping outcomes, " +
		"which reduces the bias introduced by standard bootstrap methods in financial time series data."
}

// ParameterDescription returns a description of the parameters
func (s *SequentialBootstrapAlgorithm) ParameterDescription() map[string]string {
	return map[string]string{
		"lookback_period":      "Number of past observations to consider (default: 20)",
		"confidence_threshold": "Threshold for signal generation between 0 and 1 (default: 0.65)",
		"use_sequential":       "Whether to use sequential bootstrap (1) or standard bootstrap (0) (default: 1)",
		"sample_size":          "Number of samples to take in bootstrap (default: 50)",
	}
}

// Configure configures the algorithm with the given parameters
func (s *SequentialBootstrapAlgorithm) Configure(config AlgorithmConfig) error {
	if err := s.BaseAlgorithm.Configure(config); err != nil {
		return err
	}

	// Set default values
	s.lookbackPeriod = 20
	s.confidenceThreshold = 0.65
	s.useSequential = true
	s.sampleSize = 50

	// Override with provided values
	if val, ok := config.AdditionalParams["lookback_period"]; ok {
		if val < 1 {
			return errors.New("lookback_period must be at least 1")
		}
		s.lookbackPeriod = int(val)
	}

	if val, ok := config.AdditionalParams["confidence_threshold"]; ok {
		if val < 0 || val > 1 {
			return errors.New("confidence_threshold must be between 0 and 1")
		}
		s.confidenceThreshold = val
	}

	if val, ok := config.AdditionalParams["use_sequential"]; ok {
		s.useSequential = val > 0.5
	}

	if val, ok := config.AdditionalParams["sample_size"]; ok {
		if val < 1 {
			return errors.New("sample_size must be at least 1")
		}
		s.sampleSize = int(val)
	}

	return nil
}

// Process processes the market data and generates a trading signal
func (s *SequentialBootstrapAlgorithm) Process(
	symbol string,
	currentData *types.MarketData,
	historicalData []types.MarketData,
) (*AlgorithmResult, error) {
	if len(historicalData) < s.lookbackPeriod {
		return nil, fmt.Errorf("insufficient historical data: got %d, need at least %d",
			len(historicalData), s.lookbackPeriod)
	}

	// Extract needed historical data
	histLen := len(historicalData)
	if histLen > s.lookbackPeriod {
		historicalData = historicalData[histLen-s.lookbackPeriod:]
	}

	// Create t1 series (time index where label is determined)
	// In this implementation, we'll use price movements as our "labels"
	barIx := make([]int, len(historicalData))
	t1 := make([]float64, len(historicalData))

	for i := range historicalData {
		barIx[i] = i
		// Simple example: t1[i] represents when the "label" for bar i would be known
		// In practice, this could be more complex based on domain knowledge
		t1[i] = float64(i + 1) // One bar ahead
	}

	// Get indicator matrix
	indM, err := getIndMatrix(barIx, t1)
	if err != nil {
		return nil, fmt.Errorf("failed to create indicator matrix: %v", err)
	}

	// Perform bootstrap sampling
	var samples []int
	if s.useSequential {
		samples, err = seqBootstrap(indM, s.sampleSize)
		if err != nil {
			return nil, fmt.Errorf("sequential bootstrap failed: %v", err)
		}
	} else {
		samples = standardBootstrap(indM, s.sampleSize)
	}

	// Store samples for explanation
	s.lastSamples = samples

	// Calculate statistics on the bootstrapped samples
	var upSignals, downSignals int

	// Use the samples to make predictions
	// Example: count how many bootstrapped samples would predict up vs down
	for _, idx := range samples {
		if idx >= len(historicalData)-1 {
			continue // Skip if index is out of bounds
		}

		// Simple signal: if next price > current price, it's an up signal
		if historicalData[idx+1].Price > historicalData[idx].Price {
			upSignals++
		} else {
			downSignals++
		}
	}

	// Calculate confidence based on proportion of consistent signals
	totalSignals := upSignals + downSignals
	var confidence float64
	var signal, orderType string

	if totalSignals > 0 {
		if upSignals > downSignals {
			confidence = float64(upSignals) / float64(totalSignals)
			signal = "BUY"
			orderType = "MARKET"
		} else {
			confidence = float64(downSignals) / float64(totalSignals)
			signal = "SELL"
			orderType = "MARKET"
		}
	} else {
		confidence = 0.5
		signal = "HOLD"
		orderType = "NONE"
	}

	// Only generate a signal if confidence exceeds threshold
	if confidence < s.confidenceThreshold {
		signal = "HOLD"
		orderType = "NONE"
	}

	// Generate explanation
	s.explanation = fmt.Sprintf(
		"Sequential Bootstrap analysis on %d samples with %d lookback period.\n"+
			"Up signals: %d, Down signals: %d, Confidence: %.2f%%\n"+
			"Average uniqueness of samples: %.2f\n"+
			"Confidence threshold: %.2f",
		s.sampleSize, s.lookbackPeriod,
		upSignals, downSignals, confidence*100,
		calculateAverageUniqueness(s.lastSamples),
		s.confidenceThreshold,
	)

	return &AlgorithmResult{
		Signal:      signal,
		OrderType:   orderType,
		Confidence:  confidence,
		Explanation: s.explanation,
	}, nil
}

// calculateAverageUniqueness calculates the average uniqueness of the samples
func calculateAverageUniqueness(samples []int) float64 {
	if len(samples) <= 1 {
		return 1.0
	}

	// Count unique values
	uniqueValues := make(map[int]bool)
	for _, s := range samples {
		uniqueValues[s] = true
	}

	return float64(len(uniqueValues)) / float64(len(samples))
}

// getIndMatrix builds an indicator matrix from bar indices and a t1 series
// Implementation of Snippet 4.3 from the book
func getIndMatrix(barIx []int, t1 []float64) (*mat.Dense, error) {
	if len(barIx) == 0 || len(t1) == 0 {
		return nil, errors.New("bar indices or t1 series cannot be empty")
	}

	// Initialize matrix with zeros
	indM := mat.NewDense(len(barIx), len(t1), nil)

	// For each bar and corresponding end time in t1
	for i, endTime := range t1 {
		// Find bars that influence this label
		for j, barTime := range barIx {
			// If the bar's time is less than or equal to the end time,
			// it influences the label (set matrix value to 1)
			if float64(barTime) <= endTime {
				indM.Set(j, i, 1)
			}
		}
	}

	return indM, nil
}

// getAvgUniqueness computes the average uniqueness from an indicator matrix
// Implementation of Snippet 4.4 from the book
func getAvgUniqueness(indM *mat.Dense) ([]float64, error) {
	if indM == nil {
		return nil, errors.New("indicator matrix cannot be nil")
	}

	r, c := indM.Dims()
	if r == 0 || c == 0 {
		return nil, errors.New("indicator matrix cannot have zero dimensions")
	}

	// Calculate concurrency (sum along rows)
	concurrency := make([]float64, c)
	for j := 0; j < c; j++ {
		col := mat.Col(nil, j, indM)
		sum := 0.0
		for _, val := range col {
			sum += val
		}
		concurrency[j] = sum
	}

	// Calculate uniqueness (1/concurrency for each element)
	uniqueness := make([]float64, r*c)
	for i := 0; i < r; i++ {
		for j := 0; j < c; j++ {
			val := indM.At(i, j)
			if val > 0 && concurrency[j] > 0 {
				uniqueness[i*c+j] = val / concurrency[j]
			} else {
				uniqueness[i*c+j] = 0
			}
		}
	}

	// Filter out zeros and calculate mean
	result := make([]float64, c)
	for j := 0; j < c; j++ {
		colSum := 0.0
		count := 0

		for i := 0; i < r; i++ {
			val := uniqueness[i*c+j]
			if val > 0 {
				colSum += val
				count++
			}
		}

		if count > 0 {
			result[j] = colSum / float64(count)
		} else {
			result[j] = 0
		}
	}

	return result, nil
}

// seqBootstrap performs sequential bootstrap sampling
// Implementation of Snippet 4.5 from the book
func seqBootstrap(indM *mat.Dense, sLength int) ([]int, error) {
	if indM == nil {
		return nil, errors.New("indicator matrix cannot be nil")
	}

	_, c := indM.Dims()
	if c == 0 {
		return nil, errors.New("indicator matrix must have at least one column")
	}

	// If no length specified, use number of columns
	if sLength <= 0 {
		sLength = c
	}

	if sLength > c {
		return nil, fmt.Errorf("sample length (%d) cannot exceed number of columns (%d)", sLength, c)
	}

	// Initialize sequence of draws
	phi := make([]int, 0, sLength)

	// Keep drawing until we have sLength samples
	for len(phi) < sLength {
		// Calculate average uniqueness for each column
		avgUniqueness := make(map[int]float64)

		// For each candidate column
		for i := 0; i < c; i++ {
			// Create a temporary sequence including the candidate
			tempPhi := append(phi, i)

			// Select columns from the indicator matrix based on tempPhi
			tempMatrix := selectColumns(indM, tempPhi)

			// Calculate average uniqueness
			avgU, err := getAvgUniqueness(tempMatrix)
			if err != nil {
				continue
			}

			// Use the last value (corresponding to the new candidate)
			if len(avgU) > 0 {
				avgUniqueness[i] = avgU[len(avgU)-1]
			}
		}

		// Convert to arrays for weighted sampling
		indices := make([]int, 0, len(avgUniqueness))
		weights := make([]float64, 0, len(avgUniqueness))

		for idx, u := range avgUniqueness {
			indices = append(indices, idx)
			weights = append(weights, u)
		}

		// If no valid candidates, fall back to uniform sampling
		if len(indices) == 0 {
			availIndices := make([]int, c)
			for i := 0; i < c; i++ {
				availIndices[i] = i
			}
			phi = append(phi, availIndices[rand.Intn(len(availIndices))])
		} else {
			// Draw based on uniqueness weights
			selectedIdx := weightedChoice(indices, weights)
			phi = append(phi, selectedIdx)
		}
	}

	return phi, nil
}

// selectColumns selects specific columns from a matrix and returns a new matrix
func selectColumns(m *mat.Dense, indices []int) *mat.Dense {
	r, _ := m.Dims()

	// Create a new matrix with the selected columns
	data := make([]float64, r*len(indices))
	result := mat.NewDense(r, len(indices), data)

	for j, idx := range indices {
		col := mat.Col(nil, idx, m)
		for i, val := range col {
			result.Set(i, j, val)
		}
	}

	return result
}

// standardBootstrap performs a standard bootstrap on the indicator matrix
// (simple random sampling with replacement)
func standardBootstrap(indM *mat.Dense, sLength int) []int {
	_, c := indM.Dims()
	if sLength <= 0 {
		sLength = c
	}

	log.Printf("DEBUG: Running standardBootstrap with dimensions: %d, sample length: %d", c, sLength)

	// Simple random sampling with replacement
	samples := make([]int, sLength)
	for i := range samples {
		samples[i] = rand.Intn(c)
	}
	return samples
}
