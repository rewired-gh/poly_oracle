#!/usr/bin/env python3
"""
Parameter optimization analysis for Polymarket monitoring configurations.

This script analyzes real Polymarket data to recommend optimal parameter sets
for three sensitivity levels:
- SENSITIVE: Catch every important signal (pro investors, full-time monitoring)
- MODERATE: Balance value and timeliness (regular users, timely notifications)
- STRICT: Only highly valuable signals (noise-intolerant users, competitive edge)
"""

import json
import math
import random
from collections import defaultdict
from pathlib import Path


def load_market_data(cache_file='/tmp/poly_events.json'):
    """Load and parse Polymarket event data."""
    with open(cache_file, 'r') as f:
        events = json.load(f)

    markets = []
    for event in events:
        if not event.get('active', False) or event.get('closed', False):
            continue

        vol_24hr = event.get('volume24hr', 0) or 0
        vol_1wk = event.get('volume1wk', 0) or 0
        vol_1mo = event.get('volume1mo', 0) or 0

        for market in event.get('markets', []):
            if not market.get('active', False) or market.get('closed', False):
                continue

            # Extract probability
            outcome_prices = market.get('outcomePrices', '[]')
            try:
                prices = json.loads(outcome_prices)
                yes_prob = float(prices[0]) if len(prices) > 0 else 0.5
            except:
                yes_prob = 0.5

            markets.append({
                'id': market.get('id', ''),
                'event_id': event.get('id', ''),
                'event_title': event.get('title', ''),
                'question': market.get('question', ''),
                'vol_24hr': vol_24hr,
                'vol_1wk': vol_1wk,
                'vol_1mo': vol_1mo,
                'liquidity': float(market.get('liquidity', 0) or 0),
                'volume': float(market.get('volume', 0) or 0),
                'yes_prob': yes_prob,
            })

    return markets


def analyze_volume_distribution(markets):
    """Analyze volume distribution across markets."""
    volumes_24hr = sorted([m['vol_24hr'] for m in markets if m['vol_24hr'] > 0])
    volumes_1wk = sorted([m['vol_1wk'] for m in markets if m['vol_1wk'] > 0])
    volumes_1mo = sorted([m['vol_1mo'] for m in markets if m['vol_1mo'] > 0])

    def percentile(data, p):
        if not data:
            return 0
        k = (len(data) - 1) * p / 100
        f = int(k)
        c = f + 1 if f + 1 < len(data) else f
        return data[f] + (k - f) * (data[c] - data[f])

    return {
        'total_markets': len(markets),
        '24hr': {
            'p25': percentile(volumes_24hr, 25),
            'p50': percentile(volumes_24hr, 50),
            'p75': percentile(volumes_24hr, 75),
            'p90': percentile(volumes_24hr, 90),
            'p95': percentile(volumes_24hr, 95),
            'max': max(volumes_24hr) if volumes_24hr else 0,
        },
        '1wk': {
            'p50': percentile(volumes_1wk, 50),
            'p75': percentile(volumes_1wk, 75),
            'p90': percentile(volumes_1wk, 90),
            'p95': percentile(volumes_1wk, 95),
        },
        '1mo': {
            'p50': percentile(volumes_1mo, 50),
            'p75': percentile(volumes_1mo, 75),
            'p90': percentile(volumes_1mo, 90),
            'p95': percentile(volumes_1mo, 95),
        },
    }


