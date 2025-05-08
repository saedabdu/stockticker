# Resilience Improvements

## Purpose
Resilience is a critical quality for production services, especially those that depend on external APIs like the stockticker service. Resilient systems continue to function correctly in the face of transient failures, overload, and other operational challenges. The current implementation lacks several key resilience patterns that would make it more robust in production.

## 1. Retry Logic for External API Calls

### Problem
The current implementation makes a single attempt to call the Alpha Vantage API. If that request fails due to a transient network issue, timeout, or temporary server error, the application will immediately return an error to the client, even though a retry might succeed.

### Implementation Location
Create a new utility package at `internal/util/retry/retry.go` and update `internal/client/alphavantage.go`.

### Required Implementation

```go
// internal/util/retry/retry.go
package retry

import (
	"context"
	"errors"
	"time"
)

// Options configures the retry behavior
type Options struct {
	// MaxRetries is the maximum number of retries
	MaxRetries int

	// InitialInterval is the initial retry interval
	InitialInterval time.Duration

	// MaxInterval is the maximum retry interval
	MaxInterval time.Duration

	// Multiplier is the factor by which the interval increases
	Multiplier float64

	// RandomizationFactor is the randomization factor
	RandomizationFactor float64
}

// DefaultOptions returns the default retry options
func DefaultOptions() Options {
	return Options{
		MaxRetries:          3,
		InitialInterval:     100 * time.Millisecond,
		MaxInterval:         10 * time.Second,
		Multiplier:          1.5,
		RandomizationFactor: 0.5,
	}
}

// ShouldRetry is a function that decides if a retry should be attempted
type ShouldRetry func(err error) bool

// Do executes f with exponential backoff, retrying on errors that match shouldRetry
func Do(ctx context.Context, f func() error, shouldRetry ShouldRetry, opts Options) error {
	backoff := initialBackoff(opts)

	var err error
	for i := 0; i <= opts.MaxRetries; i++ {
		// Execute the function
		err = f()

		// Return on success or non-retryable error
		if err == nil || (shouldRetry != nil && !shouldRetry(err)) {
			return err
		}

		// Check if we've reached max retries
		if i == opts.MaxRetries {
			break
		}

		// Calculate next backoff
		nextBackoff := calculateBackoff(i, backoff, opts)

		// Wait with context awareness
		select {
		case <-ctx.Done():
			return errors.Join(ctx.Err(), err)
		case <-time.After(nextBackoff):
			// Continue to next retry
		}

		backoff = nextBackoff
	}

	return err
}

// Helper functions for backoff calculation
func initialBackoff(opts Options) time.Duration {
	return opts.InitialInterval
}

func calculateBackoff(retry int, current time.Duration, opts Options) time.Duration {
	// Calculate next interval with exponential backoff
	next := float64(current) * opts.Multiplier

	// Add randomization
	randomized := next * (1 + opts.RandomizationFactor*(2*rand.Float64()-1))

	// Cap at max interval
	if randomized > float64(opts.MaxInterval) {
		randomized = float64(opts.MaxInterval)
	}

	return time.Duration(randomized)
}
```

```go
// internal/client/alphavantage.go
// Add this method to the AlphaVantage client

// GetStockDataWithRetry retrieves stock data with retry logic
func (c *AlphaVantage) GetStockDataWithRetry(ctx context.Context, symbol string, days int) (*models.AlphaVantageResponse, error) {
	var result *models.AlphaVantageResponse

	// Define the operation to retry
	operation := func() error {
		var err error
		result, err = c.GetStockData(ctx, symbol, days)
		return err
	}

	// Define which errors should be retried
	shouldRetry := func(err error) bool {
		return isTransientError(err)
	}

	// Configure retry options
	opts := retry.DefaultOptions()

	// Execute with retry
	err := retry.Do(ctx, operation, shouldRetry, opts)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// isTransientError determines if an error is transient and worth retrying
func isTransientError(err error) bool {
	// Check for network errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		// Retry network timeouts and temporary errors
		return netErr.Timeout() || netErr.Temporary()
	}

	// Check for HTTP response status codes
	var apiErr *ApiError
	if errors.As(err, &apiErr) {
		// Retry 429 (Too Many Requests) and 5xx errors
		statusCode := apiErr.StatusCode
		return statusCode == 429 || (statusCode >= 500 && statusCode < 600)
	}

	// Don't retry other errors
	return false
}

// ApiError represents an error from the API
type ApiError struct {
	StatusCode int
	Message    string
}

func (e *ApiError) Error() string {
	return fmt.Sprintf("API error (status %d): %s", e.StatusCode, e.Message)
}
```

