package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/saedabdu/stockticker/pkg/models"
)

const (
	baseURL  = "https://www.alphavantage.co/query"
	function = "TIME_SERIES_DAILY"
)

// AlphaVantage is the AlphaVantage API client
type AlphaVantage struct {
	apiKey     string
	httpClient *http.Client
}

// NewAlphaVantage creates a new AlphaVantage API client
func NewAlphaVantage(apiKey string) *AlphaVantage {
	return &AlphaVantage{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetStockData retrieves stock data from the AlphaVantage API
func (c *AlphaVantage) GetStockData(symbol string) (*models.AlphaVantageResponse, error) {
	params := url.Values{}
	params.Add("apikey", c.apiKey)
	params.Add("function", function)
	params.Add("symbol", symbol)

	reqURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	resp, err := c.httpClient.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("error making request to Alpha Vantage: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Alpha Vantage API error (status code %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var result models.AlphaVantageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("error decoding Alpha Vantage response: %w", err)
	}

	// Check for error messages in the response
	if result.TimeSeries == nil || len(result.TimeSeries) == 0 {
		return nil, fmt.Errorf("no data returned from Alpha Vantage, possibly invalid symbol or API key")
	}

	return &result, nil
}
