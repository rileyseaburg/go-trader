package algo

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"gonum.org/v1/gonum/mat"
)

// stdDev calculates the standard deviation of a slice of float64 values
func stdDev(values []float64) float64 {
	if len(values) <= 1 {
		return 0
	}
	
	// Use our own implementation since we removed the stat package
	m := mean(values)
	var variance float64
	
	for _, v := range values {
		variance += math.Pow(v-m, 2)
	}
	
	variance /= float64(len(values) - 1)
	return math.Sqrt(variance)
}

// normalizeWeights ensures that a map of weights sums to 1.0
func normalizeWeights(weights map[string]float64) map[string]float64 {
	sum := 0.0
	for _, weight := range weights {
		sum += weight
	}
	
	// If sum is zero or very small, return equal weights
	if math.Abs(sum) < 1e-10 {
		equalWeight := 1.0 / float64(len(weights))
		result := make(map[string]float64)
		for key := range weights {
			result[key] = equalWeight
		}
		return result
	}
	
	// Normalize weights
	result := make(map[string]float64)
	for key, weight := range weights {
		result[key] = weight / sum
	}
	
	return result
}

// calculateOverlap measures the overlap between sample indices
func calculateOverlap(samples []int) float64 {
	if len(samples) <= 1 {
		return 0
	}
	
	overlap := 0.0
	
	// Count unique values in the sample
	uniqueCount := make(map[int]int)
	for _, idx := range samples {
		uniqueCount[idx]++
	}
	
	// Calculate overlap as 1 - (unique values / total values)
	uniqueRatio := float64(len(uniqueCount)) / float64(len(samples))
	overlap = 1.0 - uniqueRatio
	
	return overlap
}

// weightedChoice performs a weighted random selection from a slice
func weightedChoice(items []int, weights []float64) int {
	if len(items) == 0 || len(weights) == 0 || len(items) != len(weights) {
		if len(items) > 0 {
			// Fall back to uniform selection if weights are invalid
			return items[rand.Intn(len(items))]
		}
		return -1 // Error case, should not happen
	}
	
	// Sum of weights
	sum := 0.0
	for _, w := range weights {
		sum += w
	}
	
	// Normalize weights
	normalizedWeights := make([]float64, len(weights))
	for i, w := range weights {
		normalizedWeights[i] = w / sum
	}
	
	// Generate random number
	r := rand.Float64()
	
	// Select based on cumulative distribution
	cumulative := 0.0
	for i, w := range normalizedWeights {
		cumulative += w
		if r <= cumulative {
			return items[i]
		}
	}
	
	// Fallback (should rarely get here due to floating point precision)
	return items[len(items)-1]
}

// matrixToString converts a matrix to a string representation for debugging
func matrixToString(m *mat.Dense) string {
	r, c := m.Dims()
	result := ""
	
	for i := 0; i < r; i++ {
		for j := 0; j < c; j++ {
			if j > 0 {
				result += " "
			}
			result += fmt.Sprintf("%.1f", m.At(i, j))
		}
		result += "\n"
	}
	
	return result
}

// calculatePearsonCorrelation calculates the Pearson correlation coefficient between two slices
func calculatePearsonCorrelation(x, y []float64) float64 {
	if len(x) != len(y) || len(x) == 0 {
		return 0
	}
	
	n := float64(len(x))
	sumX, sumY := 0.0, 0.0
	sumXY := 0.0
	sumX2, sumY2 := 0.0, 0.0
	
	for i := 0; i < len(x); i++ {
		sumX += x[i]
		sumY += y[i]
		sumXY += x[i] * y[i]
		sumX2 += x[i] * x[i]
		sumY2 += y[i] * y[i]
	}
	
	numerator := n*sumXY - sumX*sumY
	denominator := math.Sqrt((n*sumX2 - sumX*sumX) * (n*sumY2 - sumY*sumY))
	
	if denominator == 0 {
		return 0
	}
	
	return numerator / denominator
}

// calculateSpearmanCorrelation calculates the Spearman rank correlation coefficient
func calculateSpearmanCorrelation(x, y []float64) float64 {
	if len(x) != len(y) || len(x) == 0 {
		return 0
	}
	
	// Convert to ranks
	xRanks := rankValues(x)
	yRanks := rankValues(y)
	
	// Use Pearson correlation on the ranks
	return calculatePearsonCorrelation(xRanks, yRanks)
}

// rankValues converts values to their ranks (handling ties)
func rankValues(values []float64) []float64 {
	n := len(values)
	if n == 0 {
		return []float64{}
	}
	
	// Create index-value pairs
	type indexValue struct {
		index int
		value float64
	}
	
	pairs := make([]indexValue, n)
	for i, v := range values {
		pairs[i] = indexValue{i, v}
	}
	
	// Sort by value
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].value < pairs[j].value
	})
	
	// Assign ranks (handling ties)
	ranks := make([]float64, n)
	for i := 0; i < n; {
		j := i
		// Find all elements with the same value
		for j < n && pairs[j].value == pairs[i].value {
			j++
		}
		
		// Calculate average rank for ties
		avgRank := float64(i+j-1) / 2.0 + 1.0
		
		// Assign the same rank to all tied elements
		for k := i; k < j; k++ {
			ranks[pairs[k].index] = avgRank
		}
		
		i = j
	}
	
	return ranks
}