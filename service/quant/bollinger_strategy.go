package quant

import (
	"context"
	"fmt"
	"math"
	"time"

	"backend/models"
)

// BollingerBandsStrategy implements a Bollinger Bands strategy.
// It can operate in mean-reversion or trend-following mode.
type BollingerBandsStrategy struct {
	*BaseStrategy
	period           int     // Period for calculating the moving average and standard deviation.
	stdDevMultiplier float64 // Number of standard deviations for the upper and lower bands.
	strategyType     string  // "mean_reversion" or "trend_following"
	priceField       string  // Price field to use for calculations (e.g., "close", "open")
}

// NewBollingerBandsStrategy creates a new Bollinger Bands strategy instance.
func NewBollingerBandsStrategy() *BollingerBandsStrategy {
	strategy := &BollingerBandsStrategy{
		BaseStrategy: NewBaseStrategy(
			"Bollinger Bands Strategy",
			"Generates signals based on price interaction with Bollinger Bands.",
		),
	}
	return strategy
}

// Initialize sets up the Bollinger Bands strategy with specific parameters.
func (s *BollingerBandsStrategy) Initialize(params StrategyParams) error {
	if err := s.BaseStrategy.Initialize(params); err != nil {
		return err
	}

	s.period = s.GetIntParam("period", 20)
	s.stdDevMultiplier = s.GetFloatParam("stdDevMultiplier", 2.0)
	s.strategyType = s.GetStringParam("strategyType", "mean_reversion")
	s.priceField = s.GetStringParam("priceField", "close")

	if s.period <= 1 {
		return &StrategyError{
			Err:        ErrInvalidParameters,
			Message:    "period must be greater than 1",
			StrategyID: s.Name(),
		}
	}
	if s.stdDevMultiplier <= 0 {
		return &StrategyError{
			Err:        ErrInvalidParameters,
			Message:    "standard deviation multiplier must be positive",
			StrategyID: s.Name(),
		}
	}
	if s.strategyType != "mean_reversion" && s.strategyType != "trend_following" {
		return &StrategyError{
			Err:        ErrInvalidParameters,
			Message:    "strategyType must be 'mean_reversion' or 'trend_following'",
			StrategyID: s.Name(),
		}
	}

	s.Log("Initialized Bollinger Bands Strategy: Period=%d, StdDevMult=%.2f, Type=%s, PriceField=%s",
		s.period, s.stdDevMultiplier, s.strategyType, s.priceField)
	return nil
}

// DefaultParams returns default parameters for the Bollinger Bands strategy.
func (s *BollingerBandsStrategy) DefaultParams() StrategyParams {
	params := s.BaseStrategy.DefaultParams()
	params["period"] = 20
	params["stdDevMultiplier"] = 2.0
	params["strategyType"] = "mean_reversion"
	params["priceField"] = "close"        // e.g., close, open, high, low, typical ( (h+l+c)/3 )
	params["riskPercent"] = 1.5           // Risk 1.5% of capital per trade
	params["stopLossATRMultiplier"] = 1.5 // For ATR based stop-loss
	return params
}

