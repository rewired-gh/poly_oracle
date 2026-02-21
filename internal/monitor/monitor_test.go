package monitor

import (
	"math"
	"testing"
	"time"

	"github.com/rewired-gh/polyoracle/internal/models"
	"github.com/rewired-gh/polyoracle/internal/storage"
)

func mustStorage(t *testing.T, maxMarkets int) *storage.Storage {
	t.Helper()
	s, err := storage.New(maxMarkets, ":memory:")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// ─── V2 Algorithm Tests ─────────────────────────────────────────────────────

func TestWelfordAlgorithm(t *testing.T) {
	tests := []struct {
		name       string
		prices     []float64
		wantMean   float64
		wantStdDev float64
	}{
		{
			name:       "single value",
			prices:     []float64{0.5},
			wantMean:   0.5,
			wantStdDev: 0.01, // fallback for < 2 samples
		},
		{
			name:       "two values",
			prices:     []float64{0.5, 0.6},
			wantMean:   0.55,
			wantStdDev: 0.07071067811865476, // sqrt(0.005)
		},
		{
			name:       "stable prices",
			prices:     []float64{0.50, 0.501, 0.499, 0.500, 0.501},
			wantMean:   0.5002,
			wantStdDev: Delta, // raw sigma (≈0.00084) is below the Delta floor
		},
		{
			name:       "volatile prices",
			prices:     []float64{0.3, 0.7, 0.2, 0.8, 0.1},
			wantMean:   0.42,
			wantStdDev: 0.31, // Approximate - allow wider tolerance
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &models.MarketState{}

			for _, price := range tt.prices {
				UpdateWelford(state, price)
			}

			mean := state.WelfordMean
			sigma := GetSigma(state)

			if math.Abs(mean-tt.wantMean) > 1e-9 {
				t.Errorf("mean = %v, want %v", mean, tt.wantMean)
			}
			// Allow wider tolerance for std dev
			tol := 1e-6
			if tt.name == "volatile prices" {
				tol = 0.01 // Allow 1% tolerance for volatile data
			}
			if math.Abs(sigma-tt.wantStdDev) > tol {
				t.Errorf("sigma = %v, want %v", sigma, tt.wantStdDev)
			}
		})
	}
}

func TestTCBuffer(t *testing.T) {
	tests := []struct {
		name    string
		values  []float64
		window  int
		wantLen int
		wantSum float64
	}{
		{
			name:    "fill buffer",
			values:  []float64{1.0, 2.0, 3.0},
			window:  3,
			wantLen: 3,
			wantSum: 6.0,
		},
		{
			name:    "overflow buffer (ring)",
			values:  []float64{1.0, 2.0, 3.0, 4.0, 5.0},
			window:  3,
			wantLen: 3,
			wantSum: 12.0, // 3+4+5
		},
		{
			name:    "partial fill",
			values:  []float64{1.0, 2.0},
			window:  5,
			wantLen: 2,
			wantSum: 3.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &models.MarketState{}

			for _, v := range tt.values {
				UpdateTCBuffer(state, v, tt.window)
			}

			if len(state.TCBuffer) != tt.wantLen {
				t.Errorf("buffer len = %d, want %d", len(state.TCBuffer), tt.wantLen)
			}

			var sum float64
			for _, v := range state.TCBuffer {
				sum += v
			}
			if math.Abs(sum-tt.wantSum) > 1e-9 {
				t.Errorf("buffer sum = %v, want %v", sum, tt.wantSum)
			}
		})
	}
}

func TestErf(t *testing.T) {
	// Test that erf approximation is reasonably accurate
	tests := []struct {
		x    float64
		want float64 // approximate expected value
	}{
		{0.0, 0.0},
		{1.0, 0.8427007929497149},
		{-1.0, -0.8427007929497149},
		{2.0, 0.9953222650189527},
		{0.5, 0.5204998778130465},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := erf(tt.x)
			// Allow 1% relative error, or 0.001 absolute error for small values
			absErr := math.Abs(got - tt.want)
			relErr := absErr / math.Abs(tt.want)
			if absErr > 0.001 && relErr > 0.01 {
				t.Errorf("erf(%v) = %v, want ~%v", tt.x, got, tt.want)
			}
		})
	}
}

