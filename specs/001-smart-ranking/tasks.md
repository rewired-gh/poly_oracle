---

description: "Task list for Smart Signal Ranking implementation"
---

# Tasks: Smart Signal Ranking

**Input**: Design documents from `specs/001-smart-ranking/`
**Prerequisites**: plan.md ✅ spec.md ✅ research.md ✅ data-model.md ✅ contracts/ ✅ quickstart.md ✅

**Tests**: Included — spec FR-008 explicitly requires table-driven unit tests per scoring factor.

**Organization**: Tasks grouped by user story to enable independent delivery of each increment.

## Format: `[ID] [P?] [Story?] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to
- Exact file paths included in every task description

## Path Conventions

All paths relative to repository root. Single Go module at root.

---

## Phase 1: Setup

**Purpose**: No new packages or dependencies. Existing project structure is used as-is.
No setup tasks required for this feature.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Model and config changes that both user stories depend on.

**⚠️ CRITICAL**: T001 must complete before T008 (ScoreAndRank). T002 must complete before T018 (main.go wiring). Both can run in parallel with each other and with T003–T006.

- [X] T001 [P] Add `SignalScore float64 \`json:"signal_score,omitempty"\`` field to `Change` struct in `internal/models/change.go` — no other changes to that file; do not touch `Validate()`

- [X] T002 [P] Update `internal/config/config.go`:
  (a) Remove `Threshold float64` from `MonitorConfig` struct entirely;
  (b) Add `Sensitivity float64 \`mapstructure:"sensitivity"\`` to `MonitorConfig` struct;
  (c) Add method `func (m MonitorConfig) MinCompositeScore() float64 { return m.Sensitivity * m.Sensitivity * 0.05 }`;
  (d) In `setDefaults`: remove `v.SetDefault("monitor.threshold", ...)`, add `v.SetDefault("monitor.sensitivity", 0.5)`;
  (e) In `Validate`: remove threshold validation entirely, add `if c.Monitor.Sensitivity < 0.0 || c.Monitor.Sensitivity > 1.0 { return error }`, change TopK check from `< 1` to `< 0`;
  (f) Remove the `monitor.threshold` env binding; add `_ = v.BindEnv("monitor.sensitivity", "POLY_ORACLE_MONITOR_SENSITIVITY")`

**Checkpoint**: `go build ./...` passes with both changes.

---

## Phase 3: User Story 1 — Composite-Ranked Digest (Priority: P1) 🎯 MVP

**Goal**: Replace `RankChanges` (magnitude-only sort) with `ScoreAndRank` (four-factor composite score). The digest now reflects genuine signal quality, not raw percentage change.

**Independent Test**: `go test ./internal/monitor/... -v` passes all 12+ table-driven cases including VolumeWins, SNRWins, MonotonicBeatsNoisy, and Determinism.

### Pure Scoring Functions (implement in parallel)

- [X] T003 [P] [US1] Add exported function `KLDivergence(pOld, pNew float64) float64` to `internal/monitor/monitor.go`:
  Computes `pNew*math.Log(pNew/pOld) + (1-pNew)*math.Log((1-pNew)/(1-pOld))`.
  Clamp both probabilities to `[1e-7, 1-1e-7]` before computation.
  Add package-level constant `const probEpsilon = 1e-7`.

- [X] T004 [P] [US1] Add exported function `LogVolumeWeight(volume24h, vRef float64) float64` to `internal/monitor/monitor.go`:
  Computes `math.Log(1+volume24h/vRef) / math.Log(2)`.
  When `vRef <= 0`, treat as 1.0.
  Return `math.Max(0.1, result)` — floor at 0.1 for zero-volume markets.

- [X] T005 [P] [US1] Add exported function `HistoricalSNR(allSnapshots []models.Snapshot, netChange float64) float64` to `internal/monitor/monitor.go`:
  Compute consecutive Δp values: `Δp_i = snapshots[i+1].YesProbability - snapshots[i].YesProbability` for all i.
  Return 1.0 if fewer than 2 Δp values exist.
  Compute sample std dev (Bessel: divide by n-1). Return 1.0 if σ < 1e-4.
  Return `math.Min(5.0, math.Max(0.5, math.Abs(netChange)/σ))`.

- [X] T006 [P] [US1] Add exported function `TrajectoryConsistency(windowSnapshots []models.Snapshot) float64` to `internal/monitor/monitor.go`:
  Compute signed Δp for each consecutive pair in `windowSnapshots`.
  Return 1.0 if fewer than 2 pairs (i.e., ≤ 2 snapshots).
  Return `math.Abs(sumSigned) / sumAbs`. If `sumAbs < 1e-10`, return 1.0.

