# Implementation Plan: Smart Signal Ranking

**Branch**: `001-smart-ranking` | **Date**: 2026-02-17 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/001-smart-ranking/spec.md`

## Summary

Replace the current magnitude-only ranking (`score = |Δp|`) with a four-factor composite
signal score:

```
score = KL(p_new || p_old) × log_volume_weight × historical_snr × trajectory_consistency
```

Add a single `monitor.sensitivity` config parameter (replaces `threshold`) that maps to a
minimum composite score floor via `sensitivity² × 0.05`. Emit 0–K notifications per cycle
based solely on quality, never forcing K.

The entire change is contained within `internal/monitor`, `internal/config`, `internal/models`,
`cmd/poly-oracle/main.go`, and `configs/config.yaml.example`. No new dependencies. No new
storage structures. No new API calls.

---

## Technical Context

**Language/Version**: Go 1.24+ (latest stable)
**Primary Dependencies**: None new. `math` stdlib for `math.Log`, `math.Sqrt`, `math.Abs`.
**Storage**: Existing in-memory + JSON persistence. `storage.GetSnapshots(eventID)` (already
  exists) provides full snapshot history for SNR computation.
**Testing**: Standard `go test` with table-driven tests in `internal/monitor/monitor_test.go`.
**Target Platform**: Linux server (systemd / Docker). Single binary.
**Performance Goals**: O(m × n) per cycle, m ≤ 1000 events, n ≤ 24 snapshots. Expected wall
  time: < 10ms for the scoring step.
**Constraints**: No new external calls per cycle. All scoring from in-memory data.
**Scale/Scope**: Single binary, ~100–500 active markets at typical runtime.

---

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

**I. Simplicity Is Non-Negotiable** ✅
- Four scoring functions, each doing one thing. No abstraction layers.
- `ScoreAndRank` is the only new method on Monitor. Replaces `RankChanges`.
- No new types except `SignalScore float64` field on the existing `Change` struct.
- Complexity Justification: Four factors vs. one is justified — each addresses a distinct
  noise source documented in spec (no credibility, no noise floor, no path quality).

**II. Idiomatic Go** ✅
- All math uses `math` stdlib (`math.Log`, `math.Sqrt`, `math.Abs`). No new dependencies.
- Pure functions exported for direct testing.
- No generics. No goroutines added.

**III. Explicit Error Handling** ✅
- `GetSnapshots` returns `([]Snapshot, error)`. Error logged and SNR falls back to 1.0.
- No silent error swallowing.

**IV. Minimal Dependencies** ✅
- Zero new dependencies. All math is stdlib.

**V. Pragmatic Testing** ✅
- Table-driven tests verify behavior (ranking order) not implementation (internal state).
- One test case per factor demonstrating material effect on ranking.
- Tests are pure (no I/O, no mocks).

---

## Project Structure

### Documentation (this feature)

```text
specs/001-smart-ranking/
├── plan.md              # This file
├── research.md          # Phase 0: algorithm decisions and calibration
├── data-model.md        # Phase 1: model changes
├── quickstart.md        # Phase 1: verification steps
├── contracts/
│   └── monitor-api.md   # Phase 1: internal Go interface changes
└── tasks.md             # Phase 2 output (/speckit.tasks — not yet created)
```

### Source Code Changes

```text
internal/
├── config/
│   └── config.go              # Add Sensitivity field, MinCompositeScore(), update Validate()
├── models/
│   └── change.go              # Add SignalScore float64 field
└── monitor/
    ├── monitor.go             # Add 5 pure scoring functions + ScoreAndRank, remove threshold from DetectChanges
    └── monitor_test.go        # 8+ table-driven scoring tests + update existing tests

cmd/
└── poly-oracle/
    └── main.go                # Update runMonitoringCycle: remove threshold arg, use ScoreAndRank

