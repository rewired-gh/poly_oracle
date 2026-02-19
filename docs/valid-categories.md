# Valid Polymarket Categories

**Last updated**: 2026-02-17
**Source**: Polymarket Gamma API (`https://gamma-api.polymarket.com/events`)

> Category filtering uses the `tags[].slug` field — NOT the `category` or `categories` API fields (those are frequently null).

---

## High-Value Categories (Recommended)

| Category | Frequency (per 100 events) | Description |
|----------|-----------------------------|-------------|
| **politics** | 26 | Political events, elections |
| **world** | 18 | Global events, world affairs |
| **geopolitics** | 15 | International relations |
| **crypto** | 8 | Cryptocurrency markets |
| **sports** | 4 | Sports events, championships |
| **soccer** | 3 | Soccer / football leagues |
| **ai** | 3 | Artificial intelligence |
| **global-elections** | 3 | International elections |
| **tech** | 2+ | Technology, AI, big tech |
| **finance** | 2+ | Financial markets, economy |
| **elections** | 2 | Election events |
| **world-elections** | 2 | World election events |

## Specific Event Categories

| Category | Description |
|----------|-------------|
| trump-presidency | Trump administration events |
| ukraine | Ukraine conflict |
| ukraine-map | Ukraine territorial changes |
| trump-zelenskyy | Trump-Zelenskyy relations |
| israel | Israel-related events |
| putin | Putin / Russia events |
| foreign-policy | Foreign policy decisions |
| middle-east | Middle East events |
| courts | Court decisions |
| pop-culture | Pop culture events |

## Crypto / Finance Sub-Categories

| Category | Description |
|----------|-------------|
| bitcoin | Bitcoin price / markets |
| ethereum | Ethereum price / markets |
| crypto-prices | General crypto prices |
| pre-market | Pre-market trading |
| doge | Dogecoin |
| stablecoins | Stablecoin markets |

## Tech Sub-Categories

| Category | Description |
|----------|-------------|
| ai | Artificial intelligence |
| openai | OpenAI specific |
| big-tech | Big tech companies |
| sam-altman | Sam Altman related |

---

## Category Filtering Behavior

The service uses **OR logic** across configured categories, and **OR logic** across an event's tags:

```
Event is included if: ANY event tag slug matches ANY configured category slug
```

Example:
- Event tags: `[politics, trump-presidency, world]`
- Config categories: `[geopolitics, tech, finance, world]`
- Result: **MATCH** (matches "world")

Events often carry multiple tags, so a single event can satisfy multiple category filters.

---

## API Category Structure

The `tags` array is the authoritative source — always filter by `tags[].slug`:

```json
{
  "id": "12345",
  "title": "Event title",
  "category": null,           // IGNORE — frequently null
  "categories": null,         // IGNORE — frequently null
  "tags": [                   // USE THIS
    {
      "id": "1",
      "label": "Politics",
      "slug": "politics"      // match against this
    }
  ]
}
```

---

## Validation Commands

List all slugs currently in the API (sorted by frequency):

```bash
curl -s "https://gamma-api.polymarket.com/events?active=true&closed=false&limit=100" | \
  jq -r '.[].tags[].slug' | sort | uniq -c | sort -rn | head -20
```

Check if a specific slug exists:

```bash
curl -s "https://gamma-api.polymarket.com/events?active=true&closed=false&limit=50" | \
  jq -r '.[].tags[].slug' | grep -c "^geopolitics$"
```
