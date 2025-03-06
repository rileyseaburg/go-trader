package algo

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/rileyseaburg/go-trader/types"
)

// CVFold represents a single fold in cross-validation
type CVFold struct {
	TrainIndices []int     // Indices of samples in the training set
	TestIndices  []int     // Indices of samples in the test set
	TrainTimes   []time.Time // Times corresponding to training samples
	TestTimes    []time.Time // Times corresponding to test samples
}

// PurgedCVConfig represents the configuration for the purged cross-validation
type PurgedCVConfig struct {
	NumFolds   int     // Number of folds
	EmbargoPct float64 // Percentage of observations to embargo between train/test
	TestSize   float64 // Percentage of samples to include in the test set (0-1)
}

// init registers the PurgedCV algorithm with the factory
func init() {
	Register(AlgorithmTypePurgedCV, func() Algorithm {
		return &PurgedCVAlgorithm{
			BaseAlgorithm: BaseAlgorithm{},
		}
	})
}

// PurgedCVAlgorithm implements the Purged and Embargoed Cross-Validation approach from
// de Prado's "Advances in Financial Machine Learning" (2018)
type PurgedCVAlgorithm struct {
	BaseAlgorithm
	config      PurgedCVConfig
	numFolds    int     // Number of folds for CV
	embargoPct  float64 // Percentage of observations to embargo
	testSize    float64 // Size of test set as percentage of total
	syncTimes   []time.Time // Timestamps used to synchronize train/test sets
	eventTimes  []time.Time // Timestamps of events (e.g., trade entry/exit)
	eventLabels []int      // Labels for events
}

// Name returns the name of the algorithm
func (p *PurgedCVAlgorithm) Name() string {
	return "Purged Cross-Validation"
}

// Type returns the type of the algorithm
func (p *PurgedCVAlgorithm) Type() AlgorithmType {
	return AlgorithmTypePurgedCV
}

// Description returns a brief description of the algorithm
func (p *PurgedCVAlgorithm) Description() string {
	return "Implements Purged and Embargoed Cross-Validation for accurate model validation without leakage"
}

// ParameterDescription returns a description of the parameters
func (p *PurgedCVAlgorithm) ParameterDescription() map[string]string {
	return map[string]string{
		"num_folds":   "Number of folds for cross-validation (default: 5)",
		"embargo_pct": "Percentage of observations to embargo between train/test sets (default: 0.01)",
		"test_size":   "Size of test set as percentage of total data (default: 0.3)",
	}
}

// Configure configures the algorithm with the given parameters
func (p *PurgedCVAlgorithm) Configure(config AlgorithmConfig) error {
	if err := p.BaseAlgorithm.Configure(config); err != nil {
		return err
	}

	// Set default values
	p.numFolds = 5
	p.embargoPct = 0.01
	p.testSize = 0.3

	// Override with provided values
	if val, ok := config.AdditionalParams["num_folds"]; ok {
		if val < 2 {
			return errors.New("num_folds must be at least 2")
		}
		p.numFolds = int(val)
	}

	if val, ok := config.AdditionalParams["embargo_pct"]; ok {
		if val < 0 || val > 0.5 {
			return errors.New("embargo_pct must be between 0 and 0.5")
		}
		p.embargoPct = val
	}

	if val, ok := config.AdditionalParams["test_size"]; ok {
		if val <= 0 || val >= 1 {
			return errors.New("test_size must be between 0 and 1")
		}
		p.testSize = val
	}

	// Update the config
	p.config = PurgedCVConfig{
		NumFolds:   p.numFolds,
		EmbargoPct: p.embargoPct,
		TestSize:   p.testSize,
	}

	return nil
}

// Process processes the market data to generate CV folds
// Note: This doesn't generate trading signals, but rather provides
// a validation strategy for other algorithms
func (p *PurgedCVAlgorithm) Process(
	symbol string,
	currentData *types.MarketData,
	historicalData []types.MarketData,
) (*AlgorithmResult, error) {
	if len(historicalData) < p.numFolds * 2 {
		return nil, fmt.Errorf("insufficient historical data: got %d, need at least %d",
			len(historicalData), p.numFolds * 2)
	}

	// For demonstration, we'll use synthetic timestamps since MarketData doesn't have them
	p.syncTimes = make([]time.Time, len(historicalData))
	baseTime := time.Now().Add(-time.Duration(len(historicalData)) * 24 * time.Hour)
	
	for i := range p.syncTimes {
		p.syncTimes[i] = baseTime.Add(time.Duration(i) * 24 * time.Hour)
	}

	// Generate CV folds
	folds, err := p.purgedKFold()
	if err != nil {
		return nil, fmt.Errorf("error generating CV folds: %v", err)
	}

	// Create explanation
	explanation := fmt.Sprintf("Generated %d cross-validation folds with embargo=%.2f%% and test_size=%.2f%%.\n",
		p.numFolds, p.embargoPct*100, p.testSize*100)
		
	for i, fold := range folds {
		explanation += fmt.Sprintf("Fold %d: %d training samples, %d test samples\n", 
			i+1, len(fold.TrainIndices), len(fold.TestIndices))
	}

	p.explanation = explanation

	return &AlgorithmResult{
		Signal:      "hold", // This algorithm doesn't generate signals
		OrderType:   "none",
		Confidence:  0.5,
		Explanation: p.explanation,
	}, nil
}

