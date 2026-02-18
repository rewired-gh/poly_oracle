package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/rewired-gh/polyoracle/internal/models"
)

func TestStorage_AddAndGetMarket(t *testing.T) {
	s := New(100, 50, "/tmp/test-storage.json", 0644, 0755)

	now := time.Now()
	market := &models.Market{
		ID:             "test-1:event-1",
		EventID:        "event-1",
		MarketID:       "event-1",
		Title:          "Test question?",
		Category:       "politics",
		YesProbability: 0.75,
		NoProbability:  0.25,
		Active:         true,
		LastUpdated:    now,
		CreatedAt:      now.Add(-1 * time.Hour), // Created before last updated
	}

	// Test AddMarket
	if err := s.AddMarket(market); err != nil {
		t.Fatalf("AddMarket failed: %v", err)
	}

	// Test GetMarket
	retrieved, err := s.GetMarket("test-1:event-1")
	if err != nil {
		t.Fatalf("GetMarket failed: %v", err)
	}

	if retrieved.ID != market.ID {
		t.Errorf("Expected ID %s, got %s", market.ID, retrieved.ID)
	}
}

func TestStorage_AddSnapshot(t *testing.T) {
	s := New(100, 50, "/tmp/test-storage.json", 0644, 0755)

	// Add market first
	now := time.Now()
	market := &models.Market{
		ID:             "event-1:market-1",
		EventID:        "event-1",
		MarketID:       "market-1",
		Title:          "Test?",
		Category:       "politics",
		YesProbability: 0.75,
		NoProbability:  0.25,
		Active:         true,
		LastUpdated:    now,
		CreatedAt:      now.Add(-1 * time.Hour),
	}
	if err := s.AddMarket(market); err != nil {
		t.Fatalf("Failed to add market: %v", err)
	}

	// Add snapshot
	snapshot := &models.Snapshot{
		ID:             "snap-1",
		EventID:        "event-1:market-1",
		YesProbability: 0.75,
		NoProbability:  0.25,
		Timestamp:      time.Now(),
		Source:         "test",
	}

	if err := s.AddSnapshot(snapshot); err != nil {
		t.Fatalf("AddSnapshot failed: %v", err)
	}

	// Get snapshots
	snapshots, err := s.GetSnapshots("event-1:market-1")
	if err != nil {
		t.Fatalf("GetSnapshots failed: %v", err)
	}

	if len(snapshots) != 1 {
		t.Errorf("Expected 1 snapshot, got %d", len(snapshots))
	}
}