- [X] T007 [P] [US1] Add exported function `CompositeScore(kl, vw, snr, tc float64) float64` to `internal/monitor/monitor.go`:
  Returns `kl * vw * snr * tc`. One line.

### ScoreAndRank (depends on T001, T003–T007)

- [X] T008 [US1] Add method `func (m *Monitor) ScoreAndRank(changes []models.Change, events map[string]*models.Event, minScore float64, k int) []models.Change` to `internal/monitor/monitor.go`:
  For each change:
    1. Look up event in `events` map by `change.EventID`; if missing, log warning and skip.
    2. Fetch all stored snapshots: `allSnaps, err := m.storage.GetSnapshots(change.EventID)`; on error, log and use SNR=1.0 fallback.
    3. Fetch window snapshots: `winSnaps, err := m.storage.GetSnapshotsInWindow(change.EventID, change.TimeWindow)`; on error, log and use consistency=1.0 fallback.
    4. `score := CompositeScore(KLDivergence(...), LogVolumeWeight(event.Volume24hr, vRef), HistoricalSNR(allSnaps, ...), TrajectoryConsistency(winSnaps))`
       where `vRef` is hardcoded to 25000.0 (the default volume_24hr_min — note: ideally this would come from config, but for now hardcode the default; plan notes this is the V_ref from research.md).
    5. Set `change.SignalScore = score`.
    6. Append to candidates if `score >= minScore`.
  Sort candidates by `SignalScore` descending; break ties by `EventID` descending (lexicographic).
  Return `candidates[:min(k, len(candidates))]`. Never return nil — return `[]models.Change{}` for empty.

- [X] T009 [US1] Update `DetectChanges` signature in `internal/monitor/monitor.go`:
  Remove `threshold float64` parameter.
  Add package-level constant `const minProbabilityChange = 0.001`.
  Replace `if change >= threshold {` with `if change >= minProbabilityChange {`.
  Update the function's doc comment to explain the new hardcoded floor.

- [X] T010 [US1] Remove `RankChanges` method entirely from `internal/monitor/monitor.go`.
  Update the package-level doc comment to describe the composite scoring algorithm.

### Tests (can be written in parallel with T003–T007)

- [X] T011 [P] [US1] Add `TestKLDivergence` table-driven test in `internal/monitor/monitor_test.go`:
  Cases: normal 5% move at p=0.5 (expect ≈0.005), 10% move at p=0.5 (expect ≈0.020), boundary p=0.0 and p=1.0 (expect no NaN/panic), symmetric check KL(0.6||0.5) > 0, result always ≥ 0.

- [X] T012 [P] [US1] Add `TestLogVolumeWeight` table-driven test in `internal/monitor/monitor_test.go`:
  Cases: volume=vRef → 1.0, volume=0 → 0.1 (floor), volume=4×vRef → ≈2.32, vRef=0 → handled gracefully.

- [X] T013 [P] [US1] Add `TestHistoricalSNR` table-driven test in `internal/monitor/monitor_test.go`:
  Cases: 0 snapshots → 1.0, 1 snapshot → 1.0, stable snapshots (σ very small) → 1.0, large move on quiet market → clamped to 5.0, small move on volatile market → clamped to 0.5.

- [X] T014 [P] [US1] Add `TestTrajectoryConsistency` table-driven test in `internal/monitor/monitor_test.go`:
  Cases: 1 snapshot pair → 1.0, monotonic rise → 1.0, perfect oscillation (net=0) → 0.0, mostly directional with one reversal → between 0.5 and 1.0.

- [X] T015 [US1] Add `TestScoring` comprehensive table-driven test in `internal/monitor/monitor_test.go` with ≥ 8 cases:
  - **VolumeWins**: market A ($1M vol, 5% move) scores > market B ($30K vol, 9% move)
  - **SNRWins**: quiet market (σ=0.005) 3% move scores > volatile market (σ=0.05) 3% move
  - **KLRegimeDiff**: same magnitude (5%) at p=0.5 vs p=0.95 → different scores (KL differs)
  - **MonotonicBeatsNoisy**: [0.5,0.52,0.55,0.58] scores > [0.5,0.68,0.42,0.58] same volume/snr
  - **DegenProbabilities**: p=0.0 and p=1.0 inputs do not produce NaN or panic
  - **ZeroVolumeFloor**: volume=0 market gets non-zero score (floor 0.1 applied)
  - **SNRFallback**: market with single snapshot → SNR=1.0, score is still valid float
  - **Determinism**: two identical sets of inputs produce byte-identical ranked output

