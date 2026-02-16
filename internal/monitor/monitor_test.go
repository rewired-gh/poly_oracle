package monitor

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/poly-oracle/internal/models"
	"github.com/poly-oracle/internal/storage"
)

func TestDetectChanges(t *testing.T) {
	s := storage.New(100, 50, "/tmp/test-monitor.json")
	m := New(s)

	// Create test event
	now := time.Now()
	event := models.Event{
		ID:             "event-1",
		EventID:        "event-1",
		Title:          "Will X happen?",
		Category:       "politics",
		YesProbability: 0.75,
		NoProbability:  0.25,
		Active:         true,
		LastUpdated:    now,
		CreatedAt:      now.Add(-1 * time.Hour),
	}
	if err := s.AddEvent(&event); err != nil {
		t.Fatalf("Failed to add event: %v", err)
	}

	// Add snapshots with changing probabilities
	snapshots := []models.Snapshot{
		{
			ID:             uuid.New().String(),
			EventID:        "event-1",
			YesProbability: 0.60,
			NoProbability:  0.40,
			Timestamp:      now.Add(-1 * time.Hour),
			Source:         "test",
		},
		{
			ID:             uuid.New().String(),
			EventID:        "event-1",
			YesProbability: 0.75,
			NoProbability:  0.25,
			Timestamp:      now,
			Source:         "test",
		},
	}

	for _, snap := range snapshots {
		if err := s.AddSnapshot(&snap); err != nil {
			t.Fatalf("Failed to add snapshot: %v", err)
		}
	}

	// Detect changes
	events := []models.Event{event}
	changes, err := m.DetectChanges(events, 0.10, 2*time.Hour)
	if err != nil {
		t.Fatalf("DetectChanges failed: %v", err)
	}

	if len(changes) == 0 {
		t.Error("Expected at least 1 change, got 0")
		return
	}

	if changes[0].Magnitude < 0.149 || changes[0].Magnitude > 0.151 {
		t.Errorf("Expected magnitude 0.15, got %f", changes[0].Magnitude)
	}

	if changes[0].Direction != "increase" {
		t.Errorf("Expected direction 'increase', got '%s'", changes[0].Direction)
	}
}

func TestRankChanges(t *testing.T) {
	s := storage.New(100, 50, "/tmp/test-monitor-rank.json")
	m := New(s)

	changes := []models.Change{
		{
			ID:             "change-1",
			EventID:        "event-1",
			EventQuestion:  "Question 1",
			Magnitude:      0.15,
			Direction:      "increase",
			OldProbability: 0.60,
			NewProbability: 0.75,
			TimeWindow:     time.Hour,
			DetectedAt:     time.Now(),
		},
		{
			ID:             "change-2",
			EventID:        "event-2",
			EventQuestion:  "Question 2",
			Magnitude:      0.25,
			Direction:      "increase",
			OldProbability: 0.50,
			NewProbability: 0.75,
			TimeWindow:     time.Hour,
			DetectedAt:     time.Now(),
		},
		{
			ID:             "change-3",
			EventID:        "event-3",
			EventQuestion:  "Question 3",
			Magnitude:      0.10,
			Direction:      "decrease",
			OldProbability: 0.80,
			NewProbability: 0.70,
			TimeWindow:     time.Hour,
			DetectedAt:     time.Now(),
		},
	}

	// Get top 2
	top := m.RankChanges(changes, 2)

	if len(top) != 2 {
		t.Errorf("Expected 2 changes, got %d", len(top))
	}

	// Verify sorted by magnitude descending
	if top[0].Magnitude < top[1].Magnitude {
		t.Error("Changes not sorted by magnitude descending")
	}

	// Top should be 0.25
	if top[0].Magnitude != 0.25 {
		t.Errorf("Expected top magnitude 0.25, got %f", top[0].Magnitude)
	}

	// Second should be 0.15
	if top[1].Magnitude != 0.15 {
		t.Errorf("Expected second magnitude 0.15, got %f", top[1].Magnitude)
	}
}

func TestDetectChanges_BelowThreshold(t *testing.T) {
	s := storage.New(100, 50, "/tmp/test-threshold.json")
	m := New(s)

	now := time.Now()
	event := models.Event{
		ID:             "event-1",
		EventID:        "event-1",
		Title:          "Test?",
		Category:       "politics",
		YesProbability: 0.65,
		NoProbability:  0.35,
		Active:         true,
		LastUpdated:    now,
		CreatedAt:      now.Add(-1 * time.Hour),
	}
	if err := s.AddEvent(&event); err != nil {
		t.Fatalf("Failed to add event: %v", err)
	}

	// Add snapshots with small change (below threshold)
	snapshots := []models.Snapshot{
		{
			ID:             uuid.New().String(),
			EventID:        "event-1",
			YesProbability: 0.60,
			NoProbability:  0.40,
			Timestamp:      now.Add(-1 * time.Hour),
			Source:         "test",
		},
		{
			ID:             uuid.New().String(),
			EventID:        "event-1",
			YesProbability: 0.65,
			NoProbability:  0.35,
			Timestamp:      now,
			Source:         "test",
		},
	}

	for _, snap := range snapshots {
		if err := s.AddSnapshot(&snap); err != nil {
			t.Fatalf("Failed to add snapshot: %v", err)
		}
	}

	// Detect with 0.10 threshold (change is 0.05)
	events := []models.Event{event}
	changes, err := m.DetectChanges(events, 0.10, 2*time.Hour)
	if err != nil {
		t.Fatalf("DetectChanges failed: %v", err)
	}

	// Should not detect change below threshold
	if len(changes) != 0 {
		t.Errorf("Expected 0 changes (below threshold), got %d", len(changes))
	}
}
