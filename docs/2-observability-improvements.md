# Observability Improvements

## Purpose
Observability is critical for production services, allowing teams to understand system behavior, diagnose issues, and make data-driven decisions about performance optimizations. The current stockticker service lacks comprehensive observability features, making it difficult to monitor its health and troubleshoot problems in production.

## 1. Structured Logging

### Problem
The existing logging in the application uses Go's standard `log` package with unstructured text messages, making it difficult to parse, filter, and analyze logs effectively.

### Implementation Location
Create a new logging package at `internal/observability/logging/logger.go`.

### Required Implementation

```go
package logging

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
)

// Logger is a structured logger for the application
type Logger struct {
	log zerolog.Logger
}

// Fields is a map of key-value pairs to add to log entries
type Fields map[string]interface{}

// Level represents the level of a log message
type Level = zerolog.Level

// Log levels
const (
	DebugLevel = zerolog.DebugLevel
	InfoLevel  = zerolog.InfoLevel
	WarnLevel  = zerolog.WarnLevel
	ErrorLevel = zerolog.ErrorLevel
	FatalLevel = zerolog.FatalLevel
	PanicLevel = zerolog.PanicLevel
)

// contextKey is the type for context keys
type contextKey string

// RequestIDKey is the context key for request IDs
const RequestIDKey = contextKey("requestID")

// New creates a new logger
func New(opts ...Option) *Logger {
	// Default options
	output := os.Stdout
	level := InfoLevel
	timeFormat := time.RFC3339

	// Create basic logger
	l := zerolog.New(output).With().Timestamp().Logger().Level(level)

	// Format timestamps properly
	zerolog.TimeFieldFormat = timeFormat

	// Apply options
	for _, opt := range opts {
		l = opt(l)
	}

	return &Logger{log: l}
}

// Option is a function that configures the logger
type Option func(zerolog.Logger) zerolog.Logger

// WithOutput sets the output writer
func WithOutput(w io.Writer) Option {
	return func(l zerolog.Logger) zerolog.Logger {
		return l.Output(w)
	}
}

// WithLevel sets the minimum log level
func WithLevel(level Level) Option {
	return func(l zerolog.Logger) zerolog.Logger {
		return l.Level(level)
	}
}

// WithFields adds fields to the logger
func WithFields(fields Fields) Option {
	return func(l zerolog.Logger) zerolog.Logger {
		ctx := l.With()
		for k, v := range fields {
			ctx = ctx.Interface(k, v)
		}
		return ctx.Logger()
	}
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, fields ...Fields) {
	l.log.Debug().Fields(mergeFields(fields...)).Msg(msg)
}

// Info logs an info message
func (l *Logger) Info(msg string, fields ...Fields) {
	l.log.Info().Fields(mergeFields(fields...)).Msg(msg)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, fields ...Fields) {
	l.log.Warn().Fields(mergeFields(fields...)).Msg(msg)
}

// Error logs an error message
func (l *Logger) Error(msg string, err error, fields ...Fields) {
	merged := mergeFields(fields...)
	if err != nil {
		merged["error"] = err.Error()
	}
	l.log.Error().Fields(merged).Msg(msg)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(msg string, err error, fields ...Fields) {
	merged := mergeFields(fields...)
	if err != nil {
		merged["error"] = err.Error()
	}
	l.log.Fatal().Fields(merged).Msg(msg)
}

// WithContext returns a logger with request ID from context
func (l *Logger) WithContext(ctx context.Context) *Logger {
	// Copy the logger
	newLogger := *l

	// Add request ID from context if available
	if reqID, ok := ctx.Value(RequestIDKey).(string); ok {
		newLogger.log = l.log.With().Str("request_id", reqID).Logger()
	}

	return &newLogger
}

// For internal use - merges multiple fields maps
func mergeFields(fieldsArr ...Fields) map[string]interface{} {
	result := make(map[string]interface{})
	for _, fields := range fieldsArr {
		for k, v := range fields {
			result[k] = v
		}
	}
	return result
}
```

## 2. Prometheus Metrics Integration

### Problem
The service doesn't expose metrics for monitoring system health, performance, and business indicators.

### Implementation Location
Create a new metrics package at `internal/observability/metrics/metrics.go`.

### Required Implementation

