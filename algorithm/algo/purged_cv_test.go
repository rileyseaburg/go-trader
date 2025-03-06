package algo

import (
	"math"
	"testing"
	"time"

	"github.com/rileyseaburg/go-trader/types"
)

func TestPurgedCVAlgorithm_Interface(t *testing.T) {
	// Create an instance of the algorithm
	alg, err := Create(AlgorithmTypePurgedCV)
	if err != nil {
		t.Fatalf("Failed to create algorithm: %v", err)
	}

	// Test Name() method
	name := alg.Name()
	if name != "Purged Cross-Validation" {
		t.Errorf("Name() = %v, want %v", name, "Purged Cross-Validation")
	}

	// Test Type() method
	algType := alg.Type()
	if algType != AlgorithmTypePurgedCV {
		t.Errorf("Type() = %v, want %v", algType, AlgorithmTypePurgedCV)
	}

	// Test Description() method
	desc := alg.Description()
	if desc == "" {
		t.Error("Description() returned empty string")
	}

	// Test ParameterDescription() method
	params := alg.ParameterDescription()
	if len(params) == 0 {
		t.Error("ParameterDescription() returned empty map")
	}

	// Test parameter descriptions
	requiredParams := []string{
		"num_folds",
		"embargo_pct",
		"test_size",
	}

	for _, param := range requiredParams {
		if _, ok := params[param]; !ok {
			t.Errorf("ParameterDescription() missing required parameter: %s", param)
		}
	}
}

