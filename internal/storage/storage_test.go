package storage

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/poly-oracle/internal/models"
)

func TestStorage_AddAndGetEvent(t *testing.T) {
	s := New(100, 50, "/tmp/test-storage.json")

	now := time.Now()
	event := &models.Event{
		ID:             "test-1",
		EventID:        "test-1",
		Title:          "Test question?",
		Category:       "politics",
		YesProbability: 0.75,
		NoProbability:  0.25,
		Active:         true,
		LastUpdated:    now,
		CreatedAt:      now.Add(-1 * time.Hour), // Created before last updated
	}

	// Test AddEvent
	if err := s.AddEvent(event); err != nil {
		t.Fatalf("AddEvent failed: %v", err)
	}

	// Test GetEvent
	retrieved, err := s.GetEvent("test-1")
	if err != nil {
		t.Fatalf("GetEvent failed: %v", err)
	}

	if retrieved.ID != event.ID {
		t.Errorf("Expected ID %s, got %s", event.ID, retrieved.ID)
	}
}

func TestStorage_AddSnapshot(t *testing.T) {
	s := New(100, 50, "/tmp/test-storage.json")

	// Add event first
	now := time.Now()
	event := &models.Event{
		ID:             "event-1",
		EventID:        "event-1",
		Title:          "Test?",
		Category:       "politics",
		YesProbability: 0.75,
		NoProbability:  0.25,
		Active:         true,
		LastUpdated:    now,
		CreatedAt:      now.Add(-1 * time.Hour),
	}
	if err := s.AddEvent(event); err != nil {
		t.Fatalf("Failed to add event: %v", err)
	}

	// Add snapshot
	snapshot := &models.Snapshot{
		ID:             "snap-1",
		EventID:        "event-1",
		YesProbability: 0.75,
		NoProbability:  0.25,
		Timestamp:      time.Now(),
		Source:         "test",
	}

	if err := s.AddSnapshot(snapshot); err != nil {
		t.Fatalf("AddSnapshot failed: %v", err)
	}

	// Get snapshots
	snapshots, err := s.GetSnapshots("event-1")
	if err != nil {
		t.Fatalf("GetSnapshots failed: %v", err)
	}

	if len(snapshots) != 1 {
		t.Errorf("Expected 1 snapshot, got %d", len(snapshots))
	}
}

func TestStorage_GetTopChanges(t *testing.T) {
	s := New(100, 50, "/tmp/test-storage.json")

	// Add multiple changes
	changes := []*models.Change{
		{
			ID:             "change-1",
			EventID:        "event-1",
			EventQuestion:  "Test 1?",
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
			EventQuestion:  "Test 2?",
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
			EventQuestion:  "Test 3?",
			Magnitude:      0.10,
			Direction:      "decrease",
			OldProbability: 0.80,
			NewProbability: 0.70,
			TimeWindow:     time.Hour,
			DetectedAt:     time.Now(),
		},
	}

	for _, change := range changes {
		if err := s.AddChange(change); err != nil {
			t.Errorf("Failed to add change: %v", err)
		}
	}

	// Get top 2 changes
	top, err := s.GetTopChanges(2)
	if err != nil {
		t.Fatalf("GetTopChanges failed: %v", err)
	}

	if len(top) != 2 {
		t.Errorf("Expected 2 changes, got %d", len(top))
	}

	// Verify sorted by magnitude descending
	if top[0].Magnitude < top[1].Magnitude {
		t.Error("Changes not sorted by magnitude descending")
	}

	// Top change should be 0.25
	if top[0].Magnitude != 0.25 {
		t.Errorf("Expected magnitude 0.25, got %f", top[0].Magnitude)
	}
}

func TestStorage_RotateSnapshots(t *testing.T) {
	s := New(100, 3, "/tmp/test-storage.json") // Max 3 snapshots per event

	// Add event
	now := time.Now()
	event := &models.Event{
		ID:             "event-1",
		EventID:        "event-1",
		Title:          "Test?",
		Category:       "politics",
		YesProbability: 0.75,
		NoProbability:  0.25,
		Active:         true,
		LastUpdated:    now,
		CreatedAt:      now.Add(-1 * time.Hour),
	}
	if err := s.AddEvent(event); err != nil {
		t.Fatalf("Failed to add event: %v", err)
	}

	// Add 5 snapshots with past timestamps
	for i := 0; i < 5; i++ {
		snapshot := &models.Snapshot{
			ID:             fmt.Sprintf("snap-%d", i),
			EventID:        "event-1",
			YesProbability: 0.75,
			NoProbability:  0.25,
			Timestamp:      now.Add(time.Duration(-5+i) * time.Minute), // Past timestamps
			Source:         "test",
		}
		if err := s.AddSnapshot(snapshot); err != nil {
			t.Fatalf("Failed to add snapshot %d: %v", i, err)
		}
	}

	// Rotate
	if err := s.RotateSnapshots(); err != nil {
		t.Errorf("Failed to rotate snapshots: %v", err)
	}

	// Should have only 3 snapshots
	snapshots, _ := s.GetSnapshots("event-1")
	if len(snapshots) != 3 {
		t.Errorf("Expected 3 snapshots after rotation, got %d", len(snapshots))
	}
}

func TestStorage_EmptyFilePathUsesTmpDir(t *testing.T) {
	// Test that empty file path uses OS tmp directory
	s := New(100, 50, "") // Empty file path

	// Verify file path contains OS tmp directory and poly-oracle subdirectory
	expectedSuffix := "poly-oracle/data.json"
	if s.filePath == "" {
		t.Error("File path should not be empty")
	}
	if len(s.filePath) < len(expectedSuffix) {
		t.Errorf("File path too short: %s", s.filePath)
	}
	if s.filePath[len(s.filePath)-len(expectedSuffix):] != expectedSuffix {
		t.Errorf("Expected file path to end with '%s', got '%s'", expectedSuffix, s.filePath)
	}
}

func TestStorage_SaveAndLoad(t *testing.T) {
	tempFile := "/tmp/test-poly-oracle-save.json"
	defer func() { _ = os.Remove(tempFile) }()

	s := New(100, 50, tempFile)

	// Add test data
	now := time.Now()
	event := &models.Event{
		ID:             "event-1",
		EventID:        "event-1",
		Title:          "Test?",
		Category:       "politics",
		YesProbability: 0.75,
		NoProbability:  0.25,
		Active:         true,
		LastUpdated:    now,
		CreatedAt:      now.Add(-1 * time.Hour),
	}
	if err := s.AddEvent(event); err != nil {
		t.Fatalf("Failed to add event: %v", err)
	}

	// Save
	if err := s.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Create new storage and load
	s2 := New(100, 50, tempFile)
	if err := s2.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify data restored
	loaded, err := s2.GetEvent("event-1")
	if err != nil {
		t.Fatalf("GetEvent after load failed: %v", err)
	}

	if loaded.Title != "Test?" {
		t.Errorf("Expected question 'Test?', got '%s'", loaded.Title)
	}
}
