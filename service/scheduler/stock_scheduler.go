package scheduler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"backend/models"
	"backend/service/cache"
	"backend/service/stock"
	// "backend/configs" // Assuming SchedulerConfig might come from here eventually
)

const (
	defaultMaxHistoryLen = 100
	defaultWorkerPoolSize = 5
	defaultRetryAttempts = 3
	defaultRetryDelay = 5 * time.Second
)

// SchedulerConfig holds configuration for the StockScheduler.
type SchedulerConfig struct {
	StockCodes      []string      // List of stock codes to fetch data for.
	FetchInterval   time.Duration // How often to fetch data.
	CacheTTL        time.Duration // TTL for data stored in cache by the scheduler.
	RetryAttempts   int           // Number of retries for a failed fetch of a single stock.
	RetryDelay      time.Duration // Delay between retries for a single stock.
	WorkerPoolSize  int           // Number of concurrent workers for fetching stock data.
	InitialDelay    time.Duration // Optional delay before the first run.
}

// SchedulerStatus provides information about the scheduler's current state.
type SchedulerStatus struct {
	IsRunning    bool      `json:"is_running"`
	LastRunStart time.Time `json:"last_run_start"`
	LastRunEnd   time.Time `json:"last_run_end"`
	LastRunError string    `json:"last_run_error,omitempty"`
	NextRun      time.Time `json:"next_run"`
	TotalRuns    int64     `json:"total_runs"`
	TotalErrors  int64     `json:"total_errors"` // Errors during a run, not individual stock fetch errors
}

// TaskRunInfo provides details about a single execution of the scheduled task.
type TaskRunInfo struct {
	RunID           int64         `json:"run_id"`
	StartTime       time.Time     `json:"start_time"`
	EndTime         time.Time     `json:"end_time"`
	Duration        time.Duration `json:"duration"`
	Success         bool          `json:"success"`
	Error           string        `json:"error,omitempty"`
	StocksProcessed int           `json:"stocks_processed"`
	StocksFetched   int           `json:"stocks_fetched"`
	StocksFailed    int           `json:"stocks_failed"`
}

// StockScheduler periodically fetches stock data and stores it in the cache.
type StockScheduler struct {
	provider      stock.StockDataProvider
	cache         *cache.StockCache
	config        SchedulerConfig
	
	ctx           context.Context    // Main context for the scheduler
	cancelFunc    context.CancelFunc // To cancel the scheduler's operations
	ticker        *time.Ticker
	
	wg            sync.WaitGroup     // To wait for running tasks during shutdown
	mu            sync.RWMutex       // Protects status and history
	
	status        SchedulerStatus
	history       []TaskRunInfo
	maxHistoryLen int
	runCounter    int64
}

// NewStockScheduler creates a new StockScheduler.
// Dependencies (provider, cache) and config must be provided.
func NewStockScheduler(provider stock.StockDataProvider, cache *cache.StockCache, config SchedulerConfig) (*StockScheduler, error) {
	if provider == nil {
		return nil, fmt.Errorf("stock data provider cannot be nil")
	}
	if cache == nil {
		return nil, fmt.Errorf("stock cache cannot be nil")
	}
	if config.FetchInterval <= 0 && config.InitialDelay <= 0 { // Allow one-off run with InitialDelay only
		log.Println("[StockScheduler] Warning: FetchInterval is zero or negative. Scheduler will only run once if InitialDelay is set, or not at all.")
	}
	if len(config.StockCodes) == 0 {
		log.Println("[StockScheduler] Warning: No stock codes configured to fetch.")
	}
	if config.WorkerPoolSize <= 0 {
		config.WorkerPoolSize = defaultWorkerPoolSize
	}
	if config.RetryAttempts < 0 {
		config.RetryAttempts = defaultRetryAttempts
	}
    if config.RetryDelay <= 0 && config.RetryAttempts > 0 {
        config.RetryDelay = defaultRetryDelay
    }


	ctx, cancelFunc := context.WithCancel(context.Background())

	return &StockScheduler{
		provider:      provider,
		cache:         cache,
		config:        config,
		ctx:           ctx,
		cancelFunc:    cancelFunc,
		status:        SchedulerStatus{IsRunning: false},
		history:       make([]TaskRunInfo, 0),
		maxHistoryLen: defaultMaxHistoryLen,
		runCounter:    0,
	}, nil
}

