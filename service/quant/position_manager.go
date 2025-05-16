package quant

import (
	"context"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
)

// PositionManagerConfig holds configuration for the PositionManager.
// These would typically be loaded from a configuration file or environment variables.
type PositionManagerConfig struct {
	DefaultSizingMethod    string  // "fixed_trade_value", "percentage_capital", "volatility_adjusted"
	FixedTradeValue        float64 // Value of each trade for "fixed_trade_value" sizing
	PercentageCapitalRisk  float64 // e.g., 1.0 for 1% of capital to risk per trade
	ATRMultiplierForStop   float64 // For volatility-adjusted sizing and ATR-based stops (e.g., 2.0 for 2xATR)
	DefaultStopLossType    string  // "none", "percentage", "atr"
	StopLossPercentage     float64 // e.g., 2.0 for 2% below/above entry
	DefaultTakeProfitType  string  // "none", "percentage", "atr_multiple"
	TakeProfitPercentage   float64 // e.g., 4.0 for 4% above/below entry
	TakeProfitATRMultiple  float64 // e.g., 3.0 for 3xATR from entry
	EnableTrailingStop     bool
	TrailingStopType       string  // "percentage", "atr"
	TrailingStopPercentage float64 // e.g., 1.0 for 1% trail from peak
	TrailingStopATRDelta   float64 // e.g., 1.5 for 1.5xATR trailing distance
	MaxPortfolioRisk       float64 // Maximum percentage of portfolio to risk across all open positions (e.g., 10.0 for 10%)
	CommissionPerTrade     float64 // Fixed commission amount per trade
	CommissionRate         float64 // Commission as a percentage of trade value (e.g., 0.001 for 0.1%)
}

// InMemoryPositionManager implements the PositionManager interface for handling trading positions.
// It also includes extended portfolio management functionalities.
type InMemoryPositionManager struct {
	mu              sync.RWMutex
	config          PositionManagerConfig
	positions       map[string]Position // Key: StockCode
	cash            float64
	initialCapital  float64
	completedTrades []Trade
	nextOrderID     int64
	logger          *log.Logger
}

// NewInMemoryPositionManager creates a new instance of InMemoryPositionManager.
func NewInMemoryPositionManager(initialCapital float64, config PositionManagerConfig, logger *log.Logger) *InMemoryPositionManager {
	if logger == nil {
		logger = log.Default()
	}
	return &InMemoryPositionManager{
		config:          config,
		positions:       make(map[string]Position),
		cash:            initialCapital,
		initialCapital:  initialCapital,
		completedTrades: make([]Trade, 0),
		nextOrderID:     1,
		logger:          logger,
	}
}

// GetPosition retrieves the current position for a given stock code.
func (pm *InMemoryPositionManager) GetPosition(ctx context.Context, stockCode string) (Position, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	pos, exists := pm.positions[stockCode]
	return pos, exists
}

