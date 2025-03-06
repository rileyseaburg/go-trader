package algo

import (
	"math"
	"testing"
	"time"

	"github.com/rileyseaburg/go-trader/types"
)

func TestDailyVolatility(t *testing.T) {
	tests := []struct {
		name         string
		prices       []float64
		span         int
		expectedVol  float64
		expectErr    bool
	}{
		{
			name:         "Empty prices",
			prices:       []float64{},
			span:         20,
			expectedVol:  0,
			expectErr:    true,
		},
		{
			name:         "Single price",
			prices:       []float64{100},
			span:         20,
			expectedVol:  0,
			expectErr:    true,
		},
		{
			name:         "Invalid span",
			prices:       []float64{100, 101, 102},
			span:         0,
			expectedVol:  0,
			expectErr:    true,
		},
		{
			name:         "Constant prices",
			prices:       []float64{100, 100, 100, 100, 100},
			span:         3,
			expectedVol:  0,
			expectErr:    false,
		},
		{
			name:         "Uptrending prices",
			prices:       []float64{100, 101, 102, 103, 104},
			span:         3,
			expectedVol:  0.01, // Approximate expected value
			expectErr:    false,
		},
		{
			name:         "Volatile prices",
			prices:       []float64{100, 105, 95, 110, 90},
			span:         3,
			expectedVol:  0.05, // Approximate expected value
			expectErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vol, err := DailyVolatility(tt.prices, tt.span)
			
			if (err != nil) != tt.expectErr {
				t.Errorf("DailyVolatility() error = %v, expectErr %v", err, tt.expectErr)
				return
			}
			
			if tt.expectErr {
				return
			}
			
			// For zero expected volatility, check if result is close to zero
			if tt.expectedVol == 0 {
				if vol > 1e-6 {
					t.Errorf("DailyVolatility() = %v, want close to 0", vol)
				}
				return
			}
			
			// For non-zero volatility, check if it's in the right ballpark
			// This is approximate since the exact calculation depends on the EWMA implementation
			if vol == 0 || math.Abs(vol/tt.expectedVol - 1) > 1.0 {
				t.Errorf("DailyVolatility() = %v, expected approximately %v", vol, tt.expectedVol)
			}
		})
	}
}

func TestApplyTripleBarrier(t *testing.T) {
	// Create test times
	now := time.Now()
	times := make([]time.Time, 10)
	for i := range times {
		times[i] = now.Add(time.Duration(i) * 24 * time.Hour)
	}
	
	tests := []struct {
		name            string
		prices          []float64
		times           []time.Time
		volatility      float64
		config          TripleBarrierConfig
		expectedLen     int
		expectedLabelAt int
		expectedLabel   BarrierLabel
		expectedBarrier BarrierType
		expectErr       bool
	}{
		{
			name:        "Empty prices",
			prices:      []float64{},
			times:       []time.Time{},
			volatility:  0.01,
			config:      TripleBarrierConfig{ProfitTaking: 2, StopLoss: 1, TimeHorizon: 5},
			expectedLen: 0,
			expectErr:   true,
		},
		{
			name:        "Length mismatch",
			prices:      []float64{100, 101, 102},
			times:       []time.Time{now, now.Add(24 * time.Hour)},
			volatility:  0.01,
			config:      TripleBarrierConfig{ProfitTaking: 2, StopLoss: 1, TimeHorizon: 5},
			expectedLen: 0,
			expectErr:   true,
		},
		{
			name:        "Zero volatility",
			prices:      []float64{100, 101, 102},
			times:       []time.Time{now, now.Add(24 * time.Hour), now.Add(48 * time.Hour)},
			volatility:  0,
			config:      TripleBarrierConfig{ProfitTaking: 2, StopLoss: 1, TimeHorizon: 5},
			expectedLen: 0,
			expectErr:   true,
		},
		{
			name:            "Uptrend hits upper barrier",
			prices:          []float64{100, 102, 104, 106, 108, 110},
			times:           times[:6],
			volatility:      0.01,
			config:          TripleBarrierConfig{ProfitTaking: 3, StopLoss: 2, TimeHorizon: 5},
			expectedLen:     5, // One result per entry point, except the last
			expectedLabelAt: 0, // Check first label
			expectedLabel:   BarrierLabelBuy,
			expectedBarrier: BarrierTypeUpper,
			expectErr:       false,
		},
		{
			name:            "Downtrend hits lower barrier",
			prices:          []float64{100, 98, 96, 94, 92, 90},
			times:           times[:6],
			volatility:      0.01,
			config:          TripleBarrierConfig{ProfitTaking: 3, StopLoss: 2, TimeHorizon: 5},
			expectedLen:     5,
			expectedLabelAt: 0,
			expectedLabel:   BarrierLabelSell,
			expectedBarrier: BarrierTypeLower,
			expectErr:       false,
		},
		{
			name:            "Sideways hits time barrier",
			prices:          []float64{100, 100.5, 100.2, 100.7, 100.3, 100.6},
			times:           times[:6],
			volatility:      0.01,
			config:          TripleBarrierConfig{ProfitTaking: 10, StopLoss: 10, TimeHorizon: 2},
			expectedLen:     5,
			expectedLabelAt: 0,
			expectedLabel:   BarrierLabelBuy, // Slightly positive ending
			expectedBarrier: BarrierTypeTime,
			expectErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := ApplyTripleBarrier(tt.prices, tt.times, tt.volatility, tt.config)
			
			if (err != nil) != tt.expectErr {
				t.Errorf("ApplyTripleBarrier() error = %v, expectErr %v", err, tt.expectErr)
				return
			}
			
			if tt.expectErr {
				return
			}
			
			if len(results) != tt.expectedLen {
				t.Errorf("ApplyTripleBarrier() returned %d results, expected %d", len(results), tt.expectedLen)
				return
			}
			
			if len(results) > 0 && tt.expectedLabelAt < len(results) {
				result := results[tt.expectedLabelAt]
				
				if result.Label != tt.expectedLabel {
					t.Errorf("Label at index %d = %v, expected %v", tt.expectedLabelAt, result.Label, tt.expectedLabel)
				}
				
				if result.BarrierHit != tt.expectedBarrier {
					t.Errorf("BarrierHit at index %d = %v, expected %v", tt.expectedLabelAt, result.BarrierHit, tt.expectedBarrier)
				}
				
				// Check relationships between entry and exit prices based on label
				if result.Label == BarrierLabelBuy && result.BarrierHit == BarrierTypeUpper {
					if result.ExitPrice <= result.EntryPrice {
						t.Errorf("For upper barrier with Buy label, expected exit price > entry price, got entry=%.2f, exit=%.2f", 
							result.EntryPrice, result.ExitPrice)
					}
				} else if result.Label == BarrierLabelSell && result.BarrierHit == BarrierTypeLower {
					if result.ExitPrice >= result.EntryPrice {
						t.Errorf("For lower barrier with Sell label, expected exit price < entry price, got entry=%.2f, exit=%.2f", 
							result.EntryPrice, result.ExitPrice)
					}
				}
			}
		})
	}
}

