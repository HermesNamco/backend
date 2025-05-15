package stock

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"backend/models"
	"golang.org/x/time/rate"
)

// tonghuashunProvider implements the StockDataProvider interface for Tonghuashun API
type tonghuashunProvider struct {
	config      ProviderConfig
	apiURL      string
	apiKey      string
	apiSecret   string
	client      *http.Client
	limiter     *rate.Limiter
	accessToken string
	tokenExpiry time.Time
	initialized bool
}

// TonghuashunRequest represents the request format for Tonghuashun API
type TonghuashunRequest struct {
	Method      string                 `json:"method"`
	Version     string                 `json:"version"`
	AccessToken string                 `json:"access_token,omitempty"`
	Params      map[string]interface{} `json:"params"`
}

// TonghuashunResponse represents the response format from Tonghuashun API
type TonghuashunResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
	Error   string          `json:"error"`
}

// TonghuashunAuthResponse represents the authentication response
type TonghuashunAuthResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"` // seconds until token expires
}

// TonghuashunStockData represents stock data in Tonghuashun format
type TonghuashunStockData struct {
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	Time      string    `json:"time"`
	Open      float64   `json:"open"`
	Price     float64   `json:"price"` // current price (close)
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Volume    int64     `json:"volume"`
	Amount    float64   `json:"amount"`
	Turnover  float64   `json:"turnover,omitempty"`
	PERatio   float64   `json:"pe_ratio,omitempty"`
	MarketCap float64   `json:"market_cap,omitempty"`
}

// Initialize sets up the Tonghuashun provider with configuration
func (p *tonghuashunProvider) Initialize(config ProviderConfig) error {
	if config.APIKey == "" {
		return errors.New("Tonghuashun API key is required")
	}

	if config.APISecret == "" {
		return errors.New("Tonghuashun API secret is required")
	}

	p.config = config
	p.apiKey = config.APIKey
	p.apiSecret = config.APISecret
	
	// Set default endpoint if not provided
	if config.Endpoint == "" {
		p.apiURL = "https://api.10jqka.com.cn"
	} else {
		p.apiURL = config.Endpoint
	}
	
	// Set up HTTP client with timeout
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second // Default timeout
	}
	p.client = &http.Client{Timeout: timeout}
	
	// Set up rate limiter
	rateLimit := config.RateLimit
	if rateLimit <= 0 {
		rateLimit = 60 // Default to 60 requests per minute
	}
	p.limiter = rate.NewLimiter(rate.Limit(rateLimit)/60, 1) // Convert to requests per second
	
	p.initialized = true
	log.Printf("[Tonghuashun] Initialized provider with endpoint: %s", p.apiURL)
	
	return nil
}

// Name returns the provider name
func (p *tonghuashunProvider) Name() string {
	return string(TonghuashunProvider)
}

// FetchMinuteData retrieves minute-level data for a specific stock
func (p *tonghuashunProvider) FetchMinuteData(ctx context.Context, code string, start, end time.Time) ([]models.Stock, error) {
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
	
	log.Printf("[Tonghuashun] Fetching minute data for %s from %s to %s", 
		code, start.Format("2006-01-02"), end.Format("2006-01-02"))
	
	// Ensure we have a valid access token
	if err := p.ensureAuthenticated(ctx); err != nil {
		return nil, err
	}
	
	// Format code for Tonghuashun API (remove exchange suffix and add exchange prefix)
	formattedCode := p.formatStockCode(code)
	
	// Prepare API request parameters
	params := map[string]interface{}{
		"code":       formattedCode,
		"begin_time": start.Format("20060102150405"),
		"end_time":   end.Format("20060102150405"),
		"period":     "1", // 1-minute data
	}
	
	// Execute API request
	response, err := p.callAPI(ctx, "stock.minute.data", "1.0", params)
	if err != nil {
		return nil, err
	}
	
	// Parse the response data
	var stockDataList []TonghuashunStockData
	if err := json.Unmarshal(response.Data, &stockDataList); err != nil {
		return nil, &StockAPIError{
			Err:      ErrAPIResponseInvalid,
			Message:  fmt.Sprintf("Failed to parse stock data: %v", err),
			Provider: p.Name(),
		}
	}
	
	// Convert to Stock objects
	stocks := make([]models.Stock, 0, len(stockDataList))
	for _, data := range stockDataList {
		// Format the code back to standard format (with exchange suffix)
		data.Code = p.reverseFormatStockCode(data.Code)
		
		stock := models.NewStock()
		stockData := map[string]interface{}{
			"code":    data.Code,
			"name":    data.Name,
			"time":    data.Time,
			"open":    data.Open,
			"price":   data.Price,
			"high":    data.High,
			"low":     data.Low,
			"volume":  float64(data.Volume), // Convert to float64 for FromTonghuashunFormat
			"amount":  data.Amount,
		}
		
		if err := stock.FromTonghuashunFormat(stockData); err != nil {
			log.Printf("[Tonghuashun] Error converting data: %v", err)
			continue
		}
		
		stocks = append(stocks, *stock)
	}
	
	log.Printf("[Tonghuashun] Retrieved %d minute records for %s", len(stocks), code)
	return stocks, nil
}

