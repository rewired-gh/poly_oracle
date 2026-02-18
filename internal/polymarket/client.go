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
	"strings"
	"time"

	"github.com/rewired-gh/polyoracle/internal/models"
)

// Client provides access to Polymarket API
type Client struct {
	gammaAPIURL    string
	clobAPIURL     string
	httpClient     *http.Client
	timeout        time.Duration
	maxRetries     int
	retryDelayBase time.Duration
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
	ID            string  `json:"id"`
	ConditionID   string  `json:"conditionId"`
	Question      string  `json:"question"`
	Outcomes      string  `json:"outcomes"`      // JSON string: "[\"Yes\", \"No\"]"
	OutcomePrices string  `json:"outcomePrices"` // JSON string: "[\"0.75\", \"0.25\"]"
	ClobTokenIds  string  `json:"clobTokenIds"`  // JSON string: "[\"token1\", \"token2\"]"
	Volume        string  `json:"volume"`        // Total volume (string in API)
	Volume1wk     float64 `json:"volume1wk"`     // 1-week volume (number in API)
	Volume1mo     float64 `json:"volume1mo"`     // 1-month volume (number in API)
}

// ClientConfig holds optional configuration for the Polymarket client
type ClientConfig struct {
	MaxRetries          int
	RetryDelayBase      time.Duration
	MaxIdleConns        int
	MaxIdleConnsPerHost int
	IdleConnTimeout     time.Duration
}

// NewClient creates a new Polymarket client
func NewClient(gammaAPIURL, clobAPIURL string, timeout time.Duration, cfg ...ClientConfig) *Client {
	var maxRetries = 3
	var retryDelayBase = time.Second
	var maxIdleConns = 100
	var maxIdleConnsPerHost = 10
	var idleConnTimeout = 90 * time.Second

	if len(cfg) > 0 {
		if cfg[0].MaxRetries > 0 {
			maxRetries = cfg[0].MaxRetries
		}
		if cfg[0].RetryDelayBase > 0 {
			retryDelayBase = cfg[0].RetryDelayBase
		}
		if cfg[0].MaxIdleConns > 0 {
			maxIdleConns = cfg[0].MaxIdleConns
		}
		if cfg[0].MaxIdleConnsPerHost > 0 {
			maxIdleConnsPerHost = cfg[0].MaxIdleConnsPerHost
		}
		if cfg[0].IdleConnTimeout > 0 {
			idleConnTimeout = cfg[0].IdleConnTimeout
		}
	}

	return &Client{
		gammaAPIURL: gammaAPIURL,
		clobAPIURL:  clobAPIURL,
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        maxIdleConns,
				MaxIdleConnsPerHost: maxIdleConnsPerHost,
				IdleConnTimeout:     idleConnTimeout,
				TLSHandshakeTimeout: 10 * time.Second,
			},
		},
		timeout:        timeout,
		maxRetries:     maxRetries,
		retryDelayBase: retryDelayBase,
	}
}