```go
package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics contains all the service metrics
type Metrics struct {
	// Request metrics
	RequestCounter     *prometheus.CounterVec
	RequestDuration    *prometheus.HistogramVec

	// Business metrics
	StockDataFetches   *prometheus.CounterVec
	StockPriceAverage  *prometheus.GaugeVec

	// External API metrics
	ExternalAPIRequests *prometheus.CounterVec
	ExternalAPILatency  *prometheus.HistogramVec
	ExternalAPIErrors   *prometheus.CounterVec

	// System metrics
	SysGoroutines      prometheus.Gauge
	SysMemoryUsage     prometheus.Gauge
}

// New creates and registers all metrics
func New() *Metrics {
	m := &Metrics{
		// HTTP request metrics
		RequestCounter: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "stockticker_http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "endpoint", "status"},
		),

		RequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "stockticker_http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "endpoint"},
		),

		// Business metrics
		StockDataFetches: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "stockticker_stock_data_fetches_total",
				Help: "Total number of stock data fetches",
			},
			[]string{"symbol", "source"}, // source: "cache" or "api"
		),

		StockPriceAverage: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "stockticker_stock_price_average",
				Help: "Average stock price over the requested time period",
			},
			[]string{"symbol"},
		),

		// External API metrics
		ExternalAPIRequests: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "stockticker_external_api_requests_total",
				Help: "Total number of external API requests",
			},
			[]string{"api", "status"}, // status: "success" or "error"
		),

		ExternalAPILatency: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "stockticker_external_api_latency_seconds",
				Help:    "External API request latency in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"api"},
		),

		ExternalAPIErrors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "stockticker_external_api_errors_total",
				Help: "Total number of external API errors",
			},
			[]string{"api", "error_type"},
		),

		// System metrics
		SysGoroutines: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "stockticker_goroutines",
				Help: "Current number of goroutines",
			},
		),

		SysMemoryUsage: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "stockticker_memory_bytes",
				Help: "Current memory usage in bytes",
			},
		),
	}

	return m
}

// Handler returns an HTTP handler for the metrics endpoint
func Handler() http.Handler {
	return promhttp.Handler()
}

// InstrumentHandler wraps an HTTP handler with metrics middleware
func (m *Metrics) InstrumentHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap the response writer to capture status code
		wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		// Call the next handler
		next.ServeHTTP(wrapped, r)

		// Record metrics
		duration := time.Since(start).Seconds()
		m.RequestCounter.WithLabelValues(r.Method, r.URL.Path, http.StatusText(wrapped.status)).Inc()
		m.RequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	status int
}

// WriteHeader captures the status code before writing it
func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// Write captures 200 status if WriteHeader was not called
func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.status == 0 {
		rw.status = http.StatusOK
	}
	return rw.ResponseWriter.Write(b)
}
```

## 3. Health Check Endpoint

### Problem
The current health check endpoint is too simplistic and doesn't provide meaningful information about the application's health and its dependencies.

### Implementation Location
Create a new health package at `internal/observability/health/health.go`.

### Required Implementation

