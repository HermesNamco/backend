package quant

import (
	"backend/models"
	"context"
	"errors"
	"fmt"
	"math"
)

// MACrossStrategy implements a Moving Average Crossover strategy
type MACrossStrategy struct {
	*BaseStrategy
	// Specific parameters for the MA Cross strategy
	fastPeriod   int
	slowPeriod   int
	signalPeriod int
	maType       string // "sma", "ema", or "wma"
}

// NewMACrossStrategy creates a new Moving Average Crossover strategy
func NewMACrossStrategy() *MACrossStrategy {
	strategy := &MACrossStrategy{
		BaseStrategy: NewBaseStrategy(
			"MA Cross Strategy",
			"Moving Average Crossover strategy that generates buy signals when fast MA crosses above slow MA and sell signals when fast MA crosses below slow MA",
		),
	}
	return strategy
}

// Initialize sets up the MA Cross strategy with specific parameters
func (s *MACrossStrategy) Initialize(params StrategyParams) error {
	// First, initialize the base strategy
	if err := s.BaseStrategy.Initialize(params); err != nil {
		return err
	}

	// Get MA Cross specific parameters
	s.fastPeriod = s.GetIntParam("fastPeriod", 10)
	s.slowPeriod = s.GetIntParam("slowPeriod", 30)
	s.signalPeriod = s.GetIntParam("signalPeriod", 9)
	s.maType = s.GetStringParam("maType", "ema") // Default to EMA

	// Validate parameters
	if s.fastPeriod <= 0 || s.slowPeriod <= 0 {
		return &StrategyError{
			Err:        ErrInvalidParameters,
			Message:    "MA periods must be positive",
			StrategyID: s.Name(),
		}
	}

	if s.fastPeriod >= s.slowPeriod {
		return &StrategyError{
			Err:        ErrInvalidParameters,
			Message:    "Fast period must be less than slow period",
			StrategyID: s.Name(),
		}
	}

	if s.maType != "sma" && s.maType != "ema" && s.maType != "wma" {
		return &StrategyError{
			Err:        ErrInvalidParameters,
			Message:    "MA type must be sma, ema, or wma",
			StrategyID: s.Name(),
		}
	}

	s.Log("Initialized with parameters: fastPeriod=%d, slowPeriod=%d, maType=%s",
		s.fastPeriod, s.slowPeriod, s.maType)

	return nil
}

// DefaultParams returns default parameters for the MA Cross strategy
func (s *MACrossStrategy) DefaultParams() StrategyParams {
	params := s.BaseStrategy.DefaultParams()
	params["fastPeriod"] = 10
	params["slowPeriod"] = 30
	params["signalPeriod"] = 9
	params["maType"] = "ema"
	params["priceField"] = "close"
	return params
}

// GenerateSignals analyzes market data and generates trading signals
func (s *MACrossStrategy) GenerateSignals(ctx context.Context, stockData []models.Stock) ([]Signal, error) {
	// Validate data
	minDataPoints := s.slowPeriod + 5 // Need enough data for slow MA + some extra for crossovers
	if err := s.ValidateData(stockData, minDataPoints); err != nil {
		return nil, err
	}

	// Sort data by timestamp to ensure proper sequence
	sortedData := s.SortStocksByTime(stockData)

	// Extract price data based on parameter setting
	priceField := s.GetStringParam("priceField", "close")
	prices := s.ExtractPrices(sortedData, priceField)

	// Calculate MAs based on selected MA type
	var fastMA, slowMA []float64
	switch s.maType {
	case "sma":
		fastMA = s.CalculateSMA(prices, s.fastPeriod)
		slowMA = s.CalculateSMA(prices, s.slowPeriod)
	case "ema":
		fastMA = s.CalculateEMA(prices, s.fastPeriod)
		slowMA = s.CalculateEMA(prices, s.slowPeriod)
	case "wma":
		fastMA = s.CalculateWMA(prices, s.fastPeriod)
		slowMA = s.CalculateWMA(prices, s.slowPeriod)
	}

	// Generate signals based on MA crossovers
	signals := []Signal{}
	for i := 1; i < len(sortedData); i++ {
		// Skip data points where we don't have valid MAs yet
		if i < s.slowPeriod || math.IsNaN(fastMA[i]) || math.IsNaN(slowMA[i]) ||
			math.IsNaN(fastMA[i-1]) || math.IsNaN(slowMA[i-1]) {
			continue
		}

		// Check for crossover
		// Buy signal: Fast MA crosses above Slow MA
		if fastMA[i] > slowMA[i] && fastMA[i-1] <= slowMA[i-1] {
			signal := Signal{
				Type:      SignalBuy,
				Strength:  s.calculateSignalStrength(fastMA[i], slowMA[i], true),
				StockCode: sortedData[i].Code,
				Timestamp: sortedData[i].Timestamp,
				Price:     sortedData[i].Close,
				Notes:     fmt.Sprintf("Fast MA (%.2f) crossed above Slow MA (%.2f)", fastMA[i], slowMA[i]),
				Strategy:  s.Name(),
			}
			signals = append(signals, signal)
		} else if fastMA[i] < slowMA[i] && fastMA[i-1] >= slowMA[i-1] {
			// Sell signal: Fast MA crosses below Slow MA
			signal := Signal{
				Type:      SignalSell,
				Strength:  s.calculateSignalStrength(fastMA[i], slowMA[i], false),
				StockCode: sortedData[i].Code,
				Timestamp: sortedData[i].Timestamp,
				Price:     sortedData[i].Close,
				Notes:     fmt.Sprintf("Fast MA (%.2f) crossed below Slow MA (%.2f)", fastMA[i], slowMA[i]),
				Strategy:  s.Name(),
			}
			signals = append(signals, signal)
		}
	}

	return signals, nil
}

