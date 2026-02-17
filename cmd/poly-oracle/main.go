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
	"github.com/poly-oracle/internal/logger"
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

	// Setup logging with level support
	logger.Init(cfg.Logging.Level, cfg.Logging.Format)
	logger.Info("Configuration loaded from %s", *configPath)

	// Initialize storage
	store := storage.New(
		cfg.Storage.MaxEvents,
		cfg.Storage.MaxSnapshotsPerEvent,
		cfg.Storage.FilePath,
		os.FileMode(cfg.Storage.FilePermissions),
		os.FileMode(cfg.Storage.DirPermissions),
	)

	// Load persisted data
	if err := store.Load(); err != nil {
		logger.Warn("Failed to load persisted data: %v", err)
	} else {
		logger.Debug("Successfully loaded persisted data from storage")
	}

	// Initialize Polymarket client
	polyClient := polymarket.NewClient(
		cfg.Polymarket.GammaAPIURL,
		cfg.Polymarket.CLOBAPIURL,
		cfg.Polymarket.Timeout,
		polymarket.ClientConfig{
			MaxRetries:          cfg.Polymarket.MaxRetries,
			RetryDelayBase:      cfg.Polymarket.RetryDelayBase,
			MaxIdleConns:        cfg.Polymarket.MaxIdleConns,
			MaxIdleConnsPerHost: cfg.Polymarket.MaxIdleConnsPerHost,
			IdleConnTimeout:     cfg.Polymarket.IdleConnTimeout,
		},
	)

	// Initialize monitor
	mon := monitor.New(store)

	// Initialize Telegram client
	var telegramClient *telegram.Client
	if cfg.Telegram.Enabled {
		telegramClient, err = telegram.NewClient(cfg.Telegram.BotToken, cfg.Telegram.ChatID, cfg.Telegram.MaxRetries, cfg.Telegram.RetryDelayBase)
		if err != nil {
			logger.Fatal("Failed to initialize Telegram client: %v", err)
		}
		logger.Info("Telegram client initialized successfully")
	} else {
		logger.Debug("Telegram notifications disabled")
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Info("Shutdown signal received, cleaning up...")
		cancel()
	}()

	// Start monitoring loop
	effectiveWindow := time.Duration(cfg.Monitor.DetectionIntervals+1) * cfg.Polymarket.PollInterval
	logger.Info("Starting monitoring service (interval: %v, detection_intervals: %d, effective_window: %v, sensitivity: %.2f, top_k: %d)",
		cfg.Polymarket.PollInterval,
		cfg.Monitor.DetectionIntervals,
		effectiveWindow,
		cfg.Monitor.Sensitivity,
		cfg.Monitor.TopK,
	)
	logger.Debug("Monitoring configuration: categories=%v, volume_24hr_min=%.0f, volume_filter_or=%v",
		cfg.Polymarket.Categories,
		cfg.Polymarket.Volume24hrMin,
		cfg.Polymarket.VolumeFilterOR,
	)

	ticker := time.NewTicker(cfg.Polymarket.PollInterval)
	defer ticker.Stop()

	// Run initial poll immediately
	logger.Debug("Running initial monitoring cycle")
	if err := runMonitoringCycle(ctx, polyClient, mon, store, telegramClient, cfg, time.Now()); err != nil {
		logger.Error("Monitoring cycle failed: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			// Save state before shutdown
			if err := store.Save(); err != nil {
				logger.Error("Failed to save state: %v", err)
			} else {
				logger.Info("State saved successfully before shutdown")
			}
			logger.Info("Service stopped")
			return

		case tickTime := <-ticker.C:
			logger.Debug("Starting scheduled monitoring cycle")
			if err := runMonitoringCycle(ctx, polyClient, mon, store, telegramClient, cfg, tickTime); err != nil {
				logger.Error("Monitoring cycle failed: %v", err)
			}

			// Persist data after each cycle
			var saveErr error
			for attempt := 0; attempt <= cfg.Storage.PersistenceRetries; attempt++ {
				if err := store.Save(); err == nil {
					logger.Debug("Data persisted successfully")
					saveErr = nil
					break
				} else {
					saveErr = err
					if attempt < cfg.Storage.PersistenceRetries {
						delay := cfg.Storage.PersistenceRetryDelay * time.Duration(attempt+1)
						logger.Warn("Persistence attempt %d/%d failed: %v (retrying in %v)",
							attempt+1, cfg.Storage.PersistenceRetries+1, err, delay)
						time.Sleep(delay)
					}
				}
			}
			if saveErr != nil {
				logger.Error("Failed to persist after %d attempts: %v",
					cfg.Storage.PersistenceRetries+1, saveErr)
			}

			// Rotate old data
			if err := store.RotateSnapshots(); err != nil {
				logger.Warn("Failed to rotate snapshots: %v", err)
			}
			if err := store.RotateEvents(); err != nil {
				logger.Warn("Failed to rotate events: %v", err)
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
	cycleTime time.Time, // tick time (or startup time for the initial cycle)
) error {
	startTime := time.Now()
	logger.Info("Starting monitoring cycle")

	// Fetch events from Polymarket
	logger.Debug("Fetching events from Polymarket API (categories: %v, limit: %d)", cfg.Polymarket.Categories, cfg.Polymarket.Limit)
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
	logger.Info("Fetched %d events from %d categories", len(events), len(cfg.Polymarket.Categories))

	// Update storage with new events and create snapshots
	logger.Debug("Processing fetched events and creating snapshots")
	newEvents := 0
	updatedEvents := 0
	for i := range events {
		event := &events[i]

		// Add or update event
		existingEvent, err := store.GetEvent(event.ID)
		if err != nil {
			// Event doesn't exist, create it
			if err := store.AddEvent(event); err != nil {
				logger.Warn("Failed to add event %s: %v", event.ID, err)
				continue
			}
			newEvents++
		} else {
			// Update existing event
			event.CreatedAt = existingEvent.CreatedAt
			if err := store.UpdateEvent(event); err != nil {
				logger.Warn("Failed to update event %s: %v", event.ID, err)
				continue
			}
			updatedEvents++
		}

		// Create snapshot for current probability.
		// Use cycleTime (tick time) as the timestamp, not time.Now() after processing.
		// This ensures snapshot ages are exact multiples of pollInterval, so the
		// detection window math is not skewed by per-cycle processing latency.
		snapshot := &models.Snapshot{
			ID:             generateID(),
			EventID:        event.ID,
			YesProbability: event.YesProbability,
			NoProbability:  event.NoProbability,
			Timestamp:      cycleTime,
			Source:         "polymarket-gamma-api",
		}

		if err := store.AddSnapshot(snapshot); err != nil {
			logger.Warn("Failed to add snapshot for event %s: %v", event.ID, err)
		}
	}
	logger.Debug("Event processing complete: %d new, %d updated", newEvents, updatedEvents)

	// Detect significant changes
	allEvents, err := store.GetAllEvents()
	if err != nil {
		return fmt.Errorf("failed to get events: %w", err)
	}
	// Window = (N+1) × pollInterval, not N × pollInterval.
	// With cycleTime-stamped snapshots, the oldest snapshot from N cycles ago is
	// exactly N×pollInterval old at tick time, but GetSnapshotsInWindow runs after
	// processing completes (tick + τ), making it N×pollInterval + τ old. The extra
	// interval absorbs τ so the boundary snapshot is never accidentally excluded.
	detectionWindow := time.Duration(cfg.Monitor.DetectionIntervals+1) * cfg.Polymarket.PollInterval
	logger.Debug("Detecting changes across %d total events (window: %v = (%d+1) × %v)",
		len(allEvents), detectionWindow, cfg.Monitor.DetectionIntervals, cfg.Polymarket.PollInterval)
	changes, detectionErrors, err := mon.DetectChanges(convertEvents(allEvents), detectionWindow)
	if err != nil {
		return fmt.Errorf("failed to detect changes: %w", err)
	}
	for _, detErr := range detectionErrors {
		logger.Warn("Failed to detect changes for event %s: %v", detErr.EventID, detErr.Err)
	}

	// Clear old changes and store new ones
	if err := store.ClearChanges(); err != nil {
		logger.Warn("Failed to clear old changes: %v", err)
	}
	for i := range changes {
		if err := store.AddChange(&changes[i]); err != nil {
			logger.Warn("Failed to add change: %v", err)
		}
	}

	logger.Info("Detected %d changes above floor", len(changes))

	// Score and rank changes using composite signal quality.
	// The four factors (KL, volume, SNR, trajectory) are already window-agnostic:
	// SNR normalizes netChange by historical per-interval volatility, so scaling
	// minScore by window duration is incorrect and creates a near-zero bar at 15m.
	minScore := cfg.Monitor.MinCompositeScore()
	eventsMap := buildEventsMap(allEvents)
	topChanges := mon.ScoreAndRank(changes, eventsMap, minScore, cfg.Monitor.TopK, cfg.Polymarket.Volume24hrMin)

	if len(topChanges) > 0 {
		logger.Info("Scored changes: %d detected, %d passed quality bar (min_score=%.4f)",
			len(changes), len(topChanges), minScore)

		if cfg.Telegram.Enabled && telegramClient != nil {
			logger.Debug("Sending top %d changes to Telegram", len(topChanges))
			if err := telegramClient.Send(topChanges); err != nil {
				logger.Error("Failed to send Telegram notification: %v", err)
			} else {
				logger.Info("Sent Telegram notification with top %d changes", len(topChanges))
			}
		} else {
			logger.Debug("Changes detected but Telegram notifications disabled or client not initialized")
		}
	} else {
		logger.Info("No changes above quality bar this cycle (min_score=%.4f)", minScore)
	}

	duration := time.Since(startTime)
	logger.Info("Monitoring cycle completed in %v", duration)

	return nil
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

func buildEventsMap(events []*models.Event) map[string]*models.Event {
	result := make(map[string]*models.Event, len(events))
	for _, event := range events {
		result[event.ID] = event
	}
	return result
}
