// Package config handles YAML configuration loading with environment variable overrides.
package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config represents the complete application configuration.
type Config struct {
	Polymarket PolymarketConfig `mapstructure:"polymarket"`
	Monitor    MonitorConfig    `mapstructure:"monitor"`
	Telegram   TelegramConfig   `mapstructure:"telegram"`
	Storage    StorageConfig    `mapstructure:"storage"`
	Logging    LoggingConfig    `mapstructure:"logging"`
}

// PolymarketConfig holds Polymarket API configuration.
type PolymarketConfig struct {
	GammaAPIURL         string        `mapstructure:"gamma_api_url"`
	ClobAPIURL          string        `mapstructure:"clob_api_url"`
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

// MonitorConfig holds monitoring behavior configuration.
type MonitorConfig struct {
	WindowSize         int     `mapstructure:"window_size"`
	Alpha              float64 `mapstructure:"alpha"`
	Ceiling            float64 `mapstructure:"ceiling"`
	Threshold          float64 `mapstructure:"threshold"`
	Volume24hrMin      float64 `mapstructure:"volume_24hr_min"`
	Volume1wkMin       float64 `mapstructure:"volume_1wk_min"`
	Volume1moMin       float64 `mapstructure:"volume_1mo_min"`
	TopK               int     `mapstructure:"top_k"`
	CooldownMultiplier int     `mapstructure:"cooldown_multiplier"`
	CheckpointInterval int     `mapstructure:"checkpoint_interval"`
}

// TelegramConfig holds Telegram notification configuration.
type TelegramConfig struct {
	BotToken       string        `mapstructure:"bot_token"`
	ChatID         string        `mapstructure:"chat_id"`
	Enabled        bool          `mapstructure:"enabled"`
	MaxRetries     int           `mapstructure:"max_retries"`
	RetryDelayBase time.Duration `mapstructure:"retry_delay_base"`
}

// StorageConfig holds storage configuration.
type StorageConfig struct {
	MaxEvents int    `mapstructure:"max_events"`
	DBPath    string `mapstructure:"db_path"`
}

// LoggingConfig holds logging configuration.
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// Load reads configuration from a YAML file with environment variable overrides.
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	setDefaults(v)
	v.SetEnvPrefix("POLY_ORACLE")
	v.AutomaticEnv()

	// Bind environment variables to nested config keys
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
	_ = v.BindEnv("monitor.top_k", "POLY_ORACLE_MONITOR_TOP_K")
	_ = v.BindEnv("telegram.bot_token", "POLY_ORACLE_TELEGRAM_BOT_TOKEN")
	_ = v.BindEnv("telegram.chat_id", "POLY_ORACLE_TELEGRAM_CHAT_ID")
	_ = v.BindEnv("telegram.enabled", "POLY_ORACLE_TELEGRAM_ENABLED")
	_ = v.BindEnv("telegram.max_retries", "POLY_ORACLE_TELEGRAM_MAX_RETRIES")
	_ = v.BindEnv("telegram.retry_delay_base", "POLY_ORACLE_TELEGRAM_RETRY_DELAY_BASE")
	_ = v.BindEnv("storage.max_events", "POLY_ORACLE_STORAGE_MAX_EVENTS")
	_ = v.BindEnv("storage.db_path", "POLY_ORACLE_STORAGE_DB_PATH")
	_ = v.BindEnv("logging.level", "POLY_ORACLE_LOGGING_LEVEL")
	_ = v.BindEnv("logging.format", "POLY_ORACLE_LOGGING_FORMAT")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	// Polymarket
	v.SetDefault("polymarket.gamma_api_url", "https://gamma-api.polymarket.com")
	v.SetDefault("polymarket.clob_api_url", "https://clob.polymarket.com")
	v.SetDefault("polymarket.poll_interval", "1h")
	v.SetDefault("polymarket.categories", []string{"geopolitics", "tech", "finance", "crypto", "world"})
	v.SetDefault("polymarket.volume_24hr_min", 25000.0)
	v.SetDefault("polymarket.volume_1wk_min", 100000.0)
	v.SetDefault("polymarket.volume_1mo_min", 250000.0)
	v.SetDefault("polymarket.volume_filter_or", true)
	v.SetDefault("polymarket.limit", 500)
	v.SetDefault("polymarket.timeout", "30s")
	v.SetDefault("polymarket.max_retries", 3)
	v.SetDefault("polymarket.retry_delay_base", "1s")
	v.SetDefault("polymarket.max_idle_conns", 100)
	v.SetDefault("polymarket.max_idle_conns_per_host", 10)
	v.SetDefault("polymarket.idle_conn_timeout", "90s")

	// Monitor
	v.SetDefault("monitor.top_k", 5)

	// Telegram
	v.SetDefault("telegram.enabled", false)
	v.SetDefault("telegram.max_retries", 3)
	v.SetDefault("telegram.retry_delay_base", "1s")

	// Storage
	v.SetDefault("storage.max_events", 10000)
	v.SetDefault("storage.db_path", "")

	// Logging
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
}

// Validate checks that all configuration values are valid.
func (c *Config) Validate() error {
	if c.Polymarket.GammaAPIURL == "" {
		return fmt.Errorf("polymarket.gamma_api_url is required")
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

	if c.Monitor.WindowSize < 1 {
		return fmt.Errorf("monitor.window_size must be at least 1")
	}
	if c.Monitor.Alpha < 0.0 || c.Monitor.Alpha > 1.0 {
		return fmt.Errorf("monitor.alpha must be between 0.0 and 1.0")
	}
	if c.Monitor.Ceiling <= 0 {
		return fmt.Errorf("monitor.ceiling must be positive")
	}
	if c.Monitor.Threshold <= 0 {
		return fmt.Errorf("monitor.threshold must be positive")
	}
	if c.Monitor.Volume24hrMin < 0 {
		return fmt.Errorf("monitor.volume_24hr_min must not be negative")
	}
	if c.Monitor.Volume1wkMin < 0 {
		return fmt.Errorf("monitor.volume_1wk_min must not be negative")
	}
	if c.Monitor.Volume1moMin < 0 {
		return fmt.Errorf("monitor.volume_1mo_min must not be negative")
	}
	if c.Monitor.TopK < 0 {
		return fmt.Errorf("monitor.top_k must not be negative")
	}
	if c.Monitor.CooldownMultiplier < 1 {
		return fmt.Errorf("monitor.cooldown_multiplier must be at least 1")
	}
	if c.Monitor.CheckpointInterval < 1 {
		return fmt.Errorf("monitor.checkpoint_interval must be at least 1")
	}

	if c.Telegram.Enabled {
		if c.Telegram.BotToken == "" {
			return fmt.Errorf("telegram.bot_token is required when telegram is enabled")
		}
		if c.Telegram.ChatID == "" {
			return fmt.Errorf("telegram.chat_id is required when telegram is enabled")
		}
	}

	if c.Storage.MaxEvents < 1 {
		return fmt.Errorf("storage.max_events must be at least 1")
	}

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
