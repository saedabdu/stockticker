# Stock Ticker API

```
stockticker/
|-- cmd/
|   `-- stockticker/
|       `-- main.go              # Application entry point
|-- internal/
|   |-- api/
|   |   |-- handler/
|   |   |   `-- stock.go         # HTTP handlers
|   |   `-- models.go            # API response/request models
|   |-- client/
|   |   `-- alphavantage.go      # External API client
|   |-- config/
|   |   `-- config.go            # Application configuration
|   `-- service/
|       `-- stock.go             # Business logic
|-- pkg/
|   `-- models/
|       `-- stock.go             # Domain models
|-- go.mod                       # Module definition
`-- README.md                    # Readme
```