## 2. Circuit Breaker Pattern

### Problem
Persistently failing external services can overwhelm an application with timeouts and error handling. The circuit breaker pattern prevents cascading failures by "tripping" after a certain number of failures and temporarily rejecting requests rather than attempting to call a failing service.

### Implementation Location
Create a new package at `internal/util/circuitbreaker/circuitbreaker.go` and update `internal/client/alphavantage.go`.

### Required Implementation

```go
// internal/util/circuitbreaker/circuitbreaker.go
package circuitbreaker

import (
	"errors"
	"sync"
	"time"
)

// State represents the state of the circuit breaker
type State int

const (
	// StateClosed means the circuit breaker is closed (allowing requests)
	StateClosed State = iota
	// StateOpen means the circuit breaker is open (preventing requests)
	StateOpen
	// StateHalfOpen means the circuit breaker is half-open (allowing a test request)
	StateHalfOpen
)

// String returns a string representation of the state
func (s State) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// Counts tracks success and failure counts
type Counts struct {
	Successes     int64
	Failures      int64
	ConsecFailures int64
	TotalRequests int64
}

// Settings configures the circuit breaker
type Settings struct {
	// Name is a descriptive name for the circuit breaker
	Name string

	// MaxRequests is the maximum number of requests allowed when the circuit breaker is half-open
	MaxRequests int64

	// Timeout is how long to wait before transitioning from open to half-open
	Timeout time.Duration

	// ReadyToTrip is called when a request fails
	// It should return true if the circuit breaker should trip
	ReadyToTrip func(counts Counts) bool

	// OnStateChange is called when the circuit breaker state changes
	OnStateChange func(name string, from, to State)
}

// DefaultSettings returns default circuit breaker settings
func DefaultSettings(name string) Settings {
	return Settings{
		Name:        name,
		MaxRequests: 1,
		Timeout:     60 * time.Second,
		ReadyToTrip: func(counts Counts) bool {
			// Trip after 5 consecutive failures
			return counts.ConsecFailures >= 5
		},
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	name        string
	maxRequests int64
	timeout     time.Duration
	readyToTrip func(counts Counts) bool
	onStateChange func(name string, from, to State)

	mu            sync.Mutex
	state         State
	counts        Counts
	expiry        time.Time
	halfOpenCount int64
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(settings Settings) *CircuitBreaker {
	return &CircuitBreaker{
		name:          settings.Name,
		maxRequests:   settings.MaxRequests,
		timeout:       settings.Timeout,
		readyToTrip:   settings.ReadyToTrip,
		onStateChange: settings.OnStateChange,
		state:         StateClosed,
	}
}

// Execute runs a function with circuit breaker protection
func (cb *CircuitBreaker) Execute(req func() (interface{}, error)) (interface{}, error) {
	generation, err := cb.beforeRequest()
	if err != nil {
		return nil, err
	}

	defer func() {
		if r := recover(); r != nil {
			// Handle panic
			cb.afterRequest(generation, false)
			panic(r)
		}
	}()

	result, err := req()
	cb.afterRequest(generation, err == nil)

	return result, err
}

// Trip manually trips the circuit breaker
func (cb *CircuitBreaker) Trip() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == StateClosed {
		cb.setState(StateOpen)
	}
}

// Reset manually resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.setState(StateClosed)
	cb.counts = Counts{}
}

// State returns the current state of the circuit breaker
func (cb *CircuitBreaker) State() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	return cb.state
}

// Counts returns the current counts
func (cb *CircuitBreaker) Counts() Counts {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	return cb.counts
}

// Internal methods

// beforeRequest is called before the protected call
func (cb *CircuitBreaker) beforeRequest() (uint64, error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Update state if needed
	if cb.state == StateOpen {
		if time.Now().After(cb.expiry) {
			cb.setState(StateHalfOpen)
		} else {
			return 0, errors.New("circuit breaker is open")
		}
	}

	// Check if we can allow a request in half-open state
	if cb.state == StateHalfOpen && cb.halfOpenCount >= cb.maxRequests {
		return 0, errors.New("circuit breaker is half-open and at max requests")
	}

	// Generate a new count for this request
	cb.counts.TotalRequests++
	generation := cb.counts.TotalRequests

	if cb.state == StateHalfOpen {
		cb.halfOpenCount++
	}

	return generation, nil
}

// afterRequest is called after the protected call
func (cb *CircuitBreaker) afterRequest(generation uint64, success bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if success {
		// Handle success
		cb.onSuccess()
	} else {
		// Handle failure
		cb.onFailure()
	}
}

// onSuccess handles a successful call
func (cb *CircuitBreaker) onSuccess() {
	cb.counts.Successes++
	cb.counts.ConsecFailures = 0

	if cb.state == StateHalfOpen {
		// If we've had enough successes in half-open state, close the circuit
		if cb.halfOpenCount >= cb.maxRequests {
			cb.setState(StateClosed)
			cb.halfOpenCount = 0
		}
	}
}

// onFailure handles a failed call
func (cb *CircuitBreaker) onFailure() {
	cb.counts.Failures++
	cb.counts.ConsecFailures++

	// Check if circuit should trip
	if cb.state == StateClosed && cb.readyToTrip(cb.counts) {
		cb.setState(StateOpen)
	} else if cb.state == StateHalfOpen {
		// Any failure in half-open state should trip the circuit
		cb.setState(StateOpen)
	}
}

// setState changes the circuit breaker state
func (cb *CircuitBreaker) setState(newState State) {
	if cb.state == newState {
		return
	}

	oldState := cb.state
	cb.state = newState

	if newState == StateOpen {
		cb.expiry = time.Now().Add(cb.timeout)
		cb.halfOpenCount = 0
	}

	if cb.onStateChange != nil {
		cb.onStateChange(cb.name, oldState, newState)
	}
}
```

