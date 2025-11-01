package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	promexporter_config "github.com/d0ugal/promexporter/config"
	"gopkg.in/yaml.v3"
)

// Duration uses promexporter Duration type
type Duration = promexporter_config.Duration

type Config struct {
	promexporter_config.BaseConfig

	Backends map[string]BackendConfig `yaml:"backends"`
}

type BackendConfig struct {
	Type     string   `yaml:"type"`     // "speedtest" or "fast"
	Enabled  bool     `yaml:"enabled"`  // Enable this backend
	Interval Duration `yaml:"interval"` // Test interval
	Timeout  Duration `yaml:"timeout"`  // Test timeout

	// Speedtest-specific config
	Speedtest *SpeedtestConfig `yaml:"speedtest,omitempty"`

	// Fast.com-specific config
	Fast *FastConfig `yaml:"fast,omitempty"`
}

type SpeedtestConfig struct {
	ServerID      int    `yaml:"server_id,omitempty"`      // Specific server ID (0 = auto-select)
	ServerName    string `yaml:"server_name,omitempty"`    // Server name filter
	ServerCountry string `yaml:"server_country,omitempty"` // Server country filter
}

type FastConfig struct {
	// Fast.com doesn't have server selection, but we can configure retries
	Retries int `yaml:"retries,omitempty"` // Number of retries on failure (default: 1)
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(path string, configFromEnv bool) (*Config, error) {
	if configFromEnv {
		return loadFromEnv()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply tracing configuration from environment variables (can override config file)
	applyTracingFromEnv(&config)

	// Set defaults
	setDefaults(&config)

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return &config, nil
}

// loadFromEnv loads configuration from environment variables
func loadFromEnv() (*Config, error) {
	config := &Config{}

	// Load base configuration from environment
	baseConfig := &promexporter_config.BaseConfig{}

	// Server configuration
	if address := os.Getenv("INTERNET_PERF_EXPORTER_SERVER_ADDRESS"); address != "" {
		if host, portStr, err := net.SplitHostPort(address); err == nil {
			baseConfig.Server.Host = host
			if port, err := strconv.Atoi(portStr); err != nil {
				return nil, fmt.Errorf("invalid server port in address: %w", err)
			} else {
				baseConfig.Server.Port = port
			}
		} else {
			return nil, fmt.Errorf("invalid server address format: %w", err)
		}
	} else {
		if host := os.Getenv("INTERNET_PERF_EXPORTER_SERVER_HOST"); host != "" {
			baseConfig.Server.Host = host
		} else {
			baseConfig.Server.Host = "0.0.0.0"
		}
		if portStr := os.Getenv("INTERNET_PERF_EXPORTER_SERVER_PORT"); portStr != "" {
			if port, err := strconv.Atoi(portStr); err != nil {
				return nil, fmt.Errorf("invalid server port: %w", err)
			} else {
				baseConfig.Server.Port = port
			}
		} else {
			baseConfig.Server.Port = 8080
		}
	}

	// Logging configuration
	if level := os.Getenv("INTERNET_PERF_EXPORTER_LOGGING_LEVEL"); level != "" {
		baseConfig.Logging.Level = level
	} else {
		baseConfig.Logging.Level = "info"
	}

	if format := os.Getenv("INTERNET_PERF_EXPORTER_LOGGING_FORMAT"); format != "" {
		baseConfig.Logging.Format = format
	} else {
		baseConfig.Logging.Format = "json"
	}

	// Metrics configuration
	if intervalStr := os.Getenv("INTERNET_PERF_EXPORTER_METRICS_COLLECTION_DEFAULT_INTERVAL"); intervalStr != "" {
		if interval, err := time.ParseDuration(intervalStr); err != nil {
			return nil, fmt.Errorf("invalid metrics default interval: %w", err)
		} else {
			baseConfig.Metrics.Collection.DefaultInterval = promexporter_config.Duration{Duration: interval}
			baseConfig.Metrics.Collection.DefaultIntervalSet = true
		}
	} else {
		baseConfig.Metrics.Collection.DefaultInterval = promexporter_config.Duration{Duration: time.Minute * 5}
	}

	// Tracing configuration
	if enabledStr := os.Getenv("TRACING_ENABLED"); enabledStr != "" {
		enabled := enabledStr == "true"
		baseConfig.Tracing.Enabled = &enabled
	}

	if serviceName := os.Getenv("TRACING_SERVICE_NAME"); serviceName != "" {
		baseConfig.Tracing.ServiceName = serviceName
	}

	if endpoint := os.Getenv("TRACING_ENDPOINT"); endpoint != "" {
		baseConfig.Tracing.Endpoint = endpoint
	}

	config.BaseConfig = *baseConfig

	// Load backends from environment variables
	config.loadBackendsFromEnv()

	// Set defaults for any missing values
	setDefaults(config)

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

// loadBackendsFromEnv loads backend configuration from environment variables
func (c *Config) loadBackendsFromEnv() {
	if c.Backends == nil {
		c.Backends = make(map[string]BackendConfig)
	}

	// Check for speedtest backend
	if enabledStr := os.Getenv("INTERNET_PERF_EXPORTER_SPEEDTEST_ENABLED"); enabledStr == "true" {
		backend := BackendConfig{
			Type:    "speedtest",
			Enabled: true,
		}

		if intervalStr := os.Getenv("INTERNET_PERF_EXPORTER_SPEEDTEST_INTERVAL"); intervalStr != "" {
			if interval, err := time.ParseDuration(intervalStr); err == nil {
				backend.Interval = Duration{Duration: interval}
			}
		}

		if timeoutStr := os.Getenv("INTERNET_PERF_EXPORTER_SPEEDTEST_TIMEOUT"); timeoutStr != "" {
			if timeout, err := time.ParseDuration(timeoutStr); err == nil {
				backend.Timeout = Duration{Duration: timeout}
			}
		}

		speedtestConfig := &SpeedtestConfig{}
		if serverIDStr := os.Getenv("INTERNET_PERF_EXPORTER_SPEEDTEST_SERVER_ID"); serverIDStr != "" {
			if serverID, err := strconv.Atoi(serverIDStr); err == nil {
				speedtestConfig.ServerID = serverID
			}
		}
		if serverName := os.Getenv("INTERNET_PERF_EXPORTER_SPEEDTEST_SERVER_NAME"); serverName != "" {
			speedtestConfig.ServerName = serverName
		}
		if serverCountry := os.Getenv("INTERNET_PERF_EXPORTER_SPEEDTEST_SERVER_COUNTRY"); serverCountry != "" {
			speedtestConfig.ServerCountry = serverCountry
		}

		if speedtestConfig.ServerID != 0 || speedtestConfig.ServerName != "" || speedtestConfig.ServerCountry != "" {
			backend.Speedtest = speedtestConfig
		}

		c.Backends["speedtest"] = backend
	}

	// Check for fast.com backend
	if enabledStr := os.Getenv("INTERNET_PERF_EXPORTER_FAST_ENABLED"); enabledStr == "true" {
		backend := BackendConfig{
			Type:    "fast",
			Enabled: true,
		}

		if intervalStr := os.Getenv("INTERNET_PERF_EXPORTER_FAST_INTERVAL"); intervalStr != "" {
			if interval, err := time.ParseDuration(intervalStr); err == nil {
				backend.Interval = Duration{Duration: interval}
			}
		}

		if timeoutStr := os.Getenv("INTERNET_PERF_EXPORTER_FAST_TIMEOUT"); timeoutStr != "" {
			if timeout, err := time.ParseDuration(timeoutStr); err == nil {
				backend.Timeout = Duration{Duration: timeout}
			}
		}

		fastConfig := &FastConfig{}
		if retriesStr := os.Getenv("INTERNET_PERF_EXPORTER_FAST_RETRIES"); retriesStr != "" {
			if retries, err := strconv.Atoi(retriesStr); err == nil && retries > 0 {
				fastConfig.Retries = retries
			}
		}

		if fastConfig.Retries > 0 {
			backend.Fast = fastConfig
		}

		c.Backends["fast"] = backend
	}
}

// applyTracingFromEnv applies tracing configuration from environment variables to an existing config
func applyTracingFromEnv(config *Config) {
	if enabledStr := os.Getenv("TRACING_ENABLED"); enabledStr != "" {
		enabled := enabledStr == "true"
		config.Tracing.Enabled = &enabled
	}

	if serviceName := os.Getenv("TRACING_SERVICE_NAME"); serviceName != "" {
		config.Tracing.ServiceName = serviceName
	}

	if endpoint := os.Getenv("TRACING_ENDPOINT"); endpoint != "" {
		config.Tracing.Endpoint = endpoint
	}
}

// setDefaults sets default values for configuration
func setDefaults(config *Config) {
	if config.Server.Host == "" {
		config.Server.Host = "0.0.0.0"
	}

	if config.Server.Port == 0 {
		config.Server.Port = 8080
	}

	if config.Logging.Level == "" {
		config.Logging.Level = "info"
	}

	if config.Logging.Format == "" {
		config.Logging.Format = "json"
	}

	if !config.Metrics.Collection.DefaultIntervalSet {
		config.Metrics.Collection.DefaultInterval = promexporter_config.Duration{Duration: time.Minute * 5}
	}

	// Set defaults for backends
	for name, backend := range config.Backends {
		if backend.Interval.Duration == 0 {
			backend.Interval = Duration{Duration: time.Hour}
			config.Backends[name] = backend
		}
		if backend.Timeout.Duration == 0 {
			backend.Timeout = Duration{Duration: time.Minute * 5}
			config.Backends[name] = backend
		}
	}

	// If no backends configured, enable speedtest with defaults
	if len(config.Backends) == 0 {
		config.Backends = map[string]BackendConfig{
			"speedtest": {
				Type:     "speedtest",
				Enabled:  true,
				Interval: Duration{Duration: time.Hour},
				Timeout:  Duration{Duration: time.Minute * 5},
			},
		}
	}
}

// Validate performs comprehensive validation of the configuration
func (c *Config) Validate() error {
	if err := c.validateServerConfig(); err != nil {
		return fmt.Errorf("server config: %w", err)
	}

	if err := c.validateLoggingConfig(); err != nil {
		return fmt.Errorf("logging config: %w", err)
	}

	if err := c.validateMetricsConfig(); err != nil {
		return fmt.Errorf("metrics config: %w", err)
	}

	if err := c.validateBackendsConfig(); err != nil {
		return fmt.Errorf("backends config: %w", err)
	}

	return nil
}

func (c *Config) validateServerConfig() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", c.Server.Port)
	}
	return nil
}

func (c *Config) validateLoggingConfig() error {
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLevels[c.Logging.Level] {
		return fmt.Errorf("invalid logging level: %s", c.Logging.Level)
	}

	validFormats := map[string]bool{
		"json": true,
		"text": true,
	}
	if !validFormats[c.Logging.Format] {
		return fmt.Errorf("invalid logging format: %s", c.Logging.Format)
	}

	return nil
}

func (c *Config) validateMetricsConfig() error {
	if c.Metrics.Collection.DefaultInterval.Seconds() < 60 {
		return fmt.Errorf("default interval must be at least 60 seconds, got %d", int(c.Metrics.Collection.DefaultInterval.Seconds()))
	}

	if c.Metrics.Collection.DefaultInterval.Seconds() > 86400 {
		return fmt.Errorf("default interval must be at most 86400 seconds (24 hours), got %d", int(c.Metrics.Collection.DefaultInterval.Seconds()))
	}

	return nil
}

func (c *Config) validateBackendsConfig() error {
	enabledCount := 0
	for name, backend := range c.Backends {
		if name == "" {
			return fmt.Errorf("backend name cannot be empty")
		}

		if backend.Type != "speedtest" && backend.Type != "fast" {
			return fmt.Errorf("invalid backend type: %s (must be 'speedtest' or 'fast')", backend.Type)
		}

		if backend.Enabled {
			enabledCount++
			if backend.Interval.Seconds() < 60 {
				return fmt.Errorf("backend %s interval must be at least 60 seconds, got %d", name, int(backend.Interval.Seconds()))
			}

			if backend.Timeout.Seconds() < 10 {
				return fmt.Errorf("backend %s timeout must be at least 10 seconds, got %d", name, int(backend.Timeout.Seconds()))
			}
		}
	}

	if enabledCount == 0 {
		return fmt.Errorf("at least one backend must be enabled")
	}

	return nil
}

// GetLogging returns the logging configuration
func (c *Config) GetLogging() *promexporter_config.LoggingConfig {
	return c.BaseConfig.GetLogging()
}

// GetServer returns the server configuration
func (c *Config) GetServer() *promexporter_config.ServerConfig {
	return c.BaseConfig.GetServer()
}

// GetDisplayConfig returns configuration data safe for display
func (c *Config) GetDisplayConfig() map[string]interface{} {
	config := c.BaseConfig.GetDisplayConfig()

	if len(c.Backends) > 0 {
		backends := make(map[string]interface{})
		for name, backend := range c.Backends {
			backends[name] = map[string]interface{}{
				"type":     backend.Type,
				"enabled":  backend.Enabled,
				"interval": backend.Interval.String(),
				"timeout":  backend.Timeout.String(),
			}
		}
		config["Backends"] = backends
	} else {
		config["Backends"] = "None configured"
	}

	return config
}