def simulate_detection(markets, config, num_cycles=50):
    """
    Simulate multiple polling cycles with realistic probability changes.

    Returns statistics about alert frequency and characteristics.
    """
    random.seed(42)  # Reproducible results

    alerts_per_cycle = []
    score_distribution = []

    for cycle in range(num_cycles):
        alerts = []

        for m in markets:
            # Volume filter (OR logic)
            if not (m['vol_24hr'] >= config['volume_24hr_min'] or
                    m['vol_1wk'] >= config['volume_1wk_min'] or
                    m['vol_1mo'] >= config['volume_1mo_min']):
                continue

            # Skip extreme probabilities (less likely to change)
            base_prob = m['yes_prob']
            if base_prob < 0.05 or base_prob > 0.95:
                continue

            # Simulate realistic price changes
            # Most cycles: no change or tiny changes
            # Some cycles: moderate changes
            # Rare cycles: large changes
            change_type = random.random()

            if change_type < 0.7:  # 70% - no significant change
                continue
            elif change_type < 0.9:  # 20% - moderate change (1-5%)
                change_magnitude = random.uniform(0.01, 0.05)
            else:  # 10% - significant change (5-15%)
                change_magnitude = random.uniform(0.05, 0.15)

            direction = random.choice([-1, 1])
            price_delta = change_magnitude * direction
            new_prob = max(0.01, min(0.99, base_prob + price_delta))

            # Volume delta (correlated with price change magnitude)
            if change_magnitude > 0.05:
                volume_delta = random.uniform(50000, 500000)
            else:
                volume_delta = random.uniform(1000, 50000)

            # Calculate composite score (simplified version of actual algorithm)
            # 1. Hellinger distance
            h_dist = math.sqrt(1 - (math.sqrt(new_prob * base_prob) +
                                    math.sqrt((1-new_prob) * (1-base_prob))))

            # 2. Liquidity pressure
            avg_depth = m['liquidity'] * 0.8 if m['liquidity'] > 0 else 1000
            load_ratio = volume_delta / (abs(price_delta) * avg_depth + 100)
            liq_pressure = min(load_ratio / 10, 1.0)

            # 3. Instant energy
            sigma = max(0.01, random.gauss(0.05, 0.02))
            inst_energy = (h_dist * liq_pressure) / sigma

            # 4. Trajectory consistency
            tc_factor = random.uniform(0.5, 2.5)
            final_score = inst_energy * math.sqrt(tc_factor)

            score_distribution.append(final_score)

            if final_score > config['threshold']:
                alerts.append({
                    'market_id': m['id'],
                    'title': m['event_title'],
                    'score': final_score,
                    'price_delta': abs(price_delta),
                    'vol_24hr': m['vol_24hr'],
                })

        # Sort by score and take top_k
        alerts.sort(key=lambda x: x['score'], reverse=True)
        alerts_per_cycle.append(len(alerts[:config['top_k']]))

    # Calculate score percentiles for threshold guidance
    if score_distribution:
        sorted_scores = sorted(score_distribution)
        score_pct = {
            'p50': sorted_scores[int(len(sorted_scores) * 0.5)],
            'p75': sorted_scores[int(len(sorted_scores) * 0.75)],
            'p90': sorted_scores[int(len(sorted_scores) * 0.9)],
            'p95': sorted_scores[int(len(sorted_scores) * 0.95)],
        }
    else:
        score_pct = {'p50': 0, 'p75': 0, 'p90': 0, 'p95': 0}

    return {
        'avg_alerts': sum(alerts_per_cycle) / len(alerts_per_cycle),
        'max_alerts': max(alerts_per_cycle),
        'min_alerts': min(alerts_per_cycle),
        'cycles_with_alerts': sum(1 for a in alerts_per_cycle if a > 0),
        'score_percentiles': score_pct,
    }


