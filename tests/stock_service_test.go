package tests

import (
	"backend/configs"
	"backend/models"
	"backend/service/cache"
	"backend/service/scheduler"
	"backend/service/stock"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// mockStockDataProvider is a mock implementation of the StockDataProvider interface.
// It allows controlling responses for testing purposes.
type mockStockDataProvider struct {
	mu              sync.Mutex
	FetchMinuteFunc func(ctx context.Context, code string, start, end time.Time) ([]models.Stock, error)
	FetchLatestFunc func(ctx context.Context, code string) (*models.Stock, error)
	FetchBatchFunc  func(ctx context.Context, codes []string, start, end time.Time) (map[string][]models.Stock, error)
	InitializeFunc  func(config stock.ProviderConfig) error
	NameFunc        func() string
}

func (m *mockStockDataProvider) FetchMinuteData(ctx context.Context, code string, start, end time.Time) ([]models.Stock, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FetchMinuteFunc != nil {
		return m.FetchMinuteFunc(ctx, code, start, end)
	}
	return nil, fmt.Errorf("FetchMinuteFunc not implemented")
}

func (m *mockStockDataProvider) FetchLatestMinute(ctx context.Context, code string) (*models.Stock, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FetchLatestFunc != nil {
		return m.FetchLatestFunc(ctx, code)
	}
	return nil, fmt.Errorf("FetchLatestFunc not implemented")
}

func (m *mockStockDataProvider) FetchBatchMinuteData(ctx context.Context, codes []string, start, end time.Time) (map[string][]models.Stock, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.FetchBatchFunc != nil {
		return m.FetchBatchFunc(ctx, codes, start, end)
	}
	return nil, fmt.Errorf("FetchBatchFunc not implemented")
}

func (m *mockStockDataProvider) Initialize(config stock.ProviderConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.InitializeFunc != nil {
		return m.InitializeFunc(config)
	}
	return nil // Default: no error
}

func (m *mockStockDataProvider) Name() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.NameFunc != nil {
		return m.NameFunc()
	}
	return "mockProvider"
}

// Helper to setup config for tests
func setupTestConfig() {
	// Set minimal environment variables for config loading to pass
	os.Setenv("SMTP_USERNAME", "testuser")
	os.Setenv("SMTP_PASSWORD", "testpass")
	os.Setenv("STOCK_API_KEY", "testkey") // For providers requiring a key

	if err := configs.InitConfig(); err != nil {
		log.Fatalf("Failed to initialize test config: %v", err)
	}
}

func TestMain(m *testing.M) {
	setupTestConfig()
	// Run all tests
	code := m.Run()
	os.Exit(code)
}

// --- Model Tests ---
func TestStockModelValidation(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name      string
		stock     models.Stock
		expectError bool
	}{
		{
			name: "Valid Stock",
			stock: models.Stock{
				Code:      "600000.SH",
				Name:      "Ping An Bank",
				Timestamp: now,
				Open:      10.0,
				Close:     10.5,
				High:      10.8,
				Low:       9.9,
				Volume:    100000,
				Amount:    1000000.0,
			},
			expectError: false,
		},
		{
			name: "Missing Code",
			stock: models.Stock{Name: "No Code Inc", Timestamp: now, Open: 1, Close: 1, High: 1, Low: 1, Volume: 100, Amount: 1000},
			expectError: true,
		},
		{
			name: "Invalid Volume",
			stock: models.Stock{Code: "000001.SZ", Name: "Test", Timestamp: now, Open: 1, Close: 1, High: 1, Low: 1, Volume: -100, Amount: 1000},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.stock.Validate()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}

	t.Run("IsValidCode", func(t *testing.T) {
		assert.True(t, (&models.Stock{Code: "600000.SH"}).IsValidCode())
		assert.True(t, (&models.Stock{Code: "000001.SZ"}).IsValidCode())
		assert.True(t, (&models.Stock{Code: "830001.BJ"}).IsValidCode())
		assert.False(t, (&models.Stock{Code: "60000.SH"}).IsValidCode())    // Invalid length
		assert.False(t, (&models.Stock{Code: "600000.SS"}).IsValidCode())   // Invalid exchange
		assert.False(t, (&models.Stock{Code: "60000A.SH"}).IsValidCode())   // Non-numeric
		assert.False(t, (&models.Stock{Code: "600000SH"}).IsValidCode())    // No separator
		assert.False(t, (&models.Stock{Code: ""}).IsValidCode())           // Empty
	})

	t.Run("BeforeCreate", func(t *testing.T) {
		stock := models.Stock{Code: "300001.SZ", Name: "Test Stock"}
		err := stock.BeforeCreate()
		assert.NoError(t, err)
		assert.Equal(t, "SZ", stock.Exchange)
		assert.NotZero(t, stock.CreatedAt)
		assert.NotZero(t, stock.UpdatedAt)

		stockInvalid := models.Stock{Code: "300001SZ"}
		errInvalid := stockInvalid.BeforeCreate()
		assert.Error(t, errInvalid)
	})
}

