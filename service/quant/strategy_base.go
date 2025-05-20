package quant

import (
	"fmt"
	"log"
	"math"
	"sort"
	"sync"

	"backend/models"
)

// BaseStrategy provides common functionality for all trading strategies
type BaseStrategy struct {
	// Name of the strategy
	name string

	// Description of the strategy
	description string

	// Parameters for the strategy
	params StrategyParams

	// Whether the strategy has been initialized
	initialized bool

	// Mutex for thread safety
	mu sync.RWMutex

	// Logger for strategy operations
	logger *log.Logger

	// Debug mode flag
	debug bool
}

// NewBaseStrategy creates a new base strategy with the given name and description
func NewBaseStrategy(name, description string) *BaseStrategy {
	return &BaseStrategy{
		name:        name,
		description: description,
		params:      make(StrategyParams),
		initialized: false,
		logger:      log.Default(),
		debug:       false,
	}
}

// Initialize sets up the strategy with parameters
func (b *BaseStrategy) Initialize(params StrategyParams) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Store parameters
	b.params = params

	// Set debug mode if specified
	if debugMode, ok := params["debug"].(bool); ok {
		b.debug = debugMode
	}

	b.initialized = true
	return nil
}

// Name returns the strategy name
func (b *BaseStrategy) Name() string {
	return b.name
}

// Description returns the strategy description
func (b *BaseStrategy) Description() string {
	return b.description
}

// DefaultParams returns default parameters for the strategy
func (b *BaseStrategy) DefaultParams() StrategyParams {
	return StrategyParams{
		"debug": false,
	}
}

// IsInitialized checks if the strategy has been properly initialized
func (b *BaseStrategy) IsInitialized() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.initialized
}

// ValidateData ensures there's enough data for analysis
func (b *BaseStrategy) ValidateData(data []models.Stock, minLength int) error {
	if len(data) < minLength {
		return &StrategyError{
			Err:        ErrInsufficientData,
			Message:    fmt.Sprintf("need at least %d data points, got %d", minLength, len(data)),
			StrategyID: b.name,
		}
	}
	return nil
}

// GetParam gets a parameter value with type assertion
func (b *BaseStrategy) GetParam(key string, defaultValue interface{}) interface{} {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if value, exists := b.params[key]; exists {
		return value
	}
	return defaultValue
}

// GetIntParam gets an integer parameter with a default value
func (b *BaseStrategy) GetIntParam(key string, defaultValue int) int {
	value := b.GetParam(key, defaultValue)
	if intValue, ok := value.(int); ok {
		return intValue
	}

	// Try to convert from float64 (JSON unmarshaling often produces float64)
	if floatValue, ok := value.(float64); ok {
		return int(floatValue)
	}

	return defaultValue
}

// GetFloatParam gets a float parameter with a default value
func (b *BaseStrategy) GetFloatParam(key string, defaultValue float64) float64 {
	value := b.GetParam(key, defaultValue)
	if floatValue, ok := value.(float64); ok {
		return floatValue
	}
	if intValue, ok := value.(int); ok {
		return float64(intValue)
	}
	return defaultValue
}

// GetBoolParam gets a boolean parameter with a default value
func (b *BaseStrategy) GetBoolParam(key string, defaultValue bool) bool {
	value := b.GetParam(key, defaultValue)
	if boolValue, ok := value.(bool); ok {
		return boolValue
	}
	return defaultValue
}

// GetStringParam gets a string parameter with a default value
func (b *BaseStrategy) GetStringParam(key string, defaultValue string) string {
	value := b.GetParam(key, defaultValue)
	if stringValue, ok := value.(string); ok {
		return stringValue
	}
	return defaultValue
}

// Log logs a message if debug mode is enabled
func (b *BaseStrategy) Log(format string, v ...interface{}) {
	if b.debug {
		b.logger.Printf(fmt.Sprintf("[%s] %s", b.name, format), v...)
	}
}

