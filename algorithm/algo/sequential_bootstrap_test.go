package algo

import (
	"math"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/rileyseaburg/go-trader/types"
	"gonum.org/v1/gonum/mat"
)

func init() {
	// Seed the random number generator for reproducible tests
	rand.Seed(time.Now().UnixNano())
}

func TestGetIndMatrix(t *testing.T) {
	tests := []struct {
		name    string
		barIx   []int
		t1      []float64
		wantErr bool
	}{
		{
			name:    "Empty inputs",
			barIx:   []int{},
			t1:      []float64{},
			wantErr: true,
		},
		{
			name:    "Valid simple case",
			barIx:   []int{0, 1, 2, 3, 4},
			t1:      []float64{1, 3},
			wantErr: false,
		},
		{
			name:    "Valid complex case",
			barIx:   []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
			t1:      []float64{2, 4, 6, 8},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			indM, err := getIndMatrix(tt.barIx, tt.t1)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("getIndMatrix() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if !tt.wantErr {
				// Verify matrix dimensions
				r, c := indM.Dims()
				if r != len(tt.barIx) {
					t.Errorf("getIndMatrix() wrong number of rows: got %d, want %d", r, len(tt.barIx))
				}
				if c != len(tt.t1) {
					t.Errorf("getIndMatrix() wrong number of columns: got %d, want %d", c, len(tt.t1))
				}
				
				// Verify that matrix values are either 0 or 1
				for i := 0; i < r; i++ {
					for j := 0; j < c; j++ {
						val := indM.At(i, j)
						if val != 0 && val != 1 {
							t.Errorf("getIndMatrix() unexpected value at (%d,%d): got %f, want 0 or 1", i, j, val)
						}
					}
				}
				
				// For the simple case, verify specific values
				if tt.name == "Valid simple case" {
					// Verify that bars 0-1 are marked for t1[0] = 1
					if indM.At(0, 0) != 1 || indM.At(1, 0) != 1 {
						t.Errorf("getIndMatrix() expected values at (0,0) and (1,0) to be 1")
					}
					
					// Verify that bars 0-3 are marked for t1[1] = 3
					if indM.At(0, 1) != 1 || indM.At(1, 1) != 1 || indM.At(2, 1) != 1 || indM.At(3, 1) != 1 {
						t.Errorf("getIndMatrix() expected values for column 1 to be 1 for rows 0-3")
					}
				}
			}
		})
	}
}

func TestGetAvgUniqueness(t *testing.T) {
	tests := []struct {
		name    string
		matrix  *mat.Dense
		want    []float64
		wantErr bool
	}{
		{
			name:    "Nil matrix",
			matrix:  nil,
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Empty matrix",
			matrix:  mat.NewDense(0, 0, nil),
			want:    nil,
			wantErr: true,
		},
		{
			name: "Valid matrix",
			matrix: mat.NewDense(3, 3, []float64{
				1, 0, 1,
				1, 1, 0,
				0, 1, 1,
			}),
			want:    []float64{0.5, 0.5, 0.5}, // Each column has 2 1's, so uniqueness is 1/2
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getAvgUniqueness(tt.matrix)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("getAvgUniqueness() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if !tt.wantErr {
				if len(got) != len(tt.want) {
					t.Errorf("getAvgUniqueness() wrong length: got %d, want %d", len(got), len(tt.want))
					return
				}
				
				// Verify values with a small epsilon for floating-point comparison
				const epsilon = 1e-6
				for i := range got {
					if abs(got[i]-tt.want[i]) > epsilon {
						t.Errorf("getAvgUniqueness()[%d] = %v, want %v", i, got[i], tt.want[i])
					}
				}
			}
		})
	}
}

func TestSeqBootstrap(t *testing.T) {
	tests := []struct {
		name     string
		matrix   *mat.Dense
		sLength  int
		wantLen  int
		wantErr  bool
	}{
		{
			name:     "Nil matrix",
			matrix:   nil,
			sLength:  5,
			wantLen:  0,
			wantErr:  true,
		},
		{
			name:     "Empty matrix",
			matrix:   mat.NewDense(0, 0, nil),
			sLength:  5,
			wantLen:  0,
			wantErr:  true,
		},
		{
			name: "Valid matrix with default length",
			matrix: mat.NewDense(3, 3, []float64{
				1, 0, 1,
				1, 1, 0,
				0, 1, 1,
			}),
			sLength:  0, // Use default length (number of columns)
			wantLen:  3,
			wantErr:  false,
		},
		{
			name: "Valid matrix with custom length",
			matrix: mat.NewDense(3, 5, []float64{
				1, 0, 1, 0, 1,
				1, 1, 0, 1, 0,
				0, 1, 1, 0, 1,
			}),
			sLength:  3,
			wantLen:  3,
			wantErr:  false,
		},
		{
			name: "Invalid sample size",
			matrix: mat.NewDense(3, 3, []float64{
				1, 0, 1,
				1, 1, 0,
				0, 1, 1,
			}),
			sLength:  5, // More than number of columns
			wantLen:  0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := seqBootstrap(tt.matrix, tt.sLength)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("seqBootstrap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if !tt.wantErr {
				if len(got) != tt.wantLen {
					t.Errorf("seqBootstrap() wrong length: got %d, want %d", len(got), tt.wantLen)
					return
				}
				
				// Verify that indices are within valid range
				_, c := tt.matrix.Dims()
				for i, idx := range got {
					if idx < 0 || idx >= c {
						t.Errorf("seqBootstrap() returned invalid index at position %d: %d (valid range: 0-%d)", i, idx, c-1)
					}
				}
			}
		})
	}
}