func TestStockDataConversion(t *testing.T) {
	stockTime := time.Date(2023, 1, 1, 10, 30, 0, 0, time.UTC)
	stock := models.Stock{
		Code: "600000.SH", Name: "Test", Timestamp: stockTime,
		Open: 10, Close: 11, High: 12, Low: 9, Volume: 1000, Amount: 10500,
	}

	t.Run("ToDataPoint", func(t *testing.T) {
		dp := stock.ToDataPoint()
		assert.Equal(t, stockTime.Unix(), dp.Time)
		assert.Equal(t, 10.0, dp.Open)
		assert.Equal(t, 11.0, dp.Close)
	})

	t.Run("FromDataPoint", func(t *testing.T) {
		dp := models.StockDataPoint{Time: stockTime.Unix(), Open: 1, Close: 2, High: 3, Low: 0.5, Volume: 200, Amount: 300}
		newStock := models.NewStock()
		err := newStock.FromDataPoint("000001.SZ", "New Test", dp)
		assert.NoError(t, err)
		assert.Equal(t, "000001.SZ", newStock.Code)
		assert.Equal(t, "New Test", newStock.Name)
		assert.Equal(t, stockTime.Unix(), newStock.Timestamp.Unix())
		assert.Equal(t, "SZ", newStock.Exchange)
	})

	t.Run("FromTuShareFormat", func(t *testing.T) {
		tuShareData := map[string]interface{}{
			"ts_code":    "600000.SH",
			"name":       "PingAn",
			"trade_time": "2023-01-01 10:30:00",
			"open":       10.1,
			"close":      10.2,
			"high":       10.3,
			"low":        10.0,
			"vol":        float64(2000),
			"amount":     float64(20200),
		}
		parsedStock := models.NewStock()
		err := parsedStock.FromTuShareFormat(tuShareData)
		assert.NoError(t, err)
		assert.Equal(t, "600000.SH", parsedStock.Code)
		assert.Equal(t, "PingAn", parsedStock.Name)
		assert.Equal(t, stockTime.Unix(), parsedStock.Timestamp.Unix())
		assert.Equal(t, "SH", parsedStock.Exchange)
		assert.Equal(t, 10.2, parsedStock.Close)
	})

	t.Run("FromTonghuashunFormat", func(t *testing.T) {
		thsData := map[string]interface{}{
			"code":   "000002.SZ", // Assume already standard format for test simplicity
			"name":   "Vanke",
			"time":   "2023-01-01 10:30:00",
			"open":   20.1,
			"price":  20.2, // close
			"high":   20.3,
			"low":    20.0,
			"volume": float64(3000),
			"amount": float64(60300),
		}
		parsedStock := models.NewStock()
		err := parsedStock.FromTonghuashunFormat(thsData)
		assert.NoError(t, err)
		assert.Equal(t, "000002.SZ", parsedStock.Code)
		assert.Equal(t, "Vanke", parsedStock.Name)
		assert.Equal(t, stockTime.Unix(), parsedStock.Timestamp.Unix())
		assert.Equal(t, "SZ", parsedStock.Exchange)
		assert.Equal(t, 20.2, parsedStock.Close)
	})
}

// --- Stock Cache Tests ---
func TestStockCache_SetGet(t *testing.T) {
	cacheOpts := cache.DefaultCacheOptions()
	stockCache := cache.NewStockCache(cacheOpts)
	stockTime := time.Now().Truncate(time.Minute)

	stock1 := &models.Stock{Code: "600000.SH", Name: "Test1", Timestamp: stockTime, Close: 10.0}
	stockCache.Set(stock1, 5*time.Minute)

	retrievedStock, found := stockCache.Get("600000.SH", stockTime)
	assert.True(t, found)
	assert.NotNil(t, retrievedStock)
	assert.Equal(t, stock1.Close, retrievedStock.Close)

	_, found = stockCache.Get("600000.SH", stockTime.Add(time.Minute)) // Different timestamp
	assert.False(t, found)

	_, found = stockCache.Get("000001.SZ", stockTime) // Different code
	assert.False(t, found)
}

