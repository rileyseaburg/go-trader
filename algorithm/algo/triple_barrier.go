package algo

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/rileyseaburg/go-trader/types"
)

// BarrierType represents the type of barrier that was hit
type BarrierType string

const (
	// BarrierTypeUpper represents the upper (profit-taking) barrier
	BarrierTypeUpper BarrierType = "upper"
	// BarrierTypeLower represents the lower (stop-loss) barrier
	BarrierTypeLower BarrierType = "lower"
	// BarrierTypeTime represents the vertical (time) barrier
	BarrierTypeTime BarrierType = "time"
)

// BarrierLabel represents the direction label for trading
type BarrierLabel int

const (
	// BarrierLabelSell represents a sell signal (-1)
	BarrierLabelSell BarrierLabel = -1
	// BarrierLabelHold represents a hold signal (0)
	BarrierLabelHold BarrierLabel = 0
	// BarrierLabelBuy represents a buy signal (1)
	BarrierLabelBuy BarrierLabel = 1
)

// BarrierResult represents the result of applying the triple barrier method
type BarrierResult struct {
	Label      BarrierLabel `json:"label"`       // -1 (sell), 0 (hold), 1 (buy)
	ExitTime   time.Time    `json:"exit_time"`   // When the position was exited
	ExitPrice  float64      `json:"exit_price"`  // At what price
	BarrierHit BarrierType  `json:"barrier_hit"` // Which barrier was hit
	EntryTime  time.Time    `json:"entry_time"`  // When the position was entered
	EntryPrice float64      `json:"entry_price"` // At what price
}

// TripleBarrierConfig represents the configuration for the triple barrier method
type TripleBarrierConfig struct {
	ProfitTaking       float64 // Multiple of volatility for upper barrier
	StopLoss           float64 // Multiple of volatility for lower barrier
	TimeHorizon        int     // Days for vertical barrier
	VolatilityLookback int     // Lookback window for volatility estimation
}

// init registers the Triple Barrier algorithm with the factory
func init() {
	Register(AlgorithmTypeTripleBarrier, func() Algorithm {
		return &TripleBarrierAlgorithm{
			BaseAlgorithm: BaseAlgorithm{},
		}
	})
}

// TripleBarrierAlgorithm implements the Triple Barrier method from
// de Prado's "Advances in Financial Machine Learning" (2018)
type TripleBarrierAlgorithm struct {
	BaseAlgorithm
	config           TripleBarrierConfig
	profitTaking     float64 // Multiple of volatility for upper barrier
	stopLoss         float64 // Multiple of volatility for lower barrier
	timeHorizon      int     // Days for vertical barrier
	volatilityWindow int     // Lookback window for volatility estimation
}

// Name returns the name of the algorithm
func (t *TripleBarrierAlgorithm) Name() string {
	return "Triple Barrier Method"
}

// Type returns the type of the algorithm
func (t *TripleBarrierAlgorithm) Type() AlgorithmType {
	return AlgorithmTypeTripleBarrier
}

// Description returns a brief description of the algorithm
func (t *TripleBarrierAlgorithm) Description() string {
	return "Implements the Triple Barrier Method for labeling financial data with take-profit, stop-loss, and time exit conditions"
}

// ParameterDescription returns a description of the parameters
func (t *TripleBarrierAlgorithm) ParameterDescription() map[string]string {
	return map[string]string{
		"profit_taking":       "Multiple of volatility for upper/profit-taking barrier (default: 2.0)",
		"stop_loss":           "Multiple of volatility for lower/stop-loss barrier (default: 1.0)",
		"time_horizon":        "Number of days for vertical/time barrier (default: 5)",
		"volatility_lookback": "Lookback window in days for volatility estimation (default: 20)",
	}
}

// Configure configures the algorithm with the given parameters
func (t *TripleBarrierAlgorithm) Configure(config AlgorithmConfig) error {
	if err := t.BaseAlgorithm.Configure(config); err != nil {
		return err
	}

	// Set default values
	t.profitTaking = 2.0
	t.stopLoss = 1.0
	t.timeHorizon = 5
	t.volatilityWindow = 20

	// Override with provided values
	if val, ok := config.AdditionalParams["profit_taking"]; ok {
		if val <= 0 {
			return errors.New("profit_taking must be positive")
		}
		t.profitTaking = val
	}

	if val, ok := config.AdditionalParams["stop_loss"]; ok {
		if val <= 0 {
			return errors.New("stop_loss must be positive")
		}
		t.stopLoss = val
	}

	if val, ok := config.AdditionalParams["time_horizon"]; ok {
		if val < 1 {
			return errors.New("time_horizon must be at least 1")
		}
		t.timeHorizon = int(val)
	}

	if val, ok := config.AdditionalParams["volatility_lookback"]; ok {
		if val < 1 {
			return errors.New("volatility_lookback must be at least 1")
		}
		t.volatilityWindow = int(val)
	}

	// Update the config
	t.config = TripleBarrierConfig{
		ProfitTaking:       t.profitTaking,
		StopLoss:           t.stopLoss,
		TimeHorizon:        t.timeHorizon,
		VolatilityLookback: t.volatilityWindow,
	}

	return nil
}

