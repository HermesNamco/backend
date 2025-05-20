package quant

import (
	"context"
	"fmt"
	"math"
	"time"

	"backend/models"
)

// MomentumStrategy implements a momentum-based trading strategy.
// It can use Rate of Change (ROC) and optionally confirm with MACD.
type MomentumStrategy struct {
	*BaseStrategy
	lookbackPeriod   int     // For ROC calculation
	rocThresholdBuy  float64 // ROC value above which a buy signal is considered
	rocThresholdSell float64 // ROC value below which a sell signal is considered
	smoothingPeriod  int     // Period for smoothing ROC (0 for no smoothing)
	macdConfirm      bool    // Whether to use MACD for trend confirmation
	// MACD specific params will be inherited if macdConfirm is true
	// macdFastPeriod, macdSlowPeriod, macdSignalPeriod
	atrPeriod         int     // Period for ATR calculation (for volatility-adjusted position sizing)
	atrStopMultiplier float64 // Multiplier for ATR to set stop-loss
	priceField        string  // Price field for calculations ("close", "open", etc.)
}

// NewMomentumStrategy creates a new Momentum strategy instance.
func NewMomentumStrategy() *MomentumStrategy {
	strategy := &MomentumStrategy{
		BaseStrategy: NewBaseStrategy(
			"Momentum Strategy",
			"Generates signals based on price momentum (ROC) and optionally confirmed by MACD trend.",
		),
	}
	return strategy
}

// Initialize sets up the Momentum strategy with specific parameters.
func (s *MomentumStrategy) Initialize(params StrategyParams) error {
	if err := s.BaseStrategy.Initialize(params); err != nil {
		return err
	}

	s.lookbackPeriod = s.GetIntParam("lookbackPeriod", 14)
	s.rocThresholdBuy = s.GetFloatParam("rocThresholdBuy", 2.0)    // e.g., ROC > 2%
	s.rocThresholdSell = s.GetFloatParam("rocThresholdSell", -2.0) // e.g., ROC < -2%
	s.smoothingPeriod = s.GetIntParam("smoothingPeriod", 0)        // Default no smoothing
	s.macdConfirm = s.GetBoolParam("macdConfirm", false)           // Default no MACD confirmation
	s.atrPeriod = s.GetIntParam("atrPeriod", 14)
	s.atrStopMultiplier = s.GetFloatParam("atrStopMultiplier", 2.0)
	s.priceField = s.GetStringParam("priceField", "close")

	if s.lookbackPeriod <= 0 {
		return &StrategyError{Err: ErrInvalidParameters, Message: "lookbackPeriod must be positive", StrategyID: s.Name()}
	}
	if s.smoothingPeriod < 0 {
		return &StrategyError{Err: ErrInvalidParameters, Message: "smoothingPeriod cannot be negative", StrategyID: s.Name()}
	}
	if s.atrPeriod <= 0 {
		return &StrategyError{Err: ErrInvalidParameters, Message: "atrPeriod must be positive", StrategyID: s.Name()}
	}
	if s.atrStopMultiplier <= 0 {
		return &StrategyError{Err: ErrInvalidParameters, Message: "atrStopMultiplier must be positive", StrategyID: s.Name()}
	}

	s.Log("Initialized Momentum Strategy: Lookback=%d, ROCBuyTh=%.2f, ROCSellTh=%.2f, Smoothing=%d, MACDConfirm=%t, ATRPeriod=%d, ATRStopMult=%.2f",
		s.lookbackPeriod, s.rocThresholdBuy, s.rocThresholdSell, s.smoothingPeriod, s.macdConfirm, s.atrPeriod, s.atrStopMultiplier)
	return nil
}

// DefaultParams returns default parameters for the Momentum strategy.
func (s *MomentumStrategy) DefaultParams() StrategyParams {
	params := s.BaseStrategy.DefaultParams()
	params["lookbackPeriod"] = 14
	params["rocThresholdBuy"] = 2.0
	params["rocThresholdSell"] = -2.0
	params["smoothingPeriod"] = 0 // No smoothing by default
	params["macdConfirm"] = false
	// MACD defaults from BaseStrategy will be used if macdConfirm is true
	// or could be set here explicitly if different defaults are desired for this strategy
	params["macdFastPeriod"] = 12  // Default for BaseStrategy.CalculateMACD
	params["macdSlowPeriod"] = 26  // Default for BaseStrategy.CalculateMACD
	params["macdSignalPeriod"] = 9 // Default for BaseStrategy.CalculateMACD
	params["atrPeriod"] = 14
	params["atrStopMultiplier"] = 2.0
	params["priceField"] = "close"
	params["riskPercent"] = 2.0 // Default risk per trade
	return params
}

