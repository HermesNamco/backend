package stock

import (
	"backend/configs"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"backend/models"
	"golang.org/x/time/rate"
)

// TuShareProvider implements the StockDataProvider interface for TuShare API
type tuShareProvider struct {
	config      ProviderConfig
	apiURL      string
	token       string
	client      *http.Client
	limiter     *rate.Limiter
	initialized bool
}

// TuShareRequest represents the request format for TuShare API
type TuShareRequest struct {
	APIName string                 `json:"api_name"`
	Token   string                 `json:"token"`
	Params  map[string]interface{} `json:"params"`
	Fields  string                 `json:"fields,omitempty"`
}

// TuShareResponse represents the response format from TuShare API
type TuShareResponse struct {
	RequestID string `json:"request_id"`
	Code      int    `json:"code"`
	Msg       string `json:"msg"`
	Data      struct {
		Fields  []string        `json:"fields"`
		Items   [][]interface{} `json:"items"`
		HasMore bool            `json:"has_more"`
	} `json:"data"`
}

// Initialize sets up the TuShare provider with configuration
func (p *tuShareProvider) Initialize(config ProviderConfig) error {
	if config.APIKey == "" {
		return errors.New("TuShare API key (token) is required")
	}

	p.config = config
	p.token = config.APIKey

	// Set default endpoint if not provided
	if config.Endpoint == "" {
		p.apiURL = "https://api.tushare.pro"
	} else {
		p.apiURL = config.Endpoint
	}

	// Set up HTTP client with timeout
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second // Default timeout
	}
	p.client = &http.Client{Timeout: timeout}

	// Set up rate limiter to prevent API rate limit issues
	rateLimit := config.RateLimit
	if rateLimit <= 0 {
		rateLimit = 100 // Default to 100 requests per minute (TuShare's default limit)
	}
	p.limiter = rate.NewLimiter(rate.Limit(rateLimit)/60, 1) // Convert to requests per second

	p.initialized = true
	log.Printf("[TuShare] Initialized provider with endpoint: %s", p.apiURL)

	return nil
}

// Name returns the provider name
func (p *tuShareProvider) Name() string {
	return string(configs.TuShareProvider)
}

// FetchMinuteData retrieves minute-level data for a specific stock
func (p *tuShareProvider) FetchMinuteData(ctx context.Context, code string, start, end time.Time) ([]models.Stock, error) {
	if !p.initialized {
		return nil, ErrProviderNotInitialized
	}

	// Validate stock code format
	stock := &models.Stock{Code: code}
	if !stock.IsValidCode() {
		return nil, &StockAPIError{
			Err:      ErrInvalidStockCode,
			Message:  fmt.Sprintf("Invalid stock code format: %s", code),
			Provider: p.Name(),
		}
	}

	log.Printf("[TuShare] Fetching minute data for %s from %s to %s",
		code, start.Format("2006-01-02"), end.Format("2006-01-02"))

	// Prepare API request parameters
	params := map[string]interface{}{
		"ts_code":    code,
		"start_date": start.Format("20060102"),
		"end_date":   end.Format("20060102"),
		"freq":       "1min", // Default to 1-minute data
	}

	// Execute API request
	result, err := p.callAPI(ctx, "stk_mins", params)
	if err != nil {
		return nil, err
	}

	// Process the response data
	stocks, err := p.processMinuteData(result, code)
	if err != nil {
		return nil, &StockAPIError{
			Err:      err,
			Message:  "Failed to process minute data",
			Provider: p.Name(),
		}
	}

	log.Printf("[TuShare] Retrieved %d minute records for %s", len(stocks), code)
	return stocks, nil
}

// FetchLatestMinute retrieves the latest minute data for a specific stock
func (p *tuShareProvider) FetchLatestMinute(ctx context.Context, code string) (*models.Stock, error) {
	if !p.initialized {
		return nil, ErrProviderNotInitialized
	}

	// Validate stock code format
	stock := &models.Stock{Code: code}
	if !stock.IsValidCode() {
		return nil, &StockAPIError{
			Err:      ErrInvalidStockCode,
			Message:  fmt.Sprintf("Invalid stock code format: %s", code),
			Provider: p.Name(),
		}
	}

	log.Printf("[TuShare] Fetching latest minute data for %s", code)

	// For latest data, use today's date
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Prepare API request parameters
	params := map[string]interface{}{
		"ts_code":    code,
		"start_date": today.Format("20060102"),
		"end_date":   today.Format("20060102"),
		"freq":       "1min",
		"limit":      1, // Only get the latest record
	}

	// Execute API request
	result, err := p.callAPI(ctx, "stk_mins", params)
	if err != nil {
		return nil, err
	}

	// Process the response data
	stocks, err := p.processMinuteData(result, code)
	if err != nil {
		return nil, &StockAPIError{
			Err:      err,
			Message:  "Failed to process latest minute data",
			Provider: p.Name(),
		}
	}

	if len(stocks) == 0 {
		return nil, &StockAPIError{
			Err:      ErrNoDataFound,
			Message:  fmt.Sprintf("No latest minute data found for %s", code),
			Provider: p.Name(),
		}
	}

	// Return the latest record
	return &stocks[0], nil
}