- [X] T016 [US1] Update existing tests in `internal/monitor/monitor_test.go` to remove the `threshold` argument from all `DetectChanges` call sites. Verify existing behavior tests still compile and pass.

**Checkpoint**: `go test ./internal/monitor/... -v` — all existing + new tests pass.

---

## Phase 4: User Story 2 — Single Sensitivity Knob (Priority: P2)

**Goal**: Wire the new scoring into the main monitoring cycle. Replace the `threshold + RankChanges` call chain with `ScoreAndRank` driven by `sensitivity`. Update config documentation.

**Independent Test**: Run `make build` and then `./bin/polyoracle --config configs/config.yaml` for one cycle; observe log line `"Starting monitoring service (sensitivity: ..."` and verify notification count changes when sensitivity is adjusted.

**Depends on**: T002 (Sensitivity config), T008 (ScoreAndRank), T009 (DetectChanges signature change)

- [X] T017 [P] [US2] Update `runMonitoringCycle` in `cmd/polyoracle/main.go`:
  (a) Change `mon.DetectChanges(convertEvents(allEvents), cfg.Monitor.Threshold, cfg.Monitor.Window)` to `mon.DetectChanges(convertEvents(allEvents), cfg.Monitor.Window)`.
  (b) After detection, add: `eventsMap := buildEventsMap(allEvents)`.
  (c) Replace `mon.RankChanges(changes, cfg.Monitor.TopK)` with `mon.ScoreAndRank(changes, eventsMap, cfg.Monitor.MinCompositeScore(), cfg.Monitor.TopK)`.
  (d) Change the Telegram send condition from `if len(changes) > 0 && cfg.Telegram.Enabled` to `if len(topChanges) > 0 && cfg.Telegram.Enabled`.
  (e) Update the log line `"Detected N significant changes"` to also log how many passed the quality bar.

- [X] T018 [P] [US2] Add `buildEventsMap(events []*models.Event) map[string]*models.Event` helper at the bottom of `cmd/polyoracle/main.go`. Creates and returns a map keyed by `event.ID`.

- [X] T019 [US2] Update startup log in `cmd/polyoracle/main.go` from:
  `logger.Info("Starting monitoring service (poll: %v, threshold: %.2f, window: %v, top_k: %d)", ..., cfg.Monitor.Threshold, ...)`
  to:
  `logger.Info("Starting monitoring service (poll: %v, sensitivity: %.2f, window: %v, top_k: %d)", ..., cfg.Monitor.Sensitivity, ...)`

- [X] T020 [P] [US2] Replace the `monitor:` section in `configs/config.yaml.example` with:
  ```yaml
  monitor:
    # sensitivity controls the minimum signal quality floor (0.0 = permissive, 1.0 = strict).
    # Score formula: KL_divergence × log_volume_weight × historical_snr × trajectory_consistency
    # Min score threshold = sensitivity^2 × 0.05
    #   0.3 → permissive: most changes pass (~5 events/cycle in active markets)
    #   0.5 → default: medium and above signals (~2-3 events/cycle)
    #   0.7 → strict: only clear, well-supported moves (~0-1 events/cycle)
    #   1.0 → very strict: only extreme signals
    sensitivity: 0.5

    window: 1h    # time window for change detection (oldest vs. newest snapshot)
    top_k: 5      # maximum notifications per cycle (0 = never notify)
    enabled: true
  ```

**Checkpoint**: `make build` succeeds. `go vet ./...` passes.

---

## Phase 5: User Story 3 — Zero-Emission (Priority: P3)

**Goal**: Confirm the system sends no notification when nothing clears the quality floor. This falls out of T008 + T017 — no new code is needed. This phase is a verification and cleanup phase only.

**Independent Test**: Set `sensitivity: 1.0` in config, run one cycle against the real API (or fixture), confirm no Telegram message is sent even if raw changes exist.

- [X] T021 [US3] Verify zero-emission path in `cmd/polyoracle/main.go`: confirm that when `len(topChanges) == 0`, the log emits `"No changes above quality bar this cycle"` (or similar) and the Telegram client is NOT called. Add that log line if missing.

**Checkpoint**: `make test` passes. `go test ./...` passes.

---