func TestGetMetaLabels(t *testing.T) {
	now := time.Now()
	
	tests := []struct {
		name           string
		barrierResults []*BarrierResult
		expectedLabels []bool
	}{
		{
			name:           "Empty results",
			barrierResults: []*BarrierResult{},
			expectedLabels: nil,
		},
		{
			name: "Successful buy trade",
			barrierResults: []*BarrierResult{
				{
					Label:      BarrierLabelBuy,
					EntryPrice: 100,
					ExitPrice:  110,
					EntryTime:  now,
					ExitTime:   now.Add(24 * time.Hour),
					BarrierHit: BarrierTypeUpper,
				},
			},
			expectedLabels: []bool{true},
		},
		{
			name: "Unsuccessful buy trade",
			barrierResults: []*BarrierResult{
				{
					Label:      BarrierLabelBuy,
					EntryPrice: 100,
					ExitPrice:  95,
					EntryTime:  now,
					ExitTime:   now.Add(24 * time.Hour),
					BarrierHit: BarrierTypeLower,
				},
			},
			expectedLabels: []bool{false},
		},
		{
			name: "Successful sell trade",
			barrierResults: []*BarrierResult{
				{
					Label:      BarrierLabelSell,
					EntryPrice: 100,
					ExitPrice:  90,
					EntryTime:  now,
					ExitTime:   now.Add(24 * time.Hour),
					BarrierHit: BarrierTypeLower,
				},
			},
			expectedLabels: []bool{true},
		},
		{
			name: "Unsuccessful sell trade",
			barrierResults: []*BarrierResult{
				{
					Label:      BarrierLabelSell,
					EntryPrice: 100,
					ExitPrice:  105,
					EntryTime:  now,
					ExitTime:   now.Add(24 * time.Hour),
					BarrierHit: BarrierTypeUpper,
				},
			},
			expectedLabels: []bool{false},
		},
		{
			name: "Mixed trades",
			barrierResults: []*BarrierResult{
				{
					Label:      BarrierLabelBuy,
					EntryPrice: 100,
					ExitPrice:  110,
					EntryTime:  now,
					ExitTime:   now.Add(24 * time.Hour),
					BarrierHit: BarrierTypeUpper,
				},
				{
					Label:      BarrierLabelSell,
					EntryPrice: 110,
					ExitPrice:  115,
					EntryTime:  now.Add(24 * time.Hour),
					ExitTime:   now.Add(48 * time.Hour),
					BarrierHit: BarrierTypeTime,
				},
				{
					Label:      BarrierLabelBuy,
					EntryPrice: 115,
					ExitPrice:  105,
					EntryTime:  now.Add(48 * time.Hour),
					ExitTime:   now.Add(72 * time.Hour),
					BarrierHit: BarrierTypeLower,
				},
			},
			expectedLabels: []bool{true, false, false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			labels := GetMetaLabels(tt.barrierResults)
			
			if len(labels) != len(tt.expectedLabels) {
				t.Errorf("GetMetaLabels() returned %d labels, expected %d", len(labels), len(tt.expectedLabels))
				return
			}
			
			for i := range labels {
				if labels[i] != tt.expectedLabels[i] {
					t.Errorf("Label at index %d = %v, expected %v", i, labels[i], tt.expectedLabels[i])
				}
			}
		})
	}
}