// Process processes the market data and generates labels using the triple barrier method
func (t *TripleBarrierAlgorithm) Process(
	symbol string,
	currentData *types.MarketData,
	historicalData []types.MarketData,
) (*AlgorithmResult, error) {
	if len(historicalData) < t.volatilityWindow {
		return nil, fmt.Errorf("insufficient historical data: need at least %d data points for volatility estimation", t.volatilityWindow)
	}

	// Extract price series
	prices := make([]float64, len(historicalData))
	times := make([]time.Time, len(historicalData))
	baseTime := time.Now().Add(-time.Duration(len(historicalData)) * 24 * time.Hour) // Start from current time - days of data
	
	for i, data := range historicalData {
		prices[i] = data.Price
		// Since MarketData doesn't have a timestamp, we'll create synthetic ones
		// This assumes daily data for illustration purposes
		times[i] = baseTime.Add(time.Duration(i) * 24 * time.Hour)
	}

	// Calculate daily volatility
	vol, err := DailyVolatility(prices, t.volatilityWindow)
	if err != nil {
		return nil, fmt.Errorf("error calculating volatility: %v", err)
	}

	// Apply triple barrier on historical data
	result, err := ApplyTripleBarrier(prices, times, vol, t.config)
	if err != nil {
		return nil, fmt.Errorf("error applying triple barrier: %v", err)
	}

	// Generate explanation based on the most recent barrier result
	var explanation string
	var signal string
	var orderType string
	var confidence float64

	if result == nil || len(result) == 0 {
		explanation = "Triple barrier method did not generate any labels"
		signal = "hold"
		orderType = "none"
		confidence = 0.5
	} else {
		// Use the most recent barrier result
		latestResult := result[len(result)-1]
		
		explanation = fmt.Sprintf("Triple barrier method applied with profit-taking=%.2f, stop-loss=%.2f, time-horizon=%d days.\n",
			t.profitTaking, t.stopLoss, t.timeHorizon)
			
		explanation += fmt.Sprintf("Entry at %.2f on %s, exit at %.2f on %s.\n",
			latestResult.EntryPrice, 
			latestResult.EntryTime.Format("2006-01-02"),
			latestResult.ExitPrice, 
			latestResult.ExitTime.Format("2006-01-02"))
			
		explanation += fmt.Sprintf("Barrier hit: %s, resulting label: %d", 
			latestResult.BarrierHit, latestResult.Label)

		// Convert barrier label to signal
		switch latestResult.Label {
		case BarrierLabelBuy:
			signal = "buy"
			orderType = "market"
			confidence = 0.7 // Higher confidence based on triple barrier validation
		case BarrierLabelSell:
			signal = "sell"
			orderType = "market"
			confidence = 0.7
		default:
			signal = "hold"
			orderType = "none"
			confidence = 0.5
		}
	}

	t.explanation = explanation

	return &AlgorithmResult{
		Signal:      signal,
		OrderType:   orderType,
		Confidence:  confidence,
		Explanation: explanation,
	}, nil
}

// DailyVolatility estimates the daily volatility using an exponentially weighted moving standard deviation
// This corresponds to Snippet 3.1 in the book
func DailyVolatility(prices []float64, span int) (float64, error) {
	if len(prices) < 2 {
		return 0, errors.New("need at least 2 price points to calculate volatility")
	}

	if span < 1 {
		return 0, errors.New("span must be at least 1")
	}

	// Compute daily returns
	returns := make([]float64, len(prices)-1)
	for i := 1; i < len(prices); i++ {
		returns[i-1] = math.Log(prices[i] / prices[i-1])
	}

	// Compute exponentially weighted moving standard deviation
	// For simplicity, using a exponentially weighted moving average with decay factor
	alpha := 2.0 / float64(span+1)
	var ewma, ewmVar float64
	
	// Initialize with the first return
	ewma = returns[0]
	ewmVar = 0
	
	for i := 1; i < len(returns); i++ {
		// Update the exponentially weighted moving average
		ewma = alpha*returns[i] + (1-alpha)*ewma
		
		// Update the exponentially weighted moving variance
		deviation := returns[i] - ewma
		ewmVar = alpha*deviation*deviation + (1-alpha)*ewmVar
	}
	
	// Return the square root of the variance (standard deviation)
	return math.Sqrt(ewmVar), nil
}

