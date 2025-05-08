# Configuration Management Improvements

## Purpose
Effective configuration management is critical for production services, allowing operators to adjust behavior without code changes and ensuring proper validation of settings. The current implementation of the stockticker service has basic configuration from environment variables but lacks several important features for robust production deployment.

## 1. Configuration Validation

### Problem
The current configuration implementation loads values from environment variables but has limited validation. Missing or invalid configuration can lead to runtime errors or unexpected behavior.

### Implementation Location
Update `internal/config/config.go`.

### Required Implementation

```go
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Constants for default values
const (
	DefaultPort          = "8080"
	DefaultSymbol        = "IBM"
	DefaultNDays         = "7"
	DefaultCacheSize     = "1000"
	DefaultCacheDuration = "15m"
)

// Config holds the application configuration
type Config struct {
	// Server configuration
	Port            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration

	// API client configuration
	APIKey         string
	RequestTimeout time.Duration
	MaxRetries     int

	// Business logic configuration
	Symbol string
	NDays  int

	// Cache configuration
	CacheSize     int
	CacheDuration time.Duration

	// Feature flags
	EnableMetrics       bool
	EnableTestData      bool
	EnableRateLimiting  bool
	EnableCircuitBreaker bool
}

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("invalid configuration for %s: %s", e.Field, e.Message)
}

// Option is a function that configures the Config
type Option func(*Config)

// WithTestDefaults returns an option that sets test defaults
func WithTestDefaults() Option {
	return func(c *Config) {
		c.Port = "8080"
		c.ReadTimeout = 5 * time.Second
		c.WriteTimeout = 10 * time.Second
		c.ShutdownTimeout = 30 * time.Second
		c.APIKey = "test-api-key"
		c.RequestTimeout = 10 * time.Second
		c.MaxRetries = 3
		c.Symbol = "IBM"
		c.NDays = 7
		c.CacheSize = 1000
		c.CacheDuration = 15 * time.Minute
		c.EnableMetrics = true
		c.EnableTestData = true
		c.EnableRateLimiting = true
		c.EnableCircuitBreaker = true
	}
}

// WithEnv returns an option that loads configuration from environment variables
func WithEnv() Option {
	return func(c *Config) {
		// Load values from environment
		c.Port = getEnvOrDefault("PORT", DefaultPort)
		c.APIKey = os.Getenv("API_KEY")
		c.Symbol = getEnvOrDefault("SYMBOL", DefaultSymbol)

		// Parse numeric values with defaults
		c.NDays = getEnvAsIntOrDefault("NDAYS", DefaultNDays)
		c.CacheSize = getEnvAsIntOrDefault("CACHE_SIZE", DefaultCacheSize)

		// Parse durations
		c.ReadTimeout = getEnvAsDurationOrDefault("READ_TIMEOUT", "5s")
		c.WriteTimeout = getEnvAsDurationOrDefault("WRITE_TIMEOUT", "10s")
		c.ShutdownTimeout = getEnvAsDurationOrDefault("SHUTDOWN_TIMEOUT", "30s")
		c.RequestTimeout = getEnvAsDurationOrDefault("REQUEST_TIMEOUT", "10s")
		c.CacheDuration = getEnvAsDurationOrDefault("CACHE_DURATION", DefaultCacheDuration)

		// Parse integers
		c.MaxRetries = getEnvAsIntOrDefault("MAX_RETRIES", "3")

		// Parse booleans
		c.EnableMetrics = getEnvAsBoolOrDefault("ENABLE_METRICS", "true")
		c.EnableTestData = getEnvAsBoolOrDefault("ENABLE_TEST_DATA", "false")
		c.EnableRateLimiting = getEnvAsBoolOrDefault("ENABLE_RATE_LIMITING", "true")
		c.EnableCircuitBreaker = getEnvAsBoolOrDefault("ENABLE_CIRCUIT_BREAKER", "true")
	}
}

// New creates a new Config with the given options
func New(opts ...Option) (*Config, error) {
	// Create config with defaults
	cfg := &Config{
		Port:            DefaultPort,
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    10 * time.Second,
		ShutdownTimeout: 30 * time.Second,
		RequestTimeout:  10 * time.Second,
		MaxRetries:      3,
		Symbol:          DefaultSymbol,
		NDays:           7,
		CacheSize:       1000,
		CacheDuration:   15 * time.Minute,
		EnableMetrics:   true,
	}

	// Apply options
	for _, opt := range opts {
		opt(cfg)
	}

	// Validate the config
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	var validationErrors []error

	// Required fields
	if c.APIKey == "" {
		validationErrors = append(validationErrors, &ValidationError{
			Field:   "API_KEY",
			Message: "API key is required",
		})
	}

	// Numeric ranges
	if c.NDays <= 0 {
		validationErrors = append(validationErrors, &ValidationError{
			Field:   "NDAYS",
			Message: "must be a positive integer",
		})
	}

	if c.NDays > 100 {
		validationErrors = append(validationErrors, &ValidationError{
			Field:   "NDAYS",
			Message: "must be 100 or less due to API limitations",
		})
	}

	if c.CacheSize <= 0 {
		validationErrors = append(validationErrors, &ValidationError{
			Field:   "CACHE_SIZE",
			Message: "must be a positive integer",
		})
	}

	// Port must be a valid port number
	if port, err := strconv.Atoi(c.Port); err != nil || port <= 0 || port > 65535 {
		validationErrors = append(validationErrors, &ValidationError{
			Field:   "PORT",
			Message: "must be a valid port number (1-65535)",
		})
	}

	// Timeouts must be reasonable
	if c.ReadTimeout < time.Second {
		validationErrors = append(validationErrors, &ValidationError{
			Field:   "READ_TIMEOUT",
			Message: "must be at least 1 second",
		})
	}

	if c.WriteTimeout < time.Second {
		validationErrors = append(validationErrors, &ValidationError{
			Field:   "WRITE_TIMEOUT",
			Message: "must be at least 1 second",
		})
	}

	if c.ShutdownTimeout < 5*time.Second {
		validationErrors = append(validationErrors, &ValidationError{
			Field:   "SHUTDOWN_TIMEOUT",
			Message: "must be at least 5 seconds",
		})
	}

	// If there are validation errors, return them
	if len(validationErrors) > 0 {
		errStrings := make([]string, len(validationErrors))
		for i, err := range validationErrors {
			errStrings[i] = err.Error()
		}
		return errors.New("configuration validation failed: " + strings.Join(errStrings, "; "))
	}

	return nil
}

// String returns a string representation of the config (with sensitive fields masked)
func (c *Config) String() string {
	return fmt.Sprintf(
		"Config{Port: %s, Symbol: %s, NDays: %d, APIKey: %s, CacheSize: %d, CacheDuration: %s, "+
			"EnableMetrics: %t, EnableTestData: %t, EnableRateLimiting: %t, EnableCircuitBreaker: %t}",
		c.Port, c.Symbol, c.NDays, maskString(c.APIKey), c.CacheSize, c.CacheDuration,
		c.EnableMetrics, c.EnableTestData, c.EnableRateLimiting, c.EnableCircuitBreaker,
	)
}

// Helper functions

// getEnvOrDefault gets an environment variable or returns a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// getEnvAsIntOrDefault gets an environment variable as an integer or returns a default
func getEnvAsIntOrDefault(key, defaultValue string) int {
	valueStr := getEnvOrDefault(key, defaultValue)
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		// Log error and use default
		fmt.Printf("Warning: invalid value for %s: %s, using default: %s\n", key, valueStr, defaultValue)
		value, _ = strconv.Atoi(defaultValue)
	}
	return value
}

// getEnvAsDurationOrDefault gets an environment variable as a duration or returns a default
func getEnvAsDurationOrDefault(key, defaultValue string) time.Duration {
	valueStr := getEnvOrDefault(key, defaultValue)
	value, err := time.ParseDuration(valueStr)
	if err != nil {
		// Log error and use default
		fmt.Printf("Warning: invalid duration for %s: %s, using default: %s\n", key, valueStr, defaultValue)
		value, _ = time.ParseDuration(defaultValue)
	}
	return value
}

// getEnvAsBoolOrDefault gets an environment variable as a boolean or returns a default
func getEnvAsBoolOrDefault(key, defaultValue string) bool {
	valueStr := getEnvOrDefault(key, defaultValue)
	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		// Log error and use default
		fmt.Printf("Warning: invalid boolean for %s: %s, using default: %s\n", key, valueStr, defaultValue)
		value, _ = strconv.ParseBool(defaultValue)
	}
	return value
}

// maskString masks a string for display (e.g., for sensitive data like API keys)
func maskString(s string) string {
	if len(s) <= 4 {
		return "****"
	}
	return s[:2] + "****" + s[len(s)-2:]
}
```

