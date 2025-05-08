# Stockticker API

A lightweight microservice for retrieving and caching stock ticker information using the AlphaVantage API.

## Project Structure

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
|   |-- cache/
|   |   `-- cache.go             # Simple in-memory cache
|   |-- client/
|   |   `-- alphavantage.go      # External API client
|   |-- config/
|   |   `-- config.go            # Application configuration
|   `-- service/
|       `-- stock.go             # Business logic
|-- pkg/
|   `-- models/
|       `-- stock.go             # Domain models
|-- kubernetes/                  # Kubernetes deployment files
|-- go.mod                       # Module definition
`-- README.md                    # Documentation
```

## Data Flow

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

## Comprehensive Overview
- [Read the full overview](docs/overview.md)

## Getting Started

Follow the instructions below to get started.

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/)
- [Go 1.21+](https://golang.org/dl/)
- [Kubernetes](https://kubernetes.io/docs/tasks/tools/) Or
- [Minikube](https://minikube.sigs.k8s.io/docs/start/)
- [Alpha Vantage API key](https://www.alphavantage.co/support/#api-key) - For stock data access

### Running with Docker

The simplest way to run the service:

1. Clone the repository and navigate to the project directory:
   ```bash
   git clone https://github.com/saedabdu/stockticker.git
   cd stockticker
   ```

2. Check prerequisites:
   ```bash
   make check
   ```

3. Run the service:
   ```bash
   SYMBOL=MSFT NDAYS=7 API_KEY=your_alphavantage_api_key make run
   ```

4. Access the service at http://localhost:8080

### Development with Go

For local development without Docker:

1. Clone the repository and navigate to the project directory:
   ```bash
   git clone https://github.com/saedabdu/stockticker.git
   cd stockticker
   ```

2. Install dependencies:
   ```bash
   go mod download
   ```

3. Set the required environment variables:
   ```bash
   export SYMBOL=MSFT
   export NDAYS=7
   export API_KEY=your_alphavantage_api_key
   ```

4. Run the service:
   ```bash
   # From the project root directory
   go run cmd/stockticker/main.go
   ```

5. Or build and run the executable:
   ```bash
   # Build the executable
   go build -o stockticker cmd/stockticker/main.go

   # Run the executable
   ./stockticker
   ```

6. Access the service at http://localhost:8080

### Kubernetes Deployment

To deploy the service to Kubernetes (Minikube):

1. Clone the repository and navigate to the project directory (if not already done):
   ```bash
   git clone https://github.com/saedabdu/stockticker.git
   cd stockticker
   ```

2. Check prerequisites:
   ```bash
   make check
   ```

3. Update the Kubernetes secret `stockticker-secret` in `kubernetes/deployment.yaml` with your Alpha Vantage API key

4. Deploy to Minikube:
   ```bash
   make deploy
   ```

5. Set up port forwarding to access the service:
   ```bash
   make port-forward
   ```

6. Access the service at http://localhost:8080

## API Reference

### Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check endpoint |
| `/stocks` | GET | Get stock data for the configured symbol |

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `SYMBOL` | Stock symbol to track | `MSFT` |
| `NDAYS` | Number of days of historical data | `7` |
| `API_KEY` | Alpha Vantage API key | Required |

### Sample Response

```json
{
  "symbol": "MSFT",
  "prices": [
    {"date": "2025-05-02", "close": 435.28},
    {"date": "2025-05-01", "close": 425.4},
    {"date": "2025-04-30", "close": 395.26},
    {"date": "2025-04-29", "close": 394.04},
    {"date": "2025-04-28", "close": 391.16},
    {"date": "2025-04-25", "close": 391.85},
    {"date": "2025-04-24", "close": 387.3}
  ],
  "average": 402.8985714285715
}
```

The response includes:
- `symbol`: The stock ticker symbol
- `prices`: An array of daily closing prices with dates
- `average`: The average closing price over the requested period

## Troubleshooting

- **Connection issues**
  ```bash
  # Ensure Docker and Minikube are running
  make check
  ```

- **Port conflicts**
  ```bash
  # If port 8080 is already in use, the command will automatically kill existing processes using that port.
  make port-forward
  ```
- **Container logs**
  ```bash
  # Check container logs
  kubectl logs -l app=stockticker
  ```