package algo

import (
	"errors"
	"fmt"
	"math"

	"github.com/rileyseaburg/go-trader/types"
)

// MetaLabelResult represents the output of the meta-labeling process
type MetaLabelResult struct {
	OriginalSignal string  // The original signal (buy/sell/hold)
	MetaLabel      bool    // Whether to take the trade (true/false)
	Confidence     float64 // Confidence in the meta-label (0-1)
	SuggestedSize  float64 // Suggested position size
}

// ModelType represents the type of ML model to use for meta-labeling
type ModelType string

const (
	// ModelTypeLogisticRegression represents a simple logistic regression model
	ModelTypeLogisticRegression ModelType = "logistic_regression"
	// ModelTypeRandomForest represents a random forest model
	ModelTypeRandomForest ModelType = "random_forest"
	// ModelTypeSimpleRules represents a simple rule-based model (non-ML)
	ModelTypeSimpleRules ModelType = "simple_rules"
)

// FeatureType represents the type of features to use for meta-labeling
type FeatureType string

const (
	// FeatureTypePrice represents price-based features
	FeatureTypePrice FeatureType = "price"
	// FeatureTypeVolume represents volume-based features
	FeatureTypeVolume FeatureType = "volume"
	// FeatureTypeVolatility represents volatility-based features
	FeatureTypeVolatility FeatureType = "volatility"
	// FeatureTypeTechnical represents technical indicators
	FeatureTypeTechnical FeatureType = "technical"
)

// init registers the MetaLabeling algorithm with the factory
func init() {
	Register(AlgorithmTypeMetaLabeling, func() Algorithm {
		return &MetaLabelingAlgorithm{
			BaseAlgorithm: BaseAlgorithm{},
		}
	})
}

// MetaLabelingAlgorithm implements the Meta-Labeling approach from
// de Prado's "Advances in Financial Machine Learning" (2018)
type MetaLabelingAlgorithm struct {
	BaseAlgorithm
	confidenceThreshold float64                // Minimum confidence to execute trade
	features            []FeatureType          // Features to use for meta-labeling
	modelType           ModelType              // Type of ML model to use
	modelParams         map[string]interface{} // Model parameters
	primaryAlgorithm    AlgorithmType          // Primary signal generator algorithm
	weights             []float64              // Model weights (for simple models)
	bias                float64                // Model bias term (for simple models)
	featureRanges       map[string][2]float64  // Min/max ranges for feature normalization
}

// Name returns the name of the algorithm
func (m *MetaLabelingAlgorithm) Name() string {
	return "Meta-Labeling"
}

// Type returns the type of the algorithm
func (m *MetaLabelingAlgorithm) Type() AlgorithmType {
	return AlgorithmTypeMetaLabeling
}

// Description returns a brief description of the algorithm
func (m *MetaLabelingAlgorithm) Description() string {
	return "Implements Meta-Labeling to filter primary trading signals using a secondary model"
}

// ParameterDescription returns a description of the parameters
func (m *MetaLabelingAlgorithm) ParameterDescription() map[string]string {
	return map[string]string{
		"confidence_threshold": "Minimum confidence to execute trade (default: 0.6)",
		"model_type":           "Type of model to use: logistic_regression, random_forest, simple_rules (default: simple_rules)",
		"primary_algorithm":    "Type of primary algorithm to use for signal generation (default: sequential_bootstrap)",
		"use_price_features":   "Whether to use price-based features (default: 1)",
		"use_volume_features":  "Whether to use volume-based features (default: 1)",
		"use_volatility_features": "Whether to use volatility-based features (default: 1)",
		"use_technical_features": "Whether to use technical indicators (default: 1)",
	}
}

