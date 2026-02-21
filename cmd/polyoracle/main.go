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

	"github.com/rewired-gh/polyoracle/internal/config"
	"github.com/rewired-gh/polyoracle/internal/logger"
	"github.com/rewired-gh/polyoracle/internal/monitor"
	"github.com/rewired-gh/polyoracle/internal/polymarket"
	"github.com/rewired-gh/polyoracle/internal/storage"
	"github.com/rewired-gh/polyoracle/internal/telegram"
)

var configPath = flag.String("config", "configs/config.yaml", "Path to configuration file")

func main() {
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	logger.Init(cfg.Logging.Level, cfg.Logging.Format)
	logger.Info("Configuration loaded from %s", *configPath)

	store, err := storage.New(
		cfg.Storage.MaxEvents,
		cfg.Storage.DBPath,
	)
	if err != nil {
		logger.Fatal("Failed to initialize storage: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			logger.Error("Failed to close storage: %v", err)
		}
	}()

	polyClient := polymarket.NewClient(
		cfg.Polymarket.GammaAPIURL,
		cfg.Polymarket.ClobAPIURL,
		cfg.Polymarket.Timeout,
		polymarket.ClientConfig{
			MaxRetries:          cfg.Polymarket.MaxRetries,
			RetryDelayBase:      cfg.Polymarket.RetryDelayBase,
			MaxIdleConns:        cfg.Polymarket.MaxIdleConns,
			MaxIdleConnsPerHost: cfg.Polymarket.MaxIdleConnsPerHost,
			IdleConnTimeout:     cfg.Polymarket.IdleConnTimeout,
		},
	)

	monitorConfig := monitor.Config{
		WindowSize:         cfg.Monitor.WindowSize,
		Alpha:              cfg.Monitor.Alpha,
		Ceiling:            cfg.Monitor.Ceiling,
		Threshold:          cfg.Monitor.Threshold,
		Volume24hrMin:      cfg.Monitor.Volume24hrMin,
		Volume1wkMin:       cfg.Monitor.Volume1wkMin,
		Volume1moMin:       cfg.Monitor.Volume1moMin,
		TopK:               cfg.Monitor.TopK,
		CooldownMultiplier: cfg.Monitor.CooldownMultiplier,
		CheckpointInterval: cfg.Monitor.CheckpointInterval,
	}
	mon := monitor.New(store, monitorConfig)

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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Info("Shutdown signal received, cleaning up...")
		mon.Shutdown()
		cancel()
	}()

	if cfg.Telegram.Enabled && telegramClient != nil {
		telegramClient.ListenForCommands(ctx)
	}

	logger.Info("Starting monitoring service (interval: %v, window_size: %d, threshold: %.1f, top_k: %d)",
		cfg.Polymarket.PollInterval,
		cfg.Monitor.WindowSize,
		cfg.Monitor.Threshold,
		cfg.Monitor.TopK,
	)

	ticker := time.NewTicker(cfg.Polymarket.PollInterval)
	defer ticker.Stop()

	consecutiveFailures := 0

	handleCycleResult := func(err error) {
		if err != nil {
			consecutiveFailures++
			logger.Error("Monitoring cycle failed: %v", err)
			if consecutiveFailures == 1 && cfg.Telegram.Enabled && telegramClient != nil {
				if sendErr := telegramClient.SendError(err); sendErr != nil {
					logger.Warn("Failed to send error notification to Telegram: %v", sendErr)
				}
			}
		} else {
			if consecutiveFailures > 0 && cfg.Telegram.Enabled && telegramClient != nil {
				if sendErr := telegramClient.SendRecovery(consecutiveFailures); sendErr != nil {
					logger.Warn("Failed to send recovery notification to Telegram: %v", sendErr)
				}
			}
			consecutiveFailures = 0
		}
	}

	logger.Debug("Running initial monitoring cycle")
	handleCycleResult(runMonitoringCycle(ctx, polyClient, mon, store, telegramClient, cfg))

	for {
		select {
		case <-ctx.Done():
			logger.Info("Service stopped")
			return

		case <-ticker.C:
			logger.Debug("Starting scheduled monitoring cycle")
			handleCycleResult(runMonitoringCycle(ctx, polyClient, mon, store, telegramClient, cfg))
			if err := store.RotateMarkets(); err != nil {
				logger.Warn("Failed to rotate markets: %v", err)
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
	logger.Info("Starting monitoring cycle")

	logger.Debug("Fetching markets from Polymarket API (categories: %v, limit: %d)", cfg.Polymarket.Categories, cfg.Polymarket.Limit)
	// Volume filtering is handled by monitor, not at API level
	markets, err := polyClient.FetchEvents(ctx, cfg.Polymarket.Categories, 0, 0, 0, false, cfg.Polymarket.Limit)
	if err != nil {
		return fmt.Errorf("failed to fetch markets: %w", err)
	}
	logger.Info("Fetched %d markets from %d categories", len(markets), len(cfg.Polymarket.Categories))

	logger.Debug("Processing fetched markets")
	newMarkets := 0
	updatedMarkets := 0
	for i := range markets {
		market := &markets[i]

		existingMarket, err := store.GetMarket(market.ID)
		if err != nil {
			if err := store.AddMarket(market); err != nil {
				logger.Warn("Failed to add market %s: %v", market.ID, err)
				continue
			}
			newMarkets++
		} else {
			market.CreatedAt = existingMarket.CreatedAt
			if err := store.UpdateMarket(market); err != nil {
				logger.Warn("Failed to update market %s: %v", market.ID, err)
				continue
			}
			updatedMarkets++
		}
	}
	logger.Debug("Market processing complete: %d new, %d updated", newMarkets, updatedMarkets)

	alerts := mon.ProcessPoll(markets)
	logger.Info("Detected %d alerts above threshold", len(alerts))

	groups := mon.PostProcessAlerts(alerts, cfg.Polymarket.PollInterval)

	if len(groups) > 0 {
		totalMarkets := 0
		for _, g := range groups {
			totalMarkets += len(g.Markets)
		}
		logger.Info("Post-processed alerts: %d groups (%d markets)", len(groups), totalMarkets)

		if cfg.Telegram.Enabled && telegramClient != nil {
			logger.Debug("Sending top %d event groups to Telegram", len(groups))
			if err := telegramClient.Send(groups); err != nil {
				logger.Error("Failed to send Telegram notification: %v", err)
			} else {
				logger.Info("Sent Telegram notification with top %d event groups", len(groups))
				mon.RecordNotified(groups)
			}
		} else {
			logger.Debug("Alerts detected but Telegram notifications disabled or client not initialized")
		}
	} else {
		logger.Info("No alerts above quality bar this cycle")
	}

	duration := time.Since(startTime)
	logger.Info("Monitoring cycle completed in %v", duration)

	return nil
}
