package config

import (
	"os"
	"testing"
)

func TestLoadAndValidate(t *testing.T) {
	// Create temp config file
	content := `
polymarket:
  api_base_url: "https://gamma-api.polymarket.com"
  poll_interval: 5m
  categories:
    - politics
    - sports
  timeout: 30s

monitor:
  threshold: 0.10
  window: 1h
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
  persistence_interval: 5m
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
	defer os.Remove(tmpfile.Name())

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
	if cfg.Polymarket.GammaAPIURL != "https://gamma-api.polymarket.com" {
		t.Errorf("Unexpected API URL: %s", cfg.Polymarket.GammaAPIURL)
	}

	if cfg.Monitor.Threshold != 0.10 {
		t.Errorf("Unexpected threshold: %f", cfg.Monitor.Threshold)
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
					PollInterval: 5 * 60 * 1000 * 1000 * 1000,
					Categories:   []string{"politics"},
				},
				Monitor: MonitorConfig{
					Threshold: 0.10,
					Window:    60 * 60 * 1000 * 1000 * 1000,
					TopK:      10,
				},
				Telegram: TelegramConfig{
					Enabled: true,
					// Missing BotToken
				},
				Storage: StorageConfig{
					MaxEvents:            1000,
					MaxSnapshotsPerEvent: 100,
					MaxFileSizeMB:        100,
					PersistenceInterval:  5 * 60 * 1000 * 1000 * 1000,
					FilePath:             "./data/test.json",
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid threshold",
			config: &Config{
				Polymarket: PolymarketConfig{
					GammaAPIURL:  "https://example.com",
					PollInterval: 5 * 60 * 1000 * 1000 * 1000,
					Categories:   []string{"politics"},
				},
				Monitor: MonitorConfig{
					Threshold: 1.5, // Invalid
					Window:    60 * 60 * 1000 * 1000 * 1000,
					TopK:      10,
				},
				Storage: StorageConfig{
					MaxEvents:            1000,
					MaxSnapshotsPerEvent: 100,
					MaxFileSizeMB:        100,
					PersistenceInterval:  5 * 60 * 1000 * 1000 * 1000,
					FilePath:             "./data/test.json",
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