func TestPurgedCVAlgorithm_Configure(t *testing.T) {
	// Create an instance of the algorithm
	alg, err := Create(AlgorithmTypePurgedCV)
	if err != nil {
		t.Fatalf("Failed to create algorithm: %v", err)
	}

	tests := []struct {
		name    string
		config  AlgorithmConfig
		wantErr bool
	}{
		{
			name: "Valid configuration",
			config: AlgorithmConfig{
				RiskAversion:      5.0,
				MaxPositionWeight: 0.3,
				MinPositionWeight: 0.05,
				TargetReturn:      0.1,
				HistoricalDays:    30,
				AdditionalParams: map[string]float64{
					"num_folds":   3,
					"embargo_pct": 0.05,
					"test_size":   0.3,
				},
			},
			wantErr: false,
		},
		{
			name: "Invalid num_folds",
			config: AlgorithmConfig{
				AdditionalParams: map[string]float64{
					"num_folds": 1, // Must be at least 2
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid embargo_pct (too high)",
			config: AlgorithmConfig{
				AdditionalParams: map[string]float64{
					"embargo_pct": 0.6, // Must be between 0 and 0.5
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid test_size (too high)",
			config: AlgorithmConfig{
				AdditionalParams: map[string]float64{
					"test_size": 1.0, // Must be between 0 and 1 (exclusive)
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := alg.Configure(tt.config)

			if (err != nil) != tt.wantErr {
				t.Errorf("Configure() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPurgedKFold(t *testing.T) {
	// Create test sample times
	now := time.Now()
	samples := make([]time.Time, 100)
	for i := range samples {
		samples[i] = now.Add(time.Duration(i) * time.Hour)
	}

	tests := []struct {
		name       string
		samples    []time.Time
		numFolds   int
		embargoPct float64
		expectErr  bool
	}{
		{
			name:       "5-fold CV with no embargo",
			samples:    samples,
			numFolds:   5,
			embargoPct: 0,
			expectErr:  false,
		},
		{
			name:       "5-fold CV with embargo",
			samples:    samples,
			numFolds:   5,
			embargoPct: 0.05,
			expectErr:  false,
		},
		{
			name:       "Empty samples",
			samples:    []time.Time{},
			numFolds:   5,
			embargoPct: 0,
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			folds, err := PurgedKFold(tt.samples, tt.numFolds, tt.embargoPct)

			if (err != nil) != tt.expectErr {
				t.Errorf("PurgedKFold() error = %v, expectErr %v", err, tt.expectErr)
				return
			}

			if tt.expectErr {
				return
			}

			// Verify number of folds
			if len(folds) != tt.numFolds {
				t.Errorf("PurgedKFold() returned %d folds, expected %d", len(folds), tt.numFolds)
			}

			// Verify that each sample appears exactly once across all test sets
			testSampleCount := make(map[int]int)
			
			for _, fold := range folds {
				// Test indices shouldn't be empty
				if len(fold.TestIndices) == 0 {
					t.Error("PurgedKFold() returned fold with empty test indices")
				}
				
				// Count each test index
				for _, idx := range fold.TestIndices {
					testSampleCount[idx]++
				}
			}
			
			// Check that each sample is used in exactly one test set
			for i := 0; i < len(tt.samples); i++ {
				count := testSampleCount[i]
				if count != 1 {
					t.Errorf("Sample %d is used in %d test sets, expected 1", i, count)
				}
			}
			
			// Check that train and test sets are disjoint for each fold
			for i, fold := range folds {
				trainMap := make(map[int]bool)
				for _, idx := range fold.TrainIndices {
					trainMap[idx] = true
				}
				
				for _, testIdx := range fold.TestIndices {
					if trainMap[testIdx] {
						t.Errorf("Fold %d: Test index %d is also in train set", i, testIdx)
					}
				}
			}
		})
	}
}

func TestWalkForwardValidation(t *testing.T) {
	// Create test sample times (sorted)
	now := time.Now()
	samples := make([]time.Time, 100)
	for i := range samples {
		samples[i] = now.Add(time.Duration(i) * time.Hour)
	}

	tests := []struct {
		name       string
		samples    []time.Time
		numFolds   int
		embargoPct float64
		expectErr  bool
	}{
		{
			name:       "3-fold walk-forward with no embargo",
			samples:    samples,
			numFolds:   3,
			embargoPct: 0,
			expectErr:  false,
		},
		{
			name:       "3-fold walk-forward with embargo",
			samples:    samples,
			numFolds:   3,
			embargoPct: 0.05,
			expectErr:  false,
		},
		{
			name:       "Insufficient samples",
			samples:    samples[:3],
			numFolds:   3,
			embargoPct: 0,
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			folds, err := WalkForwardValidation(tt.samples, tt.numFolds, tt.embargoPct)

			if (err != nil) != tt.expectErr {
				t.Errorf("WalkForwardValidation() error = %v, expectErr %v", err, tt.expectErr)
				return
			}

			if tt.expectErr {
				return
			}

			// Verify number of folds
			if len(folds) != tt.numFolds {
				t.Errorf("WalkForwardValidation() returned %d folds, expected %d", len(folds), tt.numFolds)
			}

			// Verify time ordering constraint: all training data must be before test data
			for i, fold := range folds {
				// Find the latest time in training data
				var latestTrainTime time.Time
				for _, trainTime := range fold.TrainTimes {
					if trainTime.After(latestTrainTime) {
						latestTrainTime = trainTime
					}
				}
				
				// For non-empty train sets, all test times should be after the latest train time
				if len(fold.TrainTimes) > 0 {
					for j, testTime := range fold.TestTimes {
						if !testTime.After(latestTrainTime) {
							t.Errorf("Fold %d: Test time %d (%v) is not after latest train time (%v)",
								i, j, testTime, latestTrainTime)
						}
					}
				}
			}
		})
	}
}

func TestTimeOperations(t *testing.T) {
	// Test areTimesClose
	t1 := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)
	
	tests := []struct {
		name     string
		t2       time.Time
		days     int
		expected bool
	}{
		{
			name:     "Same time",
			t2:       t1,
			days:     1,
			expected: true,
		},
		{
			name:     "12 hours difference (within 1 day)",
			t2:       t1.Add(12 * time.Hour),
			days:     1,
			expected: true,
		},
		{
			name:     "25 hours difference (outside 1 day)",
			t2:       t1.Add(25 * time.Hour),
			days:     1,
			expected: false,
		},
		{
			name:     "36 hours difference (within 2 days)",
			t2:       t1.Add(36 * time.Hour),
			days:     2,
			expected: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := areTimesClose(t1, tt.t2, tt.days)
			if result != tt.expected {
				t.Errorf("areTimesClose(%v, %v, %d) = %v, expected %v", 
					t1, tt.t2, tt.days, result, tt.expected)
			}
		})
	}
}

func TestPurgedCVAlgorithm_Process(t *testing.T) {
	// Create an instance of the algorithm
	alg, err := Create(AlgorithmTypePurgedCV)
	if err != nil {
		t.Fatalf("Failed to create algorithm: %v", err)
	}

	// Configure the algorithm
	config := AlgorithmConfig{
		AdditionalParams: map[string]float64{
			"num_folds":   3,
			"embargo_pct": 0.05,
			"test_size":   0.3,
		},
	}

	err = alg.Configure(config)
	if err != nil {
		t.Fatalf("Failed to configure algorithm: %v", err)
	}

	// Create test data
	currentData := &types.MarketData{
		Symbol:    "AAPL",
		Price:     150.0,
		High24h:   155.0,
		Low24h:    145.0,
		Volume24h: 1000000,
		Change24h: 2.5,
	}

	// Create historical data
	historicalData := make([]types.MarketData, 30)
	for i := range historicalData {
		historicalData[i] = types.MarketData{
			Symbol:    "AAPL",
			Price:     float64(100 + i),
			High24h:   float64(105 + i),
			Low24h:    float64(95 + i),
			Volume24h: 1000000,
			Change24h: 0.5,
		}
	}

	// Test Process method
	result, err := alg.Process("AAPL", currentData, historicalData)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	// Verify result (this algorithm doesn't generate trading signals)
	if result == nil {
		t.Error("Process() returned nil result")
	} else {
		// Should return a hold signal since this is not a trading strategy
		if result.Signal != "hold" {
			t.Errorf("Process() returned signal %s, expected 'hold'", result.Signal)
		}

		if result.Explanation == "" {
			t.Error("Process() result has empty explanation")
		}

		t.Logf("Algorithm explanation: %s", result.Explanation)
	}
}

func TestPurgeAndEmbargo(t *testing.T) {
	// Create an instance of the algorithm and access its internal methods
	alg := &PurgedCVAlgorithm{
		BaseAlgorithm: BaseAlgorithm{},
		numFolds:      3,
		embargoPct:    0.1,
		testSize:      0.3,
	}
	
	// Create test times
	now := time.Now()
	alg.syncTimes = make([]time.Time, 100)
	for i := range alg.syncTimes {
		alg.syncTimes[i] = now.Add(time.Duration(i) * time.Hour)
	}
	
	// Test purgeTrainSamples
	trainIndices := make([]int, 50)
	for i := range trainIndices {
		trainIndices[i] = i
	}
	
	testIndices := []int{60, 70, 80}
	
	// Create event times (needed for purging)
	alg.eventTimes = alg.syncTimes
	
	// Test purging
	purgedIndices := alg.purgeTrainSamples(trainIndices, testIndices)
	
	// Since we're using a simple implementation of purging based on areTimesClose,
	// we expect some indices to be purged, but not all
	if len(purgedIndices) >= len(trainIndices) {
		t.Errorf("purgeTrainSamples() did not remove any indices")
	}
	
	if len(purgedIndices) == 0 {
		t.Errorf("purgeTrainSamples() removed all indices")
	}
	
	// Test embargoing
	embargoedIndices := alg.embargoTrainSamples(trainIndices, testIndices)
	
	// Embargo should remove indices immediately after test indices
	for _, testIdx := range testIndices {
		embargoSize := int(math.Ceil(float64(len(alg.syncTimes)) * alg.embargoPct))
		for i := 1; i <= embargoSize; i++ {
			embargoIdx := testIdx + i
			if embargoIdx < len(alg.syncTimes) {
				// Check that embargoIdx is not in embargoedIndices
				for _, idx := range embargoedIndices {
					if idx == embargoIdx {
						t.Errorf("embargoTrainSamples() did not remove index %d (embargo after test index %d)",
							embargoIdx, testIdx)
					}
				}
			}
		}
	}
}