## 2. Configuration Hot Reloading

### Problem
The current service requires a restart to apply configuration changes. In a production environment, it's useful to be able to change certain configuration parameters without a service restart.

### Implementation Location
Create a new file at `internal/config/watcher.go`.

### Required Implementation

```go
package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// ConfigWatcher watches for configuration changes
type ConfigWatcher struct {
	configFile   string
	config       *Config
	onChange     []func(*Config)
	watcher      *fsnotify.Watcher
	stopCh       chan struct{}
	mu           sync.RWMutex
	logger       Logger
}

// Logger defines the interface for logging
type Logger interface {
	Info(msg string, fields ...map[string]interface{})
	Error(msg string, err error, fields ...map[string]interface{})
}

// defaultLogger is a simple logger that writes to stdout/stderr
type defaultLogger struct{}

func (l *defaultLogger) Info(msg string, fields ...map[string]interface{}) {
	log.Println("INFO:", msg)
}

func (l *defaultLogger) Error(msg string, err error, fields ...map[string]interface{}) {
	log.Printf("ERROR: %s: %v\n", msg, err)
}

// WatcherOption is a function that configures the ConfigWatcher
type WatcherOption func(*ConfigWatcher)

// WithLogger sets the logger for the watcher
func WithLogger(logger Logger) WatcherOption {
	return func(cw *ConfigWatcher) {
		cw.logger = logger
	}
}

// NewConfigWatcher creates a new ConfigWatcher
func NewConfigWatcher(configFile string, opts ...WatcherOption) (*ConfigWatcher, error) {
	// Create fsnotify watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("error creating file watcher: %w", err)
	}

	// Create config watcher
	cw := &ConfigWatcher{
		configFile: configFile,
		onChange:   make([]func(*Config), 0),
		watcher:    watcher,
		stopCh:     make(chan struct{}),
		logger:     &defaultLogger{},
	}

	// Apply options
	for _, opt := range opts {
		opt(cw)
	}

	// Load initial config
	config, err := loadConfigFromFile(configFile)
	if err != nil {
		cw.logger.Error("Error loading config file", err, map[string]interface{}{
			"file": configFile,
		})
		return nil, err
	}
	cw.config = config

	// Start watching
	if err := watcher.Add(filepath.Dir(configFile)); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("error watching directory: %w", err)
	}

	// Start watching goroutine
	go cw.watch()

	return cw, nil
}

// OnChange registers a callback to be called when the configuration changes
func (cw *ConfigWatcher) OnChange(callback func(*Config)) {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	cw.onChange = append(cw.onChange, callback)
}

// GetConfig returns the current configuration
func (cw *ConfigWatcher) GetConfig() *Config {
	cw.mu.RLock()
	defer cw.mu.RUnlock()
	return cw.config
}

// Close stops the watcher
func (cw *ConfigWatcher) Close() error {
	close(cw.stopCh)
	return cw.watcher.Close()
}

// watch watches for file changes
func (cw *ConfigWatcher) watch() {
	defer cw.watcher.Close()

	for {
		select {
		case event, ok := <-cw.watcher.Events:
			if !ok {
				return
			}

			// Check if the event is for our config file
			if event.Name != cw.configFile {
				continue
			}

			// Check if the event is a write or create event
			if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}

			// Wait a little bit to ensure the file is fully written
			time.Sleep(100 * time.Millisecond)

			// Reload the config
			config, err := loadConfigFromFile(cw.configFile)
			if err != nil {
				cw.logger.Error("Error loading config file", err, map[string]interface{}{
					"file": cw.configFile,
				})
				continue
			}

			// Update the config
			cw.mu.Lock()
			oldConfig := cw.config
			cw.config = config
			callbacks := cw.onChange
			cw.mu.Unlock()

			// Log the change
			cw.logger.Info("Configuration reloaded", map[string]interface{}{
				"file": cw.configFile,
			})

			// Call the callbacks
			for _, callback := range callbacks {
				go callback(config)
			}

			// Log the changes
			logConfigChanges(oldConfig, config, cw.logger)

		case err, ok := <-cw.watcher.Errors:
			if !ok {
				return
			}
			cw.logger.Error("Error watching config file", err)

		case <-cw.stopCh:
			return
		}
	}
}

// loadConfigFromFile loads a Config from a file
func loadConfigFromFile(filename string) (*Config, error) {
	// Read the file
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	// Parse the JSON
	var jsonConfig map[string]interface{}
	if err := json.Unmarshal(data, &jsonConfig); err != nil {
		return nil, fmt.Errorf("error parsing config file: %w", err)
	}

	// Set environment variables from the JSON
	for k, v := range jsonConfig {
		os.Setenv(k, fmt.Sprintf("%v", v))
	}

	// Create a config from environment variables
	return New(WithEnv())
}

// logConfigChanges logs the changes between two configs
func logConfigChanges(old, new *Config, logger Logger) {
	if old.Port != new.Port {
		logger.Info("Port changed", map[string]interface{}{
			"old": old.Port,
			"new": new.Port,
		})
	}

	if old.Symbol != new.Symbol {
		logger.Info("Symbol changed", map[string]interface{}{
			"old": old.Symbol,
			"new": new.Symbol,
		})
	}

	if old.NDays != new.NDays {
		logger.Info("NDays changed", map[string]interface{}{
			"old": old.NDays,
			"new": new.NDays,
		})
	}

	if old.CacheSize != new.CacheSize {
		logger.Info("CacheSize changed", map[string]interface{}{
			"old": old.CacheSize,
			"new": new.CacheSize,
		})
	}

	if old.CacheDuration != new.CacheDuration {
		logger.Info("CacheDuration changed", map[string]interface{}{
			"old": old.CacheDuration,
			"new": new.CacheDuration,
		})
	}

	// Don't log sensitive fields like API keys
}
```

