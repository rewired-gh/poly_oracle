// Package monitor provides probability change detection and composite signal scoring.
//
// Changes are detected when probability movements exceed a minimum floor (0.1%).
// Each change is then scored using a four-factor composite algorithm:
//
//	score = KL(p_new || p_old) × log_volume_weight × historical_snr × trajectory_consistency
//
// KL divergence captures the information content of the probability update.
// Log volume weight scales by market liquidity (larger markets = more credible).
// Historical SNR measures how unusual this move is relative to the market's noise floor.
// Trajectory consistency rewards clean directional moves over oscillating noise.
//
// Use ScoreAndRank to apply quality filtering and return the top-K highest-signal changes.
package monitor

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/poly-oracle/internal/logger"
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

// DetectionError represents a per-event error during change detection
type DetectionError struct {
	EventID string
	Err     error
}

func (e DetectionError) Error() string {
	return fmt.Sprintf("detection error for event %s: %v", e.EventID, e.Err)
}

// minProbabilityChange is the hardcoded floor for change detection.
// Suppresses floating-point noise; all changes ≥ 0.1% are returned for scoring.
const minProbabilityChange = 0.001

// probEpsilon clamps probabilities away from 0 and 1 to prevent ln(0) in KL divergence.
const probEpsilon = 1e-7

// DetectChanges identifies probability changes within a time window that exceed the
// minimum floor (0.1%). Scoring via ScoreAndRank is responsible for quality filtering.
// Returns changes, per-event errors (non-fatal), and a fatal error if window is invalid.
func (m *Monitor) DetectChanges(events []models.Event, window time.Duration) ([]models.Change, []DetectionError, error) {
	if window <= 0 {
		return nil, nil, fmt.Errorf("invalid window %v: must be positive", window)
	}

	var changes []models.Change
	var detectionErrors []DetectionError
	now := time.Now()

	eventsWithZeroSnapshots := 0
	eventsWithOneSnapshot := 0
	eventsWithEnoughSnapshots := 0
	eventsWithChangeBelowFloor := 0
	maxChangeSeen := 0.0

	for _, event := range events {
		snapshots, err := m.storage.GetSnapshotsInWindow(event.ID, window)
		if err != nil {
			detectionErrors = append(detectionErrors, DetectionError{EventID: event.ID, Err: err})
			continue
		}

		if len(snapshots) == 0 {
			eventsWithZeroSnapshots++
			continue
		}
		if len(snapshots) == 1 {
			eventsWithOneSnapshot++
			continue
		}

		eventsWithEnoughSnapshots++

		oldest := snapshots[0]
		current := snapshots[len(snapshots)-1]

		change := math.Abs(current.YesProbability - oldest.YesProbability)

		if change > maxChangeSeen {
			maxChangeSeen = change
		}

		if change >= minProbabilityChange {
			direction := "increase"
			if current.YesProbability < oldest.YesProbability {
				direction = "decrease"
			}

			changes = append(changes, models.Change{
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
			})
		} else if change > 0 {
			eventsWithChangeBelowFloor++
		}
	}

	// Debug logging for understanding detection behavior
	logger.Debug("DetectChanges: 0 snapshots=%d, 1 snapshot=%d, >=2 snapshots=%d, below floor=%d, max_change=%.6f",
		eventsWithZeroSnapshots, eventsWithOneSnapshot, eventsWithEnoughSnapshots, eventsWithChangeBelowFloor, maxChangeSeen)

	return changes, detectionErrors, nil
}

// KLDivergence computes KL(pNew || pOld) for a binary (YES/NO) distribution.
// Both probabilities are clamped to [1e-7, 1-1e-7] to avoid ln(0).
// Returns the information gain (in nats) of updating from pOld to pNew.
func KLDivergence(pOld, pNew float64) float64 {
	pOld = math.Max(probEpsilon, math.Min(1-probEpsilon, pOld))
	pNew = math.Max(probEpsilon, math.Min(1-probEpsilon, pNew))
	return pNew*math.Log(pNew/pOld) + (1-pNew)*math.Log((1-pNew)/(1-pOld))
}

// LogVolumeWeight returns log2(1 + volume24h/vRef), floored at 0.1.
// At vRef volume the weight is 1.0; at 4×vRef it is ~2.32; at 0 volume it is 0.1.
// When vRef <= 0 it is treated as 1.0 to avoid division by zero.
func LogVolumeWeight(volume24h, vRef float64) float64 {
	if vRef <= 0 {
		vRef = 1.0
	}
	return math.Max(0.1, math.Log(1+volume24h/vRef)/math.Log(2))
}

