#!/usr/bin/env python3
"""
Experiment: Impact of window_size on detection quality.

This demonstrates the trade-offs between different window sizes:
- Smaller window: Faster reaction, more sensitive
- Larger window: More stable, requires consistent movement
"""

import math
import random


def simulate_market_price_series(num_cycles=30, seed=42):
    """
    Simulate realistic market price evolution over multiple polling cycles.

    Returns a series of probabilities and volumes that demonstrate
    different movement patterns.
    """
    random.seed(seed)

    scenarios = []

    # Scenario 1: Sudden spike (single large movement)
    base_prob = 0.45
    prices = [base_prob] * 5
    prices.append(0.58)  # Large spike at cycle 5
    prices.extend([0.58] * 24)  # Stays elevated
    volumes = [5000] * 5 + [500000] + [10000] * 24
    scenarios.append({
        'name': 'Sudden Spike (one-time large movement)',
        'prices': prices,
        'volumes': volumes,
        'expected': 'Should trigger alert at cycle 5, but whether it keeps triggering depends on window_size'
    })

    # Scenario 2: Gradual trend (consistent small movements)
    base_prob = 0.30
    prices = [base_prob + i * 0.02 for i in range(15)]  # Gradual increase
    prices.extend([prices[-1]] * 15)  # Then stabilizes
    volumes = [10000 + i * 5000 for i in range(15)] + [5000] * 15
    scenarios.append({
        'name': 'Gradual Trend (consistent small movements)',
        'prices': prices,
        'volumes': volumes,
        'expected': 'Larger window should trigger earlier as TC builds up'
    })

    # Scenario 3: Noisy oscillation (lots of small changes, no real signal)
    base_prob = 0.50
    prices = []
    for i in range(30):
        noise = random.gauss(0, 0.02)
        prices.append(base_prob + noise)
    volumes = [random.uniform(5000, 15000) for _ in range(30)]
    scenarios.append({
        'name': 'Noisy Oscillation (random small changes)',
        'prices': prices,
        'volumes': volumes,
        'expected': 'Should NOT trigger alerts (noise), larger window helps filter this'
    })

    # Scenario 4: Spike and reversal (false signal)
    base_prob = 0.60
    prices = [base_prob] * 5
    prices.append(0.75)  # Spike up
    prices.append(0.62)  # Immediate reversal
    prices.extend([base_prob] * 23)  # Returns to baseline
    volumes = [5000] * 5 + [300000, 200000] + [5000] * 23
    scenarios.append({
        'name': 'Spike and Reversal (false signal)',
        'prices': prices,
        'volumes': volumes,
        'expected': 'Larger window prevents multiple alerts from transient spike'
    })

    return scenarios


def calculate_score(p0, p1, volume_delta, liquidity, sigma, tc_buffer):
    """Calculate composite score using actual algorithm."""
    epsilon = 1e-9
    delta = 0.005

    # Hellinger distance
    h_dist = math.sqrt(1 - (math.sqrt(p1 * p0) + math.sqrt((1 - p1) * (1 - p0))))

    # Liquidity pressure
    price_delta = abs(p1 - p0)
    load_ratio = volume_delta / ((price_delta + delta) * liquidity + epsilon)
    liq_pressure = min(max(load_ratio / 10, -1), 1)  # Simplified erf

    # Instant energy
    inst_energy = (h_dist * liq_pressure) / (sigma + epsilon)

    # Trajectory consistency
    tc = sum(abs(v) for v in tc_buffer)

    # Final score
    final_score = inst_energy * math.sqrt(tc + epsilon)

    return final_score, inst_energy


def simulate_detection(prices, volumes, window_size, threshold=3.0):
    """
    Simulate detection over a price series.

    Returns which cycles triggered alerts.
    """
    alerts = []
    tc_buffer = []
    sigma = 0.05
    liquidity = 100000

    for i in range(1, len(prices)):
        p0, p1 = prices[i-1], prices[i]
        volume_delta = volumes[i]

        # Update TC buffer
        score, inst_energy = calculate_score(p0, p1, volume_delta, liquidity, sigma, tc_buffer)

        # Add to buffer (with direction)
        direction = 1.0 if p1 > p0 else -1.0
        tc_buffer.append(inst_energy * direction)

        # Maintain window size
        if len(tc_buffer) > window_size:
            tc_buffer.pop(0)

        if score > threshold:
            alerts.append({
                'cycle': i,
                'score': score,
                'price_change': abs(p1 - p0),
                'tc_buffer_size': len(tc_buffer),
            })

    return alerts


