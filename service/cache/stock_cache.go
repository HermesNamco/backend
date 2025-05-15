package cache

import (
	"container/list"
	"fmt"
	"sync"
	"time"

	"backend/models"
)

// StockCache provides an in-memory cache for stock data with TTL and eviction policies
type StockCache struct {
	// Mutex for thread safety
	mu sync.RWMutex

	// The main cache data structure - maps stock code to another map of timestamp to stock data
	data map[string]map[time.Time]*cacheEntry

	// Eviction policy data structures
	evictionList *list.List                             // Linked list for tracking access order (LRU)
	evictionMap  map[string]map[time.Time]*list.Element // Maps code and timestamp to list elements

	// Configuration
	capacity       int           // Maximum number of items in cache
	defaultTTL     time.Duration // Default time-to-live for cache entries
	evictionPolicy string        // Eviction policy: "lru", "fifo", or "ttl"

	// Statistics
	hits        int64
	misses      int64
	evictions   int64
	insertions  int64
	lastCleanup time.Time
}

// cacheEntry represents a single cached stock data entry
type cacheEntry struct {
	stock      *models.Stock // The actual stock data
	expiry     time.Time     // When this entry expires
	insertedAt time.Time     // When this entry was inserted into the cache
}

// CacheKey combines a stock code and timestamp for eviction map keys
type CacheKey struct {
	Code      string
	Timestamp time.Time
}

// CacheOptions configures the cache behavior
type CacheOptions struct {
	Capacity        int           // Maximum number of items to store
	DefaultTTL      time.Duration // Default TTL for entries
	EvictionPolicy  string        // "lru", "fifo", or "ttl"
	CleanupInterval time.Duration // How often to clean expired entries
}

// DefaultCacheOptions returns sensible default options
func DefaultCacheOptions() CacheOptions {
	return CacheOptions{
		Capacity:        10000,            // Store up to 10,000 stock data points
		DefaultTTL:      24 * time.Hour,   // Cache entries expire after 24 hours
		EvictionPolicy:  "lru",            // Least Recently Used eviction
		CleanupInterval: 10 * time.Minute, // Clean up every 10 minutes
	}
}

// NewStockCache creates a new stock cache with the given options
func NewStockCache(options CacheOptions) *StockCache {
	cache := &StockCache{
		data:           make(map[string]map[time.Time]*cacheEntry),
		evictionList:   list.New(),
		evictionMap:    make(map[string]map[time.Time]*list.Element),
		capacity:       options.Capacity,
		defaultTTL:     options.DefaultTTL,
		evictionPolicy: options.EvictionPolicy,
		lastCleanup:    time.Now(),
	}

	// Start background cleanup if interval > 0
	if options.CleanupInterval > 0 {
		go cache.startCleanupRoutine(options.CleanupInterval)
	}

	return cache
}

// Get retrieves a stock data entry from the cache
func (c *StockCache) Get(code string, timestamp time.Time) (*models.Stock, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Check if code exists in cache
	codeMap, exists := c.data[code]
	if !exists {
		c.misses++
		return nil, false
	}

	// Check if timestamp exists for this code
	entry, exists := codeMap[timestamp]
	if !exists {
		c.misses++
		return nil, false
	}

	// Check if entry has expired
	if time.Now().After(entry.expiry) {
		c.misses++
		// We'll let the cleanup routine remove it
		return nil, false
	}

	c.hits++

	// If using LRU, update the entry position in the eviction list
	if c.evictionPolicy == "lru" {
		c.updateLRUPosition(code, timestamp)
	}

	return entry.stock, true
}

