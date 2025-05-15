package stock

import (
	"context"
	"errors"
	"fmt"
	"time"

	"backend/models"
)

// StockDataProvider is an interface for retrieving minute-level stock data
type StockDataProvider interface {
	// FetchMinuteData retrieves minute-level data for a specific stock within time range
	FetchMinuteData(ctx context.Context, code string, start, end time.Time) ([]models.Stock, error)
	
	// FetchLatestMinute retrieves the latest minute data for a specific stock
	FetchLatestMinute(ctx context.Context, code string) (*models.Stock, error)
	
	// FetchBatchMinuteData retrieves minute-level data for multiple stocks
	FetchBatchMinuteData(ctx context.Context, codes []string, start, end time.Time) (map[string][]models.Stock, error)
	
	// Name returns the provider name
	Name() string
	
	// Initialize sets up the provider with configuration
	Initialize(config ProviderConfig) error
}

// ProviderType defines the available stock data provider types
type ProviderType string

const (
	// TuShareProvider represents the TuShare API provider
	TuShareProvider ProviderType = "tushare"
	
	// TonghuashunProvider represents the Tonghuashun API provider
	TonghuashunProvider ProviderType = "tonghuashun"
	
	// DefaultProvider defines the default provider to use
	DefaultProvider = TuShareProvider
)

// ProviderConfig holds configuration for stock data providers
type ProviderConfig struct {
	// Type specifies which provider implementation to use
	Type ProviderType `json:"type"`
	
	// APIKey is the authentication key for the API
	APIKey string `json:"apiKey"`
	
	// APISecret is the authentication secret for providers that require it
	APISecret string `json:"apiSecret,omitempty"`
	
	// Endpoint is the base URL for API requests
	Endpoint string `json:"endpoint"`
	
	// Timeout specifies the maximum duration for API requests
	Timeout time.Duration `json:"timeout"`
	
	// RateLimit defines requests per minute limit
	RateLimit int `json:"rateLimit"`
	
	// RetryCount defines how many times to retry failed requests
	RetryCount int `json:"retryCount"`
	
	// RetryDelay defines delay between retries
	RetryDelay time.Duration `json:"retryDelay"`
}

// MinuteDataRequest contains parameters for fetching minute-level stock data
type MinuteDataRequest struct {
	// Code is the stock code with exchange suffix (e.g., 600000.SH)
	Code string `json:"code" validate:"required"`
	
	// Start is the beginning of the time range
	Start time.Time `json:"start" validate:"required"`
	
	// End is the end of the time range
	End time.Time `json:"end" validate:"required"`
	
	// Frequency defines the data frequency (1m, 5m, 15m, 30m, etc.)
	Frequency string `json:"frequency,omitempty"`
	
	// AdjustType defines the price adjustment type (none, qfq, hfq)
	// qfq: forward adjustment, hfq: backward adjustment
	AdjustType string `json:"adjustType,omitempty"`
}

// BatchDataRequest contains parameters for fetching data for multiple stocks
type BatchDataRequest struct {
	// Codes is a list of stock codes with exchange suffix
	Codes []string `json:"codes" validate:"required,min=1,max=100"`
	
	// Start is the beginning of the time range
	Start time.Time `json:"start" validate:"required"`
	
	// End is the end of the time range
	End time.Time `json:"end" validate:"required"`
	
	// Frequency defines the data frequency (1m, 5m, 15m, 30m, etc.)
	Frequency string `json:"frequency,omitempty"`
	
	// AdjustType defines the price adjustment type (none, qfq, hfq)
	AdjustType string `json:"adjustType,omitempty"`
}

