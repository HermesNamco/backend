package quant

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"sync"
	"time"

	"backend/models"
	// Assuming stockdataProvider is from the higher level stock package, 
	// but for strategy execution, we might need a quant-specific data provider
	// that aligns with quant.DataProvider interface.
	stockApi "backend/service/stock/api"
)

// ExecutionMode defines the operational mode of the StrategyExecutor.
//go:generate stringer -type=ExecutionMode
type ExecutionMode int

const (
	// PaperTrading simulates trades without executing them on a live exchange.
	PaperTrading ExecutionMode = iota
	// LiveTrading executes trades on a live exchange.
	LiveTrading
	// Backtesting runs strategies against historical data to evaluate performance.
	Backtesting
)

// OrderType defines the type of order.
//go:generate stringer -type=OrderType
type OrderType string

const (
	OrderTypeBuy  OrderType = "BUY"
	OrderTypeSell OrderType = "SELL"
)

// OrderStatus defines the status of an order.
//go:generate stringer -type=OrderStatus
type OrderStatus string

const (
	OrderStatusNew       OrderStatus = "NEW"
	OrderStatusFilled    OrderStatus = "FILLED"
	OrderStatusPartial   OrderStatus = "PARTIAL_FILL"
	OrderStatusCancelled OrderStatus = "CANCELLED"
	OrderStatusRejected  OrderStatus = "REJECTED"
	OrderStatusError     OrderStatus = "ERROR"
)

// Order represents a trading order to be placed with a broker or simulated.
type Order struct {
	ID        string      `json:"id"`         // Unique order identifier, can be assigned by executor or broker
	StockCode string      `json:"stockCode"` // Stock symbol
	Type      OrderType   `json:"type"`      // BUY or SELL
	Quantity  float64     `json:"quantity"`   // Number of shares/contracts
	Price     float64     `json:"price"`      // Limit price (0 for market order)
	Timestamp time.Time   `json:"timestamp"`  // Time the order was created/placed
	Status    OrderStatus `json:"status"`    // Current status of the order
	Strategy  string      `json:"strategy"`   // Strategy that generated this order
	Notes     string      `json:"notes,omitempty"`
}

// OrderExecutor defines an interface for placing, cancelling, and checking orders.
// This can be implemented for live brokers or paper trading simulators.	ype OrderExecutor interface {
	// PlaceOrder submits an order and returns an order ID or an error.
	PlaceOrder(ctx context.Context, order Order) (string, error)
	// CancelOrder attempts to cancel an existing order.
	CancelOrder(ctx context.Context, orderID string, stockCode string) error
	// GetOrderStatus retrieves the current status of an order.
	GetOrderStatus(ctx context.Context, orderID string, stockCode string) (OrderStatus, error)
	// GetBrokerName returns the name of the broker/exchange this executor interacts with.
	GetBrokerName() string
}

// PositionManager defines an interface for managing trading positions and portfolio state.	ype PositionManager interface {
	// GetPosition retrieves the current position for a given stock code.
	GetPosition(ctx context.Context, stockCode string) (Position, bool)
	// UpdatePosition updates or creates a position for a stock code.
	// This would be called after an order is filled.
	UpdatePosition(ctx context.Context, tradeFilled Trade) error
	// GetAllPositions retrieves all current open positions.
	GetAllPositions(ctx context.Context) ([]Position, error)
	// GetCashBalance retrieves the current available cash balance.
	GetCashBalance(ctx context.Context) (float64, error)
	// UpdateCashBalance updates the cash balance (e.g., after a trade or deposit/withdrawal).
	UpdateCashBalance(ctx context.Context, amountChange float64) error
	// RecordTrade records a completed trade for performance tracking.
	RecordTrade(ctx context.Context, trade Trade) error
}

// Notifier defines an interface for sending notifications about trading events.	ype Notifier interface {
	NotifySignal(ctx context.Context, signal Signal) error
	NotifyOrder(ctx context.Context, order Order) error
	NotifyTrade(ctx context.Context, trade Trade) error
	NotifyError(ctx context.Context, err error, strategyID string, message string) error
	NotifyMessage(ctx context.Context, level string, message string) error // level: INFO, WARN, ERROR
}