// Configure configures the algorithm with the given parameters
func (m *MetaLabelingAlgorithm) Configure(config AlgorithmConfig) error {
	if err := m.BaseAlgorithm.Configure(config); err != nil {
		return err
	}

	// Set default values
	m.confidenceThreshold = 0.6
	m.features = []FeatureType{FeatureTypePrice, FeatureTypeVolume, FeatureTypeVolatility, FeatureTypeTechnical}
	m.modelType = ModelTypeSimpleRules
	m.primaryAlgorithm = AlgorithmTypeSequentialBootstrap
	m.modelParams = make(map[string]interface{})
	
	// Initialize feature ranges
	m.featureRanges = map[string][2]float64{
		"price_change":     {-0.05, 0.05},
		"volume_ratio":     {0, 3},
		"volatility":       {0, 0.05},
		"rsi":              {0, 100},
		"macd":             {-0.05, 0.05},
		"bollinger_pct_b":  {0, 1},
	}

	// Override with provided values
	if val, ok := config.AdditionalParams["confidence_threshold"]; ok {
		if val < 0 || val > 1 {
			return errors.New("confidence_threshold must be between 0 and 1")
		}
		m.confidenceThreshold = val
	}

	if val, ok := config.AdditionalParams["use_price_features"]; ok {
		if val <= 0.5 {
			m.removeFeature(FeatureTypePrice)
		}
	}

	if val, ok := config.AdditionalParams["use_volume_features"]; ok {
		if val <= 0.5 {
			m.removeFeature(FeatureTypeVolume)
		}
	}

	if val, ok := config.AdditionalParams["use_volatility_features"]; ok {
		if val <= 0.5 {
			m.removeFeature(FeatureTypeVolatility)
		}
	}

	if val, ok := config.AdditionalParams["use_technical_features"]; ok {
		if val <= 0.5 {
			m.removeFeature(FeatureTypeTechnical)
		}
	}

	if len(m.features) == 0 {
		return errors.New("at least one feature type must be enabled")
	}

	// Set up a simple model with weights based on empirical observations
	// In a real implementation, these would be trained on historical data
	m.weights = []float64{0.2, 0.2, 0.3, 0.3}
	m.bias = -0.1

	return nil
}

// removeFeature removes a feature type from the features slice
func (m *MetaLabelingAlgorithm) removeFeature(featureType FeatureType) {
	for i, f := range m.features {
		if f == featureType {
			m.features = append(m.features[:i], m.features[i+1:]...)
			return
		}
	}
}

// Process processes the market data and generates meta-labeled trading signals
func (m *MetaLabelingAlgorithm) Process(
	symbol string,
	currentData *types.MarketData,
	historicalData []types.MarketData,
) (*AlgorithmResult, error) {
	if len(historicalData) < 10 {
		return nil, errors.New("insufficient historical data for meta-labeling")
	}

	// Step 1: Get the primary signal from the primary algorithm
	primaryAlg, err := Create(m.primaryAlgorithm)
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

	// If primary signal is "hold", we don't need meta-labeling
	if primaryResult.Signal == "hold" {
		m.explanation = "Primary signal is 'hold'. No meta-labeling needed."
		return primaryResult, nil
	}

	// Step 2: Extract features for meta-labeling
	features := m.extractFeatures(currentData, historicalData)

	// Step 3: Apply meta-labeling
	metaLabelResult, err := m.applyMetaLabeling(primaryResult.Signal, features, primaryResult.Confidence)
	if err != nil {
		return nil, fmt.Errorf("error applying meta-labeling: %v", err)
	}

	// Step 4: Determine final signal based on meta-label
	var finalSignal, finalOrderType string
	var finalConfidence float64

	if metaLabelResult.MetaLabel {
		// Keep the primary signal but adjust confidence
		finalSignal = primaryResult.Signal
		finalOrderType = primaryResult.OrderType
		finalConfidence = metaLabelResult.Confidence
	} else {
		// Override with "hold" if meta-label is false
		finalSignal = "hold"
		finalOrderType = "none"
		finalConfidence = 0.5
	}

	// Generate explanation
	m.explanation = fmt.Sprintf("Primary algorithm (%s) generated %s signal with confidence %.2f. ",
		primaryAlg.Name(), primaryResult.Signal, primaryResult.Confidence)

	if metaLabelResult.MetaLabel {
		m.explanation += fmt.Sprintf("Meta-labeling confirmed signal with adjusted confidence %.2f. ",
			metaLabelResult.Confidence)
		m.explanation += fmt.Sprintf("Suggested position size: %.2f%%", metaLabelResult.SuggestedSize*100)
	} else {
		m.explanation += "Meta-labeling rejected signal (insufficient probability of profitability)."
	}

	// Add feature importance information
	m.explanation += "\nFeature importance:"
	for i, featType := range m.features {
		m.explanation += fmt.Sprintf("\n - %s: %.2f", featType, m.weights[i])
	}

	return &AlgorithmResult{
		Signal:      finalSignal,
		OrderType:   finalOrderType,
		Confidence:  finalConfidence,
		Explanation: m.explanation,
	}, nil
}

