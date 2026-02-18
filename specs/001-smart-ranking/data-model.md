# Data Model: Smart Signal Ranking

**Branch**: `001-smart-ranking` | **Date**: 2026-02-17

No new entities are introduced. Three existing models are modified and one new computed type
is added (not persisted).

---

## Modified: `models.Change`

**File**: `internal/models/change.go`

Add one field:

```
Change {
  // ... existing fields unchanged ...
  SignalScore float64  // composite score from scoring algorithm; 0 = unscored
}
```

`SignalScore` is zero-valued by default (unscored). It is populated by `ScoreAndRank` and
reflects the composite signal quality at the time of scoring. It is NOT persisted to disk
(changes are transient — cleared each cycle). Including it in the struct enables logging and
future debugging.

**Validation**: The existing `Validate()` method is unchanged. `SignalScore` is not validated
(it is a computed output, not an input).

---

## Modified: `config.MonitorConfig`

**File**: `internal/config/config.go`

```
MonitorConfig {
  Threshold   float64       // DEPRECATED — retained for config parsing only, ignored in scoring
  Window      time.Duration
  TopK        int           // ceiling on notifications per cycle; 0 = never notify
  Enabled     bool
  Sensitivity float64       // NEW: [0.0, 1.0], default 0.5
}
```

**New method on MonitorConfig**:
```
func (m MonitorConfig) MinCompositeScore() float64 {
    return m.Sensitivity * m.Sensitivity * 0.05
}
```

**Validation changes**:
- `Sensitivity` must be in [0.0, 1.0]
- `TopK` now allows 0 (meaning "never send notifications" — valid operational state)
- `Threshold` validation is removed (field kept for backward parse compatibility)

---

## Modified: `monitor.Monitor` — New Internal Method Signatures

**File**: `internal/monitor/monitor.go`

The following pure functions (no state, fully testable) are added to the package:

### `klDivergence(pOld, pNew float64) float64`
Computes KL(p_new || p_old) for a binary distribution. Input probabilities clamped to
[1e-7, 1−1e-7] before use.

### `logVolumeWeight(volume24h, vRef float64) float64`
Computes `log(1 + volume24h/vRef) / log(2)`. Clamped to minimum 0.1 (zero-volume floor).

### `historicalSNR(allSnapshots []models.Snapshot, netChange float64) float64`
Computes sample std dev of consecutive Δp across all stored snapshots. Returns
`clamp(|netChange| / σ, 0.5, 5.0)`. Returns 1.0 when fewer than 2 changes exist or σ < 1e-4.

### `trajectoryConsistency(windowSnapshots []models.Snapshot) float64`
Computes `|Σ Δp_i| / Σ|Δp_i|` over consecutive pairs in window. Returns 1.0 when ≤ 1 pair.

### `compositeScore(kl, vw, snr, tc float64) float64`
Returns `kl × vw × snr × tc`.

### `(m *Monitor) ScoreAndRank(changes []models.Change, events map[string]*models.Event, minScore float64, k int) []models.Change`
Main ranking method. For each change:
1. Looks up event in provided map (uses composite EventID)
2. Fetches all stored snapshots via `m.storage.GetSnapshots(change.EventID)`
3. Fetches window snapshots via `m.storage.GetSnapshotsInWindow(change.EventID, change.TimeWindow)`
4. Computes composite score
5. Sets `change.SignalScore`
Filters changes below `minScore`, sorts descending by score, breaks ties by EventID
lexicographic descending, returns at most `k` results.

### `(m *Monitor) DetectChanges(events []models.Event, window time.Duration) ([]models.Change, []DetectionError, error)`
**Signature change**: `threshold float64` parameter removed. Hardcoded floor of 0.001
(0.1%) prevents floating-point noise from generating Change records.

---

## No Change: `storage.Storage`

`GetSnapshots(eventID)` already exists and returns full history. `GetAllEvents()` already
exists and returns all events. No new methods required.

---

## Composite Score — Expected Range Reference

| Scenario | KL | V_weight | SNR | Consistency | Score |
|---|---|---|---|---|---|
| Tiny market noise | 0.001 | 1.0 | 0.5 | 0.6 | 0.0003 |
| Weak signal | 0.003 | 1.2 | 0.8 | 0.7 | 0.002 |
| Medium signal | 0.008 | 2.0 | 2.0 | 0.9 | 0.029 |
| Strong signal | 0.020 | 3.5 | 4.0 | 1.0 | 0.28 |
| Extreme signal | 0.050 | 5.0 | 5.0 | 1.0 | 1.25 |

Default sensitivity (0.5) → min_score = 0.013 → medium signals and above pass.