```go
// internal/client/alphavantage.go
// Update the client struct and initializer

type AlphaVantage struct {
	apiKey     string
	httpClient *http.Client
	cb         *circuitbreaker.CircuitBreaker
}

// NewAlphaVantage creates a new AlphaVantage API client
func NewAlphaVantage(apiKey string) *AlphaVantage {
	// Configure circuit breaker
	settings := circuitbreaker.DefaultSettings("alphavantage")
	settings.Timeout = 1 * time.Minute
	settings.ReadyToTrip = func(counts circuitbreaker.Counts) bool {
		// Trip after 5 consecutive failures or if failure rate exceeds 50% of at least 20 requests
		if counts.ConsecFailures >= 5 {
			return true
		}

		if counts.TotalRequests >= 20 {
			failureRatio := float64(counts.Failures) / float64(counts.TotalRequests)
			return failureRatio >= 0.5
		}

		return false
	}
	settings.OnStateChange = func(name string, from, to circuitbreaker.State) {
		log.Printf("Circuit breaker '%s' changed from '%s' to '%s'", name, from, to)
	}

	return &AlphaVantage{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		cb: circuitbreaker.NewCircuitBreaker(settings),
	}
}

// GetStockData with circuit breaker
func (c *AlphaVantage) GetStockData(ctx context.Context, symbol string, days int) (*models.AlphaVantageResponse, error) {
	// Execute the API call through the circuit breaker
	result, err := c.cb.Execute(func() (interface{}, error) {
		// Call the actual API method
		return c.getStockDataInternal(ctx, symbol, days)
	})

	if err != nil {
		return nil, err
	}

	// Type assertion
	return result.(*models.AlphaVantageResponse), nil
}

// getStockDataInternal does the actual API call
func (c *AlphaVantage) getStockDataInternal(ctx context.Context, symbol string, days int) (*models.AlphaVantageResponse, error) {
	// Original implementation of GetStockData
	// ...
}
```

## 3. Rate Limiting

### Problem
External APIs like Alpha Vantage typically have rate limits. Exceeding these limits can result in requests being rejected or even an account being temporarily suspended. The application should respect these limits to ensure reliable service.

### Implementation Location
Create a new package at `internal/util/ratelimit/ratelimit.go` and update `internal/client/alphavantage.go`.

### Required Implementation

