package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/saedabdu/stockticker/internal/api"
	"github.com/saedabdu/stockticker/internal/service"
)

// StockHandler handles HTTP requests for stock data
type StockHandler struct {
	stockService *service.StockService
}

// NewStockHandler creates a new StockHandler
func NewStockHandler(stockService *service.StockService) *StockHandler {
	return &StockHandler{
		stockService: stockService,
	}
}

// HandleStocks handles requests to the /stocks endpoint
func (h *StockHandler) HandleStocks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stockData, err := h.stockService.GetStockData()
	if err != nil {
		log.Printf("Error getting stock data: %v", err)
		h.sendErrorResponse(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert domain model to API response
	response := api.StockResponse{
		Symbol:  stockData.Symbol,
		Prices:  stockData.Prices,
		Average: stockData.Average,
	}

	h.sendJSONResponse(w, response)
}

// sendJSONResponse sends a JSON response to the client
func (h *StockHandler) sendJSONResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
	}
}

// sendErrorResponse sends an error response to the client
func (h *StockHandler) sendErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := api.ErrorResponse{Error: message}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding error response: %v", err)
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
	}
}
