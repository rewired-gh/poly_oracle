// Package models defines the core domain entities: markets, alerts, and event groups.
package models

import (
	"errors"
	"time"
)

// Market represents a single yes/no prediction market tracked from Polymarket.
// Uses composite ID format "EventID:MarketID" for per-market change detection.
type Market struct {
	ID             string    `json:"id"`
	EventID        string    `json:"event_id"`
	MarketID       string    `json:"market_id"`
	MarketQuestion string    `json:"market_question"`
	Title          string    `json:"title"`
	EventURL       string    `json:"event_url"`
	Description    string    `json:"description,omitempty"`
	Category       string    `json:"category"`
	Subcategory    string    `json:"subcategory,omitempty"`
	YesProbability float64   `json:"yes_probability"`
	NoProbability  float64   `json:"no_probability"`
	Volume24hr     float64   `json:"volume_24hr"`
	Volume1wk      float64   `json:"volume_1wk"`
	Volume1mo      float64   `json:"volume_1mo"`
	Liquidity      float64   `json:"liquidity"`
	Active         bool      `json:"active"`
	Closed         bool      `json:"closed"`
	LastUpdated    time.Time `json:"last_updated"`
	CreatedAt      time.Time `json:"created_at"`
}

// Validate checks market field constraints.
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