// FetchEvents retrieves events from Polymarket Gamma API with filtering
// Filter order: 1) categories, 2) top K by volume (logical OR), 3) then detect changes
// Uses pagination to fetch events beyond the API's 500 per-request limit.
func (c *Client) FetchEvents(ctx context.Context, categories []string, vol24hrMin, vol1wkMin, vol1moMin float64, volumeFilterOR bool, limit int) ([]models.Market, error) {
	// Filter by categories
	categoryMap := make(map[string]bool)
	for _, cat := range categories {
		categoryMap[cat] = true
	}

	var allEvents []models.Market
	const pageSize = 500 // API max per request
	maxFetch := limit * 3

	// Paginate through results
	for offset := 0; offset < maxFetch; offset += pageSize {
		// Build URL with query parameters
		u, err := url.Parse(c.gammaAPIURL + "/events")
		if err != nil {
			return nil, fmt.Errorf("failed to parse URL: %w", err)
		}

		q := u.Query()
		q.Set("active", "true")
		q.Set("closed", "false")
		q.Set("limit", fmt.Sprintf("%d", pageSize))
		q.Set("offset", fmt.Sprintf("%d", offset))

		// Sort by volume24hr descending (one of the volume metrics)
		q.Set("order", "volume24hr")
		q.Set("ascending", "false")

		u.RawQuery = q.Encode()

		resp, err := c.doRequest(ctx, u.String())
		if err != nil {
			return nil, fmt.Errorf("failed to fetch events from %s: %w", u.String(), err)
		}

		// Validate content type
		contentType := resp.Header.Get("Content-Type")
		if contentType != "" && contentType != "application/json" && !containsJSON(contentType) {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("unexpected content type: %s (expected application/json)", contentType)
		}

		// Response is array directly, not wrapped
		var pmEvents []PolymarketEvent
		if err := json.NewDecoder(resp.Body).Decode(&pmEvents); err != nil {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("failed to decode events JSON: %w", err)
		}
		_ = resp.Body.Close()

		// No more events
		if len(pmEvents) == 0 {
			break
		}

		// Process events from this page
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

				// Always use composite ID format for consistency
				// This prevents data loss when events transition from single to multi-market
				compositeID := pe.ID + ":" + market.ID

				// Use market-level volume for scoring accuracy in multi-market events
				// Markets have volume1wk/volume1mo but not volume24hr
				// Estimate volume24hr proportionally based on market's share of event's weekly volume
				marketVolume1wk := market.Volume1wk
				marketVolume1mo := market.Volume1mo
				marketVolume24hr := pe.Volume24hr // fallback to event-level

				// Proportionally estimate 24hr volume from market's share of weekly volume
				if pe.Volume1wk > 0 && marketVolume1wk > 0 {
					marketShare := marketVolume1wk / pe.Volume1wk
					marketVolume24hr = pe.Volume24hr * marketShare
				}

				event := models.Market{
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
					Volume24hr:     marketVolume24hr,
					Volume1wk:      marketVolume1wk,
					Volume1mo:      marketVolume1mo,
					Liquidity:      pe.Liquidity,
					Active:         pe.Active && !pe.Closed,
					LastUpdated:    now,
					CreatedAt:      now,
				}

				allEvents = append(allEvents, event)
			}
		}

		// Stop if we got fewer than pageSize (last page)
		if len(pmEvents) < pageSize {
			break
		}

		// Stop if we have enough events
		if len(allEvents) >= maxFetch {
			break
		}
	}

	// Return top K after filtering
	if len(allEvents) > limit {
		allEvents = allEvents[:limit]
	}

	return allEvents, nil
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
		if _, err := fmt.Sscanf(outcomePrices[i], "%f", &price); err != nil {
			return 0, 0, fmt.Errorf("failed to parse price '%s': %w", outcomePrices[i], err)
		}

		switch outcome {
		case "Yes":
			yesProb = price
		case "No":
			noProb = price
		}
	}

	return yesProb, noProb, nil
}

// containsJSON checks if a content-type header indicates JSON
func containsJSON(contentType string) bool {
	return contentType == "application/json" ||
		strings.HasPrefix(contentType, "application/json;")
}

// doRequest performs HTTP request with retry logic
func (c *Client) doRequest(ctx context.Context, urlStr string) (*http.Response, error) {
	var lastErr error

	for i := 0; i < c.maxRetries; i++ {
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
			case <-time.After(c.retryDelayBase * time.Duration(i+1)):
				continue
			}
		}

		// Handle various HTTP status codes
		if resp.StatusCode >= 500 {
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("server error (status %d): %s", resp.StatusCode, resp.Status)
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("request cancelled during retry: %w", ctx.Err())
			case <-time.After(c.retryDelayBase * time.Duration(i+1)):
				continue
			}
		}

		if resp.StatusCode >= 400 {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("client error (status %d): %s", resp.StatusCode, resp.Status)
		}

		return resp, nil
	}

	return nil, fmt.Errorf("max retries (%d) exceeded: %w", c.maxRetries, lastErr)
}