// Error types for stock API operations
var (
	// ErrInvalidStockCode is returned when a stock code is not valid
	ErrInvalidStockCode = errors.New("invalid stock code format")
	
	// ErrProviderNotInitialized is returned when attempting to use a provider that hasn't been initialized
	ErrProviderNotInitialized = errors.New("stock data provider not initialized")
	
	// ErrAPIConnectionFailed is returned when the connection to the API fails
	ErrAPIConnectionFailed = errors.New("connection to stock API failed")
	
	// ErrAPIResponseInvalid is returned when the API response cannot be parsed
	ErrAPIResponseInvalid = errors.New("invalid API response format")
	
	// ErrAPIRateLimitExceeded is returned when the provider's rate limit is exceeded
	ErrAPIRateLimitExceeded = errors.New("API rate limit exceeded")
	
	// ErrAPIAuthenticationFailed is returned when authentication with the API fails
	ErrAPIAuthenticationFailed = errors.New("API authentication failed")
	
	// ErrNoDataFound is returned when no data is available for the requested parameters
	ErrNoDataFound = errors.New("no stock data found for the specified parameters")
)

// StockAPIError represents an error from the stock API with additional context
type StockAPIError struct {
	Err        error  // Original error
	Message    string // Error message
	StatusCode int    // HTTP status code if applicable
	Provider   string // Provider name
}

// Error satisfies the error interface
func (e *StockAPIError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("[%s] %s (status: %d): %v", e.Provider, e.Message, e.StatusCode, e.Err)
	}
	return fmt.Sprintf("[%s] %s: %v", e.Provider, e.Message, e.Err)
}

// Unwrap returns the original error
func (e *StockAPIError) Unwrap() error {
	return e.Err
}

// NewStockDataProvider creates a new stock data provider based on the specified type
func NewStockDataProvider(providerType ProviderType) (StockDataProvider, error) {
	switch providerType {
	case TuShareProvider:
		return &tuShareProvider{}, nil
	case TonghuashunProvider:
		return &tonghuashunProvider{}, nil
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", providerType)
	}
}

// Placeholder provider implementations for compilation
// These will be replaced with actual implementations in their respective files

type tuShareProvider struct {
	config ProviderConfig
}

func (p *tuShareProvider) FetchMinuteData(ctx context.Context, code string, start, end time.Time) ([]models.Stock, error) {
	// Implementation will be in tushare_provider.go
	return nil, errors.New("not implemented")
}

func (p *tuShareProvider) FetchLatestMinute(ctx context.Context, code string) (*models.Stock, error) {
	// Implementation will be in tushare_provider.go
	return nil, errors.New("not implemented")
}

func (p *tuShareProvider) FetchBatchMinuteData(ctx context.Context, codes []string, start, end time.Time) (map[string][]models.Stock, error) {
	// Implementation will be in tushare_provider.go
	return nil, errors.New("not implemented")
}

func (p *tuShareProvider) Name() string {
	return string(TuShareProvider)
}

func (p *tuShareProvider) Initialize(config ProviderConfig) error {
	// Implementation will be in tushare_provider.go
	p.config = config
	return nil
}

type tonghuashunProvider struct {
	config ProviderConfig
}

func (p *tonghuashunProvider) FetchMinuteData(ctx context.Context, code string, start, end time.Time) ([]models.Stock, error) {
	// Implementation will be in tonghuashun_provider.go
	return nil, errors.New("not implemented")
}

func (p *tonghuashunProvider) FetchLatestMinute(ctx context.Context, code string) (*models.Stock, error) {
	// Implementation will be in tonghuashun_provider.go
	return nil, errors.New("not implemented")
}

func (p *tonghuashunProvider) FetchBatchMinuteData(ctx context.Context, codes []string, start, end time.Time) (map[string][]models.Stock, error) {
	// Implementation will be in tonghuashun_provider.go
	return nil, errors.New("not implemented")
}

func (p *tonghuashunProvider) Name() string {
	return string(TonghuashunProvider)
}

func (p *tonghuashunProvider) Initialize(config ProviderConfig) error {
	// Implementation will be in tonghuashun_provider.go
	p.config = config
	return nil
}