package algo

import (
	"errors"
	"fmt"
	"math"

	"github.com/rileyseaburg/go-trader/types"
)

// PositionSizeResult represents the result of position sizing calculation
type PositionSizeResult struct {
	Symbol       string  `json:"symbol"`        // Symbol for this position
	Signal       string  `json:"signal"`        // Buy, Sell, or Hold signal
	Size         float64 `json:"size"`          // Position size as percentage of capital
	Confidence   float64 `json:"confidence"`    // Confidence in the signal
	VolAdjusted  bool    `json:"vol_adjusted"`  // Whether position was volatility-adjusted
	RiskPerTrade float64 `json:"risk_per_trade"` // Risk amount per trade
}

// init registers the PositionSizing algorithm with the factory
func init() {
	Register(AlgorithmTypePositionSizing, func() Algorithm {
		return &PositionSizingAlgorithm{
			BaseAlgorithm: BaseAlgorithm{},
		}
	})
}

// PositionSizingAlgorithm implements advanced position sizing strategies
// based on techniques from de Prado's "Advances in Financial Machine Learning" (2018)
type PositionSizingAlgorithm struct {
	BaseAlgorithm
	maxSize         float64 // Maximum position size as pct of capital (0-1)
	riskFraction    float64 // Kelly fraction (typically 0.2-0.5)
	useVolAdjustment bool   // Whether to adjust for volatility
	volLookback     int     // Volatility lookback period
	maxDrawdown     float64 // Maximum acceptable drawdown
	primaryAlgorithm AlgorithmType // Primary signal generator algorithm
	metaLabeling     bool   // Whether to use meta-labeling for confidence
}

// Name returns the name of the algorithm
func (p *PositionSizingAlgorithm) Name() string {
	return "Advanced Position Sizing"
}

// Type returns the type of the algorithm
func (p *PositionSizingAlgorithm) Type() AlgorithmType {
	return AlgorithmTypePositionSizing
}

// Description returns a brief description of the algorithm
func (p *PositionSizingAlgorithm) Description() string {
	return "Implements advanced position sizing strategies based on Kelly Criterion, volatility adjustment, and meta-labeling confidence"
}

// ParameterDescription returns a description of the parameters
func (p *PositionSizingAlgorithm) ParameterDescription() map[string]string {
	return map[string]string{
		"max_size":           "Maximum position size as percentage of capital (default: 0.2)",
		"risk_fraction":      "Kelly fraction for bet sizing (default: 0.3)",
		"use_vol_adjustment": "Whether to adjust position size for volatility (default: 1)",
		"vol_lookback":       "Lookback period for volatility calculation (default: 20)",
		"max_drawdown":       "Maximum acceptable drawdown (default: 0.1)",
		"primary_algorithm":  "Type of primary algorithm to use for signal generation (default: sequential_bootstrap)",
		"use_meta_labeling":  "Whether to use meta-labeling for confidence (default: 1)",
	}
}

// Configure configures the algorithm with the given parameters
func (p *PositionSizingAlgorithm) Configure(config AlgorithmConfig) error {
	if err := p.BaseAlgorithm.Configure(config); err != nil {
		return err
	}

	// Set default values
	p.maxSize = 0.2
	p.riskFraction = 0.3
	p.useVolAdjustment = true
	p.volLookback = 20
	p.maxDrawdown = 0.1
	p.primaryAlgorithm = AlgorithmTypeSequentialBootstrap
	p.metaLabeling = true

	// Override with provided values
	if val, ok := config.AdditionalParams["max_size"]; ok {
		if val <= 0 || val > 1 {
			return errors.New("max_size must be between 0 and 1")
		}
		p.maxSize = val
	}

	if val, ok := config.AdditionalParams["risk_fraction"]; ok {
		if val <= 0 || val > 1 {
			return errors.New("risk_fraction must be between 0 and 1")
		}
		p.riskFraction = val
	}

	if val, ok := config.AdditionalParams["use_vol_adjustment"]; ok {
		p.useVolAdjustment = val > 0.5
	}

	if val, ok := config.AdditionalParams["vol_lookback"]; ok {
		if val < 1 {
			return errors.New("vol_lookback must be at least 1")
		}
		p.volLookback = int(val)
	}

	if val, ok := config.AdditionalParams["max_drawdown"]; ok {
		if val <= 0 || val > 0.5 {
			return errors.New("max_drawdown must be between 0 and 0.5")
		}
		p.maxDrawdown = val
	}

	if val, ok := config.AdditionalParams["use_meta_labeling"]; ok {
		p.metaLabeling = val > 0.5
	}

	return nil
}