## 3. Configuration File Support

### Problem
The current implementation only supports environment variables for configuration. In many production environments, configuration files are preferred for complex settings.

### Implementation Location
Create a new file at `internal/config/file.go`.

### Required Implementation

```go
package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// LoadFromFile loads configuration from a file
func LoadFromFile(filename string) (*Config, error) {
	// Check if the file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file does not exist: %s", filename)
	}

	// Read the file
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	// Parse the config based on file type
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".json":
		return parseJSONConfig(data)
	case ".env":
		return parseEnvConfig(data)
	default:
		return nil, fmt.Errorf("unsupported config file format: %s", filepath.Ext(filename))
	}
}

// parseJSONConfig parses a JSON config file
func parseJSONConfig(data []byte) (*Config, error) {
	// Parse the JSON
	var jsonConfig map[string]interface{}
	if err := json.Unmarshal(data, &jsonConfig); err != nil {
		return nil, fmt.Errorf("error parsing JSON config: %w", err)
	}

	// Create a map of environment variable settings
	for k, v := range jsonConfig {
		// Handle different types
		var strValue string
		switch val := v.(type) {
		case string:
			strValue = val
		case float64:
			// JSON numbers are always float64
			if val == float64(int64(val)) {
				// It's an integer
				strValue = fmt.Sprintf("%d", int64(val))
			} else {
				strValue = fmt.Sprintf("%g", val)
			}
		case bool:
			strValue = fmt.Sprintf("%t", val)
		case nil:
			continue // Skip null values
		default:
			// Complex values (objects, arrays) are set as JSON strings
			bytes, err := json.Marshal(val)
			if err != nil {
				return nil, fmt.Errorf("error marshaling complex value to JSON: %w", err)
			}
			strValue = string(bytes)
		}

		// Set the environment variable
		os.Setenv(k, strValue)
	}

	// Create config from environment variables
	return New(WithEnv())
}

// parseEnvConfig parses an .env config file
func parseEnvConfig(data []byte) (*Config, error) {
	// Split by newlines
	lines := strings.Split(string(data), "\n")

	// Process each line
	for _, line := range lines {
		// Skip empty lines and comments
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Split key=value
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue // Skip invalid lines
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove quotes if present
		if len(value) >= 2 && (value[0] == '"' && value[len(value)-1] == '"' ||
			value[0] == '\'' && value[len(value)-1] == '\'') {
			value = value[1 : len(value)-1]
		}

		// Set the environment variable
		os.Setenv(key, value)
	}

	// Create config from environment variables
	return New(WithEnv())
}

// WriteConfigFile writes the configuration to a file
func WriteConfigFile(cfg *Config, filename string) error {
	// Create a map of the config values
	configMap := map[string]interface{}{
		"PORT":                   cfg.Port,
		"SYMBOL":                 cfg.Symbol,
		"NDAYS":                  cfg.NDays,
		"API_KEY":                cfg.APIKey,
		"CACHE_SIZE":             cfg.CacheSize,
		"CACHE_DURATION":         cfg.CacheDuration.String(),
		"READ_TIMEOUT":           cfg.ReadTimeout.String(),
		"WRITE_TIMEOUT":          cfg.WriteTimeout.String(),
		"SHUTDOWN_TIMEOUT":       cfg.ShutdownTimeout.String(),
		"REQUEST_TIMEOUT":        cfg.RequestTimeout.String(),
		"MAX_RETRIES":            cfg.MaxRetries,
		"ENABLE_METRICS":         cfg.EnableMetrics,
		"ENABLE_TEST_DATA":       cfg.EnableTestData,
		"ENABLE_RATE_LIMITING":   cfg.EnableRateLimiting,
		"ENABLE_CIRCUIT_BREAKER": cfg.EnableCircuitBreaker,
	}

	// Serialize based on file format
	var data []byte
	var err error

	switch strings.ToLower(filepath.Ext(filename)) {
	case ".json":
		data, err = json.MarshalIndent(configMap, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshaling config to JSON: %w", err)
		}
	case ".env":
		// Build .env file content
		var builder strings.Builder
		for k, v := range configMap {
			builder.WriteString(fmt.Sprintf("%s=%v\n", k, v))
		}
		data = []byte(builder.String())
	default:
		return fmt.Errorf("unsupported config file format: %s", filepath.Ext(filename))
	}

	// Write to file
	if err := ioutil.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("error writing config file: %w", err)
	}

	return nil
}
```

