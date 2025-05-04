package api

import "github.com/saedabdu/stockticker/pkg/models"

// StockResponse represents the response sent to the client
type StockResponse struct {
	Symbol  string              `json:"symbol"`
	Prices  []models.StockPrice `json:"prices"`
	Average float64             `json:"average"`
}

// ErrorResponse represents an error response sent to the client
type ErrorResponse struct {
	Error string `json:"error"`
}

// HealthResponse represents a health check response
type HealthResponse struct {
	Status string `json:"status"`
}
