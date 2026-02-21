package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadAndValidate(t *testing.T) {
	// Create temp config file
	content := `
polymarket:
  poll_interval: 5m
  categories:
    - politics
    - sports

monitor:
  window_size: 3
  alpha: 0.1
  ceiling: 10.0
  threshold: 3.0
  volume_24hr_min: 25000
  volume_1wk_min: 100000
  volume_1mo_min: 500000
  top_k: 10
  cooldown_multiplier: 5
  checkpoint_interval: 12

telegram:
  bot_token: "test_token"
  chat_id: "test_chat_id"
  enabled: true

storage:
  max_events: 1000
  db_path: "./data/test.db"

logging:
  level: "info"
  format: "json"
`
	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(tmpfile.Name()) }()

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	// Test Load
	cfg, err := Load(tmpfile.Name())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify values
	if cfg.Polymarket.PollInterval != 5*time.Minute {
		t.Errorf("Unexpected poll interval: %v", cfg.Polymarket.PollInterval)
	}

	if cfg.Monitor.WindowSize != 3 {
		t.Errorf("Unexpected window size: %d", cfg.Monitor.WindowSize)
	}

	if cfg.Monitor.Alpha != 0.1 {
		t.Errorf("Unexpected alpha: %f", cfg.Monitor.Alpha)
	}

	if len(cfg.Polymarket.Categories) != 2 {
		t.Errorf("Expected 2 categories, got %d", len(cfg.Polymarket.Categories))
	}

	// Test Validate
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
}

func TestValidateErrors(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "missing telegram token when enabled",
			config: &Config{
				Polymarket: PolymarketConfig{
					GammaAPIURL:  "https://example.com",
					PollInterval: 5 * time.Minute,
					Categories:   []string{"politics"},
				},
				Monitor: MonitorConfig{
					WindowSize:         3,
					Alpha:              0.1,
					Ceiling:            10.0,
					Threshold:          3.0,
					Volume24hrMin:      25000,
					Volume1wkMin:       100000,
					Volume1moMin:       500000,
					TopK:               10,
					CooldownMultiplier: 5,
					CheckpointInterval: 12,
				},
				Telegram: TelegramConfig{
					Enabled: true,
					// Missing BotToken
				},
				Storage: StorageConfig{
					MaxEvents: 1000,
					DBPath:    "",
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid alpha",
			config: &Config{
				Polymarket: PolymarketConfig{
					GammaAPIURL:  "https://example.com",
					PollInterval: 5 * time.Minute,
					Categories:   []string{"politics"},
				},
				Monitor: MonitorConfig{
					WindowSize:         3,
					Alpha:              1.5, // Invalid: > 1.0
					Ceiling:            10.0,
					Threshold:          3.0,
					Volume24hrMin:      25000,
					Volume1wkMin:       100000,
					Volume1moMin:       500000,
					TopK:               10,
					CooldownMultiplier: 5,
					CheckpointInterval: 12,
				},
				Storage: StorageConfig{
					MaxEvents: 1000,
					DBPath:    "",
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid window size",
			config: &Config{
				Polymarket: PolymarketConfig{
					GammaAPIURL:  "https://example.com",
					PollInterval: 5 * time.Minute,
					Categories:   []string{"politics"},
				},
				Monitor: MonitorConfig{
					WindowSize:         0, // Invalid: must be at least 1
					Alpha:              0.1,
					Ceiling:            10.0,
					Threshold:          3.0,
					Volume24hrMin:      25000,
					Volume1wkMin:       100000,
					Volume1moMin:       500000,
					TopK:               10,
					CooldownMultiplier: 5,
					CheckpointInterval: 12,
				},
				Storage: StorageConfig{
					MaxEvents: 1000,
					DBPath:    "",
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