## 4. Secret Management

### Problem
The current implementation stores the API key directly in the environment variables, which is not secure for production use. We need a more secure way to handle sensitive configuration.

### Implementation Location
Create a new file at `internal/config/secrets.go`.

### Required Implementation

```go
package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// SecretStore defines an interface for getting secrets
type SecretStore interface {
	GetSecret(key string) (string, error)
}

// FileSecretStore implements SecretStore using a local file
type FileSecretStore struct {
	filepath string
	cache    map[string]string
}

// NewFileSecretStore creates a new FileSecretStore
func NewFileSecretStore(filepath string) (*FileSecretStore, error) {
	store := &FileSecretStore{
		filepath: filepath,
		cache:    make(map[string]string),
	}

	// Load secrets if file exists
	if _, err := os.Stat(filepath); err == nil {
		if err := store.loadSecrets(); err != nil {
			return nil, err
		}
	}

	return store, nil
}

// GetSecret gets a secret by key
func (s *FileSecretStore) GetSecret(key string) (string, error) {
	// Check if we need to reload
	if _, err := os.Stat(s.filepath); err == nil {
		if err := s.loadSecrets(); err != nil {
			return "", err
		}
	}

	// Check if the secret exists
	if value, exists := s.cache[key]; exists {
		return value, nil
	}

	return "", fmt.Errorf("secret not found: %s", key)
}

// loadSecrets loads secrets from the file
func (s *FileSecretStore) loadSecrets() error {
	// Read the file
	data, err := ioutil.ReadFile(s.filepath)
	if err != nil {
		return fmt.Errorf("error reading secrets file: %w", err)
	}

	// Parse based on file extension
	switch strings.ToLower(filepath.Ext(s.filepath)) {
	case ".json":
		// Parse JSON
		var secrets map[string]string
		if err := json.Unmarshal(data, &secrets); err != nil {
			return fmt.Errorf("error parsing JSON secrets: %w", err)
		}
		s.cache = secrets

	case ".env":
		// Parse .env
		secrets := make(map[string]string)
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}

			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}

			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			// Remove quotes if present
			if len(value) >= 2 && (value[0] == '"' && value[len(value)-1] == '"' ||
				value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}

			secrets[key] = value
		}
		s.cache = secrets

	default:
		return fmt.Errorf("unsupported secrets file format: %s", filepath.Ext(s.filepath))
	}

	return nil
}

// EnvSecretStore implements SecretStore using environment variables
type EnvSecretStore struct{}

// NewEnvSecretStore creates a new EnvSecretStore
func NewEnvSecretStore() *EnvSecretStore {
	return &EnvSecretStore{}
}

// GetSecret gets a secret from environment variables
func (s *EnvSecretStore) GetSecret(key string) (string, error) {
	if value, exists := os.LookupEnv(key); exists {
		return value, nil
	}
	return "", fmt.Errorf("secret not found in environment: %s", key)
}

// WithSecretStore returns an option that configures a Config with a secret store
func WithSecretStore(store SecretStore) Option {
	return func(c *Config) {
		// Check if API_KEY is not set in environment
		if c.APIKey == "" {
			// Try to get from secret store
			if apiKey, err := store.GetSecret("API_KEY"); err == nil {
				c.APIKey = apiKey
			}
		}

		// Add other sensitive configurations here
	}
}
```