func TestProcessMarket(t *testing.T) {
	store := mustStorage(t, 100)
	cfg := DefaultConfig()
	mon := New(store, cfg)

	// Add a market
	market := models.Market{
		ID:             "test:market",
		EventID:        "test",
		MarketID:       "market",
		Title:          "Test Market",
		YesProbability: 0.60,
		Volume24hr:     100_000,
		Liquidity:      50_000,
		Active:         true,
	}

	// First poll initializes state
	alerts := mon.ProcessPoll([]models.Market{market})
	if len(alerts) != 0 {
		t.Error("first poll should not generate alerts (initialization)")
	}

	// Second poll with price change
	market.YesProbability = 0.65
	alerts = mon.ProcessPoll([]models.Market{market})

	// May or may not generate alert depending on threshold
	// Just verify no panic/error
	for _, alert := range alerts {
		if alert.MarketID != "test:market" {
			t.Errorf("unexpected market ID: %s", alert.MarketID)
		}
		if alert.FinalScore <= 0 {
			t.Errorf("final score should be positive: %v", alert.FinalScore)
		}
	}
}

func TestShouldProcessMarket(t *testing.T) {
	cfg := DefaultConfig()
	store := mustStorage(t, 100)
	mon := New(store, cfg)

	tests := []struct {
		name   string
		market models.Market
		want   bool
	}{
		{
			name: "active with volume",
			market: models.Market{
				Active:     true,
				Volume24hr: 100_000,
			},
			want: true,
		},
		{
			name: "inactive market",
			market: models.Market{
				Active:     false,
				Volume24hr: 100_000,
			},
			want: false,
		},
		{
			name: "closed market",
			market: models.Market{
				Active:     true,
				Closed:     true,
				Volume24hr: 100_000,
			},
			want: false,
		},
		{
			name: "low volume",
			market: models.Market{
				Active:     true,
				Volume24hr: 1000,
			},
			want: false,
		},
		{
			name: "passes with 1wk volume",
			market: models.Market{
				Active:     true,
				Volume24hr: 1000,
				Volume1wk:  150_000,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mon.shouldProcessMarket(tt.market)
			if got != tt.want {
				t.Errorf("shouldProcessMarket() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGroupByEvent(t *testing.T) {
	store := mustStorage(t, 100)
	cfg := DefaultConfig()
	mon := New(store, cfg)

	alerts := []models.Alert{
		{MarketID: "btc:100k", EventTitle: "BTC Targets", EventURL: "url1", FinalScore: 5.0},
		{MarketID: "btc:150k", EventTitle: "BTC Targets", EventURL: "url1", FinalScore: 3.5},
		{MarketID: "eth:flip", EventTitle: "ETH Flip", EventURL: "url2", FinalScore: 4.0},
	}

	groups := mon.GroupByEvent(alerts)

	if len(groups) != 2 {
		t.Errorf("expected 2 groups, got %d", len(groups))
	}

	// Find BTC group
	var btcGroup *models.EventGroup
	for i := range groups {
		if groups[i].EventID == "btc" {
			btcGroup = &groups[i]
		}
	}

	if btcGroup == nil {
		t.Fatal("BTC group not found")
	}
	if len(btcGroup.Markets) != 2 {
		t.Errorf("expected 2 markets in BTC group, got %d", len(btcGroup.Markets))
	}
	if btcGroup.BestScore != 5.0 {
		t.Errorf("best score should be 5.0, got %v", btcGroup.BestScore)
	}
}

func TestFilterRecentlySent(t *testing.T) {
	store := mustStorage(t, 100)
	cfg := DefaultConfig()
	mon := New(store, cfg)

	group := models.EventGroup{
		EventID: "test",
		Markets: []models.Alert{
			{
				MarketID: "test:market",
				OldProb:  0.50,
				NewProb:  0.60,
			},
		},
	}

	// Record as notified with increase direction
	mon.RecordNotified([]models.EventGroup{group})

	// Same direction should be filtered
	filtered := mon.FilterRecentlySent([]models.EventGroup{group}, time.Hour)
	if len(filtered) != 0 {
		t.Error("same direction should be filtered within cooldown")
	}

	// Direction change should pass
	group.Markets[0].OldProb = 0.60
	group.Markets[0].NewProb = 0.50
	filtered = mon.FilterRecentlySent([]models.EventGroup{group}, time.Hour)
	if len(filtered) != 1 {
		t.Error("direction change should pass")
	}

	// Entering deterministic zone should pass
	group.Markets[0].OldProb = 0.85
	group.Markets[0].NewProb = 0.92
	filtered = mon.FilterRecentlySent([]models.EventGroup{group}, time.Hour)
	if len(filtered) != 1 {
		t.Error("entering deterministic zone should pass")
	}
}

func TestPostProcessAlerts(t *testing.T) {
	store := mustStorage(t, 100)
	cfg := Config{
		WindowSize:         3,
		Alpha:              0.1,
		Ceiling:            10.0,
		Threshold:          2.0,
		Volume24hrMin:      25000,
		Volume1wkMin:       100000,
		Volume1moMin:       500000,
		TopK:               2,
		CooldownMultiplier: 5,
		CheckpointInterval: 12,
	}
	mon := New(store, cfg)

	alerts := []models.Alert{
		{MarketID: "a:1", EventTitle: "Event A", FinalScore: 5.0},
		{MarketID: "a:2", EventTitle: "Event A", FinalScore: 4.5},
		{MarketID: "b:1", EventTitle: "Event B", FinalScore: 6.0},
		{MarketID: "c:1", EventTitle: "Event C", FinalScore: 3.0},
	}

	groups := mon.PostProcessAlerts(alerts, time.Hour)

	// Should limit to top 2 events
	if len(groups) > 2 {
		t.Errorf("expected at most 2 groups (TopK=2), got %d", len(groups))
	}

	// Verify ordering by BestScore descending
	for i := 1; i < len(groups); i++ {
		if groups[i].BestScore > groups[i-1].BestScore {
			t.Error("groups not sorted by BestScore descending")
		}
	}
}

func TestProcessPoll_Initialization(t *testing.T) {
	store := mustStorage(t, 100)
	cfg := DefaultConfig()
	mon := New(store, cfg)

	markets := []models.Market{
		{
			ID:             "test:1",
			YesProbability: 0.50,
			Volume24hr:     100_000,
			Liquidity:      50_000,
			Active:         true,
		},
		{
			ID:             "test:2",
			YesProbability: 0.60,
			Volume24hr:     200_000,
			Liquidity:      75_000,
			Active:         true,
		},
	}

	// First poll initializes state
	alerts := mon.ProcessPoll(markets)
	if len(alerts) != 0 {
		t.Error("first poll should return 0 alerts (initialization)")
	}

	// Verify states were created
	if len(mon.states) != 2 {
		t.Errorf("expected 2 states, got %d", len(mon.states))
	}
}

func TestProcessPoll_WithPriceChange(t *testing.T) {
	store := mustStorage(t, 100)
	cfg := Config{
		WindowSize:         3,
		Alpha:              0.1,
		Ceiling:            10.0,
		Threshold:          0.5, // Low threshold to catch changes
		Volume24hrMin:      25000,
		Volume1wkMin:       100000,
		Volume1moMin:       500000,
		TopK:               10,
		CooldownMultiplier: 5,
		CheckpointInterval: 12,
	}
	mon := New(store, cfg)

	// Initial market
	markets := []models.Market{
		{
			ID:             "test:1",
			YesProbability: 0.50,
			Volume24hr:     100_000,
			Liquidity:      50_000,
			Active:         true,
		},
	}

	// First poll
	mon.ProcessPoll(markets)

	// Second poll with significant price change
	markets[0].YesProbability = 0.70
	markets[0].Volume24hr = 150_000 // Volume increased

	alerts := mon.ProcessPoll(markets)

	// Should detect the change
	if len(alerts) == 0 {
		t.Error("expected alert for significant price change")
	}

	// Verify alert properties
	alert := alerts[0]
	if alert.OldProb != 0.50 {
		t.Errorf("old prob should be 0.50, got %v", alert.OldProb)
	}
	if alert.NewProb != 0.70 {
		t.Errorf("new prob should be 0.70, got %v", alert.NewProb)
	}
	if math.Abs(alert.PriceDelta-0.20) > 1e-9 {
		t.Errorf("price delta should be 0.20, got %v", alert.PriceDelta)
	}
	if alert.VolumeDelta != 50_000 {
		t.Errorf("volume delta should be 50000, got %v", alert.VolumeDelta)
	}
}

// ─── Determinism Test ──────────────────────────────────────────────────────

func TestProcessPoll_Determinism(t *testing.T) {
	store1 := mustStorage(t, 100)
	store2 := mustStorage(t, 100)
	cfg := DefaultConfig()

	mon1 := New(store1, cfg)
	mon2 := New(store2, cfg)

	markets := []models.Market{
		{
			ID:             "test:1",
			YesProbability: 0.50,
			Volume24hr:     100_000,
			Liquidity:      50_000,
			Active:         true,
		},
	}

	// Initialize both
	mon1.ProcessPoll(markets)
	mon2.ProcessPoll(markets)

	// Change price
	markets[0].YesProbability = 0.65
	markets[0].Volume24hr = 120_000

	alerts1 := mon1.ProcessPoll(markets)
	alerts2 := mon2.ProcessPoll(markets)

	if len(alerts1) != len(alerts2) {
		t.Errorf("different number of alerts: %d vs %d", len(alerts1), len(alerts2))
	}

	for i := range alerts1 {
		if alerts1[i].FinalScore != alerts2[i].FinalScore {
			t.Errorf("alert %d: different scores: %v vs %v", i, alerts1[i].FinalScore, alerts2[i].FinalScore)
		}
	}
}

// ─── Edge Cases ────────────────────────────────────────────────────────────

func TestProcessPoll_EmptyInput(t *testing.T) {
	store := mustStorage(t, 100)
	cfg := DefaultConfig()
	mon := New(store, cfg)

	alerts := mon.ProcessPoll([]models.Market{})
	if len(alerts) != 0 {
		t.Error("ProcessPoll should return empty slice for empty input")
	}
}

func TestProcessPoll_AllFiltered(t *testing.T) {
	store := mustStorage(t, 100)
	cfg := DefaultConfig()
	mon := New(store, cfg)

	// All markets have low volume, should be filtered
	markets := []models.Market{
		{
			ID:             "test:1",
			YesProbability: 0.50,
			Volume24hr:     100, // Below threshold
			Active:         true,
		},
	}

	alerts := mon.ProcessPoll(markets)
	if len(alerts) != 0 {
		t.Error("expected 0 alerts for low-volume market")
	}
}

// TestProcessPoll_SigmaCollapseAtLowProbability tests that a market stuck near 0%
// does not falsely trigger alerts when only tiny API-level fluctuations occur.
// Regression test for: Welford sigma collapses to near-zero at p≈0.001, causing
// hDist/sigma to explode even for negligible moves.
func TestProcessPoll_SigmaCollapseAtLowProbability(t *testing.T) {
	store := mustStorage(t, 100)
	cfg := DefaultConfig() // threshold = 3.0
	mon := New(store, cfg)

	// Market stuck at ~0.1% with realistic high volume.
	// Simulate several cycles at constant price to collapse Welford sigma.
	stable := models.Market{
		ID:             "test:lowprob",
		YesProbability: 0.001,
		Volume24hr:     63_000,
		Liquidity:      395_000,
		Active:         true,
	}

	// First poll: initializes state (no alert)
	mon.ProcessPoll([]models.Market{stable})

	// Run 10 more cycles at same price to drive Welford sigma toward zero.
	for i := 0; i < 10; i++ {
		alerts := mon.ProcessPoll([]models.Market{stable})
		if len(alerts) != 0 {
			t.Fatalf("cycle %d: no alert expected for zero-change market, got score=%.3f",
				i+1, alerts[0].FinalScore)
		}
	}

	// Tiny fluctuation that rounds to the same display value (0.001 → 0.001 with %.3f)
	// but differs in float64 — exactly the scenario seen in production.
	fluctuated := stable
	fluctuated.YesProbability = 0.0017 // rounds to 0.002 at %.3f but is very small

	alerts := mon.ProcessPoll([]models.Market{fluctuated})
	if len(alerts) != 0 {
		t.Errorf("sigma-collapsed market should NOT alert on tiny fluctuation: score=%.3f hDist≈%.4f",
			alerts[0].FinalScore, alerts[0].HellingerDist)
	}
}

func TestProcessPoll_BoundaryProbabilities(t *testing.T) {
	store := mustStorage(t, 100)
	cfg := Config{
		WindowSize:         3,
		Alpha:              0.1,
		Ceiling:            10.0,
		Threshold:          0.1,
		Volume24hrMin:      25000,
		Volume1wkMin:       100000,
		Volume1moMin:       500000,
		TopK:               10,
		CooldownMultiplier: 5,
		CheckpointInterval: 12,
	}
	mon := New(store, cfg)

	tests := []struct {
		name string
		pOld float64
		pNew float64
	}{
		{"zero to small", 0.0, 0.05},
		{"near one to one", 0.95, 1.0},
		{"one to near one", 1.0, 0.95},
		{"boundary crossing", 0.05, 0.95},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			markets := []models.Market{
				{
					ID:             "test:1",
					YesProbability: tt.pOld,
					Volume24hr:     100_000,
					Liquidity:      50_000,
					Active:         true,
				},
			}

			// Initialize
			mon.ProcessPoll(markets)

			// Change to new probability
			markets[0].YesProbability = tt.pNew
			markets[0].Volume24hr = 110_000

			// Should not panic
			alerts := mon.ProcessPoll(markets)

			// Verify no NaN/Inf in results
			for _, alert := range alerts {
				if math.IsNaN(alert.FinalScore) || math.IsInf(alert.FinalScore, 0) {
					t.Errorf("invalid score: %v", alert.FinalScore)
				}
				if math.IsNaN(alert.HellingerDist) || math.IsInf(alert.HellingerDist, 0) {
					t.Errorf("invalid Hellinger distance: %v", alert.HellingerDist)
				}
			}
		})
	}
}