// UpdatePosition updates or creates a position based on a filled trade (fill record).
// The 'tradeFilled' here represents a single leg of a trade (a buy or a sell fill).
func (pm *InMemoryPositionManager) UpdatePosition(ctx context.Context, tradeFilled Trade) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	stockCode := tradeFilled.StockCode
	fillPrice := tradeFilled.EntryPrice    // For BUY, this is entry. For SELL, this is effectively exit of existing.
	fillQuantity := tradeFilled.Quantity
	fillTime := tradeFilled.EntryTime // Assuming EntryTime on the Trade struct is the fill time

	// Adjust fillPrice for SELL direction if it was placed in EntryPrice
	if tradeFilled.Direction == string(SignalSell) && tradeFilled.ExitPrice > 0 {
		fillPrice = tradeFilled.ExitPrice
	}

	commission := pm.config.CommissionPerTrade + (fillPrice * fillQuantity * pm.config.CommissionRate)

	currentPos, exists := pm.positions[stockCode]

	if tradeFilled.Direction == string(SignalBuy) {
		cost := fillPrice*fillQuantity + commission
		if pm.cash < cost {
			return fmt.Errorf("insufficient cash (%.2f) to cover cost (%.2f) for BUY %s", pm.cash, cost, stockCode)
		}
		pm.cash -= cost

		if exists { // Averaging into an existing position
			newTotalQuantity := currentPos.Quantity + fillQuantity
			newAvgEntryPrice := ((currentPos.EntryPrice * currentPos.Quantity) + (fillPrice * fillQuantity)) / newTotalQuantity
			currentPos.EntryPrice = newAvgEntryPrice
			currentPos.Quantity = newTotalQuantity
			currentPos.CurrentPrice = fillPrice // Update current price to the latest fill
			currentPos.EntryTime = fillTime    // Could average time, or use latest entry time
			pm.logger.Printf("Averaged into position %s: New Qty %.2f, AvgPrice %.2f", stockCode, newTotalQuantity, newAvgEntryPrice)
		} else { // New position
			currentPos = Position{
				StockCode:    stockCode,
				EntryPrice:   fillPrice,
				CurrentPrice: fillPrice,
				Quantity:     fillQuantity,
				EntryTime:    fillTime,
				Strategy:     tradeFilled.Strategy, // Assuming strategy is on the fill record
			}
			pm.logger.Printf("Opened new position %s: Qty %.2f, Price %.2f", stockCode, fillQuantity, fillPrice)
		}
		// Set initial stop-loss and take-profit based on config
		currentPos.StopLoss, currentPos.TakeProfit = pm.calculateInitialRiskLevels(currentPos, fillPrice, 0) // ATR is 0 for initial calculation without history here
		pm.positions[stockCode] = currentPos

	} else if tradeFilled.Direction == string(SignalSell) {
		if !exists || currentPos.Quantity < fillQuantity {
			return fmt.Errorf("cannot sell %s: no position or insufficient quantity (have %.2f, need %.2f)", stockCode, currentPos.Quantity, fillQuantity)
		}

		proceeds := fillPrice*fillQuantity - commission
		pm.cash += proceeds

		realizedPL := (fillPrice - currentPos.EntryPrice) * fillQuantity // For long positions
		// For short positions (not fully implemented here): (currentPos.EntryPrice - fillPrice) * fillQuantity

		completedTrade := Trade{
			StockCode:     stockCode,
			EntryTime:     currentPos.EntryTime,
			EntryPrice:    currentPos.EntryPrice,
			ExitTime:      fillTime,
			ExitPrice:     fillPrice,
			Quantity:      fillQuantity,
			Direction:     currentPos.Strategy, // This should be "LONG" or "SHORT" from position
			ProfitLoss:    realizedPL - commission, // Net P&L after commission
			HoldingPeriod: fillTime.Sub(currentPos.EntryTime),
			ExitReason:    "Signal", // Or from tradeFilled.Notes
			Strategy:      currentPos.Strategy,
		}
		if currentPos.EntryPrice != 0 {
			completedTrade.ReturnRate = (completedTrade.ProfitLoss / (currentPos.EntryPrice * fillQuantity)) * 100
		}

		pm.recordCompletedTrade(completedTrade)

		currentPos.Quantity -= fillQuantity
		pm.logger.Printf("Sold %s: Qty %.2f, Price %.2f. Realized P&L: %.2f", stockCode, fillQuantity, fillPrice, completedTrade.ProfitLoss)

		if currentPos.Quantity == 0 {
			delete(pm.positions, stockCode)
			pm.logger.Printf("Closed position %s", stockCode)
		} else {
			pm.positions[stockCode] = currentPos // Update with reduced quantity
		}
	} else {
		return fmt.Errorf("unknown trade direction: %s", tradeFilled.Direction)
	}

	return nil
}

// GetAllPositions retrieves all current open positions.
func (pm *InMemoryPositionManager) GetAllPositions(ctx context.Context) ([]Position, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	positions := make([]Position, 0, len(pm.positions))
	for _, pos := range pm.positions {
		positions = append(positions, pos)
	}
	return positions, nil
}

// GetCashBalance retrieves the current available cash balance.
func (pm *InMemoryPositionManager) GetCashBalance(ctx context.Context) (float64, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.cash, nil
}

// UpdateCashBalance updates the cash balance.
func (pm *InMemoryPositionManager) UpdateCashBalance(ctx context.Context, amountChange float64) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.cash += amountChange
	pm.logger.Printf("Cash balance updated by %.2f. New balance: %.2f", amountChange, pm.cash)
	return nil
}

