// Package models defines the core domain entities for the poly-oracle application.
// These models represent prediction markets, probability snapshots, and detected changes.
// All models include built-in validation to ensure data integrity throughout the application.
//
// Terminology (matching Polymarket's own naming):
//   - Event: a Polymarket event page, which groups one or more related markets.
//   - Market: a single yes/no question within an event. This is the unit we track.
package models

import (
	"errors"
	"time"
)

// Market represents a single yes/no prediction market being monitored from Polymarket.
// Each market belongs to a parent Polymarket event (identified by EventID).
// Probability data, volume metrics, and metadata are used to detect significant moves.
//
// When a Polymarket event has multiple markets, each market is tracked independently
// using a composite ID (EventID:MarketID), allowing per-market change detection.
type Market struct {
	ID             string    `json:"id"`              // Composite ID: "EventID:MarketID"
	EventID        string    `json:"event_id"`        // Parent Polymarket event ID
	MarketID       string    `json:"market_id"`       // Polymarket market ID
	MarketQuestion string    `json:"market_question"` // Yes/no question for this market
	Title          string    `json:"title"`           // Parent event title (from Polymarket API)
	EventURL       string    `json:"event_url"`       // URL to the parent Polymarket event page
	Description    string    `json:"description,omitempty"`
	Category       string    `json:"category"`
	Subcategory    string    `json:"subcategory,omitempty"`
	YesProbability float64   `json:"yes_probability"` // Current Yes probability (0–1)
	NoProbability  float64   `json:"no_probability"`  // Current No probability (0–1)
	Volume24hr     float64   `json:"volume_24hr"`     // Estimated 24-hour volume in USD (proportionally allocated from event)
	Volume1wk      float64   `json:"volume_1wk"`      // 1-week volume in USD (market-level from API)
	Volume1mo      float64   `json:"volume_1mo"`      // 1-month volume in USD (market-level from API)
	Liquidity      float64   `json:"liquidity"`       // Current liquidity in USD (event-level)
	Active         bool      `json:"active"`
	Closed         bool      `json:"closed"`
	LastUpdated    time.Time `json:"last_updated"`
	CreatedAt      time.Time `json:"created_at"`
}

// Validate checks that all market fields are valid.
func (m *Market) Validate() error {
	if m.ID == "" {
		return errors.New("market ID must not be empty")
	}
	if m.EventID == "" {
		return errors.New("event ID must not be empty")
	}
	if m.Title == "" {
		return errors.New("event title must not be empty")
	}
	if m.Category == "" {
		return errors.New("market category must not be empty")
	}
	if m.YesProbability < 0.0 || m.YesProbability > 1.0 {
		return errors.New("yes probability must be between 0.0 and 1.0")
	}
	if m.NoProbability < 0.0 || m.NoProbability > 1.0 {
		return errors.New("no probability must be between 0.0 and 1.0")
	}
	// Allow small tolerance for sum != 1.0 due to floating point precision
	sum := m.YesProbability + m.NoProbability
	if sum < 0.99 || sum > 1.01 {
		return errors.New("yes + no probability should approximately equal 1.0")
	}
	if m.Volume24hr < 0 {
		return errors.New("volume 24hr must not be negative")
	}
	if m.Volume1wk < 0 {
		return errors.New("volume 1wk must not be negative")
	}
	if m.Volume1mo < 0 {
		return errors.New("volume 1mo must not be negative")
	}
	if m.Liquidity < 0 {
		return errors.New("liquidity must not be negative")
	}
	if m.LastUpdated.After(time.Now()) {
		return errors.New("last updated must not be in the future")
	}
	if m.CreatedAt.After(m.LastUpdated) {
		return errors.New("created at must be <= last updated")
	}
	return nil
}