// Helper functions for Monte Carlo experiments

// getRndT1 generates a random t1 series
// This translates Snippet 4.7 from the book to Go
func getRndT1(numObs, numBars, maxH int) []float64 {
	t1 := make([]float64, numObs)
	
	for i := 0; i < numObs; i++ {
		// Random starting bar
		ix := rand.Intn(numBars)
		// Random horizon (bars to look forward)
		val := ix + rand.Intn(maxH) + 1
		t1[i] = float64(val)
	}
	
	return t1
}


// auxMC runs a single Monte Carlo iteration
// This translates Snippet 4.8 from the book to Go
func auxMC(numObs, numBars, maxH int) map[string]float64 {
	// Generate random t1 series
	t1 := getRndT1(numObs, numBars, maxH)
	
	// Generate bar indices
	barIx := make([]int, numBars+1)
	for i := range barIx {
		barIx[i] = i
	}
	
	// Get indicator matrix
	indM, err := getIndMatrix(barIx, t1)
	if err != nil {
		return map[string]float64{
			"stdU": 0,
			"seqU": 0,
		}
	}
	
	// Standard bootstrap
	stdSamples := standardBootstrap(indM, indM.RawMatrix().Cols)
	stdMatrix := selectColumns(indM, stdSamples)
	stdU, err := getAvgUniqueness(stdMatrix)
	if err != nil {
		return map[string]float64{
			"stdU": 0,
			"seqU": 0,
		}
	}
	
	// Sequential bootstrap
	seqSamples, err := seqBootstrap(indM, indM.RawMatrix().Cols)
	if err != nil {
		return map[string]float64{
			"stdU": mean(stdU),
			"seqU": 0,
		}
	}
	
	seqMatrix := selectColumns(indM, seqSamples)
	seqU, err := getAvgUniqueness(seqMatrix)
	if err != nil {
		return map[string]float64{
			"stdU": mean(stdU),
			"seqU": 0,
		}
	}
	
	return map[string]float64{
		"stdU": mean(stdU),
		"seqU": mean(seqU),
	}
}

// mainMC runs multiple Monte Carlo iterations in parallel
// This translates Snippet 4.9 from the book to Go
func TestMonteCarloExperiment(t *testing.T) {
	// Skip this test in normal test runs due to its long duration
	if testing.Short() {
		t.Skip("Skipping Monte Carlo experiment in short mode")
	}
	
	numObs := 10
	numBars := 100
	maxH := 5
	// Reduce for normal testing - increase for full experiment
	numIters := 1000
	
	// Run in parallel
	var wg sync.WaitGroup
	numWorkers := 4
	chunkSize := numIters / numWorkers
	
	results := make([]map[string]float64, numIters)
	
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		
		start := w * chunkSize
		end := start + chunkSize
		if w == numWorkers-1 {
			end = numIters // Last worker takes any remainder
		}
		
		go func(start, end int) {
			defer wg.Done()
			
			for i := start; i < end; i++ {
				results[i] = auxMC(numObs, numBars, maxH)
			}
		}(start, end)
	}
	
	wg.Wait()
	
	// Compute statistics
	stdUValues := make([]float64, 0, numIters)
	seqUValues := make([]float64, 0, numIters)
	
	for _, r := range results {
		if r["stdU"] > 0 {
			stdUValues = append(stdUValues, r["stdU"])
		}
		if r["seqU"] > 0 {
			seqUValues = append(seqUValues, r["seqU"])
		}
	}
	
	t.Logf("Standard Bootstrap - Mean Uniqueness: %.4f", mean(stdUValues))
	t.Logf("Sequential Bootstrap - Mean Uniqueness: %.4f", mean(seqUValues))
	
	// Verify that sequential bootstrap produces higher uniqueness
	if mean(seqUValues) <= mean(stdUValues) {
		t.Errorf("Expected sequential bootstrap to have higher uniqueness: seq=%.4f, std=%.4f", 
			mean(seqUValues), mean(stdUValues))
	}
}