// CalculateROC calculates the Rate of Change indicator.
// ROC = [(Current Price - Price N periods ago) / Price N periods ago] * 100
func (s *MomentumStrategy) CalculateROC(prices []float64, period int) []float64 {
	if len(prices) < period+1 { // Need at least period+1 prices to have one ROC value
		return nil
	}
	roc := make([]float64, len(prices))
	for i := 0; i < period; i++ {
		roc[i] = math.NaN() // Not enough data for ROC
	}
	for i := period; i < len(prices); i++ {
		if prices[i-period] == 0 { // Avoid division by zero
			roc[i] = math.NaN()
		} else {
			roc[i] = ((prices[i] - prices[i-period]) / prices[i-period]) * 100
		}
	}
	return roc
}

// GenerateSignals analyzes market data and generates trading signals.
func (s *MomentumStrategy) GenerateSignals(ctx context.Context, stockData []models.Stock) ([]Signal, error) {
	if !s.IsInitialized() {
		return nil, ErrStrategyNotInitialized
	}

	// Determine minimum data points needed
	minDataPoints := s.lookbackPeriod + 1 // For ROC
	if s.smoothingPeriod > 0 {
		minDataPoints = s.lookbackPeriod + s.smoothingPeriod // For ROC then SMA of ROC
	}
	if s.macdConfirm {
		macdSlowPeriod := s.GetIntParam("macdSlowPeriod", 26)
		macdSignalPeriod := s.GetIntParam("macdSignalPeriod", 9)
		// MACD itself needs slowPeriod, then signal line needs signalPeriod on top of that
		macdMinPoints := macdSlowPeriod + macdSignalPeriod - 1
		if macdMinPoints > minDataPoints {
			minDataPoints = macdMinPoints
		}
	}

	if err := s.ValidateData(stockData, minDataPoints); err != nil {
		return nil, err
	}

	sortedData := s.SortStocksByTime(stockData)
	prices := s.ExtractPrices(sortedData, s.priceField)

	rocValues := s.CalculateROC(prices, s.lookbackPeriod)
	if rocValues == nil {
		return nil, &StrategyError{Err: ErrInsufficientData, Message: "failed to calculate ROC", StrategyID: s.Name()}
	}

	momentumSignalValues := rocValues
	if s.smoothingPeriod > 0 {
		smoothedROC := s.CalculateSMA(rocValues, s.smoothingPeriod)
		if smoothedROC == nil {
			return nil, &StrategyError{Err: ErrInsufficientData, Message: "failed to calculate smoothed ROC", StrategyID: s.Name()}
		}
		momentumSignalValues = smoothedROC
	}

	var macdLine, macdSignalLine []float64
	if s.macdConfirm {
		macdFastPeriod := s.GetIntParam("macdFastPeriod", 12)
		macdSlowPeriod := s.GetIntParam("macdSlowPeriod", 26)
		macdSignalPeriod := s.GetIntParam("macdSignalPeriod", 9)
		var macdHist []float64 // Not used directly for signals here, but returned by CalculateMACD
		macdLine, macdSignalLine, macdHist = s.CalculateMACD(prices, macdFastPeriod, macdSlowPeriod, macdSignalPeriod)
		if macdLine == nil || macdSignalLine == nil || macdHist == nil {
			return nil, &StrategyError{Err: ErrInsufficientData, Message: "failed to calculate MACD", StrategyID: s.Name()}
		}
	}

	signals := []Signal{}
	for i := 1; i < len(sortedData); i++ { // Start from 1 to check previous values
		currentStock := sortedData[i]
		currentMomentum := momentumSignalValues[i]

		if math.IsNaN(currentMomentum) {
			continue
		}

		trendConfirmationBuy := true
		trendConfirmationSell := true

		if s.macdConfirm {
			if math.IsNaN(macdLine[i]) || math.IsNaN(macdSignalLine[i]) || math.IsNaN(macdLine[i-1]) || math.IsNaN(macdSignalLine[i-1]) {
				continue // Not enough data for MACD confirmation yet
			}
			trendConfirmationBuy = macdLine[i] > macdSignalLine[i] && macdLine[i-1] <= macdSignalLine[i-1]  // MACD bullish crossover
			trendConfirmationSell = macdLine[i] < macdSignalLine[i] && macdLine[i-1] >= macdSignalLine[i-1] // MACD bearish crossover
		}

		var signalType SignalType = SignalHold
		notes := ""

		// Buy Signal
		if currentMomentum > s.rocThresholdBuy {
			if s.macdConfirm {
				if trendConfirmationBuy {
					signalType = SignalBuy
					notes = fmt.Sprintf("ROC (%.2f) > Threshold (%.2f) with MACD bullish confirmation.", currentMomentum, s.rocThresholdBuy)
				} else {
					notes = fmt.Sprintf("ROC (%.2f) > Threshold (%.2f) but MACD not bullish.", currentMomentum, s.rocThresholdBuy)
				}
			} else {
				signalType = SignalBuy
				notes = fmt.Sprintf("ROC (%.2f) > Threshold (%.2f).", currentMomentum, s.rocThresholdBuy)
			}
		}

		// Sell Signal
		if currentMomentum < s.rocThresholdSell {
			if s.macdConfirm {
				if trendConfirmationSell {
					signalType = SignalSell
					notes = fmt.Sprintf("ROC (%.2f) < Threshold (%.2f) with MACD bearish confirmation.", currentMomentum, s.rocThresholdSell)
				} else {
					notes = fmt.Sprintf("ROC (%.2f) < Threshold (%.2f) but MACD not bearish.", currentMomentum, s.rocThresholdSell)
				}
			} else {
				signalType = SignalSell
				notes = fmt.Sprintf("ROC (%.2f) < Threshold (%.2f).", currentMomentum, s.rocThresholdSell)
			}
		}

		if signalType != SignalHold {
			strength := math.Min(1.0, math.Abs(currentMomentum)/math.Max(math.Abs(s.rocThresholdBuy), math.Abs(s.rocThresholdSell))/2.0) // Normalized strength
			signals = append(signals, Signal{
				Type:      signalType,
				Strength:  math.Max(0.1, strength), // Ensure minimum strength
				StockCode: currentStock.Code,
				Timestamp: currentStock.Timestamp,
				Price:     currentStock.Close,
				Notes:     notes,
				Strategy:  s.Name(),
			})
		}
	}
	return signals, nil
}

