# StockTicker Microservice: Code Walkthrough and Data Flow

This document provides a comprehensive overview of the StockTicker microservice, explaining its architecture, component interactions, and data flow. Use this as your entry point for understanding the codebase and as a reference during code reviews or interviews.

## Table of Contents

1. [Project Overview](#project-overview)
2. [Architecture Components](#architecture-components)
3. [Data Flow Visualization](#data-flow-visualization)
4. [Key Interactions](#key-interactions)
5. [Current Limitations](#current-limitations)
6. [Improvement Areas](#improvement-areas)

## Project Overview

The StockTicker microservice is a Go-based application that provides stock price information via a RESTful API. It fetches data from the Alpha Vantage API, processes it, and returns stock prices and averages over a configurable time period.

**Key Features:**
- REST API endpoint for stock data (`/stocks`)
- Health check endpoint (`/health`)
- In-memory caching of stock data
- Configurable stock symbol and time period

## Architecture Components

The service follows a layered architecture with clear separation of concerns:

### Entry Point
- `cmd/stockticker/main.go`: Application bootstrap, component wiring, and server setup

### API Layer
- `internal/api/handler/stock.go`: HTTP handlers for endpoints
- `internal/api/models.go`: API response models

### Business Logic
- `internal/service/stock.go`: Business logic for stock data retrieval and processing

### Data Access
- `internal/client/alphavantage.go`: External API client for Alpha Vantage
- `internal/cache/cache.go`: In-memory cache implementation

### Configuration
- `internal/config/config.go`: Environment-based configuration management

### Domain Models
- `pkg/models/stock.go`: Shared data structures for both API and domain models

## Data Flow Visualization

Below is the typical flow of a request through the system:

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌───────────────┐
│  HTTP       │    │  Handler    │    │  Service    │    │  Cache        │
│  Request    │───>│  Layer      │───>│  Layer      │───>│  (Check)      │
└─────────────┘    └─────────────┘    └─────────────┘    └───────────────┘
                                                                │
                                                                │ (Cache Miss)
                                                                ▼
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌───────────────┐
│  HTTP       │    │  Handler    │    │  Service    │    │  API Client   │
│  Response   │<───│  Layer      │<───│  Layer      │<───│  (Fetch)      │
└─────────────┘    └─────────────┘    └─────────────┘    └───────────────┘
                                            │
                                            │ (Store)
                                            ▼
                                     ┌───────────────┐
                                     │  Cache        │
                                     │  (Update)     │
                                     └───────────────┘
```

## Key Interactions

### Request Processing Flow

1. **HTTP Request** → An HTTP GET request arrives at the `/stocks` endpoint.

2. **Handler Layer** → `StockHandler.HandleStocks` processes the request:
   - Validates the HTTP method
   - Calls the service layer to retrieve stock data
   - Converts the service response to API format
   - Returns a JSON response

3. **Service Layer** → `StockService.GetStockData` contains business logic:
   - Checks the cache for existing data
   - If not in cache, calls the client layer
   - Processes and transforms the API response
   - Caches the result
   - Returns the processed data

4. **Client Layer** → `AlphaVantage.GetStockData` handles external communication:
   - Constructs the API request with appropriate parameters
   - Makes HTTP request to the Alpha Vantage API
   - Handles response status codes and errors
   - Parses the JSON response
   - Returns the raw API response

5. **Cache Layer** → `Cache` provides data storage:
   - Stores processed data with an expiration time
   - Returns cached data when available
   - Implements thread-safe operations for concurrent access

### Configuration and Initialization Flow

1. **Load Configuration** → `config.New()` loads app settings:
   - Reads environment variables
   - Applies default values
   - Validates required settings

2. **Initialize Components** → `main()` sets up the application:
   - Creates API client with configured API key
   - Initializes in-memory cache
   - Creates service with dependencies
   - Configures HTTP handlers and routes
   - Starts the HTTP server

3. **Graceful Shutdown** → Signal handling for clean termination:
   - Captures OS termination signals
   - Logs shutdown process
   - Releases resources (implementation incomplete)

## Current Limitations

The service has several limitations that should be addressed for production readiness:

1. **Incomplete Graceful Shutdown**: Signal handling is set up, but the HTTP server is not properly shut down with a timeout context.

2. **Basic Error Handling**: Error handling exists but lacks structure, context, and proper classification.

3. **Limited Caching**: The in-memory cache lacks cleanup routines, size limits, and proper error handling for type assertions.

4. **Tight Coupling**: Components use concrete types rather than interfaces, limiting testability and flexibility.

5. **Minimal Observability**: Basic logging with no structured format, metrics, or request tracing.

6. **Basic Configuration**: Limited configuration options with no validation, hot reloading, or environment-specific settings.

## Improvement Areas

To make the service production-ready, we've identified six key areas for improvement. Each area has a detailed implementation guide:

1. [**Error Handling**](1-error-handling-improvements.md) - Implementing structured errors, domain-specific error types, and request ID tracking

2. [**Observability**](2-observability-improvements.md) - Adding structured logging, metrics collection, health checks, and request tracing

3. [**Configuration**](3-configuration-improvements.md) - Enhancing validation, supporting configuration files, and implementing hot reloading

4. [**Resilience**](4-resilience-improvements.md) - Adding retry logic, circuit breakers, rate limiting, and graceful degradation

5. [**Caching**](5-cache-improvements.md) - Implementing automatic cleanup, size limiting, and cache metrics

6. [**Interfaces**](6-interface-improvements.md) - Defining proper interfaces for all components and implementing cleaner dependency injection

For a more detailed overview of these improvements and implementation priorities, see the [Improvements Index](0-improvements-index.md).

## Code Review Process

When reviewing this codebase, follow this sequence:

1. Start with `cmd/stockticker/main.go` to understand the overall application structure
2. Review the handler, service, and client components to understand the request flow
3. Examine the cache implementation and configuration management
4. Identify areas for improvement using the guides linked above

This approach will give you a comprehensive understanding of the application's architecture and data flow, as well as a clear path to making it production-ready.
