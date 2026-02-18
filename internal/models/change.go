package models

import (
	"errors"
	"math"
	"time"
)

// Change represents a detected significant probability change for an event.
// Changes are identified when probability movements exceed a configurable
// threshold within a specified time window.
//
// Each Change captures the magnitude and direction of the probability shift,
// along with the old and new probabilities, enabling users to understand
// market sentiment changes over time.
type Change struct {
	ID              string        `json:"id"`
	EventID         string        `json:"event_id"`          // Composite ID: "EventID:MarketID"
	OriginalEventID string        `json:"original_event_id"` // Original Polymarket event ID (before composite)
	EventQuestion   string        `json:"event_question"`
	EventURL        string        `json:"event_url"`       // URL to Polymarket event page
	MarketID        string        `json:"market_id"`       // Market ID (for multi-market events)
	MarketQuestion  string        `json:"market_question"` // Specific market question (if multi-market)
	Magnitude       float64       `json:"magnitude"`       // Absolute probability change (0.0 to 1.0)
	Direction       string        `json:"direction"`       // "increase" or "decrease"
	OldProbability  float64       `json:"old_probability"`
	NewProbability  float64       `json:"new_probability"`
	TimeWindow      time.Duration `json:"time_window"` // Duration over which change was detected
	DetectedAt      time.Time     `json:"detected_at"`
	Notified        bool          `json:"notified"`               // Whether notification was sent
	SignalScore     float64       `json:"signal_score,omitempty"` // composite score from scoring algorithm; 0 = unscored
}

// EventGroup represents a collection of related markets from the same prediction
// market event. Markets are grouped so that multiple markets from the same event
// consume only one slot in top-k notifications.
type EventGroup struct {
	EventID       string   // Original Polymarket event ID
	EventQuestion string   // Event title / question
	EventURL      string   // URL to Polymarket event page
	BestScore     float64  // Highest signal score among markets in this group
	Markets       []Change // Individual market changes, sorted by score desc
}

// Validate checks that all change fields are valid
func (c *Change) Validate() error {
	if c.ID == "" {
		return errors.New("change ID must not be empty")
	}
	if c.EventID == "" {
		return errors.New("event ID must not be empty")
	}
	if c.Magnitude < 0.0 || c.Magnitude > 1.0 {
		return errors.New("magnitude must be between 0.0 and 1.0")
	}

	// Verify magnitude equals absolute difference
	expectedMagnitude := math.Abs(c.NewProbability - c.OldProbability)
	if math.Abs(c.Magnitude-expectedMagnitude) > 0.001 {
		return errors.New("magnitude must equal |new_probability - old_probability|")
	}

	if c.Direction != "increase" && c.Direction != "decrease" {
		return errors.New("direction must be 'increase' or 'decrease'")
	}
	if c.OldProbability < 0.0 || c.OldProbability > 1.0 {
		return errors.New("old probability must be between 0.0 and 1.0")
	}
	if c.NewProbability < 0.0 || c.NewProbability > 1.0 {
		return errors.New("new probability must be between 0.0 and 1.0")
	}
	if c.DetectedAt.After(time.Now()) {
		return errors.New("detected at must not be in the future")
	}
	return nil
}