// RecordTrade records a completed trade for performance tracking.
// This method is intended for externally completed trades or for reconciling.
// P&L calculation happens within UpdatePosition when a position is closed.
func (pm *InMemoryPositionManager) RecordTrade(ctx context.Context, trade Trade) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.recordCompletedTrade(trade) // Uses internal helper
	return nil
}

func (pm *InMemoryPositionManager) recordCompletedTrade(trade Trade) {
	pm.completedTrades = append(pm.completedTrades, trade)
	// Realized P&L is already accounted for in UpdatePosition if this trade came from closing a position managed by it.
	// If this is for an externally managed trade, then P&L should be added to a portfolio P&L counter here.
	pm.logger.Printf("Recorded completed trade for %s: P&L %.2f", trade.StockCode, trade.ProfitLoss)
}

// GetTotalPortfolioValue calculates the current total value of the portfolio (cash + value of open positions).
func (pm *InMemoryPositionManager) GetTotalPortfolioValue(ctx context.Context, marketPrices map[string]float64) (float64, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	totalValue := pm.cash
	for stockCode, pos := range pm.positions {
		currentPrice, ok := marketPrices[stockCode]
		if !ok {
			currentPrice = pos.CurrentPrice // Use last known price if market price not provided
		}
		totalValue += pos.Quantity * currentPrice
	}
	return totalValue, nil
}

// UpdateMarketPrice updates the current market price for a stock, recalculates Unrealized P&L and checks risk limits.
func (pm *InMemoryPositionManager) UpdateMarketPrice(ctx context.Context, stockCode string, currentPrice float64, currentATR float64) ([]Order, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	generatedOrders := []Order{}

	pos, exists := pm.positions[stockCode]
	if !exists {
		return generatedOrders, nil // No position to update
	}

	pos.CurrentPrice = currentPrice
	pos.UnrealizedPL = (currentPrice - pos.EntryPrice) * pos.Quantity // Assuming long position

	// Check Stop Loss
	if pos.StopLoss > 0 && currentPrice <= pos.StopLoss {
		pm.logger.Printf("Stop-loss triggered for %s at %.2f (SL: %.2f)", stockCode, currentPrice, pos.StopLoss)
		order, err := pm.generateExitOrderInternal(pos, pos.StopLoss, "Stop-loss triggered")
		if err == nil {
			generatedOrders = append(generatedOrders, order)
		}
		// The actual closing of position happens when this order is confirmed filled via UpdatePosition
		// For immediate simulation or backtesting, one might close it here.
	}

	// Check Take Profit
	if pos.TakeProfit > 0 && currentPrice >= pos.TakeProfit {
		pm.logger.Printf("Take-profit triggered for %s at %.2f (TP: %.2f)", stockCode, currentPrice, pos.TakeProfit)
		order, err := pm.generateExitOrderInternal(pos, pos.TakeProfit, "Take-profit triggered")
		if err == nil {
			generatedOrders = append(generatedOrders, order)
		}
	}

	// Update Trailing Stop if enabled
	if pm.config.EnableTrailingStop {
		newTrailingStop := pos.StopLoss // Start with current stop loss
		trailAmount := 0.0

		if pm.config.TrailingStopType == "percentage" {
			trailAmount = currentPrice * (pm.config.TrailingStopPercentage / 100.0)
		} else if pm.config.TrailingStopType == "atr" && currentATR > 0 {
			trailAmount = currentATR * pm.config.TrailingStopATRDelta
		}

		if trailAmount > 0 {
			// Assuming long position for trailing stop logic
			potentialNewStop := currentPrice - trailAmount
			if potentialNewStop > pos.StopLoss { // Only trail upwards for long positions
				newTrailingStop = potentialNewStop
				pm.logger.Printf("Trailing stop for %s updated to %.2f (Price: %.2f, TrailAmount: %.2f)", stockCode, newTrailingStop, currentPrice, trailAmount)
			}
		}
		pos.StopLoss = newTrailingStop // Update the position's stop loss to the new trailing stop
	}

	pm.positions[stockCode] = pos
	return generatedOrders, nil
}

