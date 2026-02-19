# Configuration Tuning Guide

**Last updated**: 2026-02-17
**Based on**: analysis of 228 active events across geopolitics / tech / finance / crypto / world categories

For the full list of config fields and their defaults see [`configs/config.yaml.example`](../configs/config.yaml.example).
For valid category slugs see [`docs/valid-categories.md`](valid-categories.md).

---

## Key Findings

- **Top 10%** of events by volume have $100K+ 24hr volume
- **Top 20%** of events by volume have $25K+ 24hr volume
- Crypto overlaps significantly with tech/finance and is worth including
- World / politics categories have the highest raw volume

### Highest-Volume Events (sample)

| Rank | 24hr Volume | Categories | Title |
|------|-------------|------------|-------|
| 1 | $14.6M | politics, world, elections | Dutch government coalition |
| 2 | $6.0M | geopolitics, middle-east | US strikes Iran |
| 3 | $2.5M | bitcoin, crypto-prices | Bitcoin price in February |
| 4 | $1.9M | bitcoin, weekly | Bitcoin above X on Feb 16 |
| 5 | $1.7M | venezuela, trump | Venezuela leader end of 2026 |
| 6 | $1.6M | sports, olympics | 2026 Winter Olympics medals |
| 7 | $1.3M | big-tech, openai, ai | Best AI model end of February |
| 8 | $1.0M | finance, big-tech | Largest company end of February |

---

## Adjustment Guidelines

### Too many alerts (>5 per cycle consistently)

```yaml
# Increase volume thresholds
volume_24hr_min: 200000
volume_1wk_min: 1000000
volume_1mo_min: 4000000

# Raise sensitivity (stricter quality gate)
sensitivity: 0.8
```

### Too few alerts (<1 per cycle consistently)

```yaml
# Lower volume thresholds
volume_24hr_min: 25000
volume_1wk_min: 100000
volume_1mo_min: 250000

# Lower sensitivity (more permissive)
sensitivity: 0.5

# Widen detection window
detection_intervals: 12
```

### High noise (alerts on minor/irrelevant events)

```yaml
# Raise minimum absolute change (hard filter runs before KL scoring)
min_abs_change: 0.15

# Raise sensitivity
sensitivity: 0.8
```

---

## Expected Alert Variability

| Period | Alerts per cycle |
|--------|-----------------|
| Quiet | 0–1 |
| Normal | 2–5 |
| Active news | 5–10 |
| Major event unfolding | 10+ |

Tune for your own tolerance. The example config targets 0–3 high-conviction alerts per cycle.

---

## Testing Strategy

### End-to-end verification

Lower thresholds temporarily to guarantee notifications within 1–2 hours:

```yaml
volume_24hr_min: 5000
volume_1wk_min: 25000
volume_1mo_min: 50000
sensitivity: 0.3
min_abs_change: 0.02
```

Enable debug logging during testing:

```yaml
logging:
  level: debug
  format: text
```

### Production rollout

1. Deploy with `configs/config.yaml.example` defaults
2. Monitor for 24–48 hours; check actual alert frequency
3. Adjust thresholds using the guidelines above