// GetLatest retrieves the most recent stock data for a given code
func (c *StockCache) GetLatest(code string) (*models.Stock, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	codeMap, exists := c.data[code]
	if !exists || len(codeMap) == 0 {
		c.misses++
		return nil, false
	}

	var latestStock *models.Stock
	var latestTime time.Time

	for timestamp, entry := range codeMap {
		// Skip expired entries
		if time.Now().After(entry.expiry) {
			continue
		}

		// Find the most recent valid timestamp
		if latestStock == nil || timestamp.After(latestTime) {
			latestStock = entry.stock
			latestTime = timestamp
		}
	}

	if latestStock == nil {
		c.misses++
		return nil, false
	}

	c.hits++

	// If using LRU, update the position
	if c.evictionPolicy == "lru" {
		c.updateLRUPosition(code, latestTime)
	}

	return latestStock, true
}

// GetRange retrieves all stock data for a given code within a time range
func (c *StockCache) GetRange(code string, start, end time.Time) []models.Stock {
	c.mu.RLock()
	defer c.mu.RUnlock()

	codeMap, exists := c.data[code]
	if !exists {
		return []models.Stock{}
	}

	result := make([]models.Stock, 0)
	for timestamp, entry := range codeMap {
		// Skip expired entries
		if time.Now().After(entry.expiry) {
			continue
		}

		// Check if timestamp is within range
		if (timestamp.Equal(start) || timestamp.After(start)) &&
			(timestamp.Equal(end) || timestamp.Before(end)) {
			result = append(result, *entry.stock)

			// If using LRU, update position for each accessed entry
			if c.evictionPolicy == "lru" {
				c.updateLRUPosition(code, timestamp)
			}
		}
	}

	// Update statistics
	if len(result) > 0 {
		c.hits++
	} else {
		c.misses++
	}

	return result
}

// Set adds or updates a stock data entry in the cache
func (c *StockCache) Set(stock *models.Stock, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if stock == nil {
		return
	}

	code := stock.Code
	timestamp := stock.Timestamp

	// Apply default TTL if not specified
	if ttl <= 0 {
		ttl = c.defaultTTL
	}

	// Create map for this code if it doesn't exist
	if _, exists := c.data[code]; !exists {
		c.data[code] = make(map[time.Time]*cacheEntry)
		c.evictionMap[code] = make(map[time.Time]*list.Element)
	}

	// Create the cache entry
	entry := &cacheEntry{
		stock:      stock,
		expiry:     time.Now().Add(ttl),
		insertedAt: time.Now(),
	}

	// Check if we're updating an existing entry
	if _, exists := c.data[code][timestamp]; exists {
		c.data[code][timestamp] = entry

		// Update the eviction list if using LRU or FIFO
		if c.evictionPolicy == "lru" || c.evictionPolicy == "fifo" {
			// Get the list element for this entry
			if elem, ok := c.evictionMap[code][timestamp]; ok {
				// For LRU, move to front
				if c.evictionPolicy == "lru" {
					c.evictionList.MoveToFront(elem)
				}
				// For FIFO, position doesn't change
				// Update the value in the list
				elem.Value = CacheKey{Code: code, Timestamp: timestamp}
			}
		}
	} else {
		// This is a new entry
		c.data[code][timestamp] = entry
		c.insertions++

		// Add to eviction structures
		key := CacheKey{Code: code, Timestamp: timestamp}
		var elem *list.Element

		if c.evictionPolicy == "lru" || c.evictionPolicy == "fifo" {
			elem = c.evictionList.PushFront(key)
			c.evictionMap[code][timestamp] = elem
		}

		// Check if cache capacity has been reached
		c.evictIfNeeded()
	}
}

// SetBatch adds or updates multiple stock data entries at once
func (c *StockCache) SetBatch(stocks []models.Stock, ttl time.Duration) {
	for i := range stocks {
		c.Set(&stocks[i], ttl)
	}
}

