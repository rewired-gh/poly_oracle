// Package monitor provides probability change detection functionality.
// It analyzes probability snapshots over time windows to identify significant
// changes that exceed configurable thresholds.
//
// The monitor uses a threshold-based algorithm to detect meaningful probability
// movements and ranks them by magnitude for notification purposes.
package monitor

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/poly-oracle/internal/models"
	"github.com/poly-oracle/internal/storage"
)

// Monitor handles event monitoring and change detection
type Monitor struct {
	storage *storage.Storage
}

// New creates a new Monitor instance
func New(s *storage.Storage) *Monitor {
	return &Monitor{
		storage: s,
	}
}

// DetectChanges identifies significant probability changes within a time window
func (m *Monitor) DetectChanges(events []models.Event, threshold float64, window time.Duration) ([]models.Change, error) {
	if threshold < 0 || threshold > 1 {
		return nil, fmt.Errorf("invalid threshold %.2f: must be between 0 and 1", threshold)
	}
	if window <= 0 {
		return nil, fmt.Errorf("invalid window %v: must be positive", window)
	}

	var changes []models.Change
	now := time.Now()

	for _, event := range events {
		// Get snapshots within the time window
		snapshots, err := m.storage.GetSnapshotsInWindow(event.ID, window)
		if err != nil {
			// Log error but continue processing other events
			continue
		}

		// Need at least 2 snapshots to detect change
		if len(snapshots) < 2 {
			continue
		}

		// Find oldest and current snapshots
		oldest := snapshots[0]
		current := snapshots[len(snapshots)-1]

		// Calculate change magnitude (using Yes probability)
		change := math.Abs(current.YesProbability - oldest.YesProbability)

		// Check if change exceeds threshold
		if change >= threshold {
			direction := "increase"
			if current.YesProbability < oldest.YesProbability {
				direction = "decrease"
			}

			changeRecord := models.Change{
				ID:             uuid.New().String(),
				EventID:        event.ID,
				EventQuestion:  event.Title,
				EventURL:       event.EventURL,
				MarketID:       event.MarketID,
				MarketQuestion: event.MarketQuestion,
				Magnitude:      change,
				Direction:      direction,
				OldProbability: oldest.YesProbability,
				NewProbability: current.YesProbability,
				TimeWindow:     window,
				DetectedAt:     now,
				Notified:       false,
			}

			changes = append(changes, changeRecord)
		}
	}

	return changes, nil
}

// RankChanges sorts changes by magnitude and returns top K
func (m *Monitor) RankChanges(changes []models.Change, k int) []models.Change {
	if k <= 0 {
		return []models.Change{}
	}
	if len(changes) == 0 {
		return []models.Change{}
	}

	// Sort by magnitude descending
	sorted := make([]models.Change, len(changes))
	copy(sorted, changes)

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Magnitude > sorted[j].Magnitude
	})

	// Return top K
	if k > len(sorted) {
		k = len(sorted)
	}

	return sorted[:k]
}
