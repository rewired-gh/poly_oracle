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
	Title       string             `json:"title"`
	Subtitle    string             `json:"subtitle"`
	Description string             `json:"description"`
	Category    string             `json:"category"`
	Subcategory string             `json:"subcategory"`
	Active      bool               `json:"active"`
	Closed      bool               `json:"closed"`
	Volume      float64            `json:"volume"`
	Volume24hr  float64            `json:"volume24hr"`
	Volume1wk   float64            `json:"volume1wk"`
	Volume1mo   float64            `json:"volume1mo"`
	Liquidity   float64            `json:"liquidity"`
	Markets     []PolymarketMarket `json:"markets"`
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
		return nil, fmt.Errorf("failed to fetch events: %w", err)
	}
	defer resp.Body.Close()

	// Response is array directly, not wrapped
	var pmEvents []PolymarketEvent
	if err := json.NewDecoder(resp.Body).Decode(&pmEvents); err != nil {
		return nil, fmt.Errorf("failed to decode events: %w", err)
	}

	// Filter by categories
	categoryMap := make(map[string]bool)
	for _, cat := range categories {
		categoryMap[cat] = true
	}

	var events []models.Event
	for _, pe := range pmEvents {
		// Filter by category if specified
		if len(categories) > 0 && !categoryMap[pe.Category] {
			continue
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

		// Extract probabilities from markets
		// One event can have multiple markets - select max probability change across all markets
		var maxYesProb, maxNoProb float64
		for _, market := range pe.Markets {
			yesProb, noProb, err := parseMarketProbabilities(market)
			if err != nil {
				continue // Skip invalid markets
			}

			// Track maximum probabilities across all markets
			if yesProb > maxYesProb {
				maxYesProb = yesProb
			}
			if noProb > maxNoProb {
				maxNoProb = noProb
			}
		}

		// Skip events with no valid market data
		if maxYesProb == 0 && maxNoProb == 0 {
			continue
		}

		event := models.Event{
			ID:             pe.ID,
			Title:          pe.Title,
			Description:    pe.Description,
			Category:       pe.Category,
			Subcategory:    pe.Subcategory,
			YesProbability: maxYesProb,
			NoProbability:  maxNoProb,
			Volume24hr:     pe.Volume24hr,
			Volume1wk:      pe.Volume1wk,
			Volume1mo:      pe.Volume1mo,
			Liquidity:      pe.Liquidity,
			Active:         pe.Active && !pe.Closed,
			LastUpdated:    time.Now(),
			CreatedAt:      time.Now(),
		}

		events = append(events, event)
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

		if outcome == "Yes" {
			yesProb = price
		} else if outcome == "No" {
			noProb = price
		}
	}

	return yesProb, noProb, nil
}

// doRequest performs HTTP request with retry logic
func (c *Client) doRequest(ctx context.Context, urlStr string) (*http.Response, error) {
	maxRetries := 3
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
		if err != nil {
			return nil, err
		}

		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(i+1) * time.Second)
			continue
		}

		if resp.StatusCode >= 500 {
			resp.Body.Close()
			lastErr = fmt.Errorf("server error: %d", resp.StatusCode)
			time.Sleep(time.Duration(i+1) * time.Second)
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}