// Process processes the market data and calculates the optimal position size
func (p *PositionSizingAlgorithm) Process(
	symbol string,
	currentData *types.MarketData,
	historicalData []types.MarketData,
) (*AlgorithmResult, error) {
	if len(historicalData) < p.volLookback {
		return nil, fmt.Errorf("insufficient historical data: got %d, need at least %d",
			len(historicalData), p.volLookback)
	}

	// Step 1: Get the primary signal from the primary algorithm
	primaryAlg, err := Create(p.primaryAlgorithm)
	if err != nil {
		return nil, fmt.Errorf("error creating primary algorithm: %v", err)
	}

	// Configure primary algorithm with default parameters
	err = primaryAlg.Configure(AlgorithmConfig{})
	if err != nil {
		return nil, fmt.Errorf("error configuring primary algorithm: %v", err)
	}

	// Get primary signal
	primaryResult, err := primaryAlg.Process(symbol, currentData, historicalData)
	if err != nil {
		return nil, fmt.Errorf("error processing primary algorithm: %v", err)
	}

	// If primary signal is "hold", no position sizing needed
	if primaryResult.Signal == "hold" {
		p.explanation = "Primary signal is 'hold'. No position sizing needed."
		return primaryResult, nil
	}

	// Step 2: Apply meta-labeling if enabled
	confidence := primaryResult.Confidence
	var metaLabelResult *MetaLabelResult

	if p.metaLabeling {
		metaLabelAlg, err := Create(AlgorithmTypeMetaLabeling)
		if err != nil {
			return nil, fmt.Errorf("error creating meta-labeling algorithm: %v", err)
		}

		// Configure meta-labeler with default parameters
		err = metaLabelAlg.Configure(AlgorithmConfig{})
		if err != nil {
			return nil, fmt.Errorf("error configuring meta-labeling algorithm: %v", err)
		}

		// Get meta-label
		metaLabelResult, err := metaLabelAlg.Process(symbol, currentData, historicalData)
		if err != nil {
			return nil, fmt.Errorf("error processing meta-labeling algorithm: %v", err)
		}

		// If meta-labeling rejected the signal, return a hold
		if metaLabelResult.Signal == "hold" {
			p.explanation = "Meta-labeling rejected the primary signal. No position taken."
			return metaLabelResult, nil
		}

		// Update confidence with meta-label confidence
		confidence = metaLabelResult.Confidence
	}

	// Step 3: Calculate volatility for position sizing
	prices := make([]float64, len(historicalData))
	for i, data := range historicalData {
		prices[i] = data.Price
	}

	volatility, err := calculateVolatility(prices, p.volLookback)
	if err != nil {
		return nil, fmt.Errorf("error calculating volatility: %v", err)
	}

	// Step 4: Calculate position size
	positionResult, err := p.calculatePositionSize(primaryResult.Signal, confidence, volatility, currentData.Price)
	if err != nil {
		return nil, fmt.Errorf("error calculating position size: %v", err)
	}

	// Generate explanation
	p.explanation = fmt.Sprintf("Primary algorithm (%s) generated %s signal with confidence %.2f.\n",
		primaryAlg.Name(), primaryResult.Signal, primaryResult.Confidence)

	if p.metaLabeling && metaLabelResult != nil {
		p.explanation += fmt.Sprintf("Meta-labeling adjusted confidence to %.2f.\n", confidence)
	}

	p.explanation += fmt.Sprintf("Current volatility: %.2f%%\n", volatility*100)
	p.explanation += fmt.Sprintf("Position sizing: %.2f%% of capital", positionResult.Size*100)

	if positionResult.VolAdjusted {
		p.explanation += " (volatility adjusted)"
	}

	p.explanation += fmt.Sprintf("\nRisk per trade: %.2f%% of capital", positionResult.RiskPerTrade*100)

	return &AlgorithmResult{
		Signal:      primaryResult.Signal,
		OrderType:   primaryResult.OrderType,
		Confidence:  confidence,
		Explanation: p.explanation,
	}, nil
}