// calculateSignalStrength determines signal strength based on MA difference
// For buy signals, larger positive differences mean stronger signals
// For sell signals, larger negative differences mean stronger signals
func (s *MACrossStrategy) calculateSignalStrength(fast, slow float64, isBuy bool) float64 {
	diff := fast - slow
	absPercentage := math.Abs(diff/slow) * 100

	// Cap strength at 1.0
	if absPercentage > 5 {
		return 1.0
	}

	// Scale strength between 0.2 and 1.0 based on percentage difference
	strength := 0.2 + (absPercentage/5.0)*0.8
	if strength > 1.0 {
		strength = 1.0
	}

	return strength
}

// ExecuteSignal processes a trading signal and returns resulting position changes
func (s *MACrossStrategy) ExecuteSignal(ctx context.Context, signal Signal, positions []Position) ([]Position, error) {
	if !s.IsInitialized() {
		return nil, ErrStrategyNotInitialized
	}

	// Check if signal is valid
	if signal.StockCode == "" || (signal.Type != SignalBuy && signal.Type != SignalSell) {
		return nil, ErrInvalidSignal
	}

	// Default risk per trade as a percentage of capital (can be parameterized)
	riskPercent := s.GetFloatParam("riskPercent", 2.0)

	// Calculate total capital from existing positions or use default
	totalCapital := s.calculateTotalCapital(positions)
	if totalCapital <= 0 {
		totalCapital = s.GetFloatParam("initialCapital", 100000.0)
	}

	// Process signal based on type
	switch signal.Type {
	case SignalBuy:
		// For a buy signal, we create a new position
		// Skip if we already have a position in this stock
		for _, pos := range positions {
			if pos.StockCode == signal.StockCode {
				return positions, nil // Already have a position, no change
			}
		}

		// Calculate position size based on risk management rules
		// Here we use a simple fixed percentage of capital approach
		quantity := s.calculatePositionSize(totalCapital, riskPercent, signal.Price)
		if quantity <= 0 {
			return positions, nil // Invalid quantity, no change
		}

		// Calculate stop loss (e.g., 2% below entry price)
		stopLossPercent := s.GetFloatParam("stopLossPercent", 2.0)
		stopLoss := signal.Price * (1.0 - stopLossPercent/100.0)

		// Create new position
		newPosition := Position{
			StockCode:    signal.StockCode,
			EntryPrice:   signal.Price,
			CurrentPrice: signal.Price,
			Quantity:     quantity,
			EntryTime:    signal.Timestamp,
			StopLoss:     stopLoss,
			Strategy:     s.Name(),
		}

		// Add the new position to the existing positions
		return append(positions, newPosition), nil

	case SignalSell:
		// For a sell signal, we close any existing position in this stock
		result := []Position{}
		for _, pos := range positions {
			if pos.StockCode != signal.StockCode {
				result = append(result, pos) // Keep positions for other stocks
			}
		}
		return result, nil
	}

	// If we get here, it's an unexpected signal type
	return positions, nil
}

// calculateTotalCapital computes the total value of all positions
func (s *MACrossStrategy) calculateTotalCapital(positions []Position) float64 {
	total := 0.0
	for _, pos := range positions {
		total += pos.CurrentPrice * pos.Quantity
	}
	return total
}