// SetLogger sets a custom logger for the strategy
func (b *BaseStrategy) SetLogger(logger *log.Logger) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.logger = logger
}

// --- Technical Indicators ---

// CalculateSMA computes Simple Moving Average for the given period
func (b *BaseStrategy) CalculateSMA(prices []float64, period int) []float64 {
	if len(prices) < period {
		return nil
	}

	result := make([]float64, len(prices))

	// First (period-1) elements have no SMA
	for i := 0; i < period-1; i++ {
		result[i] = math.NaN()
	}

	// Calculate SMA for the rest
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += prices[i]
	}
	result[period-1] = sum / float64(period)

	// Sliding window for the rest of the data
	for i := period; i < len(prices); i++ {
		sum = sum - prices[i-period] + prices[i]
		result[i] = sum / float64(period)
	}

	return result
}

// CalculateEMA computes Exponential Moving Average for the given period
func (b *BaseStrategy) CalculateEMA(prices []float64, period int) []float64 {
	if len(prices) < period {
		return nil
	}

	result := make([]float64, len(prices))

	// First (period-1) elements have no EMA
	for i := 0; i < period-1; i++ {
		result[i] = math.NaN()
	}

	// Initialize EMA with SMA for the first value
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += prices[i]
	}
	result[period-1] = sum / float64(period)

	// Calculate multiplier
	multiplier := 2.0 / float64(period+1)

	// Calculate EMA for the rest
	for i := period; i < len(prices); i++ {
		result[i] = (prices[i]-result[i-1])*multiplier + result[i-1]
	}

	return result
}

// CalculateRSI computes Relative Strength Index for the given period
func (b *BaseStrategy) CalculateRSI(prices []float64, period int) []float64 {
	if len(prices) < period+1 {
		return nil
	}

	result := make([]float64, len(prices))
	gains := make([]float64, len(prices))
	losses := make([]float64, len(prices))

	// Calculate price changes
	for i := 1; i < len(prices); i++ {
		change := prices[i] - prices[i-1]
		if change > 0 {
			gains[i] = change
			losses[i] = 0
		} else {
			gains[i] = 0
			losses[i] = -change
		}
	}

	// First period elements have no RSI
	for i := 0; i < period; i++ {
		result[i] = math.NaN()
	}

	// Calculate first Average Gain and Average Loss
	var avgGain, avgLoss float64
	for i := 1; i <= period; i++ {
		avgGain += gains[i]
		avgLoss += losses[i]
	}
	avgGain /= float64(period)
	avgLoss /= float64(period)

	// Calculate first RSI
	if avgLoss == 0 {
		result[period] = 100
	} else {
		rs := avgGain / avgLoss
		result[period] = 100 - (100 / (1 + rs))
	}

	// Calculate RSI for the rest of the data
	for i := period + 1; i < len(prices); i++ {
		// Use smoothed averages
		avgGain = ((avgGain * float64(period-1)) + gains[i]) / float64(period)
		avgLoss = ((avgLoss * float64(period-1)) + losses[i]) / float64(period)

		if avgLoss == 0 {
			result[i] = 100
		} else {
			rs := avgGain / avgLoss
			result[i] = 100 - (100 / (1 + rs))
		}
	}

	return result
}

// CalculateBollingerBands computes Bollinger Bands with given period and standard deviation multiplier
func (b *BaseStrategy) CalculateBollingerBands(prices []float64, period int, stdDevMultiplier float64) ([]float64, []float64, []float64) {
	if len(prices) < period {
		return nil, nil, nil
	}

	// Calculate SMA
	middle := b.CalculateSMA(prices, period)

	upper := make([]float64, len(prices))
	lower := make([]float64, len(prices))

	// First (period-1) elements have no bands
	for i := 0; i < period-1; i++ {
		upper[i] = math.NaN()
		lower[i] = math.NaN()
	}

	// Calculate bands for the rest
	for i := period - 1; i < len(prices); i++ {
		// Calculate standard deviation
		sum := 0.0
		for j := i - period + 1; j <= i; j++ {
			diff := prices[j] - middle[i]
			sum += diff * diff
		}
		stdDev := math.Sqrt(sum / float64(period))

		upper[i] = middle[i] + stdDevMultiplier*stdDev
		lower[i] = middle[i] - stdDevMultiplier*stdDev
	}

	return upper, middle, lower
}

