# Feature Specification: Smart Signal Ranking

**Feature Branch**: `001-smart-ranking`
**Created**: 2026-02-17
**Status**: Draft

## Background

The current system ranks probability changes by raw magnitude (`score = |Δp|`). This produces
noise: a low-volume, volatile market moving 8% outranks a large stable market moving 5%, even
though the latter move reflects far more genuine information.

Three distinct problems need fixing:

1. **No credibility weighting** — all markets treated equally regardless of trading volume.
2. **No per-market noise floor** — a normally-quiet market's 3% move is identical to a
   volatile market's 3% move. SNR is not market-relative.
3. **No path quality** — oldest vs. newest snapshot only. A clean directional move
   (0.50 → 0.52 → 0.55 → 0.58) is treated identically to a noisy oscillation that happens
   to net the same endpoints (0.50 → 0.72 → 0.38 → 0.58).

Additionally, the existing "magnitude factor" and "information content" would double-count
if naively combined: KL divergence already encodes magnitude. The scoring model must use
KL divergence as the *sole* measure of information content.

**Target formula:**
```
score = KL(p_old → p_new) × log_volume_weight × historical_snr × trajectory_consistency
```

All four factors are derived from data already in memory. No additional API calls are needed.
Maximum computational cost per cycle: O(m × n) where m ≤ 1000 events and n ≤ 24 snapshots.

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Composite-Ranked Digest (Priority: P1)

The user receives a periodic digest where entries are ranked by a composite signal score that
accounts for market credibility (volume), information content (KL divergence from prior to
posterior probability), per-market noise floor (historical volatility), and signal path quality
(trajectory consistency). The result is a digest where every entry represents a move that is
genuinely surprising relative to that market's own history, backed by real capital, and arrived
at cleanly rather than through noise.

**Why this priority**: This is the entire feature. The other stories enable configuration.

**Independent Test**: Can be validated entirely through unit tests against synthetic fixture
data — no running service required.

**Acceptance Scenarios**:

1. **Given** a high-volume market ($1M 24hr) moving 5% and a low-volume market ($30K) moving 9%,
   **When** the ranking runs,
   **Then** the high-volume market ranks higher — volume credibility outweighs raw magnitude
   when the difference is material.

2. **Given** a normally-quiet market (historical σ = 0.5%/cycle) moving 3%, and a normally
   volatile market (historical σ = 5%/cycle) also moving 3%,
   **When** the ranking runs,
   **Then** the quiet market ranks higher — its move is 6σ while the volatile market's is 0.6σ.

3. **Given** two markets with identical net probability change and identical volume,
   one arriving via a clean monotonic path across all snapshots and one via a noisy oscillating
   path,
   **When** the ranking runs,
   **Then** the monotonic-path market ranks higher.

4. **Given** a probability revision near certainty (e.g., 0.91 → 0.96) vs. the same magnitude
   change near maximum uncertainty (e.g., 0.47 → 0.52),
   **When** scored by KL divergence,
   **Then** the scores differ according to the information content of each revision — KL
   divergence, not raw magnitude, determines the weight.

5. **Given** identical input data across two separate invocations,
   **When** the ranking runs both times,
   **Then** the output order is byte-identical (determinism guarantee).

---

### User Story 2 — Single Sensitivity Knob (Priority: P2)

The user controls how permissive or strict the quality floor is by adjusting a single
`sensitivity` value. Today tuning requires understanding the interaction between a probability
threshold, three volume thresholds, and their OR/AND logic — which is too opaque for routine
tuning. The `sensitivity` parameter replaces this complexity for the common case.

**Why this priority**: The ranking improvement has standalone value, but sensitivity control
is what makes the system livable day-to-day.

**Independent Test**: Run the monitor against known fixture data with three sensitivity values
(low, default, high). Verify that event counts move monotonically.

**Acceptance Scenarios**:

1. **Given** default sensitivity,
   **When** the cycle runs,
   **Then** only events meeting the default quality bar appear in the digest.

2. **Given** sensitivity raised above default (stricter),
   **When** the cycle runs,
   **Then** the number of entries is ≤ the default-sensitivity count.

3. **Given** sensitivity lowered below default (more permissive),
   **When** the cycle runs,
   **Then** the number of entries is ≥ the default-sensitivity count.

4. **Given** sensitivity at maximum,
   **When** no events have extreme signal,
   **Then** zero entries are emitted.

---

### User Story 3 — Zero-Emission When Nothing Is Significant (Priority: P3)

The system sends no notification when no event clears the quality floor. An empty digest is
not sent. Zero notifications is a valid and expected output.