configs/
└── config.yaml.example        # Add sensitivity, deprecate threshold
```

**Structure Decision**: Single project (existing Go module). No new packages, no new
directories. All changes are within existing package boundaries.

---

## Phase 0: Research Findings Summary

Full details in [research.md](research.md). Key decisions:

| Question | Decision |
|---|---|
| KL direction | `KL(p_new \|\| p_old)`: surprise at posterior given prior |
| Volume scaling | Log-normalized: `log(1 + V/V_ref) / log(2)`, floor 0.1 |
| SNR σ computation | Sample std dev (Bessel) of all stored Δp; neutral 1.0 fallback |
| SNR clamping | `[0.5, 5.0]` — prevents single outlier dominating |
| Trajectory formula | `\|Σ Δp_i\| / Σ\|Δp_i\|` in window; 1.0 fallback for single pair |
| Sensitivity mapping | `sensitivity² × 0.05` → min_score |
| Multi-market volume | Event-level Volume24hr shared across all markets — correct, no change |
| Threshold removal | Hardcoded floor of 0.001 (0.1%) replaces configurable threshold |

---

## Phase 1: Design

### 1. `internal/config/config.go`

**Add to `MonitorConfig`**:
```go
Sensitivity float64 `mapstructure:"sensitivity"`
```

**Add method**:
```go
func (m MonitorConfig) MinCompositeScore() float64 {
    return m.Sensitivity * m.Sensitivity * 0.05
}
```

**Update `setDefaults`**:
```go
v.SetDefault("monitor.sensitivity", 0.5)
v.SetDefault("monitor.top_k", 5)
// Keep threshold default for backward compat parse, but remove from active path
```

**Update `Validate`**:
- Add: `Sensitivity` must be in [0.0, 1.0]
- Change: `TopK` allows 0 (K=0 means "never notify", valid)
- Remove: threshold validation (field still parsed but not enforced)

**Add env var binding**:
```go
_ = v.BindEnv("monitor.sensitivity", "POLY_ORACLE_MONITOR_SENSITIVITY")
```

---

### 2. `internal/models/change.go`

**Add field to `Change`**:
```go
SignalScore float64 `json:"signal_score,omitempty"`
```

No other changes. `Validate()` unchanged.

---

### 3. `internal/monitor/monitor.go`

**Remove `threshold float64` from `DetectChanges` signature**. Replace with hardcoded minimum:
```go
const minProbabilityChange = 0.001  // 0.1%: suppresses floating-point noise only
```

**Add 5 exported pure functions** (exported so tests can call directly):

```go
// KLDivergence computes KL(pNew || pOld) for binary distributions.
// Probabilities are clamped to [1e-7, 1-1e-7] to avoid ln(0).
func KLDivergence(pOld, pNew float64) float64

// LogVolumeWeight returns log(1 + volume/vRef) / log(2).
// Returns at least 0.1 (floor for zero-volume markets).
func LogVolumeWeight(volume24h, vRef float64) float64

// HistoricalSNR computes |netChange| / σ_hist clamped to [0.5, 5.0].
// σ_hist is the sample std dev of consecutive Δp across all stored snapshots.
// Returns 1.0 when fewer than 2 changes exist or σ < 1e-4.
func HistoricalSNR(allSnapshots []models.Snapshot, netChange float64) float64

// TrajectoryConsistency returns |ΣΔp| / Σ|Δp| across consecutive window pairs.
// Returns 1.0 when window has ≤ 1 pair (no inconsistency to measure).
func TrajectoryConsistency(windowSnapshots []models.Snapshot) float64