// ExecutorConfig holds configuration for the StrategyExecutor.	ype ExecutorConfig struct {
	ExecutionMode         ExecutionMode     `json:"executionMode"`
	SubscribedStockCodes  []string          `json:"subscribedStockCodes"`
	DataTimeframe         DataTimeframe     `json:"dataTimeframe"`
	SignalStrengthFilter  float64           `json:"signalStrengthFilter"` // Minimum signal strength to act upon (0.0-1.0)
	DefaultRiskPerTrade   float64           `json:"defaultRiskPerTrade"`  // Default percentage of capital to risk per trade (e.g., 1.0 for 1%)
	MaxConcurrentStrategies int             `json:"maxConcurrentStrategies"`
	LogFile               string            `json:"logFile,omitempty"`
	PaperTradingConfig    PaperTradingConfig `json:"paperTradingConfig,omitempty"`
	LiveTradingConfig     LiveTradingConfig  `json:"liveTradingConfig,omitempty"`
}

// PaperTradingConfig specific to paper trading mode
type PaperTradingConfig struct {
	InitialCapital float64 `json:"initialCapital"`
	CommissionRate float64 `json:"commissionRate"` // e.g., 0.001 for 0.1%
	Slippage       float64 `json:"slippage"`       // e.g., 0.0005 for 0.05% per side
}

// LiveTradingConfig specific to live trading mode (broker specific details)
type LiveTradingConfig struct {
	BrokerAPIKey    string  `json:"brokerApiKey"`
	BrokerAPISecret string  `json:"brokerApiSecret"`
	BrokerEndpoint  string  `json:"brokerEndpoint"`
	CommissionRate  float64 `json:"commissionRate"`
}

// StrategyExecutor runs trading strategies, processes signals, and manages orders.	ype StrategyExecutor struct {
	config          ExecutorConfig
	dataProvider    DataProvider      // For market data subscription
	stockProvider   stockApi.StockDataProvider // For fetching historical data for ATR etc.
	orderExecutor   OrderExecutor
	positionManager PositionManager
	notifier        Notifier
	strategies      map[string]Strategy // Map of strategy name to strategy instance

	// Real-time processing state
	activeSubscriptions map[string]Subscription // map[stockCode]Subscription
	dataChannels        map[string]chan models.Stock // map[stockCode]chan models.Stock
	lastProcessedSignal map[string]time.Time      // map[strategyName_stockCode]time.Time for debouncing
	mu                  sync.RWMutex
	logger              *log.Logger
	ctx                 context.Context
	cancelFunc          context.CancelFunc
	wg                  sync.WaitGroup
}

// NewStrategyExecutor creates a new StrategyExecutor.
func NewStrategyExecutor(
	config ExecutorConfig,
	dataProvider DataProvider,
	stockProvider stockApi.StockDataProvider, // Added for historical data needs
	orderExecutor OrderExecutor,
	positionManager PositionManager,
	nilNotifier Notifier,
) (*StrategyExecutor, error) {

	logger := log.New(os.Stdout, "[StrategyExecutor] ", log.LstdFlags|log.Lshortfile)
	if config.LogFile != "" {
		file, err := os.OpenFile(config.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			logger.Printf("Failed to open log file %s: %v. Defaulting to stdout.", config.LogFile, err)
		} else {
			logger.SetOutput(file)
		}
	}

	if dataProvider == nil {
		return nil, fmt.Errorf("dataProvider cannot be nil")
	}
	if orderExecutor == nil {
		return nil, fmt.Errorf("orderExecutor cannot be nil")
	}
	if positionManager == nil {
		return nil, fmt.Errorf("positionManager cannot be nil")
	}
	if config.MaxConcurrentStrategies <= 0 {
		config.MaxConcurrentStrategies = 10 // Default value
	}

	ctx, cancelFunc := context.WithCancel(context.Background())

	executor := &StrategyExecutor{
		config:              config,
		dataProvider:        dataProvider,
		stockProvider:       stockProvider,
		orderExecutor:       orderExecutor,
		positionManager:     positionManager,
		notifier:            nilNotifier,
		strategies:          make(map[string]Strategy),
		activeSubscriptions: make(map[string]Subscription),
		dataChannels:        make(map[string]chan models.Stock),
		lastProcessedSignal: make(map[string]time.Time),
		logger:              logger,
		ctx:                 ctx,
		cancelFunc:          cancelFunc,
	}

	// Initialize position manager with initial capital if paper trading
	if config.ExecutionMode == PaperTrading {
		err := positionManager.UpdateCashBalance(ctx, config.PaperTradingConfig.InitialCapital)
		if err != nil {
			return nil, fmt.Errorf("failed to set initial capital for paper trading: %w", err)
		}
	}

	return executor, nil
}