// Start begins the scheduled fetching of stock data.
func (s *StockScheduler) Start() {
	s.mu.Lock()
	if s.status.IsRunning {
		s.mu.Unlock()
		log.Println("[StockScheduler] Scheduler is already running.")
		return
	}
	s.status.IsRunning = true
	firstRunTime := time.Now()
	if s.config.InitialDelay > 0 {
		firstRunTime = firstRunTime.Add(s.config.InitialDelay)
	}
	s.status.NextRun = firstRunTime
	s.mu.Unlock()

	log.Println("[StockScheduler] Starting scheduler...")
	s.wg.Add(1) // Add for the main scheduler loop
	go s.runSchedulerLoop()
}

// runSchedulerLoop is the main loop that triggers tasks based on the ticker.
func (s *StockScheduler) runSchedulerLoop() {
	defer s.wg.Done()
    
    if s.config.InitialDelay > 0 {
        initialTimer := time.NewTimer(s.config.InitialDelay)
        select {
        case <-initialTimer.C:
            s.triggerFetchTask()
        case <-s.ctx.Done():
            initialTimer.Stop()
            log.Println("[StockScheduler] Scheduler stopped during initial delay.")
            s.mu.Lock()
            s.status.IsRunning = false
            s.mu.Unlock()
            return
        }
    } else {
         s.triggerFetchTask() // Run immediately if no initial delay
    }


	if s.config.FetchInterval <= 0 {
		log.Println("[StockScheduler] Fetch interval is zero or negative, scheduler will not run periodically.")
		s.mu.Lock()
		s.status.IsRunning = false 
		s.mu.Unlock()
		return
	}

	s.ticker = time.NewTicker(s.config.FetchInterval)
	defer s.ticker.Stop()

	for {
		s.mu.Lock()
		s.status.NextRun = time.Now().Add(s.config.FetchInterval)
		s.mu.Unlock()

		select {
		case <-s.ticker.C:
			s.triggerFetchTask()
		case <-s.ctx.Done():
			log.Println("[StockScheduler] Scheduler loop stopping due to context cancellation.")
            s.mu.Lock()
            s.status.IsRunning = false
            s.mu.Unlock()
			return
		}
	}
}

// triggerFetchTask initiates a single run of the data fetching process.
func (s *StockScheduler) triggerFetchTask() {
	s.mu.Lock()
    // Check if scheduler was stopped right before this task was to be triggered
    if !s.status.IsRunning && s.ctx.Err() != nil { 
        s.mu.Unlock()
        log.Printf("[StockScheduler] Skipping fetch task trigger; scheduler is stopped or stopping.")
        return
    }
	s.runCounter++
	currentRunID := s.runCounter
	s.status.TotalRuns = currentRunID
	s.status.LastRunStart = time.Now()
	s.mu.Unlock()

	log.Printf("[StockScheduler] Triggering fetch task (Run ID: %d) for %d stock(s)...", currentRunID, len(s.config.StockCodes))
	
	s.wg.Add(1) // Add for this fetch task execution
	go func(runID int64) {
		defer s.wg.Done()
		s.executeFetchTask(s.ctx, runID) // Pass the main scheduler context
	}(currentRunID)
}


