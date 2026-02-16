# Data Model: Event Probability Monitor

**Date**: 2026-02-16
**Purpose**: Define domain entities, relationships, and validation rules

## Overview

This document defines the core data entities for the Event Probability Monitor service. The model is designed for simplicity, clarity, and efficient in-memory operations with file-based persistence.

---

## Core Entities

### 1. Event

Represents a prediction market event being monitored from Polymarket.

**Fields**:
- `ID` (string, required): Unique identifier from Polymarket
- `Title` (string, required): Event title/question
- `Description` (string, optional): Detailed description
- `Category` (string, required): Event category (politics, sports, crypto, etc.)
- `Subcategory` (string, optional): Event subcategory
- `YesProbability` (float64, required): Current "Yes" probability (0.0-1.0) - maximum across all markets
- `NoProbability` (float64, required): Current "No" probability (0.0-1.0) - maximum across all markets
- `Volume24hr` (float64, required): 24-hour trading volume
- `Volume1wk` (float64, required): 1-week trading volume
- `Volume1mo` (float64, required): 1-month trading volume
- `Liquidity` (float64, required): Current liquidity
- `Active` (bool, required): Whether event is still active
- `Closed` (bool, required): Whether event is closed
- `LastUpdated` (time.Time, required): Last update timestamp
- `CreatedAt` (time.Time, required): When event was first tracked

**Multi-Market Handling**:
Per Polymarket API, one event can have multiple markets. The service must:
1. For each event, examine all associated markets
2. Calculate probability changes for each market independently
3. Select the **maximum change magnitude** across all markets for that event
4. Use that maximum change for ranking and notification purposes

**Validation Rules**:
- `ID` must not be empty
- `Title` must not be empty
- `Category` must be in configured allowed categories
- `YesProbability` must be between 0.0 and 1.0
- `NoProbability` must be between 0.0 and 1.0
- `YesProbability + NoProbability` should approximately equal 1.0 (within tolerance)
- `Volume24hr`, `Volume1wk`, `Volume1mo` must be >= 0
- `Liquidity` must be >= 0
- `LastUpdated` must be <= current time
- `CreatedAt` must be <= `LastUpdated`

**Relationships**:
- One Event has many Snapshots (1:N)
- One Event has many Changes (1:N)

---

### 2. Snapshot

Represents a point-in-time probability reading for an event.

**Fields**:
- `ID` (string, required): Unique snapshot identifier (UUID)
- `EventID` (string, required): Reference to parent Event
- `YesProbability` (float64, required): "Yes" probability at snapshot time
- `NoProbability` (float64, required): "No" probability at snapshot time
- `Timestamp` (time.Time, required): When snapshot was taken
- `Source` (string, required): Data source (e.g., "polymarket-gamma-api")

**Validation Rules**:
- `EventID` must reference existing Event
- `YesProbability` must be between 0.0 and 1.0
- `NoProbability` must be between 0.0 and 1.0
- `Timestamp` must not be in the future
- `Source` must not be empty

**Relationships**:
- Many Snapshots belong to one Event (N:1)

**Storage Considerations**:
- Snapshots are immutable (never updated, only created)
- Rotation: Keep only last N snapshots per event (configurable, default 100)
- Oldest snapshots removed during rotation

---

### 3. Change

Represents a detected significant probability change for an event.

**Fields**:
- `ID` (string, required): Unique change identifier (UUID)
- `EventID` (string, required): Reference to Event
- `EventQuestion` (string, required): Denormalized event question for notifications
- `Magnitude` (float64, required): Absolute change magnitude (0.0-1.0)
- `Direction` (string, required): "increase" or "decrease"
- `OldProbability` (float64, required): Previous probability value
- `NewProbability` (float64, required): Current probability value
- `TimeWindow` (time.Duration, required): Duration over which change was measured
- `DetectedAt` (time.Time, required): When change was detected
- `Notified` (bool, required): Whether notification was sent for this change

**Validation Rules**:
- `EventID` must reference existing Event
- `Magnitude` must be >= configured threshold
- `Magnitude` must equal `|NewProbability - OldProbability|`
- `Direction` must be "increase" or "decrease"
- `OldProbability` and `NewProbability` must be between 0.0 and 1.0
- `DetectedAt` must not be in the future

**Relationships**:
- Many Changes belong to one Event (N:1)