**Why this priority**: A forced "top K" notification when K items are below the quality bar is
noise. Silence is more valuable than a notification you've trained yourself to ignore.

**Independent Test**: Set sensitivity to maximum, supply low-signal fixture data, confirm no
Telegram message is sent.

**Acceptance Scenarios**:

1. **Given** no events with composite score above the minimum floor,
   **When** the cycle completes,
   **Then** no notification is sent.

2. **Given** 2 events above the floor and K = 5,
   **When** the cycle completes,
   **Then** exactly 2 events are notified — K is a ceiling, not a target.

---

### Edge Cases

- **Boundary probabilities** (0.0 or 1.0): KL divergence is undefined at exact boundaries.
  Probabilities MUST be clamped to [ε, 1−ε] before KL computation. ε = 1e-7 or similar.

- **Zero volume**: Volume weight MUST use a floor so a market with zero 24hr volume still
  receives a small (not zero) credibility weight. Dividing out or returning 0 would suppress
  potentially valid moves on newly listed markets.

- **Insufficient snapshot history for SNR**: When fewer than 2 historical probability changes
  exist for a market (e.g., first 2 cycles after startup), SNR MUST fall back to 1.0 (neutral).
  The system MUST NOT skip the event or return NaN.

- **Single snapshot in window**: Cannot compute a change at all. Event is skipped — existing
  behavior must be preserved.

- **All identical scores**: Ties MUST be broken by a stable secondary key (e.g., lexicographic
  event composite ID) to guarantee determinism.

- **Trajectory with only one snapshot pair**: Cannot compute consistency meaningfully. MUST
  default to 1.0 (neutral — no evidence of inconsistency).

- **K = 0 in config**: No notifications ever sent. Valid configuration, not an error.

---

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The ranking algorithm MUST incorporate exactly four factors: KL divergence
  (information content of the probability revision), log-normalized trading volume (market
  credibility), historical signal-to-noise ratio (change magnitude relative to the market's
  own historical volatility), and trajectory consistency (monotonicity of the probability path
  across all snapshots in the window). No other scoring factors are required.

- **FR-002**: The composite signal score MUST be deterministic — given identical snapshots,
  event data, and configuration, the ranked output MUST be identical across all invocations.

- **FR-003**: The system MUST emit between 0 and K entries per cycle, inclusive. Events whose
  composite score falls below the minimum quality floor MUST be excluded even if K slots remain.

- **FR-004**: Configuration MUST expose a single `monitor.sensitivity` parameter (float in
  [0.0, 1.0]) that controls the minimum composite score floor. 0.0 is most permissive (nearly
  everything passes); 1.0 is most strict (only extreme signals pass). Default is 0.5.
  The parameter's effect MUST be documented with concrete behavioral examples in config.yaml.

- **FR-005**: The hard probability `threshold` parameter is deprecated and SHOULD be removed
  from the primary code path. The composite score floor (driven by `sensitivity`) replaces it.
  For backward compatibility, `threshold` MAY remain parseable from config but MUST be ignored
  when `sensitivity` is set. The volume filter parameters (`volume_*_min`) are retained as
  pre-fetch filters and are not part of the ranking formula.

- **FR-006**: The historical SNR factor MUST be computed from the stored snapshot history for
  each event — specifically the standard deviation of consecutive probability changes
  (`Δp_i = p_{i+1} − p_i`) across the stored snapshots (up to the current `max_snapshots_per_event`
  window). No additional storage or API calls are required; the existing snapshot store
  provides the data.

- **FR-007**: The trajectory consistency factor MUST be computed as `|Σ Δp_i| / Σ|Δp_i|`
  across all consecutive snapshot pairs within the detection window, where Δp_i is the signed
  change between each adjacent pair. Result is ∈ [0, 1]. A perfectly monotonic path scores 1.0;
  a perfectly oscillating path that nets zero direction scores 0.0.

- **FR-008**: The ranking algorithm MUST be covered by table-driven unit tests with at minimum
  one test case per factor demonstrating that the factor materially affects ranking outcome.
  Required cases: volume dominance, SNR dominance (quiet market beats volatile market), KL
  regime difference (same magnitude different probability regimes), trajectory consistency
  (monotonic vs. oscillating paths with same net change), degenerate probabilities, zero volume
  floor, insufficient-history SNR fallback, and determinism under equal scores.

- **FR-009**: Configuration documentation (`config.yaml.example`) MUST document `sensitivity`
  with at least 3 concrete examples: a permissive setting with expected typical event count,
  a default setting, and a strict setting. The documentation MUST be sufficient for a user to
  tune sensitivity without reading source code.

### Scoring Model