// AddStrategy registers a strategy instance to be run by the executor.
func (e *StrategyExecutor) AddStrategy(strategy Strategy) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.strategies[strategy.Name()]; exists {
		return fmt.Errorf("strategy '%s' already added", strategy.Name())
	}
	e.strategies[strategy.Name()] = strategy
	e.logger.Printf("Strategy '%s' added.", strategy.Name())
	return nil
}

// Start initiates the strategy execution process.
func (e *StrategyExecutor) Start() error {
	e.logger.Printf("Starting StrategyExecutor in %s mode for stocks: %v", e.config.ExecutionMode, e.config.SubscribedStockCodes)

	if len(e.strategies) == 0 {
		return fmt.Errorf("no strategies added to the executor")
	}

	if e.config.ExecutionMode == Backtesting {
		return e.runBacktestMode()
	}

	// For PaperTrading or LiveTrading modes
	for _, stockCode := range e.config.SubscribedStockCodes {
		e.dataChannels[stockCode] = make(chan models.Stock, 100) // Buffered channel
		go e.listenToMarketData(stockCode)
		sub, err := e.dataProvider.SubscribeToUpdates(e.ctx, stockCode, e.config.DataTimeframe, func(data models.Stock) error {
			// Non-blocking send to the dedicated channel for this stock
			select {
			case e.dataChannels[stockCode] <- data:
			default:
				e.logger.Printf("Warning: Data channel for %s is full. Dropping market data: %+v", stockCode, data)
				if e.notifier != nil {
					e.notifier.NotifyError(e.ctx, fmt.Errorf("data channel full for %s", stockCode), "StrategyExecutor", "Market data dropped")
				}
			}
			return nil
		})
		if err != nil {
			e.logger.Printf("Failed to subscribe to market data for %s: %v", stockCode, err)
			if e.notifier != nil {
				e.notifier.NotifyError(e.ctx, err, "StrategyExecutor", fmt.Sprintf("Subscription failed for %s", stockCode))
			}
			// Potentially stop or handle partial subscription failure
			return fmt.Errorf("failed to subscribe for %s: %w", stockCode, err)
		}
		e.activeSubscriptions[stockCode] = sub
		e.logger.Printf("Subscribed to market data for %s", stockCode)
	}

	e.logger.Printf("StrategyExecutor started successfully.")
	return nil
}

// listenToMarketData processes data from a specific stock's channel.
func (e *StrategyExecutor) listenToMarketData(stockCode string) {
	e.wg.Add(1)
	defer e.wg.Done()

	dataChannel, ok := e.dataChannels[stockCode]
	if !ok {
		e.logger.Printf("Error: No data channel found for stock %s", stockCode)
		return
	}

	for {
		select {
		case data, open := <-dataChannel:
			if !open {
				e.logger.Printf("Data channel for %s closed.", stockCode)
				return
			}
			// Process data for all strategies interested in this stock (currently all strategies)
			e.processIncomingData(stockCode, data)
		case <-e.ctx.Done():
			e.logger.Printf("Stopping market data listener for %s due to context cancellation.", stockCode)
			return
		}
	}
}

