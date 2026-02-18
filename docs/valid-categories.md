# Valid Polymarket Categories

**Date**: 2026-02-17
**Source**: Polymarket Gamma API (https://gamma-api.polymarket.com/events)

## Category Filtering Verification

### ✅ Current Implementation Status

The category filtering is **WORKING CORRECTLY**:

1. **Config defines categories**: `geopolitics`, `tech`, `finance`, `world`
2. **Code filters by tag slugs**: Lines 127-140 in `internal/polymarket/client.go`
3. **Logic**: Events are included if ANY of their tags match ANY configured category

### Categories in Example Config

The example config (`configs/config.yaml.example`) contains:

```yaml
categories:
  - geopolitics    ✓ VALID (found in API)
  - tech           ✓ VALID (found in API)
  - finance        ✓ VALID (found in API)
  - world          ✓ VALID (found in API)
```

**Status**: All 4 categories are valid and exist in Polymarket API.

---

## Complete List of Valid Categories

Based on API analysis of 100 active events, here are frequently used category slugs:

### High-Value Categories (Recommended for Monitoring)

| Category | Frequency | Description |
|----------|-----------|-------------|
| **politics** | 26 events | Political events, elections |
| **world** | 18 events | Global events, world affairs |
| **geopolitics** | 15 events | Geopolitical events, international relations |
| **crypto** | 8 events | Cryptocurrency markets |
| **tech** | 2+ events | Technology, AI, big tech |
| **finance** | 2+ events | Financial markets, economy |
| **sports** | 4 events | Sports events, championships |
| **soccer** | 3 events | Soccer/Football leagues |
| **ai** | 3 events | Artificial intelligence |
| **elections** | 2 events | Election events |
| **global-elections** | 3 events | International elections |
| **world-elections** | 2 events | World election events |

### Specific Event Categories

| Category | Description |
|----------|-------------|
| trump-presidency | Trump administration events |
| ukraine | Ukraine conflict |
| ukraine-map | Ukraine territorial changes |
| trump-zelenskyy | Trump-Zelenskyy relations |
| israel | Israel-related events |
| putin | Putin/Russia events |
| foreign-policy | Foreign policy decisions |
| world-affairs | World affairs general |
| middle-east | Middle East events |
| courts | Court decisions |
| pop-culture | Pop culture events |

### Crypto/Finance Sub-Categories

| Category | Description |
|----------|-------------|
| bitcoin | Bitcoin price/markets |
| ethereum | Ethereum price/markets |
| crypto-prices | General crypto prices |
| pre-market | Pre-market trading |
| airdrops | Crypto airdrops |
| doge | Dogecoin |
| stablecoins | Stablecoin markets |

### Tech Sub-Categories

| Category | Description |
|----------|-------------|
| ai | Artificial intelligence |
| openai | OpenAI specific |
| big-tech | Big tech companies |
| sam-altman | Sam Altman related |

---

## Category Filtering Behavior

### Current Logic

The code implements **OR logic** for category matching:

```
Event is included if: ANY event tag matches ANY configured category
```

Example:
- Event tags: `[politics, trump-presidency, world]`
- Config categories: `[geopolitics, tech, finance, world]`
- Result: ✅ **MATCH** (matches "world")

### Multiple Category Support

Events often have multiple tags. For example, an event might have:
```
tags: [
  {slug: "politics"},
  {slug: "world"},
  {slug: "trump-presidency"},
  {slug: "geopolitics"}
]
```

This event would match if ANY of these categories are in your config:
- ✅ `politics`
- ✅ `world`
- ✅ `geopolitics`
- ❌ `trump-presidency` (only if in config)

---

## Recommended Category Configurations

### For Geopolitical/Financial Insights (Current Config)

```yaml
categories:
  - geopolitics    # Geopolitical events
  - tech          # Technology events
  - finance       # Financial markets
  - world         # Global events
```

**Expected events**: ~30-50 per cycle

### For Broader Coverage

```yaml
categories:
  - geopolitics
  - tech
  - finance
  - crypto        # Add crypto markets
  - world
  - politics      # Add politics (high volume)
```

**Expected events**: ~80-120 per cycle

### For Testing (Maximum Coverage)

```yaml
categories:
  - geopolitics
  - tech
  - finance
  - crypto
  - world
  - politics
  - sports
  - ai
```

**Expected events**: ~150-200 per cycle

---

## How to Verify Category Filtering

Run the service with debug logging:

```bash
./bin/polyoracle --config configs/config.yaml
```

Look for:
```
[DEBUG] Fetching events from Polymarket API (categories: [geopolitics tech finance world], limit: 200)
[INFO] Fetched X events from 4 categories
```

If you see 0 events fetched, check:
1. Category slugs are correct (use this document as reference)
2. Volume thresholds are not too restrictive
3. API is accessible

---

## API Category Structure

The Polymarket API returns categories in the **`tags` array**:

```json
{
  "id": "12345",
  "title": "Event title",
  "category": null,           // ⚠️ IGNORE - always null
  "categories": null,         // ⚠️ IGNORE - always null
  "tags": [                   // ✅ USE THIS
    {
      "id": "1",
      "label": "Politics",
      "slug": "politics"      // ✅ Match against this
    }
  ]
}
```

**Important**: Always filter by `tags[].slug`, not by `category` or `categories` fields.

---

## Validation Commands

### Check if categories exist in API

```bash
curl -s "https://gamma-api.polymarket.com/events?active=true&closed=false&limit=100" | \
  jq -r '.[].tags[].slug' | sort | uniq -c | sort -rn | head -20
```

### Test specific category

```bash
# Test if "geopolitics" category exists
curl -s "https://gamma-api.polymarket.com/events?active=true&closed=false&limit=50" | \
  jq -r '.[].tags[].slug' | grep -c "^geopolitics$"
```

---

## Summary

✅ **Category filtering is working correctly**
✅ **All categories in example config are valid**
✅ **Implementation matches tags against configured categories**
✅ **OR logic allows events with multiple tags to be included**

For production use, the current categories (geopolitics, tech, finance, world) provide good coverage of high-value events without overwhelming volume.
