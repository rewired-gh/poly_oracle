// Package models defines the core domain entities for the poly-oracle application.
// These models represent prediction market events, probability snapshots, and detected changes.
// All models include built-in validation to ensure data integrity throughout the application.
package models

import (
	"errors"
	"time"
)

// Event represents a prediction market event being monitored from Polymarket.
// Each event contains probability data, volume metrics, and metadata used for
// tracking significant market movements over time.
//
// Events can have multiple underlying markets. When an event has multiple markets,
// each market is tracked independently by using a composite ID (EventID:MarketID).
// This allows change detection at the individual market level.
type Event struct {
	ID             string    `json:"id"`              // Composite ID: "EventID:MarketID" for multi-market events
	EventID        string    `json:"event_id"`        // Original Polymarket event ID
	MarketID       string    `json:"market_id"`       // Market ID (empty for single-market events)
	MarketQuestion string    `json:"market_question"` // Specific market question (if multi-market)
	Title          string    `json:"title"`           // Event title (from Polymarket API)
	EventURL       string    `json:"event_url"`       // URL to Polymarket event page
	Description    string    `json:"description,omitempty"`
	Category       string    `json:"category"`
	Subcategory    string    `json:"subcategory,omitempty"`
	YesProbability float64   `json:"yes_probability"` // Yes probability for this specific market
	NoProbability  float64   `json:"no_probability"`  // No probability for this specific market
	Volume24hr     float64   `json:"volume_24hr"`     // 24-hour volume in USD (event-level)
	Volume1wk      float64   `json:"volume_1wk"`      // 1-week volume in USD (event-level)
	Volume1mo      float64   `json:"volume_1mo"`      // 1-month volume in USD (event-level)
	Liquidity      float64   `json:"liquidity"`       // Current liquidity in USD (event-level)
	Active         bool      `json:"active"`
	Closed         bool      `json:"closed"`
	LastUpdated    time.Time `json:"last_updated"`
	CreatedAt      time.Time `json:"created_at"`
}

// Validate checks that all event fields are valid
func (e *Event) Validate() error {
	if e.ID == "" {
		return errors.New("event ID must not be empty")
	}
	if e.EventID == "" {
		return errors.New("original event ID must not be empty")
	}
	if e.Title == "" {
		return errors.New("event title must not be empty")
	}
	if e.Category == "" {
		return errors.New("event category must not be empty")
	}
	if e.YesProbability < 0.0 || e.YesProbability > 1.0 {
		return errors.New("yes probability must be between 0.0 and 1.0")
	}
	if e.NoProbability < 0.0 || e.NoProbability > 1.0 {
		return errors.New("no probability must be between 0.0 and 1.0")
	}
	// Allow small tolerance for sum != 1.0 due to floating point precision
	sum := e.YesProbability + e.NoProbability
	if sum < 0.99 || sum > 1.01 {
		return errors.New("yes + no probability should approximately equal 1.0")
	}
	if e.Volume24hr < 0 {
		return errors.New("volume 24hr must not be negative")
	}
	if e.Volume1wk < 0 {
		return errors.New("volume 1wk must not be negative")
	}
	if e.Volume1mo < 0 {
		return errors.New("volume 1mo must not be negative")
	}
	if e.Liquidity < 0 {
		return errors.New("liquidity must not be negative")
	}
	if e.LastUpdated.After(time.Now()) {
		return errors.New("last updated must not be in the future")
	}
	if e.CreatedAt.After(e.LastUpdated) {
		return errors.New("created at must be <= last updated")
	}
	return nil
}