// calculateVolatilityAdjustedPositionSize determines position size based on risk and ATR.
func (s *MomentumStrategy) calculateVolatilityAdjustedPositionSize(capital, riskPercent, entryPrice float64, atrAtEntry float64) float64 {
	if entryPrice <= 0 || atrAtEntry <= 0 {
		return 0
	}

	riskAmount := capital * (riskPercent / 100.0)
	// Stop loss is defined by ATR * multiplier away from entry
	stopLossDistance := atrAtEntry * s.atrStopMultiplier
	if stopLossDistance == 0 { // Avoid division by zero if ATR is somehow zero or multiplier is zero
		return 0
	}

	shares := riskAmount / stopLossDistance
	return math.Floor(shares) // Number of shares
}

// ExecuteSignal processes a trading signal, adjusting for volatility in position sizing.
func (s *MomentumStrategy) ExecuteSignal(ctx context.Context, signal Signal, positions []Position) ([]Position, error) {
	if !s.IsInitialized() {
		return nil, ErrStrategyNotInitialized
	}

	capital := s.GetFloatParam("initialCapital", 100000.0) // Simplified capital management
	riskPercent := s.GetFloatParam("riskPercent", 2.0)

	// Fetch current ATR for the stock at the time of the signal for position sizing
	// This requires access to historical data up to signal.Timestamp
	// For simplicity in this example, we'll assume ATR is passed or calculated externally before this call
	// In a real system, this might involve querying a data provider or cache.
	// As a placeholder, we will try to calculate it if we have data, otherwise use a fixed risk.
	var atrAtSignal float64 = signal.Price * 0.02 // Default to 2% of price if ATR not available. ATR calculation needs historical data.

	// In a real execution engine, ATR would be available from the data feed or pre-calculated for the signal's timestamp.
	// For this example, we acknowledge it's a placeholder. In a backtest, we can calculate it based on historical data up to this point.

	updatedPositions := make([]Position, 0, len(positions))
	positionFoundAndClosed := false // Flag to track if a position was closed for the current signal's stock
	positionFoundAndHeld := false   // Flag to track if a buy signal came for an existing long position

	for _, pos := range positions {
		if pos.StockCode == signal.StockCode {
			if signal.Type == SignalSell {
				// Close existing long position
				s.Log("Closing position for %s at %.2f due to SELL signal", signal.StockCode, signal.Price)
				positionFoundAndClosed = true
				// Actual trade execution (P&L calculation) would happen in a portfolio manager or backtester
			} else if signal.Type == SignalBuy {
				// Already in a buy position, do nothing or could implement pyramiding based on strategy rules
				s.Log("Already in BUY position for %s, no action on new BUY signal", signal.StockCode)
				updatedPositions = append(updatedPositions, pos) // Keep existing position
				positionFoundAndHeld = true
			} else {
				updatedPositions = append(updatedPositions, pos) // Keep if not closing and not relevant signal type
			}
		} else {
			updatedPositions = append(updatedPositions, pos) // Keep positions for other stocks
		}
	}

	if signal.Type == SignalBuy && !positionFoundAndClosed && !positionFoundAndHeld {
		// This is where ATR calculation for the specific signal time would be ideal.
		// Using the placeholder atrAtSignal for now.
		quantity := s.calculateVolatilityAdjustedPositionSize(capital, riskPercent, signal.Price, atrAtSignal)
		if quantity > 0 {
			stopLossPrice := signal.Price - (atrAtSignal * s.atrStopMultiplier)
			newPos := Position{
				StockCode:    signal.StockCode,
				EntryPrice:   signal.Price,
				CurrentPrice: signal.Price,
				Quantity:     quantity,
				EntryTime:    signal.Timestamp,
				StopLoss:     stopLossPrice,
				Strategy:     s.Name(),
			}
			s.Log("Opening new position for %s: Qty %.2f at %.2f, SL %.2f (ATR %.2f)", signal.StockCode, quantity, signal.Price, stopLossPrice, atrAtSignal)
			updatedPositions = append(updatedPositions, newPos)
		}
	}

	return updatedPositions, nil
}