// FetchLatestMinute retrieves the latest minute data for a specific stock
func (p *tonghuashunProvider) FetchLatestMinute(ctx context.Context, code string) (*models.Stock, error) {
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
	
	log.Printf("[Tonghuashun] Fetching latest minute data for %s", code)
	
	// Ensure we have a valid access token
	if err := p.ensureAuthenticated(ctx); err != nil {
		return nil, err
	}
	
	// Format code for Tonghuashun API
	formattedCode := p.formatStockCode(code)
	
	// Prepare API request parameters
	params := map[string]interface{}{
		"code": formattedCode,
	}
	
	// Execute API request
	response, err := p.callAPI(ctx, "stock.quote.minute", "1.0", params)
	if err != nil {
		return nil, err
	}
	
	// Parse the response data
	var stockData TonghuashunStockData
	if err := json.Unmarshal(response.Data, &stockData); err != nil {
		return nil, &StockAPIError{
			Err:      ErrAPIResponseInvalid,
			Message:  fmt.Sprintf("Failed to parse stock data: %v", err),
			Provider: p.Name(),
		}
	}
	
	// Format the code back to standard format
	stockData.Code = p.reverseFormatStockCode(stockData.Code)
	
	// Convert to Stock object
	stock = models.NewStock()
	data := map[string]interface{}{
		"code":    stockData.Code,
		"name":    stockData.Name,
		"time":    stockData.Time,
		"open":    stockData.Open,
		"price":   stockData.Price,
		"high":    stockData.High,
		"low":     stockData.Low,
		"volume":  float64(stockData.Volume),
		"amount":  stockData.Amount,
	}
	
	if err := stock.FromTonghuashunFormat(data); err != nil {
		return nil, &StockAPIError{
			Err:      err,
			Message:  "Failed to convert stock data",
			Provider: p.Name(),
		}
	}
	
	return stock, nil
}

