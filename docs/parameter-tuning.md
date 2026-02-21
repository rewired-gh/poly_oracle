# Parameter Tuning Guide

This document explains the three sensitivity profiles for Polyoracle and how they were optimized using real Polymarket data.

## Overview

Polyoracle uses a composite scoring algorithm to detect significant probability changes in prediction markets. The algorithm balances four factors:

1. **Hellinger Distance** - Measures probability distribution shift
2. **Liquidity Pressure** - Weighted by trading volume relative to market depth
3. **Instant Energy** - Normalized by historical volatility (sigma)
4. **Trajectory Consistency** - Confirms directional movement over time

The final score determines whether to trigger an alert:

```
Score = InstEnergy × √(TrajectoryConsistency)
```

## Sensitivity Profiles

### 1. SENSITIVE - "Catch Every Important Signal"

**Best for:** Pro investors monitoring markets 8+ hours/day

**Configuration:**
```yaml
monitor:
  window_size: 2              # Faster reaction to changes
  alpha: 0.15                 # More responsive depth tracking
  threshold: 2.0              # Lower threshold, catches more signals

  volume_24hr_min: 10000      # $10K minimum
  volume_1wk_min: 50000       # $50K minimum
  volume_1mo_min: 200000      # $200K minimum

  top_k: 15                   # More alerts per cycle
  cooldown_multiplier: 3      # Shorter cooldown
```

**Characteristics:**
- **Markets monitored:** 35.7% of active markets (1,313 out of 3,680)
- **Expected alerts:** 10-15 per cycle (varies by market activity)
- **Alerts trigger on:** Significant (5%+) and major (10%+) price changes
- **False negative rate:** Low - catches most meaningful movements

**Use case:**
- Professional traders who can process many notifications
- High-frequency monitoring (poll_interval: 2-5 minutes)
- Situations where missing a signal is costly

---

### 2. MODERATE - "Balance Value and Timeliness" (DEFAULT)

**Best for:** Regular users wanting timely notifications

**Configuration:**
```yaml
monitor:
  window_size: 3              # Balanced trajectory tracking
  alpha: 0.1                  # Standard depth smoothing
  threshold: 3.0              # Moderate quality bar

  volume_24hr_min: 25000      # $25K minimum
  volume_1wk_min: 100000      # $100K minimum
  volume_1mo_min: 500000      # $500K minimum

  top_k: 10                   # Moderate alert volume
  cooldown_multiplier: 5      # Standard cooldown
```

**Characteristics:**
- **Markets monitored:** 26.1% of active markets (960 out of 3,680)
- **Expected alerts:** 5-10 per cycle
- **Alerts trigger on:** Most significant (5%+) and almost all major (10%+) changes
- **Balance:** Good signal quality with reasonable notification frequency

**Use case:**
- Regular users checking notifications periodically
- Standard monitoring frequency (poll_interval: 5-15 minutes)
- Best starting point for most users

---

### 3. STRICT - "Only Highly Valuable Signals"

**Best for:** Noise-intolerant users seeking competitive edge

**Configuration:**
```yaml
monitor:
  window_size: 4              # More stable trajectory
  alpha: 0.08                 # Smoother depth tracking
  threshold: 4.5              # High threshold, strong signals only

  volume_24hr_min: 50000      # $50K minimum
  volume_1wk_min: 250000      # $250K minimum
  volume_1mo_min: 1000000     # $1M minimum

  top_k: 5                    # Fewer alerts
  cooldown_multiplier: 7      # Longer cooldown
```

**Characteristics:**
- **Markets monitored:** 22.2% of active markets (818 out of 3,680)
- **Expected alerts:** 0-5 per cycle
- **Alerts trigger on:** Only major events (10%+ changes) with strong volume
- **Noise tolerance:** Minimal false positives, maximum signal quality

**Use case:**
- Users who hate notification spam
- Long-term position monitoring (poll_interval: 15-30 minutes)
- Focus on high-impact, high-confidence events only

---

## Validation Results

### Score Distribution (1,000 simulated scenarios)

| Percentile | Score | Typical Trigger |
|------------|-------|-----------------|
| P50 | 0.09 | Minimal noise |
| P75 | 0.68 | Small movements |
| P90 | 1.96 | Moderate changes |
| P95 | 3.29 | Significant movements |
| P99 | 5.94 | Major events |

### Alert Rate by Price Change

| Price Change | SENSITIVE | MODERATE | STRICT |
|--------------|-----------|----------|--------|
| Minimal (<2%) | 0% | 0% | 0% |
| Moderate (2-5%) | 2.8% | 0% | 0% |
| Significant (5-10%) | 52.9% | 20.6% | 8.8% |
| Major (10-20%) | 100% | 92.3% | 61.5% |

### Volume Filter Coverage

| Profile | Markets Passing | % of Total |
|---------|----------------|------------|
| SENSITIVE | 1,313 | 35.7% |
| MODERATE | 960 | 26.1% |
| STRICT | 818 | 22.2% |

---

## How Parameters Were Optimized

### Methodology