```go
// internal/util/ratelimit/ratelimit.go
package ratelimit

import (
	"context"
	"sync"
	"time"
)

// Limiter provides rate limiting functionality
type Limiter struct {
	rate       int           // requests per period
	period     time.Duration // time period
	tokens     int           // current token count
	lastRefill time.Time     // last token refill time
	mu         sync.Mutex    // mutex for thread safety
}

// NewLimiter creates a new rate limiter
func NewLimiter(rate int, period time.Duration) *Limiter {
	return &Limiter{
		rate:       rate,
		period:     period,
		tokens:     rate, // Start with full tokens
		lastRefill: time.Now(),
	}
}

// Allow checks if a request is allowed immediately
func (l *Limiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.refill()

	if l.tokens > 0 {
		l.tokens--
		return true
	}

	return false
}

// Wait waits until a request is allowed or the context is canceled
func (l *Limiter) Wait(ctx context.Context) error {
	for {
		l.mu.Lock()
		l.refill()

		if l.tokens > 0 {
			l.tokens--
			l.mu.Unlock()
			return nil
		}

		// Calculate time until next token
		waitTime := l.timeUntilNextToken()
		l.mu.Unlock()

		// Wait with context awareness
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
			// Continue and check again
		}
	}
}

// refill refills tokens based on elapsed time
func (l *Limiter) refill() {
	now := time.Now()
	elapsed := now.Sub(l.lastRefill)

	// Calculate how many tokens to add
	newTokens := int(float64(elapsed) / float64(l.period) * float64(l.rate))
	if newTokens > 0 {
		l.tokens = min(l.tokens+newTokens, l.rate)
		l.lastRefill = now
	}
}

// timeUntilNextToken calculates time until next token is available
func (l *Limiter) timeUntilNextToken() time.Duration {
	if l.tokens > 0 {
		return 0
	}

	elapsedSinceRefill := time.Since(l.lastRefill)
	tokenRefillInterval := l.period / time.Duration(l.rate)
	return tokenRefillInterval - (elapsedSinceRefill % tokenRefillInterval)
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
```

```go
// internal/client/alphavantage.go
// Add rate limiting to the client struct

type AlphaVantage struct {
	apiKey     string
	httpClient *http.Client
	cb         *circuitbreaker.CircuitBreaker
	limiter    *ratelimit.Limiter
}

// Update the constructor
func NewAlphaVantage(apiKey string) *AlphaVantage {
	// ... existing circuit breaker setup

	// Add rate limiter
	// Alpha Vantage free tier: 5 requests per minute
	limiter := ratelimit.NewLimiter(5, time.Minute)

	return &AlphaVantage{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		cb:         circuitbreaker.NewCircuitBreaker(settings),
		limiter:    limiter,
	}
}

// Update the internal API method to respect rate limits
func (c *AlphaVantage) getStockDataInternal(ctx context.Context, symbol string, days int) (*models.AlphaVantageResponse, error) {
	// Wait for rate limiter
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait error: %w", err)
	}

	// Original implementation
	// ...
}
```

## 4. Timeout Management

### Problem
The current implementation sets a timeout on the HTTP client, but doesn't properly propagate context timeouts or handle cancellation. This can lead to resource leaks and zombie goroutines.

### Implementation Location
Update all client methods in `internal/client/alphavantage.go`.

### Required Implementation

