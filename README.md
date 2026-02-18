# Polyoracle

A lightweight Go service that monitors Polymarket prediction markets for significant probability shifts and delivers Telegram alerts for the top-K highest-signal events.

## How It Works

Each polling cycle:

1. Fetches events from the Polymarket Gamma + CLOB APIs, filtered by category and volume thresholds
2. Stores probability snapshots in memory (persisted to disk)
3. Detects changes over a rolling detection window using a four-factor composite signal score:

   ```
   score = KL(p_new ∥ p_old) × log_volume_weight × historical_SNR × trajectory_consistency
   ```

4. Applies pre-score hard filters (minimum absolute change, minimum base probability) to suppress tail-probability noise
5. Groups per-market changes by parent event, ranks by best score, deduplicates against recent notifications
6. Sends a Telegram message for the top-K event groups

Multi-market events (e.g., "Bitcoin hits $X by date Y") are tracked per market with composite IDs (`EventID:MarketID`).

## Quick Start

### Prerequisites

- Go 1.24+
- Telegram bot token — create one with [@BotFather](https://t.me/botfather)
- Telegram chat ID — get yours from [@userinfobot](https://t.me/userinfobot)

### Setup

```bash
git clone <repository-url>
cd polyoracle

make install

cp configs/config.yaml.example configs/config.yaml
# Edit configs/config.yaml and add your bot_token and chat_id

make run
```

## Configuration

`configs/config.yaml`:

```yaml
polymarket:
  poll_interval: 15m       # How often to poll Polymarket
  limit: 5000              # Max events to fetch per cycle
  categories:
    - geopolitics
    - tech
    - finance
    - world
  # Volume pre-filters (OR logic): events pass if they meet any threshold
  volume_24hr_min: 25000   # $25K minimum 24hr volume
  volume_1wk_min: 100000   # $100K minimum weekly volume
  volume_1mo_min: 250000   # $250K minimum monthly volume

monitor:
  sensitivity: 0.5         # Composite score threshold (0=permissive, 1=strict)
                           # 0.3 → noisy, 0.5 → ~2-3 alerts/cycle (recommended), 0.7 → strict
  top_k: 10                # Max event groups per notification
  detection_intervals: 4   # Window = (detection_intervals + 1) × poll_interval
  min_abs_change: 0.03     # Minimum absolute probability shift (3pp)
  min_base_prob: 0.05      # Ignore markets below 5% (tail-probability zone)

telegram:
  bot_token: "YOUR_BOT_TOKEN"
  chat_id: "YOUR_CHAT_ID"
  enabled: true

storage:
  max_events: 10000
  max_snapshots_per_event: 672   # 7 days of 15-min snapshots
  max_file_size_mb: 2048

logging:
  level: info              # debug, info, warn, error
```

### Configuration Reference

| Section | Field | Default | Description |
|---------|-------|---------|-------------|
| polymarket | poll_interval | 15m | Polling frequency |
| polymarket | categories | geopolitics, tech, finance, world | Categories to monitor |
| polymarket | volume_24hr_min | 25000 | Min $24hr volume (OR filter) |
| polymarket | volume_1wk_min | 100000 | Min weekly volume (OR filter) |
| polymarket | volume_1mo_min | 250000 | Min monthly volume (OR filter) |
| monitor | sensitivity | 0.5 | Quality threshold (sensitivity² × 0.05 = minScore) |
| monitor | top_k | 10 | Max event groups per alert |
| monitor | detection_intervals | 4 | Polling periods per detection window |
| monitor | min_abs_change | 0.03 | Min absolute probability change (fraction) |
| monitor | min_base_prob | 0.05 | Min base probability to avoid tail-zone KL inflation |
| storage | max_events | 10000 | Max events tracked in memory |
| storage | max_snapshots_per_event | 672 | Snapshot history per market |
| telegram | bot_token | — | Required when telegram.enabled = true |
| telegram | chat_id | — | Required when telegram.enabled = true |

## Deployment

### Binary

```bash
make build
./bin/polyoracle --config configs/config.yaml
```

### Docker

```bash
make docker-build
docker run -d \
  --name polyoracle \
  -v $(PWD)/configs:/app/configs \
  -v $(PWD)/data:/app/data \
  polyoracle:latest
```

### systemd

```bash
sudo cp deployments/systemd/polyoracle.service /etc/systemd/system/
sudo systemctl enable --now polyoracle
```

## Development

```bash
make test           # Run all tests
make test-coverage  # With coverage report
make lint           # golangci-lint
make fmt            # gofmt
make dev            # Auto-reload on file change (requires entr)
```

### Project Structure

```
cmd/polyoracle/        Entry point (main.go)
internal/
  config/               YAML config loading and validation
  logger/               Structured logger (debug/info/warn/error)
  models/               Domain types: Event, Market, Snapshot, Change
  polymarket/           Gamma + CLOB API client
  monitor/              Composite scoring, ranking, deduplication
  storage/              In-memory store with file persistence
  telegram/             Telegram bot client (MarkdownV2 formatting)
configs/                config.yaml.example, config.test.yaml
deployments/            Dockerfile, systemd service
specs/                  Feature spec documents
docs/                   Tuning notes, valid category reference
```

## Example Notification

```
🚨 Notable Odds Movements

📅 Detected: 2026-02-18 10:30:00

1. Will candidate X win the election?
   📈 15.0% (60.0% → 75.0%) ⏱ 75m

2. Will Bitcoin hit $100K by March?
   🎯 Will Bitcoin hit $100K by March?
   📉 8.2% (72.3% → 64.1%) ⏱ 75m
```

## Gotchas

- **Config file required**: Service exits without a valid `configs/config.yaml`
- **Polymarket category field**: The API `category` field is frequently null; filtering uses `tags[]` slugs
- **Tail-probability suppression**: Markets below `min_base_prob` (default 5%) are excluded because KL divergence is structurally unreliable at the tails
- **Cooldown deduplication**: Markets recently notified in the same direction are suppressed unless they cross into the high-conviction zone (>90% or <10%)
- **Storage path**: Defaults to `/tmp/polyoracle/data.json`

## Dependencies

- [Viper](https://github.com/spf13/viper) — configuration management
- [go-telegram-bot-api](https://github.com/go-telegram-bot-api/telegram-bot-api) — Telegram integration
- [google/uuid](https://github.com/google/uuid) — change record IDs

## License

MIT
