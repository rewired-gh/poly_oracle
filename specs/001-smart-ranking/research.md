# Research: Smart Signal Ranking

**Branch**: `001-smart-ranking` | **Date**: 2026-02-17

## 1. KL Divergence Direction

**Decision**: Use `KL(p_new || p_old)` — new distribution relative to old.

**Formula**:
```
KL(p_new || p_old) = p_new × ln(p_new / p_old) + (1 − p_new) × ln((1 − p_new) / (1 − p_old))
```

**Rationale**: We are asking "how surprising is this update?" from the perspective of the new
data. This is the standard choice when the new distribution is the "truth" and the old is the
prior we are measuring surprise against. The result is always ≥ 0 (Gibbs' inequality).

**Boundary handling**: KL is undefined at p = 0 or p = 1 (ln(0) = −∞). Clamp all
probabilities to [1e-7, 1 − 1e-7] before computation. This is standard practice.

**Numerical validation**:
```
KL(0.55 || 0.50) = 0.55 × ln(1.1) + 0.45 × ln(0.9) = 0.0524 − 0.0474 = 0.0050
KL(0.60 || 0.50) = 0.60 × ln(1.2) + 0.40 × ln(0.8) = 0.1094 − 0.0892 = 0.0202
KL(0.95 || 0.90) = 0.95 × ln(1.056) + 0.05 × ln(0.5) = 0.0514 − 0.0347 = 0.0167
```
A 10% move at p=0.5 (KL=0.020) scores 4× a 5% move (KL=0.005). Expected.

**Alternatives considered**:
- **Forward KL `KL(p_old || p_new)`**: Measures surprise from the prior's perspective; less
  natural here since we want to measure how different the new data is.
- **Jensen-Shannon Divergence**: Symmetric, bounded [0, ln 2]; would work but loses the
  directional surprise interpretation. Overkill for this use case.
- **Binary entropy change `|H(p_new) − H(p_old)|`**: Simpler but conflates direction and
  magnitude; doesn't capture the asymmetric surprise of extreme probability moves.

---

## 2. Log Volume Weight — Formula and Calibration

**Decision**: `V_weight = log(1 + V_24h / V_ref) / log(2)` normalized so V_ref → weight 1.0.

Where `V_ref = volume_24hr_min` (default $25K from config).

**Rationale**: Logarithmic scaling prevents high-volume markets from crowding out everything
else. The normalization ensures the weight is a meaningful multiplier, not an arbitrary number.

**Numerical validation**:
```
V = $25K  (= V_ref): log(2)/log(2) = 1.00  (baseline)
V = $100K (4× ref) : log(5)/log(2) = 2.32
V = $500K (20× ref): log(21)/log(2) = 4.39
V = $1M   (40× ref): log(41)/log(2) = 5.36
V = $0             : log(1)/log(2) = 0.00  → clamped to 0.1 (floor)
```

**Zero-volume floor**: A market with no 24h volume still gets a small weight (0.1) to avoid
completely suppressing potentially valid moves on newly listed or illiquid markets. The floor
is a matter of product judgment; 0.1 represents "almost irrelevant but not zero."

---

## 3. Historical SNR — Standard Deviation Computation

**Decision**: Compute `σ_hist` as the sample standard deviation of all consecutive probability
changes `Δp_i = p_{i+1} − p_i` across the full stored snapshot history for the event (market).

**Formula**:
```
μ = mean(Δp_i)
σ_hist = sqrt( Σ(Δp_i − μ)² / (n − 1) )   (Bessel's correction for small samples)
```

**Fallbacks**:
- n < 2 (fewer than 2 Δp values): `SNR = 1.0` (neutral, no history)
- σ_hist < 1e-4 (practically zero variance): `SNR = 1.0` (neutral, avoid division by ~0)

**Clamping**: `SNR = clamp(|Δp_net| / σ_hist, 0.5, 5.0)` where Δp_net is newest − oldest in
the detection window. The clamp floor of 0.5 prevents SNR from penalizing events too heavily
when they have noisy history; the cap of 5.0 prevents a single outlier from dominating.

**Multi-market note**: Each market has its own composite ID and its own snapshot history. SNR
is computed per-market, not per-event. This is correct: a quiet market within a multi-market
event may be more significant than a volatile one even if they share the same event volume.

---

## 4. Trajectory Consistency

**Decision**: `consistency = |Σ Δp_i| / Σ|Δp_i|` across all consecutive pairs in the detection
window. Result ∈ [0, 1].

**Rationale**: The oldest-vs-newest snapshot comparison in DetectChanges gives the net change.
But a net change of +5% could be a clean monotonic rise or a +20% followed by a −15%. The
trajectory factor penalizes oscillations and rewards directional conviction.

**Fallback**: When the window has only one snapshot pair (2 snapshots), consistency = 1.0.
There is no path to be inconsistent.

**Example calculations**:
```
[0.50, 0.52, 0.55, 0.58]: Δ = +0.02, +0.03, +0.03 → |0.08|/0.08 = 1.00 (monotonic)
[0.50, 0.65, 0.45, 0.58]: Δ = +0.15, -0.20, +0.13 → |0.08|/0.48 = 0.17 (noisy)
[0.50, 0.58]:              Δ = +0.08 (1 pair)    → 1.00 (default)
```

---

## 5. Sensitivity → Minimum Composite Score Mapping

**Decision**: `min_score = sensitivity² × 0.05`

**Calibration** (based on expected score ranges for real prediction market data):
```
Weak signal   (small move, low volume, noisy):   ≈ 0.001–0.003
Medium signal (5% move, $100K vol, clean path):  ≈ 0.012–0.030
Strong signal (10% move, $1M vol, monotonic):    ≈ 0.10–0.30
```

**Mapping with C = 0.05**:
```
sensitivity = 0.0 → min_score = 0.000  (everything passes)
sensitivity = 0.3 → min_score = 0.005  (permissive, most medium signals pass)
sensitivity = 0.5 → min_score = 0.013  (default, medium and above)
sensitivity = 0.7 → min_score = 0.025  (strict, above-medium only)
sensitivity = 1.0 → min_score = 0.050  (very strict, strong signals only)
```

**Rationale**: Quadratic mapping (sensitivity²) gives more tuning range at the bottom (easy to
open the filter) and sharp cutoff at the top. Linear would make high-end tuning too coarse.
The constant 0.05 was chosen so that sensitivity = 1.0 corresponds to a score roughly
achievable only by significant moves (≥ 8%) on liquid markets (≥ $500K) with clean trajectories.

---

## 6. Multi-Market Event Volume Handling

**Decision**: Use per-market volume for scoring, estimating `Volume24hr` proportionally
from event-level 24hr volume based on each market's share of weekly volume.

**Rationale**: The Gamma API provides per-market volume fields (`volume1wk`, `volume1mo`)
that differ significantly across markets within the same event. For example, in a
"MicroStrategy sells Bitcoin" event:
- "Sells any Bitcoin in 2025?" has $17.9M volume (97% of total)
- "Sells by Dec 31, 2026?" has $280K volume (1.5% of total)

Using event-level volume for all markets would incorrectly boost the score of low-volume
markets. The API does not provide `volume24hr` per market, so we estimate it:

```
market_volume24hr = event_volume24hr × (market_volume1wk / event_volume1wk)
```

**Fallback**: When `event_volume1wk` is 0 or `market_volume1wk` is 0, fall back to
event-level `volume24hr`.

**Key architectural point**: Each market in an event has a composite ID `EventID:MarketID`
and its own snapshot history. Volume is stored per-market in the `Market` model.

---

## 7. Threshold Deprecation Strategy

**Decision**: Remove `threshold` from `DetectChanges` signature. Replace with a hardcoded
floor of 0.001 (0.1% minimum probability change) to suppress floating-point noise.

**Rationale**: The old threshold (default 4%) was a blunt filter that pre-empted scoring.
With composite scoring, a 3% move on a normally-stable market might score higher than a 6%
move on a volatile one. Removing threshold lets the composite score do all filtering via
`min_score`.

The hardcoded 0.001 floor serves a different purpose: it avoids creating Change records for
probability changes that are effectively floating-point rounding artifacts from the API. This
is not a user-tunable parameter.

**Migration**: Users with `threshold: 0.04` in their config will need to switch to
`sensitivity: 0.5` (default, roughly equivalent behavior on most markets). This is a breaking
change and must be documented in the changelog.

---

## 8. Storage Access Pattern for Scoring

**Decision**: The `ScoreAndRank` method on `Monitor` uses direct `storage` access (already
injected via `monitor.New(store)`). It calls `GetSnapshots(eventID)` (full history, for SNR)
and looks up the event via the `events` map passed in from main.go.

**Rationale**: Passing the full events map to ScoreAndRank is necessary because the volume
comes from the Event model, which is best fetched once in main.go rather than re-queried per
change. This avoids N lock acquisitions in storage for N changes.

**Implementation approach**:
- main.go builds `eventsMap map[string]*models.Event` after `store.GetAllEvents()` (already done)
- main.go passes eventsMap to `mon.ScoreAndRank(changes, eventsMap, minScore, k)`
- ScoreAndRank iterates changes, looks up event by composite ID, calls `m.storage.GetSnapshots`
  once per unique event ID to get historical data