// CalculateMACD computes Moving Average Convergence Divergence
func (b *BaseStrategy) CalculateMACD(prices []float64, fastPeriod, slowPeriod, signalPeriod int) ([]float64, []float64, []float64) {
	if len(prices) < slowPeriod {
		return nil, nil, nil
	}

	// Calculate fast and slow EMAs
	fastEMA := b.CalculateEMA(prices, fastPeriod)
	slowEMA := b.CalculateEMA(prices, slowPeriod)

	// Calculate MACD line
	macdLine := make([]float64, len(prices))
	for i := 0; i < len(prices); i++ {
		if i < slowPeriod-1 {
			macdLine[i] = math.NaN()
		} else {
			macdLine[i] = fastEMA[i] - slowEMA[i]
		}
	}

	// Calculate signal line (EMA of MACD line)
	signalLine := b.CalculateEMA(macdLine, signalPeriod)

	// Calculate histogram
	histogram := make([]float64, len(prices))
	for i := 0; i < len(prices); i++ {
		if i < slowPeriod+signalPeriod-2 {
			histogram[i] = math.NaN()
		} else {
			histogram[i] = macdLine[i] - signalLine[i]
		}
	}

	return macdLine, signalLine, histogram
}

// --- Data Manipulation Utilities ---

// ExtractPrices extracts a specific price field from stock data
func (b *BaseStrategy) ExtractPrices(stockData []models.Stock, field string) []float64 {
	prices := make([]float64, len(stockData))
	for i, stock := range stockData {
		switch field {
		case "open":
			prices[i] = stock.Open
		case "high":
			prices[i] = stock.High
		case "low":
			prices[i] = stock.Low
		case "close":
			prices[i] = stock.Close
		default:
			prices[i] = stock.Close // Default to close prices
		}
	}
	return prices
}

// NormalizeData normalizes a data series to [0, 1] range
func (b *BaseStrategy) NormalizeData(data []float64) []float64 {
	if len(data) == 0 {
		return nil
	}

	// Find min and max
	min, max := data[0], data[0]
	for _, value := range data {
		if value < min {
			min = value
		}
		if value > max {
			max = value
		}
	}

	// Handle case where all values are the same
	if min == max {
		normalized := make([]float64, len(data))
		for i := range normalized {
			normalized[i] = 0.5 // Middle of the range
		}
		return normalized
	}

	// Normalize
	normalized := make([]float64, len(data))
	scale := max - min
	for i, value := range data {
		normalized[i] = (value - min) / scale
	}

	return normalized
}

// ZScoreNormalize normalizes data using Z-score (mean 0, standard deviation 1)
func (b *BaseStrategy) ZScoreNormalize(data []float64) []float64 {
	if len(data) == 0 {
		return nil
	}

	// Calculate mean
	sum := 0.0
	for _, value := range data {
		sum += value
	}
	mean := sum / float64(len(data))

	// Calculate standard deviation
	sumSquaredDiff := 0.0
	for _, value := range data {
		diff := value - mean
		sumSquaredDiff += diff * diff
	}
	stdDev := math.Sqrt(sumSquaredDiff / float64(len(data)))

	// Handle case where standard deviation is zero
	if stdDev == 0 {
		normalized := make([]float64, len(data))
		for i := range normalized {
			normalized[i] = 0
		}
		return normalized
	}

	// Normalize
	normalized := make([]float64, len(data))
	for i, value := range data {
		normalized[i] = (value - mean) / stdDev
	}

	return normalized
}

// --- Position Sizing and Risk Management ---