**Business Logic**:
- Changes are ephemeral (regenerated each monitoring cycle)
- Only top K changes by magnitude trigger notifications
- Changes are sorted by magnitude descending for ranking

---

### 4. Config

Represents the user configuration for monitoring behavior.

**Fields**:
- `Polymarket` (PolymarketConfig, required): Polymarket API settings
- `Monitor` (MonitorConfig, required): Monitoring parameters
- `Telegram` (TelegramConfig, required): Telegram notification settings
- `Storage` (StorageConfig, required): Storage and persistence settings
- `Logging` (LoggingConfig, required): Logging configuration

#### PolymarketConfig
- `GammaAPIURL` (string, required): Base URL for Polymarket Gamma API (default: "https://gamma-api.polymarket.com")
- `CLOBAPIURL` (string, required): Base URL for Polymarket CLOB API (default: "https://clob.polymarket.com")
- `PollInterval` (time.Duration, required): How often to poll for updates
- `Categories` ([]string, required): Event categories/tags to monitor (e.g., ["politics", "sports", "crypto"])
- `Volume24hrMin` (float64, optional): Minimum 24-hour volume threshold (default: 0 = no filter)
- `Volume1wkMin` (float64, optional): Minimum 1-week volume threshold (default: 0 = no filter)
- `Volume1moMin` (float64, optional): Minimum 1-month volume threshold (default: 0 = no filter)
- `VolumeFilterLogic` (string, required): "or" for union (any condition) or "and" for intersection (all conditions) (default: "or")
- `Limit` (int, required): Maximum number of events to fetch from API (default: 100)
- `Timeout` (time.Duration, optional): API request timeout (default: 30s)

#### MonitorConfig
- `Threshold` (float64, required): Minimum change magnitude to trigger (0.0-1.0)
- `Window` (time.Duration, required): Time window for change detection
- `TopK` (int, required): Number of top events to include in notification
- `Enabled` (bool, optional): Enable/disable monitoring (default: true)

#### TelegramConfig
- `BotToken` (string, required): Telegram bot token
- `ChatID` (string, required): Telegram chat ID for notifications
- `Enabled` (bool, required): Enable/disable Telegram notifications

#### StorageConfig
- `MaxEvents` (int, required): Maximum events to track
- `MaxSnapshotsPerEvent` (int, required): Maximum snapshots per event before rotation
- `MaxFileSizeMB` (int, required): Maximum persistence file size in MB
- `PersistenceInterval` (time.Duration, required): How often to save to disk
- `FilePath` (string, required): Path to persistence file
- `DataDir` (string, optional): Directory for data storage (default: "./data")

#### LoggingConfig
- `Level` (string, required): Log level (debug, info, warn, error)
- `Format` (string, required): Log format (json, text)

**Validation Rules**:
- `Polymarket.GammaAPIURL` must be valid URL
- `Polymarket.CLOBAPIURL` must be valid URL
- `Polymarket.PollInterval` must be >= 1 minute
- `Polymarket.Categories` must contain at least one category
- `Polymarket.Volume24hrMin` must be >= 0
- `Polymarket.Volume1wkMin` must be >= 0
- `Polymarket.Volume1moMin` must be >= 0
- `Polymarket.VolumeFilterLogic` must be "or" or "and"
- `Polymarket.Limit` must be >= 1 and <= 1000
- `Monitor.Threshold` must be between 0.0 and 1.0
- `Monitor.Window` must be >= 1 minute
- `Monitor.TopK` must be >= 1
- `Telegram.BotToken` must not be empty if `Telegram.Enabled` is true
- `Telegram.ChatID` must not be empty if `Telegram.Enabled` is true
- `Storage.MaxEvents` must be >= 1
- `Storage.MaxSnapshotsPerEvent` must be >= 10
- `Storage.MaxFileSizeMB` must be >= 1
- `Storage.PersistenceInterval` must be >= 1 minute
- `Storage.FilePath` must be valid file path
- `Logging.Level` must be one of: debug, info, warn, error
- `Logging.Format` must be one of: json, text

---

## Data Flow

### 1. Initialization
```
Load Config → Validate Config → Initialize Storage → Load persisted data
```

### 2. Monitoring Cycle
```
1. Poll Polymarket API for events in configured categories
2. Update Event entities with latest data
3. Create new Snapshot for each event
4. Apply storage rotation (remove old snapshots)
5. Detect significant changes using algorithm
6. Rank changes by magnitude
7. Select top K changes
8. Send Telegram notifications
9. Persist state to disk
```

