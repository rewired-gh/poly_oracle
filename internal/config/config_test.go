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
  sensitivity: 0.5
  top_k: 10
  enabled: true

telegram:
  bot_token: "test_token"
  chat_id: "test_chat_id"
  enabled: true

storage:
  max_events: 1000
  max_snapshots_per_event: 100
  max_file_size_mb: 100
  file_path: "./data/test.json"
  data_dir: "./data"

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

	if cfg.Monitor.Sensitivity != 0.5 {
		t.Errorf("Unexpected sensitivity: %f", cfg.Monitor.Sensitivity)
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
					Sensitivity: 0.5,
					TopK:        10,
				},
				Telegram: TelegramConfig{
					Enabled: true,
					// Missing BotToken
				},
				Storage: StorageConfig{
					MaxEvents:            1000,
					MaxSnapshotsPerEvent: 100,
					DBPath: "",
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid sensitivity",
			config: &Config{
				Polymarket: PolymarketConfig{
					GammaAPIURL:  "https://example.com",
					PollInterval: 5 * time.Minute,
					Categories:   []string{"politics"},
				},
				Monitor: MonitorConfig{
					Sensitivity: 1.5, // Invalid: > 1.0
					TopK:        10,
				},
				Storage: StorageConfig{
					MaxEvents:            1000,
					MaxSnapshotsPerEvent: 100,
					DBPath: "",
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