// Remove removes a specific stock data entry from the cache
func (c *StockCache) Remove(code string, timestamp time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if codeMap, exists := c.data[code]; exists {
		if _, exists := codeMap[timestamp]; exists {
			// Remove from data map
			delete(codeMap, timestamp)

			// Remove from eviction structures
			if c.evictionPolicy == "lru" || c.evictionPolicy == "fifo" {
				if elem, ok := c.evictionMap[code][timestamp]; ok {
					c.evictionList.Remove(elem)
					delete(c.evictionMap[code], timestamp)
				}
			}

			// If this code's map is now empty, remove it too
			if len(codeMap) == 0 {
				delete(c.data, code)
				delete(c.evictionMap, code)
			}
		}
	}
}

// RemoveCode removes all stock data for a given code
func (c *StockCache) RemoveCode(code string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.data[code]; exists {
		// Remove all entries for this code from the eviction list
		if c.evictionPolicy == "lru" || c.evictionPolicy == "fifo" {
			if evictionMap, ok := c.evictionMap[code]; ok {
				for _, elem := range evictionMap {
					c.evictionList.Remove(elem)
				}
			}
		}

		// Remove the code's map
		delete(c.data, code)
		delete(c.evictionMap, code)
	}
}

// Clear removes all entries from the cache
func (c *StockCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data = make(map[string]map[time.Time]*cacheEntry)
	c.evictionList = list.New()
	c.evictionMap = make(map[string]map[time.Time]*list.Element)
	c.hits = 0
	c.misses = 0
	c.evictions = 0
	c.insertions = 0
}

// Size returns the total number of entries in the cache
func (c *StockCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := 0
	for _, codeMap := range c.data {
		total += len(codeMap)
	}
	return total
}

// Stats returns cache statistics
func (c *StockCache) Stats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	hitRate := float64(0)
	if c.hits+c.misses > 0 {
		hitRate = float64(c.hits) / float64(c.hits+c.misses)
	}

	stats := map[string]interface{}{
		"size":            c.Size(),
		"capacity":        c.capacity,
		"hits":            c.hits,
		"misses":          c.misses,
		"hit_rate":        hitRate,
		"evictions":       c.evictions,
		"insertions":      c.insertions,
		"last_cleanup":    c.lastCleanup,
		"eviction_policy": c.evictionPolicy,
	}

	// Count expired items
	expired := 0
	now := time.Now()
	for _, codeMap := range c.data {
		for _, entry := range codeMap {
			if now.After(entry.expiry) {
				expired++
			}
		}
	}
	stats["expired_items"] = expired

	// Count items by code
	codeStats := make(map[string]int)
	for code, codeMap := range c.data {
		codeStats[code] = len(codeMap)
	}
	stats["items_by_code"] = codeStats

	return stats
}

// Cleanup removes expired entries from the cache
func (c *StockCache) Cleanup() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.cleanupLocked()
}

// cleanupLocked removes expired entries (internal use with lock held)
func (c *StockCache) cleanupLocked() int {
	removed := 0
	now := time.Now()

	for code, codeMap := range c.data {
		for timestamp, entry := range codeMap {
			if now.After(entry.expiry) {
				// Remove from data map
				delete(codeMap, timestamp)

				// Remove from eviction structures
				if c.evictionPolicy == "lru" || c.evictionPolicy == "fifo" {
					if elem, ok := c.evictionMap[code][timestamp]; ok {
						c.evictionList.Remove(elem)
						delete(c.evictionMap[code], timestamp)
					}
				}

				removed++
			}
		}

		// If this code's map is now empty, remove it too
		if len(codeMap) == 0 {
			delete(c.data, code)
			delete(c.evictionMap, code)
		}
	}

	c.lastCleanup = now
	return removed
}

// startCleanupRoutine runs a background goroutine to periodically clean up expired entries
func (c *StockCache) startCleanupRoutine(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		removed := c.Cleanup()
		if removed > 0 {
			fmt.Printf("Removed %d expired entries from cache\n", removed)
		}
	}
}