### 3. Change Detection Algorithm
```
For each monitored Event:
  1. Retrieve snapshots within time window
  2. If < 2 snapshots, skip (insufficient data)
  3. Calculate: current_prob - oldest_prob_in_window
  4. If |change| >= threshold:
     - Create Change entity
     - Add to changes list
Sort changes by magnitude (descending)
Return top K changes
```

---

## Storage Schema

### In-Memory Structure
```go
type Storage struct {
    events    map[string]*Event       // event_id -> Event
    snapshots map[string][]Snapshot   // event_id -> []Snapshot
    changes   []Change                // Recent detected changes (ephemeral)
    config    *Config                 // Configuration reference

    mu        sync.RWMutex            // Thread-safe access

    // Rotation settings
    maxEvents          int
    maxSnapshotsPerEvent int
}

// Indexes for efficient queries
// events map provides O(1) lookup by ID
// snapshots map provides O(1) lookup by event ID
// No database indexes needed for in-memory operations
```

### File-Based Persistence Schema
```json
{
  "version": "1.0",
  "saved_at": "2026-02-16T10:30:00Z",
  "events": {
    "event-123": {
      "id": "event-123",
      "question": "Will X happen?",
      "category": "politics",
      "yes_probability": 0.75,
      "no_probability": 0.25,
      "active": true,
      "last_updated": "2026-02-16T10:25:00Z",
      "created_at": "2026-02-10T08:00:00Z"
    }
  },
  "snapshots": {
    "event-123": [
      {
        "id": "snap-456",
        "event_id": "event-123",
        "yes_probability": 0.75,
        "no_probability": 0.25,
        "timestamp": "2026-02-16T10:25:00Z",
        "source": "polymarket-gamma-api"
      }
    ]
  }
}
```

---

## State Transitions

### Event Lifecycle
```
[Discovered] → [Active] → [Inactive]
     ↓            ↓           ↓
  (first seen)  (monitoring)  (event resolved/expired)
```

### Snapshot Lifecycle
```
[Created] → [Stored] → [Rotated Out]
     ↓          ↓            ↓
  (API poll)  (in memory)  (oldest removed)
```

### Change Lifecycle
```
[Detected] → [Notified] → [Cleared]
     ↓           ↓            ↓
  (algorithm)  (sent)      (next cycle, regenerated)
```

---

## Query Patterns

### Common Queries

1. **Get all events in category**:
   - Filter `events` map by `Category` field
   - O(N) where N = number of events

2. **Get snapshots for event within time window**:
   - Lookup `snapshots[event_id]`
   - Filter by `Timestamp`
   - O(M) where M = snapshots per event

3. **Get top K changes**:
   - Already sorted during detection
   - Return first K elements
   - O(1)

4. **Find event by ID**:
   - Direct map lookup `events[id]`
   - O(1)

---

## Performance Characteristics

| Operation | Time Complexity | Space Complexity |
|-----------|----------------|------------------|
| Poll events | O(N) | O(N) |
| Create snapshot | O(1) | O(1) |
| Detect changes | O(N * M) | O(N) |
| Rotate snapshots | O(M) per event | O(1) |
| Persist to disk | O(N + total_snapshots) | O(N + total_snapshots) |
| Load from disk | O(N + total_snapshots) | O(N + total_snapshots) |

Where:
- N = number of monitored events
- M = max snapshots per event

**Target**: N = 1000, M = 100
**Memory estimate**: ~10-20 MB for full dataset

---

## Entity Relationships Diagram

```
┌─────────────┐
│    Event    │
│─────────────│
│ ID          │───┐
│ Question    │   │
│ Category    │   │
│ Active      │   │
└─────────────┘   │
                  │ 1:N
                  │
                  ▼
            ┌─────────────┐
            │  Snapshot   │
            │─────────────│
            │ EventID     │───┐
            │ Timestamp   │   │
            │ Probability │   │
            └─────────────┘   │
                              │ N:1
                              │
                              │
        ┌─────────────────────┘
        │
        ▼
┌─────────────┐
│   Change    │
│─────────────│
│ EventID     │
│ Magnitude   │
│ Direction   │
│ DetectedAt  │
└─────────────┘
```

---

## Next Steps

1. Implement Go structs for each entity
2. Add validation methods
3. Implement storage layer with rotation logic
4. Create JSON serialization for persistence
5. Write unit tests for entity validation and state transitions