The composite score is the product of four factors. Each factor is dimensionless and bounded
on a reasonable positive range. The product is used directly for ranking (higher is better)
and compared against the minimum score floor derived from `sensitivity`.

**Factor 1 — KL Divergence (information content)**

```
KL(p_old → p_new) = p_new × ln(p_new / p_old) + (1 − p_new) × ln((1 − p_new) / (1 − p_old))
```

Probabilities MUST be clamped to [ε, 1−ε] before computation. KL divergence measures how
much the market revised its beliefs; it is naturally larger for moves that were more
surprising given the prior. Note: this is the reverse KL (from new to old), which weights
the surprise at the posterior.

**Factor 2 — Log Volume Weight (market credibility)**

```
V_weight = log(1 + V_24h / V_ref) / log(2)
```

Where V_ref is `volume_24hr_min` from config (default $25K). At V_24h = V_ref the weight is
1.0. At 10× V_ref the weight is log(11)/log(2) ≈ 3.46. Logarithmic scaling prevents large
markets from completely dominating — a $10M market is credible, not 400× more important.

**Factor 3 — Historical SNR (per-market noise floor)**

```
σ_hist = stddev(Δp_i) across stored snapshots for this event
SNR = clamp(|Δp_net| / σ_hist, 0.5, 5.0)
```

Where Δp_net = p_newest − p_oldest in the window, and Δp_i = p_{i+1} − p_i across all stored
snapshots (not just the window). The clamp prevents a single data point from dominating. When
σ_hist is too small to be meaningful (< 1e-4), SNR = 1.0 (neutral fallback). When fewer than
2 historical changes exist, SNR = 1.0.

**Factor 4 — Trajectory Consistency (path quality)**

```
consistency = |Σ Δp_i| / Σ|Δp_i|   across all snapshot pairs in the detection window
```

Where Δp_i = p_{i+1} − p_i for each consecutive pair. Result ∈ [0, 1]. A market moving
cleanly in one direction scores 1.0. A market that oscillates and nets the same endpoint
scores near 0. When the window contains only one snapshot pair, consistency = 1.0.

**Composite:**

```
score = KL(p_old → p_new) × V_weight × SNR × consistency
```

Ties broken by lexicographic event composite ID (descending) for stability.

### Key Entities

- **SignalScore**: A computed scalar and its four component values for a detected change.
  Not persisted — recomputed from `Change` + `Event` + snapshot history each cycle.

- **SensitivityConfig**: User-facing config under `monitor`. Contains `sensitivity` (float64,
  [0.0, 1.0]) and optionally explicit weight overrides per factor for advanced tuning.

- **SnapshotHistory**: The existing stored snapshots per event, now serving double duty: both
  detection window (oldest-to-newest within `window` duration) and historical volatility
  baseline (all stored snapshots for σ_hist computation).

---

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In unit tests, a high-volume market with lower raw magnitude consistently outranks
  a low-volume market with higher magnitude when the volume difference is ≥ 10×. Pass rate: 100%.

- **SC-002**: In unit tests, a normally-quiet market (σ_hist = 0.5%) moving 2% outranks a
  volatile market (σ_hist = 4%) moving 3%. The SNR factor must produce this inversion. Pass rate: 100%.

- **SC-003**: In unit tests, a monotonic-path market with trajectory_consistency = 1.0 outranks
  a noisy-path market with trajectory_consistency < 0.3, given otherwise identical inputs. Pass
  rate: 100%.

- **SC-004**: Given identical input data, the ranked output is byte-identical across 1000
  successive invocations. Determinism verified in a dedicated test.

- **SC-005**: Changing `sensitivity` from 0.3 to 0.7 on a fixture dataset of 20 markets
  reduces the number of emitted entries by ≥ 30% (directional monotonicity of the parameter).

- **SC-006**: Zero entries are emitted when sensitivity = 1.0 and all fixture events have
  composite scores below 0.001. Test verified without any Telegram call being made.

- **SC-007**: The unit test suite includes ≥ 8 table-driven cases (per FR-008) and all pass.

- **SC-008**: No existing passing tests are broken.

---

## Assumptions

- Volume field `volume_24h` is available on the Event model from the Gamma API — confirmed.
- `max_snapshots_per_event = 24` (current default) provides enough history for a meaningful
  σ_hist after a few polling cycles. No storage changes needed for this feature.
- The existing snapshot store sorts by timestamp and is accessible for arbitrary lookups per
  event — confirmed by codebase review.
- Default categories (geopolitics, tech, finance, world) are unchanged.
- The `threshold` removal (FR-005) is a breaking config change and should be noted in the
  release changelog. Users with `threshold` in their config.yaml will need to migrate to
  `sensitivity`.