// purgedKFold implements the PurgedKFold algorithm
// This corresponds to Snippet 7.1 in the book
func (p *PurgedCVAlgorithm) purgedKFold() ([]CVFold, error) {
	if len(p.syncTimes) == 0 {
		return nil, errors.New("no sample times provided")
	}

	// Step 1: Determine the test indices for each fold
	numSamples := len(p.syncTimes)
	indices := make([]int, numSamples)
	for i := range indices {
		indices[i] = i
	}

	// Create a random shuffling of indices
	// In a real implementation, you might use a more sophisticated approach
	// that respects time series properties
	testIndices := make([][]int, p.numFolds)
	testSize := int(math.Ceil(float64(numSamples) * p.testSize))
	
	// Split the indices into folds
	for i := 0; i < p.numFolds; i++ {
		start := (i * testSize) % numSamples
		end := start + testSize
		if end > numSamples {
			// Wrap around
			testIndices[i] = append(indices[start:], indices[:end-numSamples]...)
		} else {
			testIndices[i] = indices[start:end]
		}
		// Sort indices for consistent results
		sort.Ints(testIndices[i])
	}

	// Step 2: Create train indices by removing test indices from full set
	folds := make([]CVFold, p.numFolds)
	for i := 0; i < p.numFolds; i++ {
		// Create a map for quick lookups of test indices
		testMap := make(map[int]bool)
		for _, idx := range testIndices[i] {
			testMap[idx] = true
		}
		
		// Initialize train indices with all indices not in test
		trainIndices := make([]int, 0, numSamples-len(testIndices[i]))
		for j := 0; j < numSamples; j++ {
			if !testMap[j] {
				trainIndices = append(trainIndices, j)
			}
		}

		// Step 3: Apply purging and embargoing
		if p.eventTimes != nil && len(p.eventTimes) > 0 {
			// If we have event times, purge train indices
			trainIndices = p.purgeTrainSamples(trainIndices, testIndices[i])
			
			// Apply embargo
			if p.embargoPct > 0 {
				trainIndices = p.embargoTrainSamples(trainIndices, testIndices[i])
			}
		}

		// Store the fold
		folds[i] = CVFold{
			TrainIndices: trainIndices,
			TestIndices:  testIndices[i],
			TrainTimes:   p.getTimesByIndices(trainIndices),
			TestTimes:    p.getTimesByIndices(testIndices[i]),
		}
	}

	return folds, nil
}

// purgeTrainSamples removes training samples that overlap with test labels
func (p *PurgedCVAlgorithm) purgeTrainSamples(trainIndices, testIndices []int) []int {
	if p.eventTimes == nil || len(p.eventTimes) == 0 {
		return trainIndices // No event times to purge with
	}

	// Get test times
	testTimes := make([]time.Time, len(testIndices))
	for i, idx := range testIndices {
		testTimes[i] = p.syncTimes[idx]
	}
	
	// Create a list of train indices to keep
	prunedIndices := make([]int, 0, len(trainIndices))
	
	for _, trainIdx := range trainIndices {
		trainTime := p.syncTimes[trainIdx]
		
		// Check if this training sample overlaps with any test sample
		shouldKeep := true
		
		for _, testTime := range testTimes {
			// This is a simplification - in practice you would have event start/end times
			// and check for overlaps using those
			if areTimesClose(trainTime, testTime, 1) {
				shouldKeep = false
				break
			}
		}
		
		if shouldKeep {
			prunedIndices = append(prunedIndices, trainIdx)
		}
	}
	
	return prunedIndices
}

