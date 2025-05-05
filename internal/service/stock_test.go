package service

import (
	"strings"
	"testing"

	"github.com/saedabdu/stockticker/internal/cache"
	"github.com/saedabdu/stockticker/internal/client"
	"github.com/saedabdu/stockticker/internal/config"
	"github.com/saedabdu/stockticker/pkg/models"
)

func TestProcessAPIResponse(t *testing.T) {
	tests := []struct {
		name           string
		apiResponse    *models.AlphaVantageResponse
		config         *config.Config
		expectedData   *models.StockData
		expectedError  bool
		expectedErrMsg string
	}{
		{
			name: "successful processing",
			apiResponse: &models.AlphaVantageResponse{
				TimeSeries: map[string]models.DailyPrice{
					"2023-01-03": {Close: "150.10"},
					"2023-01-02": {Close: "145.50"},
					"2023-01-01": {Close: "140.20"},
				},
			},
			config: &config.Config{
				Symbol: "AAPL",
				NDays:  3,
			},
			expectedData: &models.StockData{
				Symbol: "AAPL",
				Prices: []models.StockPrice{
					{Date: "2023-01-03", Close: 150.10},
					{Date: "2023-01-02", Close: 145.50},
					{Date: "2023-01-01", Close: 140.20},
				},
				Average: 145.26666666666668, // (150.10 + 145.50 + 140.20) / 3
			},
			expectedError: false,
		},
		{
			name: "limit days to config",
			apiResponse: &models.AlphaVantageResponse{
				TimeSeries: map[string]models.DailyPrice{
					"2023-01-05": {Close: "160.00"},
					"2023-01-04": {Close: "155.75"},
					"2023-01-03": {Close: "150.10"},
					"2023-01-02": {Close: "145.50"},
					"2023-01-01": {Close: "140.20"},
				},
			},
			config: &config.Config{
				Symbol: "AAPL",
				NDays:  3,
			},
			expectedData: &models.StockData{
				Symbol: "AAPL",
				Prices: []models.StockPrice{
					{Date: "2023-01-05", Close: 160.00},
					{Date: "2023-01-04", Close: 155.75},
					{Date: "2023-01-03", Close: 150.10},
				},
				Average: 155.28333333333333, // (160.00 + 155.75 + 150.10) / 3
			},
			expectedError: false,
		},
		{
			name: "invalid close price",
			apiResponse: &models.AlphaVantageResponse{
				TimeSeries: map[string]models.DailyPrice{
					"2023-01-03": {Close: "invalid"},
					"2023-01-02": {Close: "145.50"},
				},
			},
			config: &config.Config{
				Symbol: "AAPL",
				NDays:  3,
			},
			expectedError:  true,
			expectedErrMsg: "error parsing close price for date 2023-01-03",
		},
		{
			name: "no price data",
			apiResponse: &models.AlphaVantageResponse{
				TimeSeries: map[string]models.DailyPrice{},
			},
			config: &config.Config{
				Symbol: "AAPL",
				NDays:  3,
			},
			expectedError:  true,
			expectedErrMsg: "no price data available for symbol AAPL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a service instance with the test config
			service := &StockService{
				config: tt.config,
				client: &client.AlphaVantage{}, // Using empty client since we're testing processAPIResponse directly
				cache:  cache.New(),
			}

			// Call the function under test
			result, err := service.processAPIResponse(tt.apiResponse)

			// Verify error cases
			if tt.expectedError {
				if err == nil {
					t.Fatalf("expected error containing '%s', got nil", tt.expectedErrMsg)
				}
				if tt.expectedErrMsg != "" && err != nil {
					if !contains(err.Error(), tt.expectedErrMsg) {
						t.Errorf("expected error containing '%s', got '%s'", tt.expectedErrMsg, err.Error())
					}
				}
				return
			}

			// Verify no error when not expected
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify the result
			if result.Symbol != tt.expectedData.Symbol {
				t.Errorf("expected Symbol %s, got %s", tt.expectedData.Symbol, result.Symbol)
			}

			if result.Average != tt.expectedData.Average {
				t.Errorf("expected Average %f, got %f", tt.expectedData.Average, result.Average)
			}

			if len(result.Prices) != len(tt.expectedData.Prices) {
				t.Errorf("expected %d prices, got %d", len(tt.expectedData.Prices), len(result.Prices))
				return
			}

			// Verify each price item
			for i, expectedPrice := range tt.expectedData.Prices {
				if i >= len(result.Prices) {
					t.Errorf("missing expected price at index %d", i)
					continue
				}

				actualPrice := result.Prices[i]
				if expectedPrice.Date != actualPrice.Date {
					t.Errorf("price[%d]: expected Date %s, got %s", i, expectedPrice.Date, actualPrice.Date)
				}

				if expectedPrice.Close != actualPrice.Close {
					t.Errorf("price[%d]: expected Close %f, got %f", i, expectedPrice.Close, actualPrice.Close)
				}
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