// calculateInitialRiskLevels sets stop-loss and take-profit based on configuration when a position is opened/updated.
func (pm *InMemoryPositionManager) calculateInitialRiskLevels(pos Position, entryPrice float64, currentATR float64) (stopLoss float64, takeProfit float64) {
	// Stop Loss
	switch pm.config.DefaultStopLossType {
	case "percentage":
		// Assuming long position, stop loss is below entry price
		stopLoss = entryPrice * (1 - pm.config.StopLossPercentage/100.0)
	case "atr":
		if currentATR > 0 {
			stopLoss = entryPrice - (currentATR * pm.config.ATRMultiplierForStop)
		} else {
			stopLoss = 0 // Cannot set ATR stop without ATR value
		}
	default: // "none" or unrecognized
		stopLoss = 0
	}

	// Take Profit
	switch pm.config.DefaultTakeProfitType {
	case "percentage":
		// Assuming long position, take profit is above entry price
		takeProfit = entryPrice * (1 + pm.config.TakeProfitPercentage/100.0)
	case "atr_multiple":
		if currentATR > 0 {
			takeProfit = entryPrice + (currentATR * pm.config.TakeProfitATRMultiple)
		} else {
			takeProfit = 0 // Cannot set ATR take profit without ATR value
		}
	default: // "none" or unrecognized
		takeProfit = 0
	}

	return stopLoss, takeProfit
}

// --- Position Sizing Algorithms ---

// CalculatePositionQuantity calculates the number of shares to trade based on the configured method.
// ATR value is optional and only used for volatility-adjusted sizing or ATR-based stops if configured.
func (pm *InMemoryPositionManager) CalculatePositionQuantity(ctx context.Context, stockCode string, price float64, atrValue float64, signalType SignalType) (float64, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	cashBalance, _ := pm.GetCashBalance(ctx) // Error ignored for simplicity, consider handling
	portfolioValue, _ := pm.GetTotalPortfolioValue(ctx, map[string]float64{stockCode: price}) // Approx portfolio value

	var quantity float64
	var err error

	// For SELL signals, if it's to close an existing position, quantity is determined by the position size.
	if signalType == SignalSell {
		if pos, exists := pm.positions[stockCode]; exists {
			return pos.Quantity, nil // Sell the entire existing position
		}
		// If trying to short sell and no existing position, sizing logic would apply.
		// This example primarily focuses on sizing for new long positions.
	}

	switch pm.config.DefaultSizingMethod {
	case "fixed_trade_value":
		if price <= 0 {
			return 0, fmt.Errorf("price must be positive for fixed_trade_value sizing")
		}
		quantity = math.Floor(pm.config.FixedTradeValue / price)
	case "percentage_capital":
		if price <= 0 {
			return 0, fmt.Errorf("price must be positive for percentage_capital sizing")
		}
		allowedRiskCapital := portfolioValue * (pm.config.PercentageCapitalRisk / 100.0)
		// Determine stop-loss distance to calculate quantity based on risk
		stopLossDistance := price * (pm.config.StopLossPercentage / 100.0) // Default to percentage stop
		if pm.config.DefaultStopLossType == "atr" && atrValue > 0 {
			stopLossDistance = atrValue * pm.config.ATRMultiplierForStop
		}
		if stopLossDistance <= 0 {
			return 0, fmt.Errorf("stop loss distance must be positive for risk-based sizing")
		}
		quantity = math.Floor(allowedRiskCapital / stopLossDistance)

	case "volatility_adjusted": // ATR-based sizing
		if price <= 0 || atrValue <= 0 {
			return 0, fmt.Errorf("price and ATR must be positive for volatility_adjusted sizing")
		}
		dollarRiskPerShare := atrValue * pm.config.ATRMultiplierForStop
		if dollarRiskPerShare <= 0 {
			return 0, fmt.Errorf("dollar risk per share must be positive")
		}
		portfolioRiskAmount := portfolioValue * (pm.config.PercentageCapitalRisk / 100.0)
		quantity = math.Floor(portfolioRiskAmount / dollarRiskPerShare)
	default:
		return 0, fmt.Errorf("unknown position sizing method: %s", pm.config.DefaultSizingMethod)
	}

	// Ensure total cost does not exceed available cash for BUY orders.
	if signalType == SignalBuy {
		maxAffordableQuantity := math.Floor(cashBalance / price)
		if quantity > maxAffordableQuantity {
			quantity = maxAffordableQuantity
			pm.logger.Printf("Sizing for %s (BUY) reduced to %.2f due to cash limits.", stockCode, quantity)
		}
	}

	if quantity < 0 {
		quantity = 0
	}
	return quantity, err
}

