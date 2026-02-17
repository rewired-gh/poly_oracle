package monitor

import (
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/poly-oracle/internal/models"
	"github.com/poly-oracle/internal/storage"
)

// ─── Existing DetectChanges tests (T016: updated to remove threshold arg) ────

func TestDetectChanges(t *testing.T) {
	s := storage.New(100, 50, "/tmp/test-monitor.json", 0644, 0755)
	m := New(s)

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

	events := []models.Event{event}
	changes, _, err := m.DetectChanges(events, 2*time.Hour)
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

func TestDetectChanges_BelowThreshold(t *testing.T) {
	s := storage.New(100, 50, "/tmp/test-threshold.json", 0644, 0755)
	m := New(s)

	now := time.Now()
	event := models.Event{
		ID:             "event-1",
		EventID:        "event-1",
		Title:          "Test?",
		Category:       "politics",
		YesProbability: 0.6001,
		NoProbability:  0.3999,
		Active:         true,
		LastUpdated:    now,
		CreatedAt:      now.Add(-1 * time.Hour),
	}
	if err := s.AddEvent(&event); err != nil {
		t.Fatalf("Failed to add event: %v", err)
	}

	// Very tiny change — below 0.001 floor
	snapshots := []models.Snapshot{
		{
			ID:             uuid.New().String(),
			EventID:        "event-1",
			YesProbability: 0.6000,
			NoProbability:  0.4000,
			Timestamp:      now.Add(-1 * time.Hour),
			Source:         "test",
		},
		{
			ID:             uuid.New().String(),
			EventID:        "event-1",
			YesProbability: 0.6001,
			NoProbability:  0.3999,
			Timestamp:      now,
			Source:         "test",
		},
	}
	for _, snap := range snapshots {
		if err := s.AddSnapshot(&snap); err != nil {
			t.Fatalf("Failed to add snapshot: %v", err)
		}
	}

	events := []models.Event{event}
	changes, _, err := m.DetectChanges(events, 2*time.Hour)
	if err != nil {
		t.Fatalf("DetectChanges failed: %v", err)
	}
	if len(changes) != 0 {
		t.Errorf("Expected 0 changes (below 0.001 floor), got %d", len(changes))
	}
}

func TestDetectChanges_OutOfOrderSnapshots(t *testing.T) {
	s := storage.New(100, 50, "/tmp/test-out-of-order.json", 0644, 0755)
	m := New(s)

	now := time.Now()
	event := models.Event{
		ID:             "event-1",
		EventID:        "event-1",
		Title:          "Test?",
		Category:       "politics",
		YesProbability: 0.85,
		NoProbability:  0.15,
		Active:         true,
		LastUpdated:    now,
		CreatedAt:      now.Add(-2 * time.Hour),
	}
	if err := s.AddEvent(&event); err != nil {
		t.Fatalf("Failed to add event: %v", err)
	}

	// Add snapshots OUT OF ORDER to test sorting
	snapshots := []models.Snapshot{
		{
			ID:             uuid.New().String(),
			EventID:        "event-1",
			YesProbability: 0.70,
			NoProbability:  0.30,
			Timestamp:      now.Add(-1 * time.Hour),
			Source:         "test",
		},
		{
			ID:             uuid.New().String(),
			EventID:        "event-1",
			YesProbability: 0.50,
			NoProbability:  0.50,
			Timestamp:      now.Add(-2 * time.Hour),
			Source:         "test",
		},
		{
			ID:             uuid.New().String(),
			EventID:        "event-1",
			YesProbability: 0.85,
			NoProbability:  0.15,
			Timestamp:      now,
			Source:         "test",
		},
	}
	for _, snap := range snapshots {
		if err := s.AddSnapshot(&snap); err != nil {
			t.Fatalf("Failed to add snapshot: %v", err)
		}
	}

	events := []models.Event{event}
	changes, _, err := m.DetectChanges(events, 3*time.Hour)
	if err != nil {
		t.Fatalf("DetectChanges failed: %v", err)
	}
	if len(changes) == 0 {
		t.Fatal("Expected at least 1 change, got 0")
	}

	expectedMagnitude := 0.35
	if changes[0].Magnitude < expectedMagnitude-0.01 || changes[0].Magnitude > expectedMagnitude+0.01 {
		t.Errorf("Expected magnitude %.2f (0.85 - 0.50), got %.2f", expectedMagnitude, changes[0].Magnitude)
	}
	if changes[0].Direction != "increase" {
		t.Errorf("Expected direction 'increase', got '%s'", changes[0].Direction)
	}
	if changes[0].OldProbability != 0.50 {
		t.Errorf("Expected old probability 0.50, got %.2f", changes[0].OldProbability)
	}
	if changes[0].NewProbability != 0.85 {
		t.Errorf("Expected new probability 0.85, got %.2f", changes[0].NewProbability)
	}
}

// ─── T011: TestKLDivergence ───────────────────────────────────────────────────

func TestKLDivergence(t *testing.T) {
	tests := []struct {
		name       string
		pOld, pNew float64
		wantMin    float64
		wantMax    float64
		wantPos    bool // result must be > 0
	}{
		{
			name: "5% move at p=0.5 — small positive",
			pOld: 0.50, pNew: 0.55,
			wantMin: 0.001, wantMax: 0.01,
		},
		{
			name: "10% move at p=0.5 — medium positive",
			pOld: 0.50, pNew: 0.60,
			wantMin: 0.005, wantMax: 0.025,
		},
		{
			name: "KL(0.6||0.5) must be positive",
			pOld: 0.50, pNew: 0.60,
			wantPos: true,
		},
		{
			name: "no change — near zero",
			pOld: 0.70, pNew: 0.70,
			wantMin: 0.0, wantMax: 1e-10,
		},
		{
			name: "boundary p=0.0 does not panic or NaN",
			pOld: 0.0, pNew: 0.05,
		},
		{
			name: "boundary p=1.0 does not panic or NaN",
			pOld: 1.0, pNew: 0.95,
		},
		{
			name: "always non-negative",
			pOld: 0.3, pNew: 0.7,
			wantMin: 0.0, wantMax: math.MaxFloat64,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := KLDivergence(tt.pOld, tt.pNew)
			if math.IsNaN(got) {
				t.Errorf("KLDivergence(%v, %v) = NaN", tt.pOld, tt.pNew)
				return
			}
			if math.IsInf(got, 0) {
				t.Errorf("KLDivergence(%v, %v) = Inf", tt.pOld, tt.pNew)
				return
			}
			if got < 0 {
				t.Errorf("KLDivergence(%v, %v) = %v, want >= 0", tt.pOld, tt.pNew, got)
			}
			if tt.wantPos && got <= 0 {
				t.Errorf("KLDivergence(%v, %v) = %v, want > 0", tt.pOld, tt.pNew, got)
			}
			if tt.wantMax > 0 && (got < tt.wantMin || got > tt.wantMax) {
				t.Errorf("KLDivergence(%v, %v) = %v, want [%v, %v]", tt.pOld, tt.pNew, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

// ─── T012: TestLogVolumeWeight ────────────────────────────────────────────────

func TestLogVolumeWeight(t *testing.T) {
	const vRef = 25000.0
	tests := []struct {
		name             string
		volume24h, vRef  float64
		wantMin, wantMax float64
	}{
		{
			name:      "volume == vRef → 1.0",
			volume24h: vRef, vRef: vRef,
			wantMin: 0.99, wantMax: 1.01,
		},
		{
			name:      "volume = 0 → floor 0.1",
			volume24h: 0, vRef: vRef,
			wantMin: 0.1, wantMax: 0.1,
		},
		{
			name:      "volume = 4×vRef → ~2.32",
			volume24h: 4 * vRef, vRef: vRef,
			wantMin: 2.20, wantMax: 2.40,
		},
		{
			name:      "vRef = 0 treated as 1.0",
			volume24h: 100, vRef: 0,
			wantMin: 0.1, wantMax: 10.0,
		},
		{
			name:      "very large volume → capped by log growth",
			volume24h: 1_000_000, vRef: vRef,
			wantMin: 5.0, wantMax: 6.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LogVolumeWeight(tt.volume24h, tt.vRef)
			if math.IsNaN(got) || math.IsInf(got, 0) {
				t.Errorf("LogVolumeWeight(%v, %v) = %v (invalid)", tt.volume24h, tt.vRef, got)
				return
			}
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("LogVolumeWeight(%v, %v) = %v, want [%v, %v]",
					tt.volume24h, tt.vRef, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

// ─── T013: TestHistoricalSNR ──────────────────────────────────────────────────

func makeSnaps(probs []float64) []models.Snapshot {
	snaps := make([]models.Snapshot, len(probs))
	for i, p := range probs {
		snaps[i] = models.Snapshot{
			ID:             uuid.New().String(),
			YesProbability: p,
			Timestamp:      time.Now().Add(time.Duration(i) * time.Hour),
		}
	}
	return snaps
}

func TestHistoricalSNR(t *testing.T) {
	tests := []struct {
		name      string
		probs     []float64
		netChange float64
		wantMin   float64
		wantMax   float64
	}{
		{
			name:      "0 snapshots → 1.0",
			probs:     nil,
			netChange: 0.10,
			wantMin:   1.0, wantMax: 1.0,
		},
		{
			name:      "1 snapshot → 1.0",
			probs:     []float64{0.5},
			netChange: 0.10,
			wantMin:   1.0, wantMax: 1.0,
		},
		{
			name:      "stable snapshots (σ ≈ 0) → 1.0",
			probs:     []float64{0.5, 0.50001, 0.50002, 0.50001, 0.5},
			netChange: 0.10,
			wantMin:   1.0, wantMax: 1.0,
		},
		{
			name:      "large move on quiet market → clamp 5.0",
			probs:     []float64{0.50, 0.501, 0.502, 0.501, 0.500},
			netChange: 0.30, // huge net change relative to tiny historical σ
			wantMin:   5.0, wantMax: 5.0,
		},
		{
			name:      "tiny move on volatile market → clamp 0.5",
			probs:     []float64{0.50, 0.60, 0.40, 0.65, 0.35},
			netChange: 0.005, // tiny net change relative to large historical σ
			wantMin:   0.5, wantMax: 0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HistoricalSNR(makeSnaps(tt.probs), tt.netChange)
			if math.IsNaN(got) || math.IsInf(got, 0) {
				t.Errorf("HistoricalSNR = %v (invalid)", got)
				return
			}
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("HistoricalSNR = %v, want [%v, %v]", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

// ─── T014: TestTrajectoryConsistency ─────────────────────────────────────────

func TestTrajectoryConsistency(t *testing.T) {
	tests := []struct {
		name    string
		probs   []float64
		wantMin float64
		wantMax float64
	}{
		{
			name:    "0 snapshots → 1.0",
			probs:   nil,
			wantMin: 1.0, wantMax: 1.0,
		},
		{
			name:    "1 snapshot → 1.0",
			probs:   []float64{0.5},
			wantMin: 1.0, wantMax: 1.0,
		},
		{
			name:    "2 snapshots (1 pair) → 1.0",
			probs:   []float64{0.5, 0.6},
			wantMin: 1.0, wantMax: 1.0,
		},
		{
			name:    "monotonic rise → 1.0",
			probs:   []float64{0.50, 0.55, 0.60, 0.65},
			wantMin: 1.0, wantMax: 1.0,
		},
		{
			name:    "perfect oscillation (net ≈ 0) → ~0",
			probs:   []float64{0.50, 0.60, 0.50, 0.60, 0.50},
			wantMin: 0.0, wantMax: 0.05,
		},
		{
			name:    "mostly up, one reversal → between 0.5 and 1.0",
			probs:   []float64{0.50, 0.55, 0.60, 0.55, 0.65},
			wantMin: 0.5, wantMax: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TrajectoryConsistency(makeSnaps(tt.probs))
			if math.IsNaN(got) || math.IsInf(got, 0) {
				t.Errorf("TrajectoryConsistency = %v (invalid)", got)
				return
			}
			if got < 0 || got > 1.0001 {
				t.Errorf("TrajectoryConsistency = %v, out of [0, 1]", got)
			}
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("TrajectoryConsistency = %v, want [%v, %v]", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

// ─── T015: TestScoring — 8 comprehensive cases ───────────────────────────────

func TestScoring(t *testing.T) {
	// buildChange creates a Change with given fields
	buildChange := func(eventID string, pOld, pNew float64, window time.Duration) models.Change {
		mag := math.Abs(pNew - pOld)
		dir := "increase"
		if pNew < pOld {
			dir = "decrease"
		}
		return models.Change{
			ID:             uuid.New().String(),
			EventID:        eventID,
			OldProbability: pOld,
			NewProbability: pNew,
			Magnitude:      mag,
			Direction:      dir,
			TimeWindow:     window,
			DetectedAt:     time.Now(),
		}
	}

	// buildEvent creates an Event with given volume
	buildEvent := func(id string, vol float64) *models.Event {
		return &models.Event{
			ID:         id,
			EventID:    id,
			Volume24hr: vol,
		}
	}

	t.Run("VolumeWins — large market 5% beats small market 9%", func(t *testing.T) {
		klHigh := KLDivergence(0.50, 0.55) // 5% move
		klLow := KLDivergence(0.50, 0.59)  // 9% move
		scoreA := CompositeScore(klHigh, LogVolumeWeight(1_000_000, 25000), 1.0, 1.0)
		scoreB := CompositeScore(klLow, LogVolumeWeight(30_000, 25000), 1.0, 1.0)
		if scoreA <= scoreB {
			t.Errorf("VolumeWins: A(1M vol, 5%%) = %.6f should beat B(30K vol, 9%%) = %.6f", scoreA, scoreB)
		}
	})

	t.Run("SNRWins — quiet market 3% beats volatile market 3%", func(t *testing.T) {
		kl := KLDivergence(0.50, 0.53)
		snrQuiet := HistoricalSNR(makeSnaps([]float64{0.50, 0.501, 0.499, 0.500, 0.501}), 0.03)
		snrNoisy := HistoricalSNR(makeSnaps([]float64{0.50, 0.60, 0.40, 0.65, 0.35}), 0.03)
		scoreQ := CompositeScore(kl, 1.0, snrQuiet, 1.0)
		scoreN := CompositeScore(kl, 1.0, snrNoisy, 1.0)
		if scoreQ <= scoreN {
			t.Errorf("SNRWins: quiet(SNR=%.2f)=%.6f should beat noisy(SNR=%.2f)=%.6f", snrQuiet, scoreQ, snrNoisy, scoreN)
		}
	})

	t.Run("KLRegimeDiff — same magnitude different regime gives different scores", func(t *testing.T) {
		klMid := KLDivergence(0.50, 0.55)  // 5% at mid
		klTail := KLDivergence(0.95, 1.00) // ~5% at tail (near certainty)
		if math.Abs(klMid-klTail) < 1e-6 {
			t.Errorf("KLRegimeDiff: expected different KL values, got %.6f vs %.6f", klMid, klTail)
		}
	})

	t.Run("MonotonicBeatsNoisy — clean path scores higher than oscillating path", func(t *testing.T) {
		kl := KLDivergence(0.50, 0.58)
		tcMono := TrajectoryConsistency(makeSnaps([]float64{0.50, 0.52, 0.55, 0.58}))
		tcNoisy := TrajectoryConsistency(makeSnaps([]float64{0.50, 0.68, 0.42, 0.58}))
		scoreM := CompositeScore(kl, 1.0, 1.0, tcMono)
		scoreN := CompositeScore(kl, 1.0, 1.0, tcNoisy)
		if scoreM <= scoreN {
			t.Errorf("MonotonicBeatsNoisy: mono(TC=%.2f)=%.6f should beat noisy(TC=%.2f)=%.6f",
				tcMono, scoreM, tcNoisy, scoreN)
		}
	})

	t.Run("DegenProbabilities — p=0.0 and p=1.0 do not panic or NaN", func(t *testing.T) {
		kl1 := KLDivergence(0.0, 0.05)
		kl2 := KLDivergence(1.0, 0.95)
		kl3 := KLDivergence(0.0, 1.0)
		for _, kl := range []float64{kl1, kl2, kl3} {
			if math.IsNaN(kl) || math.IsInf(kl, 0) || kl < 0 {
				t.Errorf("DegenProbabilities: KL = %v (invalid)", kl)
			}
		}
	})

	t.Run("ZeroVolumeFloor — volume=0 gets non-zero score", func(t *testing.T) {
		kl := KLDivergence(0.50, 0.55)
		vw := LogVolumeWeight(0, 25000)
		score := CompositeScore(kl, vw, 1.0, 1.0)
		if score <= 0 {
			t.Errorf("ZeroVolumeFloor: expected positive score, got %v", score)
		}
		if vw < 0.1 {
			t.Errorf("ZeroVolumeFloor: volume weight should be at least 0.1, got %v", vw)
		}
	})

	t.Run("SNRFallback — single snapshot gives SNR=1.0 and valid score", func(t *testing.T) {
		snr := HistoricalSNR(makeSnaps([]float64{0.5}), 0.05)
		if snr != 1.0 {
			t.Errorf("SNRFallback: expected SNR=1.0 for single snapshot, got %v", snr)
		}
		score := CompositeScore(KLDivergence(0.5, 0.55), 1.0, snr, 1.0)
		if math.IsNaN(score) || math.IsInf(score, 0) || score <= 0 {
			t.Errorf("SNRFallback: invalid score %v", score)
		}
	})

	t.Run("Determinism — identical inputs produce identical ranked output", func(t *testing.T) {
		store := storage.New(100, 50, "/tmp/test-determinism.json", 0644, 0755)
		mon := New(store)

		events := map[string]*models.Event{
			"evt-a": buildEvent("evt-a", 100_000),
			"evt-b": buildEvent("evt-b", 200_000),
			"evt-c": buildEvent("evt-c", 50_000),
		}
		changes := []models.Change{
			buildChange("evt-a", 0.50, 0.60, time.Hour),
			buildChange("evt-b", 0.40, 0.55, time.Hour),
			buildChange("evt-c", 0.60, 0.75, time.Hour),
		}

		result1 := mon.ScoreAndRank(changes, events, 0.0, 10, 25000.0)
		result2 := mon.ScoreAndRank(changes, events, 0.0, 10, 25000.0)

		if len(result1) != len(result2) {
			t.Fatalf("Determinism: different lengths %d vs %d", len(result1), len(result2))
		}
		for i := range result1 {
			if result1[i].EventID != result2[i].EventID || result1[i].SignalScore != result2[i].SignalScore {
				t.Errorf("Determinism: position %d differs: %s/%.6f vs %s/%.6f",
					i, result1[i].EventID, result1[i].SignalScore,
					result2[i].EventID, result2[i].SignalScore)
			}
		}
	})
}

// ─── ScoreAndRank integration tests ──────────────────────────────────────────

func TestScoreAndRank_TopKLimit(t *testing.T) {
	store := storage.New(100, 50, "/tmp/test-rank-topk.json", 0644, 0755)
	mon := New(store)

	events := map[string]*models.Event{
		"e1": {ID: "e1", Volume24hr: 100_000},
		"e2": {ID: "e2", Volume24hr: 100_000},
		"e3": {ID: "e3", Volume24hr: 100_000},
	}
	changes := []models.Change{
		{ID: "c1", EventID: "e1", OldProbability: 0.50, NewProbability: 0.65, Magnitude: 0.15, Direction: "increase", TimeWindow: time.Hour, DetectedAt: time.Now()},
		{ID: "c2", EventID: "e2", OldProbability: 0.50, NewProbability: 0.70, Magnitude: 0.20, Direction: "increase", TimeWindow: time.Hour, DetectedAt: time.Now()},
		{ID: "c3", EventID: "e3", OldProbability: 0.50, NewProbability: 0.60, Magnitude: 0.10, Direction: "increase", TimeWindow: time.Hour, DetectedAt: time.Now()},
	}

	top := mon.ScoreAndRank(changes, events, 0.0, 2, 25000.0)
	if len(top) != 2 {
		t.Errorf("Expected 2 results (k=2), got %d", len(top))
	}
}

func TestScoreAndRank_NeverNil(t *testing.T) {
	store := storage.New(100, 50, "/tmp/test-rank-nil.json", 0644, 0755)
	mon := New(store)

	result := mon.ScoreAndRank(nil, map[string]*models.Event{}, 0.0, 5, 25000.0)
	if result == nil {
		t.Error("ScoreAndRank should never return nil, got nil")
	}
}

func TestScoreAndRank_MinScoreFilters(t *testing.T) {
	store := storage.New(100, 50, "/tmp/test-rank-minscore.json", 0644, 0755)
	mon := New(store)

	events := map[string]*models.Event{
		"e1": {ID: "e1", Volume24hr: 100_000},
	}
	changes := []models.Change{
		{ID: "c1", EventID: "e1", OldProbability: 0.50, NewProbability: 0.51, Magnitude: 0.01, Direction: "increase", TimeWindow: time.Hour, DetectedAt: time.Now()},
	}

	// With very high minScore, nothing should pass
	result := mon.ScoreAndRank(changes, events, 999.0, 5, 25000.0)
	if len(result) != 0 {
		t.Errorf("Expected 0 results with minScore=999, got %d", len(result))
	}
}

func TestScoreAndRank_TopKZero(t *testing.T) {
	store := storage.New(100, 50, "/tmp/test-rank-k0.json", 0644, 0755)
	mon := New(store)

	events := map[string]*models.Event{
		"e1": {ID: "e1", Volume24hr: 100_000},
	}
	changes := []models.Change{
		{ID: "c1", EventID: "e1", OldProbability: 0.50, NewProbability: 0.70, Magnitude: 0.20, Direction: "increase", TimeWindow: time.Hour, DetectedAt: time.Now()},
	}

	result := mon.ScoreAndRank(changes, events, 0.0, 0, 25000.0)
	if len(result) != 0 {
		t.Errorf("Expected 0 results when k=0, got %d", len(result))
	}
}

// ─── Scenario tests: quality bar calibration ──────────────────────────────────
//
// These tests verify the composite score algorithm produces appropriate
// signal/noise discrimination for two real-world polling configurations.
// Probability values and volumes are inspired by real 2026-02-17 market data.

// TestScenario_PollInterval5m validates the quality bar at 5m polling
// (config.test.yaml: sensitivity=0.4, detection_intervals=4 → 20m window,
// minScore = 0.4² × 0.05 = 0.008).
//
// Signals tested:
//   - SHEIN IPO: 7% probability drop on a $500K-volume market → passes (high-information move)
//   - Díaz-Canel: 0.4%→0.8% tail move on $10K volume → filtered (tiny, illiquid)
//   - Podcast market: 8.8% near-certainty drop, noisy history, $30K volume → lower than SHEIN
func TestScenario_PollInterval5m(t *testing.T) {
	const sensitivity = 0.4
	minScore := sensitivity * sensitivity * 0.05 // 0.008
	const vRef = 25000.0

	// Stable history for high-SNR calculation: tiny alternating noise ≈ σ=0.0015
	stableHistory := makeSnaps([]float64{0.450, 0.451, 0.449, 0.450, 0.451, 0.449, 0.450, 0.451})
	// Noisy history for low-SNR: volatile oscillations across 10+ percent
	noisyHistory := makeSnaps([]float64{0.87, 0.93, 0.81, 0.90, 0.85, 0.92, 0.80, 0.91, 0.84, 0.90})

	// SHEIN IPO: 7% drop from 45% → 38%, $500K daily volume, quiet history
	t.Run("ValuableSignal_LargeMove_HighVolume", func(t *testing.T) {
		kl := KLDivergence(0.45, 0.38)
		vw := LogVolumeWeight(500_000, vRef)
		snr := HistoricalSNR(stableHistory, 0.45-0.38)
		score := CompositeScore(kl, vw, snr, 1.0)
		if score < minScore {
			t.Errorf("SHEIN 7%% drop: score=%.6f should exceed minScore=%.4f (kl=%.5f, vw=%.3f, snr=%.3f)",
				score, minScore, kl, vw, snr)
		}
	})

	// Díaz-Canel: 0.4%→0.8% tail move, $10K volume — pure noise
	t.Run("NoiseFiltered_TailMove_LowVolume", func(t *testing.T) {
		kl := KLDivergence(0.004, 0.008)
		vw := LogVolumeWeight(10_000, vRef)
		score := CompositeScore(kl, vw, 1.0, 1.0)
		if score >= minScore {
			t.Errorf("Diaz-Canel 0.4→0.8%%%% tail: score=%.6f should be below minScore=%.4f (kl=%.5f, vw=%.3f)",
				score, minScore, kl, vw)
		}
	})

	// Podcast market: large move near certainty but noisy and low-volume — ranks below SHEIN
	t.Run("NoisyMarket_RanksBelow_CleanLargeMove", func(t *testing.T) {
		klPodcast := KLDivergence(0.898, 0.810)
		vwPodcast := LogVolumeWeight(30_000, vRef)
		snrPodcast := HistoricalSNR(noisyHistory, 0.898-0.810)
		scorePodcast := CompositeScore(klPodcast, vwPodcast, snrPodcast, 1.0)

		klSHEIN := KLDivergence(0.45, 0.38)
		vwSHEIN := LogVolumeWeight(500_000, vRef)
		snrSHEIN := HistoricalSNR(stableHistory, 0.45-0.38)
		scoreSHEIN := CompositeScore(klSHEIN, vwSHEIN, snrSHEIN, 1.0)

		if scorePodcast >= scoreSHEIN {
			t.Errorf("Podcast (score=%.6f) should rank below clean SHEIN (score=%.6f)", scorePodcast, scoreSHEIN)
		}
	})
}

// TestScenario_PollInterval15m validates the quality bar at 15m polling
// (config.yaml.example: sensitivity=0.5, detection_intervals=4 → 60m window,
// minScore = 0.5² × 0.05 = 0.0125).
//
// Signals tested:
//   - Grok AI: 9.4% drop from high certainty (91.9%→82.5%), $200K volume → passes
//   - Norway Olympics: 0.6% noise at near-certainty (94%→94.6%), $50K volume → filtered
//   - Iran geopolitics: 4% move in quiet $1M market → passes (large liquid market)
//   - Iran specific date: 0.5%→1.5% tail move, $20K volume → filtered
func TestScenario_PollInterval15m(t *testing.T) {
	const sensitivity = 0.5
	minScore := sensitivity * sensitivity * 0.05 // 0.0125
	const vRef = 25000.0

	stableHistory := makeSnaps([]float64{0.920, 0.919, 0.921, 0.918, 0.920, 0.919, 0.921, 0.920})

	// Grok AI market: 9.4% drop from high certainty, significant volume
	t.Run("ValuableSignal_HighCertaintyDrop_MedVolume", func(t *testing.T) {
		kl := KLDivergence(0.919, 0.825)
		vw := LogVolumeWeight(200_000, vRef)
		snr := HistoricalSNR(stableHistory, 0.919-0.825)
		score := CompositeScore(kl, vw, snr, 1.0)
		if score < minScore {
			t.Errorf("Grok 9.4%% drop: score=%.6f should exceed minScore=%.4f (kl=%.5f, vw=%.3f, snr=%.3f)",
				score, minScore, kl, vw, snr)
		}
	})

	// Norway Olympics: 0.6% near-certainty noise, moderate volume
	t.Run("NoiseFiltered_TinyMove_NearCertain", func(t *testing.T) {
		kl := KLDivergence(0.940, 0.946)
		vw := LogVolumeWeight(50_000, vRef)
		score := CompositeScore(kl, vw, 1.0, 1.0)
		if score >= minScore {
			t.Errorf("Norway 0.6%% move: score=%.6f should be below minScore=%.4f (kl=%.5f, vw=%.3f)",
				score, minScore, kl, vw)
		}
	})

	// Iran geopolitics: 4% move in large liquid market (US/Israel attack question)
	t.Run("ValuableSignal_ModerateMove_HighVolume", func(t *testing.T) {
		quietHistory := makeSnaps([]float64{0.201, 0.200, 0.201, 0.200, 0.201, 0.200, 0.201, 0.200})
		kl := KLDivergence(0.20, 0.24)
		vw := LogVolumeWeight(1_000_000, vRef)
		snr := HistoricalSNR(quietHistory, 0.20-0.24)
		score := CompositeScore(kl, vw, snr, 1.0)
		if score < minScore {
			t.Errorf("Iran 4%% move: score=%.6f should exceed minScore=%.4f (kl=%.5f, vw=%.3f, snr=%.3f)",
				score, minScore, kl, vw, snr)
		}
	})

	// Iran specific-date contract: 0.5%→1.5% tail move, low volume
	t.Run("NoiseFiltered_TailMove_LowVolume", func(t *testing.T) {
		kl := KLDivergence(0.005, 0.015)
		vw := LogVolumeWeight(20_000, vRef)
		score := CompositeScore(kl, vw, 1.0, 1.0)
		if score >= minScore {
			t.Errorf("Iran tail 0.5→1.5%%%% move: score=%.6f should be below minScore=%.4f (kl=%.5f, vw=%.3f)",
				score, minScore, kl, vw)
		}
	})
}

// ─── Scenario tests: TC discrimination with multi-interval detection window ──
//
// These tests exercise ScoreAndRank with real storage snapshots to verify that
// TrajectoryConsistency actually discriminates once the detection window spans
// multiple poll intervals (detection_intervals ≥ 2 in config).
//
// "Important" = high-volume market (best-effort proxy for event significance).
// Multi-market events are represented by separate composite-ID entries.

// TestScenario_NoisySignalImportantEventFiltered verifies that a high-volume
// multi-market event is filtered when its window snapshots oscillate (low TC)
// and its history is volatile (low SNR). A clean signal from the same event
// serves as a positive control.
//
// Configuration: config.yaml.example (15m polling, sensitivity=0.5,
// detection_intervals=4 → 60m window, minScore=0.0125).
func TestScenario_NoisySignalImportantEventFiltered(t *testing.T) {
	// 15m polling × 4 intervals = 60m detection window (config.yaml.example)
	const detectionIntervals = 4
	const pollInterval = 15 * time.Minute
	detectionWindow := time.Duration(detectionIntervals) * pollInterval // 60m

	const sensitivity = 0.5
	minScore := sensitivity * sensitivity * 0.05 // 0.0125
	const vRef = 25000.0

	store := storage.New(200, 200, "/tmp/test-noisy-important.json", 0644, 0755)
	mon := New(store)

	// Multi-market event "BTC price targets": high-volume, two separate markets.
	// Market btc:100k ($2M volume) — oscillating window snapshots, volatile history.
	//   Window: [0.50, 0.62, 0.47, 0.61, 0.57]  →  Δs: +0.12, −0.15, +0.14, −0.04
	//   TC = |ΣΔ| / Σ|Δ| = 0.07/0.45 ≈ 0.156
	//   History: ±0.20 swings → σ≈0.28 → SNR clamped to 0.5 (minimum)
	//   Score = KL(0.50,0.57) × vw($2M) × 0.5 × 0.156 ≈ 0.005 < 0.0125 → FILTERED
	noisyEventID := "btc:100k"
	noisyEvt := &models.Event{
		ID: noisyEventID, EventID: "btc", MarketID: "100k",
		Title: "BTC price targets", Category: "crypto", Volume24hr: 2_000_000,
		YesProbability: 0.57, NoProbability: 0.43,
	}

	// Market btc:150k ($1.5M volume) — same oscillating pattern, also filtered.
	noisyEventID2 := "btc:150k"
	noisyEvt2 := &models.Event{
		ID: noisyEventID2, EventID: "btc", MarketID: "150k",
		Title: "BTC price targets", Category: "crypto", Volume24hr: 1_500_000,
		YesProbability: 0.18, NoProbability: 0.82,
	}

	// Positive control: eth:flip ($500K), monotonic window → TC=1.0, stable history → SNR=5.0
	// Score = KL(0.40,0.60) × vw($500K) × 5.0 × 1.0 ≈ 1.78 >> 0.0125 → PASSES
	cleanEventID := "eth:flip"
	cleanEvt := &models.Event{
		ID: cleanEventID, EventID: "eth", MarketID: "flip",
		Title: "ETH rally", Category: "crypto", Volume24hr: 500_000,
		YesProbability: 0.60, NoProbability: 0.40,
	}

	// Register events in storage so AddSnapshot succeeds.
	if err := store.AddEvent(noisyEvt); err != nil {
		t.Fatalf("AddEvent btc:100k failed: %v", err)
	}
	if err := store.AddEvent(noisyEvt2); err != nil {
		t.Fatalf("AddEvent btc:150k failed: %v", err)
	}
	if err := store.AddEvent(cleanEvt); err != nil {
		t.Fatalf("AddEvent eth:flip failed: %v", err)
	}

	now := time.Now()

	addSnap := func(t *testing.T, eventID string, p float64, age time.Duration) {
		t.Helper()
		if err := store.AddSnapshot(&models.Snapshot{
			ID: uuid.New().String(), EventID: eventID,
			YesProbability: p, NoProbability: 1 - p, Source: "test",
			Timestamp: now.Add(-age),
		}); err != nil {
			t.Fatalf("AddSnapshot(%s, p=%.3f, age=%v) failed: %v", eventID, p, age, err)
		}
	}

	// Historical snapshots OUTSIDE 60m window: ±0.20 swings → high σ → low SNR.
	noisyHistProbs := []float64{0.50, 0.70, 0.30, 0.70, 0.30, 0.50}
	for i, p := range noisyHistProbs {
		// Place at (window + (len-i)*15min) to be clearly outside window
		histAge := detectionWindow + time.Duration(len(noisyHistProbs)-i)*pollInterval
		addSnap(t, noisyEventID, p, histAge)
		addSnap(t, noisyEventID2, p*0.32, histAge)
	}

	cleanHistProbs := []float64{0.400, 0.401, 0.399, 0.400, 0.401, 0.400}
	for i, p := range cleanHistProbs {
		histAge := detectionWindow + time.Duration(len(cleanHistProbs)-i)*pollInterval
		addSnap(t, cleanEventID, p, histAge)
	}

	// Window snapshots INSIDE 60m window.
	// Noisy markets: oscillate [0.50, 0.62, 0.47, 0.61, 0.57] across the window.
	// Oldest is at window-5min (55m ago) to avoid boundary exclusion.
	noisyWindowProbs := []float64{0.50, 0.62, 0.47, 0.61, 0.57}
	winStep := (detectionWindow - 5*time.Minute) / time.Duration(len(noisyWindowProbs)-1)
	for i, p := range noisyWindowProbs {
		winAge := detectionWindow - 5*time.Minute - time.Duration(i)*winStep
		addSnap(t, noisyEventID, p, winAge)
		addSnap(t, noisyEventID2, p*0.32, winAge)
	}

	// Clean market: monotonic [0.40, 0.45, 0.50, 0.55, 0.60] across the window.
	cleanWindowProbs := []float64{0.40, 0.45, 0.50, 0.55, 0.60}
	for i, p := range cleanWindowProbs {
		winAge := detectionWindow - 5*time.Minute - time.Duration(i)*winStep
		addSnap(t, cleanEventID, p, winAge)
	}

	changes := []models.Change{
		{ID: uuid.New().String(), EventID: noisyEventID, OldProbability: 0.50, NewProbability: 0.57, Magnitude: 0.07, Direction: "increase", TimeWindow: detectionWindow, DetectedAt: now},
		{ID: uuid.New().String(), EventID: noisyEventID2, OldProbability: 0.16, NewProbability: 0.182, Magnitude: 0.022, Direction: "increase", TimeWindow: detectionWindow, DetectedAt: now},
		{ID: uuid.New().String(), EventID: cleanEventID, OldProbability: 0.40, NewProbability: 0.60, Magnitude: 0.20, Direction: "increase", TimeWindow: detectionWindow, DetectedAt: now},
	}
	eventsMap := map[string]*models.Event{
		noisyEventID:  noisyEvt,
		noisyEventID2: noisyEvt2,
		cleanEventID:  cleanEvt,
	}

	results := mon.ScoreAndRank(changes, eventsMap, minScore, 5, vRef)

	cleanPassed := false
	for _, r := range results {
		if r.EventID == cleanEventID {
			cleanPassed = true
		}
		if r.EventID == noisyEventID {
			t.Errorf("NoisyImportant: btc:100k oscillating signal should be filtered (score=%.6f, minScore=%.4f)", r.SignalScore, minScore)
		}
		if r.EventID == noisyEventID2 {
			t.Errorf("NoisyImportant: btc:150k oscillating signal should be filtered (score=%.6f, minScore=%.4f)", r.SignalScore, minScore)
		}
	}
	if !cleanPassed {
		t.Errorf("NoisyImportant: clean eth:flip signal should pass quality bar but was filtered")
	}
}

// TestScenario_SignificantSignalUnimportantEventFiltered verifies that a
// meaningful probability move on a minimum-volume market is filtered by the
// combined volume + SNR penalty, while the identical move on a high-volume
// liquid market (with rich history) passes.
//
// "Unimportant" = just above the liquidity floor ($25K min), sparse history.
// "Important" = high-volume ($1M), well-established with stable history.
//
// Configuration: config.yaml.example (15m polling, sensitivity=0.5,
// detection_intervals=4 → 60m window, minScore=0.0125).
//
// Score math (7% move, 50%→57%):
//
//	KL(0.50, 0.57) ≈ 0.00984
//	Unimportant: vw($30K)=log2(2.2)≈1.14, SNR=1.0 → score≈0.0112 < 0.0125 → FILTERED
//	Important:   vw($1M) =log2(41) ≈5.36, SNR=5.0 → score≈0.264  > 0.0125 → PASSES
func TestScenario_SignificantSignalUnimportantEventFiltered(t *testing.T) {
	// 15m polling × 4 intervals = 60m detection window (config.yaml.example)
	const detectionIntervals = 4
	const pollInterval = 15 * time.Minute
	detectionWindow := time.Duration(detectionIntervals) * pollInterval // 60m

	const sensitivity = 0.5
	minScore := sensitivity * sensitivity * 0.05 // 0.0125
	const vRef = 25000.0

	store := storage.New(200, 200, "/tmp/test-unimportant-event.json", 0644, 0755)
	mon := New(store)

	// "min-vol" — market just above liquidity floor ($30K), no historical data.
	// Volume barely passes pre-filter; SNR falls back to 1.0 (no history).
	// vw = log2(1 + 30000/25000) = log2(2.2) ≈ 1.14
	// Score = KL(0.50,0.57) × 1.14 × 1.0 × 1.0 ≈ 0.0112 < minScore=0.0125 → FILTERED
	lowVolID := "min-vol"
	lowVolEvt := &models.Event{
		ID: lowVolID, EventID: "min-vol", Title: "Low-volume market", Category: "other",
		Volume24hr: 30_000, YesProbability: 0.57, NoProbability: 0.43,
	}

	// "liq-vol" — highly liquid market ($1M volume), stable history → SNR=5.0.
	// vw = log2(1 + 1000000/25000) = log2(41) ≈ 5.36
	// Score = KL(0.50,0.57) × 5.36 × 5.0 × 1.0 ≈ 0.264 >> minScore=0.0125 → PASSES
	highVolID := "liq-vol"
	highVolEvt := &models.Event{
		ID: highVolID, EventID: "liq-vol", Title: "High-volume liquid market", Category: "politics",
		Volume24hr: 1_000_000, YesProbability: 0.57, NoProbability: 0.43,
	}

	if err := store.AddEvent(lowVolEvt); err != nil {
		t.Fatalf("AddEvent min-vol failed: %v", err)
	}
	if err := store.AddEvent(highVolEvt); err != nil {
		t.Fatalf("AddEvent liq-vol failed: %v", err)
	}

	now := time.Now()

	addSnap := func(t *testing.T, eventID string, p float64, age time.Duration) {
		t.Helper()
		if err := store.AddSnapshot(&models.Snapshot{
			ID: uuid.New().String(), EventID: eventID,
			YesProbability: p, NoProbability: 1 - p, Source: "test",
			Timestamp: now.Add(-age),
		}); err != nil {
			t.Fatalf("AddSnapshot(%s) failed: %v", eventID, err)
		}
	}

	// Stable historical snapshots for liq-vol ONLY (outside 60m window).
	// Tiny σ ≈ 0.001 → SNR = min(5, 0.07/0.001) = 5.0 for 7% move.
	// min-vol gets no history → SNR = 1.0 (fallback for sparse market).
	stableHistProbs := []float64{0.500, 0.501, 0.499, 0.500, 0.501, 0.499}
	for i, p := range stableHistProbs {
		histAge := detectionWindow + time.Duration(len(stableHistProbs)-i)*pollInterval
		addSnap(t, highVolID, p, histAge)
	}

	// Window snapshots for liq-vol: monotonic [0.50, 0.52, 0.54, 0.56, 0.57] → TC=1.0
	liqWindowProbs := []float64{0.50, 0.52, 0.54, 0.56, 0.57}
	winStep := (detectionWindow - 5*time.Minute) / time.Duration(len(liqWindowProbs)-1)
	for i, p := range liqWindowProbs {
		winAge := detectionWindow - 5*time.Minute - time.Duration(i)*winStep
		addSnap(t, highVolID, p, winAge)
	}
	// No window snapshots for min-vol (sparse market, TC=1.0 fallback).

	changes := []models.Change{
		{ID: uuid.New().String(), EventID: lowVolID, OldProbability: 0.50, NewProbability: 0.57, Magnitude: 0.07, Direction: "increase", TimeWindow: detectionWindow, DetectedAt: now},
		{ID: uuid.New().String(), EventID: highVolID, OldProbability: 0.50, NewProbability: 0.57, Magnitude: 0.07, Direction: "increase", TimeWindow: detectionWindow, DetectedAt: now},
	}
	eventsMap := map[string]*models.Event{
		lowVolID:  lowVolEvt,
		highVolID: highVolEvt,
	}

	results := mon.ScoreAndRank(changes, eventsMap, minScore, 5, vRef)

	highVolPassed := false
	for _, r := range results {
		if r.EventID == lowVolID {
			t.Errorf("UnimportantFiltered: min-vol ($30K, no history) should be filtered (score=%.6f, minScore=%.4f)", r.SignalScore, minScore)
		}
		if r.EventID == highVolID {
			highVolPassed = true
		}
	}
	if !highVolPassed {
		t.Errorf("UnimportantFiltered: liq-vol ($1M, stable history) same 7%% move should pass quality bar")
	}
}

// TestTrajectoryConsistency_SinglePairWindow documents that TC always returns
// 1.0 when the detection window contains exactly two snapshots (one polling
// interval). This is expected behaviour: TC provides no discrimination at the
// default poll_interval window, but kicks in when windows span multiple polls.
func TestTrajectoryConsistency_SinglePairWindow(t *testing.T) {
	// Two snapshots = one consecutive pair → TC definition returns 1.0
	twoSnaps := makeSnaps([]float64{0.50, 0.57})
	tc := TrajectoryConsistency(twoSnaps)
	if tc != 1.0 {
		t.Errorf("SinglePairWindow: expected TC=1.0 for 2-snapshot window, got %.6f", tc)
	}

	// Confirm with a clean monotonic 4-snapshot window TC > single-pair TC
	// (only relevant when window spans multiple poll intervals)
	fourSnapsMono := makeSnaps([]float64{0.50, 0.53, 0.56, 0.59})
	tcMono := TrajectoryConsistency(fourSnapsMono)
	if tcMono != 1.0 {
		t.Errorf("MonotonicMultiPair: expected TC=1.0 for perfectly monotonic window, got %.6f", tcMono)
	}

	// Oscillating multi-pair window gives TC < 1.0
	fourSnapsOscil := makeSnaps([]float64{0.50, 0.60, 0.50, 0.60})
	tcOscil := TrajectoryConsistency(fourSnapsOscil)
	if tcOscil >= 1.0 {
		t.Errorf("OscillatingMultiPair: expected TC < 1.0 for oscillating window, got %.6f", tcOscil)
	}
}
