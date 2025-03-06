package algo

import (
	"math"
	"testing"

	"github.com/rileyseaburg/go-trader/types"
)

func TestGetWeights(t *testing.T) {
	tests := []struct {
		name     string
		d        float64
		size     int
		expected []float64
	}{
		{
			name: "d=0.5, size=5",
			d:    0.5,
			size: 5,
			expected: []float64{
				1.0,
				-0.5,
				-0.125,
				-0.0625,
				-0.0390625,
			},
		},
		{
			name: "d=0.1, size=3",
			d:    0.1,
			size: 3,
			expected: []float64{
				1.0,
				-0.1,
				-0.045,
			},
		},
		{
			name: "d=1.0, size=4 (first differencing)",
			d:    1.0,
			size: 4,
			expected: []float64{
				1.0,
				-1.0,
				0.0,
				0.0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			weights := GetWeights(tt.d, tt.size)

			// Check if the length is correct
			if len(weights) != tt.size {
				t.Errorf("expected weights of length %d, got %d", tt.size, len(weights))
			}

			// Check if the values are approximately correct (allowing for floating point precision)
			for i, expected := range tt.expected {
				if i < len(weights) {
					if math.Abs(weights[i]-expected) > 1e-6 {
						t.Errorf("weights[%d] = %f, expected %f", i, weights[i], expected)
					}
				}
			}
		})
	}
}

func TestFixedWidthFractionalDiff(t *testing.T) {
	tests := []struct {
		name         string
		series       []float64
		d            float64
		windowSize   int
		expectedLen  int
		expectedErr  bool
		checkSpecial bool
		specialCheck func(t *testing.T, result []float64)
	}{
		{
			name:        "Empty series",
			series:      []float64{},
			d:           0.5,
			windowSize:  5,
			expectedLen: 0,
			expectedErr: true,
		},
		{
			name:        "Window larger than series",
			series:      []float64{1.0, 2.0, 3.0},
			d:           0.5,
			windowSize:  5,
			expectedLen: 0,
			expectedErr: true,
		},
		{
			name:        "Valid series with d=0.5",
			series:      []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0},
			d:           0.5,
			windowSize:  3,
			expectedLen: 8,
			expectedErr: false,
			checkSpecial: true,
			specialCheck: func(t *testing.T, result []float64) {
				// With d=0.5 and window=3, the first value should be:
				// 1.0*3.0 - 0.5*2.0 - 0.125*1.0 = 3 - 1 - 0.125 = 1.875
				expected := 1.875
				if math.Abs(result[0]-expected) > 1e-6 {
					t.Errorf("First value expected to be %f, got %f", expected, result[0])
				}
			},
		},
		{
			name:        "Constant series with d=0.5",
			series:      []float64{5.0, 5.0, 5.0, 5.0, 5.0},
			d:           0.5,
			windowSize:  2,
			expectedLen: 4,
			expectedErr: false,
			checkSpecial: true,
			specialCheck: func(t *testing.T, result []float64) {
				// For a constant series, all values after differencing should be close to 0
				// With d=0.5, weights=[1, -0.5]
				// First value: 1*5 - 0.5*5 = 5 - 2.5 = 2.5
				expected := 2.5
				if math.Abs(result[0]-expected) > 1e-6 {
					t.Errorf("First value expected to be %f, got %f", expected, result[0])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FixedWidthFractionalDiff(tt.series, tt.d, tt.windowSize)

			// Check if error status matches expectation
			if (err != nil) != tt.expectedErr {
				t.Errorf("expected error: %v, got error: %v", tt.expectedErr, err != nil)
				return
			}

			// Skip remaining checks if we expected an error
			if tt.expectedErr {
				return
			}

			// Check result length
			if len(result) != tt.expectedLen {
				t.Errorf("expected result length %d, got %d", tt.expectedLen, len(result))
			}

			// Run special checks if needed
			if tt.checkSpecial && tt.specialCheck != nil {
				tt.specialCheck(t, result)
			}
		})
	}
}

