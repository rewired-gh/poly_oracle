// Package polymarket provides a client for interacting with Polymarket APIs.
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

// Client provides access to Polymarket API.
type Client struct {
	gammaAPIURL    string
	clobAPIURL     string // reserved for future CLOB integration
	httpClient     *http.Client
	timeout        time.Duration
	maxRetries     int
	retryDelayBase time.Duration
}

// PolymarketEvent represents an event from the Gamma API.
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

// PolymarketTag represents a tag from the API (contains actual category info).
type PolymarketTag struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Slug  string `json:"slug"`
}

// PolymarketMarket represents a market from the API.
type PolymarketMarket struct {
	ID            string  `json:"id"`
	ConditionID   string  `json:"conditionId"`
	Question      string  `json:"question"`
	Outcomes      string  `json:"outcomes"`      // JSON-encoded string array
	OutcomePrices string  `json:"outcomePrices"` // JSON-encoded string array
	ClobTokenIds  string  `json:"clobTokenIds"`  // JSON-encoded string array (for future CLOB use)
	Volume        string  `json:"volume"`
	Volume1wk     float64 `json:"volume1wk"`
	Volume1mo     float64 `json:"volume1mo"`
}

// ClientConfig holds optional transport/retry configuration.
type ClientConfig struct {
	MaxRetries          int
	RetryDelayBase      time.Duration
	MaxIdleConns        int
	MaxIdleConnsPerHost int
	IdleConnTimeout     time.Duration
}

// NewClient creates a new Polymarket client.
// The clobAPIURL is stored for future CLOB integration.
func NewClient(gammaAPIURL, clobAPIURL string, timeout time.Duration, cfg ...ClientConfig) *Client {
	maxRetries := 3
	retryDelayBase := time.Second
	maxIdleConns := 100
	maxIdleConnsPerHost := 10
	idleConnTimeout := 90 * time.Second

	if len(cfg) > 0 {
		c := cfg[0]
		if c.MaxRetries > 0 {
			maxRetries = c.MaxRetries
		}
		if c.RetryDelayBase > 0 {
			retryDelayBase = c.RetryDelayBase
		}
		if c.MaxIdleConns > 0 {
			maxIdleConns = c.MaxIdleConns
		}
		if c.MaxIdleConnsPerHost > 0 {
			maxIdleConnsPerHost = c.MaxIdleConnsPerHost
		}
		if c.IdleConnTimeout > 0 {
			idleConnTimeout = c.IdleConnTimeout
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

// FetchEvents retrieves events from the Gamma API with category and volume filtering.
// Uses pagination to fetch events beyond the API's 500 per-request limit.
func (c *Client) FetchEvents(ctx context.Context, categories []string, vol24hrMin, vol1wkMin, vol1moMin float64, volumeFilterOR bool, limit int) ([]models.Market, error) {
	categoryMap := make(map[string]bool)
	for _, cat := range categories {
		categoryMap[cat] = true
	}

	var allEvents []models.Market
	const pageSize = 500
	maxFetch := limit * 3

	for offset := 0; offset < maxFetch; offset += pageSize {
		u, err := url.Parse(c.gammaAPIURL + "/events")
		if err != nil {
			return nil, fmt.Errorf("failed to parse URL: %w", err)
		}

		q := u.Query()
		q.Set("active", "true")
		q.Set("closed", "false")
		q.Set("limit", fmt.Sprintf("%d", pageSize))
		q.Set("offset", fmt.Sprintf("%d", offset))
		q.Set("order", "volume24hr")
		q.Set("ascending", "false")

		u.RawQuery = q.Encode()

		resp, err := c.doRequest(ctx, u.String())
		if err != nil {
			return nil, fmt.Errorf("failed to fetch events from %s: %w", u.String(), err)
		}

		contentType := resp.Header.Get("Content-Type")
		if contentType != "" && contentType != "application/json" && !containsJSON(contentType) {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("unexpected content type: %s (expected application/json)", contentType)
		}

		var pmEvents []PolymarketEvent
		if err := json.NewDecoder(resp.Body).Decode(&pmEvents); err != nil {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("failed to decode events JSON: %w", err)
		}
		_ = resp.Body.Close()

		if len(pmEvents) == 0 {
			break
		}

		for _, pe := range pmEvents {
			// Filter by category using tags (category field is often null in API)
			if len(categories) > 0 {
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

			if vol24hrMin > 0 || vol1wkMin > 0 || vol1moMin > 0 {
				vol24hrPass := pe.Volume24hr >= vol24hrMin
				vol1wkPass := pe.Volume1wk >= vol1wkMin
				vol1moPass := pe.Volume1mo >= vol1moMin

				if volumeFilterOR {
					if !vol24hrPass && !vol1wkPass && !vol1moPass {
						continue
					}
				} else {
					if !vol24hrPass || !vol1wkPass || !vol1moPass {
						continue
					}
				}
			}

			primaryCategory := ""
			if len(pe.Tags) > 0 {
				for _, tag := range pe.Tags {
					if categoryMap[tag.Slug] {
						primaryCategory = tag.Slug
						break
					}
				}
				if primaryCategory == "" {
					primaryCategory = pe.Tags[0].Slug
				}
			}

			for _, market := range pe.Markets {
				yesProb, noProb, err := parseMarketProbabilities(market)
				if err != nil {
					continue
				}
				if yesProb == 0 && noProb == 0 {
					continue
				}

				now := time.Now()
				compositeID := pe.ID + ":" + market.ID

				// Estimate market-level 24hr volume proportionally from weekly share
				marketVolume1wk := market.Volume1wk
				marketVolume1mo := market.Volume1mo
				marketVolume24hr := pe.Volume24hr
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

		if len(pmEvents) < pageSize {
			break
		}

		if len(allEvents) >= maxFetch {
			break
		}
	}

	if len(allEvents) > limit {
		allEvents = allEvents[:limit]
	}

	return allEvents, nil
}

func parseMarketProbabilities(market PolymarketMarket) (float64, float64, error) {
	var outcomes []string
	if err := json.Unmarshal([]byte(market.Outcomes), &outcomes); err != nil {
		return 0, 0, fmt.Errorf("failed to parse outcomes: %w", err)
	}

	var outcomePrices []string
	if err := json.Unmarshal([]byte(market.OutcomePrices), &outcomePrices); err != nil {
		return 0, 0, fmt.Errorf("failed to parse outcome prices: %w", err)
	}

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

func containsJSON(contentType string) bool {
	return contentType == "application/json" ||
		strings.HasPrefix(contentType, "application/json;")
}

func (c *Client) doRequest(ctx context.Context, urlStr string) (*http.Response, error) {
	var lastErr error

	for i := 0; i < c.maxRetries; i++ {
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
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("request cancelled during retry: %w", ctx.Err())
			case <-time.After(c.retryDelayBase * time.Duration(i+1)):
				continue
			}
		}

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
