package types

import "time"

// Constants for signal types
const (
	SignalBuy  = "buy"
	SignalSell = "sell"
	SignalHold = "hold"
)

// TradeSignal represents a trading signal
type TradeSignal struct {
	Symbol     string    `json:"symbol"`
	Signal     string    `json:"signal"`      // "buy", "sell", "hold"
	OrderType  string    `json:"order_type"`  // "market", "limit"
	LimitPrice *float64  `json:"limit_price"` // Only for limit orders
	Timestamp  time.Time `json:"timestamp"`
	Reasoning  string    `json:"reasoning"`
	Confidence *float64  `json:"confidence"`
}