// Backtest runs the strategy against historical data.
func (s *MomentumStrategy) Backtest(ctx context.Context, stockData []models.Stock, initialCapital float64) (*BacktestResult, error) {
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

	atrValues := s.CalculateATR(sortedData, s.atrPeriod)
	if atrValues == nil && s.atrStopMultiplier > 0 {
		s.Log("Warning: Could not calculate ATR for backtest; using fallback for position sizing/stops.")
		atrValues = make([]float64, len(sortedData))
		for i := range atrValues {
			atrValues[i] = sortedData[i].Close * 0.02
		} // Fallback to 2% of close price
	}

	capital := initialCapital
	var currentPosition *Position
	trades := []Trade{}
	equityCurve := []float64{initialCapital}

	signalsByTime := make(map[time.Time][]Signal)
	for _, sig := range signals {
		signalsByTime[sig.Timestamp] = append(signalsByTime[sig.Timestamp], sig)
	}

	for i, bar := range sortedData {
		currentAtr := 0.0
		if atrValues != nil && i < len(atrValues) && !math.IsNaN(atrValues[i]) {
			currentAtr = atrValues[i]
		} else if i > 0 && atrValues != nil && i < len(atrValues) && math.IsNaN(atrValues[i]) && !math.IsNaN(atrValues[i-1]) {
			currentAtr = atrValues[i-1] // Use previous ATR if current is NaN
		} else {
			currentAtr = bar.Close * 0.02 // Fallback ATR if still NaN or atrValues is nil
		}

		// Update current position
		if currentPosition != nil {
			currentPosition.CurrentPrice = bar.Close
			currentPosition.UnrealizedPL = (bar.Close - currentPosition.EntryPrice) * currentPosition.Quantity
			// Check stop-loss for long positions
			if currentPosition.StopLoss > 0 && bar.Low <= currentPosition.StopLoss {
				s.Log("[Backtest] Stop-Loss triggered for %s at %.2f on %s", currentPosition.StockCode, currentPosition.StopLoss, bar.Timestamp.Format("2006-01-02"))
				exitPrice := currentPosition.StopLoss // Assume execution at stop-loss price
				proceeds := currentPosition.Quantity * exitPrice
				trade := Trade{
					StockCode: currentPosition.StockCode, EntryTime: currentPosition.EntryTime, EntryPrice: currentPosition.EntryPrice,
					ExitTime: bar.Timestamp, ExitPrice: exitPrice, Quantity: currentPosition.Quantity, Direction: "LONG",
					ProfitLoss: proceeds - (currentPosition.Quantity * currentPosition.EntryPrice), HoldingPeriod: bar.Timestamp.Sub(currentPosition.EntryTime), ExitReason: "Stop-Loss",
				}
				if currentPosition.EntryPrice != 0 { // Avoid division by zero
					trade.ReturnRate = (trade.ProfitLoss / (currentPosition.Quantity * currentPosition.EntryPrice)) * 100
				}
				trades = append(trades, trade)
				capital += proceeds
				currentPosition = nil
			}
		}

		// Process signals for this bar, only if no stop-loss was hit for currentPosition on this bar
		if currentPosition == nil { // Check if position was closed by SL
			if currentSignals, ok := signalsByTime[bar.Timestamp]; ok {
				for _, sig := range currentSignals {
					if sig.StockCode != bar.Code {
						continue
					}

					if sig.Type == SignalBuy && currentPosition == nil { // Ensure we are not already in a position
						riskPercent := s.GetFloatParam("riskPercent", 2.0)
						quantity := s.calculateVolatilityAdjustedPositionSize(capital, riskPercent, sig.Price, currentAtr)

						if quantity > 0 {
							cost := quantity * sig.Price
							if capital >= cost {
								capital -= cost
								stopLossPrice := sig.Price - (currentAtr * s.atrStopMultiplier)
								if stopLossPrice >= sig.Price { // Ensure SL is below entry for a long
									stopLossPrice = sig.Price - (sig.Price * 0.01) // Fallback SL if ATR is too small or negative
								}
								currentPosition = &Position{
									StockCode: sig.StockCode, EntryPrice: sig.Price, CurrentPrice: sig.Price, Quantity: quantity, EntryTime: sig.Timestamp, Strategy: s.Name(),
									StopLoss: stopLossPrice,
								}
								s.Log("[Backtest] BUY %s: Qty %.2f @ %.2f on %s, SL %.2f (ATR %.2f)", sig.StockCode, quantity, sig.Price, currentPosition.StopLoss, currentAtr)
							}
						}
					} else if sig.Type == SignalSell && currentPosition != nil && currentPosition.StockCode == sig.StockCode {
						// Sell signal to exit an existing long position
						proceeds := currentPosition.Quantity * sig.Price
						trade := Trade{
							StockCode: currentPosition.StockCode, EntryTime: currentPosition.EntryTime, EntryPrice: currentPosition.EntryPrice,
							ExitTime: sig.Timestamp, ExitPrice: sig.Price, Quantity: currentPosition.Quantity, Direction: "LONG",
							ProfitLoss: proceeds - (currentPosition.Quantity * currentPosition.EntryPrice), HoldingPeriod: sig.Timestamp.Sub(currentPosition.EntryTime), ExitReason: "Signal",
						}
						if currentPosition.EntryPrice != 0 { // Avoid division by zero
							trade.ReturnRate = (trade.ProfitLoss / (currentPosition.Quantity * currentPosition.EntryPrice)) * 100
						}
						trades = append(trades, trade)
						capital += proceeds
						currentPosition = nil
						s.Log("[Backtest] SELL %s: Qty %.2f @ %.2f on %s, P&L: %.2f", sig.StockCode, trade.Quantity, sig.Price, sig.Timestamp.Format("2006-01-02"), trade.ProfitLoss)
					}
				}
			}
		}

		currentEquity := capital
		if currentPosition != nil {
			currentEquity += currentPosition.Quantity * bar.Close // Use current bar's close for equity calculation
		}
		equityCurve = append(equityCurve, currentEquity)
	}

	// Close any open position at the end of the backtest period
	if currentPosition != nil {
		lastBar := sortedData[len(sortedData)-1]
		proceeds := currentPosition.Quantity * lastBar.Close
		trade := Trade{
			StockCode: currentPosition.StockCode, EntryTime: currentPosition.EntryTime, EntryPrice: currentPosition.EntryPrice,
			ExitTime: lastBar.Timestamp, ExitPrice: lastBar.Close, Quantity: currentPosition.Quantity, Direction: "LONG",
			ProfitLoss: proceeds - (currentPosition.Quantity * currentPosition.EntryPrice), HoldingPeriod: lastBar.Timestamp.Sub(currentPosition.EntryTime), ExitReason: "End of backtest",
		}
		if currentPosition.EntryPrice != 0 { // Avoid division by zero
			trade.ReturnRate = (trade.ProfitLoss / (currentPosition.Quantity * currentPosition.EntryPrice)) * 100
		}
		trades = append(trades, trade)
		capital += proceeds
		s.Log("[Backtest] Close EOD %s: Qty %.2f @ %.2f on %s, P&L: %.2f", trade.StockCode, trade.Quantity, trade.ExitPrice, trade.ExitTime.Format("2006-01-02"), trade.ProfitLoss)
	}

	// Calculate performance metrics
	finalCapital := capital
	winCount, lossCount := 0, 0
	for _, tr := range trades {
		if tr.ProfitLoss > 0 {
			winCount++
		} else if tr.ProfitLoss < 0 {
			lossCount++
		}
	}

	returns := s.CalculateReturns(equityCurve)
	sharpe := s.CalculateSharpeRatio(returns, 0.0)
	maxDD := s.CalculateMaxDrawdown(equityCurve)

	vol := 0.0
	if len(returns) > 1 {
		sumReturns, sumSqDiff := 0.0, 0.0
		for _, r := range returns {
			sumReturns += r
		}
		meanReturn := sumReturns / float64(len(returns))
		for _, r := range returns {
			sumSqDiff += math.Pow(r-meanReturn, 2)
		}
		vol = math.Sqrt(sumSqDiff / float64(len(returns)))
	}

	return &BacktestResult{
		StartTime: sortedData[0].Timestamp, EndTime: sortedData[len(sortedData)-1].Timestamp,
		TotalTrades: len(trades), WinningTrades: winCount, LosingTrades: lossCount,
		InitialCapital: initialCapital, FinalCapital: finalCapital, ProfitLoss: finalCapital - initialCapital,
		ReturnRate:  (finalCapital - initialCapital) / initialCapital * 100,
		MaxDrawdown: maxDD, SharpeRatio: sharpe, Volatility: vol,
		Trades: trades, StrategyName: s.Name(), Parameters: s.params,
	}, nil
}