// calculatePositionSize calculates the position size based on signal, confidence, and volatility
func (p *PositionSizingAlgorithm) calculatePositionSize(
	signal string,
	confidence float64,
	volatility float64,
	currentPrice float64,
) (*PositionSizeResult, error) {
	if signal != "buy" && signal != "sell" {
		return nil, fmt.Errorf("invalid signal: %s", signal)
	}

	if confidence <= 0 || confidence > 1 {
		return nil, fmt.Errorf("confidence must be between 0 and 1, got %.2f", confidence)
	}

	if volatility <= 0 {
		return nil, fmt.Errorf("volatility must be positive, got %.6f", volatility)
	}

	// Step 1: Calculate Kelly optimal bet size
	// Kelly f* = (p * b - (1-p)) / b
	// where p = probability of winning
	//       b = win/loss ratio

	// Simplification: use confidence as win probability
	winProb := confidence

	// Assume win/loss ratio of 1 (risk/reward of 1) for simplicity
	// In practice, this could be derived from historical performance or stop/target levels
	winLossRatio := 1.0

	// Calculate Kelly fraction
	kellyFraction := (winProb*winLossRatio - (1-winProb)) / winLossRatio

	// Apply fractional Kelly (risk fraction)
	// This is a safer approach than using full Kelly
	kellyFraction *= p.riskFraction

	// Ensure Kelly fraction is non-negative
	kellyFraction = math.Max(0, kellyFraction)

	// Step 2: Apply volatility adjustment if enabled
	var size float64
	volAdjusted := false

	if p.useVolAdjustment {
		// Adjust position size inversely to volatility
		// Higher volatility = smaller position
		// Formula: size = kellyFraction / (volatility * scalingFactor)
		
		// Baseline volatility (considered "normal")
		baselineVol := 0.01 // 1% daily volatility
		
		// Volatility scaling factor - how much to reduce position for increased vol
		volScalingFactor := volatility / baselineVol
		
		// Apply volatility adjustment with reasonable limits
		adjustment := math.Min(2.0, math.Max(0.5, 1.0/volScalingFactor))
		
		size = kellyFraction * adjustment
		volAdjusted = true
	} else {
		size = kellyFraction
	}

	// Step 3: Apply position limits
	// Cap at maximum size
	size = math.Min(p.maxSize, size)

	// Step 4: Calculate risk per trade
	// Risk = Position Size * Stop Loss Distance
	// Assuming stop loss at 1 standard deviation (volatility)
	riskPerTrade := size * volatility

	// Create position sizing result
	return &PositionSizeResult{
		Symbol:       signal,
		Signal:       signal,
		Size:         size,
		Confidence:   confidence,
		VolAdjusted:  volAdjusted,
		RiskPerTrade: riskPerTrade,
	}, nil
}

// KellyOptimalF calculates the optimal Kelly criterion betting fraction
func KellyOptimalF(winProb float64, winLossRatio float64) float64 {
	return (winProb*winLossRatio - (1-winProb)) / winLossRatio
}

// AdjustPositionForVolatility adjusts a position size based on volatility
func AdjustPositionForVolatility(baseSize float64, volatility float64, baselineVol float64) float64 {
	volScalingFactor := volatility / baselineVol
	adjustment := math.Min(2.0, math.Max(0.5, 1.0/volScalingFactor))
	return baseSize * adjustment
}

// CalculateDiversifiedPositionSize calculates position size accounting for portfolio correlation
// This is a simplified implementation - more sophisticated approaches would model
// the full correlation structure of the portfolio
func CalculateDiversifiedPositionSize(baseSize float64, numPositions int, avgCorrelation float64) float64 {
	// With high correlation, reduce sizes to account for concentration risk
	// With low correlation, can increase sizes due to diversification benefit
	
	if numPositions <= 1 {
		return baseSize
	}
	
	// Scale based on effective number of bets
	// When correlation is high, effective N is low
	effectiveN := float64(numPositions) * (1 - avgCorrelation) + avgCorrelation
	
	// Divide by sqrt(effectiveN) to account for diversification
	scalingFactor := 1.0 / math.Sqrt(effectiveN)
	
	// Apply reasonable bounds
	scalingFactor = math.Min(1.0, math.Max(0.25, scalingFactor))
	
	return baseSize * scalingFactor
}