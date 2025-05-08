# Cache Improvements

## Purpose
The caching mechanism in the stockticker service is critical for reducing external API calls, improving response times, and handling rate limits. The current implementation has several limitations that could lead to memory leaks, inefficient resource usage, and performance issues in production.

## 1. Automatic Cache Cleanup

### Problem
The current cache implementation lacks automatic cleanup of expired items. Expired entries remain in memory until they're explicitly accessed, leading to potential memory leaks over time.

### Implementation Location
Update `internal/cache/cache.go` to add a background cleanup routine.

### Required Implementation

```go
package cache

import (
    "sync"
    "time"
    "sync/atomic"
)

// Configurable constants
const (
    defaultCleanupInterval = 5 * time.Minute
    defaultMaxItems = 1000
)

// Cache is a simple in-memory cache with expiration
type Cache struct {
    items      map[string]Item
    mu         sync.RWMutex
    maxItems   int
    metrics    CacheMetrics
    stopCleanup chan struct{}
}

// Item represents a cached item with expiration
type Item struct {
    Value      interface{}
    Expiration int64
}

// CacheMetrics tracks cache performance statistics
type CacheMetrics struct {
    Hits        int64
    Misses      int64
    Evictions   int64
    Cleanups    int64
}

// New creates a new cache with automatic cleanup
func New(options ...CacheOption) *Cache {
    // Default configuration
    c := &Cache{
        items:       make(map[string]Item),
        maxItems:    defaultMaxItems,
        stopCleanup: make(chan struct{}),
    }

    // Apply options
    for _, option := range options {
        option(c)
    }

    // Start background cleanup
    go c.startCleanupRoutine()

    return c
}

// CacheOption defines a cache configuration option
type CacheOption func(*Cache)

// WithMaxItems sets the maximum number of items in the cache
func WithMaxItems(maxItems int) CacheOption {
    return func(c *Cache) {
        if maxItems > 0 {
            c.maxItems = maxItems
        }
    }
}

// WithCleanupInterval sets a custom cleanup interval
func WithCleanupInterval(interval time.Duration) CacheOption {
    return func(c *Cache) {
        if interval > 0 {
            // This option would configure the cleanup interval
            // The actual implementation would store this value
        }
    }
}

// startCleanupRoutine runs a goroutine that periodically cleans up expired items
func (c *Cache) startCleanupRoutine() {
    ticker := time.NewTicker(defaultCleanupInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            c.Cleanup()
        case <-c.stopCleanup:
            return
        }
    }
}

// Cleanup removes expired items from the cache
func (c *Cache) Cleanup() {
    c.mu.Lock()
    defer c.mu.Unlock()

    now := time.Now().UnixNano()
    expiredCount := 0

    for k, v := range c.items {
        if now > v.Expiration {
            delete(c.items, k)
            expiredCount++
        }
    }

    // Update metrics
    if expiredCount > 0 {
        atomic.AddInt64(&c.metrics.Evictions, int64(expiredCount))
        atomic.AddInt64(&c.metrics.Cleanups, 1)
    }
}

// Close stops the background cleanup routine
func (c *Cache) Close() {
    close(c.stopCleanup)
}
```

## 2. Cache Size Limiting (LRU Eviction Policy)

### Problem
Without size limits, the cache can consume unbounded memory as more unique items are added. This is particularly problematic for applications with high cardinality data, like stock symbols.

### Implementation Location
Enhance `internal/cache/cache.go` with LRU eviction functionality.

### Required Implementation