// processIncomingData fans out data to all strategies and collects signals.
func (e *StrategyExecutor) processIncomingData(stockCode string, data models.Stock) {
	var allSignals []Signal
	var strategyWg sync.WaitGroup
	signalChan := make(chan []Signal, len(e.strategies))

	e.mu.RLock()
	strategiesToRun := make([]Strategy, 0, len(e.strategies))
	for _, s := range e.strategies {
		strategiesToRun = append(strategiesToRun, s)
	}
	e.mu.RUnlock()

	for _, strategy := range strategiesToRun {
		strategyWg.Add(1)
		go func(s Strategy, stockData models.Stock) {
			defer strategyWg.Done()
			// Strategies might need a small buffer of historical data to generate signals.
			// Here, we're passing only the latest tick. Strategies need to handle this or
			// the executor needs to maintain a buffer per stock per strategy.
			// For simplicity, passing single stock data. Strategies like MA need history.
			// This means the Strategy interface or its implementations need to manage historical data windows.
			// OR, the DataProvider's SubscribeToUpdates could provide a window, or we fetch history here.

			// Let's assume for now that strategies manage their own state or the executor can fetch history.
			// Fetching minimal history for strategy to operate on the new tick:
			// This is a simplified approach; a real system would have more sophisticated data window management.
			
			// Fetch historical data for context if needed by the strategy. Many strategies need more than one point.
			// The amount of history needed depends on the strategy's longest lookback period.
			// This could be optimized by caching or having strategies manage their data windows.
			const historyLookbackBars = 100 // Example, should be configurable or strategy-dependent
			
			historicalData, err := e.fetchMinimalHistory(e.ctx, stockData.Code, stockData.Timestamp, historyLookbackBars)
			if err != nil {
				e.logger.Printf("Error fetching history for %s for strategy %s: %v", stockData.Code, s.Name(), err)
				signalChan <- nil
				return
			}
			dataForSignalGeneration := append(historicalData, stockData) // Add current tick to historical window

			signals, err := s.GenerateSignals(e.ctx, dataForSignalGeneration) // Pass the window
			if err != nil {
				e.logger.Printf("Error generating signals from strategy %s for %s: %v", s.Name(), stockData.Code, err)
				if e.notifier != nil {
					e.notifier.NotifyError(e.ctx, err, s.Name(), fmt.Sprintf("Signal generation error for %s", stockData.Code))
				}
				signalChan <- nil
				return
			}
			signalChan <- signals
		}(strategy, data) // Pass current data point
	}

	strategyWg.Wait()
	close(signalChan)

	for signals := range signalChan {
		if signals != nil {
			allSignals = append(allSignals, signals...)
		}
	}

	if len(allSignals) > 0 {
		e.processAndExecuteSignals(allSignals)
	}
}

// fetchMinimalHistory is a helper to get recent data points for strategies.
func (e *StrategyExecutor) fetchMinimalHistory(ctx context.Context, stockCode string, currentTimestamp time.Time, bars int) ([]models.Stock, error) {
	if e.stockProvider == nil {
		return nil, fmt.Errorf("stockProvider (for historical data) is not configured in executor")
	}
	// Estimate start time based on 1-minute interval for 'bars'. This is a rough estimate.
	// A more robust solution would consider market open hours and actual trading days.
	startTime := currentTimestamp.Add(-time.Duration(bars*2) * time.Minute) // *2 for safety margin
	 endTime := currentTimestamp.Add(-1 * time.Minute) // Up to the bar before the current one

	if startTime.After(endTime) { // If currentTimestamp is too early in the session
		return []models.Stock{}, nil
	}

	// FetchMinuteData is from stockApi.StockDataProvider
	history, err := e.stockProvider.FetchMinuteData(ctx, stockCode, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("FetchMinuteData for %s from %v to %v failed: %w", stockCode, startTime, endTime, err)
	}

	// Ensure data is sorted and take last 'bars' if more are returned.
	if len(history) > bars {
		history = history[len(history)-bars:]
	}
	return history, nil
}