func TestStockCache_GetLatest(t *testing.T) {
	stockCache := cache.NewStockCache(cache.DefaultCacheOptions())
	time1 := time.Now().Truncate(time.Minute).Add(-2 * time.Minute)
	time2 := time.Now().Truncate(time.Minute).Add(-1 * time.Minute)
	time3 := time.Now().Truncate(time.Minute)

	stock1 := &models.Stock{Code: "600000.SH", Timestamp: time1, Close: 1.0}
	stock2 := &models.Stock{Code: "600000.SH", Timestamp: time2, Close: 2.0} // latest for a while
	stock3 := &models.Stock{Code: "000001.SZ", Timestamp: time3, Close: 3.0}
	stock4 := &models.Stock{Code: "600000.SH", Timestamp: time3, Close: 4.0} // actual latest for 600000.SH

	stockCache.Set(stock1, time.Hour)
	stockCache.Set(stock2, time.Hour)
	stockCache.Set(stock3, time.Hour)
	stockCache.Set(stock4, time.Hour)

	latest, found := stockCache.GetLatest("600000.SH")
	assert.True(t, found)
	assert.Equal(t, 4.0, latest.Close)
	assert.Equal(t, time3, latest.Timestamp)

	latestSZ, foundSZ := stockCache.GetLatest("000001.SZ")
	assert.True(t, foundSZ)
	assert.Equal(t, 3.0, latestSZ.Close)

	_, notFound := stockCache.GetLatest("999999.SZ")
	assert.False(t, notFound)
}

func TestStockCache_TTL(t *testing.T) {
	cacheOpts := cache.DefaultCacheOptions()
	cacheOpts.DefaultTTL = 100 * time.Millisecond
	stockCache := cache.NewStockCache(cacheOpts)
	stockTime := time.Now().Truncate(time.Minute)

	stock1 := &models.Stock{Code: "600000.SH", Timestamp: stockTime, Close: 10.0}
	stockCache.Set(stock1, 50*time.Millisecond) // Specific shorter TTL

	_, found := stockCache.Get("600000.SH", stockTime)
	assert.True(t, found, "Should be found immediately after set")

	time.Sleep(60 * time.Millisecond)
	_, found = stockCache.Get("600000.SH", stockTime)
	assert.False(t, found, "Should be expired after 60ms")

	// Test with default TTL
	stock2 := &models.Stock{Code: "000001.SZ", Timestamp: stockTime, Close: 20.0}
	stockCache.Set(stock2, 0) // Use default TTL
	time.Sleep(110 * time.Millisecond)
	_, found = stockCache.Get("000001.SZ", stockTime)
	assert.False(t, found, "Should be expired after 110ms with default TTL")
}

func TestStockCache_EvictionLRU(t *testing.T) {
	cacheOpts := cache.DefaultCacheOptions()
	cacheOpts.Capacity = 2
	cacheOpts.EvictionPolicy = "lru"
	stockCache := cache.NewStockCache(cacheOpts)

	tS1 := time.Now().Add(-3 * time.Minute)
	tS2 := time.Now().Add(-2 * time.Minute)
	tS3 := time.Now().Add(-1 * time.Minute)

	stock1 := &models.Stock{Code: "001.SZ", Timestamp: tS1, Close: 1.0}
	stock2 := &models.Stock{Code: "002.SZ", Timestamp: tS2, Close: 2.0}
	stock3 := &models.Stock{Code: "003.SZ", Timestamp: tS3, Close: 3.0}

	stockCache.Set(stock1, time.Hour) // s1 is oldest
	stockCache.Set(stock2, time.Hour) // s2 is newer

	// Access stock1 to make it most recently used
	_, _ = stockCache.Get("001.SZ", tS1)

	stockCache.Set(stock3, time.Hour) // This should evict stock2 (least recently used)

	_, found1 := stockCache.Get("001.SZ", tS1)
	assert.True(t, found1, "Stock1 should still be in cache")

	_, found2 := stockCache.Get("002.SZ", tS2)
	assert.False(t, found2, "Stock2 should have been evicted")

	_, found3 := stockCache.Get("003.SZ", tS3)
	assert.True(t, found3, "Stock3 should be in cache")
}