// CompositeScore multiplies four factors into a single signal quality scalar.
func CompositeScore(kl, vw, snr, tc float64) float64
```

**Add `ScoreAndRank` method**:
```go
// ScoreAndRank scores each change using the four-factor composite signal score,
// filters below minScore, and returns at most k changes sorted by score descending.
// Ties are broken by EventID lexicographic descending for determinism.
// Returns empty slice (never nil) when nothing clears the quality bar.
func (m *Monitor) ScoreAndRank(
    changes  []models.Change,
    events   map[string]*models.Event,
    minScore float64,
    k        int,
) []models.Change
```

Implementation outline:
1. For each change: look up event in `events` map; if missing, skip with log warning.
2. Call `m.storage.GetSnapshots(change.EventID)` for full history (SNR).
3. Call `m.storage.GetSnapshotsInWindow(change.EventID, change.TimeWindow)` (trajectory).
4. Compute `CompositeScore(KLDivergence(...), LogVolumeWeight(...), HistoricalSNR(...), TrajectoryConsistency(...))`.
5. Set `change.SignalScore = score`.
6. Append to candidate list if `score >= minScore`.
7. Sort candidates by `SignalScore` desc, break ties by `EventID` desc.
8. Return `candidates[:min(k, len(candidates))]`.

**Remove `RankChanges` method** — no longer needed.

**Update package doc comment** to describe the new composite scoring algorithm.

---

### 4. `internal/monitor/monitor_test.go`

Keep all existing tests (they test `DetectChanges` which is unchanged except signature).
Update call sites to remove `threshold` argument.

Add table-driven test `TestScoring` with 8+ cases (one per FR-008 requirement):

| Case | What it tests |
|---|---|
| VolumeWins | High volume + lower KL > low volume + higher KL |
| SNRWins | Quiet market 3% move > volatile market 3% move |
| KLRegimeDiff | Same magnitude, different probability regime → different scores |
| MonotonicBeatsNoisy | Same net change, clean path > oscillating path |
| DegenProbabilities | p=0.0, p=1.0 inputs do not panic or NaN |
| ZeroVolumeFloor | Volume=0 gets floor weight, not zero |
| SNRFallback | Fewer than 2 snapshots → SNR=1.0 (not 0, not panic) |
| Determinism | Identical inputs → identical ranked order across multiple calls |

Also add `TestKLDivergence`, `TestLogVolumeWeight`, `TestHistoricalSNR`,
`TestTrajectoryConsistency` as separate table-driven tests for the pure functions.

---

### 5. `cmd/poly-oracle/main.go`

In `runMonitoringCycle`:

```go
// BEFORE
changes, detectionErrors, err := mon.DetectChanges(convertEvents(allEvents), cfg.Monitor.Threshold, cfg.Monitor.Window)
// ...
topChanges := mon.RankChanges(changes, cfg.Monitor.TopK)
if len(changes) > 0 && cfg.Telegram.Enabled ...

// AFTER
changes, detectionErrors, err := mon.DetectChanges(convertEvents(allEvents), cfg.Monitor.Window)
// ...
eventsMap := buildEventsMap(allEvents)  // new helper: []*Event → map[string]*Event
topChanges := mon.ScoreAndRank(changes, eventsMap, cfg.Monitor.MinCompositeScore(), cfg.Monitor.TopK)
if len(topChanges) > 0 && cfg.Telegram.Enabled ...
```

Add `buildEventsMap` helper at bottom of file:
```go
func buildEventsMap(events []*models.Event) map[string]*models.Event {
    m := make(map[string]*models.Event, len(events))
    for _, e := range events {
        m[e.ID] = e
    }
    return m
}
```

Update startup log message:
```go
logger.Info("Starting monitoring service (sensitivity: %.2f, window: %v, top_k: %d)",
    cfg.Monitor.Sensitivity, cfg.Monitor.Window, cfg.Monitor.TopK)
```

---

### 6. `configs/config.yaml.example`

Replace the `monitor:` section:

```yaml
monitor:
  # sensitivity: controls the minimum signal quality bar (0.0 = permissive, 1.0 = strict).
  # Maps to a minimum composite score via sensitivity^2 × 0.05.
  #   0.3 → very permissive: most changes pass, expect ~5 events per cycle in active markets
  #   0.5 → default: medium and above signals pass, expect ~2-3 events per cycle
  #   0.7 → strict: only clear, well-supported moves pass, expect 0-1 events per cycle
  #   1.0 → very strict: only extreme signals (large market, clean move, unusual for that market)
  sensitivity: 0.5

  # threshold: DEPRECATED. Replaced by sensitivity. Kept for backward compatibility only.
  # Setting this field has no effect when sensitivity is configured.
  # threshold: 0.04

  window: 1h     # time window for detecting changes (oldest vs newest snapshot)
  top_k: 5       # maximum notifications per cycle (0 = never notify)
  enabled: true
```

---

## Complexity Tracking

No constitution violations. All complexity is justified by the feature requirements.

| Factor | Justification | Simpler Alternative Rejected Because |
|---|---|---|
| 4 scoring factors vs 1 | Each factor addresses a distinct, documented noise source | Using fewer factors was the previous approach — it produced the noise the user is trying to eliminate |
| ScoreAndRank needs events map | Volume is event-level, required for V_weight | Alternative (embed volume in Change at detection time) would require more model changes for less gain |