// HistoricalSNR computes the signal-to-noise ratio of netChange relative to
// the market's historical volatility. σ is the sample std dev of consecutive
// Δp across all stored snapshots (Bessel correction, divide by n-1).
// Returns clamp(|netChange|/σ, 0.5, 5.0).
// Falls back to 1.0 when fewer than 2 consecutive pairs exist or σ < 1e-4.
func HistoricalSNR(allSnapshots []models.Snapshot, netChange float64) float64 {
	if len(allSnapshots) < 2 {
		return 1.0
	}

	deltas := make([]float64, 0, len(allSnapshots)-1)
	for i := 1; i < len(allSnapshots); i++ {
		deltas = append(deltas, allSnapshots[i].YesProbability-allSnapshots[i-1].YesProbability)
	}
	if len(deltas) == 0 {
		return 1.0
	}

	// Sample mean
	var sum float64
	for _, d := range deltas {
		sum += d
	}
	mean := sum / float64(len(deltas))

	// Sample std dev (Bessel correction: divide by n-1)
	if len(deltas) < 2 {
		return 1.0
	}
	var variance float64
	for _, d := range deltas {
		diff := d - mean
		variance += diff * diff
	}
	variance /= float64(len(deltas) - 1)
	sigma := math.Sqrt(variance)

	if sigma < 1e-4 {
		return 1.0
	}

	snr := math.Abs(netChange) / sigma
	return math.Max(0.5, math.Min(5.0, snr))
}

// TrajectoryConsistency returns |ΣΔp| / Σ|Δp| across consecutive snapshot pairs
// in the window. A value of 1.0 means perfectly directional; 0.0 means fully
// oscillating. Falls back to 1.0 when the window has ≤ 1 consecutive pair.
func TrajectoryConsistency(windowSnapshots []models.Snapshot) float64 {
	if len(windowSnapshots) < 2 {
		return 1.0
	}

	var sumSigned, sumAbs float64
	for i := 1; i < len(windowSnapshots); i++ {
		delta := windowSnapshots[i].YesProbability - windowSnapshots[i-1].YesProbability
		sumSigned += delta
		sumAbs += math.Abs(delta)
	}

	if sumAbs < 1e-10 {
		return 1.0
	}

	return math.Abs(sumSigned) / sumAbs
}

// CompositeScore multiplies the four factors into a single signal quality scalar.
func CompositeScore(kl, vw, snr, tc float64) float64 {
	return kl * vw * snr * tc
}

// ScoreAndRank scores each change using the four-factor composite signal score,
// filters out changes below minScore, and returns at most k changes sorted by
// score descending. Ties are broken by EventID lexicographic descending for
// determinism. Returns an empty (non-nil) slice when nothing clears the quality bar.
// vRef is the reference volume for log-volume weighting (typically volume_24hr_min
// from config); events at this volume receive weight ≈ 1.0.
func (m *Monitor) ScoreAndRank(
	changes []models.Change,
	events map[string]*models.Event,
	minScore float64,
	k int,
	vRef float64,
) []models.Change {
	if vRef <= 0 {
		vRef = 25000.0
	}

	var candidates []models.Change

	for _, change := range changes {
		event, ok := events[change.EventID]
		if !ok {
			logger.Warn("ScoreAndRank: event %s not found in map, skipping", change.EventID)
			continue
		}

		allSnaps, err := m.storage.GetSnapshots(change.EventID)
		snr := 1.0
		if err == nil {
			snr = HistoricalSNR(allSnaps, change.NewProbability-change.OldProbability)
		}

		winSnaps, err := m.storage.GetSnapshotsInWindow(change.EventID, change.TimeWindow)
		tc := 1.0
		if err == nil {
			tc = TrajectoryConsistency(winSnaps)
		}

		kl := KLDivergence(change.OldProbability, change.NewProbability)
		vw := LogVolumeWeight(event.Volume24hr, vRef)
		score := CompositeScore(kl, vw, snr, tc)

		change.SignalScore = score
		if score >= minScore {
			candidates = append(candidates, change)
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].SignalScore != candidates[j].SignalScore {
			return candidates[i].SignalScore > candidates[j].SignalScore
		}
		// Tie-break: EventID lexicographic descending
		return candidates[i].EventID > candidates[j].EventID
	})

	if k <= 0 || len(candidates) == 0 {
		return []models.Change{}
	}
	if k > len(candidates) {
		k = len(candidates)
	}
	return candidates[:k]
}