```go
package cache

import (
    "container/list"
    // other imports from above
)

// Cache with LRU eviction policy
type Cache struct {
    // Existing fields from above
    evictList *list.List
    itemMap   map[string]*list.Element
}

// cacheEntry represents an entry in the eviction list
type cacheEntry struct {
    key        string
    value      interface{}
    expiration int64
}

// New creates a cache with LRU eviction
func New(options ...CacheOption) *Cache {
    c := &Cache{
        items:       make(map[string]Item),
        evictList:   list.New(),
        itemMap:     make(map[string]*list.Element),
        maxItems:    defaultMaxItems,
        stopCleanup: make(chan struct{}),
    }

    // Apply options
    for _, option := range options {
        option(c)
    }

    // Start background cleanup
    go c.startCleanupRoutine()

    return c
}

// Set adds an item to the cache with LRU tracking
func (c *Cache) Set(key string, value interface{}, duration time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()

    expiration := time.Now().Add(duration).UnixNano()

    // Check if key already exists
    if el, exists := c.itemMap[key]; exists {
        // Move to front (most recently used)
        c.evictList.MoveToFront(el)

        // Update the value and expiration
        entry := el.Value.(*cacheEntry)
        entry.value = value
        entry.expiration = expiration

        // Update the items map
        c.items[key] = Item{
            Value:      value,
            Expiration: expiration,
        }

        return
    }

    // Add new item
    entry := &cacheEntry{
        key:        key,
        value:      value,
        expiration: expiration,
    }

    // Add to eviction list and item map
    element := c.evictList.PushFront(entry)
    c.itemMap[key] = element

    // Add to items map
    c.items[key] = Item{
        Value:      value,
        Expiration: expiration,
    }

    // Check if we need to evict
    if c.evictList.Len() > c.maxItems {
        c.removeOldest()
    }
}

// Get retrieves an item from the cache
func (c *Cache) Get(key string) (interface{}, bool) {
    c.mu.RLock()

    // Get item from map
    item, found := c.items[key]
    if !found {
        c.mu.RUnlock()
        atomic.AddInt64(&c.metrics.Misses, 1)
        return nil, false
    }

    // Check if expired
    if time.Now().UnixNano() > item.Expiration {
        c.mu.RUnlock()

        // Remove expired item (with write lock)
        c.mu.Lock()
        defer c.mu.Unlock()

        // Double-check after getting write lock
        if el, exists := c.itemMap[key]; exists {
            c.removeElement(el)
        }

        atomic.AddInt64(&c.metrics.Misses, 1)
        return nil, false
    }

    // Update LRU list
    element := c.itemMap[key]
    c.mu.RUnlock()

    // Move to front with write lock
    c.mu.Lock()
    c.evictList.MoveToFront(element)
    c.mu.Unlock()

    atomic.AddInt64(&c.metrics.Hits, 1)
    return item.Value, true
}

// Delete removes an item from the cache
func (c *Cache) Delete(key string) {
    c.mu.Lock()
    defer c.mu.Unlock()

    if el, exists := c.itemMap[key]; exists {
        c.removeElement(el)
    }
}

// removeOldest removes the oldest entry from the cache
func (c *Cache) removeOldest() {
    element := c.evictList.Back()
    if element != nil {
        c.removeElement(element)
        atomic.AddInt64(&c.metrics.Evictions, 1)
    }
}

// removeElement removes an element from the cache
func (c *Cache) removeElement(e *list.Element) {
    c.evictList.Remove(e)
    entry := e.Value.(*cacheEntry)
    delete(c.items, entry.key)
    delete(c.itemMap, entry.key)
}

// Cleanup with LRU handling
func (c *Cache) Cleanup() {
    c.mu.Lock()
    defer c.mu.Unlock()

    now := time.Now().UnixNano()
    expiredCount := 0

    // Iterate from oldest to newest
    for e := c.evictList.Back(); e != nil; {
        entry := e.Value.(*cacheEntry)
        if now > entry.expiration {
            // Get the next element before removing
            next := e.Prev()
            c.removeElement(e)
            e = next
            expiredCount++
        } else {
            e = e.Prev()
        }
    }

    // Update metrics
    if expiredCount > 0 {
        atomic.AddInt64(&c.metrics.Evictions, int64(expiredCount))
        atomic.AddInt64(&c.metrics.Cleanups, 1)
    }
}
```

## 3. Cache Metrics and Instrumentation

