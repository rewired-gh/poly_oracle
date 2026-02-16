# Research: Event Probability Monitor

**Date**: 2026-02-16
**Purpose**: Resolve technical unknowns and document dependency selections

## Overview

This document consolidates research findings for key technical decisions in the Event Probability Monitor service. All decisions align with constitutional principles: simplicity, maintainability, minimal robust dependencies, and good code taste.

---

## 1. Telegram Bot Library

### Decision
**github.com/go-telegram-bot-api/telegram-bot-api** (v5.5.0+)

### Rationale
- **Maturity**: Most widely adopted Go Telegram library (14k+ GitHub stars)
- **Simplicity**: Straightforward API that aligns with "simple and maintainable" principle
- **Maintenance**: Actively maintained with regular updates
- **Documentation**: Comprehensive examples and good community support
- **Standard approach**: Clean, idiomatic Go code without unnecessary abstractions

### Alternatives Considered
- **github.com/go-telegram/bot**: Newer, more modern API but smaller community and less battle-tested
- **github.com/PaulSonOfLars/gotgbot/v2**: Good alternative but adds complexity we don't need for simple message sending

### Usage Pattern
```go
bot, err := tgbotapi.NewBotAPI("token")
if err != nil {
    log.Fatal(err)
}

msg := tgbotapi.NewMessage(chatID, "message text")
_, err = bot.Send(msg)
```

