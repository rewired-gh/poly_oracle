# Internal Interface Contract: Monitor Package

**Branch**: `001-smart-ranking` | **Date**: 2026-02-17

This is an internal Go interface contract (no HTTP endpoints). Documents the changed and new
public API surface of `internal/monitor`.

---

## Removed / Changed APIs

### `DetectChanges` — signature change (breaking)

```go
// BEFORE
func (m *Monitor) DetectChanges(
    events    []models.Event,
    threshold float64,        // REMOVED
    window    time.Duration,
) ([]models.Change, []DetectionError, error)

// AFTER
func (m *Monitor) DetectChanges(
    events []models.Event,
    window time.Duration,
) ([]models.Change, []DetectionError, error)
```

Behavior: same as before except minimum probability change is now a hardcoded floor of 0.001
(not user-configurable). All changes ≥ 0.1% are returned; scoring is responsible for quality
filtering.

### `RankChanges` — removed

```go
// REMOVED
func (m *Monitor) RankChanges(changes []models.Change, k int) []models.Change
```

Replaced by `ScoreAndRank`.

---

## New APIs

### `ScoreAndRank`

```go
func (m *Monitor) ScoreAndRank(
    changes   []models.Change,
    events    map[string]*models.Event,  // keyed by composite EventID
    minScore  float64,                   // from cfg.Monitor.MinCompositeScore()
    k         int,                       // from cfg.Monitor.TopK
) []models.Change
```

Returns at most `k` changes with `SignalScore` set, filtered by `minScore`, sorted by
`SignalScore` descending. Ties broken by `EventID` lexicographic descending.

Returns empty slice (never nil) when no changes meet the quality bar.

### Pure scoring functions (exported for testing)

```go
func KLDivergence(pOld, pNew float64) float64
func LogVolumeWeight(volume24h, vRef float64) float64
func HistoricalSNR(allSnapshots []models.Snapshot, netChange float64) float64
func TrajectoryConsistency(windowSnapshots []models.Snapshot) float64
func CompositeScore(kl, vw, snr, tc float64) float64
```

These are exported so they can be tested directly in `monitor_test.go` without needing a
full Monitor instance. Each function is a pure computation with no side effects.

---

## Callers — Required Updates

### `cmd/poly-oracle/main.go`

```go
// BEFORE
changes, detectionErrors, err := mon.DetectChanges(convertEvents(allEvents), cfg.Monitor.Threshold, cfg.Monitor.Window)
// ...
topChanges := mon.RankChanges(changes, cfg.Monitor.TopK)

// AFTER
changes, detectionErrors, err := mon.DetectChanges(convertEvents(allEvents), cfg.Monitor.Window)
// ...
eventsMap := buildEventsMap(allEvents)  // map[string]*models.Event from []*models.Event
topChanges := mon.ScoreAndRank(changes, eventsMap, cfg.Monitor.MinCompositeScore(), cfg.Monitor.TopK)
```

The Telegram send condition also changes:
```go
// BEFORE
if len(changes) > 0 && cfg.Telegram.Enabled && telegramClient != nil {

// AFTER
if len(topChanges) > 0 && cfg.Telegram.Enabled && telegramClient != nil {
```
Previously: notify if any raw changes detected, then send top-K.
Now: notify only if scoring produces at least 1 result above quality bar.

---

## Config Interface

### `config.MonitorConfig`

```go
// New method
func (m MonitorConfig) MinCompositeScore() float64
```

Returns `m.Sensitivity * m.Sensitivity * 0.05`. Used in main.go to get the score floor.

### Environment variables (new)

```
POLY_ORACLE_MONITOR_SENSITIVITY=0.5
```

Maps to `monitor.sensitivity` in config.yaml.
