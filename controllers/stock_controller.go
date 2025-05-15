package controllers

import (
	"backend/middleware"
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"backend/configs"
	"backend/models"
	"backend/service/cache"
	"backend/service/stock"
	"github.com/gin-gonic/gin"
)

// StockController handles stock data-related HTTP requests
type StockController struct {
	// Stock data provider for getting live data
	provider stock.StockDataProvider

	// Stock data cache
	cache *cache.StockCache

	// Default time-to-live for cached entries
	cacheTTL time.Duration
}

// StockDataRequest represents the request format for stock data
type StockDataRequest struct {
	Code      string `form:"code" binding:"required"`
	StartDate string `form:"start_date"`
	EndDate   string `form:"end_date"`
	Format    string `form:"format"` // json or csv
}

// BatchStockDataRequest represents the request format for batch stock data
type BatchStockDataRequest struct {
	Codes     string `form:"codes" binding:"required"` // Comma-separated list of stock codes
	StartDate string `form:"start_date"`
	EndDate   string `form:"end_date"`
	Format    string `form:"format"` // json or csv
}

// NewStockController creates a new instance of StockController
func NewStockController() *StockController {
	// Get config from configs package
	config := configs.GetConfig()

	// Create and initialize the stock data provider
	providerType := stock.ProviderType(config.Stock.ProviderType)
	if providerType == "" {
		providerType = stock.DefaultProvider
	}

	provider, err := stock.NewStockDataProvider(providerType)
	if err != nil {
		// Fall back to default provider if specified one fails
		provider, _ = stock.NewStockDataProvider(stock.DefaultProvider)
	}

	// Initialize the provider with configuration
	providerConfig := stock.ProviderConfig{
		Type:       providerType,
		APIKey:     config.Stock.APIKey,
		APISecret:  config.Stock.APISecret,
		Endpoint:   config.Stock.Endpoint,
		Timeout:    time.Duration(config.Stock.TimeoutSeconds) * time.Second,
		RateLimit:  config.Stock.RateLimit,
		RetryCount: config.Stock.RetryCount,
		RetryDelay: time.Duration(config.Stock.RetryDelaySeconds) * time.Second,
	}
	provider.Initialize(providerConfig)

	// Create cache
	cacheOptions := cache.CacheOptions{
		Capacity:        config.Stock.CacheSize,
		DefaultTTL:      time.Duration(config.Stock.CacheTTLMinutes) * time.Minute,
		EvictionPolicy:  config.Stock.CacheEvictionPolicy,
		CleanupInterval: time.Duration(config.Stock.CacheCleanupMinutes) * time.Minute,
	}

	// Use default options if not configured
	if cacheOptions.Capacity <= 0 {
		defaultOptions := cache.DefaultCacheOptions()
		cacheOptions = defaultOptions
	}

	return &StockController{
		provider: provider,
		cache:    cache.NewStockCache(cacheOptions),
		cacheTTL: cacheOptions.DefaultTTL,
	}
}

// GetStockData handles GET requests to retrieve minute-level data for a specific stock
func (sc *StockController) GetStockData(c *gin.Context) {
	var req StockDataRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid request parameters",
			"error":   err.Error(),
		})
		return
	}

	// Parse date parameters
	start, end, err := sc.parseDateRange(req.StartDate, req.EndDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid date format",
			"error":   err.Error(),
		})
		return
	}

	// Check stock code format
	stock := &models.Stock{Code: req.Code}
	if !stock.IsValidCode() {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid stock code format",
			"error":   "Stock code must be in format XXXXXX.XX",
		})
		return
	}

	// Try to get data from cache first
	cachedData := sc.cache.GetRange(req.Code, start, end)

	// If not in cache or insufficient data, fetch from provider
	if len(cachedData) == 0 {
		stockData, err := sc.provider.FetchMinuteData(context.Background(), req.Code, start, end)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "Failed to fetch stock data",
				"error":   err.Error(),
			})
			return
		}

		// Store in cache for future requests
		sc.cache.SetBatch(stockData, sc.cacheTTL)

		// Use the fetched data
		cachedData = stockData
	}

	// Return data in requested format
	if strings.ToLower(req.Format) == "csv" {
		sc.respondWithCSV(c, cachedData)
	} else {
		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"data": gin.H{
				"code":       req.Code,
				"start_date": start.Format("2006-01-02"),
				"end_date":   end.Format("2006-01-02"),
				"records":    cachedData,
				"count":      len(cachedData),
			},
		})
	}
}

// GetLatestStockData handles GET requests to retrieve the latest minute data for a stock
func (sc *StockController) GetLatestStockData(c *gin.Context) {
	code := c.Param("code")

	// Check stock code format
	stock := &models.Stock{Code: code}
	if !stock.IsValidCode() {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid stock code format",
			"error":   "Stock code must be in format XXXXXX.XX",
		})
		return
	}

	// Try to get from cache first
	cachedStock, found := sc.cache.GetLatest(code)

	// If not in cache or data is stale (older than 5 minutes), fetch from provider
	if !found || time.Since(cachedStock.Timestamp) > 5*time.Minute {
		latestStock, err := sc.provider.FetchLatestMinute(context.Background(), code)
		if err != nil {
			// If API call fails but we have cached data, use it anyway
			if found {
				c.JSON(http.StatusOK, gin.H{
					"status": "success",
					"data":   cachedStock,
					"source": "cache",
				})
				return
			}

			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "Failed to fetch latest stock data",
				"error":   err.Error(),
			})
			return
		}

		// Cache the new data
		sc.cache.Set(latestStock, sc.cacheTTL)

		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"data":   latestStock,
			"source": "api",
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"data":   cachedStock,
			"source": "cache",
		})
	}
}

