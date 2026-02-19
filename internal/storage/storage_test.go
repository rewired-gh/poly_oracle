package storage

import (
	"fmt"
	"testing"
	"time"

	"github.com/rewired-gh/polyoracle/internal/models"
)

func newTestStorage(t *testing.T) *Storage {
	t.Helper()
	s, err := New(100, 50, ":memory:")
	if err != nil {
		t.Fatalf("failed to create test storage: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func testMarket(id, eventID, marketID string, lastUpdated time.Time) *models.Market {
	return &models.Market{
		ID:             id,
		EventID:        eventID,
		MarketID:       marketID,
		Title:          "Test Market",
		Category:       "politics",
		YesProbability: 0.75,
		NoProbability:  0.25,
		Active:         true,
		LastUpdated:    lastUpdated,
		CreatedAt:      lastUpdated.Add(-time.Hour),
	}
}

func TestStorage_AddAndGetMarket(t *testing.T) {
	s := newTestStorage(t)
	now := time.Now()
	m := testMarket("event-1:market-1", "event-1", "market-1", now)

	if err := s.AddMarket(m); err != nil {
		t.Fatalf("AddMarket: %v", err)
	}
	got, err := s.GetMarket("event-1:market-1")
	if err != nil {
		t.Fatalf("GetMarket: %v", err)
	}
	if got.ID != m.ID {
		t.Errorf("got ID %s, want %s", got.ID, m.ID)
	}
}

func TestStorage_GetMarket_NotFound(t *testing.T) {
	s := newTestStorage(t)
	if _, err := s.GetMarket("nonexistent"); err == nil {
		t.Error("expected error for missing market")
	}
}

func TestStorage_UpdateMarket(t *testing.T) {
	s := newTestStorage(t)
	now := time.Now()
	m := testMarket("e:m", "e", "m", now)
	if err := s.AddMarket(m); err != nil {
		t.Fatalf("AddMarket: %v", err)
	}
	m.Title = "Updated"
	m.YesProbability = 0.80
	m.NoProbability = 0.20
	if err := s.UpdateMarket(m); err != nil {
		t.Fatalf("UpdateMarket: %v", err)
	}
	got, _ := s.GetMarket("e:m")
	if got.Title != "Updated" {
		t.Errorf("title not updated: got %q", got.Title)
	}
	if got.YesProbability != 0.80 {
		t.Errorf("yes_prob not updated: got %f", got.YesProbability)
	}
}

func TestStorage_UpdateMarket_NotFound(t *testing.T) {
	s := newTestStorage(t)
	now := time.Now()
	m := testMarket("nonexistent:m", "nonexistent", "m", now)
	if err := s.UpdateMarket(m); err == nil {
		t.Error("expected error updating nonexistent market")
	}
}

func TestStorage_GetAllMarkets(t *testing.T) {
	s := newTestStorage(t)
	now := time.Now()
	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("e-%d:m-%d", i, i)
		if err := s.AddMarket(testMarket(id, fmt.Sprintf("e-%d", i), fmt.Sprintf("m-%d", i), now)); err != nil {
			t.Fatalf("AddMarket: %v", err)
		}
	}
	markets, err := s.GetAllMarkets()
	if err != nil {
		t.Fatalf("GetAllMarkets: %v", err)
	}
	if len(markets) != 3 {
		t.Errorf("got %d markets, want 3", len(markets))
	}
}

func TestStorage_AddSnapshot(t *testing.T) {
	s := newTestStorage(t)
	now := time.Now()
	if err := s.AddMarket(testMarket("e:m", "e", "m", now)); err != nil {
		t.Fatalf("AddMarket: %v", err)
	}
	snap := &models.Snapshot{
		ID:             "snap-1",
		EventID:        "e:m",
		YesProbability: 0.75,
		NoProbability:  0.25,
		Timestamp:      now.Add(-time.Minute),
		Source:         "test",
	}
	if err := s.AddSnapshot(snap); err != nil {
		t.Fatalf("AddSnapshot: %v", err)
	}
	snaps, err := s.GetSnapshots("e:m")
	if err != nil {
		t.Fatalf("GetSnapshots: %v", err)
	}
	if len(snaps) != 1 {
		t.Errorf("got %d snapshots, want 1", len(snaps))
	}
}

func TestStorage_GetSnapshotsInWindow(t *testing.T) {
	s := newTestStorage(t)
	now := time.Now()
	if err := s.AddMarket(testMarket("e:m", "e", "m", now)); err != nil {
		t.Fatalf("AddMarket: %v", err)
	}

	// Add snapshots at -40m, -20m, -10m, -5m (4 total, only 3 within 30m window)
	timestamps := []time.Duration{-40 * time.Minute, -20 * time.Minute, -10 * time.Minute, -5 * time.Minute}
	for i, d := range timestamps {
		snap := &models.Snapshot{
			ID:             fmt.Sprintf("s%d", i),
			EventID:        "e:m",
			YesProbability: 0.5,
			NoProbability:  0.5,
			Timestamp:      now.Add(d),
			Source:         "test",
		}
		if err := s.AddSnapshot(snap); err != nil {
			t.Fatalf("AddSnapshot: %v", err)
		}
	}

	snaps, err := s.GetSnapshotsInWindow("e:m", 30*time.Minute)
	if err != nil {
		t.Fatalf("GetSnapshotsInWindow: %v", err)
	}
	if len(snaps) != 3 {
		t.Errorf("got %d snapshots in 30m window, want 3", len(snaps))
	}
	// Must be sorted ascending
	for i := 1; i < len(snaps); i++ {
		if !snaps[i-1].Timestamp.Before(snaps[i].Timestamp) {
			t.Errorf("snapshots not sorted ascending at index %d", i)
		}
	}
}

func TestStorage_RotateSnapshots(t *testing.T) {
	s, err := New(100, 3, ":memory:")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Close()

	now := time.Now()
	if err := s.AddMarket(testMarket("e:m", "e", "m", now)); err != nil {
		t.Fatalf("AddMarket: %v", err)
	}
	// Add 5 snapshots in order (oldest first)
	for i := 0; i < 5; i++ {
		snap := &models.Snapshot{
			ID:             fmt.Sprintf("s%d", i),
			EventID:        "e:m",
			YesProbability: 0.5,
			NoProbability:  0.5,
			Timestamp:      now.Add(time.Duration(-5+i) * time.Minute),
			Source:         "test",
		}
		if err := s.AddSnapshot(snap); err != nil {
			t.Fatalf("AddSnapshot: %v", err)
		}
	}
	if err := s.RotateSnapshots(); err != nil {
		t.Fatalf("RotateSnapshots: %v", err)
	}
	snaps, _ := s.GetSnapshots("e:m")
	if len(snaps) != 3 {
		t.Errorf("got %d snapshots after rotation, want 3", len(snaps))
	}
	// The 3 newest should remain (s2, s3, s4 = -3m, -2m, -1m)
	// Oldest remaining timestamp should be -3m
	expected := now.Add(-3 * time.Minute)
	if snaps[0].Timestamp.Unix() != expected.Unix() {
		t.Errorf("oldest remaining snapshot: got %v, want ~%v", snaps[0].Timestamp, expected)
	}
}

func TestStorage_RotateSnapshots_ByTimestamp_NotInsertionOrder(t *testing.T) {
	// Insert snapshots OUT OF chronological order; rotation must keep newest by timestamp.
	s, err := New(100, 3, ":memory:")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Close()

	now := time.Now()
	if err := s.AddMarket(testMarket("e:m", "e", "m", now)); err != nil {
		t.Fatalf("AddMarket: %v", err)
	}
	// Insert in reverse chronological order (newest first)
	timestamps := []time.Duration{-1 * time.Minute, -5 * time.Minute, -3 * time.Minute, -10 * time.Minute, -2 * time.Minute}
	for i, d := range timestamps {
		snap := &models.Snapshot{
			ID:             fmt.Sprintf("s%d", i),
			EventID:        "e:m",
			YesProbability: 0.5,
			NoProbability:  0.5,
			Timestamp:      now.Add(d),
			Source:         "test",
		}
		if err := s.AddSnapshot(snap); err != nil {
			t.Fatalf("AddSnapshot: %v", err)
		}
	}
	if err := s.RotateSnapshots(); err != nil {
		t.Fatalf("RotateSnapshots: %v", err)
	}
	snaps, _ := s.GetSnapshots("e:m")
	if len(snaps) != 3 {
		t.Errorf("got %d snapshots, want 3", len(snaps))
	}
	// Newest 3: -1m, -2m, -3m; oldest remaining = -3m
	if snaps[0].Timestamp.Unix() != now.Add(-3*time.Minute).Unix() {
		t.Errorf("wrong oldest: %v", snaps[0].Timestamp)
	}
}

func TestStorage_RotateMarkets(t *testing.T) {
	s, err := New(5, 50, ":memory:")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Close()

	now := time.Now()
	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("e-%d:m-%d", i, i)
		m := testMarket(id, fmt.Sprintf("e-%d", i), fmt.Sprintf("m-%d", i), now.Add(-time.Duration(10-i)*time.Second))
		if err := s.AddMarket(m); err != nil {
			t.Fatalf("AddMarket %d: %v", i, err)
		}
	}
	if err := s.RotateMarkets(); err != nil {
		t.Fatalf("RotateMarkets: %v", err)
	}
	markets, _ := s.GetAllMarkets()
	if len(markets) != 5 {
		t.Errorf("got %d markets after rotation, want 5", len(markets))
	}
	// Newest 5 markets (indices 5-9) should remain
	ids := make(map[string]bool)
	for _, m := range markets {
		ids[m.ID] = true
	}
	for i := 0; i < 5; i++ {
		old := fmt.Sprintf("e-%d:m-%d", i, i)
		if ids[old] {
			t.Errorf("old market %s should have been rotated out", old)
		}
	}
}