### Problem
The current implementation lacks visibility into cache performance, making it difficult to tune and optimize the caching strategy.

### Implementation Location
Enhance `internal/cache/cache.go` with metrics tracking.

### Required Implementation

```go
// The metrics struct is defined above in the CacheMetrics type

// Metrics returns a copy of the current cache metrics
func (c *Cache) Metrics() CacheMetrics {
    return CacheMetrics{
        Hits:      atomic.LoadInt64(&c.metrics.Hits),
        Misses:    atomic.LoadInt64(&c.metrics.Misses),
        Evictions: atomic.LoadInt64(&c.metrics.Evictions),
        Cleanups:  atomic.LoadInt64(&c.metrics.Cleanups),
    }
}

// ResetMetrics resets all metrics counters to zero
func (c *Cache) ResetMetrics() {
    atomic.StoreInt64(&c.metrics.Hits, 0)
    atomic.StoreInt64(&c.metrics.Misses, 0)
    atomic.StoreInt64(&c.metrics.Evictions, 0)
    atomic.StoreInt64(&c.metrics.Cleanups, 0)
}

// Stats returns a map of cache statistics including metrics and size
func (c *Cache) Stats() map[string]interface{} {
    c.mu.RLock()
    itemCount := len(c.items)
    c.mu.RUnlock()

    metrics := c.Metrics()
    hitRatio := float64(0)
    if metrics.Hits+metrics.Misses > 0 {
        hitRatio = float64(metrics.Hits) / float64(metrics.Hits+metrics.Misses)
    }

    return map[string]interface{}{
        "size":       itemCount,
        "max_size":   c.maxItems,
        "hit_count":  metrics.Hits,
        "miss_count": metrics.Misses,
        "hit_ratio":  hitRatio,
        "evictions":  metrics.Evictions,
        "cleanups":   metrics.Cleanups,
    }
}
```

## 4. Prometheus Integration for Metrics

### Problem
While internal metrics are useful for debugging, they should be exposed to monitoring systems for alerting and dashboarding.

### Implementation Location
Create a new file at `internal/cache/prometheus.go`.

### Required Implementation

```go
package cache

import (
    "github.com/prometheus/client_golang/prometheus"
)

var (
    cacheHits = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "stockticker_cache_hits_total",
            Help: "Total number of cache hits",
        },
        []string{"cache_name"},
    )

    cacheMisses = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "stockticker_cache_misses_total",
            Help: "Total number of cache misses",
        },
        []string{"cache_name"},
    )

    cacheEvictions = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "stockticker_cache_evictions_total",
            Help: "Total number of cache evictions",
        },
        []string{"cache_name"},
    )

    cacheSize = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "stockticker_cache_size",
            Help: "Current number of items in the cache",
        },
        []string{"cache_name"},
    )

    cacheHitRatio = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "stockticker_cache_hit_ratio",
            Help: "Ratio of cache hits to total requests",
        },
        []string{"cache_name"},
    )
)

func init() {
    // Register the metrics with Prometheus
    prometheus.MustRegister(cacheHits)
    prometheus.MustRegister(cacheMisses)
    prometheus.MustRegister(cacheEvictions)
    prometheus.MustRegister(cacheSize)
    prometheus.MustRegister(cacheHitRatio)
}

// WithPrometheusMetrics enables Prometheus metrics for the cache
func WithPrometheusMetrics(name string) CacheOption {
    return func(c *Cache) {
        c.name = name
        c.enablePrometheus = true
    }
}

// updatePrometheusMetrics updates Prometheus metrics with current values
func (c *Cache) updatePrometheusMetrics() {
    if !c.enablePrometheus {
        return
    }

    // Update gauge metrics
    c.mu.RLock()
    size := len(c.items)
    c.mu.RUnlock()

    cacheSize.WithLabelValues(c.name).Set(float64(size))

    metrics := c.Metrics()
    total := metrics.Hits + metrics.Misses
    if total > 0 {
        hitRatio := float64(metrics.Hits) / float64(total)
        cacheHitRatio.WithLabelValues(c.name).Set(hitRatio)
    }
}

// The modified Get method increments Prometheus counters
func (c *Cache) Get(key string) (interface{}, bool) {
    // Existing implementation from above...

    // For cache hit:
    if c.enablePrometheus {
        cacheHits.WithLabelValues(c.name).Inc()
    }

    // For cache miss:
    if c.enablePrometheus {
        cacheMisses.WithLabelValues(c.name).Inc()
    }

    // Update metrics occasionally
    if c.enablePrometheus && (metrics.Hits+metrics.Misses)%100 == 0 {
        c.updatePrometheusMetrics()
    }

    // return value as before...
}

// The modified removeOldest method increments Prometheus eviction counter
func (c *Cache) removeOldest() {
    // Existing implementation from above...

    if c.enablePrometheus {
        cacheEvictions.WithLabelValues(c.name).Inc()
    }
}
```