```go
// internal/client/alphavantage.go

// GetStockData with proper context and timeout handling
func (c *AlphaVantage) getStockDataInternal(ctx context.Context, symbol string, days int) (*models.AlphaVantageResponse, error) {
	// Parameter validation
	if symbol == "" {
		return nil, errors.New("symbol cannot be empty")
	}

	// Create the request with context
	params := url.Values{}
	params.Add("apikey", c.apiKey)
	params.Add("function", function)
	params.Add("symbol", symbol)

	// Set output size
	if days > compactOutputSizeLimit {
		params.Add("outputsize", outputSizeFull)
	} else {
		params.Add("outputsize", outputSizeCompact)
	}

	reqURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// Make the request
	start := time.Now()
	resp, err := c.httpClient.Do(req)
	duration := time.Since(start)

	// Record metrics
	if metrics := c.metrics; metrics != nil {
		metrics.ExternalAPILatency.WithLabelValues("alphavantage").Observe(duration.Seconds())

		if err != nil {
			metrics.ExternalAPIRequests.WithLabelValues("alphavantage", "error").Inc()
			// Classify error type
			var netErr net.Error
			if errors.As(err, &netErr) {
				if netErr.Timeout() {
					metrics.ExternalAPIErrors.WithLabelValues("alphavantage", "timeout").Inc()
				} else {
					metrics.ExternalAPIErrors.WithLabelValues("alphavantage", "network").Inc()
				}
			} else {
				metrics.ExternalAPIErrors.WithLabelValues("alphavantage", "other").Inc()
			}
		} else {
			metrics.ExternalAPIRequests.WithLabelValues("alphavantage", "success").Inc()
		}
	}

	// Handle request errors
	if err != nil {
		return nil, fmt.Errorf("error making request to Alpha Vantage: %w", err)
	}
	defer resp.Body.Close()

	// Handle non-200 responses
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		errorMsg := string(bodyBytes)

		apiErr := &ApiError{
			StatusCode: resp.StatusCode,
			Message:    errorMsg,
		}

		return nil, apiErr
	}

	// Parse the response
	var result models.AlphaVantageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("error decoding Alpha Vantage response: %w", err)
	}

	// Check for API error conditions in the response body
	if result.TimeSeries == nil || len(result.TimeSeries) == 0 {
		if result.ErrorMessage != "" {
			return nil, fmt.Errorf("Alpha Vantage API error: %s", result.ErrorMessage)
		}
		return nil, fmt.Errorf("no data returned from Alpha Vantage, possibly invalid symbol or API key")
	}

	return &result, nil
}
```

## 5. Graceful Degradation

### Problem
The application currently has no way to degrade gracefully when external dependencies are unavailable. It should provide meaningful responses even when the external API is down.

### Implementation Location
Update `internal/service/stock.go` to implement fallback mechanisms.

### Required Implementation

```go
// internal/service/stock.go

// GetStockData with graceful degradation
func (s *StockService) GetStockData(ctx context.Context) (*models.StockData, error) {
	cacheKey := s.config.Symbol

	// Try to get data from cache first
	if cachedData, found := s.cache.Get(cacheKey); found {
		// Increment metrics
		s.metrics.StockDataFetches.WithLabelValues(s.config.Symbol, "cache").Inc()

		return cachedData.(*models.StockData), nil
	}

	// Not in cache, try to get from API
	apiResponse, err := s.client.GetStockDataWithRetry(ctx, s.config.Symbol, s.config.NDays)

	// Handle API errors with fallback strategies
	if err != nil {
		// Increment metrics
		s.metrics.StockDataFetches.WithLabelValues(s.config.Symbol, "fallback").Inc()

		// Log the error
		s.logger.Error("Error fetching stock data from API", err, logging.Fields{
			"symbol": s.config.Symbol,
			"days":   s.config.NDays,
		})

		// Try fallback strategies

		// 1. Try to get slightly older data from cache (with a longer expiry)
		if fallbackData, found := s.fallbackCache.Get(cacheKey); found {
			s.logger.Info("Using fallback data from cache", logging.Fields{
				"symbol": s.config.Symbol,
			})
			return fallbackData.(*models.StockData), nil
		}

		// 2. Try to use default/sample data for demo/testing
		if s.config.EnableTestData && s.isCommonSymbol(s.config.Symbol) {
			s.logger.Info("Using sample data for common symbol", logging.Fields{
				"symbol": s.config.Symbol,
			})
			return s.getSampleStockData(s.config.Symbol)
		}

		// No fallbacks available, return the original error
		return nil, NewExternalAPIError(err, s.config.Symbol)
	}

	// Increment metrics
	s.metrics.StockDataFetches.WithLabelValues(s.config.Symbol, "api").Inc()

	// Process the API response
	stockData, err := s.processAPIResponse(apiResponse)
	if err != nil {
		return nil, err
	}

	// Cache the response in both caches
	cacheDuration := s.determineCacheDuration()
	s.cache.Set(cacheKey, stockData, cacheDuration)

	// Store in fallback cache with a longer duration
	s.fallbackCache.Set(cacheKey, stockData, 24*time.Hour)

	// Record average price metric
	s.metrics.StockPriceAverage.WithLabelValues(s.config.Symbol).Set(stockData.Average)

	return stockData, nil
}

// isCommonSymbol checks if a symbol is a common one that might have sample data
func (s *StockService) isCommonSymbol(symbol string) bool {
	commonSymbols := map[string]bool{
		"AAPL": true,
		"MSFT": true,
		"GOOG": true,
		"AMZN": true,
		"FB":   true,
		"TSLA": true,
		"IBM":  true,
	}

	return commonSymbols[symbol]
}

// getSampleStockData returns sample data for common symbols
func (s *StockService) getSampleStockData(symbol string) (*models.StockData, error) {
	// In a real implementation, this would load from a file or embedded data
	// For simplicity, we're generating sample data here

	now := time.Now()
	prices := make([]models.StockPrice, s.config.NDays)
	total := 0.0

	// Base price depends on the symbol
	basePrice := 100.0
	switch symbol {
	case "AAPL":
		basePrice = 150.0
	case "MSFT":
		basePrice = 250.0
	case "GOOG":
		basePrice = 2000.0
	case "AMZN":
		basePrice = 3000.0
	case "TSLA":
		basePrice = 800.0
	case "IBM":
		basePrice = 125.0
	}

	// Generate sample prices with some randomness
	for i := 0; i < s.config.NDays; i++ {
		date := now.AddDate(0, 0, -i).Format("2006-01-02")

		// Add some randomness to the price (±5%)
		variance := basePrice * 0.05 * (2*rand.Float64() - 1)
		price := basePrice + variance

		prices[i] = models.StockPrice{
			Date:  date,
			Close: price,
		}

		total += price
	}

	average := total / float64(s.config.NDays)

	return &models.StockData{
		Symbol:  symbol,
		Prices:  prices,
		Average: average,
	}, nil
}
```

