package models

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
)

// Stock represents A-share market minute-level stock data
type Stock struct {
	Code      string    `json:"code" validate:"required"`       // Stock code with exchange suffix (e.g., 600000.SH)
	Name      string    `json:"name" validate:"required"`       // Stock name
	Timestamp time.Time `json:"timestamp" validate:"required"`  // Data timestamp
	Open      float64   `json:"open" validate:"required"`       // Opening price
	Close     float64   `json:"close" validate:"required"`      // Closing price
	High      float64   `json:"high" validate:"required"`       // Highest price
	Low       float64   `json:"low" validate:"required"`        // Lowest price
	Volume    int64     `json:"volume" validate:"required,gte=0"` // Trading volume
	Amount    float64   `json:"amount" validate:"required,gte=0"` // Trading amount in CNY
	Change    float64   `json:"change"`                         // Price change
	Percent   float64   `json:"percent"`                        // Price change percentage
	Exchange  string    `json:"exchange"`                       // Exchange code (SH, SZ, BJ)
	CreatedAt time.Time `json:"createdAt"`                      // Record creation time
	UpdatedAt time.Time `json:"updatedAt"`                      // Record update time
}

// StockDataPoint represents a single time point of stock data for serialization
type StockDataPoint struct {
	Time   int64   `json:"t"` // Unix timestamp in seconds
	Open   float64 `json:"o"`
	Close  float64 `json:"c"`
	High   float64 `json:"h"`
	Low    float64 `json:"l"`
	Volume int64   `json:"v"`
	Amount float64 `json:"a"`
}

// StockRepository interface represents the expected behavior for stock data storage
type StockRepository interface {
	FindByCode(code string, start, end time.Time) ([]Stock, error)
	FindByCodeAndTime(code string, timestamp time.Time) (*Stock, error)
	FindLatest(code string) (*Stock, error)
	BatchInsert(stocks []Stock) error
	DeleteByCodeAndTime(code string, start, end time.Time) error
}

// StockService interface represents the expected behavior for stock business logic
type StockService interface {
	GetStockData(code string, start, end time.Time) ([]Stock, error)
	GetLatestStockData(code string) (*Stock, error)
	SaveStockData(stocks []Stock) error
	ValidateStock(stock *Stock) error
}

