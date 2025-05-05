package service

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/saedabdu/stockticker/internal/cache"
	"github.com/saedabdu/stockticker/internal/client"
	"github.com/saedabdu/stockticker/internal/config"
	"github.com/saedabdu/stockticker/pkg/models"
)

const (
	cacheDuration = 15 * time.Minute
)

// StockService handles stock data retrieval and processing
type StockService struct {
	client *client.AlphaVantage
	cache  *cache.Cache
	config *config.Config
}

// New creates a new StockService
func New(cfg *config.Config, client *client.AlphaVantage, cache *cache.Cache) *StockService {
	return &StockService{
		client: client,
		cache:  cache,
		config: cfg,
	}
}

// GetStockData retrieves stock data either from cache or the API
func (s *StockService) GetStockData() (*models.StockData, error) {
	cacheKey := s.config.Symbol

	// Try to get data from cache first
	if cachedData, found := s.cache.Get(cacheKey); found {
		return cachedData.(*models.StockData), nil
	}

	// Get data from the API - pass the number of days to ensure we get enough data
	apiResponse, err := s.client.GetStockData(s.config.Symbol, s.config.NDays)
	if err != nil {
		return nil, err
	}

	// Process the API response
	stockData, err := s.processAPIResponse(apiResponse)
	if err != nil {
		return nil, err
	}

	// Cache the response
	s.cache.Set(cacheKey, stockData, cacheDuration)

	return stockData, nil
}

// processAPIResponse converts the API response to our model and calculates the average
func (s *StockService) processAPIResponse(apiResponse *models.AlphaVantageResponse) (*models.StockData, error) {
	var prices []models.StockPrice
	var totalClose float64

	// Extract dates and sort them
	dates := make([]string, 0, len(apiResponse.TimeSeries))
	for date := range apiResponse.TimeSeries {
		dates = append(dates, date)
	}

	// Sort dates in descending order (newest first)
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))

	// Limit to the requested number of days
	if len(dates) > s.config.NDays {
		dates = dates[:s.config.NDays]
	}

	// Process each date's data
	for _, date := range dates {
		dailyPrice := apiResponse.TimeSeries[date]
		closePrice, err := strconv.ParseFloat(dailyPrice.Close, 64)
		if err != nil {
			return nil, fmt.Errorf("error parsing close price for date %s: %w", date, err)
		}

		prices = append(prices, models.StockPrice{
			Date:  date,
			Close: closePrice,
		})

		totalClose += closePrice
	}

	if len(prices) == 0 {
		return nil, fmt.Errorf("no price data available for symbol %s", s.config.Symbol)
	}

	// Calculate average
	average := totalClose / float64(len(prices))

	return &models.StockData{
		Symbol:  s.config.Symbol,
		Prices:  prices,
		Average: average,
	}, nil
}