// OptimizeParameters placeholder for momentum strategy parameter optimization.
func (s *MomentumStrategy) OptimizeParameters(ctx context.Context, stockData []models.Stock, initialCapital float64) (StrategyParams, *BacktestResult, error) {
	s.Log("Optimization for MomentumStrategy: Iterating through lookback periods and ROC thresholds.")

	bestReturnRate := -math.MaxFloat64
	var bestParams StrategyParams
	var bestResult *BacktestResult

	// Define ranges for parameters to optimize
	lookbackPeriods := []int{10, 14, 20, 25}               // Example range for lookback
	rocBuyThresholds := []float64{1.0, 1.5, 2.0, 2.5, 3.0} // Example ROC buy thresholds
	// rocSellThresholds could also be optimized, e.g., symmetric or asymmetric to buy thresholds

	for _, lbp := range lookbackPeriods {
		for _, rocBuy := range rocBuyThresholds {
			rocSell := -rocBuy // Keep sell threshold symmetric for this example

			currentParams := s.DefaultParams() // Start with defaults
			currentParams["lookbackPeriod"] = lbp
			currentParams["rocThresholdBuy"] = rocBuy
			currentParams["rocThresholdSell"] = rocSell
			// Potentially iterate over smoothingPeriod, macdConfirm, ATR params as well for more comprehensive optimization

			tempStrategy := NewMomentumStrategy()
			if err := tempStrategy.Initialize(currentParams); err != nil {
				s.Log("Skipping invalid param set in optimization: %v", err)
				continue
			}

			result, err := tempStrategy.Backtest(ctx, stockData, initialCapital)
			if err != nil {
				s.Log("Backtest failed for params %+v: %v", currentParams, err)
				continue
			}

			if result.ReturnRate > bestReturnRate {
				bestReturnRate = result.ReturnRate
				bestParams = currentParams
				bestResult = result
				s.Log("New best params found in optimization: %+v, Return: %.2f%%", bestParams, bestReturnRate)
			}
		}
	}

	if bestParams == nil {
		return s.DefaultParams(), nil, fmt.Errorf("optimization failed to find any valid parameters or profitable strategy")
	}

	s.Log("Optimization complete. Best params: %+v, Best Return: %.2f%%", bestParams, bestReturnRate)
	return bestParams, bestResult, nil
}

// combineMomentumAndTrend is a conceptual helper for combining signals.
// This specific implementation for GenerateSignals uses direct checks rather than this helper.
func (s *MomentumStrategy) combineMomentumAndTrend(rocVal, macdVal, macdSignalVal float64, isBuySignal bool) bool {
	if isBuySignal {
		// For a buy: ROC must be bullish, and MACD must confirm bullish trend
		return rocVal > s.rocThresholdBuy && macdVal > macdSignalVal
	} else {
		// For a sell: ROC must be bearish, and MACD must confirm bearish trend
		return rocVal < s.rocThresholdSell && macdVal < macdSignalVal
	}
}
