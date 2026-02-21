package storage

import (
	"fmt"
	"testing"
	"time"

	"github.com/rewired-gh/polyoracle/internal/models"
)

func newTestStorage(t *testing.T) *Storage {
	t.Helper()
	s, err := New(100, ":memory:")
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

func TestStorage_RotateMarkets(t *testing.T) {
	s, err := New(5, ":memory:")
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

func TestStorage_AddMarket_EnforcesMaxEvents(t *testing.T) {
	// max_events=3: adding a 4th should evict the oldest.
	s, err := New(3, ":memory:")
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

func TestStorage_SaveLoadState(t *testing.T) {
	s := newTestStorage(t)

	// First add the market
	m := testMarket("test:market", "test", "market", time.Now())
	if err := s.AddMarket(m); err != nil {
		t.Fatalf("AddMarket: %v", err)
	}

	state := &models.MarketState{
		MarketID:     "test:market",
		WelfordCount: 10,
		WelfordMean:  0.55,
		WelfordM2:    0.025,
		AvgDepth:     50000,
		TCBuffer:     []float64{1.0, 2.0, 3.0},
		TCIndex:      1,
		LastPrice:    0.60,
		LastSigma:    0.05,
		LastVolume:   100000,
		UpdatedAt:    time.Now(),
	}

	if err := s.SaveState("test:market", state); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	loaded, err := s.LoadState("test:market")
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}

	if loaded.MarketID != state.MarketID {
		t.Errorf("market ID: got %s, want %s", loaded.MarketID, state.MarketID)
	}
	if loaded.WelfordCount != state.WelfordCount {
		t.Errorf("welford count: got %d, want %d", loaded.WelfordCount, state.WelfordCount)
	}
	if loaded.WelfordMean != state.WelfordMean {
		t.Errorf("welford mean: got %v, want %v", loaded.WelfordMean, state.WelfordMean)
	}
	if len(loaded.TCBuffer) != len(state.TCBuffer) {
		t.Errorf("TC buffer length: got %d, want %d", len(loaded.TCBuffer), len(state.TCBuffer))
	}
}

func TestStorage_LoadAllStates(t *testing.T) {
	s := newTestStorage(t)

	// Add markets first
	now := time.Now()
	markets := []*models.Market{
		testMarket("m1", "e1", "m1", now),
		testMarket("m2", "e2", "m2", now),
		testMarket("m3", "e3", "m3", now),
	}

	for _, m := range markets {
		if err := s.AddMarket(m); err != nil {
			t.Fatalf("AddMarket: %v", err)
		}
	}

	states := []*models.MarketState{
		{MarketID: "m1", WelfordCount: 5, LastPrice: 0.5},
		{MarketID: "m2", WelfordCount: 10, LastPrice: 0.6},
		{MarketID: "m3", WelfordCount: 15, LastPrice: 0.7},
	}

	for _, state := range states {
		if err := s.SaveState(state.MarketID, state); err != nil {
			t.Fatalf("SaveState: %v", err)
		}
	}

	loaded, err := s.LoadAllStates()
	if err != nil {
		t.Fatalf("LoadAllStates: %v", err)
	}

	if len(loaded) != 3 {
		t.Errorf("expected 3 states, got %d", len(loaded))
	}

	for _, state := range states {
		if _, ok := loaded[state.MarketID]; !ok {
			t.Errorf("state %s not found", state.MarketID)
		}
	}
}

func TestStorage_AddAlert(t *testing.T) {
	s := newTestStorage(t)

	// First add the market
	m := testMarket("test:market", "test", "market", time.Now())
	if err := s.AddMarket(m); err != nil {
		t.Fatalf("AddMarket: %v", err)
	}

	alert := &models.Alert{
		MarketID:    "test:market",
		EventTitle:  "Test Event",
		FinalScore:  5.5,
		OldProb:     0.50,
		NewProb:     0.65,
		PriceDelta:  0.15,
		VolumeDelta: 50000,
		DetectedAt:  time.Now(),
	}

	if err := s.AddAlert(alert); err != nil {
		t.Fatalf("AddAlert: %v", err)
	}

	alerts, err := s.GetTopAlerts(10)
	if err != nil {
		t.Fatalf("GetTopAlerts: %v", err)
	}

	if len(alerts) != 1 {
		t.Errorf("expected 1 alert, got %d", len(alerts))
	}

	if alerts[0].MarketID != alert.MarketID {
		t.Errorf("market ID: got %s, want %s", alerts[0].MarketID, alert.MarketID)
	}
}

func TestStorage_GetTopAlerts(t *testing.T) {
	s := newTestStorage(t)

	// Add markets first
	now := time.Now()
	for i := 1; i <= 3; i++ {
		mid := fmt.Sprintf("m%d", i)
		m := testMarket(mid, fmt.Sprintf("e%d", i), mid, now)
		if err := s.AddMarket(m); err != nil {
			t.Fatalf("AddMarket: %v", err)
		}
	}

	alerts := []*models.Alert{
		{MarketID: "m1", FinalScore: 5.0, DetectedAt: time.Now()},
		{MarketID: "m2", FinalScore: 7.0, DetectedAt: time.Now()},
		{MarketID: "m3", FinalScore: 3.0, DetectedAt: time.Now()},
	}

	for _, a := range alerts {
		if err := s.AddAlert(a); err != nil {
			t.Fatalf("AddAlert: %v", err)
		}
	}

	top, err := s.GetTopAlerts(2)
	if err != nil {
		t.Fatalf("GetTopAlerts: %v", err)
	}

	if len(top) != 2 {
		t.Fatalf("expected 2 alerts, got %d", len(top))
	}

	// Should be sorted by FinalScore descending
	if top[0].FinalScore < top[1].FinalScore {
		t.Error("alerts not sorted by FinalScore descending")
	}
	if top[0].FinalScore != 7.0 {
		t.Errorf("top score: got %v, want 7.0", top[0].FinalScore)
	}
}

func TestStorage_ClearAlerts(t *testing.T) {
	s := newTestStorage(t)

	// Add market first
	m := testMarket("test", "e", "test", time.Now())
	if err := s.AddMarket(m); err != nil {
		t.Fatalf("AddMarket: %v", err)
	}

	alert := &models.Alert{
		MarketID:   "test",
		FinalScore: 5.0,
		DetectedAt: time.Now(),
	}

	_ = s.AddAlert(alert)
	if err := s.ClearAlerts(); err != nil {
		t.Fatalf("ClearAlerts: %v", err)
	}

	alerts, _ := s.GetTopAlerts(10)
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts after clear, got %d", len(alerts))
	}
}

func TestStorage_DefaultPath(t *testing.T) {
	s, err := New(10, "")
	if err != nil {
		t.Fatalf("New with empty path: %v", err)
	}
	defer s.Close()
}
