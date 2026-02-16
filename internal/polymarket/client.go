// Package polymarket provides a client for interacting with Polymarket APIs.
// It fetches prediction market events from the Gamma API and extracts probability
// data for monitoring purposes.
//
// The client includes built-in retry logic, timeout handling, and context
// cancellation support for robust API interactions.
package polymarket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/poly-oracle/internal/models"
)

// Client provides access to Polymarket API
type Client struct {
	gammaAPIURL string
	clobAPIURL  string
	httpClient  *http.Client
	timeout     time.Duration
}

// PolymarketEvent represents an event from Polymarket Gamma API
type PolymarketEvent struct {
	ID          string             `json:"id"`
	Ticker      string             `json:"ticker"`
	Slug        string             `json:"slug"` // Event slug for URL construction
	Title       string             `json:"title"`
	Subtitle    string             `json:"subtitle"`
	Description string             `json:"description"`
	Category    string             `json:"category"`    // Often null in API response
	Subcategory string             `json:"subcategory"` // Often null in API response
	Active      bool               `json:"active"`
	Closed      bool               `json:"closed"`
	Volume      float64            `json:"volume"`
	Volume24hr  float64            `json:"volume24hr"`
	Volume1wk   float64            `json:"volume1wk"`
	Volume1mo   float64            `json:"volume1mo"`
	Liquidity   float64            `json:"liquidity"`
	Markets     []PolymarketMarket `json:"markets"`
	Tags        []PolymarketTag    `json:"tags"` // Actual category information is here
}

// PolymarketTag represents a tag from Polymarket API (contains actual category info)
type PolymarketTag struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Slug  string `json:"slug"` // This is the category identifier
}

// PolymarketMarket represents a market from Polymarket API
type PolymarketMarket struct {
	ID            string `json:"id"`
	ConditionID   string `json:"conditionId"`
	Question      string `json:"question"`
	Outcomes      string `json:"outcomes"`      // JSON string: "[\"Yes\", \"No\"]"
	OutcomePrices string `json:"outcomePrices"` // JSON string: "[\"0.75\", \"0.25\"]"
	ClobTokenIds  string `json:"clobTokenIds"`  // JSON string: "[\"token1\", \"token2\"]"
}

// NewClient creates a new Polymarket client
func NewClient(gammaAPIURL, clobAPIURL string, timeout time.Duration) *Client {
	return &Client{
		gammaAPIURL: gammaAPIURL,
		clobAPIURL:  clobAPIURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
	}
}

