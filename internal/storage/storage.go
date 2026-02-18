// Package storage provides thread-safe in-memory storage with file-based persistence.
// It manages markets, probability snapshots, and detected changes with automatic
// data rotation to prevent unbounded memory growth.
//
// Storage is designed for reliability with atomic file writes and graceful
// handling of persistence failures. Data is persisted to JSON files and can
// be restored on application restart.
package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/poly-oracle/internal/models"
)

// Storage provides thread-safe in-memory storage with file-based persistence
type Storage struct {
	markets   map[string]*models.Market
	snapshots map[string][]models.Snapshot
	changes   []models.Change
	mu        sync.RWMutex

	// Configuration
	maxMarkets           int
	maxSnapshotsPerEvent int
	filePath             string
	filePermissions      os.FileMode
	dirPermissions       os.FileMode
}

// PersistenceFile represents the file structure for JSON persistence
type PersistenceFile struct {
	Version   string                       `json:"version"`
	SavedAt   time.Time                    `json:"saved_at"`
	Markets   map[string]*models.Market    `json:"events"` // json tag kept as "events" for backwards compatibility
	Snapshots map[string][]models.Snapshot `json:"snapshots"`
}

// New creates a new Storage instance with persistence to tmp directory
// If filePath is empty, uses OS-appropriate tmp directory
func New(maxMarkets, maxSnapshotsPerEvent int, filePath string, filePermissions, dirPermissions os.FileMode) *Storage {
	// Use OS-appropriate tmp directory if no path provided
	if filePath == "" {
		filePath = filepath.Join(os.TempDir(), "poly-oracle", "data.json")
	}

	return &Storage{
		markets:              make(map[string]*models.Market),
		snapshots:            make(map[string][]models.Snapshot),
		changes:              make([]models.Change, 0),
		maxMarkets:           maxMarkets,
		maxSnapshotsPerEvent: maxSnapshotsPerEvent,
		filePath:             filePath,
		filePermissions:      filePermissions,
		dirPermissions:       dirPermissions,
	}
}

// AddMarket adds a market to storage
func (s *Storage) AddMarket(market *models.Market) error {
	if err := market.Validate(); err != nil {
		return fmt.Errorf("invalid market: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.markets[market.ID] = market
	return nil
}

// GetMarket retrieves a market by ID
func (s *Storage) GetMarket(id string) (*models.Market, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	market, exists := s.markets[id]
	if !exists {
		return nil, fmt.Errorf("market not found: %s", id)
	}
	return market, nil
}

// GetAllMarkets returns all markets
func (s *Storage) GetAllMarkets() ([]*models.Market, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	markets := make([]*models.Market, 0, len(s.markets))
	for _, market := range s.markets {
		markets = append(markets, market)
	}
	return markets, nil
}

// UpdateMarket updates an existing market
func (s *Storage) UpdateMarket(market *models.Market) error {
	if err := market.Validate(); err != nil {
		return fmt.Errorf("invalid market: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.markets[market.ID]; !exists {
		return fmt.Errorf("market not found: %s", market.ID)
	}

	s.markets[market.ID] = market
	return nil
}

// AddSnapshot adds a new snapshot for a market
func (s *Storage) AddSnapshot(snapshot *models.Snapshot) error {
	if err := snapshot.Validate(); err != nil {
		return fmt.Errorf("invalid snapshot: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Verify market exists
	if _, exists := s.markets[snapshot.EventID]; !exists {
		return fmt.Errorf("market not found: %s", snapshot.EventID)
	}

	s.snapshots[snapshot.EventID] = append(s.snapshots[snapshot.EventID], *snapshot)
	return nil
}

// GetSnapshots retrieves all snapshots for a market
func (s *Storage) GetSnapshots(marketID string) ([]models.Snapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshots, exists := s.snapshots[marketID]
	if !exists {
		return []models.Snapshot{}, nil
	}

	return snapshots, nil
}

// GetSnapshotsInWindow retrieves snapshots within a time window for a market
func (s *Storage) GetSnapshotsInWindow(marketID string, window time.Duration) ([]models.Snapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshots, exists := s.snapshots[marketID]
	if !exists {
		return []models.Snapshot{}, nil
	}

	now := time.Now()
	var filtered []models.Snapshot
	for _, snapshot := range snapshots {
		if now.Sub(snapshot.Timestamp) <= window {
			filtered = append(filtered, snapshot)
		}
	}

	// Sort by timestamp ascending (oldest first)
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Timestamp.Before(filtered[j].Timestamp)
	})

	return filtered, nil
}

// AddChange adds a detected change
func (s *Storage) AddChange(change *models.Change) error {
	if err := change.Validate(); err != nil {
		return fmt.Errorf("invalid change: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.changes = append(s.changes, *change)
	return nil
}

// GetTopChanges returns the top K changes sorted by magnitude
func (s *Storage) GetTopChanges(k int) ([]models.Change, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Sort changes by magnitude descending
	sorted := make([]models.Change, len(s.changes))
	copy(sorted, s.changes)

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Magnitude > sorted[j].Magnitude
	})

	// Return top K
	if k > len(sorted) {
		k = len(sorted)
	}
	return sorted[:k], nil
}

// ClearChanges removes all stored changes
func (s *Storage) ClearChanges() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.changes = make([]models.Change, 0)
	return nil
}

// Save persists storage state to file
func (s *Storage) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Create data directory if needed
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, s.dirPermissions); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Prepare persistence file
	data := PersistenceFile{
		Version:   "2.0",
		SavedAt:   time.Now(),
		Markets:   s.markets,
		Snapshots: s.snapshots,
	}

	// Marshal to JSON
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	// Write to temporary file first (atomic write)
	tempPath := s.filePath + ".tmp"
	if err := os.WriteFile(tempPath, jsonData, s.filePermissions); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Rename temp file to actual file
	if err := os.Rename(tempPath, s.filePath); err != nil {
		_ = os.Remove(tempPath) // Clean up temp file on rename failure
		return fmt.Errorf("failed to rename file: %w", err)
	}

	return nil
}

