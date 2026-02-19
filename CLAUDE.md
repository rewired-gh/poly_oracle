# polyoracle Development Guidelines

## Project Structure

```text
cmd/polyoracle/        # Application entry point (main.go)
internal/               # Private application code
  ├── config/          # Configuration loading and validation
  ├── logger/          # Structured logging (debug/info/warn/error/fatal)
  ├── models/          # Domain entities (Event, Snapshot, Change)
  ├── polymarket/      # Polymarket API client (Gamma + CLOB APIs)
  ├── monitor/         # Change detection and composite scoring
  ├── storage/         # In-memory storage with file persistence
  └── telegram/        # Telegram bot client
pkg/api/               # Public API types (placeholder)
tests/                 # Integration tests and testdata
docs/                  # Configuration tuning results, valid categories reference
specs/                 # Feature specification plans
configs/               # YAML configuration files
deployments/           # Dockerfile & systemd service
bin/                   # Built binaries
```

## Commands

```bash
# Development
make install           # Install dependencies (go mod download)
make build             # Build binary to bin/polyoracle (native)
make build-linux       # Cross-compile for Linux x86_64 → bin/polyoracle-linux-amd64
make test              # Run all tests
make test-coverage     # Run tests with coverage
make run               # Build and run with configs/config.yaml
make fmt               # Format code with gofmt
make lint              # Run golangci-lint
make dev               # Development mode with auto-reload (requires entr)

# Deployment
make generate-config   # Generate example config from running binary
make docker-build      # Build Docker image
make docker-run        # Run Docker container

# Clean
make clean             # Remove binaries and data directory
```

## Dependencies

- **Viper** (github.com/spf13/viper) - YAML configuration management
- **Telegram Bot API** (github.com/go-telegram-bot-api/telegram-bot-api/v5) - Telegram notifications
- **UUID** (github.com/google/uuid) - Unique identifier generation
- **modernc.org/sqlite** - Pure-Go SQLite driver (no CGO required)

## Architecture

Single binary service with polling architecture:

1. **Config Loader** → Reads YAML from `configs/config.yaml`
2. **Monitor Service** → Orchestrates polling cycles
3. **Polymarket Client** → Fetches events from Gamma API + CLOB API
4. **Storage** → SQLite-backed persistence via `modernc.org/sqlite` (no CGO); WAL mode
5. **Change Detection** → Four-factor composite scoring: KL divergence × log-volume weight × historical SNR × trajectory consistency; results ranked via `ScoreAndRank`
6. **Telegram Client** → Sends notifications for top K changes

Data flow: Poll → Store → Detect Changes → Notify → Persist

## Configuration

**Default categories**: geopolitics, tech, finance, world
**Default thresholds**: $25K 24hr / $100K 1wk / $250K 1mo (OR logic)
**Default monitor threshold**: 4% probability change

Required setup before first run:

1. **Copy example config**: `cp configs/config.yaml.example configs/config.yaml`
2. **Get Telegram credentials**:
   - Bot token: Create bot with [@BotFather](https://t.me/botfather)
   - Chat ID: Get from [@userinfobot](https://t.me/userinfobot)
3. **Edit config**: Add bot_token and chat_id to configs/config.yaml

Config file must have:
- `telegram.bot_token` - Required when telegram.enabled = true
- `telegram.chat_id` - Required when telegram.enabled = true
- `polymarket.categories` - At least one category to monitor

Environment variable overrides supported: `POLY_ORACLE_*`

**Storage** (`storage.*` in config):
- `max_events` — max markets tracked; oldest evicted inline on `AddMarket` (default 10000)
- `max_snapshots_per_event` — max snapshots per market kept by `RotateSnapshots` (default 672)
- `db_path` — SQLite DB file path (default `$TMPDIR/polyoracle/data.db`); env `POLY_ORACLE_STORAGE_DB_PATH`

**Logging**: Configure verbosity via `logging.level` (debug/info/warn/error) and `logging.format` (json/text) in config.yaml. Use `debug` level for troubleshooting.

## Testing

Table-driven tests using standard Go testing package:

```bash
make test              # All tests
make test-coverage     # With coverage
go test ./internal/monitor -v   # Specific package
```

Tests located: `internal/**/*_test.go`

## Deployment Options

1. **Binary**: `make build && ./bin/polyoracle --config configs/config.yaml`
2. **Docker**: `make docker-build && make docker-run`
3. **systemd**: Copy `deployments/systemd/polyoracle.service` to `/etc/systemd/system/`

## Development Workflow

```bash
# 1. Install dependencies
make install

# 2. Setup configuration
cp configs/config.yaml.example configs/config.yaml
# Edit configs/config.yaml with Telegram credentials

# 3. Run tests
make test

# 4. Build
make build

# 5. Run locally
make run

# 6. Check logs
# Logs output to stderr (terminal/console)
```

## Key Files

- `cmd/polyoracle/main.go` - Entry point, orchestration
- `configs/config.yaml` - User configuration (gitignored, copy from example)
- `configs/config.yaml.example` - Template with sensible defaults
- `configs/config.test.yaml` - Test-time config overrides (modify for local testing)
- `internal/config/config.go` - Config loading & validation
- `internal/logger/logger.go` - Structured logger (init with `logger.Init(level, format)`)
- `internal/monitor/monitor.go` - Composite scoring and ranking algorithm

## Gotchas

- **Config file required**: Service fails without valid config.yaml
- **Telegram credentials**: Must be set before running
- **Storage path**: Default uses OS tmp dir (`$TMPDIR/polyoracle/data.db`)
- **Categories filter**: Only monitors events in configured categories
- **Volume filter**: Lower thresholds needed for niche categories (geopolitics/tech/finance)
- **Telegram MarkdownV2**: Notification messages use MarkdownV2 format with automatic escaping of special characters

## API Quirks

- **Category field often null**: Polymarket API `category` field is frequently null; actual category info is in `tags[]` array (filtering logic uses tag slugs)
- **Volume filter OR logic**: Events pass if meeting ANY volume threshold ($25K 24hr OR $100K 1wk OR $250K 1mo)
- **Multi-market event tracking**: Events with multiple markets are tracked separately. Each market creates a unique event entry with composite ID (`EventID:MarketID`). This allows detecting probability changes for each market independently.

## Multi-Market Event Handling

Polymarket events can have multiple markets (e.g., "Will Bitcoin hit $100K?" might have separate markets for different dates). The service tracks each market independently:

- **Composite ID**: `EventID:MarketID` format ensures unique tracking
- **Market-specific changes**: Probability changes are detected per market
- **Telegram notifications**: Show which specific market changed (with market question)
- **URL handling**: All markets share the same event URL

Example: An event "Will Bitcoin hit price targets?" with 3 markets:
- Market 1: "Will Bitcoin hit $100K by March?" (tracked as `event123:market1`)
- Market 2: "Will Bitcoin hit $150K by June?" (tracked as `event123:market2`)
- Market 3: "Will Bitcoin hit $200K by Dec?" (tracked as `event123:market3`)

Each market is monitored separately for probability changes.

## Code Style

Go 1.24+ (latest stable): Follow standard Go conventions

<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->