// extractFeatures extracts features for meta-labeling from market data
func (m *MetaLabelingAlgorithm) extractFeatures(
	currentData *types.MarketData,
	historicalData []types.MarketData,
) []float64 {
	features := make([]float64, 0, 10)

	// Helper function to get historical data at index
	getHistorical := func(i int) *types.MarketData {
		idx := len(historicalData) - 1 - i
		if idx >= 0 && idx < len(historicalData) {
			return &historicalData[idx]
		}
		return nil
	}

	// Price-based features
	if containsFeatureType(m.features, FeatureTypePrice) {
		// 1. Recent price change (1-day)
		prev := getHistorical(0)
		if prev != nil {
			priceChange := (currentData.Price - prev.Price) / prev.Price
			features = append(features, normalizeFeature(priceChange, "price_change", m.featureRanges))
		} else {
			features = append(features, 0)
		}

		// 2. Price momentum (5-day)
		prev5 := getHistorical(4)
		if prev5 != nil {
			momentum := (currentData.Price - prev5.Price) / prev5.Price
			features = append(features, normalizeFeature(momentum, "price_change", m.featureRanges))
		} else {
			features = append(features, 0)
		}
	}

	// Volume-based features
	if containsFeatureType(m.features, FeatureTypeVolume) {
		// 3. Volume ratio (current/avg)
		var avgVolume float64
		count := 0
		for i := 0; i < 5; i++ {
			prev := getHistorical(i)
			if prev != nil {
				avgVolume += prev.Volume24h
				count++
			}
		}
		if count > 0 {
			avgVolume /= float64(count)
			volumeRatio := currentData.Volume24h / avgVolume
			features = append(features, normalizeFeature(volumeRatio, "volume_ratio", m.featureRanges))
		} else {
			features = append(features, 0)
		}
	}

	// Volatility-based features
	if containsFeatureType(m.features, FeatureTypeVolatility) {
		// 4. Historical volatility
		prices := make([]float64, len(historicalData))
		for i, data := range historicalData {
			prices[i] = data.Price
		}
		volatility, err := calculateVolatility(prices, 10)
		if err != nil {
			volatility = 0.01 // Default value
		}
		features = append(features, normalizeFeature(volatility, "volatility", m.featureRanges))

		// 5. Price range relative to volatility
		priceRange := (currentData.High24h - currentData.Low24h) / currentData.Price
		features = append(features, normalizeFeature(priceRange, "volatility", m.featureRanges))
	}

	// Technical indicators
	if containsFeatureType(m.features, FeatureTypeTechnical) {
		// 6. RSI
		prices := make([]float64, len(historicalData))
		for i, data := range historicalData {
			prices[i] = data.Price
		}
		rsi := calculateRSI(prices, 14)
		features = append(features, normalizeFeature(rsi, "rsi", m.featureRanges))

		// 7. MACD signal
		macdSignal := calculateMACD(prices)
		features = append(features, normalizeFeature(macdSignal, "macd", m.featureRanges))

		// 8. Bollinger Band position
		bPercent := calculateBollingerPctB(prices, 20, 2.0)
		features = append(features, normalizeFeature(bPercent, "bollinger_pct_b", m.featureRanges))
	}

	return features
}

// applyMetaLabeling applies the meta-labeling model to a primary signal
func (m *MetaLabelingAlgorithm) applyMetaLabeling(
	signal string,
	features []float64,
	primaryConfidence float64,
) (*MetaLabelResult, error) {
	if len(features) == 0 {
		return nil, errors.New("no features provided for meta-labeling")
	}

	var metaConfidence float64

	switch m.modelType {
	case ModelTypeSimpleRules:
		// Simple weighted average of features
		if len(m.weights) < len(features) {
			return nil, errors.New("insufficient weights for features")
		}

		sum := 0.0
		for i, feature := range features {
			sum += feature * m.weights[i]
		}
		// Apply sigmoid to get a probability
		metaConfidence = sigmoid(sum + m.bias)

	case ModelTypeLogisticRegression:
		// This would be a more sophisticated implementation
		// For now, we'll just use a simple approach
		metaConfidence = primaryConfidence

	case ModelTypeRandomForest:
		// This would require a proper ML model implementation
		// For now, we'll use a simplified approach
		metaConfidence = primaryConfidence
	}

	// Determine meta-label and suggested position size
	metaLabel := metaConfidence >= m.confidenceThreshold

	// Size position based on confidence and signal (buy/sell)
	var suggestedSize float64
	if metaLabel {
		// Kelly criterion-inspired sizing
		suggestedSize = (metaConfidence - (1 - metaConfidence)) * 0.5
		suggestedSize = math.Max(0.1, math.Min(1.0, suggestedSize))
	} else {
		suggestedSize = 0
	}

	return &MetaLabelResult{
		OriginalSignal: signal,
		MetaLabel:      metaLabel,
		Confidence:     metaConfidence,
		SuggestedSize:  suggestedSize,
	}, nil
}

