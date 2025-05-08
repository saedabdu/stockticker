# Error Handling Improvements

## Purpose
Robust error handling is essential for production applications to provide clear feedback to clients, facilitate debugging, and enable proper error recovery. The current implementation lacks structured error responses and domain-specific error types, making it difficult to diagnose issues and provide meaningful error messages to clients.

## 1. Domain-Specific Error Types

### Implementation Location
Create a new file at `internal/service/errors.go`.

### Required Implementation

```go
package service

import (
    "errors"
    "fmt"
)

// Standard error sentinels
var (
    // ErrNotFound indicates requested data wasn't found
    ErrNotFound = errors.New("stock data not found")

    // ErrExternalAPI indicates a failure in the external API
    ErrExternalAPI = errors.New("external API error")

    // ErrInvalidSymbol indicates the requested stock symbol is invalid
    ErrInvalidSymbol = errors.New("invalid stock symbol")

    // ErrProcessingError indicates a generic data processing error
    ErrProcessingError = errors.New("error processing stock data")

    // ErrRateLimited indicates the external API rate limit was hit
    ErrRateLimited = errors.New("rate limit exceeded")
)

// StockError represents a detailed error related to stock data operations
type StockError struct {
    // The underlying error
    Err error

    // Machine-readable error code
    Code string

    // Human-readable error message
    Message string

    // Stock symbol related to the error
    Symbol string
}

// Error implements the error interface
func (e *StockError) Error() string {
    if e.Symbol != "" {
        return fmt.Sprintf("%s: %s [symbol=%s]", e.Code, e.Message, e.Symbol)
    }
    return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap implements the errors.Unwrap interface for error chains
func (e *StockError) Unwrap() error {
    return e.Err
}

// NewNotFoundError creates a new error for when stock data is not found
func NewNotFoundError(symbol string) error {
    return &StockError{
        Err:     ErrNotFound,
        Code:    "STOCK_NOT_FOUND",
        Message: fmt.Sprintf("Stock data not found for symbol: %s", symbol),
        Symbol:  symbol,
    }
}

// NewExternalAPIError creates a new error for external API failures
func NewExternalAPIError(err error, symbol string) error {
    return &StockError{
        Err:     ErrExternalAPI,
        Code:    "EXTERNAL_API_ERROR",
        Message: fmt.Sprintf("Error from external API: %s", err.Error()),
        Symbol:  symbol,
    }
}

// NewInvalidSymbolError creates a new error for invalid stock symbols
func NewInvalidSymbolError(symbol string) error {
    return &StockError{
        Err:     ErrInvalidSymbol,
        Code:    "INVALID_SYMBOL",
        Message: fmt.Sprintf("Invalid stock symbol: %s", symbol),
        Symbol:  symbol,
    }
}

// NewRateLimitedError creates a new error for rate limiting
func NewRateLimitedError(symbol string) error {
    return &StockError{
        Err:     ErrRateLimited,
        Code:    "RATE_LIMITED",
        Message: "API rate limit exceeded, please try again later",
        Symbol:  symbol,
    }
}
```

## 2. API Error Response Model

### Implementation Location
Update `internal/api/models.go` to include a structured error response.

### Required Implementation

```go
// ErrorResponse represents a structured error response for API clients
type ErrorResponse struct {
    // Machine-readable error code
    Code string `json:"code"`

    // Human-readable error message
    Message string `json:"message"`

    // Unique identifier for the request (for correlation with logs)
    RequestID string `json:"requestId,omitempty"`

    // Additional error details (optional)
    Details any `json:"details,omitempty"`
}
```

## 3. Middleware for Request ID Generation

### Implementation Location
Create a new directory and file at `internal/api/middleware/request_id.go`.

### Required Implementation

```go
package middleware

import (
    "context"
    "net/http"

    "github.com/google/uuid"
)

// Key for request ID in context
type contextKey string
const requestIDKey contextKey = "requestID"

// RequestID middleware adds a unique request ID to each request
func RequestID(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Check if request already has an ID from an upstream service
        requestID := r.Header.Get("X-Request-ID")
        if requestID == "" {
            requestID = uuid.New().String()
        }

        // Add ID to response headers
        w.Header().Set("X-Request-ID", requestID)

        // Add ID to request context
        ctx := context.WithValue(r.Context(), requestIDKey, requestID)

        // Call the next handler with the updated context
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// GetRequestID retrieves the request ID from the context
func GetRequestID(ctx context.Context) string {
    if id, ok := ctx.Value(requestIDKey).(string); ok {
        return id
    }
    return "unknown"
}
```

## 4. Enhanced Error Response Handler

### Implementation Location
Update `internal/api/handler/stock.go` to improve error responses.

### Required Implementation