func TestTripleBarrierAlgorithm_Interface(t *testing.T) {
	// Create an instance of the algorithm
	alg, err := Create(AlgorithmTypeTripleBarrier)
	if err != nil {
		t.Fatalf("Failed to create algorithm: %v", err)
	}

	// Test Name() method
	name := alg.Name()
	if name != "Triple Barrier Method" {
		t.Errorf("Name() = %v, want %v", name, "Triple Barrier Method")
	}

	// Test Type() method
	algType := alg.Type()
	if algType != AlgorithmTypeTripleBarrier {
		t.Errorf("Type() = %v, want %v", algType, AlgorithmTypeTripleBarrier)
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
		"profit_taking",
		"stop_loss",
		"time_horizon",
		"volatility_lookback",
	}

	for _, param := range requiredParams {
		if _, ok := params[param]; !ok {
			t.Errorf("ParameterDescription() missing required parameter: %s", param)
		}
	}
}

func TestTripleBarrierAlgorithm_Configure(t *testing.T) {
	// Create an instance of the algorithm
	alg, err := Create(AlgorithmTypeTripleBarrier)
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
					"profit_taking":       3.0,
					"stop_loss":           1.5,
					"time_horizon":        10,
					"volatility_lookback": 30,
				},
			},
			wantErr: false,
		},
		{
			name: "Invalid profit taking",
			config: AlgorithmConfig{
				AdditionalParams: map[string]float64{
					"profit_taking": -1.0,
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid stop loss",
			config: AlgorithmConfig{
				AdditionalParams: map[string]float64{
					"stop_loss": 0,
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid time horizon",
			config: AlgorithmConfig{
				AdditionalParams: map[string]float64{
					"time_horizon": 0,
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid volatility lookback",
			config: AlgorithmConfig{
				AdditionalParams: map[string]float64{
					"volatility_lookback": 0,
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

func TestTripleBarrierAlgorithm_Process(t *testing.T) {
	// Create an instance of the algorithm
	alg, err := Create(AlgorithmTypeTripleBarrier)
	if err != nil {
		t.Fatalf("Failed to create algorithm: %v", err)
	}

	// Configure the algorithm
	config := AlgorithmConfig{
		AdditionalParams: map[string]float64{
			"profit_taking":       2.0,
			"stop_loss":           1.0,
			"time_horizon":        5,
			"volatility_lookback": 10,
		},
	}

	err = alg.Configure(config)
	if err != nil {
		t.Fatalf("Failed to configure algorithm: %v", err)
	}

	// Test scenarios
	tests := []struct {
		name          string
		currentData   *types.MarketData
		historicalData []types.MarketData
		expectErr     bool
	}{
		{
			name: "Insufficient historical data",
			currentData: &types.MarketData{
				Symbol: "AAPL",
				Price:  150.0,
			},
			historicalData: make([]types.MarketData, 5), // Less than volatility window
			expectErr:     true,
		},
		{
			name: "Uptrending market",
			currentData: &types.MarketData{
				Symbol: "AAPL",
				Price:  150.0,
			},
			historicalData: createTestMarketData("AAPL", 20, func(i int) float64 {
				return 100.0 + float64(i)
			}),
			expectErr: false,
		},
		{
			name: "Downtrending market",
			currentData: &types.MarketData{
				Symbol: "AAPL",
				Price:  80.0,
			},
			historicalData: createTestMarketData("AAPL", 20, func(i int) float64 {
				return 100.0 - float64(i)
			}),
			expectErr: false,
		},
		{
			name: "Volatile market",
			currentData: &types.MarketData{
				Symbol: "AAPL",
				Price:  105.0,
			},
			historicalData: createTestMarketData("AAPL", 20, func(i int) float64 {
				return 100.0 + math.Sin(float64(i)*0.5)*10.0
			}),
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := alg.Process(tt.currentData.Symbol, tt.currentData, tt.historicalData)

			if (err != nil) != tt.expectErr {
				t.Errorf("Process() error = %v, expectErr %v", err, tt.expectErr)
				return
			}

			if tt.expectErr {
				return
			}

			// Verify basic result properties
			if result == nil {
				t.Fatal("Process() returned nil result")
			}

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
		})
	}
}

// Helper function to create test market data
func createTestMarketData(symbol string, n int, priceFn func(int) float64) []types.MarketData {
	data := make([]types.MarketData, n)
	for i := range data {
		price := priceFn(i)
		data[i] = types.MarketData{
			Symbol:    symbol,
			Price:     price,
			High24h:   price * 1.02,
			Low24h:    price * 0.98,
			Volume24h: 1000000,
			Change24h: 0.5,
		}
	}
	return data
}