func TestStorage_RotateMarkets_CascadesSnapshots(t *testing.T) {
	s, err := New(1, 50, ":memory:")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Close()

	now := time.Now()
	// Add market-0 first (older), then its snapshot, then market-1 which evicts m0.
	m0 := testMarket("e-0:m-0", "e-0", "m-0", now.Add(-2*time.Second))
	_ = s.AddMarket(m0)

	// Add snapshot for m0 while it still exists
	snap := &models.Snapshot{
		ID:             "snap-for-m0",
		EventID:        "e-0:m-0",
		YesProbability: 0.5,
		NoProbability:  0.5,
		Timestamp:      now.Add(-2 * time.Second),
		Source:         "test",
	}
	if err := s.AddSnapshot(snap); err != nil {
		t.Fatalf("AddSnapshot: %v", err)
	}

	// Adding a newer market triggers cap enforcement (max_events=1): m0 is evicted with its snapshot.
	m1 := testMarket("e-1:m-1", "e-1", "m-1", now.Add(-1*time.Second))
	_ = s.AddMarket(m1)

	// m0 should be gone along with its snapshot
	if _, err := s.GetMarket("e-0:m-0"); err == nil {
		t.Error("expected m0 to be deleted")
	}
	snaps, _ := s.GetSnapshots("e-0:m-0")
	if len(snaps) != 0 {
		t.Errorf("expected snapshots for deleted market to be gone, got %d", len(snaps))
	}
}

