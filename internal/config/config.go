package config

import (
	"fmt"
	"os"
	"strconv"
)

// Default values
const (
	DefaultPort   = "8080"
	DefaultSymbol = "IBM"
	DefaultNDays  = 7
)

// Config holds the application configuration
type Config struct {
	Port   string
	APIKey string
	Symbol string
	NDays  int
}

// New creates a new Config with values from environment variables or defaults
func New() (*Config, error) {
	port := getEnvOrDefault("PORT", DefaultPort)
	apiKey := os.Getenv("API_KEY")
	symbol := getEnvOrDefault("SYMBOL", DefaultSymbol)

	nDaysStr := getEnvOrDefault("N_DAYS", strconv.Itoa(DefaultNDays))
	nDays, err := strconv.Atoi(nDaysStr)
	if err != nil {
		return nil, fmt.Errorf("invalid N_DAYS value: %w", err)
	}

	if apiKey == "" {
		return nil, fmt.Errorf("API_KEY environment variable is required")
	}

	return &Config{
		Port:   port,
		APIKey: apiKey,
		Symbol: symbol,
		NDays:  nDays,
	}, nil
}

// getEnvOrDefault returns the value of the environment variable or the default value
func getEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
