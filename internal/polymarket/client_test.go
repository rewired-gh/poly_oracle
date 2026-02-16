package polymarket

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchEvents_RealAPIFormat(t *testing.T) {
	// Create a mock server that returns data in real Polymarket API format
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request parameters
		if r.URL.Path != "/events" {
			t.Errorf("Expected path /events, got %s", r.URL.Path)
		}

		// Check query parameters
		query := r.URL.Query()
		if query.Get("active") != "true" {
			t.Errorf("Expected active=true, got %s", query.Get("active"))
		}
		if query.Get("closed") != "false" {
			t.Errorf("Expected closed=false, got %s", query.Get("closed"))
		}

		// Return mock data in real API format
		// Note: category field is null, actual categories are in tags array
		// Note: outcomes and outcomePrices are JSON STRINGS, not arrays
		events := []PolymarketEvent{
			{
				ID:          "event-1",
				Title:       "Will candidate X win the election?",
				Description: "Test event description",
				Category:    "", // Often null in real API
				Subcategory: "",
				Active:      true,
				Closed:      false,
				Volume24hr:  50000.0,
				Volume1wk:   350000.0,
				Volume1mo:   1500000.0,
				Liquidity:   100000.0,
				Markets: []PolymarketMarket{
					{
						ID:            "market-1",
						ConditionID:   "condition-1",
						Question:      "Will candidate X win?",
						Outcomes:      "[\"Yes\", \"No\"]",
						OutcomePrices: "[\"0.75\", \"0.25\"]",
						ClobTokenIds:  "[\"token1\", \"token2\"]",
					},
				},
				Tags: []PolymarketTag{
					{ID: "1", Label: "Politics", Slug: "politics"},
					{ID: "2", Label: "Elections", Slug: "elections"},
				},
			},
			{
				ID:          "event-2",
				Title:       "Will team Y win the championship?",
				Description: "Sports event",
				Category:    "", // Often null in real API
				Subcategory: "",
				Active:      true,
				Closed:      false,
				Volume24hr:  25000.0,
				Volume1wk:   175000.0,
				Volume1mo:   750000.0,
				Liquidity:   50000.0,
				Markets: []PolymarketMarket{
					{
						ID:            "market-2",
						ConditionID:   "condition-2",
						Question:      "Will team Y win?",
						Outcomes:      "[\"Yes\", \"No\"]",
						OutcomePrices: "[\"0.60\", \"0.40\"]",
						ClobTokenIds:  "[\"token3\", \"token4\"]",
					},
				},
				Tags: []PolymarketTag{
					{ID: "3", Label: "Sports", Slug: "sports"},
					{ID: "4", Label: "Basketball", Slug: "basketball"},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(events); err != nil {
			t.Errorf("Failed to encode events: %v", err)
		}
	}))
	defer mockServer.Close()

	// Create client with mock server URL
	client := NewClient(mockServer.URL, "https://clob.polymarket.com", 30*time.Second)

	// Test fetching events
	ctx := context.Background()
	events, err := client.FetchEvents(ctx, []string{"politics"}, 0, 0, 0, true, 10)
	if err != nil {
		t.Fatalf("FetchEvents failed: %v", err)
	}

	// Verify results
	if len(events) != 1 {
		t.Fatalf("Expected 1 event (politics category), got %d", len(events))
	}

	event := events[0]
	if event.EventID != "event-1" {
		t.Errorf("Expected event ID 'event-1', got '%s'", event.EventID)
	}
	if event.ID == "" {
		t.Errorf("Expected composite ID to be set, got empty string")
	}
	if event.Title != "Will candidate X win the election?" {
		t.Errorf("Expected title 'Will candidate X win the election?', got '%s'", event.Title)
	}
	if event.Category != "politics" {
		t.Errorf("Expected category 'politics', got '%s'", event.Category)
	}
	if event.YesProbability != 0.75 {
		t.Errorf("Expected yes probability 0.75, got %f", event.YesProbability)
	}
	if event.NoProbability != 0.25 {
		t.Errorf("Expected no probability 0.25, got %f", event.NoProbability)
	}
	if event.Volume24hr != 50000.0 {
		t.Errorf("Expected volume24hr 50000.0, got %f", event.Volume24hr)
	}
}