// calculatePositionSize determines how many shares to buy based on risk parameters
func (s *MACrossStrategy) calculatePositionSize(capital, riskPercent, entryPrice float64) float64 {
	if entryPrice <= 0 {
		return 0
	}

	// Calculate position size based on a percentage of total capital
	positionValue := capital * (riskPercent / 100.0)
	shares := positionValue / entryPrice

	// Round down to nearest whole number of shares
	return math.Floor(shares)
}

// Backtest runs the strategy against historical data and returns performance metrics
func (s *MACrossStrategy) Backtest(ctx context.Context, stockData []models.Stock, initialCapital float64) (*BacktestResult, error) {
	if !s.IsInitialized() {
		return nil, ErrStrategyNotInitialized
	}

	if len(stockData) == 0 {
		return nil, ErrInsufficientData
	}

	// Sort data chronologically
	sortedData := s.SortStocksByTime(stockData)

	// Generate signals
	signals, err := s.GenerateSignals(ctx, sortedData)
	if err != nil {
		return nil, err
	}

	// Initialize backtest variables
	capital := initialCapital
	positions := []Position{}
	trades := []Trade{}
	equityCurve := []float64{initialCapital}

	// Process each stock data point in chronological order
	for i, data := range sortedData {
		// Skip the first few data points where MA isn't calculated yet
		if i < s.slowPeriod {
			continue
		}

		// Update current positions with latest price
		for j := range positions {
			positions[j].CurrentPrice = data.Close
			positions[j].UnrealizedPL = (data.Close - positions[j].EntryPrice) * positions[j].Quantity
		}

		// Check for a signal at this data point
		for _, signal := range signals {
			// Process signals that match this timestamp
			if signal.Timestamp.Equal(data.Timestamp) && signal.StockCode == data.Code {
				// If it's a buy signal and we don't already have a position
				if signal.Type == SignalBuy && !s.hasPosition(positions, signal.StockCode) {
					// Create a new position
					newPositions, err := s.ExecuteSignal(ctx, signal, positions)
					if err == nil {
						positions = newPositions
					}
				}

				// If it's a sell signal and we have a position
				if signal.Type == SignalSell {
					for _, pos := range positions {
						if pos.StockCode == signal.StockCode {
							// Record the trade
							trade := Trade{
								StockCode:     pos.StockCode,
								EntryTime:     pos.EntryTime,
								EntryPrice:    pos.EntryPrice,
								ExitTime:      signal.Timestamp,
								ExitPrice:     signal.Price,
								Quantity:      pos.Quantity,
								Direction:     "LONG",
								ProfitLoss:    (signal.Price - pos.EntryPrice) * pos.Quantity,
								ReturnRate:    (signal.Price - pos.EntryPrice) / pos.EntryPrice * 100,
								HoldingPeriod: signal.Timestamp.Sub(pos.EntryTime),
								ExitReason:    "Signal",
							}
							trades = append(trades, trade)

							// Update capital
							capital += trade.ProfitLoss

							// Close the position
							newPositions, err := s.ExecuteSignal(ctx, signal, positions)
							if err == nil {
								positions = newPositions
							}
						}
					}
				}
			}
		}

		// Record equity curve at this point
		// Capital + value of open positions
		equity := capital
		for _, pos := range positions {
			equity += pos.UnrealizedPL
		}
		equityCurve = append(equityCurve, equity)
	}

	// Close any remaining open positions at the end of the backtest
	finalStock := sortedData[len(sortedData)-1]
	for _, pos := range positions {
		trade := Trade{
			StockCode:     pos.StockCode,
			EntryTime:     pos.EntryTime,
			EntryPrice:    pos.EntryPrice,
			ExitTime:      finalStock.Timestamp,
			ExitPrice:     finalStock.Close,
			Quantity:      pos.Quantity,
			Direction:     "LONG",
			ProfitLoss:    (finalStock.Close - pos.EntryPrice) * pos.Quantity,
			ReturnRate:    (finalStock.Close - pos.EntryPrice) / pos.EntryPrice * 100,
			HoldingPeriod: finalStock.Timestamp.Sub(pos.EntryTime),
			ExitReason:    "End of Backtest",
		}
		trades = append(trades, trade)
		capital += trade.ProfitLoss
	}

	// Calculate backtest metrics
	winningTrades := 0
	losingTrades := 0
	for _, trade := range trades {
		if trade.ProfitLoss > 0 {
			winningTrades++
		} else if trade.ProfitLoss < 0 {
			losingTrades++
		}
	}

	// Calculate performance metrics
	returns := s.CalculateReturns(equityCurve)
	sharpeRatio := s.CalculateSharpeRatio(returns, 0.0) // Assuming 0 risk-free rate
	maxDrawdown := s.CalculateMaxDrawdown(equityCurve)

	// Calculate volatility (standard deviation of returns)
	var volatility float64
	if len(returns) > 1 {
		sumReturns := 0.0
		for _, r := range returns {
			sumReturns += r
		}
		meanReturn := sumReturns / float64(len(returns))

		sumSquaredDiff := 0.0
		for _, r := range returns {
			diff := r - meanReturn
			sumSquaredDiff += diff * diff
		}
		volatility = math.Sqrt(sumSquaredDiff / float64(len(returns)))
	}

	// Prepare result
	result := &BacktestResult{
		StartTime:      sortedData[0].Timestamp,
		EndTime:        sortedData[len(sortedData)-1].Timestamp,
		TotalTrades:    len(trades),
		WinningTrades:  winningTrades,
		LosingTrades:   losingTrades,
		InitialCapital: initialCapital,
		FinalCapital:   capital,
		ProfitLoss:     capital - initialCapital,
		ReturnRate:     (capital - initialCapital) / initialCapital * 100,
		MaxDrawdown:    maxDrawdown,
		SharpeRatio:    sharpeRatio,
		Volatility:     volatility,
		Trades:         trades,
		StrategyName:   s.Name(),
		Parameters:     s.params,
	}

	return result, nil
}

