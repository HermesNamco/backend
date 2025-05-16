package quant

import (
	"backend/models"
	"math"
)

// TechnicalIndicators provides a comprehensive library of technical indicators for strategy development
type TechnicalIndicators struct{}

// NewTechnicalIndicators creates a new technical indicators instance
func NewTechnicalIndicators() *TechnicalIndicators {
	return &TechnicalIndicators{}
}

// CalculateSMA calculates Simple Moving Average for a given period
func (ti *TechnicalIndicators) CalculateSMA(prices []float64, period int) []float64 {
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

// CalculateEMA calculates Exponential Moving Average for a given period
func (ti *TechnicalIndicators) CalculateEMA(prices []float64, period int) []float64 {
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

// CalculateWMA calculates Weighted Moving Average for a given period
func (ti *TechnicalIndicators) CalculateWMA(prices []float64, period int) []float64 {
	if len(prices) < period {
		return nil
	}
	
	result := make([]float64, len(prices))
	
	// First (period-1) elements have no WMA
	for i := 0; i < period-1; i++ {
		result[i] = math.NaN()
	}
	
	// Calculate denominator (sum of weights)
	denominator := float64(period * (period + 1)) / 2.0
	
	// Calculate WMA for each point
	for i := period - 1; i < len(prices); i++ {
		sum := 0.0
		for j := 0; j < period; j++ {
			// Apply weight (period-j) to price[i-j]
			weight := float64(period - j)
			sum += prices[i-period+j+1] * weight
		}
		result[i] = sum / denominator
	}
	
	return result
}

// CalculateVWAP calculates Volume Weighted Average Price
func (ti *TechnicalIndicators) CalculateVWAP(prices []float64, volumes []float64) []float64 {
	if len(prices) != len(volumes) {
		return nil
	}
	
	result := make([]float64, len(prices))
	cumulativeVolume := 0.0
	cumulativePV := 0.0
	
	result[0] = prices[0] // First point is just the price
	
	for i := 0; i < len(prices); i++ {
		pv := prices[i] * volumes[i]
		cumulativePV += pv
		cumulativeVolume += volumes[i]
		
		if cumulativeVolume > 0 {
			result[i] = cumulativePV / cumulativeVolume
		} else {
			result[i] = prices[i] // No volume, use price
		}
	}
	
	return result
}

// CalculateRSI calculates Relative Strength Index for a given period
func (ti *TechnicalIndicators) CalculateRSI(prices []float64, period int) []float64 {
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

// CalculateStochastic calculates Stochastic Oscillator (%K and %D)
func (ti *TechnicalIndicators) CalculateStochastic(high, low, close []float64, periodK, periodD int) ([]float64, []float64) {
	length := len(close)
	if length < periodK || len(high) != length || len(low) != length {
		return nil, nil
	}
	
	// Calculate %K
	k := make([]float64, length)
	for i := 0; i < periodK-1; i++ {
		k[i] = math.NaN()
	}
	
	for i := periodK - 1; i < length; i++ {
		// Find highest high and lowest low in the lookback period
		highest := high[i-periodK+1]
		lowest := low[i-periodK+1]
		
		for j := i - periodK + 2; j <= i; j++ {
			if high[j] > highest {
				highest = high[j]
			}
			if low[j] < lowest {
				lowest = low[j]
			}
		}
		
		// Calculate %K
		if highest == lowest {
			k[i] = 50 // If no range, use middle value
		} else {
			k[i] = 100 * ((close[i] - lowest) / (highest - lowest))
		}
	}
	
	// Calculate %D (SMA of %K)
	d := ti.CalculateSMA(k, periodD)
	
	return k, d
}

// CalculateMACD calculates Moving Average Convergence Divergence
func (ti *TechnicalIndicators) CalculateMACD(prices []float64, fastPeriod, slowPeriod, signalPeriod int) ([]float64, []float64, []float64) {
	if len(prices) < slowPeriod + signalPeriod {
		return nil, nil, nil
	}
	
	// Calculate fast and slow EMAs
	fastEMA := ti.CalculateEMA(prices, fastPeriod)
	slowEMA := ti.CalculateEMA(prices, slowPeriod)
	
	// Calculate MACD line (fast EMA - slow EMA)
	macdLine := make([]float64, len(prices))
	for i := 0; i < len(prices); i++ {
		if i < slowPeriod-1 {
			macdLine[i] = math.NaN()
		} else {
			macdLine[i] = fastEMA[i] - slowEMA[i]
		}
	}
	
	// Calculate signal line (EMA of MACD line)
	signalLine := ti.CalculateEMA(macdLine, signalPeriod)
	
	// Calculate histogram (MACD line - signal line)
	histogram := make([]float64, len(prices))
	for i := 0; i < len(prices); i++ {
		if math.IsNaN(signalLine[i]) || math.IsNaN(macdLine[i]) {
			histogram[i] = math.NaN()
		} else {
			histogram[i] = macdLine[i] - signalLine[i]
		}
	}
	
	return macdLine, signalLine, histogram
}

// CalculateBollingerBands calculates Bollinger Bands
func (ti *TechnicalIndicators) CalculateBollingerBands(prices []float64, period int, stdDevMultiplier float64) ([]float64, []float64, []float64) {
	if len(prices) < period {
		return nil, nil, nil
	}
	
	// Calculate SMA (middle band)
	middle := ti.CalculateSMA(prices, period)
	
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

// CalculateATR calculates Average True Range for volatility measurement
func (ti *TechnicalIndicators) CalculateATR(stockData []models.Stock, period int) []float64 {
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
	return ti.CalculateSMA(trueRanges, period)
}

// CalculateADX calculates Average Directional Index
func (ti *TechnicalIndicators) CalculateADX(stockData []models.Stock, period int) []float64 {
	length := len(stockData)
	if length < period*2 {
		return nil
	}
	
	// Create slices for calculations
	tr := make([]float64, length)
	plusDM := make([]float64, length)
	minusDM := make([]float64, length)
	
	// Calculate True Range and Directional Movement
	for i := 1; i < length; i++ {
		// Calculate True Range
		tr1 := stockData[i].High - stockData[i].Low
		tr2 := math.Abs(stockData[i].High - stockData[i-1].Close)
		tr3 := math.Abs(stockData[i].Low - stockData[i-1].Close)
		tr[i] = math.Max(tr1, math.Max(tr2, tr3))
		
		// Calculate +DM and -DM
		upMove := stockData[i].High - stockData[i-1].High
		downMove := stockData[i-1].Low - stockData[i].Low
		
		if upMove > downMove && upMove > 0 {
			plusDM[i] = upMove
		} else {
			plusDM[i] = 0
		}
		
		if downMove > upMove && downMove > 0 {
			minusDM[i] = downMove
		} else {
			minusDM[i] = 0
		}
	}
	
	// Calculate smoothed values
	smoothedTR := make([]float64, length)
	smoothedPlusDM := make([]float64, length)
	smoothedMinusDM := make([]float64, length)
	
	// Initial values for smoothing
	for i := 0; i < period && i < length; i++ {
		if i == 0 {
			smoothedTR[i] = tr[i]
			smoothedPlusDM[i] = plusDM[i]
			smoothedMinusDM[i] = minusDM[i]
		} else {
			smoothedTR[i] = smoothedTR[i-1] - (smoothedTR[i-1]/float64(period)) + tr[i]
			smoothedPlusDM[i] = smoothedPlusDM[i-1] - (smoothedPlusDM[i-1]/float64(period)) + plusDM[i]
			smoothedMinusDM[i] = smoothedMinusDM[i-1] - (smoothedMinusDM[i-1]/float64(period)) + minusDM[i]
		}
	}
	
	// Continue smoothing for the rest of the data
	for i := period; i < length; i++ {
		smoothedTR[i] = smoothedTR[i-1] - (smoothedTR[i-1]/float64(period)) + tr[i]
		smoothedPlusDM[i] = smoothedPlusDM[i-1] - (smoothedPlusDM[i-1]/float64(period)) + plusDM[i]
		smoothedMinusDM[i] = smoothedMinusDM[i-1] - (smoothedMinusDM[i-1]/float64(period)) + minusDM[i]
	}
	
	// Calculate +DI and -DI
	plusDI := make([]float64, length)
	minusDI := make([]float64, length)
	
	for i := 0; i < length; i++ {
		if smoothedTR[i] > 0 {
			plusDI[i] = 100 * (smoothedPlusDM[i] / smoothedTR[i])
			minusDI[i] = 100 * (smoothedMinusDM[i] / smoothedTR[i])
		} else {
			plusDI[i] = 0
			minusDI[i] = 0
		}
	}
	
	// Calculate DX and ADX
	dx := make([]float64, length)
	adx := make([]float64, length)
	
	for i := 0; i < length; i++ {
		if plusDI[i]+minusDI[i] > 0 {
			dx[i] = 100 * (math.Abs(plusDI[i]-minusDI[i]) / (plusDI[i] + minusDI[i]))
		} else {
			dx[i] = 0
		}
	}
	
	// First (period*2-1) elements have no ADX
	for i := 0; i < period*2-1; i++ {
		adx[i] = math.NaN()
	}
	
	// Calculate the first ADX
	sum := 0.0
	for i := period; i < period*2; i++ {
		sum += dx[i]
	}
	adx[period*2-1] = sum / float64(period)
	
	// Calculate the rest of ADX values
	for i := period*2; i < length; i++ {
		adx[i] = ((adx[i-1] * float64(period-1)) + dx[i]) / float64(period)
	}
	
	return adx
}

// ExtractPrices extracts close prices from stock data
func (ti *TechnicalIndicators) ExtractPrices(stockData []models.Stock) []float64 {
	prices := make([]float64, len(stockData))
	for i, stock := range stockData {
		prices[i] = stock.Close
	}
	return prices
}

// ExtractVolumes extracts volumes from stock data
func (ti *TechnicalIndicators) ExtractVolumes(stockData []models.Stock) []float64 {
	volumes := make([]float64, len(stockData))
	for i, stock := range stockData {
		volumes[i] = float64(stock.Volume)
	}
	return volumes
}