## 5. Integration in Main.go

### Implementation Location
Update `cmd/stockticker/main.go` to use the improved configuration system.

### Required Implementation

```go
package main

import (
	// ... existing imports
	"flag"
	"path/filepath"
)

func main() {
	// Parse command line flags
	configFile := flag.String("config", "", "Path to configuration file")
	secretsFile := flag.String("secrets", "", "Path to secrets file")
	flag.Parse()

	// Initialize logger
	logger := logging.New(
		logging.WithLevel(logging.InfoLevel),
		logging.WithFields(logging.Fields{
			"service": "stockticker",
		}),
	)

	// Load configuration
	var cfg *config.Config
	var err error
	var configWatcher *config.ConfigWatcher

	if *configFile != "" {
		// Load from file
		logger.Info("Loading configuration from file", logging.Fields{
			"file": *configFile,
		})

		// If config file is provided, load from it
		cfg, err = config.LoadFromFile(*configFile)
		if err != nil {
			logger.Fatal("Error loading configuration", err, logging.Fields{
				"file": *configFile,
			})
		}

		// Start config watcher if the file exists
		if _, err := os.Stat(*configFile); err == nil {
			configWatcher, err = config.NewConfigWatcher(
				*configFile,
				config.WithLogger(logger),
			)
			if err != nil {
				logger.Error("Error setting up config watcher", err)
			} else {
				defer configWatcher.Close()
				logger.Info("Configuration hot-reloading enabled", logging.Fields{
					"file": *configFile,
				})

				// Add callback for configuration changes
				configWatcher.OnChange(func(newCfg *config.Config) {
					// Handle config changes
					logger.Info("Configuration changed", logging.Fields{
						"new_config": newCfg.String(),
					})

					// Update components that can be updated without restart
					// For example, update cache size, logging level, etc.
				})
			}
		}
	} else {
		// Load from environment
		logger.Info("Loading configuration from environment")

		// Setup secret store if provided
		var opts []config.Option
		opts = append(opts, config.WithEnv())

		if *secretsFile != "" {
			secretStore, err := config.NewFileSecretStore(*secretsFile)
			if err != nil {
				logger.Error("Error setting up secret store", err)
			} else {
				opts = append(opts, config.WithSecretStore(secretStore))
				logger.Info("Using secrets file", logging.Fields{
					"file": *secretsFile,
				})
			}
		}

		// Create config with options
		cfg, err = config.New(opts...)
		if err != nil {
			logger.Fatal("Error loading configuration", err)
		}
	}

	// Log configuration (with sensitive fields masked)
	logger.Info("Configuration loaded", logging.Fields{
		"config": cfg.String(),
	})

	// ... rest of initialization code

	// Create API client with configuration
	apiClient := client.NewAlphaVantage(
		cfg.APIKey,
		client.WithRequestTimeout(cfg.RequestTimeout),
		client.WithMaxRetries(cfg.MaxRetries),
	)

	// Create cache with configuration
	cacheInstance := cache.New(
		cache.WithMaxItems(cfg.CacheSize),
		cache.WithDefaultDuration(cfg.CacheDuration),
	)

	// ... rest of initialization code

	// Configure server with timeouts from config
	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      handler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.ShutdownTimeout,
	}

	// ... rest of the code

	// In shutdown handler, use the configured timeout
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", err)
	}
}
```