```go
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Status represents the health status of a component
type Status string

// Health statuses
const (
	StatusUp   Status = "UP"
	StatusDown Status = "DOWN"
)

// Check represents a health check for a component
type Check struct {
	Name    string      `json:"name"`
	Status  Status      `json:"status"`
	Message string      `json:"message,omitempty"`
	Error   string      `json:"error,omitempty"`
	Time    time.Time   `json:"time"`
	Data    interface{} `json:"data,omitempty"`
}

// Response represents a health check response
type Response struct {
	Status  Status            `json:"status"`
	Checks  map[string]*Check `json:"checks"`
	Version string            `json:"version,omitempty"`
}

// CheckFunc is a function that performs a health check
type CheckFunc func(ctx context.Context) *Check

// Health manages health checks for the application
type Health struct {
	checks  map[string]CheckFunc
	mu      sync.RWMutex
	version string
}

// Option is a function that configures the health checker
type Option func(*Health)

// WithVersion sets the application version
func WithVersion(version string) Option {
	return func(h *Health) {
		h.version = version
	}
}

// New creates a new health checker
func New(opts ...Option) *Health {
	h := &Health{
		checks: make(map[string]CheckFunc),
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
}

// AddCheck adds a health check
func (h *Health) AddCheck(name string, check CheckFunc) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checks[name] = check
}

// Handler returns an HTTP handler for the health endpoint
func (h *Health) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Use request context with timeout
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		response := h.Check(ctx)

		// Set status code based on health
		if response.Status == StatusDown {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})
}

// Check performs all health checks and returns the result
func (h *Health) Check(ctx context.Context) *Response {
	h.mu.RLock()
	defer h.mu.RUnlock()

	response := &Response{
		Status:  StatusUp,
		Checks:  make(map[string]*Check),
		Version: h.version,
	}

	if len(h.checks) == 0 {
		// If no checks are registered, report UP
		return response
	}

	// Run all checks in parallel
	var wg sync.WaitGroup
	var mu sync.Mutex

	for name, checkFunc := range h.checks {
		wg.Add(1)
		go func(name string, fn CheckFunc) {
			defer wg.Done()

			check := fn(ctx)

			mu.Lock()
			response.Checks[name] = check
			if check.Status == StatusDown {
				response.Status = StatusDown
			}
			mu.Unlock()
		}(name, checkFunc)
	}

	wg.Wait()
	return response
}

// Common health checks

// DatabaseCheck creates a health check for database connectivity
func DatabaseCheck(db interface{ Ping(context.Context) error }) CheckFunc {
	return func(ctx context.Context) *Check {
		start := time.Now()
		err := db.Ping(ctx)
		duration := time.Since(start)

		check := &Check{
			Name: "database",
			Time: time.Now(),
			Data: map[string]interface{}{
				"latency_ms": duration.Milliseconds(),
			},
		}

		if err != nil {
			check.Status = StatusDown
			check.Message = "Database connectivity issue"
			check.Error = err.Error()
		} else {
			check.Status = StatusUp
			check.Message = "Database is reachable"
		}

		return check
	}
}

// ExternalAPICheck creates a health check for an external API
func ExternalAPICheck(name string, fn func(context.Context) error) CheckFunc {
	return func(ctx context.Context) *Check {
		start := time.Now()
		err := fn(ctx)
		duration := time.Since(start)

		check := &Check{
			Name: name,
			Time: time.Now(),
			Data: map[string]interface{}{
				"latency_ms": duration.Milliseconds(),
			},
		}

		if err != nil {
			check.Status = StatusDown
			check.Message = "API connectivity issue"
			check.Error = err.Error()
		} else {
			check.Status = StatusUp
			check.Message = "API is reachable"
		}

		return check
	}
}
```

## 4. Custom HTTP Middleware for Request Tracing

### Problem
There's no way to trace requests through the system or collect per-request performance data.

### Implementation Location
Create a new middleware package at `internal/observability/middleware/tracing.go`.

### Required Implementation

```go
package middleware

import (
	"net/http"
	"time"

	"github.com/saedabdu/stockticker/internal/observability/logging"
)

// Tracing adds request ID and duration tracking to all requests
func Tracing(logger *logging.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get or create request ID
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = newRequestID()
			}

			// Add request ID to response headers
			w.Header().Set("X-Request-ID", requestID)

			// Create a context with request ID
			ctx := context.WithValue(r.Context(), logging.RequestIDKey, requestID)
			r = r.WithContext(ctx)

			// Log request start
			path := r.URL.Path
			start := time.Now()

			// Wrap response writer to capture status code
			ww := &responseWriter{ResponseWriter: w, status: http.StatusOK}

			// Log basic request info
			logger.WithContext(ctx).Info("Request started",
				logging.Fields{
					"method":     r.Method,
					"path":       path,
					"user_agent": r.UserAgent(),
					"remote_ip":  getRemoteIP(r),
				},
			)

			// Process the request
			next.ServeHTTP(ww, r)

			// Calculate duration
			duration := time.Since(start)

			// Log request completion
			logger.WithContext(ctx).Info("Request completed",
				logging.Fields{
					"method":     r.Method,
					"path":       path,
					"status":     ww.status,
					"duration_ms": duration.Milliseconds(),
				},
			)
		})
	}
}

// Create a new unique request ID
func newRequestID() string {
	// Use UUID v4 (random)
	return uuid.New().String()
}

// Get the client's IP address
func getRemoteIP(r *http.Request) string {
	// Check for X-Forwarded-For header first (for proxied requests)
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		return forwarded
	}

	// Otherwise, use RemoteAddr
	return r.RemoteAddr
}

// responseWriter is defined in the metrics package and should be moved to a common location
```

## 5. Integration in main.go

### Implementation Location
Update `cmd/stockticker/main.go` to use the observability features.

### Required Implementation

