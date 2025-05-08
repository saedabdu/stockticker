# Interface Improvements

## Purpose
Interface improvements enhance the stockticker service's modularity, testability, and maintainability. By defining clear contracts between components, we make it easier to replace implementations, write unit tests, and understand component interactions.

## 1. Core Interface Definitions

### Implementation Location
Create a new directory at `pkg/interfaces/` to house all interface definitions.

### Required Files

#### pkg/interfaces/cache.go
```go
package interfaces

import "time"

// Cache defines the contract for caching functionality
type Cache interface {
    // Get retrieves an item from the cache by key
    // The second return value indicates whether the key was found
    Get(key string) (interface{}, bool)

    // Set adds an item to the cache with the given key and expiration duration
    Set(key string, value interface{}, duration time.Duration)

    // Delete removes an item from the cache
    Delete(key string)

    // Cleanup removes expired items from the cache
    Cleanup()
}
```

#### pkg/interfaces/client.go
```go
package interfaces

import (
    "context"

    "github.com/saedabdu/stockticker/pkg/models"
)

// StockClient defines the contract for external stock data APIs
type StockClient interface {
    // GetStockData retrieves stock data for the specified symbol and number of days
    GetStockData(ctx context.Context, symbol string, days int) (*models.AlphaVantageResponse, error)

    // Ping checks if the external API is accessible
    Ping(ctx context.Context) error
}
```

#### pkg/interfaces/service.go
```go
package interfaces

import (
    "context"

    "github.com/saedabdu/stockticker/pkg/models"
)

// StockService defines the contract for stock data business logic
type StockService interface {
    // GetStockData retrieves processed stock data
    GetStockData(ctx context.Context) (*models.StockData, error)
}
```

## 2. Enhanced Constructor Functions

### Implementation Location
Update existing constructor functions in their respective packages.

### Changes Required

#### internal/service/stock.go
```go
package service

import (
    "errors"

    "github.com/saedabdu/stockticker/internal/config"
    "github.com/saedabdu/stockticker/pkg/interfaces"
)

// New creates a new StockService with validation of dependencies
func New(cfg *config.Config, client interfaces.StockClient, cache interfaces.Cache) (*StockService, error) {
    if cfg == nil {
        return nil, errors.New("config cannot be nil")
    }
    if client == nil {
        return nil, errors.New("client cannot be nil")
    }
    if cache == nil {
        return nil, errors.New("cache cannot be nil")
    }

    return &StockService{
        config: cfg,
        client: client,
        cache:  cache,
    }, nil
}
```

#### internal/client/alphavantage.go
```go
// Update client constructor with validation
func NewAlphaVantage(apiKey string) (*AlphaVantage, error) {
    if apiKey == "" {
        return nil, errors.New("API key cannot be empty")
    }

    return &AlphaVantage{
        apiKey: apiKey,
        httpClient: &http.Client{
            Timeout: 10 * time.Second,
        },
    }, nil
}
```

#### internal/cache/cache.go
```go
// Update cache constructor with validation
func New(maxItems int) (*Cache, error) {
    if maxItems <= 0 {
        maxItems = defaultMaxItems // Define a sensible default
    }

    return &Cache{
        items: make(map[string]Item),
        maxItems: maxItems,
    }, nil
}
```

## 3. Implementation in Main

### Implementation Location
Update `cmd/stockticker/main.go` to use the new interfaces and handle constructor errors.

### Changes Required

```go
package main

import (
    // existing imports

    "github.com/saedabdu/stockticker/pkg/interfaces"
)

func main() {
    // Load configuration
    cfg, err := config.New()
    if err != nil {
        log.Fatalf("Error loading configuration: %v", err)
    }

    // Create API client
    apiClient, err := client.NewAlphaVantage(cfg.APIKey)
    if err != nil {
        log.Fatalf("Error creating API client: %v", err)
    }

    // Create cache
    cacheInstance, err := cache.New(1000) // Set reasonable max items
    if err != nil {
        log.Fatalf("Error creating cache: %v", err)
    }

    // Create service
    stockService, err := service.New(cfg, apiClient, cacheInstance)
    if err != nil {
        log.Fatalf("Error creating stock service: %v", err)
    }

    // Use as before...
}
```

## Benefits

1. **Testability**: Interfaces make it easy to create mock implementations for unit testing
2. **Flexibility**: Components can be swapped without changing dependent code
3. **Clarity**: Interfaces document the expected behavior of each component
4. **Safety**: Constructor validation ensures proper initialization and prevents nil pointer panics
5. **Documentation**: Interface definitions serve as self-documenting contracts

## Codebase Analysis

The current implementation in the stockticker service uses direct dependencies without interface abstractions, making it difficult to test components in isolation and replace implementations. The proposed changes align with Go best practices for dependency management and component design.