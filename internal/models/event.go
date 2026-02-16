package models

import (
	"errors"
	"time"
)

// Event represents a prediction market event being monitored from Polymarket
type Event struct {
	ID             string    `json:"id"`
	Title          string    `json:"title"` // Event title (from Polymarket API)
	Description    string    `json:"description,omitempty"`
	Category       string    `json:"category"`
	Subcategory    string    `json:"subcategory,omitempty"`
	YesProbability float64   `json:"yes_probability"` // Maximum Yes probability across all markets
	NoProbability  float64   `json:"no_probability"`  // Maximum No probability across all markets
	Volume24hr     float64   `json:"volume_24hr"`     // 24-hour volume
	Volume1wk      float64   `json:"volume_1wk"`      // 1-week volume
	Volume1mo      float64   `json:"volume_1mo"`      // 1-month volume
	Liquidity      float64   `json:"liquidity"`       // Current liquidity
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
