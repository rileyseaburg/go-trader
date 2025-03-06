package algo

import (
	"fmt"
	"time"
	"github.com/rileyseaburg/go-trader/types"
)

// AlgorithmType represents the type of algorithm
type AlgorithmType string

const (
	// AlgorithmTypeHRP represents Hierarchical Risk Parity algorithm
	AlgorithmTypeHRP AlgorithmType = "hrp"
	// AlgorithmTypeMVO represents Mean-Variance Optimization algorithm
	AlgorithmTypeMVO AlgorithmType = "mvo"
	// AlgorithmTypeEntropyPooling represents Entropy Pooling algorithm
	AlgorithmTypeEntropyPooling AlgorithmType = "entropy_pooling"
	// AlgorithmTypeCUSUMFilter represents CUSUM Filter algorithm
	AlgorithmTypeCUSUMFilter AlgorithmType = "cusum_filter"
	// AlgorithmTypeSequentialBootstrap represents Sequential Bootstrap algorithm
	AlgorithmTypeSequentialBootstrap AlgorithmType = "sequential_bootstrap"
	// AlgorithmTypeFractionalDiff represents Fractional Differentiation algorithm
	AlgorithmTypeFractionalDiff AlgorithmType = "fractional_diff"
	// AlgorithmTypeTripleBarrier represents Triple Barrier method algorithm
	AlgorithmTypeTripleBarrier AlgorithmType = "triple_barrier"
	// AlgorithmTypeMetaLabeling represents Meta-Labeling approach algorithm
	AlgorithmTypeMetaLabeling AlgorithmType = "meta_labeling"
	// AlgorithmTypePurgedCV represents Purged Cross-Validation algorithm
	AlgorithmTypePurgedCV AlgorithmType = "purged_cv"
	// AlgorithmTypePositionSizing represents Advanced Position Sizing algorithm
	AlgorithmTypePositionSizing AlgorithmType = "position_sizing"
)

// AlgorithmConfig represents the configuration for an algorithm
type AlgorithmConfig struct {
	// Common parameters
	RiskAversion      float64            `json:"risk_aversion"`
	MaxPositionWeight float64            `json:"max_position_weight"`
	MinPositionWeight float64            `json:"min_position_weight"`
	TargetReturn      float64            `json:"target_return"`
	HistoricalDays    int                `json:"historical_days"`
	AdditionalParams  map[string]float64 `json:"additional_params"`
}

// AlgorithmResult represents the output of an algorithm
type AlgorithmResult struct {
	Signal      string             `json:"signal"`
	OrderType   string             `json:"order_type"`
	LimitPrice  *float64           `json:"limit_price,omitempty"`
	Weights     map[string]float64 `json:"weights,omitempty"`
	Confidence  float64            `json:"confidence"`
	Explanation string             `json:"explanation"`
}

// Algorithm defines the interface for all trading algorithms
type Algorithm interface {
	// Name returns the name of the algorithm
	Name() string

	// Type returns the type of the algorithm
	Type() AlgorithmType

	// Description returns a brief description of the algorithm
	Description() string

	// ParameterDescription returns a description of the parameters this algorithm accepts
	ParameterDescription() map[string]string

	// Configure configures the algorithm with the given parameters
	Configure(config AlgorithmConfig) error

	// Process processes the market data and returns a trading signal
	Process(symbol string, data *types.MarketData, historicalData []types.MarketData) (*AlgorithmResult, error)

	// Explain provides an explanation of how the algorithm made its decision
	Explain() string
}

// BaseAlgorithm provides a base implementation for all algorithms
type BaseAlgorithm struct {
	config      AlgorithmConfig
	lastRun     time.Time
	explanation string
}

// Configure configures the base algorithm
func (b *BaseAlgorithm) Configure(config AlgorithmConfig) error {
	b.config = config
	return nil
}

// Explain returns the explanation for the last run
func (b *BaseAlgorithm) Explain() string {
	return b.explanation
}

// FactoryFunc is a function that creates a new algorithm
type FactoryFunc func() Algorithm

// algorithmRegistry maintains a registry of available algorithms
var algorithmRegistry = make(map[AlgorithmType]FactoryFunc)

// Register registers an algorithm factory function
func Register(algType AlgorithmType, factory FactoryFunc) {
	algorithmRegistry[algType] = factory
}

// Create creates a new algorithm of the given type
func Create(algType AlgorithmType) (Algorithm, error) {
	factory, exists := algorithmRegistry[algType]
	if !exists {
		return nil, fmt.Errorf("unknown algorithm type: %s", algType)
	}
	return factory(), nil
}

// GetRegisteredAlgorithms returns all registered algorithm types
func GetRegisteredAlgorithms() []AlgorithmType {
	types := make([]AlgorithmType, 0, len(algorithmRegistry))
	for algType := range algorithmRegistry {
		types = append(types, algType)
	}
	return types
}