## 6. Example Configuration Files

### Implementation Location
Create a set of example configuration files for production, development, and testing.

### Example Config Files

#### `config/production.json`
```json
{
  "PORT": "8080",
  "SYMBOL": "AAPL",
  "NDAYS": 30,
  "CACHE_SIZE": 10000,
  "CACHE_DURATION": "15m",
  "READ_TIMEOUT": "5s",
  "WRITE_TIMEOUT": "10s",
  "SHUTDOWN_TIMEOUT": "30s",
  "REQUEST_TIMEOUT": "5s",
  "MAX_RETRIES": 3,
  "ENABLE_METRICS": true,
  "ENABLE_TEST_DATA": false,
  "ENABLE_RATE_LIMITING": true,
  "ENABLE_CIRCUIT_BREAKER": true
}
```

#### `config/development.json`
```json
{
  "PORT": "8080",
  "SYMBOL": "IBM",
  "NDAYS": 7,
  "CACHE_SIZE": 1000,
  "CACHE_DURATION": "5m",
  "READ_TIMEOUT": "10s",
  "WRITE_TIMEOUT": "20s",
  "SHUTDOWN_TIMEOUT": "30s",
  "REQUEST_TIMEOUT": "10s",
  "MAX_RETRIES": 2,
  "ENABLE_METRICS": true,
  "ENABLE_TEST_DATA": true,
  "ENABLE_RATE_LIMITING": true,
  "ENABLE_CIRCUIT_BREAKER": true
}
```