// CalculatePositionSize determines position size based on risk parameters
func (b *BaseStrategy) CalculatePositionSize(capital float64, riskPercent float64, entryPrice, stopLoss float64) float64 {
	if entryPrice <= stopLoss { // For long positions
		return 0 // Invalid parameters
	}

	riskPerShare := entryPrice - stopLoss
	if riskPerShare <= 0 {
		return 0 // Invalid risk parameters
	}

	maxRiskAmount := capital * (riskPercent / 100.0)
	return maxRiskAmount / riskPerShare
}

// CalculateStopLoss calculates a stop loss price based on ATR or fixed percentage
func (b *BaseStrategy) CalculateStopLoss(isLong bool, entryPrice float64, atr float64, multiplier float64) float64 {
	if isLong {
		return entryPrice - (atr * multiplier)
	} else {
		return entryPrice + (atr * multiplier)
	}
}

// CalculateATR computes Average True Range for volatility measurement
func (b *BaseStrategy) CalculateATR(stockData []models.Stock, period int) []float64 {
	if len(stockData) < period+1 {
		return nil
	}

	trueRanges := make([]float64, len(stockData))

	// First element has no previous close
	trueRanges[0] = stockData[0].High - stockData[0].Low

	// Calculate true ranges
	for i := 1; i < len(stockData); i++ {
		tr1 := stockData[i].High - stockData[i].Low
		tr2 := math.Abs(stockData[i].High - stockData[i-1].Close)
		tr3 := math.Abs(stockData[i].Low - stockData[i-1].Close)

		trueRanges[i] = math.Max(tr1, math.Max(tr2, tr3))
	}

	// Calculate ATR using simple moving average
	return b.CalculateSMA(trueRanges, period)
}

// --- Performance Metrics for Backtesting ---

// CalculateSharpeRatio computes the Sharpe ratio for a returns series
func (b *BaseStrategy) CalculateSharpeRatio(returns []float64, riskFreeRate float64) float64 {
	if len(returns) < 2 {
		return 0
	}

	// Calculate excess returns
	excessReturns := make([]float64, len(returns))
	for i, ret := range returns {
		excessReturns[i] = ret - riskFreeRate
	}

	// Calculate mean and standard deviation
	sum := 0.0
	for _, er := range excessReturns {
		sum += er
	}
	mean := sum / float64(len(excessReturns))

	sumSquaredDiff := 0.0
	for _, er := range excessReturns {
		diff := er - mean
		sumSquaredDiff += diff * diff
	}
	stdDev := math.Sqrt(sumSquaredDiff / float64(len(excessReturns)))

	if stdDev == 0 {
		return 0
	}

	return mean / stdDev
}

// CalculateMaxDrawdown computes the maximum drawdown as a percentage
func (b *BaseStrategy) CalculateMaxDrawdown(equity []float64) float64 {
	if len(equity) < 2 {
		return 0
	}

	maxDrawdown := 0.0
	peak := equity[0]

	for _, value := range equity {
		if value > peak {
			peak = value
		}

		drawdown := (peak - value) / peak
		if drawdown > maxDrawdown {
			maxDrawdown = drawdown
		}
	}

	return maxDrawdown * 100 // Convert to percentage
}

// CalculateReturns computes percentage returns from an equity curve
func (b *BaseStrategy) CalculateReturns(equity []float64) []float64 {
	if len(equity) < 2 {
		return nil
	}

	returns := make([]float64, len(equity)-1)
	for i := 1; i < len(equity); i++ {
		returns[i-1] = (equity[i] - equity[i-1]) / equity[i-1]
	}

	return returns
}

// SortStocksByTime sorts stock data by timestamp
func (b *BaseStrategy) SortStocksByTime(stockData []models.Stock) []models.Stock {
	sorted := make([]models.Stock, len(stockData))
	copy(sorted, stockData)

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.Before(sorted[j].Timestamp)
	})

	return sorted
}