// runBacktestMode simulates strategy execution over historical data.
func (e *StrategyExecutor) runBacktestMode() error {
	e.logger.Println("Running in Backtesting mode...")
	// For backtesting, we iterate through historical data for each subscribed stock.
	// This is a simplified backtester integrated into the executor.
	// A dedicated BacktestEngine (Action 9-8) would be more comprehensive.

	if e.stockProvider == nil {
		return fmt.Errorf("stockProvider (for historical data) must be configured for backtesting mode")
	}

	// Determine the overall period for backtesting (e.g., from strategy params or executor config)
	// This part needs a defined date range for backtesting.
	// For this example, let's assume a fixed period or it comes from config.
	// Example: 1 year of data.
	endTime := time.Now()
	startTime := endTime.AddDate(-1, 0, 0)

	for _, stockCode := range e.config.SubscribedStockCodes {
		e.wg.Add(1)
		go func(sc string) {
			defer e.wg.Done()
			e.logger.Printf("Backtesting stock: %s from %v to %v", sc, startTime, endTime)
			historicalData, err := e.stockProvider.FetchMinuteData(e.ctx, sc, startTime, endTime)
			if err != nil {
				e.logger.Printf("Failed to fetch historical data for %s: %v", sc, err)
				if e.notifier != nil {
					e.notifier.NotifyError(e.ctx, err, "StrategyExecutor-Backtest", fmt.Sprintf("Historical data fetch failed for %s", sc))
				}
				return
			}

			// Initialize strategies for this stock's backtest if needed
			// (Strategies are already added, so their init should be done)

			// Feed data point by data point to simulate time flow
			for _, dataPoint := range historicalData {
				select {
				case <-e.ctx.Done():
					e.logger.Printf("Backtest for %s cancelled.", sc)
					return
				default:
					// In backtest mode, we simulate processing each historical data point as if it's a new tick.
					// The challenge is providing the historical window for signal generation accurately.
					// For a proper backtest, each strategy's GenerateSignals would be called with data UP TO dataPoint.Timestamp.
					// This means the call to GenerateSignals inside processIncomingData needs to be adapted for backtesting.
					// The current processIncomingData fetches recent history based on stockData.Timestamp.
					// This might be acceptable if fetchMinimalHistory is efficient and uses a cache itself.
					e.processIncomingData(sc, dataPoint)
				}
			}
			// After processing all data for a stock, strategies can produce final backtest reports.
			// This would require strategy interface changes or a different flow.
			e.logger.Printf("Backtesting for %s completed.", sc)
			// Here, you could call a strategy.FinalizeBacktest() or similar if it existed.
		}(stockCode)
	}

	e.wg.Wait() // Wait for all stock backtests to finish
	e.logger.Println("All backtests completed.")
	// Aggregate results from positionManager or individual strategies if they store them.
	return nil
}

// Stop gracefully shuts down the StrategyExecutor.
func (e *StrategyExecutor) Stop() {
	e.logger.Println("Stopping StrategyExecutor...")
	e.cancelFunc() // Signal cancellation to all goroutines

	e.mu.Lock()
	for stockCode, sub := range e.activeSubscriptions {
		if err := sub.Unsubscribe(); err != nil {
			e.logger.Printf("Error unsubscribing from %s: %v", stockCode, err)
		}
		delete(e.activeSubscriptions, stockCode)
		if e.dataChannels[stockCode] != nil {
		    close(e.dataChannels[stockCode])
		}
	}
	e.mu.Unlock()

	finished := make(chan struct{})
	go func() {
		e.wg.Wait() // Wait for all goroutines to finish
		close(finished)
	}()

	select {
	case <-finished:
		e.logger.Println("All executor goroutines finished.")
	case <-time.After(10 * time.Second): // Timeout for shutdown
		e.logger.Println("StrategyExecutor stop timed out. Some goroutines may still be running.")
	}
	e.logger.Println("StrategyExecutor stopped.")
}

// processAndExecuteSignals filters signals and then attempts to execute them.
func (e *StrategyExecutor) processAndExecuteSignals(signals []Signal) {
	filteredSignals := e.applySignalFilters(signals)

	for _, signal := range filteredSignals {
		e.mu.Lock()
		// Debounce signals: process one signal per strategy per stock within a short timeframe (e.g., 1 minute)
		signalKey := fmt.Sprintf("%s_%s", signal.Strategy, signal.StockCode)
		lastTime, found := e.lastProcessedSignal[signalKey]
		debounceDuration := 1 * time.Minute // Configurable
		if found && time.Since(lastTime) < debounceDuration {
			e.logger.Printf("Debouncing signal for %s from strategy %s. Last signal at %v", signal.StockCode, signal.Strategy, lastTime)
			e.mu.Unlock()
			continue
		}
		e.lastProcessedSignal[signalKey] = time.Now()
		e.mu.Unlock()

		if e.notifier != nil {
			e.notifier.NotifySignal(e.ctx, signal)
		}
		e.logger.Printf("Processing signal: %+v", signal)

		// Call position sizing and risk management
		size, err := e.calculatePositionSizeAndRisk(signal)
		if err != nil {
			e.logger.Printf("Error calculating position size for signal %+v: %v", signal, err)
			if e.notifier != nil {
				e.notifier.NotifyError(e.ctx, err, signal.Strategy, "Position sizing error")
			}
			continue
		}

		if size == 0 {
			e.logger.Printf("Calculated position size is 0 for signal: %+v. No order placed.", signal)
			continue
		}

		// Create and place order
		order := Order{
			StockCode: signal.StockCode,
			Type:      OrderType(signal.Type), // Assumes SignalType (BUY/SELL) matches OrderType
			Quantity:  size,
			Price:     signal.Price, // Could be market or limit depending on strategy/config
			Timestamp: time.Now(),
			Status:    OrderStatusNew,
			Strategy:  signal.Strategy,
			Notes:     signal.Notes,
		}

		orderID, err := e.orderExecutor.PlaceOrder(e.ctx, order)
		if err != nil {
			e.logger.Printf("Failed to place order for signal %+v: %v", signal, err)
			if e.notifier != nil {
				updatedOrder := order
				updatedOrder.Status = OrderStatusError
				e.notifier.NotifyOrder(e.ctx, updatedOrder)
				e.notifier.NotifyError(e.ctx, err, signal.Strategy, "Order placement failed")
			}
			continue
		}
		order.ID = orderID
		e.logger.Printf("Order placed successfully for signal %+v. OrderID: %s", signal, orderID)
		if e.notifier != nil {
			e.notifier.NotifyOrder(e.ctx, order)
		}

		// PositionManager should be updated upon order fill confirmation, not here.
		// This requires a mechanism to monitor order statuses and act on fills.
		// For paper trading, we might simulate immediate fill.
		if e.config.ExecutionMode == PaperTrading {
			e.simulateOrderFill(order)
		}
	}
}