#### `config/testing.env`
```
PORT=8080
SYMBOL=MSFT
NDAYS=7
CACHE_SIZE=100
CACHE_DURATION=1m
READ_TIMEOUT=5s
WRITE_TIMEOUT=5s
SHUTDOWN_TIMEOUT=5s
REQUEST_TIMEOUT=2s
MAX_RETRIES=1
ENABLE_METRICS=false
ENABLE_TEST_DATA=true
ENABLE_RATE_LIMITING=false
ENABLE_CIRCUIT_BREAKER=false
```

## 7. Docker and Kubernetes Integration

### Implementation Location
Update `Dockerfile` and Kubernetes manifests to use the configuration files.

### Dockerfile Changes

```Dockerfile
# Build stage
FROM golang:1.19-alpine AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o stockticker cmd/stockticker/main.go

# Runtime stage
FROM alpine:3.16

WORKDIR /app

COPY --from=build /app/stockticker .
COPY config/production.json /app/config/config.json
COPY config/secrets.json /app/config/secrets.json

# Create a non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

# Set up environment
ENV CONFIG_FILE=/app/config/config.json
ENV SECRETS_FILE=/app/config/secrets.json

EXPOSE 8080
ENTRYPOINT ["/app/stockticker", "-config", "/app/config/config.json", "-secrets", "/app/config/secrets.json"]
```