// --- Order Generation (Conceptual - actual placement is by OrderExecutor) ---

func (pm *InMemoryPositionManager) generateNextOrderID() string {
	// Simple counter for this example; in production, use UUIDs or broker-returned IDs.
	// For internal generation before sending to an executor that assigns its own ID.
	orderID := fmt.Sprintf("PM-%s", uuid.New().String())
	return orderID
}

// GenerateEntryOrder suggests an entry order based on signal and sizing.
func (pm *InMemoryPositionManager) GenerateEntryOrder(ctx context.Context, signal Signal, atrValue float64) (Order, error) {
	quantity, err := pm.CalculatePositionQuantity(ctx, signal.StockCode, signal.Price, atrValue, signal.Type)
	if err != nil {
		return Order{}, fmt.Errorf("failed to calculate quantity for entry order %s: %w", signal.StockCode, err)
	}

	if quantity == 0 {
		return Order{}, fmt.Errorf("calculated quantity is 0 for entry order %s, no order generated", signal.StockCode)
	}

	orderType := OrderTypeBuy
	if signal.Type == SignalSell { // This would be for initiating a short sell
		orderType = OrderTypeSell
	}

	order := Order{
		ID:        pm.generateNextOrderID(), // Placeholder ID
		StockCode: signal.StockCode,
		Type:      orderType,
		Quantity:  quantity,
		Price:     signal.Price, // Assuming limit order at signal price
		Timestamp: time.Now(),
		Status:    OrderStatusNew,
		Strategy:  signal.Strategy,
		Notes:     fmt.Sprintf("Entry based on signal: %s", signal.Notes),
	}
	return order, nil
}

// generateExitOrderInternal creates an exit order for an existing position.
// This is called internally when a risk limit is hit.
func (pm *InMemoryPositionManager) generateExitOrderInternal(pos Position, exitPrice float64, reason string) (Order, error) {
	if pos.Quantity <= 0 {
		return Order{}, fmt.Errorf("cannot generate exit order for position with zero quantity: %s", pos.StockCode)
	}

	// Assuming exiting a long position
	orderType := OrderTypeSell

	order := Order{
		ID:        pm.generateNextOrderID(),
		StockCode: pos.StockCode,
		Type:      orderType,
		Quantity:  pos.Quantity, // Exit entire position
		Price:     exitPrice,    // Exit at the specified price (e.g., stop-loss price)
		Timestamp: time.Now(),
		Status:    OrderStatusNew,
		Strategy:  pos.Strategy,
		Notes:     reason,
	}
	return order, nil
}

// --- Portfolio Rebalancing & Optimization (Placeholders) ---

// RebalancePortfolio provides a placeholder for rebalancing logic.
// True rebalancing involves adjusting asset allocations to target weights.
func (pm *InMemoryPositionManager) RebalancePortfolio(ctx context.Context, targetAllocations map[string]float64, marketPrices map[string]float64) ([]Order, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.logger.Println("Portfolio rebalancing requested. (Placeholder - Not Implemented)")
	// 1. Calculate current portfolio value.
	// 2. For each asset, calculate current weight vs. target weight.
	// 3. Generate buy/sell orders to move towards target allocations.
	// This is complex and depends on many factors (transaction costs, min trade sizes, etc.)
	return []Order{}, nil
}

// OptimizePortfolio provides a placeholder for portfolio optimization logic.
// True optimization might involve Markowitz model, risk parity, etc.
func (pm *InMemoryPositionManager) OptimizePortfolio(ctx context.Context) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.logger.Println("Portfolio optimization requested. (Placeholder - Not Implemented)")
	return nil
}

// --- P&L Tracking Helpers ---

// GetRealizedPL calculates total realized P&L from completed trades.
func (pm *InMemoryPositionManager) GetRealizedPL(ctx context.Context) (float64, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	totalRealizedPL := 0.0
	for _, trade := range pm.completedTrades {
		totalRealizedPL += trade.ProfitLoss
	}
	return totalRealizedPL, nil
}

