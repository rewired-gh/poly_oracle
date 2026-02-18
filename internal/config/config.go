// Package config handles application configuration loading and validation.
// It supports YAML configuration files with environment variable overrides,
// providing a flexible and robust configuration system.
//
// Configuration is loaded from a YAML file and can be overridden with environment
// variables prefixed with POLY_ORACLE_. All configuration values are validated
// at startup to prevent runtime errors.
package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config represents the complete application configuration
type Config struct {
	Polymarket PolymarketConfig `mapstructure:"polymarket"`
	Monitor    MonitorConfig    `mapstructure:"monitor"`
	Telegram   TelegramConfig   `mapstructure:"telegram"`
	Storage    StorageConfig    `mapstructure:"storage"`
	Logging    LoggingConfig    `mapstructure:"logging"`
}

// PolymarketConfig holds Polymarket API configuration
type PolymarketConfig struct {
	GammaAPIURL         string        `mapstructure:"gamma_api_url"`
	CLOBAPIURL          string        `mapstructure:"clob_api_url"`
	PollInterval        time.Duration `mapstructure:"poll_interval"`
	Categories          []string      `mapstructure:"categories"`
	Volume24hrMin       float64       `mapstructure:"volume_24hr_min"`
	Volume1wkMin        float64       `mapstructure:"volume_1wk_min"`
	Volume1moMin        float64       `mapstructure:"volume_1mo_min"`
	VolumeFilterOR      bool          `mapstructure:"volume_filter_or"` // true = OR (union), false = AND (intersection)
	Limit               int           `mapstructure:"limit"`
	Timeout             time.Duration `mapstructure:"timeout"`
	MaxRetries          int           `mapstructure:"max_retries"`
	RetryDelayBase      time.Duration `mapstructure:"retry_delay_base"`
	MaxIdleConns        int           `mapstructure:"max_idle_conns"`
	MaxIdleConnsPerHost int           `mapstructure:"max_idle_conns_per_host"`
	IdleConnTimeout     time.Duration `mapstructure:"idle_conn_timeout"`
}

// MonitorConfig holds monitoring behavior configuration
type MonitorConfig struct {
	Sensitivity        float64 `mapstructure:"sensitivity"`
	TopK               int     `mapstructure:"top_k"`
	Enabled            bool    `mapstructure:"enabled"`
	DetectionIntervals int     `mapstructure:"detection_intervals"`
	MinAbsChange       float64 `mapstructure:"min_abs_change"` // minimum absolute probability change (fraction, e.g. 0.03 = 3pp)
	MinBaseProb        float64 `mapstructure:"min_base_prob"`  // minimum base probability (fraction, e.g. 0.05 = 5%)
}

// MinCompositeScore returns the minimum composite score floor derived from sensitivity.
// Formula: sensitivity^2 × 0.05. At sensitivity=0.5 this yields 0.0125 (medium signals pass).
func (m MonitorConfig) MinCompositeScore() float64 {
	return m.Sensitivity * m.Sensitivity * 0.05
}

// TelegramConfig holds Telegram notification configuration
type TelegramConfig struct {
	BotToken       string        `mapstructure:"bot_token"`
	ChatID         string        `mapstructure:"chat_id"`
	Enabled        bool          `mapstructure:"enabled"`
	MaxRetries     int           `mapstructure:"max_retries"`
	RetryDelayBase time.Duration `mapstructure:"retry_delay_base"`
}