### References
- [Telegram Bot API Documentation](https://core.telegram.org/bots/api)
- [go-telegram-bot-api GitHub](https://github.com/go-telegram-bot-api/telegram-bot-api)

---

## 2. HTTP Client

### Decision
**Standard library `net/http`** with custom retry wrapper

### Rationale
- **Simplicity Principle**: Constitution mandates "use standard library preferentially"
- **Adequate for use case**: Polling Polymarket API doesn't require advanced HTTP client features
- **Control**: Custom retry logic is simple to implement and understand
- **No unnecessary dependency**: External HTTP clients (Resty, etc.) add complexity we don't need

### Implementation Pattern
```go
type RetryableClient struct {
    client    *http.Client
    maxRetries int
    backoff   time.Duration
}

func (c *RetryableClient) Do(req *http.Request) (*http.Response, error) {
    var lastErr error
    for i := 0; i < c.maxRetries; i++ {
        resp, err := c.client.Do(req)
        if err == nil && resp.StatusCode < 500 {
            return resp, nil
        }
        lastErr = err
        time.Sleep(c.backoff * time.Duration(i+1))
    }
    return nil, lastErr
}
```

### Alternatives Considered
- **github.com/go-resty/resty**: Excellent library but adds unnecessary dependency for our simple polling needs
- **github.com/hashicorp/go-retryablehttp**: Good for retry logic but we can implement simpler version ourselves

---

## 3. JSON Parsing

### Decision
**Standard library `encoding/json`**

### Rationale
- **Simplicity**: Constitution mandates minimal dependencies
- **Adequate performance**: For ~1000 events, standard library performance is sufficient
- **Compatibility**: Works with all Go types without code generation
- **Maintainability**: Standard library is well-documented and understood by all Go developers

### Performance Consideration
For monitoring 1000 events every 5 minutes (12 requests/hour), JSON parsing performance is not a bottleneck. If performance becomes critical later, can switch to Sonic or jsoniter without API changes.

### Alternatives Considered
- **github.com/bytedance/sonic**: Faster but adds dependency for minimal benefit at our scale
- **github.com/json-iterator/go**: Good alternative but standard library is sufficient

---

## 4. Storage Strategy

### Decision
**In-memory with periodic file-based persistence**

### Rationale
- **Lightweight**: Perfect for single-user mode on lightweight VPS
- **Simple**: No database setup, migration, or maintenance overhead
- **Fast**: In-memory access for frequent operations
- **Persistence**: File-based snapshots for crash recovery
- **Rotation**: Built-in size management as specified in requirements

### Architecture
```go
type Storage struct {
    events     map[string]*Event        // In-memory event data
    snapshots  map[string][]Snapshot    // Historical probability data
    mu         sync.RWMutex
    maxEvents  int                      // Maximum events to track
    maxSnapshots int                    // Maximum snapshots per event
    filePath   string                   // Persistence file path
}

// Periodic save to disk (every N minutes)
func (s *Storage) Save() error {
    s.mu.RLock()
    defer s.mu.RUnlock()
    // Marshal to JSON and write to file with rotation
}

// Load from disk on startup
func (s *Storage) Load() error {
    // Read file and unmarshal
}
```

### Rotation Strategy
- Keep last N snapshots per event (configurable, default 100)
- When limit exceeded, remove oldest snapshots
- File size limit: configurable max MB
- When file exceeds limit, compact by removing oldest data

### Alternatives Considered
- **SQLite**: Adds database complexity and dependency, unnecessary for single-user
- **PostgreSQL**: Massive overkill for single-user mode
- **BoltDB/Badger**: Embedded databases but still add complexity for simple use case

### Data Flow
1. **Polling**: Fetch data from Polymarket API → In-memory Storage
2. **Change Detection**: Read from in-memory → Calculate changes → In-memory
3. **Persistence**: Background goroutine saves to file every 5 minutes
4. **Recovery**: On startup, load from file if exists
5. **Rotation**: Automatically prune old data when limits exceeded

---

## 5. Polymarket API Integration

### Decision
**Gamma API + CLOB API** for comprehensive data access

### Endpoints
- **Gamma API** (`https://gamma-api.polymarket.com`): Event discovery, metadata, categories
- **CLOB API** (`https://clob.polymarket.com`): Real-time prices and orderbook data

### Base URLs
- Gamma API: `https://gamma-api.polymarket.com`
- CLOB API: `https://clob.polymarket.com`

### Authentication
- **None required** for public data endpoints (Gamma API events, CLOB API prices)

### Data Structure (Real API - Research-based)

#### Events Endpoint
```
GET https://gamma-api.polymarket.com/events
```

**Query Parameters:**
- `active` - boolean filter (true/false)
- `closed` - boolean filter (true/false)
- `order` - field to sort by (e.g., `volume24hr`, `volume1wk`, `volume1mo`, `liquidity`)
- `ascending` - boolean for sort direction (false = descending)
- `limit` - maximum number of results
- `offset` - pagination offset
- `tag_id` - filter by tag ID (integer)
- `tag_slug` - filter by tag slug (string)
- `liquidity_min` / `liquidity_max` - numeric range
- `volume_min` / `volume_max` - numeric range

**Response Format:** JSON array of Event objects (direct array, NOT wrapped)

**Event Fields:**
```go
type PolymarketEvent struct {
    ID           string    `json:"id"`
    Ticker       string    `json:"ticker"`
    Slug         string    `json:"slug"`
    Title        string    `json:"title"`        // Event title/question
    Subtitle     string    `json:"subtitle"`
    Description  string    `json:"description"`
    Category     string    `json:"category"`
    Subcategory  string    `json:"subcategory"`
    Active       bool      `json:"active"`
    Closed       bool      `json:"closed"`
    Archived     bool      `json:"archived"`
    Featured     bool      `json:"featured"`

    // Volume metrics
    Volume       float64   `json:"volume"`       // Total volume
    Volume24hr   float64   `json:"volume24hr"`   // 24-hour volume
    Volume1wk    float64   `json:"volume1wk"`    // 1-week volume
    Volume1mo    float64   `json:"volume1mo"`    // 1-month volume
    Volume1yr    float64   `json:"volume1yr"`    // 1-year volume

    // Other metrics
    Liquidity    float64   `json:"liquidity"`
    OpenInterest float64   `json:"openInterest"`

    // Date fields
    StartDate    *time.Time `json:"startDate"`
    EndDate      *time.Time `json:"endDate"`
    ClosedTime   *time.Time `json:"closedTime"`

    // Nested market data
    Markets      []Market  `json:"markets"`      // One event can have multiple markets

    // Related entities
    Tags         []Tag     `json:"tags"`
    Categories   []Category `json:"categories"`
    Series       *Series   `json:"series"`
}

type Market struct {
    ID             string   `json:"id"`
    ConditionID    string   `json:"condition_id"`
    Question       string   `json:"question"`
    Slug           string   `json:"slug"`
    Outcomes       []string `json:"outcomes"`       // e.g., ["Yes", "No"]
    OutcomePrices  []string `json:"outcome_prices"` // Prices as strings, maps 1:1 with outcomes
    ClobTokenIds   []string `json:"clobTokenIds"`   // Token IDs for CLOB API queries
    Active         bool     `json:"active"`
    Closed         bool     `json:"closed"`
}
```

**Key Insight:** One event can have multiple markets. When calculating probability changes, we need to:
1. For each market within an event, calculate the change
2. Select the maximum change across all markets for that event
3. Use that maximum change for ranking and notification

#### CLOB API Endpoints

**Get Price:**
```
GET https://clob.polymarket.com/price?token_id={clobTokenId}&side=buy
```

**Get Orderbook:**
```
GET https://clob.polymarket.com/book?token_id={clobTokenId}
```

**Price Response:**
```json
{
  "price": "0.75"
}
```

**Price History:**
```
GET https://clob.polymarket.com/history?token_id={clobTokenId}&interval={interval}
```

### Filtering Strategy (Per Requirements)

The service must filter events in this specific order:

1. **Filter by Categories**
   - Use `tag_id` or `tag_slug` parameter
   - User can configure which categories/tags to monitor
   - Example: `?tag_slug=politics&active=true&closed=false`

2. **Top K by Volume (Logical OR)**
   - Fetch events and check volume metrics: `volume24hr` OR `volume1wk` OR `volume1mo`
   - **Logical OR**: Include event if ANY of the volume fields meets the threshold
   - This means: `volume24hr >= threshold OR volume1wk >= threshold OR volume1mo >= threshold`
   - Sort by the highest volume field for each event
   - User can configure volume thresholds for each field OR use top K limit
   - Example approach:
     - Fetch events sorted by one volume field (e.g., `?order=volume24hr&ascending=false&limit=200`)
     - Apply logical OR filter on client side: include if any volume field is significant
     - Take top K events that satisfy the OR condition

3. **Top K by Drastic Changes**
   - This is implemented in our change detection algorithm
   - After filtering by categories and volume, we monitor those events
   - Detect probability changes over time
   - Rank by magnitude of change
   - Select top K events with highest changes

### Example API Calls

**Fetch active politics events sorted by 24hr volume:**
```bash
curl "https://gamma-api.polymarket.com/events?tag_slug=politics&active=true&closed=false&order=volume24hr&ascending=false&limit=50"
```

**Get current price for a market:**
```bash
curl "https://clob.polymarket.com/price?token_id=12345&side=buy"
```

### Rate Limits
- Not explicitly documented
- Conservative approach: 1 request per second per endpoint
- Implement exponential backoff on rate limit errors (429 status)

### References
- [Polymarket Events API Documentation](https://docs.polymarket.com/api-reference/events)
- [Polymarket CLOB API Documentation](https://docs.polymarket.com/developers/CLOB)
- [Polymarket Quickstart Guide](https://docs.polymarket.com/quickstart/fetching-data)

---

## 6. Change Detection Algorithm

### Decision
**Simple threshold-based comparison with time-window filtering**

### Rationale
- **Simplicity**: "Smart but not over-complicated" per requirements
- **Effectiveness**: Threshold-based detection is proven and understandable
- **Configurability**: All parameters exposed via YAML configuration

### Algorithm
```go
func DetectSignificantChanges(
    events []Event,
    snapshots map[string][]Snapshot,
    threshold float64,
    window time.Duration,
) []Change {
    var changes []Change
    now := time.Now()

    for _, event := range events {
        eventSnapshots := snapshots[event.ID]

        // Filter snapshots within time window
        var recentSnapshots []Snapshot
        for _, s := range eventSnapshots {
            if now.Sub(s.Timestamp) <= window {
                recentSnapshots = append(recentSnapshots, s)
            }
        }

        if len(recentSnapshots) < 2 {
            continue
        }

        // Calculate change: |current - oldest_in_window|
        current := recentSnapshots[len(recentSnapshots)-1]
        oldest := recentSnapshots[0]
        change := math.Abs(current.Probability - oldest.Probability)

        if change >= threshold {
            changes = append(changes, Change{
                EventID:    event.ID,
                Magnitude:  change,
                Direction:  getDirection(current.Probability, oldest.Probability),
                OldValue:   oldest.Probability,
                NewValue:   current.Probability,
                Timestamp:  now,
            })
        }
    }

    // Sort by magnitude descending, return top k
    sort.Slice(changes, func(i, j int) bool {
        return changes[i].Magnitude > changes[j].Magnitude
    })

    return changes
}
```

### Why Not More Complex?
- No ML needed: Simple threshold works for "drastic change" detection
- Explainable: User can understand why notifications triggered
- Tunable: User adjusts threshold/window/k via config
- No over-engineering: Matches constitutional principle

---

## 7. Configuration Management

### Decision
**github.com/spf13/viper** with YAML support

### Configuration Structure
```yaml
# Configuration schema
polymarket:
  api_base_url: "https://gamma-api.polymarket.com"
  poll_interval: 5m
  categories:
    - politics
    - sports
    - crypto

monitor:
  threshold: 0.10  # 10% change threshold
  window: 1h       # Time window
  top_k: 10        # Number of events to notify

telegram:
  bot_token: "YOUR_BOT_TOKEN"
  chat_id: "YOUR_CHAT_ID"
  enabled: true

storage:
  max_events: 1000
  max_snapshots_per_event: 100
  max_file_size_mb: 100
  persistence_interval: 5m
  file_path: "./data/poly-oracle.json"

logging:
  level: "info"
  format: "json"
```

### Rationale
- Viper is industry standard for Go configuration
- Supports YAML, environment variables, defaults
- Hot-reload capability for config changes (bonus feature)
- Clean API with strong typing support

---

## Summary of Dependencies

| Dependency | Version | Purpose | Justification |
|------------|---------|---------|---------------|
| github.com/spf13/viper | v1.19.0+ | Configuration management | Industry standard, minimal API |
| github.com/go-telegram-bot-api/telegram-bot-api | v5.5.0+ | Telegram notifications | Most mature, widely adopted |
| Standard library (net/http) | - | HTTP client | Simplicity, adequate for needs |
| Standard library (encoding/json) | - | JSON parsing | Sufficient performance, no extra dependency |

**Total external dependencies: 2** (minimal as per constitution)

---

## Next Steps

With all NEEDS CLARIFICATION items resolved, proceed to Phase 1:
1. Generate `data-model.md` with entity definitions
2. Create API contracts in `/contracts/`
3. Generate `quickstart.md` for developer onboarding
4. Update agent context with technology choices
