# Quantitative Trading Strategies Documentation

## Overview
This document provides comprehensive documentation for the quantitative trading module, including available strategies, implementation details, performance metrics, and usage guidelines.

## Table of Contents
1. [Strategy Types](#strategy-types)
2. [Strategy Implementation](#strategy-implementation)
3. [Technical Indicators](#technical-indicators)
4. [Backtesting Engine](#backtesting-engine)
5. [Position Management](#position-management)
6. [Performance Metrics](#performance-metrics)
7. [Usage Examples](#usage-examples)
8. [API Reference](#api-reference)
9. [Troubleshooting](#troubleshooting)
10. [Glossary](#glossary)

## Strategy Types

### Moving Average Crossover Strategy
The Moving Average Crossover strategy generates trading signals based on the intersection of two moving averages calculated over different time periods.

#### Parameters:
- `ShortPeriod`: Period for calculating the short-term moving average (default: 5)
- `LongPeriod`: Period for calculating the long-term moving average (default: 20)
- `MAType`: Type of moving average calculation ("SMA", "EMA", "WMA")
- `SignalSmoothing`: Apply additional smoothing to signals (default: false)

#### Signal Generation:
- Buy signal: When short MA crosses above long MA
- Sell signal: When short MA crosses below long MA

#### Optimization:
The optimal parameters depend on the specific stock and market conditions. The strategy provides methods to test various combinations of short and long periods to find optimal settings.

### RSI Strategy
The Relative Strength Index (RSI) strategy generates trading signals based on overbought and oversold conditions detected by the RSI indicator.

#### Parameters:
- `Period`: Number of periods to calculate RSI (default: 14)
- `OverboughtThreshold`: Level indicating overbought condition (default: 70)
- `OversoldThreshold`: Level indicating oversold condition (default: 30)
- `UseReversal`: Whether to wait for reversal confirmation (default: true)

#### Signal Generation:
- Buy signal: When RSI crosses above the oversold threshold
- Sell signal: When RSI crosses below the overbought threshold

### Bollinger Bands Strategy
The Bollinger Bands strategy uses the statistical concept of standard deviations from a moving average to generate trading signals.

#### Parameters:
- `Period`: Number of periods for moving average calculation (default: 20)
- `StdDevMultiplier`: Standard deviation multiplier for bands (default: 2.0)
- `MAType`: Type of moving average for the middle band (default: "SMA")
- `Strategy`: "Mean Reversion" or "Trend Following" approach

#### Signal Generation:
- Mean Reversion:
  - Buy signal: When price touches or crosses below the lower band
  - Sell signal: When price touches or crosses above the upper band
- Trend Following:
  - Buy signal: When price crosses above the upper band
  - Sell signal: When price crosses below the lower band

### Momentum Strategy
The Momentum strategy identifies and exploits directional price movements based on rate of change or similar momentum indicators.

#### Parameters:
- `LookbackPeriod`: Period for momentum calculation (default: 14)
- `ThresholdUp`: Threshold for upward momentum (default: 0.5)
- `ThresholdDown`: Threshold for downward momentum (default: -0.5)
- `ConfirmationPeriods`: Number of periods required to confirm trend (default: 2)

#### Signal Generation:
- Buy signal: When momentum crosses above the ThresholdUp
- Sell signal: When momentum crosses below the ThresholdDown

## Strategy Implementation

### Base Strategy Interface
All quantitative trading strategies implement the Strategy interface from the `service/quant/api.go` file:

```go
type Strategy interface {
    Initialize(params map[string]interface{}) error
    GenerateSignals(data []models.StockData) ([]models.Signal, error)
    Execute(signals []models.Signal, positionManager PositionManager) error
    Backtest(data []models.StockData) (models.PerformanceMetrics, error)
    GetName() string
    GetDescription() string
    GetParameters() map[string]interface{}
}
```

### Strategy Customization
The BaseStrategy struct provides common functionality that can be extended by specific strategy implementations. To customize a strategy:

1. Create a struct that embeds BaseStrategy
2. Implement specific signal generation logic
3. Override any methods that require custom behavior
4. Provide default parameters and validation

## Technical Indicators
The quantitative module provides a comprehensive set of technical indicators in `service/quant/technical_indicators.go`:

### Moving Averages
- Simple Moving Average (SMA)
- Exponential Moving Average (EMA)
- Weighted Moving Average (WMA)
- Volume Weighted Average Price (VWAP)

### Oscillators
- Relative Strength Index (RSI)
- Stochastic Oscillator
- Commodity Channel Index (CCI)
- Williams %R

### Trend Indicators
- Moving Average Convergence Divergence (MACD)
- Average Directional Index (ADX)
- Parabolic SAR

### Volatility Indicators
- Bollinger Bands
- Average True Range (ATR)
- Keltner Channels

### Volume Indicators
- On-Balance Volume (OBV)
- Accumulation/Distribution Line
- Money Flow Index (MFI)

## Backtesting Engine
The backtesting engine allows testing strategies against historical data to evaluate performance before deploying to live markets.

### Features:
- Historical data simulation
- Transaction cost modeling
- Slippage simulation
- Comprehensive performance metrics
- Multiple benchmark comparison
- Parameter optimization through grid search
- Monte Carlo simulation for robustness testing
- HTML and PDF report generation

### Usage Example:
```go
backtest := quant.NewBacktestEngine(
    strategy,
    stockData,
    &quant.BacktestConfig{
        InitialCapital: 100000,
        Commission: 0.0025,
        Slippage: 0.001,
        StartDate: "2022-01-01",
        EndDate: "2022-12-31",
    },
)
metrics, err := backtest.Run()
```

## Position Management
The position manager handles portfolio management, including position sizing, risk management, and order execution.

### Features:
- Portfolio tracking for multiple positions
- Position sizing strategies:
  - Fixed position size
  - Percentage of capital
  - Volatility-adjusted position sizing
  - Kelly criterion
- Risk management:
  - Stop loss orders
  - Take profit orders
  - Trailing stops
  - Maximum drawdown protection
- Portfolio rebalancing
- P&L tracking

### Position Sizing Methods:
1. **Fixed Size**: Uses a fixed number of shares for each trade
2. **Fixed Value**: Uses a fixed monetary amount for each trade
3. **Percentage**: Uses a percentage of available capital
4. **Volatility-Adjusted**: Adjusts position size based on market volatility

## Performance Metrics
The quantitative module calculates comprehensive performance metrics to evaluate strategy effectiveness:

### Return Metrics:
- Total Return
- Annual Return
- Daily/Monthly/Quarterly Returns
- Compound Annual Growth Rate (CAGR)

### Risk Metrics:
- Maximum Drawdown
- Value at Risk (VaR)
- Standard Deviation
- Downside Deviation
- Sortino Ratio
- Sharpe Ratio
- Calmar Ratio
- Alpha and Beta

### Trading Statistics:
- Win Rate
- Profit Factor
- Average Win/Loss
- Maximum Win/Loss
- Average Holding Period
- Number of Trades
- Payoff Ratio

## Usage Examples

### Creating a Custom Strategy
```go
type MyCustomStrategy struct {
    quant.BaseStrategy
    CustomParam float64
}

func NewMyCustomStrategy() *MyCustomStrategy {
    strategy := &MyCustomStrategy{
        BaseStrategy: *quant.NewBaseStrategy("MyCustomStrategy"),
        CustomParam: 0.5,
    }
    
    // Set default parameters
    strategy.SetParameter("lookback", 10)
    return strategy
}

func (s *MyCustomStrategy) GenerateSignals(data []models.StockData) ([]models.Signal, error) {
    signals := make([]models.Signal, len(data))
    // Custom signal generation logic
    return signals, nil
}
```

### Running a Backtest
```go
// Create strategy instance
strategy := quant.NewMACrossStrategy()
strategy.SetParameter("ShortPeriod", 5)
strategy.SetParameter("LongPeriod", 20)

// Get historical data
stockData, err := stockService.GetHistoricalData("600000.SH", 
                                               "2022-01-01",
                                               "2022-12-31",
                                               "1m")
if err != nil {
    log.Fatalf("Failed to get data: %v", err)
}

// Create backtest configuration
config := &quant.BacktestConfig{
    InitialCapital: 100000,
    Commission: 0.0025,
    Slippage: 0.001,
}

// Create and run backtest
engine := quant.NewBacktestEngine(strategy, stockData, config)
result, err := engine.Run()
if err != nil {
    log.Fatalf("Backtest failed: %v", err)
}

// Analyze results
fmt.Printf("Total Return: %.2f%%\n", result.TotalReturn * 100)
fmt.Printf("Sharpe Ratio: %.2f\n", result.SharpeRatio)
fmt.Printf("Max Drawdown: %.2f%%\n", result.MaxDrawdown * 100)
fmt.Printf("Win Rate: %.2f%%\n", result.WinRate * 100)
```

## API Reference

### Strategy Factory
```go
// Create strategy by name
strategy, err := quant.NewStrategy("MACross")

// Available strategy types
quant.GetAvailableStrategyTypes()
```

### Parameter Management
```go
// Set strategy parameters
strategy.SetParameter("ShortPeriod", 5)
strategy.SetParameter("LongPeriod", 20)

// Get all parameters
params := strategy.GetParameters()

// Validate parameters
err := strategy.ValidateParameters()
```

### Strategy Execution
```go
// Initialize strategy
err := strategy.Initialize(params)

// Generate signals
signals, err := strategy.GenerateSignals(stockData)

// Execute strategy
err := strategy.Execute(signals, positionManager)
```

### Position Management
```go
// Create position manager
positionManager := quant.NewPositionManager(100000)

// Create position
position, err := positionManager.OpenPosition("600000.SH", 
                                           quant.BUY, 
                                           100, 
                                           10.5)

// Add stop loss
position.SetStopLoss(10.0)

// Add take profit
position.SetTakeProfit(11.0)

// Close position
err := positionManager.ClosePosition(position.ID, 10.8)
```

## Troubleshooting

### Common Issues and Solutions

#### Strategy Not Generating Signals
**Problem**: The strategy is not generating any buy or sell signals.
**Solution**: 
- Verify input data format and completeness
- Check parameter values for appropriate ranges
- Ensure sufficient historical data for the selected lookback periods
- Check logs for any filtering or validation errors

#### Backtest Performance Issues
**Problem**: Backtest execution is slow or consuming too much memory.
**Solution**:
- Reduce the data time range or increase the sampling interval
- Optimize strategy calculations to avoid redundant operations
- Use batch processing for large datasets
- Check for memory leaks in custom strategy implementations

#### Parameter Optimization Issues
**Problem**: Parameter optimization taking too long or producing inconsistent results.
**Solution**:
- Narrow the parameter search range
- Use a stepped approach instead of testing all combinations
- Implement early stopping for unpromising parameter sets
- Use walk-forward optimization for better stability

## Glossary

### Common Terms in Quantitative Trading

**Alpha**: Excess return of an investment relative to a benchmark's return.

**Beta**: Measure of volatility compared to the market as a whole.

**Drawdown**: The peak-to-trough decline during a specific period.

**Mean Reversion**: Strategy based on the assumption that prices will revert to their average over time.

**Momentum**: Continuation of existing trends in price movement.

**Sharpe Ratio**: Average return earned in excess of the risk-free rate per unit of volatility.

**Slippage**: Difference between expected price and execution price.

**Stop-Loss**: Order to sell when price reaches a specified level to limit losses.

**Take-Profit**: Order to sell when price reaches a specified profit target.

**Volatility**: Statistical measure of the dispersion of returns for a given security or market index.

## References

1. Chan, E. P. (2013). Algorithmic Trading: Winning Strategies and Their Rationale. Wiley.
2. Kaufman, P. J. (2013). Trading Systems and Methods. Wiley.
3. Murphy, J. J. (1999). Technical Analysis of the Financial Markets. New York Institute of Finance.
4. Aldridge, I. (2013). High-Frequency Trading: A Practical Guide to Algorithmic Strategies and Trading Systems. Wiley.