// GenerateSignals analyzes market data and generates trading signals based on Bollinger Bands.
func (s *BollingerBandsStrategy) GenerateSignals(ctx context.Context, stockData []models.Stock) ([]Signal, error) {
	if !s.IsInitialized() {
		return nil, ErrStrategyNotInitialized
	}

	minDataPoints := s.period
	if err := s.ValidateData(stockData, minDataPoints); err != nil {
		return nil, err
	}

	sortedData := s.SortStocksByTime(stockData)
	prices := s.ExtractPrices(sortedData, s.priceField)

	upperBand, middleBand, lowerBand := s.CalculateBollingerBands(prices, s.period, s.stdDevMultiplier)
	if upperBand == nil || middleBand == nil || lowerBand == nil {
		return nil, &StrategyError{Err: ErrInsufficientData, Message: "failed to calculate Bollinger Bands", StrategyID: s.Name()}
	}

	bandwidth := s.CalculateBandwidth(upperBand, middleBand, lowerBand)

	signals := []Signal{}
	for i := 1; i < len(sortedData); i++ {
		// Ensure all band values are valid for current and previous points
		if math.IsNaN(upperBand[i]) || math.IsNaN(middleBand[i]) || math.IsNaN(lowerBand[i]) ||
			math.IsNaN(upperBand[i-1]) || math.IsNaN(middleBand[i-1]) || math.IsNaN(lowerBand[i-1]) {
			continue
		}

		currentPrice := prices[i]
		prevPrice := prices[i-1]
		currentStock := sortedData[i]

		var signal Signal
		generated := false

		if s.strategyType == "mean_reversion" {
			// Buy signal: Price touches or crosses below the lower band and then moves up
			if prevPrice <= lowerBand[i-1] && currentPrice > lowerBand[i] {
				signal = Signal{
					Type:      SignalBuy,
					Strength:  s.calculateSignalStrength(currentPrice, middleBand[i], lowerBand[i], upperBand[i], true),
					StockCode: currentStock.Code,
					Timestamp: currentStock.Timestamp,
					Price:     currentStock.Close, // Use closing price for actual trade
					Notes:     fmt.Sprintf("Mean Reversion: Price %.2f crossed above Lower Band %.2f. Bandwidth: %.4f", currentPrice, lowerBand[i], bandwidth[i]),
					Strategy:  s.Name(),
				}
				generated = true
			} else if prevPrice >= upperBand[i-1] && currentPrice < upperBand[i] {
				// Sell signal: Price touches or crosses above the upper band and then moves down
				signal = Signal{
					Type:      SignalSell,
					Strength:  s.calculateSignalStrength(currentPrice, middleBand[i], lowerBand[i], upperBand[i], false),
					StockCode: currentStock.Code,
					Timestamp: currentStock.Timestamp,
					Price:     currentStock.Close,
					Notes:     fmt.Sprintf("Mean Reversion: Price %.2f crossed below Upper Band %.2f. Bandwidth: %.4f", currentPrice, upperBand[i], bandwidth[i]),
					Strategy:  s.Name(),
				}
				generated = true
			}
		} else if s.strategyType == "trend_following" {
			// Buy signal: Price breaks out above the upper band
			if prevPrice <= upperBand[i-1] && currentPrice > upperBand[i] {
				signal = Signal{
					Type:      SignalBuy,
					Strength:  s.calculateSignalStrength(currentPrice, middleBand[i], lowerBand[i], upperBand[i], true),
					StockCode: currentStock.Code,
					Timestamp: currentStock.Timestamp,
					Price:     currentStock.Close,
					Notes:     fmt.Sprintf("Trend Following: Price %.2f broke above Upper Band %.2f. Bandwidth: %.4f", currentPrice, upperBand[i], bandwidth[i]),
					Strategy:  s.Name(),
				}
				generated = true
			} else if prevPrice >= lowerBand[i-1] && currentPrice < lowerBand[i] {
				// Sell signal: Price breaks out below the lower band
				signal = Signal{
					Type:      SignalSell,
					Strength:  s.calculateSignalStrength(currentPrice, middleBand[i], lowerBand[i], upperBand[i], false),
					StockCode: currentStock.Code,
					Timestamp: currentStock.Timestamp,
					Price:     currentStock.Close,
					Notes:     fmt.Sprintf("Trend Following: Price %.2f broke below Lower Band %.2f. Bandwidth: %.4f", currentPrice, lowerBand[i], bandwidth[i]),
					Strategy:  s.Name(),
				}
				generated = true
			}
		}

		if generated {
			signals = append(signals, signal)
		}
	}

	return signals, nil
}