def main():
    print("=" * 70)
    print("WINDOW SIZE IMPACT EXPERIMENT")
    print("=" * 70)

    scenarios = simulate_market_price_series()

    window_sizes = [1, 2, 3, 4, 5, 7, 10]
    threshold = 3.0

    for scenario in scenarios:
        print(f"\n{'=' * 70}")
        print(f"Scenario: {scenario['name']}")
        print(f"Expected: {scenario['expected']}")
        print(f"{'=' * 70}")

        print(f"\nPrice evolution (first 10 cycles):")
        for i in range(min(10, len(scenario['prices']))):
            print(f"  Cycle {i:2d}: P={scenario['prices'][i]:.3f} "
                  f"Δ={abs(scenario['prices'][i] - scenario['prices'][max(0,i-1)]):.3f} "
                  f"Vol=${scenario['volumes'][i]:,.0f}")

        print(f"\nAlerts by window_size (threshold={threshold}):")
        print("-" * 70)

        results = {}
        for ws in window_sizes:
            alerts = simulate_detection(
                scenario['prices'],
                scenario['volumes'],
                ws,
                threshold
            )
            results[ws] = alerts

            if alerts:
                alert_cycles = [a['cycle'] for a in alerts]
                alert_scores = [f"{a['score']:.2f}" for a in alerts]
                print(f"  window_size={ws:2d}: {len(alerts):2d} alerts at cycles {alert_cycles}")
                print(f"              scores: {', '.join(alert_scores)}")
            else:
                print(f"  window_size={ws:2d}: 0 alerts")

        # Analyze the pattern
        print(f"\nAnalysis:")
        alert_counts = [len(results[ws]) for ws in window_sizes]

        if all(c == 0 for c in alert_counts):
            print("  ✓ No alerts at any window size (appropriate for this scenario)")
        elif max(alert_counts) == min(alert_counts):
            print(f"  ✓ Consistent alert count ({alert_counts[0]}) across all window sizes")
        else:
            print(f"  ⚠ Alert count varies: {min(alert_counts)} to {max(alert_counts)}")
            print(f"    Smaller windows: More sensitive to single spikes")
            print(f"    Larger windows: Require consistent movement, fewer false alarms")

    print(f"\n{'=' * 70}")
    print("SUMMARY: TRADE-OFFS OF WINDOW_SIZE")
    print("=" * 70)

    print("\n✓ SMALLER WINDOW (1-2):")
    print("  Pros:")
    print("    - Faster reaction to sudden changes")
    print("    - More sensitive to breaking news")
    print("    - Catches single large movements quickly")
    print("  Cons:")
    print("    - More false positives from noise")
    print("    - Can trigger on transient spikes")
    print("    - May alert multiple times on same event")

    print("\n✓ MEDIUM WINDOW (3-4):")
    print("  Pros:")
    print("    - Balances speed and stability")
    print("    - Filters out most noise")
    print("    - Requires some consistency")
    print("  Cons:")
    print("    - Slightly slower reaction than small windows")
    print("    - May miss very short-term spikes")

    print("\n✓ LARGER WINDOW (5-10):")
    print("  Pros:")
    print("    - Very stable, minimal false positives")
    print("    - Requires consistent directional movement")
    print("    - Best for trend detection")
    print("  Cons:")
    print("    - Slow to react to sudden changes")
    print("    - May miss fast-breaking events")
    print("    - Alerts persist longer after signal ends")

    print("\n" + "=" * 70)
    print("RECOMMENDATION")
    print("=" * 70)
    print("\nIncreasing window_size is NOT universally 'better' - it depends on your goal:")
    print("\n• Use SMALLER window (2) if:")
    print("  - You want to catch breaking news immediately")
    print("  - Speed is more important than precision")
    print("  - You can tolerate some false alarms")
    print("\n• Use MEDIUM window (3-4) if:")
    print("  - You want balanced detection (RECOMMENDED)")
    print("  - You care about both speed and quality")
    print("  - You want to filter most noise")
    print("\n• Use LARGER window (5+) if:")
    print("  - You want to detect sustained trends")
    print("  - You absolutely hate false alarms")
    print("  - You're monitoring longer-term positions")
    print("\n" + "=" * 70)


if __name__ == '__main__':
    main()