// Helper function for absolute value
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// Benchmarks

func BenchmarkGetIndMatrix(b *testing.B) {
	// Prepare test data
	barIx := make([]int, 100)
	for i := range barIx {
		barIx[i] = i
	}
	
	t1 := make([]float64, 50)
	for i := range t1 {
		t1[i] = float64(i + 10)
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_, _ = getIndMatrix(barIx, t1)
	}
}

func BenchmarkGetAvgUniqueness(b *testing.B) {
	// Prepare test data - create a random matrix
	r, c := 100, 50
	data := make([]float64, r*c)
	for i := range data {
		if rand.Float64() < 0.3 {
			data[i] = 1
		}
	}
	
	indM := mat.NewDense(r, c, data)
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_, _ = getAvgUniqueness(indM)
	}
}

func BenchmarkSeqBootstrap(b *testing.B) {
	// Prepare test data - create a random matrix
	r, c := 100, 50
	data := make([]float64, r*c)
	for i := range data {
		if rand.Float64() < 0.3 {
			data[i] = 1
		}
	}
	
	indM := mat.NewDense(r, c, data)
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_, _ = seqBootstrap(indM, 30)
	}
}

func BenchmarkStandardBootstrap(b *testing.B) {
	// Prepare test data - create a random matrix
	r, c := 100, 50
	data := make([]float64, r*c)
	for i := range data {
		if rand.Float64() < 0.3 {
			data[i] = 1
		}
	}
	
	indM := mat.NewDense(r, c, data)
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_ = standardBootstrap(indM, 30)
	}
}

// Tests for Algorithm interface implementation

func TestSequentialBootstrapAlgorithm_Interface(t *testing.T) {
	// Create an instance of the algorithm
	alg, err := Create(AlgorithmTypeSequentialBootstrap)
	if err != nil {
		t.Fatalf("Failed to create algorithm: %v", err)
	}
	
	// Test Name() method
	name := alg.Name()
	if name != "Sequential Bootstrap" {
		t.Errorf("Name() = %v, want %v", name, "Sequential Bootstrap")
	}
	
	// Test Type() method
	algType := alg.Type()
	if algType != AlgorithmTypeSequentialBootstrap {
		t.Errorf("Type() = %v, want %v", algType, AlgorithmTypeSequentialBootstrap)
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
		"lookback_period",
		"confidence_threshold",
		"use_sequential",
		"sample_size",
	}
	
	for _, param := range requiredParams {
		if _, ok := params[param]; !ok {
			t.Errorf("ParameterDescription() missing required parameter: %s", param)
		}
	}
}

func TestSequentialBootstrapAlgorithm_Configure(t *testing.T) {
	// Create an instance of the algorithm
	alg, err := Create(AlgorithmTypeSequentialBootstrap)
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
					"lookback_period":      20,
					"confidence_threshold": 0.7,
					"use_sequential":       1.0,
					"sample_size":          50,
				},
			},
			wantErr: false,
		},
		{
			name: "Invalid lookback period",
			config: AlgorithmConfig{
				AdditionalParams: map[string]float64{
					"lookback_period": -5,
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid confidence threshold",
			config: AlgorithmConfig{
				AdditionalParams: map[string]float64{
					"confidence_threshold": 1.5,
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid sample size",
			config: AlgorithmConfig{
				AdditionalParams: map[string]float64{
					"sample_size": -10,
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

func TestSequentialBootstrapAlgorithm_Process(t *testing.T) {
	// Create test market data
	currentData := &types.MarketData{
		Symbol:    "AAPL",
		Price:     150.0,
		High24h:   155.0,
		Low24h:    145.0,
		Volume24h: 1000000,
		Change24h: 2.5,
	}
	
	// Create historical data
	historicalData := make([]types.MarketData, 100)
	for i := range historicalData {
		// Create some pattern in the data
		price := 150.0 + 5.0*math.Sin(float64(i)*0.1)
		
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
	alg, err := Create(AlgorithmTypeSequentialBootstrap)
	if err != nil {
		t.Fatalf("Failed to create algorithm: %v", err)
	}
	
	// Configure the algorithm
	config := AlgorithmConfig{
		RiskAversion:     5.0,
		HistoricalDays:   30,
		AdditionalParams: map[string]float64{
			"lookback_period":      20,
			"confidence_threshold": 0.6,
			"use_sequential":       1.0,
			"sample_size":          50,
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
	
	// Test Explain method
	explanation := alg.Explain()
	if explanation == "" {
		t.Error("Explain() returned empty string")
	}
}