## 6. Integration in main.go

### Implementation Location
Update `cmd/stockticker/main.go` to wire up the resilience components.

### Required Implementation

```go
func main() {
	// ... existing initialization code

	// Configure API client with resilience features
	apiClient := client.NewAlphaVantage(
		cfg.APIKey,
		client.WithMetrics(metricsService),
		client.WithLogger(logger),
		client.WithCircuitBreaker(true),
		client.WithRateLimiter(5, time.Minute), // 5 requests per minute
		client.WithRetries(3),                 // 3 retries with exponential backoff
	)

	// Create primary and fallback caches
	primaryCache := cache.New(
		cache.WithMaxItems(1000),
		cache.WithPrometheusMetrics("primary_cache"),
	)

	fallbackCache := cache.New(
		cache.WithMaxItems(10000),
		cache.WithPrometheusMetrics("fallback_cache"),
	)

	// Create service with fallback capability
	stockService := service.New(
		cfg,
		apiClient,
		primaryCache,
		service.WithFallbackCache(fallbackCache),
		service.WithLogger(logger),
		service.WithMetrics(metricsService),
	)

	// ... rest of initialization code

	// Add circuit breaker health check
	healthChecker.AddCheck("circuit_breaker", func(ctx context.Context) *health.Check {
		cbState := apiClient.CircuitBreakerState()

		check := &health.Check{
			Name: "circuit_breaker",
			Time: time.Now(),
			Data: map[string]interface{}{
				"state":         cbState.String(),
				"total_requests": apiClient.CircuitBreakerCounts().TotalRequests,
				"failures":      apiClient.CircuitBreakerCounts().Failures,
			},
		}

		if cbState == circuitbreaker.StateOpen {
			check.Status = health.StatusDown
			check.Message = "Circuit breaker is open"
		} else {
			check.Status = health.StatusUp
			check.Message = "Circuit breaker is closed or half-open"
		}

		return check
	})

	// ... rest of the code
}
```

## Benefits

1. **Improved Stability**: Retries and circuit breakers prevent cascading failures
2. **API Protection**: Rate limiting prevents quota exhaustion
3. **Graceful Degradation**: Fallback mechanisms provide a better user experience during outages
4. **Resource Management**: Proper context handling prevents resource leaks
5. **Better Error Handling**: More specific error types improve debugging and client experience
6. **Self-Healing**: Circuit breakers automatically recover from temporary outages

## Codebase Analysis

The current stockticker service lacks several critical resilience patterns, making it vulnerable to failures in its external dependencies. The proposed improvements add multiple layers of protection, making the service much more robust in the face of transient failures, API rate limits, and other challenges encountered in production environments.