// applySignalFilters processes raw signals, applying filters like strength threshold.
func (e *StrategyExecutor) applySignalFilters(signals []Signal) []Signal {
	var filteredSignals []Signal
	for _, signal := range signals {
		if signal.Strength >= e.config.SignalStrengthFilter {
			filteredSignals = append(filteredSignals, signal)
		} else {
			e.logger.Printf("Signal filtered out due to low strength (%.2f < %.2f): %+v",
				signal.Strength, e.config.SignalStrengthFilter, signal)
		}
	}
	return filteredSignals
}

// calculatePositionSizeAndRisk determines the size of the trade based on risk parameters.
func (e *StrategyExecutor) calculatePositionSizeAndRisk(signal Signal) (float64, error) {
	cash, err := e.positionManager.GetCashBalance(e.ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get cash balance: %w", err)
	}

	riskAmount := cash * (e.config.DefaultRiskPerTrade / 100.0)
	if riskAmount <= 0 {
		return 0, fmt.Errorf("risk amount is zero or negative, cannot size position (cash: %.2f, risk%%: %.2f)", cash, e.config.DefaultRiskPerTrade)
	}

	// Determine stop-loss distance. This is crucial and strategy-dependent.
	// Strategies should ideally provide a suggested stop-loss point or ATR value.
	// For now, a simple percentage-based stop-loss from signal price.
	stopLossDistance := signal.Price * 0.02 // Example: 2% stop-loss. This should be more sophisticated.
	if signal.Type == SignalSell { // For short selling, stop loss is above entry.
		// This example primarily handles long positions for simplicity.
		// For short, this would be (stopPrice - entryPrice).
		// If this is closing a long position, size is determined by existing position.
		 existingPos, found := e.positionManager.GetPosition(e.ctx, signal.StockCode)
		 if found && signal.Type == SignalSell { // Closing an existing long position
		 	return existingPos.Quantity, nil
		 }
		 // If it's initiating a short, more logic needed. For now, assume closing long or new long.
	}

	if stopLossDistance <= 0 {
		return 0, fmt.Errorf("stop-loss distance is zero or negative (price: %.2f), cannot calculate size", signal.Price)
	}

	quantity := math.Floor(riskAmount / stopLossDistance)

	// Ensure quantity does not create a position larger than available cash for buying
	if signal.Type == SignalBuy {
		cost := quantity * signal.Price
		if cost > cash {
			quantity = math.Floor(cash / signal.Price)
			e.logger.Printf("Reduced quantity for %s due to cash limit. Original: %.2f, New: %.2f", signal.StockCode, math.Floor(riskAmount/stopLossDistance), quantity)
		}
	}

	if quantity <= 0 {
		return 0, nil // Not an error, but means no trade due to risk/cash constraints.
	}

	return quantity, nil
}