```go
// sendErrorResponse sends a structured error response to the client
func (h *StockHandler) sendErrorResponse(w http.ResponseWriter, err error, statusCode int) {
    // Get request ID from context
    requestID := middleware.GetRequestID(r.Context())

    var code string
    var msg string

    // Determine error type
    var stockErr *service.StockError
    if errors.As(err, &stockErr) {
        // Use the stock error's code and message
        code = stockErr.Code
        msg = stockErr.Message
    } else {
        // Map standard errors to error codes
        switch {
        case errors.Is(err, service.ErrNotFound):
            code = "NOT_FOUND"
            msg = "The requested resource was not found"
        case errors.Is(err, service.ErrExternalAPI):
            code = "EXTERNAL_API_ERROR"
            msg = "Error communicating with external service"
        case errors.Is(err, service.ErrRateLimited):
            code = "RATE_LIMITED"
            msg = "Rate limit exceeded, please try again later"
            statusCode = http.StatusTooManyRequests
        default:
            code = "INTERNAL_ERROR"
            msg = "An internal error occurred"
        }
    }

    // Construct the response
    resp := api.ErrorResponse{
        Code:      code,
        Message:   msg,
        RequestID: requestID,
        Details:   err.Error(),
    }

    // Send the response
    h.sendJSONResponse(w, resp, statusCode)

    // Also log the error with context
    log.Printf("Error: %s - RequestID: %s - %v", code, requestID, err)
}
```

## 5. Using Domain Errors in Service Layer

### Implementation Location
Update `internal/service/stock.go` to use the domain-specific errors.

### Required Changes

```go
// GetStockData retrieves stock data either from cache or the API
func (s *StockService) GetStockData(ctx context.Context) (*models.StockData, error) {
    cacheKey := s.config.Symbol

    // Try to get data from cache first
    if cachedData, found := s.cache.Get(cacheKey); found {
        return cachedData.(*models.StockData), nil
    }

    // Validate symbol (simple example)
    if s.config.Symbol == "" {
        return nil, NewInvalidSymbolError("")
    }

    // Get data from the API
    apiResponse, err := s.client.GetStockData(ctx, s.config.Symbol, s.config.NDays)
    if err != nil {
        // Handle specific API errors
        if errors.Is(err, ErrRateLimited) {
            return nil, NewRateLimitedError(s.config.Symbol)
        }
        return nil, NewExternalAPIError(err, s.config.Symbol)
    }

    // Process the API response
    stockData, err := s.processAPIResponse(apiResponse)
    if err != nil {
        return nil, fmt.Errorf("%w: %v", ErrProcessingError, err)
    }

    // If no price data is found
    if len(stockData.Prices) == 0 {
        return nil, NewNotFoundError(s.config.Symbol)
    }

    // Cache the response
    s.cache.Set(cacheKey, stockData, cacheDuration)

    return stockData, nil
}
```

## 6. Client Layer Error Handling

### Implementation Location
Update `internal/client/alphavantage.go` to handle specific API errors.

### Required Changes

```go
// GetStockData retrieves stock data from the AlphaVantage API
func (c *AlphaVantage) GetStockData(ctx context.Context, symbol string, days int) (*models.AlphaVantageResponse, error) {
    // Wait for rate limiter
    if err := c.limiter.Wait(ctx); err != nil {
        return nil, fmt.Errorf("rate limiter error: %w", err)
    }

    // ... existing code for building and sending the request ...

    // Handle specific HTTP status codes
    if resp.StatusCode != http.StatusOK {
        bodyBytes, _ := io.ReadAll(resp.Body)
        errorMsg := string(bodyBytes)

        switch resp.StatusCode {
        case http.StatusTooManyRequests:
            return nil, service.ErrRateLimited
        case http.StatusNotFound:
            return nil, fmt.Errorf("symbol not found: %s", symbol)
        case http.StatusUnauthorized:
            return nil, fmt.Errorf("invalid API key")
        default:
            return nil, fmt.Errorf("Alpha Vantage API error (status code %d): %s", resp.StatusCode, errorMsg)
        }
    }

    // ... existing code for parsing the response ...
}
```

## 7. Main.go Configuration for Middleware

### Implementation Location
Update `cmd/stockticker/main.go` to add the request ID middleware.

### Required Changes

```go
func main() {
    // ... existing initialization code ...

    // Setup handlers with middleware
    mux := http.NewServeMux()

    // Add routes
    mux.HandleFunc("/stocks", stockHandler.HandleStocks)
    mux.HandleFunc("/health", stockHandler.HandleHealth)

    // Create middleware chain
    handler := middleware.RequestID(mux)

    // Create HTTP server with the middleware chain
    server := &http.Server{
        Addr:         fmt.Sprintf(":%s", cfg.Port),
        Handler:      handler, // Use wrapped handler with middleware
        ReadTimeout:  5 * time.Second,
        WriteTimeout: 10 * time.Second,
        IdleTimeout:  120 * time.Second,
    }

    // ... existing server startup code ...
}
```

## Benefits

1. **Better Error Classification**: Domain-specific errors provide clear error types that can be checked with `errors.Is` and `errors.As`
2. **Improved Debugging**: Request IDs allow correlation between client errors and server logs
3. **Consistent Responses**: All errors follow a standard format that clients can reliably parse
4. **Helpful Error Messages**: Detailed error information helps developers diagnose issues
5. **Correct Status Codes**: Different error types trigger appropriate HTTP status codes
6. **Proper Error Propagation**: Errors maintain context as they travel through the system

## Codebase Analysis

The current implementation uses basic error handling with simple string messages. Error classification is manual and inconsistent. There's no request tracing and error details are minimal. The proposed improvements standardize error handling across all layers of the application, making it more robust and debuggable.