```go
package main

import (
	// existing imports

	"github.com/saedabdu/stockticker/internal/observability/health"
	"github.com/saedabdu/stockticker/internal/observability/logging"
	"github.com/saedabdu/stockticker/internal/observability/metrics"
	"github.com/saedabdu/stockticker/internal/observability/middleware"
	"runtime"
)

// Version information (set during build)
var (
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	// Initialize logger
	logger := logging.New(
		logging.WithLevel(logging.InfoLevel),
		logging.WithFields(logging.Fields{
			"service": "stockticker",
			"version": Version,
		}),
	)

	// Initialize metrics
	metricsService := metrics.New()

	// Initialize health checker
	healthChecker := health.New(
		health.WithVersion(Version),
	)

	// Load configuration
	cfg, err := config.New()
	if err != nil {
		logger.Fatal("Error loading configuration", err)
	}

	// Create API client
	apiClient := client.NewAlphaVantage(cfg.APIKey)

	// Add external API health check
	healthChecker.AddCheck("alphavantage", health.ExternalAPICheck("alphavantage", func(ctx context.Context) error {
		// Simple ping implementation - could be added to the client
		return apiClient.Ping(ctx)
	}))

	// Create cache
	cacheInstance := cache.New()

	// Create service
	stockService := service.New(cfg, apiClient, cacheInstance)

	// Create handler
	stockHandler := handler.NewStockHandler(stockService)

	// Create router (using a more flexible router like gorilla/mux)
	router := mux.NewRouter()

	// Add API routes
	router.HandleFunc("/stocks", stockHandler.HandleStocks).Methods("GET")

	// Add observability endpoints
	router.Handle("/metrics", metrics.Handler())
	router.Handle("/health", healthChecker.Handler())

	// Create middleware chain
	// Order matters: tracing should be first, then metrics
	handler := middleware.Tracing(logger)(
		metricsService.InstrumentHandler(router),
	)

	// Start system metrics collection
	go collectSystemMetrics(metricsService)

	// Start HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		logger.Info("Starting server", logging.Fields{
			"port":   cfg.Port,
			"symbol": cfg.Symbol,
			"ndays":  cfg.NDays,
		})

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server error", err)
		}
	}()

	// Setup graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Gracefully shut down the server
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", err)
	}

	logger.Info("Server exited gracefully")
}

// Collect system metrics periodically
func collectSystemMetrics(m *metrics.Metrics) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Update goroutine count
			m.SysGoroutines.Set(float64(runtime.NumGoroutine()))

			// Update memory stats
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)
			m.SysMemoryUsage.Set(float64(memStats.Alloc))
		}
	}
}
```

## 6. Unit Testing the Observability Components

### Implementation Location
Create tests at `internal/observability/logging/logger_test.go` and similar locations.

### Required Implementation (Example)

```go
package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func TestLoggerBasic(t *testing.T) {
	// Create a buffer to capture log output
	var buf bytes.Buffer

	// Create logger writing to buffer
	logger := New(
		WithOutput(&buf),
		WithLevel(DebugLevel),
	)

	// Log a message
	logger.Info("test message", Fields{"key": "value"})

	// Parse the JSON output
	var entry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	// Check fields
	if entry["message"] != "test message" {
		t.Errorf("Expected message 'test message', got %v", entry["message"])
	}

	if entry["level"] != "info" {
		t.Errorf("Expected level 'info', got %v", entry["level"])
	}

	if entry["key"] != "value" {
		t.Errorf("Expected key 'value', got %v", entry["key"])
	}
}

func TestLoggerWithContext(t *testing.T) {
	// Create a buffer to capture log output
	var buf bytes.Buffer

	// Create logger writing to buffer
	logger := New(
		WithOutput(&buf),
		WithLevel(InfoLevel),
	)

	// Create context with request ID
	ctx := context.WithValue(context.Background(), RequestIDKey, "test-request-id")

	// Get logger with context
	ctxLogger := logger.WithContext(ctx)

	// Log a message
	ctxLogger.Info("test message")

	// Parse the JSON output
	var entry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	// Check request ID
	if entry["request_id"] != "test-request-id" {
		t.Errorf("Expected request_id 'test-request-id', got %v", entry["request_id"])
	}
}
```

## Benefits

1. **Better Debugging**: Structured logs with context make it easier to trace issues through the system
2. **Performance Monitoring**: Metrics provide insights into system behavior and performance
3. **Dependency Monitoring**: Health checks ensure all system components are functioning correctly
4. **Proactive Alerting**: Metrics can be used to set up alerts for potential issues
5. **Request Tracing**: Every request gets a unique ID that can be traced through the system
6. **Performance Optimization**: Data on slow requests helps identify bottlenecks
7. **Capacity Planning**: System metrics help plan for scaling and resource allocation

## Codebase Analysis

The current stockticker service has minimal observability features, with basic logging and a simple health check endpoint. The proposed improvements add structured logging, comprehensive metrics, detailed health checks, and request tracing, making the service much more observable and easier to operate in production.