// FetchEvents retrieves events from Polymarket Gamma API with filtering
// Filter order: 1) categories, 2) top K by volume (logical OR), 3) then detect changes
func (c *Client) FetchEvents(ctx context.Context, categories []string, vol24hrMin, vol1wkMin, vol1moMin float64, volumeFilterOR bool, limit int) ([]models.Event, error) {
	// Build URL with query parameters
	u, err := url.Parse(c.gammaAPIURL + "/events")
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	q := u.Query()
	q.Set("active", "true")
	q.Set("closed", "false")
	q.Set("limit", fmt.Sprintf("%d", limit*3)) // Fetch 3x to allow for filtering

	// Sort by volume24hr descending (one of the volume metrics)
	q.Set("order", "volume24hr")
	q.Set("ascending", "false")

	u.RawQuery = q.Encode()

	resp, err := c.doRequest(ctx, u.String())
	if err != nil {
		return nil, fmt.Errorf("failed to fetch events from %s: %w", u.String(), err)
	}
	defer resp.Body.Close()

	// Validate content type
	contentType := resp.Header.Get("Content-Type")
	if contentType != "" && contentType != "application/json" && !containsJSON(contentType) {
		return nil, fmt.Errorf("unexpected content type: %s (expected application/json)", contentType)
	}

	// Response is array directly, not wrapped
	var pmEvents []PolymarketEvent
	if err := json.NewDecoder(resp.Body).Decode(&pmEvents); err != nil {
		return nil, fmt.Errorf("failed to decode events JSON: %w", err)
	}

	if len(pmEvents) == 0 {
		return []models.Event{}, nil
	}

	// Filter by categories
	categoryMap := make(map[string]bool)
	for _, cat := range categories {
		categoryMap[cat] = true
	}

	var events []models.Event
	for _, pe := range pmEvents {
		// Filter by category using tags (category field is often null in API)
		if len(categories) > 0 {
			// Check if any tag matches the requested categories
			tagMatch := false
			for _, tag := range pe.Tags {
				if categoryMap[tag.Slug] {
					tagMatch = true
					break
				}
			}
			if !tagMatch {
				continue
			}
		}

		// Apply volume filtering (logical OR or AND)
		if vol24hrMin > 0 || vol1wkMin > 0 || vol1moMin > 0 {
			vol24hrPass := pe.Volume24hr >= vol24hrMin
			vol1wkPass := pe.Volume1wk >= vol1wkMin
			vol1moPass := pe.Volume1mo >= vol1moMin

			if volumeFilterOR {
				// Logical OR: include if ANY condition passes
				if !vol24hrPass && !vol1wkPass && !vol1moPass {
					continue
				}
			} else {
				// Logical AND: include if ALL conditions pass
				if !vol24hrPass || !vol1wkPass || !vol1moPass {
					continue
				}
			}
		}

		// Extract primary category from tags (first matching tag or first tag overall)
		primaryCategory := ""
		if len(pe.Tags) > 0 {
			// Try to find a tag that matches our filter categories
			for _, tag := range pe.Tags {
				if categoryMap[tag.Slug] {
					primaryCategory = tag.Slug
					break
				}
			}
			// If no match found, use the first tag
			if primaryCategory == "" {
				primaryCategory = pe.Tags[0].Slug
			}
		}

		// Process each market individually
		// An event can have multiple markets, and we track each one separately
		for _, market := range pe.Markets {
			yesProb, noProb, err := parseMarketProbabilities(market)
			if err != nil {
				continue // Skip invalid markets
			}

			// Skip markets with no valid probability data
			if yesProb == 0 && noProb == 0 {
				continue
			}

			// Capture current time once to ensure CreatedAt <= LastUpdated
			now := time.Now()

			// Create composite ID for multi-market tracking
			// If only one market, use event ID directly; otherwise use "EventID:MarketID"
			compositeID := pe.ID
			if len(pe.Markets) > 1 {
				compositeID = pe.ID + ":" + market.ID
			}

			event := models.Event{
				ID:             compositeID,
				EventID:        pe.ID,
				MarketID:       market.ID,
				MarketQuestion: market.Question,
				Title:          pe.Title,
				EventURL:       "https://polymarket.com/event/" + pe.Slug,
				Description:    pe.Description,
				Category:       primaryCategory,
				Subcategory:    pe.Subcategory,
				YesProbability: yesProb,
				NoProbability:  noProb,
				Volume24hr:     pe.Volume24hr,
				Volume1wk:      pe.Volume1wk,
				Volume1mo:      pe.Volume1mo,
				Liquidity:      pe.Liquidity,
				Active:         pe.Active && !pe.Closed,
				LastUpdated:    now,
				CreatedAt:      now,
			}

			events = append(events, event)
		}
	}

	// Return top K after filtering
	if len(events) > limit {
		events = events[:limit]
	}

	return events, nil
}

// parseMarketProbabilities extracts Yes/No probabilities from a market
func parseMarketProbabilities(market PolymarketMarket) (float64, float64, error) {
	// Parse outcomes JSON string
	var outcomes []string
	if err := json.Unmarshal([]byte(market.Outcomes), &outcomes); err != nil {
		return 0, 0, fmt.Errorf("failed to parse outcomes: %w", err)
	}

	// Parse outcome prices JSON string
	var outcomePrices []string
	if err := json.Unmarshal([]byte(market.OutcomePrices), &outcomePrices); err != nil {
		return 0, 0, fmt.Errorf("failed to parse outcome prices: %w", err)
	}

	// Extract Yes/No probabilities
	var yesProb, noProb float64
	for i, outcome := range outcomes {
		if i >= len(outcomePrices) {
			break
		}

		var price float64
		fmt.Sscanf(outcomePrices[i], "%f", &price)

		switch outcome {
		case "Yes":
			yesProb = price
		case "No":
			noProb = price
		}
	}

	return yesProb, noProb, nil
}

// containsJSON checks if a content-type string contains json
func containsJSON(contentType string) bool {
	return len(contentType) >= 16 && contentType[:16] == "application/json" ||
		len(contentType) > 16 && contentType[:17] == "application/json;"
}

// doRequest performs HTTP request with retry logic
func (c *Client) doRequest(ctx context.Context, urlStr string) (*http.Response, error) {
	maxRetries := 3
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		// Check if context is cancelled before making request
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("request cancelled: %w", ctx.Err())
		default:
		}

		req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			// Exponential backoff with context check
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("request cancelled during retry: %w", ctx.Err())
			case <-time.After(time.Duration(i+1) * time.Second):
				continue
			}
		}

		// Handle various HTTP status codes
		if resp.StatusCode >= 500 {
			resp.Body.Close()
			lastErr = fmt.Errorf("server error (status %d): %s", resp.StatusCode, resp.Status)
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("request cancelled during retry: %w", ctx.Err())
			case <-time.After(time.Duration(i+1) * time.Second):
				continue
			}
		}

		if resp.StatusCode >= 400 {
			resp.Body.Close()
			return nil, fmt.Errorf("client error (status %d): %s", resp.StatusCode, resp.Status)
		}

		return resp, nil
	}

	return nil, fmt.Errorf("max retries (%d) exceeded: %w", maxRetries, lastErr)
}