func TestStorage_GetTopChanges(t *testing.T) {
	s := New(100, 50, "/tmp/test-storage.json", 0644, 0755)

	// Add multiple changes
	changes := []*models.Change{
		{
			ID:             "change-1",
			EventID:        "event-1",
			EventTitle:     "Test 1?",
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
			EventTitle:     "Test 2?",
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
			EventTitle:     "Test 3?",
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
	s := New(100, 3, "/tmp/test-storage.json", 0644, 0755) // Max 3 snapshots per market

	// Add market
	now := time.Now()
	market := &models.Market{
		ID:             "event-1:market-1",
		EventID:        "event-1",
		MarketID:       "market-1",
		Title:          "Test?",
		Category:       "politics",
		YesProbability: 0.75,
		NoProbability:  0.25,
		Active:         true,
		LastUpdated:    now,
		CreatedAt:      now.Add(-1 * time.Hour),
	}
	if err := s.AddMarket(market); err != nil {
		t.Fatalf("Failed to add market: %v", err)
	}

	// Add 5 snapshots with past timestamps
	for i := 0; i < 5; i++ {
		snapshot := &models.Snapshot{
			ID:             fmt.Sprintf("snap-%d", i),
			EventID:        "event-1:market-1",
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
	snapshots, _ := s.GetSnapshots("event-1:market-1")
	if len(snapshots) != 3 {
		t.Errorf("Expected 3 snapshots after rotation, got %d", len(snapshots))
	}
}

func TestStorage_EmptyFilePathUsesTmpDir(t *testing.T) {
	// Test that empty file path uses OS tmp directory
	s := New(100, 50, "", 0644, 0755) // Empty file path

	// Verify file path contains OS tmp directory and polyoracle subdirectory
	expectedSuffix := "polyoracle/data.json"
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
	tempFile := "/tmp/test-polyoracle-save.json"
	defer func() { _ = os.Remove(tempFile) }()

	s := New(100, 50, tempFile, 0644, 0755)

	// Add test data
	now := time.Now()
	market := &models.Market{
		ID:             "event-1:market-1",
		EventID:        "event-1",
		MarketID:       "market-1",
		Title:          "Test?",
		Category:       "politics",
		YesProbability: 0.75,
		NoProbability:  0.25,
		Active:         true,
		LastUpdated:    now,
		CreatedAt:      now.Add(-1 * time.Hour),
	}
	if err := s.AddMarket(market); err != nil {
		t.Fatalf("Failed to add market: %v", err)
	}

	// Save
	if err := s.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Create new storage and load
	s2 := New(100, 50, tempFile, 0644, 0755)
	if err := s2.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify data restored
	loaded, err := s2.GetMarket("event-1:market-1")
	if err != nil {
		t.Fatalf("GetMarket after load failed: %v", err)
	}

	if loaded.Title != "Test?" {
		t.Errorf("Expected question 'Test?', got '%s'", loaded.Title)
	}
}

func TestStorage_GetSnapshotsInWindow_Sorted(t *testing.T) {
	s := New(100, 50, "/tmp/test-storage.json", 0644, 0755)

	// Add market
	now := time.Now()
	market := &models.Market{
		ID:             "event-1:market-1",
		EventID:        "event-1",
		MarketID:       "market-1",
		Title:          "Test?",
		Category:       "politics",
		YesProbability: 0.75,
		NoProbability:  0.25,
		Active:         true,
		LastUpdated:    now,
		CreatedAt:      now.Add(-1 * time.Hour),
	}
	if err := s.AddMarket(market); err != nil {
		t.Fatalf("Failed to add market: %v", err)
	}

	// Add 5 snapshots OUT OF ORDER (intentionally not chronological)
	timestamps := []time.Time{
		now.Add(-10 * time.Minute), // 3rd oldest
		now.Add(-30 * time.Minute), // oldest
		now.Add(-5 * time.Minute),  // 4th oldest (most recent)
		now.Add(-20 * time.Minute), // 2nd oldest
		now.Add(-8 * time.Minute),  // 3rd oldest
	}

	for i, ts := range timestamps {
		snapshot := &models.Snapshot{
			ID:             fmt.Sprintf("snap-%d", i),
			EventID:        "event-1:market-1",
			YesProbability: float64(50+i*10) / 100.0,
			NoProbability:  float64(50-i*10) / 100.0,
			Timestamp:      ts,
			Source:         "test",
		}
		if err := s.AddSnapshot(snapshot); err != nil {
			t.Fatalf("Failed to add snapshot %d: %v", i, err)
		}
	}

	// Get snapshots in 1 hour window (should get all 5)
	snapshots, err := s.GetSnapshotsInWindow("event-1:market-1", time.Hour)
	if err != nil {
		t.Fatalf("GetSnapshotsInWindow failed: %v", err)
	}

	if len(snapshots) != 5 {
		t.Errorf("Expected 5 snapshots, got %d", len(snapshots))
	}

	// Verify snapshots are sorted by timestamp ascending (oldest first)
	for i := 0; i < len(snapshots)-1; i++ {
		if !snapshots[i].Timestamp.Before(snapshots[i+1].Timestamp) {
			t.Errorf("Snapshots not sorted: snapshot[%d] timestamp %v is not before snapshot[%d] timestamp %v",
				i, snapshots[i].Timestamp, i+1, snapshots[i+1].Timestamp)
		}
	}

	// Verify oldest is first, newest is last
	if snapshots[0].Timestamp.Unix() != now.Add(-30*time.Minute).Unix() {
		t.Errorf("First snapshot should be oldest (30 min ago), got %v", snapshots[0].Timestamp)
	}
	if snapshots[len(snapshots)-1].Timestamp.Unix() != now.Add(-5*time.Minute).Unix() {
		t.Errorf("Last snapshot should be newest (5 min ago), got %v", snapshots[len(snapshots)-1].Timestamp)
	}
}

func TestStorage_UpdateMarket(t *testing.T) {
	s := New(100, 50, "/tmp/test-storage.json", 0644, 0755)

	now := time.Now()
	market := &models.Market{
		ID:             "test-event:test-market",
		EventID:        "test-event",
		MarketID:       "test-market",
		Title:          "Original Title",
		Category:       "politics",
		YesProbability: 0.75,
		NoProbability:  0.25,
		Active:         true,
		LastUpdated:    now,
		CreatedAt:      now.Add(-1 * time.Hour),
	}
	if err := s.AddMarket(market); err != nil {
		t.Fatalf("AddMarket failed: %v", err)
	}

	// Update the market
	market.Title = "Updated Title"
	market.YesProbability = 0.80
	market.NoProbability = 0.20

	if err := s.UpdateMarket(market); err != nil {
		t.Errorf("UpdateMarket failed: %v", err)
	}

	// Verify update
	retrieved, err := s.GetMarket("test-event:test-market")
	if err != nil {
		t.Fatalf("GetMarket after update failed: %v", err)
	}
	if retrieved.Title != "Updated Title" {
		t.Errorf("Expected title 'Updated Title', got '%s'", retrieved.Title)
	}
	if retrieved.YesProbability != 0.80 {
		t.Errorf("Expected YesProbability 0.80, got %.2f", retrieved.YesProbability)
	}
}

func TestStorage_UpdateMarket_NotFound(t *testing.T) {
	s := New(100, 50, "/tmp/test-storage.json", 0644, 0755)

	now := time.Now()
	market := &models.Market{
		ID:             "nonexistent:market",
		EventID:        "nonexistent",
		MarketID:       "market",
		Title:          "Does Not Exist",
		Category:       "politics",
		YesProbability: 0.50,
		NoProbability:  0.50,
		Active:         true,
		LastUpdated:    now,
		CreatedAt:      now.Add(-1 * time.Hour),
	}

	err := s.UpdateMarket(market)
	if err == nil {
		t.Error("Expected error when updating nonexistent market, got nil")
	}
}

func TestStorage_GetAllMarkets(t *testing.T) {
	s := New(100, 50, "/tmp/test-storage.json", 0644, 0755)

	// Initially empty
	markets, err := s.GetAllMarkets()
	if err != nil {
		t.Fatalf("GetAllMarkets on empty storage failed: %v", err)
	}
	if len(markets) != 0 {
		t.Errorf("Expected 0 markets, got %d", len(markets))
	}

	// Add 3 markets
	now := time.Now()
	for i := 0; i < 3; i++ {
		market := &models.Market{
			ID:             fmt.Sprintf("event-%d:market-%d", i, i),
			EventID:        fmt.Sprintf("event-%d", i),
			MarketID:       fmt.Sprintf("market-%d", i),
			Title:          fmt.Sprintf("Test Market %d", i),
			Category:       "politics",
			YesProbability: 0.50,
			NoProbability:  0.50,
			Active:         true,
			LastUpdated:    now,
			CreatedAt:      now.Add(-1 * time.Hour),
		}
		if err := s.AddMarket(market); err != nil {
			t.Fatalf("AddMarket failed: %v", err)
		}
	}

	// GetAllMarkets should return all 3
	markets, err = s.GetAllMarkets()
	if err != nil {
		t.Fatalf("GetAllMarkets failed: %v", err)
	}
	if len(markets) != 3 {
		t.Errorf("Expected 3 markets, got %d", len(markets))
	}
}

func TestStorage_RotateMarkets(t *testing.T) {
	maxMarkets := 5
	s := New(maxMarkets, 50, "/tmp/test-storage.json", 0644, 0755)

	now := time.Now()

	// Add 10 markets with different timestamps (newer markets have higher index)
	// Use past timestamps to pass validation
	for i := 0; i < 10; i++ {
		market := &models.Market{
			ID:             fmt.Sprintf("event-%d:market-%d", i, i),
			EventID:        fmt.Sprintf("event-%d", i),
			MarketID:       fmt.Sprintf("market-%d", i),
			Title:          fmt.Sprintf("Test Market %d", i),
			Category:       "politics",
			YesProbability: 0.50,
			NoProbability:  0.50,
			Active:         true,
			LastUpdated:    now.Add(-time.Duration(10-i) * time.Second), // oldest=event-0, newest=event-9
			CreatedAt:      now.Add(-1 * time.Hour),
		}
		if err := s.AddMarket(market); err != nil {
			t.Fatalf("AddMarket failed for market-%d: %v", i, err)
		}
	}

	// Verify all 10 added
	markets, _ := s.GetAllMarkets()
	if len(markets) != 10 {
		t.Fatalf("Expected 10 markets before rotation, got %d", len(markets))
	}

	// Rotate
	if err := s.RotateMarkets(); err != nil {
		t.Errorf("RotateMarkets failed: %v", err)
	}

	// Should have only maxMarkets (5) remaining - the newest ones
	markets, err := s.GetAllMarkets()
	if err != nil {
		t.Fatalf("GetAllMarkets after rotation failed: %v", err)
	}
	if len(markets) != maxMarkets {
		t.Errorf("Expected %d markets after rotation, got %d", maxMarkets, len(markets))
	}

	// Verify oldest markets (0-4) were removed, newest (5-9) remain
	for _, market := range markets {
		var idx int
		if _, err := fmt.Sscanf(market.ID, "event-%d:market-%d", &idx, &idx); err != nil {
			t.Errorf("Failed to parse market ID %s: %v", market.ID, err)
			continue
		}
		if idx < 5 {
			t.Errorf("Old market %s should have been rotated out", market.ID)
		}
	}
}

func TestStorage_MigrateToCompositeIDs(t *testing.T) {
	tempFile := "/tmp/test-polyoracle-migrate.json"
	defer func() { _ = os.Remove(tempFile) }()

	// Create v1.0 format data file with old single-market ID format
	now := time.Now()
	v1Data := PersistenceFile{
		Version: "1.0",
		SavedAt: now,
		Markets: map[string]*models.Market{
			"event-123": { // Old format: just event ID
				ID:             "event-123",
				EventID:        "event-123",
				MarketID:       "market-456",
				Title:          "Test Market",
				Category:       "politics",
				YesProbability: 0.75,
				NoProbability:  0.25,
				Active:         true,
				LastUpdated:    now,
				CreatedAt:      now.Add(-1 * time.Hour),
			},
		},
		Snapshots: map[string][]models.Snapshot{
			"event-123": { // Old format: just event ID
				{
					ID:             "snap-1",
					EventID:        "event-123",
					YesProbability: 0.70,
					NoProbability:  0.30,
					Timestamp:      now.Add(-30 * time.Minute),
					Source:         "test",
				},
				{
					ID:             "snap-2",
					EventID:        "event-123",
					YesProbability: 0.75,
					NoProbability:  0.25,
					Timestamp:      now,
					Source:         "test",
				},
			},
		},
	}

	// Write v1.0 data to file
	jsonData, err := json.Marshal(v1Data)
	if err != nil {
		t.Fatalf("Failed to marshal v1 data: %v", err)
	}
	if err := os.WriteFile(tempFile, jsonData, 0644); err != nil {
		t.Fatalf("Failed to write v1 data file: %v", err)
	}

	// Create new storage and load (should trigger migration)
	s := New(100, 50, tempFile, 0644, 0755)
	if err := s.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify market was migrated to composite ID format
	compositeID := "event-123:market-456"

	// Old ID should not exist
	if _, err := s.GetMarket("event-123"); err == nil {
		t.Error("Old ID format 'event-123' should not exist after migration")
	}

	// New composite ID should exist
	migratedMarket, err := s.GetMarket(compositeID)
	if err != nil {
		t.Fatalf("Failed to get migrated market with composite ID: %v", err)
	}

	// Verify market data is preserved
	if migratedMarket.ID != compositeID {
		t.Errorf("Expected migrated ID '%s', got '%s'", compositeID, migratedMarket.ID)
	}
	if migratedMarket.EventID != "event-123" {
		t.Errorf("Expected EventID 'event-123', got '%s'", migratedMarket.EventID)
	}
	if migratedMarket.MarketID != "market-456" {
		t.Errorf("Expected MarketID 'market-456', got '%s'", migratedMarket.MarketID)
	}
	if migratedMarket.Title != "Test Market" {
		t.Errorf("Expected title 'Test Market', got '%s'", migratedMarket.Title)
	}

	// Verify snapshots were migrated
	snapshots, err := s.GetSnapshots(compositeID)
	if err != nil {
		t.Fatalf("Failed to get migrated snapshots: %v", err)
	}
	if len(snapshots) != 2 {
		t.Errorf("Expected 2 migrated snapshots, got %d", len(snapshots))
	}

	// Verify snapshot event IDs were updated
	for _, snap := range snapshots {
		if snap.EventID != compositeID {
			t.Errorf("Expected snapshot EventID '%s', got '%s'", compositeID, snap.EventID)
		}
	}

	// Save and verify it uses v2.0 format
	if err := s.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Read back and check version
	savedData, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read saved file: %v", err)
	}
	var v2Data PersistenceFile
	if err := json.Unmarshal(savedData, &v2Data); err != nil {
		t.Fatalf("Failed to unmarshal saved data: %v", err)
	}
	if v2Data.Version != "2.0" {
		t.Errorf("Expected version '2.0' after save, got '%s'", v2Data.Version)
	}
}

// Test backward compatibility aliases
func TestStorage_EventAliases(t *testing.T) {
	s := New(100, 50, "/tmp/test-aliases.json", 0644, 0755)

	now := time.Now()
	market := &models.Market{
		ID:             "test:event",
		EventID:        "test",
		MarketID:       "event",
		Title:          "Alias Test",
		Category:       "politics",
		YesProbability: 0.50,
		NoProbability:  0.50,
		Active:         true,
		LastUpdated:    now,
		CreatedAt:      now.Add(-1 * time.Hour),
	}

	// Test AddEvent alias
	if err := s.AddEvent(market); err != nil {
		t.Fatalf("AddEvent alias failed: %v", err)
	}

	// Test GetEvent alias
	retrieved, err := s.GetEvent("test:event")
	if err != nil {
		t.Fatalf("GetEvent alias failed: %v", err)
	}
	if retrieved.Title != "Alias Test" {
		t.Errorf("Expected title 'Alias Test', got '%s'", retrieved.Title)
	}

	// Test GetAllEvents alias
	all, err := s.GetAllEvents()
	if err != nil {
		t.Fatalf("GetAllEvents alias failed: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("Expected 1 market, got %d", len(all))
	}

	// Test UpdateEvent alias
	market.Title = "Updated Alias Test"
	if err := s.UpdateEvent(market); err != nil {
		t.Fatalf("UpdateEvent alias failed: %v", err)
	}

	// Test RotateEvents alias (no error expected)
	if err := s.RotateEvents(); err != nil {
		t.Fatalf("RotateEvents alias failed: %v", err)
	}
}