// StorageConfig holds storage and persistence configuration
type StorageConfig struct {
	MaxEvents             int           `mapstructure:"max_events"`
	MaxSnapshotsPerEvent  int           `mapstructure:"max_snapshots_per_event"`
	MaxFileSizeMB         int           `mapstructure:"max_file_size_mb"`
	PersistenceRetries    int           `mapstructure:"persistence_retries"`
	PersistenceRetryDelay time.Duration `mapstructure:"persistence_retry_delay"`
	FilePermissions       uint32        `mapstructure:"file_permissions"`
	DirPermissions        uint32        `mapstructure:"dir_permissions"`
	FilePath              string        `mapstructure:"file_path"`
	DataDir               string        `mapstructure:"data_dir"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// Load reads configuration from file and environment variables
func Load(path string) (*Config, error) {
	v := viper.New()

	// Set config file
	v.SetConfigFile(path)

	// Set defaults
	setDefaults(v)

	// Enable environment variable override
	v.SetEnvPrefix("POLY_ORACLE")
	v.AutomaticEnv()

	// Bind environment variables to nested config keys
	// Polymarket
	_ = v.BindEnv("polymarket.gamma_api_url", "POLY_ORACLE_POLYMARKET_GAMMA_API_URL")
	_ = v.BindEnv("polymarket.clob_api_url", "POLY_ORACLE_POLYMARKET_CLOB_API_URL")
	_ = v.BindEnv("polymarket.poll_interval", "POLY_ORACLE_POLYMARKET_POLL_INTERVAL")
	_ = v.BindEnv("polymarket.categories", "POLY_ORACLE_POLYMARKET_CATEGORIES")
	_ = v.BindEnv("polymarket.volume_24hr_min", "POLY_ORACLE_POLYMARKET_VOLUME_24HR_MIN")
	_ = v.BindEnv("polymarket.volume_1wk_min", "POLY_ORACLE_POLYMARKET_VOLUME_1WK_MIN")
	_ = v.BindEnv("polymarket.volume_1mo_min", "POLY_ORACLE_POLYMARKET_VOLUME_1MO_MIN")
	_ = v.BindEnv("polymarket.volume_filter_or", "POLY_ORACLE_POLYMARKET_VOLUME_FILTER_OR")
	_ = v.BindEnv("polymarket.limit", "POLY_ORACLE_POLYMARKET_LIMIT")
	_ = v.BindEnv("polymarket.timeout", "POLY_ORACLE_POLYMARKET_TIMEOUT")
	_ = v.BindEnv("polymarket.max_retries", "POLY_ORACLE_POLYMARKET_MAX_RETRIES")
	_ = v.BindEnv("polymarket.retry_delay_base", "POLY_ORACLE_POLYMARKET_RETRY_DELAY_BASE")
	_ = v.BindEnv("polymarket.max_idle_conns", "POLY_ORACLE_POLYMARKET_MAX_IDLE_CONNS")
	_ = v.BindEnv("polymarket.max_idle_conns_per_host", "POLY_ORACLE_POLYMARKET_MAX_IDLE_CONNS_PER_HOST")
	_ = v.BindEnv("polymarket.idle_conn_timeout", "POLY_ORACLE_POLYMARKET_IDLE_CONN_TIMEOUT")

	// Monitor
	_ = v.BindEnv("monitor.sensitivity", "POLY_ORACLE_MONITOR_SENSITIVITY")
	_ = v.BindEnv("monitor.top_k", "POLY_ORACLE_MONITOR_TOP_K")
	_ = v.BindEnv("monitor.enabled", "POLY_ORACLE_MONITOR_ENABLED")
	_ = v.BindEnv("monitor.detection_intervals", "POLY_ORACLE_MONITOR_DETECTION_INTERVALS")
	_ = v.BindEnv("monitor.min_abs_change", "POLY_ORACLE_MONITOR_MIN_ABS_CHANGE")
	_ = v.BindEnv("monitor.min_base_prob", "POLY_ORACLE_MONITOR_MIN_BASE_PROB")

	// Telegram
	_ = v.BindEnv("telegram.bot_token", "POLY_ORACLE_TELEGRAM_BOT_TOKEN")
	_ = v.BindEnv("telegram.chat_id", "POLY_ORACLE_TELEGRAM_CHAT_ID")
	_ = v.BindEnv("telegram.enabled", "POLY_ORACLE_TELEGRAM_ENABLED")
	_ = v.BindEnv("telegram.max_retries", "POLY_ORACLE_TELEGRAM_MAX_RETRIES")
	_ = v.BindEnv("telegram.retry_delay_base", "POLY_ORACLE_TELEGRAM_RETRY_DELAY_BASE")

	// Storage
	_ = v.BindEnv("storage.max_events", "POLY_ORACLE_STORAGE_MAX_EVENTS")
	_ = v.BindEnv("storage.max_snapshots_per_event", "POLY_ORACLE_STORAGE_MAX_SNAPSHOTS_PER_EVENT")
	_ = v.BindEnv("storage.max_file_size_mb", "POLY_ORACLE_STORAGE_MAX_FILE_SIZE_MB")
	_ = v.BindEnv("storage.persistence_retries", "POLY_ORACLE_STORAGE_PERSISTENCE_RETRIES")
	_ = v.BindEnv("storage.persistence_retry_delay", "POLY_ORACLE_STORAGE_PERSISTENCE_RETRY_DELAY")
	_ = v.BindEnv("storage.file_permissions", "POLY_ORACLE_STORAGE_FILE_PERMISSIONS")
	_ = v.BindEnv("storage.dir_permissions", "POLY_ORACLE_STORAGE_DIR_PERMISSIONS")
	_ = v.BindEnv("storage.file_path", "POLY_ORACLE_STORAGE_FILE_PATH")
	_ = v.BindEnv("storage.data_dir", "POLY_ORACLE_STORAGE_DATA_DIR")

	// Logging
	_ = v.BindEnv("logging.level", "POLY_ORACLE_LOGGING_LEVEL")
	_ = v.BindEnv("logging.format", "POLY_ORACLE_LOGGING_FORMAT")

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Unmarshal into Config struct
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

// setDefaults configures default values for all configuration options
func setDefaults(v *viper.Viper) {
	// Polymarket defaults
	v.SetDefault("polymarket.gamma_api_url", "https://gamma-api.polymarket.com")
	v.SetDefault("polymarket.clob_api_url", "https://clob.polymarket.com")
	v.SetDefault("polymarket.poll_interval", "1h") // 1 hour (matches notification rhythm)
	// Categories default: include crypto and world for broader coverage
	v.SetDefault("polymarket.categories", []string{"geopolitics", "tech", "finance", "crypto", "world"})
	// Volume filters: optimized based on analysis of 228 events
	v.SetDefault("polymarket.volume_24hr_min", 25000.0) // $25K minimum
	v.SetDefault("polymarket.volume_1wk_min", 100000.0) // $100K weekly
	v.SetDefault("polymarket.volume_1mo_min", 250000.0) // $250K monthly
	v.SetDefault("polymarket.volume_filter_or", true)   // true = OR (union)
	v.SetDefault("polymarket.limit", 500)
	v.SetDefault("polymarket.timeout", "30s")
	v.SetDefault("polymarket.max_retries", 3)
	v.SetDefault("polymarket.retry_delay_base", "1s")
	v.SetDefault("polymarket.max_idle_conns", 100)
	v.SetDefault("polymarket.max_idle_conns_per_host", 10)
	v.SetDefault("polymarket.idle_conn_timeout", "90s")

	// Monitor defaults
	v.SetDefault("monitor.sensitivity", 0.5) // medium quality bar
	v.SetDefault("monitor.top_k", 5)         // Top 5 events (digestible)
	v.SetDefault("monitor.enabled", true)
	v.SetDefault("monitor.detection_intervals", 4) // 4 poll intervals for TC window
	v.SetDefault("monitor.min_abs_change", 0.03)   // 3pp minimum absolute change
	v.SetDefault("monitor.min_base_prob", 0.05)    // 5% minimum base probability

	// Telegram defaults
	v.SetDefault("telegram.enabled", false)
	v.SetDefault("telegram.max_retries", 3)
	v.SetDefault("telegram.retry_delay_base", "1s")

	// Storage defaults (tuned for 4 GiB RAM / 128 GiB SSD)
	v.SetDefault("storage.max_events", 10000)
	v.SetDefault("storage.max_snapshots_per_event", 672) // 7 days of 15-min snapshots
	v.SetDefault("storage.max_file_size_mb", 2048)       // 2 GB persistence file
	v.SetDefault("storage.persistence_retries", 3)       // Retry 3 times on failure
	v.SetDefault("storage.persistence_retry_delay", "5s")
	v.SetDefault("storage.file_permissions", 0600)
	v.SetDefault("storage.dir_permissions", 0700)
	v.SetDefault("storage.file_path", "")
	v.SetDefault("storage.data_dir", "")

	// Logging defaults
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
}

// Validate checks that all configuration values are valid
func (c *Config) Validate() error {
	// Validate Polymarket config
	if c.Polymarket.GammaAPIURL == "" {
		return fmt.Errorf("polymarket.gamma_api_url is required")
	}
	if c.Polymarket.CLOBAPIURL == "" {
		return fmt.Errorf("polymarket.clob_api_url is required")
	}
	if c.Polymarket.PollInterval < 1*time.Minute {
		return fmt.Errorf("polymarket.poll_interval must be at least 1 minute")
	}
	if len(c.Polymarket.Categories) == 0 {
		return fmt.Errorf("polymarket.categories must contain at least one category")
	}
	if c.Polymarket.Volume24hrMin < 0 {
		return fmt.Errorf("polymarket.volume_24hr_min must not be negative")
	}
	if c.Polymarket.Volume1wkMin < 0 {
		return fmt.Errorf("polymarket.volume_1wk_min must not be negative")
	}
	if c.Polymarket.Volume1moMin < 0 {
		return fmt.Errorf("polymarket.volume_1mo_min must not be negative")
	}
	if c.Polymarket.Limit < 1 || c.Polymarket.Limit > 10000 {
		return fmt.Errorf("polymarket.limit must be between 1 and 10000")
	}

	// Validate Monitor config
	if c.Monitor.Sensitivity < 0.0 || c.Monitor.Sensitivity > 1.0 {
		return fmt.Errorf("monitor.sensitivity must be between 0.0 and 1.0")
	}
	if c.Monitor.TopK < 0 {
		return fmt.Errorf("monitor.top_k must not be negative")
	}
	if c.Monitor.DetectionIntervals < 2 {
		return fmt.Errorf("monitor.detection_intervals must be at least 2 (need ≥2 poll intervals for TC computation)")
	}
	if c.Monitor.MinAbsChange < 0.0 || c.Monitor.MinAbsChange > 1.0 {
		return fmt.Errorf("monitor.min_abs_change must be between 0.0 and 1.0")
	}
	if c.Monitor.MinBaseProb < 0.0 || c.Monitor.MinBaseProb >= 0.5 {
		return fmt.Errorf("monitor.min_base_prob must be in [0.0, 0.5)")
	}

	// Validate Telegram config
	if c.Telegram.Enabled {
		if c.Telegram.BotToken == "" {
			return fmt.Errorf("telegram.bot_token is required when telegram is enabled")
		}
		if c.Telegram.ChatID == "" {
			return fmt.Errorf("telegram.chat_id is required when telegram is enabled")
		}
	}

	// Validate Storage config
	if c.Storage.MaxEvents < 1 {
		return fmt.Errorf("storage.max_events must be at least 1")
	}
	if c.Storage.MaxSnapshotsPerEvent < 10 {
		return fmt.Errorf("storage.max_snapshots_per_event must be at least 10")
	}
	if c.Storage.MaxFileSizeMB < 1 {
		return fmt.Errorf("storage.max_file_size_mb must be at least 1")
	}
	// FilePath can be empty - storage layer will use OS tmp directory

	// Validate Logging config
	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevels[c.Logging.Level] {
		return fmt.Errorf("logging.level must be one of: debug, info, warn, error")
	}
	validFormats := map[string]bool{"json": true, "text": true}
	if !validFormats[c.Logging.Format] {
		return fmt.Errorf("logging.format must be one of: json, text")
	}

	return nil
}

// GetPolymarketConfig returns the Polymarket configuration
func (c *Config) GetPolymarketConfig() PolymarketConfig {
	return c.Polymarket
}

// GetMonitorConfig returns the Monitor configuration
func (c *Config) GetMonitorConfig() MonitorConfig {
	return c.Monitor
}

// GetTelegramConfig returns the Telegram configuration
func (c *Config) GetTelegramConfig() TelegramConfig {
	return c.Telegram
}

// GetStorageConfig returns the Storage configuration
func (c *Config) GetStorageConfig() StorageConfig {
	return c.Storage
}

// GetLoggingConfig returns the Logging configuration
func (c *Config) GetLoggingConfig() LoggingConfig {
	return c.Logging
}
