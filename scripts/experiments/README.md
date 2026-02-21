# Experiments

This directory contains scripts used to optimize Polyoracle's detection parameters.

## Scripts

### `analyze_parameters.py`
Main analysis script that:
- Fetches real Polymarket market data
- Analyzes volume distribution across 3,680+ active markets
- Simulates detection with three sensitivity profiles
- Reports expected alert frequencies and market coverage

**Usage:**
```bash
python3 scripts/experiments/analyze_parameters.py
```

### `validate_with_real_data.py`
Validates parameter recommendations using:
- Real market characteristics (volume, liquidity, probability)
- Simulated realistic price changes based on volume tiers
- Actual scoring algorithm (Hellinger × LiqPressure × TC)

**Usage:**
```bash
python3 scripts/experiments/validate_with_real_data.py
```

### `fetch_multi_snapshot.py`
Score distribution analysis that:
- Simulates 1,000 realistic market change scenarios
- Calculates actual composite scores using the detection algorithm
- Validates thresholds against score percentiles
- Shows which price changes trigger alerts at each sensitivity level

**Usage:**
```bash
python3 scripts/experiments/fetch_multi_snapshot.py
```

### `test_window_size.py`
Demonstrates the impact of `window_size` on detection behavior:
- Tests different window sizes against 4 market scenarios
- Shows how window size affects alert frequency
- Explains trade-offs between speed and stability

**Usage:**
```bash
python3 scripts/experiments/test_window_size.py
```

### `window_size_tradeoffs.py`
Visual demonstration of window size mechanics:
- Shows how TC (trajectory consistency) builds up
- Compares scores across different window sizes
- Explains the formula: Score = InstEnergy × √(TC)

**Usage:**
```bash
python3 scripts/experiments/window_size_tradeoffs.py
```

## Methodology

1. **Data Collection:** Fetch active markets from Polymarket Gamma API
2. **Volume Analysis:** Calculate volume percentiles to set thresholds
3. **Simulation:** Generate realistic price change scenarios
4. **Scoring:** Calculate composite scores using actual algorithm
5. **Validation:** Ensure each profile captures appropriate signal levels

## Understanding Parameters

### Window Size

The `window_size` parameter is NOT about "better" detection - it's a trade-off:

- **Smaller (1-2):** Faster reaction, more noise, best for breaking news
- **Medium (3-4):** Balanced, recommended for most users
- **Larger (5+):** Slower, stable, best for trend detection

See `docs/parameter-tuning.md` for detailed explanation and run the visualization scripts to understand the mechanics.

## Results

The experiments determined three optimal sensitivity profiles:

| Profile | Threshold | Markets Covered | Expected Alerts |
|---------|-----------|----------------|-----------------|
| SENSITIVE | 2.0 | 35.7% (1,313) | 10-15 per cycle |
| MODERATE | 3.0 | 26.1% (960) | 5-10 per cycle |
| STRICT | 4.5 | 22.2% (818) | 0-5 per cycle |

See `docs/parameter-tuning.md` for detailed analysis and recommendations.
