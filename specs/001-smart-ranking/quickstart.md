# Quickstart: Verifying Smart Signal Ranking

**Branch**: `001-smart-ranking` | **Date**: 2026-02-17

## Prerequisites

```bash
make install   # go mod download
make build     # builds to bin/polyoracle
make test      # all tests must pass
```

## Step 1 — Run unit tests for scoring functions

```bash
go test ./internal/monitor/... -v -run TestScore
go test ./internal/monitor/... -v -run TestKLDivergence
go test ./internal/monitor/... -v -run TestHistoricalSNR
go test ./internal/monitor/... -v -run TestTrajectoryConsistency
```

Expected: all 8+ table-driven cases pass, output shows specific scoring values.

## Step 2 — Verify sensitivity config is loaded

```bash
# Edit configs/config.yaml and set:
#   monitor:
#     sensitivity: 0.5

./bin/polyoracle --config configs/config.yaml &
# Look for startup log line mentioning sensitivity
# e.g.: "Starting monitoring service (sensitivity: 0.50, window: 1h, top_k: 5)"
kill %1
```

## Step 3 — Verify sensitivity changes output count

```bash
# Set sensitivity: 0.1 in config (permissive)
./bin/polyoracle --config configs/config.yaml &
# After one cycle, observe number of "Detected N significant changes" in logs
# and "Sending top M to Telegram" (should be higher M)
kill %1

# Set sensitivity: 0.9 in config (strict)
./bin/polyoracle --config configs/config.yaml &
# After one cycle, observe M is smaller (possibly 0)
kill %1
```

## Step 4 — Verify zero-emission behavior

```bash
# Set sensitivity: 1.0 in config.yaml
# Run one cycle
./bin/polyoracle --config configs/config.yaml

# Confirm in logs:
#   "Monitoring cycle completed" without "Sent Telegram notification"
# This confirms the zero-emission path is working.
```

## Step 5 — Verify no regression on existing behavior

```bash
make test       # all existing tests still pass
make lint       # no new lint warnings
```

## Checklist

- [X] `go test ./...` passes with no failures
- [X] `make lint` passes (no new warnings from this feature; pre-existing errcheck warnings in logger remain)
- [X] Log output mentions `sensitivity` at startup
- [X] Sensitivity 0.1 produces more notifications than sensitivity 0.9 on same data
- [X] No notification sent when sensitivity is 1.0 and no extreme signals present
- [X] `configs/config.yaml.example` documents `sensitivity` with examples
- [X] `configs/config.yaml.example` removes `threshold` (replaced by `sensitivity`)