// NewStock creates a new stock data record with default values
func NewStock() *Stock {
	now := time.Now()
	return &Stock{
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Validate checks if the stock data is valid
func (s *Stock) Validate() error {
	validate := validator.New()
	return validate.Struct(s)
}

// CalculateDerived calculates derived values like change and percent
func (s *Stock) CalculateDerived(previousClose float64) {
	if previousClose > 0 {
		s.Change = s.Close - previousClose
		s.Percent = (s.Change / previousClose) * 100
	}
}

// ExtractExchange extracts the exchange code from the stock code
func (s *Stock) ExtractExchange() {
	if s.Code == "" {
		return
	}
	
	parts := strings.Split(s.Code, ".")
	if len(parts) == 2 {
		s.Exchange = parts[1]
	}
}

// IsValidCode checks if the stock code is in valid format
func (s *Stock) IsValidCode() bool {
	if s.Code == "" {
		return false
	}
	
	parts := strings.Split(s.Code, ".")
	if len(parts) != 2 {
		return false
	}
	
	// Check exchange suffix
	exchange := parts[1]
	if exchange != "SH" && exchange != "SZ" && exchange != "BJ" {
		return false
	}
	
	// Check code format
	code := parts[0]
	if len(code) != 6 {
		return false
	}
	
	// Check if code is numeric
	_, err := strconv.Atoi(code)
	return err == nil
}

// BeforeCreate prepares a stock for creation
func (s *Stock) BeforeCreate() error {
	now := time.Now()
	s.CreatedAt = now
	s.UpdatedAt = now
	
	if !s.IsValidCode() {
		return errors.New("invalid stock code format")
	}
	
	s.ExtractExchange()
	return nil
}

// BeforeUpdate prepares a stock for update
func (s *Stock) BeforeUpdate() {
	s.UpdatedAt = time.Now()
}

// ToDataPoint converts the stock to a simplified data point format
func (s *Stock) ToDataPoint() StockDataPoint {
	return StockDataPoint{
		Time:   s.Timestamp.Unix(),
		Open:   s.Open,
		Close:  s.Close,
		High:   s.High,
		Low:    s.Low,
		Volume: s.Volume,
		Amount: s.Amount,
	}
}

// FromDataPoint converts a data point to stock data
func (s *Stock) FromDataPoint(code string, name string, dp StockDataPoint) error {
	s.Code = code
	s.Name = name
	s.Timestamp = time.Unix(dp.Time, 0)
	s.Open = dp.Open
	s.Close = dp.Close
	s.High = dp.High
	s.Low = dp.Low
	s.Volume = dp.Volume
	s.Amount = dp.Amount
	
	s.ExtractExchange()
	
	if !s.IsValidCode() {
		return fmt.Errorf("invalid stock code: %s", code)
	}
	
	return nil
}

// FromTuShareFormat converts data from TuShare API format to stock structure
func (s *Stock) FromTuShareFormat(data map[string]interface{}) error {
	// Extract fields from TuShare data format
	if code, ok := data["ts_code"].(string); ok {
		s.Code = code
	} else {
		return errors.New("missing or invalid ts_code field")
	}
	
	if name, ok := data["name"].(string); ok {
		s.Name = name
	}
	
	if timeStr, ok := data["trade_time"].(string); ok {
		// Parse time from TuShare format (YYYY-MM-DD HH:MM:SS)
		timestamp, err := time.Parse("2006-01-02 15:04:05", timeStr)
		if err != nil {
			return fmt.Errorf("invalid time format: %v", err)
		}
		s.Timestamp = timestamp
	} else {
		return errors.New("missing or invalid trade_time field")
	}
	
	// Parse numeric fields
	if open, ok := data["open"].(float64); ok {
		s.Open = open
	} else {
		return errors.New("missing or invalid open field")
	}
	
	if close, ok := data["close"].(float64); ok {
		s.Close = close
	} else {
		return errors.New("missing or invalid close field")
	}
	
	if high, ok := data["high"].(float64); ok {
		s.High = high
	} else {
		return errors.New("missing or invalid high field")
	}
	
	if low, ok := data["low"].(float64); ok {
		s.Low = low
	} else {
		return errors.New("missing or invalid low field")
	}
	
	if vol, ok := data["vol"].(float64); ok {
		s.Volume = int64(vol)
	} else {
		return errors.New("missing or invalid vol field")
	}
	
	if amount, ok := data["amount"].(float64); ok {
		s.Amount = amount
	} else {
		return errors.New("missing or invalid amount field")
	}
	
	// Extract exchange from code
	s.ExtractExchange()
	
	return nil
}

// FromTonghuashunFormat converts data from Tonghuashun API format to stock structure
func (s *Stock) FromTonghuashunFormat(data map[string]interface{}) error {
	// Extract fields from Tonghuashun data format
	if code, ok := data["code"].(string); ok {
		s.Code = code
	} else {
		return errors.New("missing or invalid code field")
	}
	
	if name, ok := data["name"].(string); ok {
		s.Name = name
	}
	
	// Parse timestamp (Tonghuashun might use Unix timestamp)
	if timestamp, ok := data["time"].(float64); ok {
		s.Timestamp = time.Unix(int64(timestamp), 0)
	} else if timeStr, ok := data["time"].(string); ok {
		// Try parsing time as string (format might vary)
		timestamp, err := time.Parse("2006-01-02 15:04:05", timeStr)
		if err != nil {
			return fmt.Errorf("invalid time format: %v", err)
		}
		s.Timestamp = timestamp
	} else {
		return errors.New("missing or invalid time field")
	}
	
	// Parse numeric fields
	if open, ok := data["open"].(float64); ok {
		s.Open = open
	} else {
		return errors.New("missing or invalid open field")
	}
	
	if close, ok := data["price"].(float64); ok { // Tonghuashun might use 'price' for closing price
		s.Close = close
	} else {
		return errors.New("missing or invalid price field")
	}
	
	if high, ok := data["high"].(float64); ok {
		s.High = high
	} else {
		return errors.New("missing or invalid high field")
	}
	
	if low, ok := data["low"].(float64); ok {
		s.Low = low
	} else {
		return errors.New("missing or invalid low field")
	}
	
	if vol, ok := data["volume"].(float64); ok {
		s.Volume = int64(vol)
	} else {
		return errors.New("missing or invalid volume field")
	}
	
	if amount, ok := data["amount"].(float64); ok {
		s.Amount = amount
	} else {
		return errors.New("missing or invalid amount field")
	}
	
	// Extract exchange from code
	s.ExtractExchange()
	
	return nil
}