// FetchBatchMinuteData retrieves minute-level data for multiple stocks
func (p *tonghuashunProvider) FetchBatchMinuteData(ctx context.Context, codes []string, start, end time.Time) (map[string][]models.Stock, error) {
	if !p.initialized {
		return nil, ErrProviderNotInitialized
	}
	
	log.Printf("[Tonghuashun] Fetching batch minute data for %d stocks from %s to %s", 
		len(codes), start.Format("2006-01-02"), end.Format("2006-01-02"))
	
	// Ensure we have a valid access token
	if err := p.ensureAuthenticated(ctx); err != nil {
		return nil, err
	}
	
	// Format codes for Tonghuashun API
	formattedCodes := make([]string, 0, len(codes))
	for _, code := range codes {
		// Validate stock code format
		stock := &models.Stock{Code: code}
		if !stock.IsValidCode() {
			log.Printf("[Tonghuashun] Skipping invalid stock code: %s", code)
			continue
		}
		
		formattedCodes = append(formattedCodes, p.formatStockCode(code))
	}
	
	if len(formattedCodes) == 0 {
		return nil, &StockAPIError{
			Err:      ErrInvalidStockCode,
			Message:  "No valid stock codes provided",
			Provider: p.Name(),
		}
	}
	
	// Prepare API request parameters
	params := map[string]interface{}{
		"codes":      strings.Join(formattedCodes, ","),
		"begin_time": start.Format("20060102150405"),
		"end_time":   end.Format("20060102150405"),
		"period":     "1", // 1-minute data
	}
	
	// Execute API request
	response, err := p.callAPI(ctx, "stock.minute.batch", "1.0", params)
	if err != nil {
		return nil, err
	}
	
	// Parse the response data
	var batchData map[string][]TonghuashunStockData
	if err := json.Unmarshal(response.Data, &batchData); err != nil {
		return nil, &StockAPIError{
			Err:      ErrAPIResponseInvalid,
			Message:  fmt.Sprintf("Failed to parse batch data: %v", err),
			Provider: p.Name(),
		}
	}
	
	// Convert to map of Stock arrays
	result := make(map[string][]models.Stock)
	
	for code, stockDataList := range batchData {
		// Convert formatted code back to standard format
		standardCode := p.reverseFormatStockCode(code)
		
		stocks := make([]models.Stock, 0, len(stockDataList))
		for _, data := range stockDataList {
			stock := models.NewStock()
			stockData := map[string]interface{}{
				"code":    standardCode,
				"name":    data.Name,
				"time":    data.Time,
				"open":    data.Open,
				"price":   data.Price,
				"high":    data.High,
				"low":     data.Low,
				"volume":  float64(data.Volume),
				"amount":  data.Amount,
			}
			
			if err := stock.FromTonghuashunFormat(stockData); err != nil {
				log.Printf("[Tonghuashun] Error converting data for %s: %v", standardCode, err)
				continue
			}
			
			stocks = append(stocks, *stock)
		}
		
		if len(stocks) > 0 {
			result[standardCode] = stocks
		}
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

// ensureAuthenticated makes sure we have a valid access token
func (p *tonghuashunProvider) ensureAuthenticated(ctx context.Context) error {
	// Check if we have a valid token
	if p.accessToken != "" && time.Now().Before(p.tokenExpiry) {
		return nil
	}
	
	log.Printf("[Tonghuashun] Authenticating with API")
	
	// Prepare authentication request
	params := map[string]interface{}{
		"api_key":    p.apiKey,
		"api_secret": p.apiSecret,
	}
	
	// Call the auth API endpoint
	response, err := p.callAPI(ctx, "auth.token", "1.0", params)
	if err != nil {
		return &StockAPIError{
			Err:      ErrAPIAuthenticationFailed,
			Message:  fmt.Sprintf("Authentication failed: %v", err),
			Provider: p.Name(),
		}
	}
	
	// Parse authentication response
	var authResponse TonghuashunAuthResponse
	if err := json.Unmarshal(response.Data, &authResponse); err != nil {
		return &StockAPIError{
			Err:      ErrAPIResponseInvalid,
			Message:  fmt.Sprintf("Failed to parse authentication response: %v", err),
			Provider: p.Name(),
		}
	}
	
	if authResponse.AccessToken == "" {
		return &StockAPIError{
			Err:      ErrAPIAuthenticationFailed,
			Message:  "No access token in response",
			Provider: p.Name(),
		}
	}
	
	// Store the token and its expiry time
	p.accessToken = authResponse.AccessToken
	p.tokenExpiry = time.Now().Add(time.Duration(authResponse.ExpiresIn) * time.Second)
	
	log.Printf("[Tonghuashun] Successfully authenticated, token valid until %s", 
		p.tokenExpiry.Format("2006-01-02 15:04:05"))
	
	return nil
}

// callAPI makes a request to the Tonghuashun API
func (p *tonghuashunProvider) callAPI(ctx context.Context, method, version string, params map[string]interface{}) (*TonghuashunResponse, error) {
	// Apply rate limiting
	if err := p.limiter.Wait(ctx); err != nil {
		return nil, &StockAPIError{
			Err:      err,
			Message:  "Rate limit wait error",
			Provider: p.Name(),
		}
	}
	
	// Prepare request
	req := TonghuashunRequest{
		Method:  method,
		Version: version,
		Params:  params,
	}
	
	// Add access token for authenticated requests
	// Don't add token for auth.token request
	if method != "auth.token" && p.accessToken != "" {
		req.AccessToken = p.accessToken
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
	endpoint := fmt.Sprintf("%s/openapi/%s/%s", p.apiURL, method, version)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(jsonData))
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
			log.Printf("[Tonghuashun] API request failed, retrying (%d/%d): %v", i+1, retryCount, err)
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
			Err:      err,
			Message:  "Failed to read API response",
			Provider: p.Name(),
			StatusCode: resp.StatusCode,
		}
	}
	
	var result TonghuashunResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, &StockAPIError{
			Err:      ErrAPIResponseInvalid,
			Message:  fmt.Sprintf("Failed to parse API response: %v", err),
			Provider: p.Name(),
			StatusCode: resp.StatusCode,
		}
	}
	
	// Check for API errors
	if result.Code != 0 {
		return nil, &StockAPIError{
			Err:      errors.New(result.Error),
			Message:  fmt.Sprintf("API returned error code %d: %s", result.Code, result.Message),
			Provider: p.Name(),
			StatusCode: resp.StatusCode,
		}
	}
	
	return &result, nil
}

// formatStockCode converts standard code format (e.g., "600000.SH") to Tonghuashun format (e.g., "sh600000")
func (p *tonghuashunProvider) formatStockCode(code string) string {
	parts := strings.Split(code, ".")
	if len(parts) != 2 {
		return code // Return as-is if not in expected format
	}
	
	stockCode := parts[0]
	exchange := strings.ToLower(parts[1])
	
	return exchange + stockCode
}

// reverseFormatStockCode converts Tonghuashun format (e.g., "sh600000") back to standard format (e.g., "600000.SH")
func (p *tonghuashunProvider) reverseFormatStockCode(code string) string {
	if len(code) < 8 {
		return code // Return as-is if too short
	}
	
	exchange := strings.ToUpper(code[0:2])
	stockCode := code[2:]
	
	return stockCode + "." + exchange
}