func TestStockCache_Cleanup(t *testing.T) {
	cacheOpts := cache.DefaultCacheOptions()
	cacheOpts.DefaultTTL = 50 * time.Millisecond
	cacheOpts.CleanupInterval = 0 // Manual cleanup for test
	stockCache := cache.NewStockCache(cacheOpts)

	st1 := time.Now()
	st2 := time.Now().Add(1 * time.Minute) // Not expired

	stockCache.Set(&models.Stock{Code: "EXP.SH", Timestamp: st1, Close: 1}, 0) // Will use default TTL
	stockCache.Set(&models.Stock{Code: "NEXP.SH", Timestamp: st2, Close: 2}, time.Hour)

	assert.Equal(t, 2, stockCache.Size())
	time.Sleep(60 * time.Millisecond)

	removed := stockCache.Cleanup()
	assert.Equal(t, 1, removed, "One item should be removed by cleanup")
	assert.Equal(t, 1, stockCache.Size(), "Cache size should be 1 after cleanup")

	_, foundExp := stockCache.Get("EXP.SH", st1)
	assert.False(t, foundExp)
	_, foundNExp := stockCache.Get("NEXP.SH", st2)
	assert.True(t, foundNExp)
}

// --- Stock Provider Tests (with Mock HTTP Server) ---

// Helper to create a mock HTTP server for provider tests
func newMockServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}