func TestFetchEvents_VolumeFilterOR(t *testing.T) {
	// Create a mock server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		events := []PolymarketEvent{
			{
				ID:         "event-1",
				Title:      "High 24hr volume",
				Category:   "", // Null in real API
				Active:     true,
				Closed:     false,
				Volume24hr: 50000.0, // Passes vol24hrMin
				Volume1wk:  1000.0,  // Fails vol1wkMin
				Volume1mo:  5000.0,  // Fails vol1moMin
				Markets: []PolymarketMarket{
					{
						Outcomes:      "[\"Yes\", \"No\"]",
						OutcomePrices: "[\"0.5\", \"0.5\"]",
					},
				},
				Tags: []PolymarketTag{
					{ID: "1", Label: "Politics", Slug: "politics"},
				},
			},
			{
				ID:         "event-2",
				Title:      "High 1wk volume",
				Category:   "", // Null in real API
				Active:     true,
				Closed:     false,
				Volume24hr: 5000.0,   // Fails vol24hrMin
				Volume1wk:  350000.0, // Passes vol1wkMin
				Volume1mo:  10000.0,  // Fails vol1moMin
				Markets: []PolymarketMarket{
					{
						Outcomes:      "[\"Yes\", \"No\"]",
						OutcomePrices: "[\"0.5\", \"0.5\"]",
					},
				},
				Tags: []PolymarketTag{
					{ID: "1", Label: "Politics", Slug: "politics"},
				},
			},
			{
				ID:         "event-3",
				Title:      "Low volume all",
				Category:   "", // Null in real API
				Active:     true,
				Closed:     false,
				Volume24hr: 1000.0,  // Fails vol24hrMin
				Volume1wk:  5000.0,  // Fails vol1wkMin
				Volume1mo:  10000.0, // Fails vol1moMin
				Markets: []PolymarketMarket{
					{
						Outcomes:      "[\"Yes\", \"No\"]",
						OutcomePrices: "[\"0.5\", \"0.5\"]",
					},
				},
				Tags: []PolymarketTag{
					{ID: "1", Label: "Politics", Slug: "politics"},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(events); err != nil {
			t.Errorf("Failed to encode events: %v", err)
		}
	}))
	defer mockServer.Close()

	client := NewClient(mockServer.URL, "https://clob.polymarket.com", 30*time.Second)

	// Test with volume filter OR (union)
	ctx := context.Background()
	events, err := client.FetchEvents(ctx, []string{"politics"}, 30000.0, 300000.0, 500000.0, true, 10)
	if err != nil {
		t.Fatalf("FetchEvents failed: %v", err)
	}

	// With OR logic, should get event-1 (passes 24hr) and event-2 (passes 1wk)
	// event-3 fails all conditions
	if len(events) != 2 {
		t.Errorf("Expected 2 events (OR logic), got %d", len(events))
	}
}

func TestFetchEvents_VolumeFilterAND(t *testing.T) {
	// Create a mock server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		events := []PolymarketEvent{
			{
				ID:         "event-1",
				Title:      "Passes all",
				Category:   "", // Null in real API
				Active:     true,
				Closed:     false,
				Volume24hr: 50000.0,
				Volume1wk:  350000.0,
				Volume1mo:  1500000.0,
				Markets: []PolymarketMarket{
					{
						Outcomes:      "[\"Yes\", \"No\"]",
						OutcomePrices: "[\"0.5\", \"0.5\"]",
					},
				},
				Tags: []PolymarketTag{
					{ID: "1", Label: "Politics", Slug: "politics"},
				},
			},
			{
				ID:         "event-2",
				Title:      "Fails one",
				Category:   "", // Null in real API
				Active:     true,
				Closed:     false,
				Volume24hr: 5000.0, // Fails 24hr min
				Volume1wk:  350000.0,
				Volume1mo:  1500000.0,
				Markets: []PolymarketMarket{
					{
						Outcomes:      "[\"Yes\", \"No\"]",
						OutcomePrices: "[\"0.5\", \"0.5\"]",
					},
				},
				Tags: []PolymarketTag{
					{ID: "1", Label: "Politics", Slug: "politics"},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(events); err != nil {
			t.Errorf("Failed to encode events: %v", err)
		}
	}))
	defer mockServer.Close()

	client := NewClient(mockServer.URL, "https://clob.polymarket.com", 30*time.Second)

	// Test with volume filter AND (intersection)
	ctx := context.Background()
	events, err := client.FetchEvents(ctx, []string{"politics"}, 30000.0, 300000.0, 500000.0, false, 10)
	if err != nil {
		t.Fatalf("FetchEvents failed: %v", err)
	}

	// With AND logic, only event-1 passes all conditions
	if len(events) != 1 {
		t.Errorf("Expected 1 event (AND logic), got %d", len(events))
	}
	if len(events) > 0 && events[0].ID != "event-1" {
		t.Errorf("Expected event-1, got %s", events[0].ID)
	}
}