// calculateSignalStrength for Bollinger Bands. For mean reversion, strength increases as price moves away from the band after touching.
// For trend following, strength increases as price moves further beyond the band.
func (s *BollingerBandsStrategy) calculateSignalStrength(price, middle, lower, upper float64, isBuy bool) float64 {
	bandRange := upper - lower
	if bandRange == 0 { // Avoid division by zero if bands are flat
		return 0.5
	}

	var strength float64
	if isBuy { // Buying
		if s.strategyType == "mean_reversion" { // Price moved above lower band
			strength = math.Min(1.0, math.Abs(price-lower)/bandRange*2) // Normalized distance from lower band
		} else { // Trend following, price broke above upper band
			strength = math.Min(1.0, math.Abs(price-upper)/bandRange*2) // Normalized distance from upper band
		}
	} else { // Selling
		if s.strategyType == "mean_reversion" { // Price moved below upper band
			strength = math.Min(1.0, math.Abs(upper-price)/bandRange*2) // Normalized distance from upper band
		} else { // Trend following, price broke below lower band
			strength = math.Min(1.0, math.Abs(lower-price)/bandRange*2) // Normalized distance from lower band
		}
	}
	// Ensure strength is between 0.1 (min evidence) and 1.0 (max evidence)
	return math.Max(0.1, strength)
}

// ExecuteSignal processes a trading signal (simplified version).
func (s *BollingerBandsStrategy) ExecuteSignal(ctx context.Context, signal Signal, positions []Position) ([]Position, error) {
	if !s.IsInitialized() {
		return nil, ErrStrategyNotInitialized
	}

	riskPercent := s.GetFloatParam("riskPercent", 1.0) // Default risk 1% of capital
	capital := s.GetFloatParam("initialCapital", 100000.0)
	// In a real system, capital would be managed dynamically.

	updatedPositions := make([]Position, 0, len(positions))
	positionFound := false

	for _, pos := range positions {
		if pos.StockCode == signal.StockCode {
			positionFound = true
			// If sell signal for an existing position, close it
			if signal.Type == SignalSell {
				s.Log("Closing position for %s at %.2f due to SELL signal", signal.StockCode, signal.Price)
				// Trade execution logic would go here (e.g., calculate P&L)
			} else {
				// If buy signal for existing position, hold or add (not implemented here)
				updatedPositions = append(updatedPositions, pos)
			}
		} else {
			updatedPositions = append(updatedPositions, pos)
		}
	}

	// If buy signal and no existing position for this stock, open a new one
	if signal.Type == SignalBuy && !positionFound {
		quantity := s.CalculatePositionSize(capital, riskPercent, signal.Price, 0) // Stop-loss logic needed
		if quantity > 0 {
			stopLoss := signal.Price * (1 - s.GetFloatParam("stopLossATRMultiplier", 2.0)/100) // Placeholder stop-loss
			newPos := Position{
				StockCode:    signal.StockCode,
				EntryPrice:   signal.Price,
				CurrentPrice: signal.Price,
				Quantity:     quantity,
				EntryTime:    signal.Timestamp,
				Strategy:     s.Name(),
				StopLoss:     stopLoss,
			}
			s.Log("Opening new position for %s: Qty %.2f at %.2f", signal.StockCode, quantity, signal.Price)
			updatedPositions = append(updatedPositions, newPos)
		}
	}
	return updatedPositions, nil
}