// executeFetchTask performs the actual data fetching for all configured stocks.
// It uses a worker pool to fetch data concurrently.
func (s *StockScheduler) executeFetchTask(ctx context.Context, runID int64) {
	startTime := time.Now()
	runInfo := TaskRunInfo{
		RunID:     runID,
		StartTime: startTime,
	}

	if len(s.config.StockCodes) == 0 {
		log.Println("[StockScheduler] No stock codes to process for this run.")
		runInfo.EndTime = time.Now()
		runInfo.Duration = runInfo.EndTime.Sub(runInfo.StartTime)
		runInfo.Success = true
		s.addHistory(runInfo)
		s.updateStatusAfterRun(runInfo.Error, startTime)
		return
	}

	jobs := make(chan string, len(s.config.StockCodes))
	results := make(chan fetchResult, len(s.config.StockCodes))

	var workerWg sync.WaitGroup
	for i := 0; i < s.config.WorkerPoolSize; i++ {
		workerWg.Add(1)
		// Each worker gets its own context derived from the task's context
		// to allow for individual fetch cancellations if needed, though not strictly used here for that.
		go s.worker(ctx, &workerWg, jobs, results)
	}

	for _, code := range s.config.StockCodes {
		select {
		case jobs <- code:
			runInfo.StocksProcessed++
		case <-ctx.Done(): 
			log.Printf("[StockScheduler] Run ID %d: Execution cancelled by scheduler while queuing jobs: %v", runID, ctx.Err())
			goto endQueueLoop // Break out of the for loop
		}
	}
endQueueLoop:
	close(jobs) 

	workerWg.Wait() 
	close(results)  

	for result := range results {
		if result.err != nil {
			runInfo.StocksFailed++
			log.Printf("[StockScheduler] Run ID %d: Failed to fetch data for %s: %v", runID, result.code, result.err)
		} else if result.stock != nil {
			runInfo.StocksFetched++
			s.cache.Set(result.stock, s.config.CacheTTL)
			// log.Printf("[StockScheduler] Run ID %d: Fetched and cached data for %s at %v", runID, result.stock.Code, result.stock.Timestamp)
		}
	}
	
	runInfo.EndTime = time.Now()
	runInfo.Duration = runInfo.EndTime.Sub(runInfo.StartTime)
	runInfo.Success = runInfo.StocksFailed == 0 
	if !runInfo.Success {
		runInfo.Error = fmt.Sprintf("%d stock(s) failed to fetch", runInfo.StocksFailed)
	}
	
	s.addHistory(runInfo)
	s.updateStatusAfterRun(runInfo.Error, startTime)

	log.Printf("[StockScheduler] Fetch task (Run ID: %d) completed. Duration: %s. Fetched: %d, Failed: %d",
		runID, runInfo.Duration, runInfo.StocksFetched, runInfo.StocksFailed)
}

type fetchResult struct {
	code  string
	stock *models.Stock
	err   error
}

// worker is a goroutine that processes stock codes from the jobs channel.
func (s *StockScheduler) worker(ctx context.Context, wg *sync.WaitGroup, jobs <-chan string, results chan<- fetchResult) {
	defer wg.Done()
	for code := range jobs {
		// Check if the main scheduler context has been cancelled before starting a new fetch
		select {
		case <-ctx.Done():
			results <- fetchResult{code: code, err: fmt.Errorf("scheduler stopped before processing %s: %w", code, ctx.Err())}
			continue // Move to the next job or exit if jobs channel is closed
		default:
			// Proceed with fetching
		}

		var stockData *models.Stock
		var fetchErr error
		for i := 0; i <= s.config.RetryAttempts; i++ {
			// Create a new context for each attempt that respects the overall scheduler cancellation
			attemptCtx, cancelAttempt := context.WithTimeout(ctx, s.providerCallTimeout())
			stockData, fetchErr = s.provider.FetchLatestMinute(attemptCtx, code)
			cancelAttempt() // Release resources associated with attemptCtx

			if fetchErr == nil {
				break // Success
			}
			
			// If context was cancelled, no point in retrying for this stock code in this worker
			if errors.Is(fetchErr, context.Canceled) || errors.Is(fetchErr, context.DeadlineExceeded) && ctx.Err() != nil {
			    log.Printf("[StockScheduler Worker] Fetch for %s cancelled by scheduler context: %v", code, fetchErr)
			    break
			}

			log.Printf("[StockScheduler Worker] Attempt %d/%d failed for %s: %v", i+1, s.config.RetryAttempts+1, code, fetchErr)
			if i < s.config.RetryAttempts {
				select {
				case <-time.After(s.config.RetryDelay):
					// Wait before retrying
				case <-ctx.Done(): // Check for scheduler cancellation during retry delay
					log.Printf("[StockScheduler Worker] Fetch for %s cancelled during retry delay: %v", code, ctx.Err())
					fetchErr = fmt.Errorf("fetch for %s cancelled during retry: %w", code, ctx.Err())
					goto sendResult // Break outer loop and send result
				}
			}
		}
		sendResult:
		results <- fetchResult{code: code, stock: stockData, err: fetchErr}
	}
}

// providerCallTimeout defines the timeout for a single call to the stock data provider.
func (s *StockScheduler) providerCallTimeout() time.Duration {
    timeout := 30 * time.Second 
    // Ensure timeout is positive and less than fetch interval if possible
    if s.config.FetchInterval > 0 {
        calculatedTimeout := s.config.FetchInterval / 3
        if calculatedTimeout > 0 && calculatedTimeout < timeout {
            timeout = calculatedTimeout
        }
    }
    if timeout <= 0 {
        timeout = 10 * time.Second // Absolute minimum
    }
    return timeout
}


