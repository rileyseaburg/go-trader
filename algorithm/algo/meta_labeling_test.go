package algo

import (
	"math"
	"testing"

	"github.com/rileyseaburg/go-trader/types"
)

func TestMetaLabelingAlgorithm_Interface(t *testing.T) {
	// Create an instance of the algorithm
	alg, err := Create(AlgorithmTypeMetaLabeling)
	if err != nil {
		t.Fatalf("Failed to create algorithm: %v", err)
	}

	// Test Name() method
	name := alg.Name()
	if name != "Meta-Labeling" {
		t.Errorf("Name() = %v, want %v", name, "Meta-Labeling")
	}

	// Test Type() method
	algType := alg.Type()
	if algType != AlgorithmTypeMetaLabeling {
		t.Errorf("Type() = %v, want %v", algType, AlgorithmTypeMetaLabeling)
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
		"confidence_threshold",
		"model_type",
		"primary_algorithm",
		"use_price_features",
		"use_volume_features",
	}

	for _, param := range requiredParams {
		if _, ok := params[param]; !ok {
			t.Errorf("ParameterDescription() missing required parameter: %s", param)
		}
	}
}

func TestMetaLabelingAlgorithm_Configure(t *testing.T) {
	// Create an instance of the algorithm
	alg, err := Create(AlgorithmTypeMetaLabeling)
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
					"confidence_threshold":    0.7,
					"use_price_features":      1.0,
					"use_volume_features":     1.0,
					"use_volatility_features": 1.0,
					"use_technical_features":  1.0,
				},
			},
			wantErr: false,
		},
		{
			name: "Invalid confidence threshold",
			config: AlgorithmConfig{
				AdditionalParams: map[string]float64{
					"confidence_threshold": 1.5, // Threshold must be between 0 and 1
				},
			},
			wantErr: true,
		},
		{
			name: "Disable all features",
			config: AlgorithmConfig{
				AdditionalParams: map[string]float64{
					"use_price_features":      0.0,
					"use_volume_features":     0.0,
					"use_volatility_features": 0.0,
					"use_technical_features":  0.0,
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

func TestMetaLabelingAlgorithm_Process(t *testing.T) {
	// Create current market data
	currentData := &types.MarketData{
		Symbol:    "AAPL",
		Price:     150.0,
		High24h:   155.0,
		Low24h:    145.0,
		Volume24h: 1000000,
		Change24h: 2.5,
	}

	// Create historical data with a trend
	historicalData := make([]types.MarketData, 30)
	for i := range historicalData {
		// Create a price series with a positive trend
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

	// Create an instance of the algorithm
	alg, err := Create(AlgorithmTypeMetaLabeling)
	if err != nil {
		t.Fatalf("Failed to create algorithm: %v", err)
	}

	// Configure the algorithm
	config := AlgorithmConfig{
		AdditionalParams: map[string]float64{
			"confidence_threshold":    0.7,
			"use_price_features":      1.0,
			"use_volume_features":     1.0,
			"use_volatility_features": 1.0,
			"use_technical_features":  1.0,
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
		// Verify that result has expected fields
		if result.Signal == "" {
			t.Error("Process() result has empty signal")
		}

		if result.OrderType == "" {
			t.Error("Process() result has empty order type")
		}

		if result.Confidence <= 0 || result.Confidence > 1 {
			t.Errorf("Process() result has invalid confidence: %v", result.Confidence)
		}

		if result.Explanation == "" {
			t.Error("Process() result has empty explanation")
		}

		t.Logf("Algorithm produced signal: %s with confidence %.2f", result.Signal, result.Confidence)
		t.Logf("Explanation: %s", result.Explanation)
	}
}

func TestMetaLabelingFeatureExtraction(t *testing.T) {
	// Create an instance of the algorithm
	alg, err := Create(AlgorithmTypeMetaLabeling)
	if err != nil {
		t.Fatalf("Failed to create algorithm: %v", err)
	}

	// Cast to MetaLabelingAlgorithm to access extractFeatures method
	metaAlg, ok := alg.(*MetaLabelingAlgorithm)
	if !ok {
		t.Fatalf("Failed to cast to MetaLabelingAlgorithm")
	}

	// Create test data
	currentData := &types.MarketData{
		Symbol:    "AAPL",
		Price:     100.0,
		High24h:   105.0,
		Low24h:    95.0,
		Volume24h: 1000000,
		Change24h: 1.0,
	}

	// Create historical data
	historicalData := make([]types.MarketData, 30)
	for i := range historicalData {
		price := 90.0 + float64(i)
		historicalData[i] = types.MarketData{
			Symbol:    "AAPL",
			Price:     price,
			High24h:   price + 2.0,
			Low24h:    price - 2.0,
			Volume24h: 900000 + float64(i*10000),
			Change24h: 0.5,
		}
	}

	// Configure the algorithm
	err = metaAlg.Configure(AlgorithmConfig{})
	if err != nil {
		t.Fatalf("Failed to configure algorithm: %v", err)
	}

	// Extract features
	features := metaAlg.extractFeatures(currentData, historicalData)

	// Verify features
	if len(features) == 0 {
		t.Error("extractFeatures() returned empty feature vector")
	}

	// All features should be in the range [0, 1] after normalization
	for i, feature := range features {
		if feature < 0 || feature > 1 {
			t.Errorf("Feature %d has value %.4f, expected value in range [0, 1]", i, feature)
		}
	}

	t.Logf("Extracted %d features with values: %v", len(features), features)
}

func TestTechnicalIndicators(t *testing.T) {
	// Create test prices for RSI, MACD, and Bollinger Bands
	upTrend := []float64{100, 102, 104, 103, 105, 107, 109, 108, 110, 112, 
		114, 113, 115, 117, 119, 118, 120, 122, 124, 123, 125, 127, 129, 128, 130}
	
	downTrend := []float64{100, 98, 96, 97, 95, 93, 91, 92, 90, 88, 
		86, 87, 85, 83, 81, 82, 80, 78, 76, 77, 75, 73, 71, 72, 70}
	
	sideways := []float64{100, 101, 99, 100, 102, 98, 99, 101, 100, 102, 
		98, 99, 101, 100, 102, 98, 99, 101, 100, 102, 98, 99, 101, 100, 102}
	
	tests := []struct {
		name        string
		prices      []float64
		expectRSI   string // high, low, or mid
		expectMACD  string // positive, negative, or neutral
		expectBB    string // upper, lower, or mid
	}{
		{
			name:       "Uptrend",
			prices:     upTrend,
			expectRSI:  "high",   // RSI should be high in an uptrend
			expectMACD: "positive", // MACD should be positive in an uptrend
			expectBB:   "upper",  // Price should be near upper BB in an uptrend
		},
		{
			name:       "Downtrend",
			prices:     downTrend,
			expectRSI:  "low",     // RSI should be low in a downtrend
			expectMACD: "negative", // MACD should be negative in a downtrend
			expectBB:   "lower",    // Price should be near lower BB in a downtrend
		},
		{
			name:       "Sideways",
			prices:     sideways,
			expectRSI:  "mid",      // RSI should be mid-range in sideways market
			expectMACD: "neutral",  // MACD should be near zero in sideways market
			expectBB:   "mid",      // Price should be near middle of BB in sideways
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate technical indicators
			rsi := calculateRSI(tt.prices, 14)
			macd := calculateMACD(tt.prices)
			bb := calculateBollingerPctB(tt.prices, 20, 2.0)
			
			// Test RSI
			switch tt.expectRSI {
			case "high":
				if rsi < 60 {
					t.Errorf("RSI = %.2f, expected > 60 for uptrend", rsi)
				}
			case "low":
				if rsi > 40 {
					t.Errorf("RSI = %.2f, expected < 40 for downtrend", rsi)
				}
			case "mid":
				if rsi < 40 || rsi > 60 {
					t.Errorf("RSI = %.2f, expected between 40-60 for sideways market", rsi)
				}
			}
			
			// Test MACD
			switch tt.expectMACD {
			case "positive":
				if macd <= 0 {
					t.Errorf("MACD = %.5f, expected positive for uptrend", macd)
				}
			case "negative":
				if macd >= 0 {
					t.Errorf("MACD = %.5f, expected negative for downtrend", macd)
				}
			case "neutral":
				if math.Abs(macd) > 0.02 {
					t.Errorf("MACD = %.5f, expected close to zero for sideways market", macd)
				}
			}
			
			// Test Bollinger Bands %B
			switch tt.expectBB {
			case "upper":
				if bb < 0.7 {
					t.Errorf("Bollinger %%B = %.2f, expected > 0.7 for uptrend", bb)
				}
			case "lower":
				if bb > 0.3 {
					t.Errorf("Bollinger %%B = %.2f, expected < 0.3 for downtrend", bb)
				}
			case "mid":
				if bb < 0.3 || bb > 0.7 {
					t.Errorf("Bollinger %%B = %.2f, expected between 0.3-0.7 for sideways market", bb)
				}
			}
			
			t.Logf("%s - RSI: %.2f, MACD: %.5f, BB%%B: %.2f", tt.name, rsi, macd, bb)
		})
	}
}