// GetBatchStockData handles GET requests to retrieve data for multiple stocks
func (sc *StockController) GetBatchStockData(c *gin.Context) {
	var req BatchStockDataRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid request parameters",
			"error":   err.Error(),
		})
		return
	}

	// Parse date parameters
	start, end, err := sc.parseDateRange(req.StartDate, req.EndDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid date format",
			"error":   err.Error(),
		})
		return
	}

	// Parse stock codes
	codes := strings.Split(req.Codes, ",")
	if len(codes) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "No stock codes provided",
		})
		return
	}

	// Validate each code
	validCodes := make([]string, 0, len(codes))
	for _, code := range codes {
		code = strings.TrimSpace(code)
		stock := &models.Stock{Code: code}
		if stock.IsValidCode() {
			validCodes = append(validCodes, code)
		}
	}

	if len(validCodes) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "No valid stock codes provided",
		})
		return
	}

	// Fetch batch data from provider
	// For simplicity, we don't check cache for batch requests in this implementation
	batchData, err := sc.provider.FetchBatchMinuteData(context.Background(), validCodes, start, end)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to fetch batch stock data",
			"error":   err.Error(),
		})
		return
	}

	// Store all data in cache
	for _, stocks := range batchData {
		sc.cache.SetBatch(stocks, sc.cacheTTL)
	}

	// Return data in requested format
	if strings.ToLower(req.Format) == "csv" {
		// Flatten all stocks into a single slice for CSV export
		allStocks := make([]models.Stock, 0)
		for _, stocks := range batchData {
			allStocks = append(allStocks, stocks...)
		}
		sc.respondWithCSV(c, allStocks)
	} else {
		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"data": gin.H{
				"codes":      validCodes,
				"start_date": start.Format("2006-01-02"),
				"end_date":   end.Format("2006-01-02"),
				"records":    batchData,
				"counts":     sc.countRecords(batchData),
			},
		})
	}
}

// GetCacheStats handles GET requests to retrieve cache statistics
func (sc *StockController) GetCacheStats(c *gin.Context) {
	stats := sc.cache.Stats()
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   stats,
	})
}

// ClearCache handles POST requests to clear the cache
func (sc *StockController) ClearCache(c *gin.Context) {
	sc.cache.Clear()
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Cache cleared successfully",
	})
}

// parseDateRange parses start and end date strings into time.Time objects
func (sc *StockController) parseDateRange(startStr, endStr string) (time.Time, time.Time, error) {
	// Default to today if no dates provided
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Default range: last 7 days to today
	start := today.AddDate(0, 0, -7)
	end := today

	// Parse start date if provided
	if startStr != "" {
		var err error
		start, err = time.Parse("2006-01-02", startStr)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid start date format: %v", err)
		}
	}

	// Parse end date if provided
	if endStr != "" {
		var err error
		end, err = time.Parse("2006-01-02", endStr)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid end date format: %v", err)
		}
	}

	// Ensure end date is not before start date
	if end.Before(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("end date cannot be before start date")
	}

	return start, end, nil
}

// respondWithCSV writes stock data as CSV to the response
func (sc *StockController) respondWithCSV(c *gin.Context, stocks []models.Stock) {
	// Set headers for CSV download
	filename := fmt.Sprintf("stock_data_%s.csv", time.Now().Format("20060102_150405"))
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "text/csv")

	// Create CSV writer
	writer := csv.NewWriter(c.Writer)

	// Write header
	header := []string{"Code", "Name", "Timestamp", "Open", "Close", "High", "Low", "Volume", "Amount", "Change", "Percent", "Exchange"}
	writer.Write(header)

	// Write data rows
	for _, stock := range stocks {
		row := []string{
			stock.Code,
			stock.Name,
			stock.Timestamp.Format("2006-01-02 15:04:05"),
			strconv.FormatFloat(stock.Open, 'f', 4, 64),
			strconv.FormatFloat(stock.Close, 'f', 4, 64),
			strconv.FormatFloat(stock.High, 'f', 4, 64),
			strconv.FormatFloat(stock.Low, 'f', 4, 64),
			strconv.FormatInt(stock.Volume, 10),
			strconv.FormatFloat(stock.Amount, 'f', 2, 64),
			strconv.FormatFloat(stock.Change, 'f', 4, 64),
			strconv.FormatFloat(stock.Percent, 'f', 4, 64),
			stock.Exchange,
		}
		writer.Write(row)
	}

	// Flush the writer to ensure all data is written
	writer.Flush()
}

// countRecords counts the number of records for each stock code in batch data
func (sc *StockController) countRecords(batchData map[string][]models.Stock) map[string]int {
	counts := make(map[string]int)
	for code, stocks := range batchData {
		counts[code] = len(stocks)
	}
	return counts
}

// RegisterRoutes registers the stock controller routes on the provided router group
func (sc *StockController) RegisterRoutes(router *gin.RouterGroup) {
	stocks := router.Group("/stocks")

	stocks.Use(middleware.RateLimitMiddleware(5, 10))

	// Single stock data endpoints
	stocks.GET("/data", sc.GetStockData)
	stocks.GET("/latest/:code", sc.GetLatestStockData)

	// Batch data endpoint
	stocks.GET("/batch", sc.GetBatchStockData)

	// Protected stock management endpoints
	stockAdmin := stocks.Group("/admin")
	stockAdmin.Use(middleware.JWTAuthMiddleware())
	stockAdmin.Use(middleware.RequireRole("admin", "analyst"))
	{
		stockAdmin.POST("/cache/clear", sc.ClearCache)
		stockAdmin.GET("/cache/stats", sc.GetCacheStats)
	}
}