### Kubernetes ConfigMap and Secret

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: stockticker-config
data:
  config.json: |
    {
      "PORT": "8080",
      "SYMBOL": "AAPL",
      "NDAYS": 30,
      "CACHE_SIZE": 10000,
      "CACHE_DURATION": "15m",
      "READ_TIMEOUT": "5s",
      "WRITE_TIMEOUT": "10s",
      "SHUTDOWN_TIMEOUT": "30s",
      "REQUEST_TIMEOUT": "5s",
      "MAX_RETRIES": 3,
      "ENABLE_METRICS": true,
      "ENABLE_TEST_DATA": false,
      "ENABLE_RATE_LIMITING": true,
      "ENABLE_CIRCUIT_BREAKER": true
    }
---
apiVersion: v1
kind: Secret
metadata:
  name: stockticker-secrets
type: Opaque
data:
  secrets.json: eyJBUElfS0VZIjoieW91cl9hcGlfa2V5X2hlcmUifQ==  # Base64 encoded: {"API_KEY":"your_api_key_here"}
```

### Kubernetes Deployment Changes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: stockticker
spec:
  replicas: 3
  selector:
    matchLabels:
      app: stockticker
  template:
    metadata:
      labels:
        app: stockticker
    spec:
      containers:
      - name: stockticker
        image: stockticker:1.0.0
        ports:
        - containerPort: 8080
        volumeMounts:
        - name: config-volume
          mountPath: /app/config/config.json
          subPath: config.json
        - name: secrets-volume
          mountPath: /app/config/secrets.json
          subPath: secrets.json
        args:
        - "-config"
        - "/app/config/config.json"
        - "-secrets"
        - "/app/config/secrets.json"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 5
        readinessProbe:
          httpGet:
            path: /health/ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
      volumes:
      - name: config-volume
        configMap:
          name: stockticker-config
      - name: secrets-volume
        secret:
          secretName: stockticker-secrets
```

## Benefits

1. **Improved Validation**: Ensures all configuration is valid and consistent before the application starts
2. **More Flexible Configuration**: Support for both environment variables and configuration files
3. **Hot Reloading**: Ability to change certain configuration parameters without restarting the service
4. **Better Security**: Secure handling of sensitive configuration like API keys
5. **Clearer Documentation**: Example configuration files help document the available settings
6. **Kubernetes Integration**: Proper integration with Kubernetes ConfigMaps and Secrets
7. **Dev/Prod Parity**: Consistent configuration across development, testing, and production environments

## Codebase Analysis

The current configuration implementation in the stockticker service is minimal, with limited validation and no support for configuration files or hot reloading. The proposed improvements add robust validation, support for configuration files, secure handling of sensitive configuration, and integration with Kubernetes, making the service much easier to configure and operate in various environments.