func TestFFD(t *testing.T) {
	tests := []struct {
		name         string
		series       []float64
		d            float64
		threshold    float64
		expectedLen  int
		expectedErr  bool
		checkSpecial bool
		specialCheck func(t *testing.T, result []float64)
	}{
		{
			name:        "Empty series",
			series:      []float64{},
			d:           0.5,
			threshold:   1e-3,
			expectedLen: 0,
			expectedErr: true,
		},
		{
			name:        "Invalid threshold",
			series:      []float64{1.0, 2.0, 3.0},
			d:           0.5,
			threshold:   0.0,
			expectedLen: 0,
			expectedErr: true,
		},
		{
			name:        "Valid series with d=0.5",
			series:      []float64{1.0, 2.0, 3.0, 4.0, 5.0},
			d:           0.5,
			threshold:   1e-3,
			expectedLen: 5,
			expectedErr: false,
		},
		{
			name:        "First differencing (d=1.0)",
			series:      []float64{1.0, 3.0, 6.0, 10.0},
			d:           1.0,
			threshold:   1e-3,
			expectedLen: 4,
			expectedErr: false,
			checkSpecial: true,
			specialCheck: func(t *testing.T, result []float64) {
				// With d=1.0, we should get first differences
				// [1, 3, 6, 10] -> [1, 3-1=2, 6-3=3, 10-6=4]
				expected := []float64{1.0, 2.0, 3.0, 4.0}
				for i, exp := range expected {
					if math.Abs(result[i]-exp) > 1e-6 {
						t.Errorf("result[%d] = %f, expected %f", i, result[i], exp)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FFD(tt.series, tt.d, tt.threshold)

			// Check if error status matches expectation
			if (err != nil) != tt.expectedErr {
				t.Errorf("expected error: %v, got error: %v", tt.expectedErr, err != nil)
				return
			}

			// Skip remaining checks if we expected an error
			if tt.expectedErr {
				return
			}

			// Check result length
			if len(result) != tt.expectedLen {
				t.Errorf("expected result length %d, got %d", tt.expectedLen, len(result))
			}

			// Run special checks if needed
			if tt.checkSpecial && tt.specialCheck != nil {
				tt.specialCheck(t, result)
			}
		})
	}
}

func TestFractionalDiffAlgorithm_Interface(t *testing.T) {
	// Create an instance of the algorithm
	alg, err := Create(AlgorithmTypeFractionalDiff)
	if err != nil {
		t.Fatalf("Failed to create algorithm: %v", err)
	}

	// Test Name() method
	name := alg.Name()
	if name != "Fractional Differentiation" {
		t.Errorf("Name() = %v, want %v", name, "Fractional Differentiation")
	}

	// Test Type() method
	algType := alg.Type()
	if algType != AlgorithmTypeFractionalDiff {
		t.Errorf("Type() = %v, want %v", algType, AlgorithmTypeFractionalDiff)
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
		"d",
		"threshold",
		"window_size",
		"use_fixed_width",
	}

	for _, param := range requiredParams {
		if _, ok := params[param]; !ok {
			t.Errorf("ParameterDescription() missing required parameter: %s", param)
		}
	}
}

func TestFractionalDiffAlgorithm_Configure(t *testing.T) {
	// Create an instance of the algorithm
	alg, err := Create(AlgorithmTypeFractionalDiff)
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
					"d":               0.4,
					"threshold":       1e-4,
					"window_size":     20,
					"use_fixed_width": 1.0,
				},
			},
			wantErr: false,
		},
		{
			name: "Invalid d parameter",
			config: AlgorithmConfig{
				AdditionalParams: map[string]float64{
					"d": 1.5, // d must be between 0 and 1
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid threshold",
			config: AlgorithmConfig{
				AdditionalParams: map[string]float64{
					"threshold": -0.1, // threshold must be positive
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid window size",
			config: AlgorithmConfig{
				AdditionalParams: map[string]float64{
					"window_size": 0, // window_size must be at least 1
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

func TestFractionalDiffAlgorithm_Process(t *testing.T) {
	// Create test market data
	currentData := &types.MarketData{
		Symbol:    "AAPL",
		Price:     150.0,
		High24h:   155.0,
		Low24h:    145.0,
		Volume24h: 1000000,
		Change24h: 2.5,
	}

	// Create historical data with a trend
	historicalData := make([]types.MarketData, 50)
	for i := range historicalData {
		// Create a linear trend
		price := 100.0 + float64(i)

		historicalData[i] = types.MarketData{
			Symbol:    "AAPL",
			Price:     price,
			High24h:   price + 2.0,
			Low24h:    price - 2.0,
			Volume24h: 1000000,
			Change24h: 1.0,
		}
	}

	// Create an instance of the algorithm
	alg, err := Create(AlgorithmTypeFractionalDiff)
	if err != nil {
		t.Fatalf("Failed to create algorithm: %v", err)
	}

	// Configure the algorithm
	config := AlgorithmConfig{
		AdditionalParams: map[string]float64{
			"d":               0.5,
			"threshold":       1e-4,
			"window_size":     10,
			"use_fixed_width": 0,
		},
	}

	err = alg.Configure(config)
	if err != nil {
		t.Fatalf("Failed to configure algorithm: %v", err)
	}

	// Test Process method
	result, err := alg.Process("AAPL", currentData, historicalData)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	// Verify result
	if result == nil {
		t.Error("Process() returned nil result")
	} else {
		// For a data transformation algorithm, we expect a hold signal
		if result.Signal != "hold" {
			t.Errorf("Process() expected 'hold' signal, got %s", result.Signal)
		}

		// Verify explanation is not empty
		if result.Explanation == "" {
			t.Error("Process() result has empty explanation")
		}

		t.Logf("Algorithm explanation: %s", result.Explanation)
	}

	// Try the fixed width version
	config.AdditionalParams["use_fixed_width"] = 1.0
	err = alg.Configure(config)
	if err != nil {
		t.Fatalf("Failed to configure algorithm: %v", err)
	}

	result, err = alg.Process("AAPL", currentData, historicalData)
	if err != nil {
		t.Fatalf("Process() with fixed width error = %v", err)
	}

	if result == nil {
		t.Error("Process() with fixed width returned nil result")
	} else {
		t.Logf("Fixed width algorithm explanation: %s", result.Explanation)
	}
}

func TestFindOptimalD(t *testing.T) {
	// Create a non-stationary series (simple trend)
	trend := make([]float64, 100)
	for i := range trend {
		trend[i] = float64(i)
	}

	// Create a stationary series (random noise)
	stationary := make([]float64, 100)
	for i := range stationary {
		stationary[i] = 5.0 + (math.Sin(float64(i)*0.1) * 0.5)
	}

	tests := []struct {
		name       string
		series     []float64
		dVals      []float64
		threshold  float64
		expectLowD bool
	}{
		{
			name:       "Trend series",
			series:     trend,
			dVals:      []float64{0.1, 0.3, 0.5, 0.7, 0.9, 1.0},
			threshold:  1e-4,
			expectLowD: false, // Expect high d value to achieve stationarity
		},
		{
			name:       "Already stationary series",
			series:     stationary,
			dVals:      []float64{0.1, 0.3, 0.5, 0.7, 0.9, 1.0},
			threshold:  1e-4,
			expectLowD: true, // Expect low d value since series is already stationary
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := FindOptimalD(tt.series, tt.dVals, tt.threshold)
			if err != nil {
				t.Logf("FindOptimalD() error = %v", err)
			}

			if tt.expectLowD {
				if d > 0.5 {
					t.Errorf("Expected low d value (<= 0.5) for stationary series, got %f", d)
				}
			} else {
				if d < 0.5 && err == nil {
					t.Errorf("Expected high d value (>= 0.5) for non-stationary series, got %f", d)
				}
			}

			t.Logf("Optimal d for %s: %f", tt.name, d)
		})
	}
}