// GetUnrealizedPL calculates total unrealized P&L from open positions, given current market prices.
func (pm *InMemoryPositionManager) GetUnrealizedPL(ctx context.Context, marketPrices map[string]float64) (float64, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	totalUnrealizedPL := 0.0
	for stockCode, pos := range pm.positions {
		currentPrice, ok := marketPrices[stockCode]
		if !ok {
			// If market price for a position is not provided, we can't calculate its unrealized P&L accurately.
			// Option 1: Skip this position. Option 2: Use pos.CurrentPrice (last known).
			// For now, let's assume if not in marketPrices, it means we don't have an update, so use pos.CurrentPrice.
			currentPrice = pos.CurrentPrice
		}
		totalUnrealizedPL += (currentPrice - pos.EntryPrice) * pos.Quantity // Assuming long
	}
	return totalUnrealizedPL, nil
}

// GetCompletedTrades returns a list of all recorded completed trades.
func (pm *InMemoryPositionManager) GetCompletedTrades(ctx context.Context) ([]Trade, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	// Return a copy to prevent external modification
	tradesCopy := make([]Trade, len(pm.completedTrades))
	copy(tradesCopy, pm.completedTrades)
	return tradesCopy, nil
}

// SetLogger allows setting a custom logger.
func (pm *InMemoryPositionManager) SetLogger(logger *log.Logger) {
    pm.mu.Lock()
    defer pm.mu.Unlock()
    if logger != nil {
        pm.logger = logger
    }
}

// ResetState clears all positions, trades, and resets cash to initial capital.
// Useful for backtesting multiple runs or scenarios.
func (pm *InMemoryPositionManager) ResetState(ctx context.Context, newInitialCapital float64) {
    pm.mu.Lock()
    defer pm.mu.Unlock()

    pm.positions = make(map[string]Position)
    pm.completedTrades = make([]Trade, 0)
    pm.initialCapital = newInitialCapital
    pm.cash = newInitialCapital
    pm.nextOrderID = 1 // Reset order ID counter if it's used internally
    pm.logger.Printf("PositionManager state reset. Initial capital: %.2f", newInitialCapital)
}

// CheckPortfolioRisk evaluates if current open positions exceed max portfolio risk.
// This is a simplified check; real risk management is more nuanced.
func (pm *InMemoryPositionManager) CheckPortfolioRisk(ctx context.Context, marketPrices map[string]float64) (bool, float64, error) {
    pm.mu.RLock()
    defer pm.mu.RUnlock()

    if pm.config.MaxPortfolioRisk <= 0 { // Risk check disabled
        return false, 0, nil
    }

    totalPortfolioValue, err := pm.GetTotalPortfolioValue(ctx, marketPrices)
    if err != nil {
        return false, 0, fmt.Errorf("could not get total portfolio value for risk check: %w", err)
    }
    if totalPortfolioValue == 0 { // Avoid division by zero if portfolio value is zero (e.g. only cash, no positions yet)
        return false, 0, nil
    }

    totalRiskedAmount := 0.0
    for stockCode, pos := range pm.positions {
        if pos.StopLoss > 0 { // Only consider positions with a defined stop-loss for this calculation
            currentPrice, ok := marketPrices[stockCode]
            if !ok {
                currentPrice = pos.CurrentPrice
            }
            potentialLossPerShare := currentPrice - pos.StopLoss // For long positions
            if potentialLossPerShare < 0 { potentialLossPerShare = 0 } // Cannot gain from stop loss
            totalRiskedAmount += potentialLossPerShare * pos.Quantity
        }
    }

    currentPortfolioRiskPercent := (totalRiskedAmount / totalPortfolioValue) * 100
    isOverRiskLimit := currentPortfolioRiskPercent > pm.config.MaxPortfolioRisk

    if isOverRiskLimit {
        pm.logger.Printf("Portfolio risk limit check: Current risk %.2f%% exceeds max %.2f%%", 
            currentPortfolioRiskPercent, pm.config.MaxPortfolioRisk)
    }

    return isOverRiskLimit, currentPortfolioRiskPercent, nil
}


// ClosePositionGenerates an order to close the entire specified position at the given price.
// This is a utility that might be called by strategies or the executor based on certain conditions.
func (pm *InMemoryPositionManager) ClosePosition(ctx context.Context, stockCode string, exitPrice float64, reason string) (Order, error) {
    pm.mu.RLock()
    pos, exists := pm.positions[stockCode]
    pm.mu.RUnlock()

    if !exists {
        return Order{}, fmt.Errorf("no open position found for stock code %s to close", stockCode)
    }

    return pm.generateExitOrderInternal(pos, exitPrice, reason)
}

</code>