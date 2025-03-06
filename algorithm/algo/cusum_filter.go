package algo

import (
	"fmt"
	"math"
	"time"


	"github.com/rileyseaburg/go-trader/types"
)

// CUSUMFilterAlgorithm implements a CUSUM Filter algorithm
type CUSUMFilterAlgorithm struct {
	BaseAlgorithm
	threshold float64
	drift     float64
	spPrev    float64 // Previous positive CUSUM value
	snPrev    float64 // Previous negative CUSUM value
}

// NewCUSUMFilterAlgorithm creates a new CUSUM Filter algorithm
func NewCUSUMFilterAlgorithm() Algorithm {
	return &CUSUMFilterAlgorithm{
		threshold: 1.0,
		drift:     0.02,
		spPrev:    0.0,
		snPrev:    0.0,
	}
}

// Name returns the name of the algorithm
func (c *CUSUMFilterAlgorithm) Name() string {
	return "CUSUM Filter"
}

// Type returns the type of the algorithm
func (c *CUSUMFilterAlgorithm) Type() AlgorithmType {
	return AlgorithmTypeCUSUMFilter
}

// Description returns a brief description of the algorithm
func (c *CUSUMFilterAlgorithm) Description() string {
	return "Detects structural breaks in time series data using cumulative sum control charts"
}

// ParameterDescription returns a description of the parameters this algorithm accepts
func (c *CUSUMFilterAlgorithm) ParameterDescription() map[string]string {
	return map[string]string{
		"threshold": "The threshold for triggering a signal (default: 1.0)",
		"drift":     "The expected drift parameter (default: 0.02)",
	}
}

// Configure configures the algorithm with the given parameters
func (c *CUSUMFilterAlgorithm) Configure(config AlgorithmConfig) error {
	if err := c.BaseAlgorithm.Configure(config); err != nil {
		return err
	}

	if threshold, exists := config.AdditionalParams["threshold"]; exists {
		c.threshold = threshold
	}
	if drift, exists := config.AdditionalParams["drift"]; exists {
		c.drift = drift
	}

	return nil
}

// Process processes the market data and returns a trading signal
func (c *CUSUMFilterAlgorithm) Process(symbol string, data *types.MarketData, historicalData []types.MarketData) (*AlgorithmResult, error) {
	if len(historicalData) < 2 {
		return nil, fmt.Errorf("insufficient historical data for CUSUM analysis")
	}

	// Extract price series
	prices := make([]float64, len(historicalData))
	for i, data := range historicalData {
		prices[i] = data.Price
	}

	// Calculate returns
	returns := make([]float64, len(prices)-1)
	for i := 1; i < len(prices); i++ {
		returns[i-1] = math.Log(prices[i] / prices[i-1])
	}

	// Calculate mean and standard deviation
	mean := 0.0
	for _, r := range returns {
		mean += r
	}
	mean /= float64(len(returns))

	variance := 0.0
	for _, r := range returns {
		variance += math.Pow(r-mean, 2)
	}
	variance /= float64(len(returns))
	stddev := math.Sqrt(variance)

	// Apply CUSUM filter
	// Get the most recent return
	lastReturn := returns[len(returns)-1]
	
	// Standardize the return
	standardizedReturn := (lastReturn - mean) / stddev
	
	// Update CUSUM values
	// Use previous values to create true cumulative sums
	// that track deviations over time
	sp := math.Max(0, c.spPrev+standardizedReturn-c.drift)
	sn := math.Max(0, c.snPrev-standardizedReturn-c.drift)
	
	
	// Determine signal based on CUSUM values
	signal := "hold"
	orderType := "market"
	confidence := 0.5
	var limitPrice *float64
	explanation := ""
	
	// Generate trading signal based on CUSUM threshold breaches
	if sp > c.threshold {
		signal = "buy"
		confidence = math.Min(0.95, 0.5 + sp/10)
		currentPrice := data.Price
		targetPrice := currentPrice * 0.99 // 1% below current price for buy limit
		roundedPrice := math.Floor(targetPrice*100) / 100 // Round to 2 decimal places
		limitPrice = &roundedPrice
		orderType = "limit"
		explanation = fmt.Sprintf("CUSUM positive drift detected (sp: %.4f) exceeding threshold of %.2f, indicating potential upward trend", sp, c.threshold)
	} else if sn > c.threshold {
		signal = "sell"
		confidence = math.Min(0.95, 0.5 + sn/10)
		currentPrice := data.Price
		targetPrice := currentPrice * 1.01 // 1% above current price for sell limit
		roundedPrice := math.Floor(targetPrice*100) / 100 // Round to 2 decimal places
		limitPrice = &roundedPrice
		orderType = "limit"
		explanation = fmt.Sprintf("CUSUM negative drift detected (sn: %.4f) exceeding threshold of %.2f, indicating potential downward trend", sn, c.threshold)
	} else {
		explanation = fmt.Sprintf("No significant drift detected (sp: %.4f, sn: %.4f), threshold: %.2f", sp, sn, c.threshold)
	}
	
	c.explanation = explanation
	c.lastRun = time.Now()

	// Store current CUSUM values for next iteration
	c.spPrev = sp
	c.snPrev = sn
	
	return &AlgorithmResult{
		Signal:      signal,
		OrderType:   orderType,
		LimitPrice:  limitPrice,
		Confidence:  confidence,
		Explanation: explanation,
	}, nil
}

func init() {
	Register(AlgorithmTypeCUSUMFilter, NewCUSUMFilterAlgorithm)
}