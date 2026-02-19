# polyoracle Development Guidelines

> For project structure, quick start, configuration reference, and deployment see [README.md](README.md).

## Commands

```bash
make install           # Install dependencies (go mod download)
make build             # Build binary → bin/polyoracle
make build-linux       # Cross-compile for Linux x86_64 → bin/polyoracle-linux-amd64
make test              # Run all tests
make test-coverage     # Run tests with coverage
make run               # Build and run with configs/config.yaml
make fmt               # Format code with gofmt
make lint              # Run golangci-lint
make dev               # Development mode with auto-reload (requires entr)
make docker-build      # Build Docker image
make docker-run        # Run Docker container
make clean             # Remove binaries and data directory
```

## Architecture

Single binary service with polling architecture:

1. **Config Loader** → Reads YAML from `configs/config.yaml`
2. **Monitor Service** → Orchestrates polling cycles
3. **Polymarket Client** → Fetches events from Gamma API + CLOB API
4. **Storage** → SQLite-backed persistence via `modernc.org/sqlite` (no CGO); WAL mode
5. **Change Detection** → Four-factor composite scoring: KL divergence × log-volume weight × historical SNR × trajectory consistency; results ranked via `ScoreAndRank`
6. **Telegram Client** → Sends notifications for top K changes

Data flow: Poll → Store → Detect Changes → Notify → Persist

## Key Files

- `cmd/polyoracle/main.go` — Entry point, orchestration
- `configs/config.yaml.example` — Annotated config template (SSoT for all config fields and defaults)
- `configs/config.test.yaml` — Local test overrides (debug logging; same values as example otherwise)
- `internal/config/config.go` — Config loading & validation; Go-side defaults
- `internal/logger/logger.go` — Structured logger (init with `logger.Init(level, format)`)
- `internal/monitor/monitor.go` — Composite scoring and ranking algorithm (`ScoreAndRank`)

## Testing

Table-driven tests using the standard Go testing package:

```bash
make test                           # All tests
make test-coverage                  # With coverage
go test ./internal/monitor -v       # Specific package
```

Tests located: `internal/**/*_test.go`

## Gotchas

- **Config file required**: Service fails without valid `configs/config.yaml`; copy from `configs/config.yaml.example`
- **Telegram credentials**: `telegram.bot_token` and `telegram.chat_id` are required when `telegram.enabled = true`
- **Storage path**: Default uses OS tmp dir (`$TMPDIR/polyoracle/data.db`); override with env `POLY_ORACLE_STORAGE_DB_PATH`
- **Categories filter**: Only monitors events in configured categories; see [`docs/valid-categories.md`](docs/valid-categories.md) for valid slugs
- **Volume filter OR logic**: Events pass if they meet ANY one threshold ($24hr OR $1wk OR $1mo)
- **Telegram MarkdownV2**: Notification messages use MarkdownV2 format with automatic escaping of special characters

## API Quirks

- **Category field often null**: Polymarket API `category` field is frequently null; actual category info is in `tags[]` array — filtering uses tag slugs
- **Multi-market event tracking**: Events with multiple markets are tracked separately. Each market gets a composite ID (`EventID:MarketID`), enabling per-market change detection.

## Multi-Market Event Handling

Polymarket events can have multiple markets (e.g., "Will Bitcoin hit $X?" with separate markets for different dates). The service tracks each market independently:

- **Composite ID**: `EventID:MarketID` format ensures unique tracking
- **Market-specific changes**: Probability changes are detected per market
- **Telegram notifications**: Show which specific market changed (with market question)
- **URL handling**: All markets share the same event URL

Example — an event "Will Bitcoin hit price targets?" with 3 markets:
- `event123:market1` → "Will Bitcoin hit $100K by March?"
- `event123:market2` → "Will Bitcoin hit $150K by June?"
- `event123:market3` → "Will Bitcoin hit $200K by Dec?"

## Code Style

Go 1.24+ (latest stable): follow standard Go conventions.

<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->