// Load restores storage state from file
func (s *Storage) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clean up any stale temp files from previous crashes
	tempPath := s.filePath + ".tmp"
	if _, err := os.Stat(tempPath); err == nil {
		_ = os.Remove(tempPath)
	}

	// Check if file exists
	if _, err := os.Stat(s.filePath); os.IsNotExist(err) {
		// No file to load, start fresh
		return nil
	}

	// Read file
	jsonData, err := os.ReadFile(s.filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Unmarshal
	var data PersistenceFile
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return fmt.Errorf("failed to unmarshal data: %w", err)
	}

	// Clear and restore state
	s.markets = data.Markets
	if s.markets == nil {
		s.markets = make(map[string]*models.Market)
	}

	s.snapshots = data.Snapshots
	if s.snapshots == nil {
		s.snapshots = make(map[string][]models.Snapshot)
	}

	// Changes are transient, clear them
	s.changes = make([]models.Change, 0)

	// Migrate if needed (version < 2.0)
	if data.Version == "" || data.Version == "1.0" {
		s.migrateToCompositeIDs()
	}

	return nil
}

// migrateToCompositeIDs migrates v1.0 data format to v2.0 composite ID format
// In v1.0, single-market events used just EventID, while multi-market used EventID:MarketID
// In v2.0, all markets use EventID:MarketID format for consistency
func (s *Storage) migrateToCompositeIDs() {
	newMarkets := make(map[string]*models.Market)
	newSnapshots := make(map[string][]models.Snapshot)

	for id, market := range s.markets {
		// Check if ID needs migration (doesn't contain ":" and has MarketID)
		if !strings.Contains(id, ":") && market.MarketID != "" {
			// Migrate to composite ID format
			newID := market.EventID + ":" + market.MarketID
			market.ID = newID
			newMarkets[newID] = market

			// Migrate associated snapshots
			if snaps, exists := s.snapshots[id]; exists {
				for i := range snaps {
					snaps[i].EventID = newID
				}
				newSnapshots[newID] = snaps
			}
		} else {
			// Already in correct format or no MarketID
			newMarkets[id] = market
			if snaps, exists := s.snapshots[id]; exists {
				newSnapshots[id] = snaps
			}
		}
	}

	s.markets = newMarkets
	s.snapshots = newSnapshots
}

// RotateSnapshots removes old snapshots exceeding max limit
func (s *Storage) RotateSnapshots() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for marketID, snapshots := range s.snapshots {
		if len(snapshots) > s.maxSnapshotsPerEvent {
			// Keep only the most recent snapshots
			start := len(snapshots) - s.maxSnapshotsPerEvent
			s.snapshots[marketID] = snapshots[start:]
		}
	}

	return nil
}

// RotateMarkets removes markets when exceeding max limit
func (s *Storage) RotateMarkets() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.markets) <= s.maxMarkets {
		return nil
	}

	// Find oldest markets to remove
	type marketWithTime struct {
		id          string
		lastUpdated time.Time
	}

	var marketList []marketWithTime
	for id, market := range s.markets {
		marketList = append(marketList, marketWithTime{id: id, lastUpdated: market.LastUpdated})
	}

	// Sort by last updated (oldest first)
	sort.Slice(marketList, func(i, j int) bool {
		return marketList[i].lastUpdated.Before(marketList[j].lastUpdated)
	})

	// Remove oldest markets
	toRemove := len(s.markets) - s.maxMarkets
	for i := 0; i < toRemove; i++ {
		marketID := marketList[i].id
		delete(s.markets, marketID)
		delete(s.snapshots, marketID)
	}

	return nil
}

// AddEvent is an alias for AddMarket for backward compatibility.
// Deprecated: Use AddMarket instead.
func (s *Storage) AddEvent(market *models.Market) error {
	return s.AddMarket(market)
}

// GetEvent is an alias for GetMarket for backward compatibility.
// Deprecated: Use GetMarket instead.
func (s *Storage) GetEvent(id string) (*models.Market, error) {
	return s.GetMarket(id)
}

// UpdateEvent is an alias for UpdateMarket for backward compatibility.
// Deprecated: Use UpdateMarket instead.
func (s *Storage) UpdateEvent(market *models.Market) error {
	return s.UpdateMarket(market)
}

// GetAllEvents is an alias for GetAllMarkets for backward compatibility.
// Deprecated: Use GetAllMarkets instead.
func (s *Storage) GetAllEvents() ([]*models.Market, error) {
	return s.GetAllMarkets()
}

// RotateEvents is an alias for RotateMarkets for backward compatibility.
// Deprecated: Use RotateMarkets instead.
func (s *Storage) RotateEvents() error {
	return s.RotateMarkets()
}
