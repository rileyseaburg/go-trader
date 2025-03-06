package algo

import (
	"math"
	"testing"

	"github.com/rileyseaburg/go-trader/types"
)

func TestPositionSizingAlgorithm_Interface(t *testing.T) {
	// Create an instance of the algorithm
	alg, err := Create(AlgorithmTypePositionSizing)
	if err != nil {
		t.Fatalf("Failed to create algorithm: %v", err)
	}

	// Test Name() method
	name := alg.Name()
	if name != "Advanced Position Sizing" {
		t.Errorf("Name() = %v, want %v", name, "Advanced Position Sizing")
	}

	// Test Type() method
	algType := alg.Type()
	if algType != AlgorithmTypePositionSizing {
		t.Errorf("Type() = %v, want %v", algType, AlgorithmTypePositionSizing)
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
		"max_size",
		"risk_fraction",
		"use_vol_adjustment",
		"vol_lookback",
		"max_drawdown",
	}

	for _, param := range requiredParams {
		if _, ok := params[param]; !ok {
			t.Errorf("ParameterDescription() missing required parameter: %s", param)
		}
	}
}

func TestPositionSizingAlgorithm_Configure(t *testing.T) {
	// Create an instance of the algorithm
	alg, err := Create(AlgorithmTypePositionSizing)
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
					"max_size":           0.1,
					"risk_fraction":      0.2,
					"use_vol_adjustment": 1.0,
					"vol_lookback":       30,
					"max_drawdown":       0.05,
					"use_meta_labeling":  1.0,
				},
			},
			wantErr: false,
		},
		{
			name: "Invalid max_size (too high)",
			config: AlgorithmConfig{
				AdditionalParams: map[string]float64{
					"max_size": 1.5, // Must be between 0 and 1
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid risk_fraction (negative)",
			config: AlgorithmConfig{
				AdditionalParams: map[string]float64{
					"risk_fraction": -0.1, // Must be between 0 and 1
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid vol_lookback (zero)",
			config: AlgorithmConfig{
				AdditionalParams: map[string]float64{
					"vol_lookback": 0, // Must be at least 1
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid max_drawdown (too high)",
			config: AlgorithmConfig{
				AdditionalParams: map[string]float64{
					"max_drawdown": 0.6, // Must be between 0 and 0.5
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

func TestKellyOptimalF(t *testing.T) {
	tests := []struct {
		name         string
		winProb      float64
		winLossRatio float64
		expected     float64
	}{
		{
			name:         "50% win rate, 1:1 ratio",
			winProb:      0.5,
			winLossRatio: 1.0,
			expected:     0.0, // Break even, no edge
		},
		{
			name:         "60% win rate, 1:1 ratio",
			winProb:      0.6,
			winLossRatio: 1.0,
			expected:     0.2, // 20% optimal Kelly fraction
		},
		{
			name:         "40% win rate, 2:1 ratio",
			winProb:      0.4,
			winLossRatio: 2.0,
			expected:     0.1, // 10% optimal Kelly fraction
		},
		{
			name:         "30% win rate, 3:1 ratio",
			winProb:      0.3,
			winLossRatio: 3.0,
			expected:     0.1, // 10% optimal Kelly fraction
		},
		{
			name:         "30% win rate, 1:1 ratio (negative expectation)",
			winProb:      0.3,
			winLossRatio: 1.0,
			expected:     -0.4, // Negative Kelly (don't bet)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := KellyOptimalF(tt.winProb, tt.winLossRatio)
			
			// Check result with a small epsilon for floating point comparison
			if math.Abs(result-tt.expected) > 1e-6 {
				t.Errorf("KellyOptimalF(%v, %v) = %v, want %v", 
					tt.winProb, tt.winLossRatio, result, tt.expected)
			}
		})
	}
}

func TestAdjustPositionForVolatility(t *testing.T) {
	tests := []struct {
		name        string
		baseSize    float64
		volatility  float64
		baselineVol float64
		expected    float64
	}{
		{
			name:        "Normal volatility",
			baseSize:    0.2,
			volatility:  0.01,
			baselineVol: 0.01,
			expected:    0.2, // No adjustment
		},
		{
			name:        "High volatility (reduced size)",
			baseSize:    0.2,
			volatility:  0.02,
			baselineVol: 0.01,
			expected:    0.1, // Reduced by half
		},
		{
			name:        "Very high volatility (reduced size with floor)",
			baseSize:    0.2,
			volatility:  0.04,
			baselineVol: 0.01,
			expected:    0.1, // Reduced but minimum 0.5 scaling factor applied
		},
		{
			name:        "Low volatility (increased size)",
			baseSize:    0.2,
			volatility:  0.005,
			baselineVol: 0.01,
			expected:    0.4, // Doubled
		},
		{
			name:        "Very low volatility (increased size with ceiling)",
			baseSize:    0.2,
			volatility:  0.001,
			baselineVol: 0.01,
			expected:    0.4, // Increased but maximum 2.0 scaling factor applied
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AdjustPositionForVolatility(tt.baseSize, tt.volatility, tt.baselineVol)
			
			// Check result with a small epsilon for floating point comparison
			if math.Abs(result-tt.expected) > 1e-6 {
				t.Errorf("AdjustPositionForVolatility(%v, %v, %v) = %v, want %v", 
					tt.baseSize, tt.volatility, tt.baselineVol, result, tt.expected)
			}
		})
	}
}

func TestCalculateDiversifiedPositionSize(t *testing.T) {
	tests := []struct {
		name          string
		baseSize      float64
		numPositions  int
		avgCorrelation float64
		expected      float64
	}{
		{
			name:          "Single position",
			baseSize:      0.2,
			numPositions:  1,
			avgCorrelation: 0.0,
			expected:      0.2, // No diversification
		},
		{
			name:          "Multiple uncorrelated positions",
			baseSize:      0.2,
			numPositions:  4,
			avgCorrelation: 0.0,
			expected:      0.1, // Scaled by 1/sqrt(4) = 0.5
		},
		{
			name:          "Multiple partially correlated positions",
			baseSize:      0.2,
			numPositions:  4,
			avgCorrelation: 0.5,
			expected:      0.129, // Scaled by 1/sqrt(2.5) = ~0.632
		},
		{
			name:          "Multiple highly correlated positions",
			baseSize:      0.2,
			numPositions:  4,
			avgCorrelation: 0.9,
			expected:      0.2, // Effective N is close to 1, so little diversification
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateDiversifiedPositionSize(tt.baseSize, tt.numPositions, tt.avgCorrelation)
			
			// Check result with a small epsilon for floating point comparison
			if math.Abs(result-tt.expected) > 0.001 {
				t.Errorf("CalculateDiversifiedPositionSize(%v, %v, %v) = %v, want %v", 
					tt.baseSize, tt.numPositions, tt.avgCorrelation, result, tt.expected)
			}
		})
	}
}

func TestPositionSizingAlgorithm_calculatePositionSize(t *testing.T) {
	// Create an instance of the algorithm
	alg := &PositionSizingAlgorithm{
		BaseAlgorithm:     BaseAlgorithm{},
		maxSize:           0.2,
		riskFraction:      0.3,
		useVolAdjustment:  true,
		volLookback:       20,
		maxDrawdown:       0.1,
		primaryAlgorithm:  AlgorithmTypeSequentialBootstrap,
		metaLabeling:      true,
	}

	tests := []struct {
		name        string
		signal      string
		confidence  float64
		volatility  float64
		currentPrice float64
		expectedSize float64
		expectError bool
	}{
		{
			name:         "Buy signal with high confidence, normal volatility",
			signal:       "buy",
			confidence:   0.8,
			volatility:   0.01,
			currentPrice: 100.0,
			expectedSize: 0.18, // 0.8 * 0.3 - constrained by max size 0.2
			expectError:  false,
		},
		{
			name:         "Sell signal with medium confidence, high volatility",
			signal:       "sell",
			confidence:   0.6,
			volatility:   0.02,
			currentPrice: 100.0,
			expectedSize: 0.03, // (0.6 * 0.3) * 0.5 (vol adjustment)
			expectError:  false,
		},
		{
			name:         "Invalid signal",
			signal:       "invalid",
			confidence:   0.7,
			volatility:   0.01,
			currentPrice: 100.0,
			expectedSize: 0.0,
			expectError:  true,
		},
		{
			name:         "Invalid confidence (too high)",
			signal:       "buy",
			confidence:   1.2,
			volatility:   0.01,
			currentPrice: 100.0,
			expectedSize: 0.0,
			expectError:  true,
		},
		{
			name:         "Invalid volatility (negative)",
			signal:       "buy",
			confidence:   0.7,
			volatility:   -0.01,
			currentPrice: 100.0,
			expectedSize: 0.0,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := alg.calculatePositionSize(tt.signal, tt.confidence, tt.volatility, tt.currentPrice)
			
			if (err != nil) != tt.expectError {
				t.Errorf("calculatePositionSize() error = %v, expectError %v", err, tt.expectError)
				return
			}
			
			if tt.expectError {
				return
			}
			
			if math.Abs(result.Size-tt.expectedSize) > 0.01 {
				t.Errorf("calculatePositionSize() size = %v, want %v", result.Size, tt.expectedSize)
			}
			
			// Verify other fields
			if result.Signal != tt.signal {
				t.Errorf("calculatePositionSize() signal = %v, want %v", result.Signal, tt.signal)
			}
			
			if result.Confidence != tt.confidence {
				t.Errorf("calculatePositionSize() confidence = %v, want %v", result.Confidence, tt.confidence)
			}
			
			if !result.VolAdjusted && alg.useVolAdjustment {
				t.Errorf("calculatePositionSize() volAdjusted = %v, expected true", result.VolAdjusted)
			}
		})
	}
}

func TestPositionSizingAlgorithm_Process(t *testing.T) {
	// Create an instance of the algorithm
	alg, err := Create(AlgorithmTypePositionSizing)
	if err != nil {
		t.Fatalf("Failed to create algorithm: %v", err)
	}

	// Configure the algorithm
	config := AlgorithmConfig{
		AdditionalParams: map[string]float64{
			"max_size":           0.1,
			"risk_fraction":      0.2,
			"use_vol_adjustment": 1.0,
			"vol_lookback":       10,
			"max_drawdown":       0.05,
			"use_meta_labeling":  0.0, // Disable meta-labeling for testing
		},
	}

	err = alg.Configure(config)
	if err != nil {
		t.Fatalf("Failed to configure algorithm: %v", err)
	}

	// Create sample market data
	currentData := &types.MarketData{
		Symbol:    "AAPL",
		Price:     150.0,
		High24h:   155.0,
		Low24h:    145.0,
		Volume24h: 1000000,
		Change24h: 2.5,
	}

	// Create historical data with an uptrend
	historicalData := make([]types.MarketData, 30)
	for i := range historicalData {
		price := 100.0 + float64(i)
		historicalData[i] = types.MarketData{
			Symbol:    "AAPL",
			Price:     price,
			High24h:   price + 2.0,
			Low24h:    price - 2.0,
			Volume24h: 1000000,
			Change24h: 0.5,
		}
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
		// Since the sequential bootstrap should generate a signal with the uptrend data,
		// we expect a non-hold signal
		if result.Signal == "" {
			t.Error("Process() returned empty signal")
		}

		if result.Explanation == "" {
			t.Error("Process() returned empty explanation")
		}

		t.Logf("Signal: %s with confidence %.2f", result.Signal, result.Confidence)
		t.Logf("Explanation: %s", result.Explanation)
	}
}