## Phase N: Polish & Cross-Cutting Concerns

- [X] T022 [P] Update the package doc comment at the top of `internal/monitor/monitor.go` to describe the new four-factor composite scoring algorithm (KL, volume, SNR, trajectory). Remove references to magnitude-only ranking.

- [X] T023 [P] Run `make fmt` (gofmt) on all modified files and commit any formatting changes.

- [X] T024 [P] Run `make lint` (golangci-lint); fix any new warnings introduced by this feature.

- [X] T025 Run `make test` to confirm all tests pass end-to-end.

- [X] T026 Run the quickstart validation in `specs/001-smart-ranking/quickstart.md` — check each item in the checklist.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Foundational (Phase 2)**: No dependencies — start immediately. T001 and T002 run in parallel.
- **US1 (Phase 3)**: T003–T007 depend only on each other not blocking (pure functions, parallel). T008 depends on T001 + T003–T007. T009, T010 are independent. Tests T011–T014 can run parallel to T003–T007. T015 requires T008. T016 requires T009.
- **US2 (Phase 4)**: T017–T019 depend on T002 + T008 + T009. T020 is fully independent.
- **US3 (Phase 5)**: Depends on T017.
- **Polish (Phase N)**: Depends on all implementation tasks complete.

### User Story Dependencies

- **US1**: Can start immediately (pure math functions have no prerequisites).
- **US2**: Depends on T002 (Sensitivity config) and T008 (ScoreAndRank) and T009 (signature change).
- **US3**: Verification only. Depends on T017 (main.go wiring).

### Within Each Phase

- Pure scoring functions (T003–T007): fully parallel, different function bodies in same file.
- Tests (T011–T014): parallel with each other and with T003–T007.
- T008 (ScoreAndRank): after T001, T003–T007.
- T009, T010: independent of T003–T008, can run any time.
- T015 (TestScoring): after T008.
- T016 (update existing tests): after T009.

---

## Parallel Execution Examples

### User Story 1 — launch all pure functions together

```bash
# Implement all four pure scoring functions in parallel:
Task: "KLDivergence in internal/monitor/monitor.go"           # T003
Task: "LogVolumeWeight in internal/monitor/monitor.go"         # T004
Task: "HistoricalSNR in internal/monitor/monitor.go"           # T005
Task: "TrajectoryConsistency in internal/monitor/monitor.go"   # T006

# Implement all four unit tests in parallel (same time as functions):
Task: "TestKLDivergence in internal/monitor/monitor_test.go"   # T011
Task: "TestLogVolumeWeight in internal/monitor/monitor_test.go" # T012
Task: "TestHistoricalSNR in internal/monitor/monitor_test.go"  # T013
Task: "TestTrajectoryConsistency in internal/monitor/monitor_test.go" # T014
```

### User Story 2 — launch config and docs in parallel

```bash
Task: "Update runMonitoringCycle in cmd/polyoracle/main.go"    # T017
Task: "Add buildEventsMap in cmd/polyoracle/main.go"           # T018
Task: "Update monitor: section in configs/config.yaml.example"  # T020
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 2: T001, T002 (parallel, ~5 min each)
2. Complete T003–T007 (parallel, ~10 min each) + T011–T014 in parallel
3. Complete T008 (ScoreAndRank, ~20 min) + T009, T010 (parallel, ~5 min each)
4. Complete T015, T016 (tests, ~15 min)
5. **STOP and VALIDATE**: `go test ./internal/monitor/... -v` — all 12+ cases pass
6. The scoring algorithm is complete and verified without touching main.go

### Incremental Delivery

1. Phase 2 + Phase 3 → scoring works, tests pass (MVP)
2. Phase 4 → sensitivity wired in, service uses new ranking (deployable)
3. Phase 5 → zero-emission verified (no code change, just test/confirm)
4. Phase N → polish, lint clean

---

## Notes

- [P] tasks = different function bodies or different files — no file conflicts
- T003–T007 all go into `monitor.go` but they are independent function additions (no conflicts when done sequentially; note if done truly in parallel by multiple agents, use care with the same file)
- The `vRef` value in T008 is hardcoded to 25000.0 (the standard volume_24hr_min default). A follow-up could thread this through from config — noted in plan.md but out of scope for this task list.
- No test mocking required — all scoring functions are pure; `ScoreAndRank` is tested via the comprehensive `TestScoring` which builds fixture data directly.
- Commit after T016 (US1 complete) and after T020 (US2 complete) as logical checkpoints.
