#!/usr/bin/env python3
"""
Clearer demonstration of window_size trade-offs with visual scoring.
"""

import math


def calculate_score(p0, p1, volume_delta, liquidity, sigma, tc_buffer):
    """Calculate composite score."""
    epsilon = 1e-9
    delta = 0.005

    h_dist = math.sqrt(1 - (math.sqrt(p1 * p0) + math.sqrt((1 - p1) * (1 - p0))))
    price_delta = abs(p1 - p0)
    load_ratio = volume_delta / ((price_delta + delta) * liquidity + epsilon)
    liq_pressure = min(max(load_ratio / 10, -1), 1)
    inst_energy = (h_dist * liq_pressure) / (sigma + epsilon)

    # TC = sum of absolute values (trajectory consistency)
    tc = sum(abs(v) for v in tc_buffer) if tc_buffer else 0

    final_score = inst_energy * math.sqrt(tc + epsilon)

    return final_score, inst_energy, tc


def main():
    print("=" * 70)
    print("WINDOW SIZE VISUALIZATION")
    print("=" * 70)

    # Scenario: Price moves from 0.30 -> 0.36 over 3 cycles
    # This is a consistent 2% per cycle movement (trend)
    print("\nScenario: Gradual 2% price increase for 3 cycles")
    print("-" * 70)

    prices = [0.30, 0.32, 0.34, 0.36]
    volumes = [50000, 100000, 150000, 100000]
    liquidity = 100000
    sigma = 0.05

    print("\nPrice evolution:")
    for i, (p, v) in enumerate(zip(prices, volumes)):
        change = abs(p - prices[i-1]) if i > 0 else 0
        print(f"  Cycle {i}: P={p:.2f} (Δ={change:.2f}) Vol=${v/1000:.0f}K")

    print("\nScore calculation with different window_sizes:")
    print("-" * 70)

    for window_size in [1, 2, 3, 4, 5]:
        print(f"\nwindow_size = {window_size}")
        tc_buffer = []

        for i in range(1, len(prices)):
            p0, p1 = prices[i-1], prices[i]
            vol_delta = volumes[i]

            score, inst_energy, tc = calculate_score(
                p0, p1, vol_delta, liquidity, sigma, tc_buffer
            )

            # Add to buffer
            direction = 1.0 if p1 > p0 else -1.0
            tc_buffer.append(inst_energy * direction)
            if len(tc_buffer) > window_size:
                tc_buffer.pop(0)

            print(f"  Cycle {i}: score={score:.2f}  inst_energy={inst_energy:.2f}  "
                  f"tc={tc:.2f}  buffer_size={len(tc_buffer)}")

    print("\n" + "=" * 70)
    print("KEY INSIGHT: TC (Trajectory Consistency) grows with consistent movement")
    print("=" * 70)

    print("\nFor window_size=1:")
    print("  - TC = |current_inst_energy|")
    print("  - Score = inst_energy × √(|inst_energy|)")
    print("  - Only looks at current cycle")

    print("\nFor window_size=3:")
    print("  - TC = |inst_energy₁| + |inst_energy₂| + |inst_energy₃|")
    print("  - Score = inst_energy × √(sum_of_3_energies)")
    print("  - Builds up over 3 cycles of consistent movement")

    print("\n" + "=" * 70)
    print("PRACTICAL IMPLICATIONS")
    print("=" * 70)

    print("\n1. SUDDEN SPIKE (one large movement):")
    print("   window_size=1: Score = high × √(high) = very high")
    print("   window_size=3: Score = high × √(high) = very high")
    print("   → Both detect it, but ws=1 clears faster")

    print("\n2. GRADUAL TREND (consistent small movements):")
    print("   window_size=1: Score = small × √(small) = small")
    print("   window_size=3: Score = small × √(small×3) = medium")
    print("   → Larger window amplifies sustained trends")

    print("\n3. NOISE (random up/down):")
    print("   window_size=1: Score = varies randomly")
    print("   window_size=3: Score = inst × √(mixed directions) = reduced")
    print("   → Larger window dampens oscillation")

    print("\n" + "=" * 70)
    print("CONCLUSION")
    print("=" * 70)
    print("\nIncreasing window_size is NOT 'better' - it's a TRADE-OFF:")
    print("\n✓ LARGER window:")
    print("  + Better at detecting sustained trends")
    print("  + Better at filtering noise/random oscillations")
    print("  - Slower to react to sudden changes")
    print("  - Alerts persist longer (buffer takes time to clear)")
    print("\n✓ SMALLER window:")
    print("  + Faster reaction to any change")
    print("  + Alerts clear quickly after movement stops")
    print("  - More susceptible to noise")
    print("  - May miss gradual trends (low individual scores)")
    print("\n" + "=" * 70)
    print("\nRECOMMENDED: window_size=3 (MODERATE profile)")
    print("Balances trend detection with noise filtering.")
    print("=" * 70)


if __name__ == '__main__':
    main()