// Backtest runs the strategy against historical data.
func (s *BollingerBandsStrategy) Backtest(ctx context.Context, stockData []models.Stock, initialCapital float64) (*BacktestResult, error) {
	if !s.IsInitialized() {
		return nil, ErrStrategyNotInitialized
	}
	if len(stockData) == 0 {
		return nil, &StrategyError{Err: ErrInsufficientData, Message: "no stock data provided for backtest", StrategyID: s.Name()}
	}

	sortedData := s.SortStocksByTime(stockData)
	signals, err := s.GenerateSignals(ctx, sortedData)
	if err != nil {
		return nil, &StrategyError{Err: err, Message: "failed to generate signals for backtest", StrategyID: s.Name()}
	}

	capital := initialCapital
	var currentPosition *Position
	trades := []Trade{}
	equityCurve := []float64{initialCapital}

	// Create a map of signals by timestamp for quick lookup
	signalsByTime := make(map[time.Time][]Signal)
	for _, sig := range signals {
		signalsByTime[sig.Timestamp] = append(signalsByTime[sig.Timestamp], sig)
	}

	for _, bar := range sortedData {
		// Update current position's price and P&L
		if currentPosition != nil {
			currentPosition.CurrentPrice = bar.Close
			currentPosition.UnrealizedPL = (bar.Close - currentPosition.EntryPrice) * currentPosition.Quantity
		}

		// Check for signals at this bar's timestamp
		if currentSignals, ok := signalsByTime[bar.Timestamp]; ok {
			for _, sig := range currentSignals {
				if sig.StockCode != bar.Code { // Ensure signal is for the current stock (if multi-stock backtest)
					continue
				}

				if sig.Type == SignalBuy && currentPosition == nil {
					// Simplified position sizing for backtest
					quantity := math.Floor(capital * 0.1 / sig.Price) // Use 10% of capital
					if quantity > 0 {
						cost := quantity * sig.Price
						if capital >= cost {
							capital -= cost
							currentPosition = &Position{
								StockCode:    sig.StockCode,
								EntryPrice:   sig.Price,
								CurrentPrice: sig.Price,
								Quantity:     quantity,
								EntryTime:    sig.Timestamp,
								Strategy:     s.Name(),
							}
							s.Log("[Backtest] BUY %s: Qty %.2f @ %.2f on %s", sig.StockCode, quantity, sig.Price, sig.Timestamp.Format("2006-01-02"))
						}
					}
				} else if sig.Type == SignalSell && currentPosition != nil && currentPosition.StockCode == sig.StockCode {
					proceeds := currentPosition.Quantity * sig.Price
					trade := Trade{
						StockCode:     currentPosition.StockCode,
						EntryTime:     currentPosition.EntryTime,
						EntryPrice:    currentPosition.EntryPrice,
						ExitTime:      sig.Timestamp,
						ExitPrice:     sig.Price,
						Quantity:      currentPosition.Quantity,
						Direction:     "LONG", // Assuming long only for simplicity
						ProfitLoss:    proceeds - (currentPosition.Quantity * currentPosition.EntryPrice),
						HoldingPeriod: sig.Timestamp.Sub(currentPosition.EntryTime),
						ExitReason:    "Signal",
					}
					trade.ReturnRate = (trade.ProfitLoss / (currentPosition.Quantity * currentPosition.EntryPrice)) * 100
					trades = append(trades, trade)
					capital += proceeds
					currentPosition = nil
					s.Log("[Backtest] SELL %s: Qty %.2f @ %.2f on %s, P&L: %.2f", sig.StockCode, trade.Quantity, sig.Price, sig.Timestamp.Format("2006-01-02"), trade.ProfitLoss)
				}
			}
		}

		currentEquity := capital
		if currentPosition != nil {
			currentEquity += currentPosition.Quantity * currentPosition.CurrentPrice
		}
		equityCurve = append(equityCurve, currentEquity)
	}

	// Close any open position at the end of the backtest period
	if currentPosition != nil {
		lastBar := sortedData[len(sortedData)-1]
		proceeds := currentPosition.Quantity * lastBar.Close
		trade := Trade{
			StockCode:     currentPosition.StockCode,
			EntryTime:     currentPosition.EntryTime,
			EntryPrice:    currentPosition.EntryPrice,
			ExitTime:      lastBar.Timestamp,
			ExitPrice:     lastBar.Close,
			Quantity:      currentPosition.Quantity,
			Direction:     "LONG",
			ProfitLoss:    proceeds - (currentPosition.Quantity * currentPosition.EntryPrice),
			HoldingPeriod: lastBar.Timestamp.Sub(currentPosition.EntryTime),
			ExitReason:    "End of backtest",
		}
		trade.ReturnRate = (trade.ProfitLoss / (currentPosition.Quantity * currentPosition.EntryPrice)) * 100
		trades = append(trades, trade)
		capital += proceeds
		currentPosition = nil
		s.Log("[Backtest] Close EOD %s: Qty %.2f @ %.2f on %s, P&L: %.2f", trade.StockCode, trade.Quantity, trade.ExitPrice, trade.ExitTime.Format("2006-01-02"), trade.ProfitLoss)
	}

	// Calculate performance metrics
	finalCapital := capital
	winCount := 0
	lossCount := 0
	for _, tr := range trades {
		if tr.ProfitLoss > 0 {
			winCount++
		} else if tr.ProfitLoss < 0 {
			lossCount++
		}
	}

	returns := s.CalculateReturns(equityCurve)
	sharpe := s.CalculateSharpeRatio(returns, 0.0) // Risk-free rate assumed to be 0
	maxDD := s.CalculateMaxDrawdown(equityCurve)
	vol := 0.0
	if len(returns) > 1 {
		meanReturn := 0.0
		for _, r := range returns {
			meanReturn += r
		}
		meanReturn /= float64(len(returns))
		sumSqDiff := 0.0
		for _, r := range returns {
			sumSqDiff += math.Pow(r-meanReturn, 2)
		}
		vol = math.Sqrt(sumSqDiff / float64(len(returns)))
	}

	return &BacktestResult{
		StartTime:      sortedData[0].Timestamp,
		EndTime:        sortedData[len(sortedData)-1].Timestamp,
		TotalTrades:    len(trades),
		WinningTrades:  winCount,
		LosingTrades:   lossCount,
		InitialCapital: initialCapital,
		FinalCapital:   finalCapital,
		ProfitLoss:     finalCapital - initialCapital,
		ReturnRate:     (finalCapital - initialCapital) / initialCapital * 100,
		MaxDrawdown:    maxDD,
		SharpeRatio:    sharpe,
		Volatility:     vol,
		Trades:         trades,
		StrategyName:   s.Name(),
		Parameters:     s.params,
	}, nil
}