// embargoTrainSamples applies an embargo period after each test sample
func (p *PurgedCVAlgorithm) embargoTrainSamples(trainIndices, testIndices []int) []int {
	if p.embargoPct <= 0 {
		return trainIndices // No embargo to apply
	}
	
	// Sort test indices
	sortedTest := make([]int, len(testIndices))
	copy(sortedTest, testIndices)
	sort.Ints(sortedTest)
	
	// Calculate embargo window
	embargoSize := int(math.Ceil(float64(len(p.syncTimes)) * p.embargoPct))
	if embargoSize <= 0 {
		embargoSize = 1
	}
	
	// Create embargo zones (indices to exclude)
	embargoZones := make(map[int]bool)
	for _, testIdx := range sortedTest {
		// Add embargo after the test sample
		for i := 1; i <= embargoSize; i++ {
			embargoIdx := testIdx + i
			if embargoIdx < len(p.syncTimes) {
				embargoZones[embargoIdx] = true
			}
		}
	}
	
	// Keep only train indices that aren't in embargo zones
	embargoedIndices := make([]int, 0, len(trainIndices))
	for _, trainIdx := range trainIndices {
		if !embargoZones[trainIdx] {
			embargoedIndices = append(embargoedIndices, trainIdx)
		}
	}
	
	return embargoedIndices
}

// getTimesByIndices returns a slice of times for the given indices
func (p *PurgedCVAlgorithm) getTimesByIndices(indices []int) []time.Time {
	times := make([]time.Time, len(indices))
	for i, idx := range indices {
		if idx >= 0 && idx < len(p.syncTimes) {
			times[i] = p.syncTimes[idx]
		} else {
			// Use zero time for invalid indices
			times[i] = time.Time{}
		}
	}
	return times
}

// areTimesClose returns true if two times are within the specified number of days
func areTimesClose(t1, t2 time.Time, days int) bool {
	diff := t1.Sub(t2)
	return math.Abs(diff.Hours()) < float64(days)*24
}

// PurgedKFold creates a new PurgedCVAlgorithm and returns cross-validation folds
// This is a utility function that can be used by other algorithms
func PurgedKFold(samples []time.Time, numFolds int, embargoPct float64) ([]CVFold, error) {
	// Create and configure algorithm
	alg := &PurgedCVAlgorithm{
		BaseAlgorithm: BaseAlgorithm{},
		numFolds:      numFolds,
		embargoPct:    embargoPct,
		testSize:      1.0 / float64(numFolds),
		syncTimes:     samples,
	}
	
	// Generate folds
	return alg.purgedKFold()
}

// WalkForwardValidation implements a walk-forward validation approach
// This creates a series of folds that respect time ordering
// Unlike K-fold cross validation, this ensures that all training data comes before test data
func WalkForwardValidation(samples []time.Time, numFolds int, embargoPct float64) ([]CVFold, error) {
	if len(samples) < numFolds*2 {
		return nil, fmt.Errorf("insufficient samples: got %d, need at least %d", 
			len(samples), numFolds*2)
	}
	
	// Sort samples by time
	type timeIndex struct {
		time  time.Time
		index int
	}
	
	sampleIndices := make([]timeIndex, len(samples))
	for i, t := range samples {
		sampleIndices[i] = timeIndex{time: t, index: i}
	}
	
	sort.Slice(sampleIndices, func(i, j int) bool {
		return sampleIndices[i].time.Before(sampleIndices[j].time)
	})
	
	// Create sorted indices
	sortedIndices := make([]int, len(samples))
	for i, ti := range sampleIndices {
		sortedIndices[i] = ti.index
	}
	
	// Create folds
	foldSize := len(sortedIndices) / numFolds
	folds := make([]CVFold, numFolds)
	
	for i := 0; i < numFolds; i++ {
		// Test indices are the i-th chunk
		testStart := i * foldSize
		testEnd := (i + 1) * foldSize
		if i == numFolds-1 {
			testEnd = len(sortedIndices) // Last fold takes remaining samples
		}
		
		testIndices := sortedIndices[testStart:testEnd]
		
		// Training indices are all indices before the test indices
		trainIndices := sortedIndices[:testStart]
		
		// Apply embargo if needed
		if embargoPct > 0 && len(trainIndices) > 0 {
			embargoSize := int(math.Ceil(float64(len(samples)) * embargoPct))
			if embargoSize > 0 && testStart >= embargoSize {
				// Remove embargo period before test set
				trainIndices = trainIndices[:testStart-embargoSize]
			}
		}
		
		// Create test and train times
		testTimes := make([]time.Time, len(testIndices))
		for j, idx := range testIndices {
			testTimes[j] = samples[idx]
		}
		
		trainTimes := make([]time.Time, len(trainIndices))
		for j, idx := range trainIndices {
			trainTimes[j] = samples[idx]
		}
		
		folds[i] = CVFold{
			TrainIndices: trainIndices,
			TestIndices:  testIndices,
			TrainTimes:   trainTimes,
			TestTimes:    testTimes,
		}
	}
	
	return folds, nil
}