1. **Fetched real data:** Downloaded 3,680 active markets from Polymarket Gamma API
2. **Analyzed volume distribution:** Calculated percentiles for 24hr, 1wk, 1mo volumes
3. **Simulated realistic changes:** Generated 1,000 scenarios with realistic price movements
4. **Calculated actual scores:** Used the real detection algorithm (Hellinger × LiqPressure × TC)
5. **Validated thresholds:** Ensured each profile captures appropriate signal levels

### Key Findings

**Volume thresholds:**
- Top 35% of markets by volume generate most meaningful signals
- Markets below $10K 24hr volume have low signal-to-noise ratio
- High-volume markets ($100K+ 24hr) are more stable but produce clearer signals

**Score thresholds:**
- Score distribution follows power law (many small, few large)
- Threshold of 2.0 captures ~10% of meaningful scenarios
- Threshold of 3.0 captures ~6% (good balance)
- Threshold of 4.5 captures ~3% (only major events)

**Window size:**
- Smaller window (2) = faster reaction, more sensitive
- Larger window (4) = more stable, fewer false positives
- Window of 3 provides good balance

---

## Recommendations

### Starting Point

**Begin with MODERATE configuration.** This provides the best balance for most users:
- Reasonable notification frequency (5-10 per cycle)
- Good signal quality (significant and major events)
- Covers 26% of active markets (focuses on liquid markets)

### Adjustment Guidelines

**Switch to SENSITIVE if:**
- You're monitoring markets full-time
- Missing a signal would be costly
- You can process 10-15 notifications per polling cycle
- You want comprehensive coverage

**Switch to STRICT if:**
- Notification noise bothers you
- You only care about major market moves
- You prefer fewer, higher-quality alerts
- You're monitoring longer-term positions

### Poll Interval Recommendations

Match your polling frequency to your sensitivity profile:

| Profile | Recommended poll_interval | Reasoning |
|---------|--------------------------|-----------|
| SENSITIVE | 2-5 minutes | Capture rapid changes |
| MODERATE | 5-15 minutes | Balance timeliness and load |
| STRICT | 15-30 minutes | Focus on major events |

---

## Parameter Deep Dive

### Window Size Trade-offs

The `window_size` parameter controls the **trajectory consistency (TC) buffer length**. It's NOT about making detection "better" - it's about choosing what kind of signals you want to prioritize.

**How it works:**
```
TC = sum of |inst_energy| over last N cycles
Score = inst_energy × √(TC)
```

**Small window (1-2):**
- ✅ **Fastest reaction** to sudden changes
- ✅ Alerts clear quickly after movement stops
- ✅ Best for catching breaking news immediately
- ❌ More susceptible to noise/oscillations
- ❌ May generate false alerts on transient spikes

**Medium window (3-4) - RECOMMENDED:**
- ✅ **Balances speed and stability**
- ✅ Amplifies sustained trends (TC builds up)
- ✅ Filters most random oscillations
- ❌ Slightly slower than small windows
- ❌ May miss very short-term spikes

**Large window (5+):**
- ✅ **Most stable, minimal false positives**
- ✅ Excellent for detecting sustained trends
- ✅ Best noise filtering
- ❌ **Slow to react** to sudden changes
- ❌ Alerts persist longer after signal ends
- ❌ May miss fast-breaking events

**Example:**

Gradual 2% price increase over 3 cycles:

| Cycle | Price Δ | window=1 Score | window=3 Score | window=5 Score |
|-------|---------|----------------|----------------|----------------|
| 1 | +2% | 0.00 | 0.00 | 0.00 |
| 2 | +2% | 0.17 | 0.17 | 0.17 |
| 3 | +2% | 0.16 | **0.23** | **0.23** |

**Key insight:** Larger windows amplify sustained movements because TC accumulates energy across multiple cycles.

**Recommendation:**
- Use `window_size=3` for balanced detection (default)
- Use `window_size=2` if speed > precision
- Use `window_size=4` if you hate false alarms

Run `python3 scripts/experiments/window_size_tradeoffs.py` to see detailed visualization.

---

## Implementation Details

### Switching Profiles

To switch sensitivity profiles, update `configs/config.yaml`:

```yaml
monitor:
  # Copy parameters from desired profile above
  window_size: 3
  alpha: 0.1
  threshold: 3.0
  # ... etc
```

See `configs/config.yaml.example` for complete annotated configuration.

### Experiment Scripts

The following scripts were used to optimize parameters:

- `scripts/experiments/analyze_parameters.py` - Volume distribution and simulation
- `scripts/experiments/validate_with_real_data.py` - Real market validation
- `scripts/experiments/fetch_multi_snapshot.py` - Score distribution analysis

Run these scripts to re-analyze with fresh market data:

```bash
# Fetch current market data
curl -s "https://gamma-api.polymarket.com/events?active=true&closed=false&limit=500" > /tmp/poly_events.json

# Run analysis
python3 scripts/experiments/analyze_parameters.py
python3 scripts/experiments/fetch_multi_snapshot.py
```

---

## Conclusion

The three sensitivity profiles are optimized for different user needs:

- **SENSITIVE:** Comprehensive monitoring for professionals
- **MODERATE:** Balanced alerts for regular users (recommended default)
- **STRICT:** Minimal noise for quality-focused users

All profiles use the same underlying algorithm - they differ only in parameter tuning to match different monitoring styles and notification tolerances.

For questions or adjustments, see the experiment scripts or create an issue on GitHub.