// CalculateBandwidth computes the Bollinger Bandwidth indicator.
// Bandwidth = (Upper Band - Lower Band) / Middle Band
func (s *BollingerBandsStrategy) CalculateBandwidth(upperBand, middleBand, lowerBand []float64) []float64 {
	length := len(middleBand)
	if length == 0 || len(upperBand) != length || len(lowerBand) != length {
		return nil
	}

	bandwidth := make([]float64, length)
	for i := 0; i < length; i++ {
		if math.IsNaN(upperBand[i]) || math.IsNaN(lowerBand[i]) || math.IsNaN(middleBand[i]) || middleBand[i] == 0 {
			bandwidth[i] = math.NaN()
		} else {
			bandwidth[i] = (upperBand[i] - lowerBand[i]) / middleBand[i]
		}
	}
	return bandwidth
}

// GetBollingerBandsData returns the calculated bands for visualization or further analysis.
// This method assumes that GenerateSignals (or a similar calculation) has been run.
// In a real scenario, this might be part of a results structure or called explicitly.
func (s *BollingerBandsStrategy) GetBollingerBandsData(prices []float64) (upper []float64, middle []float64, lower []float64, bandwidth []float64, err error) {
	if !s.IsInitialized() {
		return nil, nil, nil, nil, ErrStrategyNotInitialized
	}
	if len(prices) < s.period {
		return nil, nil, nil, nil, ErrInsufficientData
	}

	upper, middle, lower = s.BaseStrategy.CalculateBollingerBands(prices, s.period, s.stdDevMultiplier)
	if upper == nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to calculate Bollinger Bands components")
	}
	bandwidth = s.CalculateBandwidth(upper, middle, lower)
	return upper, middle, lower, bandwidth, nil
}

// OptimizeParameters is a placeholder for parameter optimization.
func (s *BollingerBandsStrategy) OptimizeParameters(ctx context.Context, stockData []models.Stock, initialCapital float64) (StrategyParams, *BacktestResult, error) {
	s.Log("Optimization for BollingerBandsStrategy is not yet implemented.")
	// Example: Iterate over different periods and stdDevMultipliers
	// For each combination, run Backtest and find the best performing set.
	// This is a complex task and would typically involve a systematic search.

	// Fallback to default params and current backtest result
	currentParams := s.DefaultParams()
	selfCopy := NewBollingerBandsStrategy() // Create a new instance to avoid modifying the original
	if err := selfCopy.Initialize(currentParams); err != nil {
		return nil, nil, err
	}
	result, err := selfCopy.Backtest(ctx, stockData, initialCapital)
	if err != nil {
		return nil, nil, err
	}
	return currentParams, result, fmt.Errorf("optimization not implemented, returning default backtest")
}