// evictIfNeeded removes entries based on the eviction policy if capacity is exceeded
func (c *StockCache) evictIfNeeded() {
	// Check total size
	currentSize := c.Size()

	// If we're under capacity, nothing to do
	if currentSize <= c.capacity {
		return
	}

	// Need to evict items until we're back under capacity
	toEvict := currentSize - c.capacity
	evicted := 0

	switch c.evictionPolicy {
	case "lru", "fifo":
		// For both LRU and FIFO, we remove from the back of the list
		for evicted < toEvict && c.evictionList.Len() > 0 {
			// Get the oldest/least recently used element
			elem := c.evictionList.Back()
			if elem == nil {
				break
			}

			// Extract the key
			key, ok := elem.Value.(CacheKey)
			if !ok {
				// Something's wrong with the list element
				c.evictionList.Remove(elem)
				continue
			}

			// Remove from all structures
			if codeMap, exists := c.data[key.Code]; exists {
				if _, exists := codeMap[key.Timestamp]; exists {
					delete(codeMap, key.Timestamp)

					// If this was the last entry for this code, remove the code map
					if len(codeMap) == 0 {
						delete(c.data, key.Code)
					}
				}
			}

			if codeEvMap, exists := c.evictionMap[key.Code]; exists {
				delete(codeEvMap, key.Timestamp)

				// If this was the last entry for this code, remove the code map
				if len(codeEvMap) == 0 {
					delete(c.evictionMap, key.Code)
				}
			}

			// Remove from list
			c.evictionList.Remove(elem)

			evicted++
			c.evictions++
		}

	case "ttl":
		// For TTL, we remove the entries closest to expiration
		// This requires scanning all entries
		type evictionCandidate struct {
			code      string
			timestamp time.Time
			expiry    time.Time
		}

		candidates := make([]evictionCandidate, 0, toEvict)

		// Collect all entries with their expiry times
		for code, codeMap := range c.data {
			for timestamp, entry := range codeMap {
				candidates = append(candidates, evictionCandidate{
					code:      code,
					timestamp: timestamp,
					expiry:    entry.expiry,
				})
			}
		}

		// Sort by expiry time (closest to expiring first)
		// Simple bubble sort for simplicity - in production code, use a more efficient algorithm
		for i := 0; i < len(candidates)-1; i++ {
			for j := 0; j < len(candidates)-i-1; j++ {
				if candidates[j].expiry.After(candidates[j+1].expiry) {
					candidates[j], candidates[j+1] = candidates[j+1], candidates[j]
				}
			}
		}

		// Evict the required number of entries, starting with those closest to expiration
		for i := 0; i < toEvict && i < len(candidates); i++ {
			candidate := candidates[i]

			// Remove from data map
			if codeMap, exists := c.data[candidate.code]; exists {
				delete(codeMap, candidate.timestamp)

				// If this was the last entry for this code, remove the code map
				if len(codeMap) == 0 {
					delete(c.data, candidate.code)
				}
			}

			// Remove from eviction structures if applicable
			if c.evictionPolicy == "lru" || c.evictionPolicy == "fifo" {
				if codeEvMap, exists := c.evictionMap[candidate.code]; exists {
					if elem, ok := codeEvMap[candidate.timestamp]; ok {
						c.evictionList.Remove(elem)
					}
					delete(codeEvMap, candidate.timestamp)

					// If this was the last entry for this code, remove the code map
					if len(codeEvMap) == 0 {
						delete(c.evictionMap, candidate.code)
					}
				}
			}

			evicted++
			c.evictions++
		}
	}
}

// updateLRUPosition updates the position of an entry in the LRU list (internal use with lock held)
func (c *StockCache) updateLRUPosition(code string, timestamp time.Time) {
	if c.evictionPolicy != "lru" {
		return
	}

	// If this code+timestamp exists in our eviction map, move it to front of list
	if codeEvMap, exists := c.evictionMap[code]; exists {
		if elem, exists := codeEvMap[timestamp]; exists {
			c.evictionList.MoveToFront(elem)
		}
	}
}