func TestFetchEvents_MultiMarketMaxProbability(t *testing.T) {
	// Test that multi-market events create separate event entries for each market
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		events := []PolymarketEvent{
			{
				ID:         "event-1",
				Title:      "Multi-market event",
				Category:   "", // Null in real API
				Active:     true,
				Closed:     false,
				Volume24hr: 50000.0,
				Markets: []PolymarketMarket{
					{
						ID:            "market-1",
						Question:      "Market 1",
						Outcomes:      "[\"Yes\", \"No\"]",
						OutcomePrices: "[\"0.60\", \"0.40\"]", // Yes=0.60, No=0.40
					},
					{
						ID:            "market-2",
						Question:      "Market 2",
						Outcomes:      "[\"Yes\", \"No\"]",
						OutcomePrices: "[\"0.75\", \"0.25\"]", // Yes=0.75, No=0.25
					},
					{
						ID:            "market-3",
						Question:      "Market 3",
						Outcomes:      "[\"Yes\", \"No\"]",
						OutcomePrices: "[\"0.55\", \"0.45\"]", // Yes=0.55, No=0.45
					},
				},
				Tags: []PolymarketTag{
					{ID: "1", Label: "Politics", Slug: "politics"},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(events); err != nil {
			t.Errorf("Failed to encode events: %v", err)
		}
	}))
	defer mockServer.Close()

	client := NewClient(mockServer.URL, "https://clob.polymarket.com", 30*time.Second)

	ctx := context.Background()
	events, err := client.FetchEvents(ctx, []string{"politics"}, 0, 0, 0, true, 10)
	if err != nil {
		t.Fatalf("FetchEvents failed: %v", err)
	}

	// Should get 3 events (one for each market)
	if len(events) != 3 {
		t.Fatalf("Expected 3 events (one per market), got %d", len(events))
	}

	// Verify each market is tracked separately
	// All should have same EventID but different MarketIDs and composite IDs
	if events[0].EventID != "event-1" || events[1].EventID != "event-1" || events[2].EventID != "event-1" {
		t.Errorf("All events should have EventID 'event-1'")
	}

	// Check composite IDs are unique
	ids := make(map[string]bool)
	for _, event := range events {
		if ids[event.ID] {
			t.Errorf("Duplicate composite ID: %s", event.ID)
		}
		ids[event.ID] = true
	}

	// Verify each market has correct probabilities
	// Market 1: Yes=0.60, No=0.40
	if events[0].YesProbability != 0.60 || events[0].NoProbability != 0.40 {
		t.Errorf("Market 1: Expected (0.60, 0.40), got (%.2f, %.2f)", events[0].YesProbability, events[0].NoProbability)
	}
	// Market 2: Yes=0.75, No=0.25
	if events[1].YesProbability != 0.75 || events[1].NoProbability != 0.25 {
		t.Errorf("Market 2: Expected (0.75, 0.25), got (%.2f, %.2f)", events[1].YesProbability, events[1].NoProbability)
	}
	// Market 3: Yes=0.55, No=0.45
	if events[2].YesProbability != 0.55 || events[2].NoProbability != 0.45 {
		t.Errorf("Market 3: Expected (0.55, 0.45), got (%.2f, %.2f)", events[2].YesProbability, events[2].NoProbability)
	}
}

func TestParseMarketProbabilities(t *testing.T) {
	tests := []struct {
		name        string
		market      PolymarketMarket
		expectedYes float64
		expectedNo  float64
		expectError bool
	}{
		{
			name: "Valid Yes/No market",
			market: PolymarketMarket{
				Outcomes:      "[\"Yes\", \"No\"]",
				OutcomePrices: "[\"0.75\", \"0.25\"]",
			},
			expectedYes: 0.75,
			expectedNo:  0.25,
			expectError: false,
		},
		{
			name: "Reversed order",
			market: PolymarketMarket{
				Outcomes:      "[\"No\", \"Yes\"]",
				OutcomePrices: "[\"0.25\", \"0.75\"]",
			},
			expectedYes: 0.75,
			expectedNo:  0.25,
			expectError: false,
		},
		{
			name: "Invalid outcomes JSON",
			market: PolymarketMarket{
				Outcomes:      "not valid json",
				OutcomePrices: "[\"0.75\", \"0.25\"]",
			},
			expectError: true,
		},
		{
			name: "Invalid prices JSON",
			market: PolymarketMarket{
				Outcomes:      "[\"Yes\", \"No\"]",
				OutcomePrices: "not valid json",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yes, no, err := parseMarketProbabilities(tt.market)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if yes != tt.expectedYes {
					t.Errorf("Expected yes=%f, got %f", tt.expectedYes, yes)
				}
				if no != tt.expectedNo {
					t.Errorf("Expected no=%f, got %f", tt.expectedNo, no)
				}
			}
		})
	}
}