func TestStorage_GetTopChanges(t *testing.T) {
	s := newTestStorage(t)
	now := time.Now()

	changes := []*models.Change{
		{ID: "c1", EventID: "e1", EventTitle: "T1", Magnitude: 0.15, Direction: "increase",
			OldProbability: 0.60, NewProbability: 0.75, TimeWindow: time.Hour, DetectedAt: now},
		{ID: "c2", EventID: "e2", EventTitle: "T2", Magnitude: 0.25, Direction: "increase",
			OldProbability: 0.50, NewProbability: 0.75, TimeWindow: time.Hour, DetectedAt: now},
		{ID: "c3", EventID: "e3", EventTitle: "T3", Magnitude: 0.10, Direction: "decrease",
			OldProbability: 0.80, NewProbability: 0.70, TimeWindow: time.Hour, DetectedAt: now},
	}
	for _, c := range changes {
		if err := s.AddChange(c); err != nil {
			t.Fatalf("AddChange: %v", err)
		}
	}

	top, err := s.GetTopChanges(2)
	if err != nil {
		t.Fatalf("GetTopChanges: %v", err)
	}
	if len(top) != 2 {
		t.Fatalf("got %d changes, want 2", len(top))
	}
	if top[0].Magnitude < top[1].Magnitude {
		t.Error("changes not sorted by magnitude descending")
	}
	if top[0].Magnitude != 0.25 {
		t.Errorf("top magnitude: got %f, want 0.25", top[0].Magnitude)
	}
}

func TestStorage_ClearChanges(t *testing.T) {
	s := newTestStorage(t)
	now := time.Now()
	c := &models.Change{
		ID: "c1", EventID: "e1", EventTitle: "T", Magnitude: 0.10,
		Direction: "increase", OldProbability: 0.60, NewProbability: 0.70,
		TimeWindow: time.Hour, DetectedAt: now,
	}
	_ = s.AddChange(c)
	if err := s.ClearChanges(); err != nil {
		t.Fatalf("ClearChanges: %v", err)
	}
	top, _ := s.GetTopChanges(10)
	if len(top) != 0 {
		t.Errorf("expected 0 changes after clear, got %d", len(top))
	}
}

func TestStorage_AddMarket_EnforcesMaxEvents(t *testing.T) {
	// max_events=3: adding a 4th should evict the oldest.
	s, err := New(3, 50, ":memory:")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Close()

	now := time.Now()
	for i := 0; i < 4; i++ {
		id := fmt.Sprintf("e-%d:m-%d", i, i)
		m := testMarket(id, fmt.Sprintf("e-%d", i), fmt.Sprintf("m-%d", i), now.Add(-time.Duration(4-i)*time.Second))
		if err := s.AddMarket(m); err != nil {
			t.Fatalf("AddMarket %d: %v", i, err)
		}
	}
	markets, _ := s.GetAllMarkets()
	if len(markets) != 3 {
		t.Errorf("got %d markets, want 3 after cap enforcement", len(markets))
	}
	// Oldest market (e-0) should be gone
	if _, err := s.GetMarket("e-0:m-0"); err == nil {
		t.Error("oldest market e-0 should have been evicted")
	}
}

func TestStorage_SaveLoadNoOps(t *testing.T) {
	s := newTestStorage(t)
	if err := s.Save(); err != nil {
		t.Errorf("Save should be a no-op, got: %v", err)
	}
	if err := s.Load(); err != nil {
		t.Errorf("Load should be a no-op, got: %v", err)
	}
}

func TestStorage_DefaultPath(t *testing.T) {
	s, err := New(10, 10, "")
	if err != nil {
		t.Fatalf("New with empty path: %v", err)
	}
	defer s.Close()
}