## 5. TTL-Based vs. Max Age Caching Strategy

### Problem
The current implementation uses a fixed TTL for all items. For financial data like stock prices, a more nuanced caching strategy based on data freshness might be appropriate.

### Implementation Location
Update `internal/service/stock.go` to implement a more intelligent caching strategy.

### Required Implementation

```go
// GetStockData with improved caching strategy
func (s *StockService) GetStockData(ctx context.Context) (*models.StockData, error) {
    cacheKey := s.config.Symbol

    // Use different cache durations based on time of day
    cacheDuration := s.determineCacheDuration()

    // Try to get data from cache first
    if cachedData, found := s.cache.Get(cacheKey); found {
        return cachedData.(*models.StockData), nil
    }

    // Fetch from API and process as before...

    // Cache with dynamic duration
    s.cache.Set(cacheKey, stockData, cacheDuration)

    return stockData, nil
}

// determineCacheDuration calculates the appropriate cache duration
// based on market hours and data freshness requirements
func (s *StockService) determineCacheDuration() time.Duration {
    now := time.Now()

    // Get current hour in NYSE timezone (Eastern Time)
    loc, _ := time.LoadLocation("America/New_York")
    nyTime := now.In(loc)
    hour := nyTime.Hour()
    weekday := nyTime.Weekday()

    // Use shorter cache during market hours on weekdays
    if weekday >= time.Monday && weekday <= time.Friday {
        // Market hours: 9:30 AM - 4:00 PM ET
        if hour >= 9 && hour < 16 {
            // During market hours, cache for shorter duration
            return 5 * time.Minute
        }
    }

    // Outside market hours or on weekends, cache longer
    return 30 * time.Minute
}
```

## 6. Using the Improved Cache in Main.go

### Implementation Location
Update `cmd/stockticker/main.go` to use the enhanced cache.

### Required Implementation

```go
func main() {
    // ... existing initialization code ...

    // Create cache with options
    cacheInstance := cache.New(
        cache.WithMaxItems(1000),
        cache.WithPrometheusMetrics("stock_cache"),
    )

    // Ensure proper cleanup on shutdown
    defer cacheInstance.Close()

    // ... rest of initialization code ...

    // Setup graceful shutdown
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    log.Println("Shutting down server...")

    // Print cache stats on shutdown
    log.Printf("Cache stats at shutdown: %v", cacheInstance.Stats())
}
```

## Benefits

1. **Memory Efficiency**: Size limits and automatic cleanup prevent memory leaks
2. **Optimized Performance**: LRU eviction ensures the most valuable data stays in cache
3. **Observability**: Metrics provide visibility into cache effectiveness
4. **Adaptive Caching**: Dynamic TTLs optimize for data freshness based on context
5. **Resource Management**: Controlled eviction prevents resource exhaustion
6. **Graceful Shutdown**: Proper cleanup on shutdown prevents resource leaks

## Codebase Analysis

The current cache implementation in the stockticker service is basic and lacks several features needed for production-grade caching. The proposed improvements address memory management, performance optimization, and observability concerns, making the cache more robust and suitable for production use.