// FetchBatchMinuteData retrieves minute-level data for multiple stocks
func (p *tuShareProvider) FetchBatchMinuteData(ctx context.Context, codes []string, start, end time.Time) (map[string][]models.Stock, error) {
	if !p.initialized {
		return nil, ErrProviderNotInitialized
	}

	log.Printf("[TuShare] Fetching batch minute data for %d stocks from %s to %s",
		len(codes), start.Format("2006-01-02"), end.Format("2006-01-02"))

	// TuShare API doesn't support batch requests for minute data
	// So we have to fetch data for each stock individually
	result := make(map[string][]models.Stock)

	for _, code := range codes {
		// Validate stock code format
		stock := &models.Stock{Code: code}
		if !stock.IsValidCode() {
			log.Printf("[TuShare] Skipping invalid stock code: %s", code)
			continue
		}

		stocks, err := p.FetchMinuteData(ctx, code, start, end)
		if err != nil {
			log.Printf("[TuShare] Error fetching data for %s: %v", code, err)
			continue
		}

		result[code] = stocks
	}

	if len(result) == 0 {
		return nil, &StockAPIError{
			Err:      ErrNoDataFound,
			Message:  "No stock data found for any of the provided codes",
			Provider: p.Name(),
		}
	}

	return result, nil
}

// callAPI makes a request to the TuShare API
func (p *tuShareProvider) callAPI(ctx context.Context, apiName string, params map[string]interface{}) (*TuShareResponse, error) {
	// Apply rate limiting
	if err := p.limiter.Wait(ctx); err != nil {
		return nil, &StockAPIError{
			Err:      err,
			Message:  "Rate limit wait error",
			Provider: p.Name(),
		}
	}

	// Prepare request
	req := TuShareRequest{
		APIName: apiName,
		Token:   p.token,
		Params:  params,
		Fields:  "", // Empty means all fields
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, &StockAPIError{
			Err:      err,
			Message:  "Failed to marshal API request",
			Provider: p.Name(),
		}
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, &StockAPIError{
			Err:      err,
			Message:  "Failed to create HTTP request",
			Provider: p.Name(),
		}
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Execute HTTP request with retry logic
	var resp *http.Response
	var retryCount = p.config.RetryCount
	if retryCount <= 0 {
		retryCount = 3 // Default retry count
	}

	retryDelay := p.config.RetryDelay
	if retryDelay == 0 {
		retryDelay = 1 * time.Second // Default retry delay
	}

	for i := 0; i < retryCount; i++ {
		resp, err = p.client.Do(httpReq)
		if err == nil {
			break
		}

		if i < retryCount-1 {
			log.Printf("[TuShare] API request failed, retrying (%d/%d): %v", i+1, retryCount, err)
			select {
			case <-time.After(retryDelay):
				// Wait before retry
			case <-ctx.Done():
				return nil, &StockAPIError{
					Err:      ctx.Err(),
					Message:  "Request cancelled",
					Provider: p.Name(),
				}
			}
			retryDelay *= 2 // Exponential backoff
		}
	}

	if err != nil {
		return nil, &StockAPIError{
			Err:      ErrAPIConnectionFailed,
			Message:  fmt.Sprintf("API connection failed after %d retries: %v", retryCount, err),
			Provider: p.Name(),
		}
	}
	defer resp.Body.Close()

	// Read and parse response
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, &StockAPIError{
			Err:        err,
			Message:    "Failed to read API response",
			Provider:   p.Name(),
			StatusCode: resp.StatusCode,
		}
	}

	var result TuShareResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, &StockAPIError{
			Err:        ErrAPIResponseInvalid,
			Message:    fmt.Sprintf("Failed to parse API response: %v", err),
			Provider:   p.Name(),
			StatusCode: resp.StatusCode,
		}
	}

	// Check API response code
	if result.Code != 0 {
		return nil, &StockAPIError{
			Err:        errors.New(result.Msg),
			Message:    fmt.Sprintf("API returned error code %d", result.Code),
			Provider:   p.Name(),
			StatusCode: resp.StatusCode,
		}
	}

	return &result, nil
}

// processMinuteData converts API response to Stock objects
func (p *tuShareProvider) processMinuteData(resp *TuShareResponse, code string) ([]models.Stock, error) {
	if resp == nil || len(resp.Data.Items) == 0 {
		return []models.Stock{}, nil
	}

	// Create a map to translate field index to field name
	fieldMap := make(map[int]string)
	for i, field := range resp.Data.Fields {
		fieldMap[i] = field
	}

	stocks := make([]models.Stock, 0, len(resp.Data.Items))

	for _, item := range resp.Data.Items {
		// Convert the item row to a map
		data := make(map[string]interface{})
		for i, val := range item {
			if i < len(resp.Data.Fields) {
				data[fieldMap[i]] = val
			}
		}

		// Add the code to the data if it's not included
		if _, ok := data["ts_code"]; !ok {
			data["ts_code"] = code
		}

		// Add name if available (TuShare might not include it in minute data)
		// We'll try to get name from stock basic info in a real implementation
		if _, ok := data["name"]; !ok {
			// For now, just use code as name placeholder
			parts := data["ts_code"].(string)
			data["name"] = parts
		}

		// Create and populate Stock object
		stock := models.NewStock()
		if err := stock.FromTuShareFormat(data); err != nil {
			log.Printf("[TuShare] Error converting data row: %v", err)
			continue
		}

		stocks = append(stocks, *stock)
	}

	return stocks, nil
}

// getStockInfo fetches basic stock information like name
// This would be implemented to complement minute data which might not include names
func (p *tuShareProvider) getStockInfo(ctx context.Context, code string) (map[string]interface{}, error) {
	params := map[string]interface{}{
		"ts_code": code,
	}

	result, err := p.callAPI(ctx, "stock_basic", params)
	if err != nil {
		return nil, err
	}

	if len(result.Data.Items) == 0 {
		return nil, fmt.Errorf("no stock info found for code %s", code)
	}

	// Convert to map
	data := make(map[string]interface{})
	for i, val := range result.Data.Items[0] {
		if i < len(result.Data.Fields) {
			data[result.Data.Fields[i]] = val
		}
	}

	return data, nil
}