// hasPosition checks if there is already a position for the given stock code
func (s *MACrossStrategy) hasPosition(positions []Position, stockCode string) bool {
	for _, pos := range positions {
		if pos.StockCode == stockCode {
			return true
		}
	}
	return false
}

// CalculateWMA computes Weighted Moving Average for the given period
func (s *MACrossStrategy) CalculateWMA(prices []float64, period int) []float64 {
	if len(prices) < period {
		return nil
	}

	result := make([]float64, len(prices))

	// First (period-1) elements have no WMA
	for i := 0; i < period-1; i++ {
		result[i] = math.NaN()
	}

	// Calculate weights sum: 1+2+...+period = period*(period+1)/2
	weightsSum := period * (period + 1) / 2

	// Calculate WMA for the rest of the data
	for i := period - 1; i < len(prices); i++ {
		sum := 0.0
		weight := 1.0

		// Calculate weighted sum
		for j := i - period + 1; j <= i; j++ {
			sum += prices[j] * weight
			weight += 1.0
		}

		result[i] = sum / float64(weightsSum)
	}

	return result
}

// OptimizeParameters performs a grid search to find optimal MA parameters
func (s *MACrossStrategy) OptimizeParameters(ctx context.Context, stockData []models.Stock, initialCapital float64) (StrategyParams, *BacktestResult, error) {
	if len(stockData) == 0 {
		return nil, nil, ErrInsufficientData
	}

	// Define parameter ranges to test
	fastPeriods := []int{5, 8, 10, 12, 15, 20}
	slowPeriods := []int{20, 25, 30, 35, 40, 50}
	maTypes := []string{"sma", "ema", "wma"}

	bestResult := &BacktestResult{}
	var bestParams StrategyParams
	bestReturnRate := -math.MaxFloat64

	// Try all parameter combinations
	for _, fastPeriod := range fastPeriods {
		for _, slowPeriod := range slowPeriods {
			// Skip invalid combinations
			if fastPeriod >= slowPeriod {
				continue
			}

			for _, maType := range maTypes {
				// Configure strategy with this parameter set
				params := s.DefaultParams()
				params["fastPeriod"] = fastPeriod
				params["slowPeriod"] = slowPeriod
				params["maType"] = maType

				// Create a new strategy instance for this parameter set
				strategy := NewMACrossStrategy()
				err := strategy.Initialize(params)
				if err != nil {
					continue
				}

				// Run backtest with these parameters
				result, err := strategy.Backtest(ctx, stockData, initialCapital)
				if err != nil {
					continue
				}

				// Check if this is the best result so far
				// Here we're using return rate, but could use Sharpe ratio or other metrics
				if result.ReturnRate > bestReturnRate {
					bestReturnRate = result.ReturnRate
					bestParams = params
					bestResult = result
				}
			}
		}
	}

	// If no parameter set was successful, return error
	if bestParams == nil {
		return nil, nil, errors.New("optimization failed to find valid parameters")
	}

	return bestParams, bestResult, nil
}