// simulateOrderFill is used in PaperTrading mode to simulate an order being filled.
func (e *StrategyExecutor) simulateOrderFill(order Order) {
	e.logger.Printf("Simulating fill for order ID %s: %+v", order.ID, order)
	now := time.Now()

	fillPrice := order.Price
	commission := 0.0
	slippageAmount := 0.0

	if e.config.ExecutionMode == PaperTrading {
		// Apply slippage for paper trading
		if e.config.PaperTradingConfig.Slippage > 0 {
			slipDirection := 1.0
			if order.Type == OrderTypeBuy { // Buy orders might fill at a slightly higher price
				slipDirection = 1.0
			} else { // Sell orders might fill at a slightly lower price
				slipDirection = -1.0
			}
			slippageAmount = order.Price * e.config.PaperTradingConfig.Slippage * slipDirection
			fillPrice += slippageAmount
		}
		// Apply commission
		if e.config.PaperTradingConfig.CommissionRate > 0 {
			commission = fillPrice * order.Quantity * e.config.PaperTradingConfig.CommissionRate
		}
	}

	trade := Trade{
		StockCode:     order.StockCode,
		EntryTime:     now, // For simplicity, fill time is now. Could be linked to order time.
		EntryPrice:    fillPrice, // This is the entry/exit price of THIS trade leg.
		ExitTime:      time.Time{}, // Not set for single leg fill
		ExitPrice:     0,
		Quantity:      order.Quantity,
		Direction:     string(order.Type),
		ProfitLoss:    0, // P&L is calculated when a position is closed.
		ReturnRate:    0,
		HoldingPeriod: 0,
		ExitReason:    "Fill", // Or order notes
	}

	// Update position manager
	err := e.positionManager.UpdatePosition(e.ctx, trade) // UpdatePosition needs the trade that got filled
	if err != nil {
		e.logger.Printf("Error updating position after simulated fill for order %s: %v", order.ID, err)
		if e.notifier != nil {
			e.notifier.NotifyError(e.ctx, err, order.Strategy, "Position update failed after fill")
		}
		return
	}

	// Update cash balance
	costOrProceeds := fillPrice * order.Quantity
	cashChange := -costOrProceeds // Negative for buy, positive for sell
	if order.Type == OrderTypeSell {
		cashChange = costOrProceeds
	}
	cashChange -= commission // Subtract commission regardless of buy/sell

	err = e.positionManager.UpdateCashBalance(e.ctx, cashChange)
	if err != nil {
		e.logger.Printf("Error updating cash balance after simulated fill for order %s: %v", order.ID, err)
		// Handle error
	}

	// Notify about the (simulated) trade execution
	// This 'trade' is more like a 'fill event'. A full Trade (entry+exit) is different.
	if e.notifier != nil {
		// Create a 'Trade' record representing this fill for notification if needed.
		// The 'trade' variable here can be used for that, though its P&L is 0.
		e.notifier.NotifyTrade(e.ctx, trade) 
	}

	order.Status = OrderStatusFilled
	order.Price = fillPrice // Update order price to actual fill price
	if e.notifier != nil {
	    e.notifier.NotifyOrder(e.ctx, order) // Notify updated order status
	}

	e.logger.Printf("Simulated fill processed for %s. Fill Price: %.2f, Commission: %.2f", order.ID, fillPrice, commission)
}

// GetStatus provides insights into the executor's current state (Placeholder)
func (e *StrategyExecutor) GetStatus() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	activeSubs := []string{}
	for code := range e.activeSubscriptions {
		activeSubs = append(activeSubs, code)
	}

	status := map[string]interface{}{
		"executionMode":        e.config.ExecutionMode.String(),
		"subscribedStockCodes": e.config.SubscribedStockCodes,
		"activeSubscriptions":  activeSubs,
		"runningStrategies":    len(e.strategies),
		"contextError":         e.ctx.Err(),
	}
	// Add more details: active positions count, cash balance, etc.
	// currentPositions, _ := e.positionManager.GetAllPositions(e.ctx)
	// cash, _ := e.positionManager.GetCashBalance(e.ctx)
	// status["activePositionsCount"] = len(currentPositions)
	// status["cashBalance"] = cash

	return status
}

// Stringer methods for enums (can be generated by `stringer` tool)
func (i ExecutionMode) String() string {
	switch i {
	case PaperTrading:
		return "PaperTrading"
	case LiveTrading:
		return "LiveTrading"
	case Backtesting:
		return "Backtesting"
	default:
		return fmt.Sprintf("ExecutionMode(%d)", i)
	}
}