func (s *StockScheduler) updateStatusAfterRun(runErr string, startTime time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.LastRunEnd = time.Now()
	// s.status.LastRunStart = startTime // Already set by triggerFetchTask
	if runErr != "" {
		s.status.LastError = runErr
		s.status.TotalErrors++
	} else {
		s.status.LastError = ""
	}
}

func (s *StockScheduler) addHistory(info TaskRunInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history = append(s.history, info)
	if len(s.history) > s.maxHistoryLen {
		s.history = s.history[len(s.history)-s.maxHistoryLen:]
	}
}

// Stop gracefully shuts down the scheduler.
func (s *StockScheduler) Stop() {
    s.mu.Lock()
    if !s.status.IsRunning && s.ctx.Err() != nil { // Already stopped or stopping
        s.mu.Unlock()
        log.Println("[StockScheduler] Scheduler is already stopped or in the process of stopping.")
        return
    }
    log.Println("[StockScheduler] Initiating shutdown...")
    s.status.IsRunning = false // Prevent new tasks from being triggered logically
    s.mu.Unlock()

    s.cancelFunc() // Signal all operations using s.ctx to cancel

    // Wait for the main scheduler loop and any running fetch tasks to complete.
    s.wg.Wait()
    log.Println("[StockScheduler] Scheduler stopped successfully.")
}

// GetStatus returns the current status of the scheduler.
func (s *StockScheduler) GetStatus() SchedulerStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	statusCopy := s.status // Return a copy
	return statusCopy
}

// GetHistory returns a copy of the recent task run history.
func (s *StockScheduler) GetHistory() []TaskRunInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	historyCopy := make([]TaskRunInfo, len(s.history))
	copy(historyCopy, s.history)
	return historyCopy
}

// UpdateConfig dynamically updates the scheduler's configuration.
// Note: This is a basic implementation for non-critical fields.
// Changing FetchInterval while running effectively requires a Stop() and Start().
func (s *StockScheduler) UpdateConfig(newConfig SchedulerConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if newConfig.FetchInterval <= 0 && newConfig.InitialDelay <= 0{
		log.Println("[StockScheduler Config Update] Warning: FetchInterval is zero or negative. Scheduler will only run once if InitialDelay is set.")
	}
	if newConfig.WorkerPoolSize <= 0 {
		newConfig.WorkerPoolSize = defaultWorkerPoolSize
	}
    if newConfig.RetryAttempts < 0 {
		newConfig.RetryAttempts = defaultRetryAttempts
	}
    if newConfig.RetryDelay <= 0 && newConfig.RetryAttempts > 0 {
        newConfig.RetryDelay = defaultRetryDelay
    }

    // For a running scheduler, certain config changes are complex.
    // For example, FetchInterval would require ticker reset.
    // Here, we just update the config. User should manage Stop/Start for such changes.
    if s.status.IsRunning && s.config.FetchInterval != newConfig.FetchInterval {
        log.Printf("[StockScheduler] Configuration Warning: FetchInterval changed from %v to %v. Scheduler needs a restart (Stop then Start) for this change to fully take effect.", s.config.FetchInterval, newConfig.FetchInterval)
    }
	
	s.config = newConfig
	log.Println("[StockScheduler] Configuration updated.")
	return nil
}

// AddStockCode adds a stock code to the list of stocks to monitor.
func (s *StockScheduler) AddStockCode(code string) {
    s.mu.Lock()
    defer s.mu.Unlock()

    for _, existingCode := range s.config.StockCodes {
        if existingCode == code {
            log.Printf("[StockScheduler] Stock code %s already in monitor list.", code)
            return
        }
    }
    s.config.StockCodes = append(s.config.StockCodes, code)
    log.Printf("[StockScheduler] Added stock code %s to monitor list.", code)
}

// RemoveStockCode removes a stock code from the list.
func (s *StockScheduler) RemoveStockCode(code string) {
    s.mu.Lock()
    defer s.mu.Unlock()

    found := false
    newCodes := make([]string, 0, len(s.config.StockCodes))
    for _, existingCode := range s.config.StockCodes {
        if existingCode == code {
            found = true
        } else {
            newCodes = append(newCodes, existingCode)
        }
    }

    if found {
        s.config.StockCodes = newCodes
        log.Printf("[StockScheduler] Removed stock code %s from monitor list.", code)
    } else {
        log.Printf("[StockScheduler] Stock code %s not found in monitor list for removal.", code)
    }
}