// ApplyTripleBarrier implements the Triple Barrier method
// This corresponds to Snippets 3.2-3.3 in the book
func ApplyTripleBarrier(prices []float64, times []time.Time, volatility float64, config TripleBarrierConfig) ([]*BarrierResult, error) {
	if len(prices) < 2 {
		return nil, errors.New("need at least 2 price points")
	}
	
	if len(prices) != len(times) {
		return nil, errors.New("prices and times must have the same length")
	}
	
	if volatility <= 0 {
		return nil, errors.New("volatility must be positive")
	}
	
	// Initialize results
	results := make([]*BarrierResult, 0)
	
	// For each potential entry point
	for i := 0; i < len(prices)-1; i++ {
		entryPrice := prices[i]
		entryTime := times[i]
		
		// Calculate upper and lower barrier levels based on volatility
		upperBarrier := entryPrice * (1 + config.ProfitTaking * volatility)
		lowerBarrier := entryPrice * (1 - config.StopLoss * volatility)
		
		// Define time barrier
		timeBarrierIdx := min(i + config.TimeHorizon, len(prices)-1)
		
		// Simulate forward in time to determine which barrier is hit first
		var hitBarrier BarrierType
		var exitIdx int
		var label BarrierLabel
		
		// Determine direction of trade based on prior trend
		// Simplified: if the price has been going up, we expect it to continue (buy)
		// if it's been going down, we expect it to continue (sell)
		var trend float64
		if i > 0 {
			trend = prices[i] - prices[i-1]
		}
		
		// Default direction based on trend
		if trend > 0 {
			label = BarrierLabelBuy
		} else if trend < 0 {
			label = BarrierLabelSell
		} else {
			label = BarrierLabelHold
		}
		
		// Check which barrier is hit first
		for j := i + 1; j <= timeBarrierIdx; j++ {
			currentPrice := prices[j]
			
			// Check if upper barrier is hit
			if currentPrice >= upperBarrier {
				hitBarrier = BarrierTypeUpper
				exitIdx = j
				// Label: 1 for upper barrier hit on a buy, -1 for sell
				if label == BarrierLabelBuy {
					label = BarrierLabelBuy // confirming trend
				} else {
					label = BarrierLabelSell // reversing trend
				}
				break
			}
			
			// Check if lower barrier is hit
			if currentPrice <= lowerBarrier {
				hitBarrier = BarrierTypeLower
				exitIdx = j
				// Label: -1 for lower barrier hit on a buy, 1 for sell
				if label == BarrierLabelBuy {
					label = BarrierLabelSell // reversing trend
				} else {
					label = BarrierLabelBuy // confirming trend
				}
				break
			}
			
			// If we reach the time barrier
			if j == timeBarrierIdx {
				hitBarrier = BarrierTypeTime
				exitIdx = j
				// For time barrier, the label depends on the final price relative to entry
				if currentPrice > entryPrice {
					label = BarrierLabelBuy
				} else if currentPrice < entryPrice {
					label = BarrierLabelSell
				} else {
					label = BarrierLabelHold
				}
			}
		}
		
		// Record the result
		results = append(results, &BarrierResult{
			Label:      label,
			EntryTime:  entryTime,
			EntryPrice: entryPrice,
			ExitTime:   times[exitIdx],
			ExitPrice:  prices[exitIdx],
			BarrierHit: hitBarrier,
		})
	}
	
	return results, nil
}

// GetMetaLabels converts barrier results to meta-labels for secondary ML model
// This is used in conjunction with meta-labeling approach
func GetMetaLabels(barrierResults []*BarrierResult) []bool {
	if len(barrierResults) == 0 {
		return nil
	}
	
	// Convert barrier results to binary meta-labels
	// A meta-label of true indicates a successful trade
	metalabels := make([]bool, len(barrierResults))
	
	for i, result := range barrierResults {
		// A trade is considered successful if:
		// 1. We bought (label=1) and exited at a higher price, or
		// 2. We sold (label=-1) and exited at a lower price
		isSuccessful := (result.Label == BarrierLabelBuy && result.ExitPrice > result.EntryPrice) ||
						(result.Label == BarrierLabelSell && result.ExitPrice < result.EntryPrice)
		
		metalabels[i] = isSuccessful
	}
	
	return metalabels
}