def main():
    print("Loading Polymarket data...")
    markets = load_market_data()
    print(f"Loaded {len(markets)} active markets\n")

    print("=" * 70)
    print("VOLUME DISTRIBUTION ANALYSIS")
    print("=" * 70)
    vol_stats = analyze_volume_distribution(markets)

    print(f"Total active markets: {vol_stats['total_markets']}\n")

    print("24hr Volume Distribution:")
    print(f"  P25: ${vol_stats['24hr']['p25']:,.0f}")
    print(f"  P50: ${vol_stats['24hr']['p50']:,.0f}")
    print(f"  P75: ${vol_stats['24hr']['p75']:,.0f}")
    print(f"  P90: ${vol_stats['24hr']['p90']:,.0f}")
    print(f"  P95: ${vol_stats['24hr']['p95']:,.0f}")
    print(f"  Max: ${vol_stats['24hr']['max']:,.0f}\n")

    print("1 Week Volume Distribution:")
    print(f"  P50: ${vol_stats['1wk']['p50']:,.0f}")
    print(f"  P75: ${vol_stats['1wk']['p75']:,.0f}")
    print(f"  P90: ${vol_stats['1wk']['p90']:,.0f}\n")

    print("1 Month Volume Distribution:")
    print(f"  P50: ${vol_stats['1mo']['p50']:,.0f}")
    print(f"  P75: ${vol_stats['1mo']['p75']:,.0f}")
    print(f"  P90: ${vol_stats['1mo']['p90']:,.0f}\n")

    # Define parameter configurations
    configs = {
        'SENSITIVE': {
            'volume_24hr_min': 10000,
            'volume_1wk_min': 50000,
            'volume_1mo_min': 200000,
            'threshold': 2.0,
            'top_k': 15,
            'window_size': 2,
            'alpha': 0.15,
            'ceiling': 10.0,
            'cooldown_multiplier': 3,
        },
        'MODERATE': {
            'volume_24hr_min': 25000,
            'volume_1wk_min': 100000,
            'volume_1mo_min': 500000,
            'threshold': 3.0,
            'top_k': 10,
            'window_size': 3,
            'alpha': 0.1,
            'ceiling': 10.0,
            'cooldown_multiplier': 5,
        },
        'STRICT': {
            'volume_24hr_min': 50000,
            'volume_1wk_min': 250000,
            'volume_1mo_min': 1000000,
            'threshold': 4.5,
            'top_k': 5,
            'window_size': 4,
            'alpha': 0.08,
            'ceiling': 10.0,
            'cooldown_multiplier': 7,
        },
    }

    print("=" * 70)
    print("PARAMETER SET ANALYSIS")
    print("=" * 70)

    for name, config in configs.items():
        print(f"\n{name} Configuration:")
        print("-" * 70)

        # Count markets passing volume filter
        passing = [m for m in markets if
                   m['vol_24hr'] >= config['volume_24hr_min'] or
                   m['vol_1wk'] >= config['volume_1wk_min'] or
                   m['vol_1mo'] >= config['volume_1mo_min']]

        print(f"Markets passing volume filter: {len(passing)} ({len(passing)/len(markets)*100:.1f}%)")

        # Simulate detection
        results = simulate_detection(markets, config)

        print(f"Simulation results (50 cycles):")
        print(f"  Avg alerts per cycle: {results['avg_alerts']:.2f}")
        print(f"  Range: {results['min_alerts']}-{results['max_alerts']} alerts")
        print(f"  Cycles with alerts: {results['cycles_with_alerts']}/50")

        print(f"\nConfiguration:")
        print(f"  Volume thresholds: ${config['volume_24hr_min']/1000:.0f}K 24hr | "
              f"${config['volume_1wk_min']/1000:.0f}K 1wk | "
              f"${config['volume_1mo_min']/1000000:.1f}M 1mo")
        print(f"  Score threshold: {config['threshold']}")
        print(f"  Top K: {config['top_k']}")
        print(f"  Window size: {config['window_size']}")
        print(f"  Alpha: {config['alpha']}")
        print(f"  Cooldown multiplier: {config['cooldown_multiplier']}")

        # User persona
        if name == 'SENSITIVE':
            print(f"\nTarget user: Pro investors monitoring 8+ hrs/day")
            print(f"  - Catches most signals (lower false negative rate)")
            print(f"  - Higher volume of notifications expected")
        elif name == 'MODERATE':
            print(f"\nTarget user: Regular users wanting timely updates")
            print(f"  - Balances signal quality and quantity")
            print(f"  - Moderate notification frequency")
        else:  # STRICT
            print(f"\nTarget user: Noise-intolerant users seeking edge")
            print(f"  - Only strong, high-confidence signals")
            print(f"  - Minimal notifications, maximum quality")

    print("\n" + "=" * 70)
    print("RECOMMENDATIONS")
    print("=" * 70)
    print("\n1. SENSITIVE config best for:")
    print("   - Full-time traders/researchers who can process many alerts")
    print("   - Use cases where missing a signal is costly")
    print("   - High-frequency monitoring (poll_interval: 2-5m)")

    print("\n2. MODERATE config best for:")
    print("   - Regular users checking notifications periodically")
    print("   - Standard monitoring frequency (poll_interval: 5-15m)")
    print("   - Good balance of sensitivity and noise reduction")

    print("\n3. STRICT config best for:")
    print("   - Users who hate notification noise")
    print("   - Long-term position monitoring (poll_interval: 15-30m)")
    print("   - Focus on high-impact, high-confidence events only")

    print("\n" + "=" * 70)


if __name__ == '__main__':
    main()
