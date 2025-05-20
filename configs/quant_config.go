package configs

// SignalType represents different trading signal types
type SignalType string

const (
	// SignalBuy represents a buy signal
	SignalBuy SignalType = "BUY"

	// SignalSell represents a sell signal
	SignalSell SignalType = "SELL"

	// SignalHold represents a hold signal (no action)
	SignalHold SignalType = "HOLD"
)

// DataTimeframe represents the timeframe for market data
type DataTimeframe string

const (
	// Timeframe1Min represents 1-minute data
	Timeframe1Min DataTimeframe = "1m"

	// Timeframe5Min represents 5-minute data
	Timeframe5Min DataTimeframe = "5m"

	// Timeframe15Min represents 15-minute data
	Timeframe15Min DataTimeframe = "15m"

	// Timeframe30Min represents 30-minute data
	Timeframe30Min DataTimeframe = "30m"

	// Timeframe1Hour represents 1-hour data
	Timeframe1Hour DataTimeframe = "1h"

	// Timeframe4Hour represents 4-hour data
	Timeframe4Hour DataTimeframe = "4h"

	// Timeframe1Day represents daily data
	Timeframe1Day DataTimeframe = "1d"
)
