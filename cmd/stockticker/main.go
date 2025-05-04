package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/saedabdu/stockticker/internal/api/handler"
	"github.com/saedabdu/stockticker/internal/cache"
	"github.com/saedabdu/stockticker/internal/client"
	"github.com/saedabdu/stockticker/internal/config"
	"github.com/saedabdu/stockticker/internal/service"
)

func main() {
	// Load configuration
	cfg, err := config.New()
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}

	// Create API client
	apiClient := client.NewAlphaVantage(cfg.APIKey)

	// Create cache
	cacheInstance := cache.New()

	// Create service
	stockService := service.New(cfg, apiClient, cacheInstance)

	// Create handler
	stockHandler := handler.NewStockHandler(stockService)

	// Setup routes
	http.HandleFunc("/stocks", stockHandler.HandleStocks)

	// Start HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Starting server on port %s", cfg.Port)
		log.Printf("Configuration: Symbol=%s, NDays=%d", cfg.Symbol, cfg.NDays)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Setup graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
}
