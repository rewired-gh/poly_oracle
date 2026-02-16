package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/poly-oracle/internal/config"
	"github.com/poly-oracle/internal/models"
	"github.com/poly-oracle/internal/monitor"
	"github.com/poly-oracle/internal/polymarket"
	"github.com/poly-oracle/internal/storage"
	"github.com/poly-oracle/internal/telegram"
)

var (
	configPath = flag.String("config", "configs/config.yaml", "Path to configuration file")
)

func main() {
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Setup logging
	setupLogging(cfg.GetLoggingConfig())

	// Initialize storage
	store := storage.New(
		cfg.Storage.MaxEvents,
		cfg.Storage.MaxSnapshotsPerEvent,
		cfg.Storage.FilePath,
	)

	// Load persisted data
	if err := store.Load(); err != nil {
		log.Printf("Warning: Failed to load persisted data: %v", err)
	}

	// Initialize Polymarket client
	polyClient := polymarket.NewClient(
		cfg.Polymarket.GammaAPIURL,
		cfg.Polymarket.CLOBAPIURL,
		cfg.Polymarket.Timeout,
	)

	// Initialize monitor
	mon := monitor.New(store)

	// Initialize Telegram client
	var telegramClient *telegram.Client
	if cfg.Telegram.Enabled {
		telegramClient, err = telegram.NewClient(cfg.Telegram.BotToken, cfg.Telegram.ChatID)
		if err != nil {
			log.Fatalf("Failed to initialize Telegram client: %v", err)
		}
		log.Println("Telegram client initialized successfully")
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutdown signal received, cleaning up...")
		cancel()
	}()

	// Start monitoring loop
	log.Printf("Starting monitoring service (poll: %v, threshold: %.2f, window: %v, top_k: %d)",
		cfg.Polymarket.PollInterval,
		cfg.Monitor.Threshold,
		cfg.Monitor.Window,
		cfg.Monitor.TopK,
	)

	ticker := time.NewTicker(cfg.Polymarket.PollInterval)
	defer ticker.Stop()

	persistenceTicker := time.NewTicker(cfg.Storage.PersistenceInterval)
	defer persistenceTicker.Stop()

	// Run initial poll immediately
	if err := runMonitoringCycle(ctx, polyClient, mon, store, telegramClient, cfg); err != nil {
		log.Printf("Monitoring cycle failed: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			// Save state before shutdown
			if err := store.Save(); err != nil {
				log.Printf("Failed to save state: %v", err)
			}
			log.Println("Service stopped")
			return

		case <-ticker.C:
			if err := runMonitoringCycle(ctx, polyClient, mon, store, telegramClient, cfg); err != nil {
				log.Printf("Monitoring cycle failed: %v", err)
			}

		case <-persistenceTicker.C:
			if err := store.Save(); err != nil {
				log.Printf("Failed to persist data: %v", err)
			}
			log.Println("Data persisted to disk")

			// Rotate old data
			if err := store.RotateSnapshots(); err != nil {
				log.Printf("Failed to rotate snapshots: %v", err)
			}
			if err := store.RotateEvents(); err != nil {
				log.Printf("Failed to rotate events: %v", err)
			}
		}
	}
}

func runMonitoringCycle(
	ctx context.Context,
	polyClient *polymarket.Client,
	mon *monitor.Monitor,
	store *storage.Storage,
	telegramClient *telegram.Client,
	cfg *config.Config,
) error {
	startTime := time.Now()
	log.Println("Starting monitoring cycle")

	// Fetch events from Polymarket
	events, err := polyClient.FetchEvents(
		ctx,
		cfg.Polymarket.Categories,
		cfg.Polymarket.Volume24hrMin,
		cfg.Polymarket.Volume1wkMin,
		cfg.Polymarket.Volume1moMin,
		cfg.Polymarket.VolumeFilterOR,
		cfg.Polymarket.Limit,
	)
	if err != nil {
		return fmt.Errorf("failed to fetch events: %w", err)
	}
	log.Printf("Fetched %d events from %d categories", len(events), len(cfg.Polymarket.Categories))

	// Update storage with new events and create snapshots
	for i := range events {
		event := &events[i]

		// Add or update event
		existingEvent, err := store.GetEvent(event.ID)
		if err != nil {
			// Event doesn't exist, create it
			if err := store.AddEvent(event); err != nil {
				log.Printf("Failed to add event %s: %v", event.ID, err)
				continue
			}
		} else {
			// Update existing event
			event.CreatedAt = existingEvent.CreatedAt
			if err := store.UpdateEvent(event); err != nil {
				log.Printf("Failed to update event %s: %v", event.ID, err)
				continue
			}
		}

		// Create snapshot for current probability
		snapshot := &models.Snapshot{
			ID:             generateID(),
			EventID:        event.ID,
			YesProbability: event.YesProbability,
			NoProbability:  event.NoProbability,
			Timestamp:      time.Now(),
			Source:         "polymarket-gamma-api",
		}

		if err := store.AddSnapshot(snapshot); err != nil {
			log.Printf("Failed to add snapshot for event %s: %v", event.ID, err)
		}
	}

	// Detect significant changes
	allEvents, err := store.GetAllEvents()
	if err != nil {
		return fmt.Errorf("failed to get events: %w", err)
	}

	changes, err := mon.DetectChanges(convertEvents(allEvents), cfg.Monitor.Threshold, cfg.Monitor.Window)
	if err != nil {
		return fmt.Errorf("failed to detect changes: %w", err)
	}

	// Clear old changes and store new ones
	store.ClearChanges()
	for i := range changes {
		if err := store.AddChange(&changes[i]); err != nil {
			log.Printf("Failed to add change: %v", err)
		}
	}

	log.Printf("Detected %d significant changes", len(changes))

	// Get top K changes and send notifications
	if len(changes) > 0 && cfg.Telegram.Enabled && telegramClient != nil {
		topChanges := mon.RankChanges(changes, cfg.Monitor.TopK)

		if err := telegramClient.Send(topChanges); err != nil {
			log.Printf("Failed to send Telegram notification: %v", err)
		} else {
			log.Printf("Sent Telegram notification with top %d changes", len(topChanges))
		}
	}

	duration := time.Since(startTime)
	log.Printf("Monitoring cycle completed in %v", duration)

	return nil
}

func setupLogging(cfg config.LoggingConfig) {
	// For now, use standard Go logging
	// In production, you'd integrate with a structured logging library
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func convertEvents(events []*models.Event) []models.Event {
	result := make([]models.Event, len(events))
	for i, event := range events {
		result[i] = *event
	}
	return result
}
