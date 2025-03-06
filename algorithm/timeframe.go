package algorithm

import (
	"fmt"
	"github.com/alpacahq/alpaca-trade-api-go/v3/marketdata"
)

// ParseTimeFrame converts a string timeframe (e.g., "1D") to Alpaca's TimeFrame type
func ParseTimeFrame(timeframe string) (marketdata.TimeFrame, error) {
	switch timeframe {
	case "1Min":
		return marketdata.OneMin, nil
	case "5Min":
		// For 5 minute timeframe
		return marketdata.NewTimeFrame(5, marketdata.Min), nil
	case "15Min":
		// For 15 minute timeframe
		return marketdata.NewTimeFrame(15, marketdata.Min), nil
	case "1H":
		return marketdata.OneHour, nil
	case "1D":
		return marketdata.OneDay, nil
	default:
		// Default to 1 day if not specified correctly
		fmt.Printf("Unrecognized timeframe: %s, defaulting to 1D\n", timeframe)
		return marketdata.OneDay, nil
	}
}

// GetTimeFrameName returns a human-readable name for the timeframe
func GetTimeFrameName(tf marketdata.TimeFrame) string {
	switch tf {
	case marketdata.OneMin:
		return "1 Minute"
	case marketdata.OneHour:
		return "1 Hour"
	case marketdata.OneDay:
		return "1 Day"
	default:
		// Try to extract custom timeframes
		if tf.N == 5 && tf.Unit == marketdata.Min {
			return "5 Minutes"
		}
		if tf.N == 15 && tf.Unit == marketdata.Min {
			return "15 Minutes"
		}
		return fmt.Sprintf("%d %s", tf.N, tf.Unit)
	}
}