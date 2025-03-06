package algorithm

import (
	"log"
	"math/rand"
	"time"
)

// RecommendedTicker represents a ticker recommendation from the algorithm
type RecommendedTicker struct {
	Symbol     string  `json:"symbol"`
	Confidence float64 `json:"confidence"`
	Reasoning  string  `json:"reasoning"`
}

// RecommendTickers uses the AI to recommend tickers to watch based on market conditions
func (a *TradingAlgorithm) RecommendTickersList(sector string, maxResults int) ([]RecommendedTicker, error) {
	log.Printf("Generating ticker recommendations for sector: %s", sector)

	// This would typically involve making a request to Claude to analyze market conditions
	// and recommend tickers in the specified sector. For now, we'll provide sample responses
	// based on the sector.

	// In a real implementation, you'd create a prompt for Claude asking for ticker recommendations
	// in the specified sector with reasons, then parse the response.

	var recommendations []RecommendedTicker

	// Sample recommendations by sector
	if sector == "technology" {
		recommendations = []RecommendedTicker{
			{Symbol: "NVDA", Confidence: 0.85, Reasoning: "Strong AI and GPU market position"},
			{Symbol: "MSFT", Confidence: 0.82, Reasoning: "Cloud growth and AI integration"},
			{Symbol: "AMD", Confidence: 0.74, Reasoning: "Competitive positioning in CPU/GPU markets"},
		}
	} else if sector == "healthcare" {
		recommendations = []RecommendedTicker{
			{Symbol: "JNJ", Confidence: 0.78, Reasoning: "Stable dividend and diversified healthcare portfolio"},
			{Symbol: "ABBV", Confidence: 0.76, Reasoning: "Strong drug pipeline and recent approvals"},
		}
	} else {
		// Default recommendations
		recommendations = []RecommendedTicker{
			{Symbol: "AAPL", Confidence: 0.80, Reasoning: "Consistent performance and ecosystem strength"},
			{Symbol: "AMZN", Confidence: 0.78, Reasoning: "E-commerce dominance and AWS growth"},
		}
	}

	// Add some randomness to the recommendations
	// using a local random source
	source := rand.NewSource(time.Now().UnixNano())
	r := rand.New(source)
	r.Shuffle(len(recommendations), func(i, j int) {
		recommendations[i], recommendations[j] = recommendations[j], recommendations[i]
	})

	log.Printf("Generated %d ticker recommendations", len(recommendations))
	return recommendations[:minInt(len(recommendations), maxResults)], nil
}

// minInt returns the minimum of two integers
func minInt(a, b int) int {
	if a < b {
		return a
	} else {
		return b
	}
}