func TestTuShareProvider_FetchLatestMinute_Success(t *testing.T) {
	server := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/", r.URL.Path) // TuShare uses a single endpoint
		var req stock.TuShareRequest
		body, _ := json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, "stk_mins", req.APIName)
		assert.Equal(t, "testtoken", req.Token)
		assert.Equal(t, "600000.SH", req.Params["ts_code"])

		resp := stock.TuShareResponse{
			RequestID: "testreqid",
			Code:      0,
			Msg:       "success",
			Data: struct {
				Fields  []string        `json:"fields"`
				Items   [][]interface{} `json:"items"`
				HasMore bool            `json:"has_more"`
			}{
				Fields: []string{"ts_code", "trade_time", "name", "open", "close", "high", "low", "vol", "amount"},
				Items: [][]interface{}{
					{"600000.SH", time.Now().Format("2006-01-02 15:04:05"), "TestStock", 10.0, 10.5, 11.0, 9.5, 1000.0, 10500.0},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	provider := &stock.TuShareProvider{}
	err := provider.Initialize(stock.ProviderConfig{
		Type:     stock.TuShareProvider,
		APIKey:   "testtoken",
		Endpoint: server.URL,
		RateLimit: 200, // High to avoid interference
	})
	assert.NoError(t, err)

	data, err := provider.FetchLatestMinute(context.Background(), "600000.SH")
	assert.NoError(t, err)
	assert.NotNil(t, data)
	assert.Equal(t, "600000.SH", data.Code)
	assert.Equal(t, 10.5, data.Close)
}

func TestTuShareProvider_ErrorHandling(t *testing.T) {
	server := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Simulate an API error
		resp := stock.TuShareResponse{
			RequestID: "errreq",
			Code:      5000, // TuShare error code
			Msg:       "Invalid token or permission denied",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK) // TuShare might return 200 OK with error in body
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	provider := &stock.TuShareProvider{}
	err := provider.Initialize(stock.ProviderConfig{
		Type:     stock.TuShareProvider,
		APIKey:   "invalidtoken",
		Endpoint: server.URL,
		RetryCount: 0, // No retries for this test
	})
	assert.NoError(t, err)

	_, err = provider.FetchLatestMinute(context.Background(), "600000.SH")
	assert.Error(t, err)
	stockErr, ok := err.(*stock.StockAPIError)
	assert.True(t, ok, "Error should be of type StockAPIError")
	assert.Contains(t, stockErr.Error(), "API returned error code 5000")

	// Test connection error
	server.Close() // Close server to simulate connection error
	_, err = provider.FetchLatestMinute(context.Background(), "000001.SZ")
	assert.Error(t, err)
	stockErr, ok = err.(*stock.StockAPIError)
	assert.True(t, ok)
	assert.ErrorIs(t, stockErr.Unwrap(), stock.ErrAPIConnectionFailed)
}

// --- Stock Scheduler Tests ---
func TestStockScheduler_BasicRun(t *testing.T) {
	stockTime := time.Now().Truncate(time.Minute)
	mockProvider := &mockStockDataProvider{}
	mockProvider.FetchLatestFunc = func(ctx context.Context, code string) (*models.Stock, error) {
		return &models.Stock{Code: code, Name: "Mocked " + code, Timestamp: stockTime, Close: 10.0}, nil
	}

	cacheOpts := cache.DefaultCacheOptions()
	stockCache := cache.NewStockCache(cacheOpts)

	schedulerConfig := scheduler.SchedulerConfig{
		StockCodes:    []string{"600000.SH", "000001.SZ"},
		FetchInterval: 0, // Run once for testing
		InitialDelay:  10 * time.Millisecond,
		CacheTTL:      time.Hour,
		WorkerPoolSize: 1,
	}

	sched, err := scheduler.NewStockScheduler(mockProvider, stockCache, schedulerConfig)
	assert.NoError(t, err)

	sched.Start()
	time.Sleep(100 * time.Millisecond) // Allow scheduler to run
	sched.Stop()

	status := sched.GetStatus()
	assert.False(t, status.IsRunning)
	assert.EqualValues(t, 1, status.TotalRuns)
	assert.Empty(t, status.LastRunError)

	history := sched.GetHistory()
	assert.Len(t, history, 1)
	assert.True(t, history[0].Success)
	assert.Equal(t, 2, history[0].StocksFetched)

	// Verify cache
	stockDataSH, foundSH := stockCache.GetLatest("600000.SH")
	assert.True(t, foundSH)
	assert.NotNil(t, stockDataSH)
	assert.Equal(t, 10.0, stockDataSH.Close)

	stockDataSZ, foundSZ := stockCache.GetLatest("000001.SZ")
	assert.True(t, foundSZ)
	assert.NotNil(t, stockDataSZ)
}

func TestStockScheduler_StopDuringRun(t *testing.T) {
	var fetchCalls int32
	mockProvider := &mockStockDataProvider{}
	mockProvider.FetchLatestFunc = func(ctx context.Context, code string) (*models.Stock, error) {
		// Simulate a long fetch for the first stock
		if code == "600000.SH" {
			time.Sleep(200 * time.Millisecond)
		}
		// Check if context was cancelled
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		fetchCalls++
		return &models.Stock{Code: code, Timestamp: time.Now(), Close: 1.0}, nil
	}

	stockCache := cache.NewStockCache(cache.DefaultCacheOptions())
	schedulerConfig := scheduler.SchedulerConfig{
		StockCodes:     []string{"600000.SH", "000001.SZ", "300001.SZ"},
		FetchInterval:  time.Hour, // Long interval to ensure only one triggered run
		InitialDelay:   0, // Start immediately
		CacheTTL:       time.Hour,
		WorkerPoolSize: 1, // Sequential processing to test cancellation propagation
	}

	sched, _ := scheduler.NewStockScheduler(mockProvider, stockCache, schedulerConfig)
	sched.Start()

	time.Sleep(50 * time.Millisecond) // Let the first fetch start
	sched.Stop()                    // Stop the scheduler while it's processing

	status := sched.GetStatus()
	assert.False(t, status.IsRunning)

	history := sched.GetHistory()
	// Depending on precise timing, either 0 or 1 stock might be fully fetched.
	// The key is that not all should be fetched.
	// If worker pool is > 1, this assertion needs adjustment.
	assert.True(t, history[0].StocksFetched < len(schedulerConfig.StockCodes), "Not all stocks should be fetched if stopped early")
	assert.False(t, history[0].Success, "Run should be marked as not fully successful if stopped")
}

// --- Benchmarks ---

func BenchmarkStockCache_Set(b *testing.B) {
	stockCache := cache.NewStockCache(cache.DefaultCacheOptions())
	stockTime := time.Now()
	stock := &models.Stock{Code: "600000.SH", Name: "TestBench", Timestamp: stockTime, Close: 10.0}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Modify timestamp slightly to ensure unique entries if needed for benchmark logic
		stock.Timestamp = stockTime.Add(time.Duration(i) * time.Nanosecond)
		stockCache.Set(stock, time.Hour)
	}
}

func BenchmarkStockCache_Get(b *testing.B) {
	stockCache := cache.NewStockCache(cache.DefaultCacheOptions())
	stockTime := time.Now()
	stock := &models.Stock{Code: "600000.SH", Name: "TestBench", Timestamp: stockTime, Close: 10.0}
	stockCache.Set(stock, time.Hour)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stockCache.Get("600000.SH", stockTime)
	}
}

func BenchmarkStockProvider_FetchLatestMinute_Mocked(b *testing.B) {
	mockProvider := &mockStockDataProvider{}
	mockProvider.FetchLatestFunc = func(ctx context.Context, code string) (*models.Stock, error) {
		return &models.Stock{Code: code, Name: "Benchmark", Timestamp: time.Now(), Close: 100.0}, nil
	}
	mockProvider.InitializeFunc = func(stock.ProviderConfig) error { return nil }

	err := mockProvider.Initialize(stock.ProviderConfig{})
	if err != nil {
		b.Fatalf("Failed to initialize mock provider: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mockProvider.FetchLatestMinute(context.Background(), "000001.SZ")
	}
}