// containsFeatureType checks if the features slice contains a specific feature type
func containsFeatureType(features []FeatureType, featureType FeatureType) bool {
	for _, f := range features {
		if f == featureType {
			return true
		}
	}
	return false
}

// normalizeFeature normalizes a feature value to the range [0, 1]
func normalizeFeature(value float64, featureName string, ranges map[string][2]float64) float64 {
	r, ok := ranges[featureName]
	if !ok {
		return value // No range defined, return as is
	}

	min, max := r[0], r[1]
	norm := (value - min) / (max - min)
	return math.Max(0, math.Min(1, norm)) // Clamp to [0, 1]
}

// sigmoid applies the sigmoid function to a value
func sigmoid(x float64) float64 {
	return 1.0 / (1.0 + math.Exp(-x))
}

// calculateVolatility calculates the historical volatility of a price series
func calculateVolatility(prices []float64, window int) (float64, error) {
	if len(prices) < window {
		return 0, errors.New("insufficient data for volatility calculation")
	}

	// Calculate log returns
	returns := make([]float64, len(prices)-1)
	for i := 1; i < len(prices); i++ {
		returns[i-1] = math.Log(prices[i] / prices[i-1])
	}

	// Calculate standard deviation of returns
	mean := 0.0
	for _, r := range returns {
		mean += r
	}
	mean /= float64(len(returns))

	variance := 0.0
	for _, r := range returns {
		variance += math.Pow(r-mean, 2)
	}
	variance /= float64(len(returns) - 1)

	return math.Sqrt(variance), nil
}

// calculateRSI calculates the Relative Strength Index
func calculateRSI(prices []float64, period int) float64 {
	if len(prices) < period+1 {
		return 50 // Default value if insufficient data
	}

	gains := 0.0
	losses := 0.0

	// Calculate initial average gain and loss
	for i := 1; i <= period; i++ {
		change := prices[i] - prices[i-1]
		if change >= 0 {
			gains += change
		} else {
			losses -= change
		}
	}

	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	// Calculate RSI
	if avgLoss == 0 {
		return 100
	}

	rs := avgGain / avgLoss
	rsi := 100 - (100 / (1 + rs))

	return rsi
}

// calculateMACD calculates a simplified MACD signal
func calculateMACD(prices []float64) float64 {
	if len(prices) < 26 {
		return 0 // Default value if insufficient data
	}

	// Calculate EMA12 and EMA26
	ema12 := calculateEMA(prices, 12)
	ema26 := calculateEMA(prices, 26)

	// MACD line
	macd := ema12 - ema26

	// Normalize by price level
	return macd / prices[len(prices)-1]
}

// calculateEMA calculates the Exponential Moving Average
func calculateEMA(prices []float64, period int) float64 {
	if len(prices) < period {
		return prices[len(prices)-1] // Default to last price
	}

	multiplier := 2.0 / float64(period+1)
	ema := prices[0]

	for i := 1; i < len(prices); i++ {
		ema = (prices[i]-ema)*multiplier + ema
	}

	return ema
}

// calculateBollingerPctB calculates the %B value for Bollinger Bands
func calculateBollingerPctB(prices []float64, period int, numStdDev float64) float64 {
	if len(prices) < period {
		return 0.5 // Default value if insufficient data
	}

	// Calculate SMA
	sum := 0.0
	for i := len(prices) - period; i < len(prices); i++ {
		sum += prices[i]
	}
	sma := sum / float64(period)

	// Calculate standard deviation
	variance := 0.0
	for i := len(prices) - period; i < len(prices); i++ {
		variance += math.Pow(prices[i]-sma, 2)
	}
	stdDev := math.Sqrt(variance / float64(period))

	// Calculate Bollinger Bands
	upper := sma + numStdDev*stdDev
	lower := sma - numStdDev*stdDev

	// Calculate %B
	currentPrice := prices[len(prices)-1]
	pctB := (currentPrice - lower) / (upper - lower)

	return pctB
}