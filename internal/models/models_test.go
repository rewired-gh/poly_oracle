package models

import (
	"testing"
	"time"
)

func TestMarketValidate(t *testing.T) {
	tests := []struct {
		name    string
		market  Market
		wantErr bool
	}{
		{
			name: "valid market",
			market: Market{
				ID:             "event-123:market-1",
				EventID:        "event-123",
				Title:          "Will X happen?",
				Category:       "politics",
				YesProbability: 0.75,
				NoProbability:  0.25,
				Active:         true,
				LastUpdated:    time.Now(),
				CreatedAt:      time.Now().Add(-1 * time.Hour),
			},
			wantErr: false,
		},
		{
			name: "empty ID",
			market: Market{
				Title:          "Will X happen?",
				Category:       "politics",
				YesProbability: 0.75,
				NoProbability:  0.25,
			},
			wantErr: true,
		},
		{
			name: "empty title",
			market: Market{
				ID:             "event-123:market-1",
				Category:       "politics",
				YesProbability: 0.75,
				NoProbability:  0.25,
			},
			wantErr: true,
		},
		{
			name: "invalid yes probability",
			market: Market{
				ID:             "event-123:market-1",
				Title:          "Will X happen?",
				Category:       "politics",
				YesProbability: 1.5,
				NoProbability:  0.25,
			},
			wantErr: true,
		},
		{
			name: "probabilities don't sum to 1",
			market: Market{
				ID:             "event-123:market-1",
				Title:          "Will X happen?",
				Category:       "politics",
				YesProbability: 0.5,
				NoProbability:  0.6,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.market.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Market.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
