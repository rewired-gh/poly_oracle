package models

import (
	"testing"
	"time"
)

func TestEventValidate(t *testing.T) {
	tests := []struct {
		name    string
		event   Event
		wantErr bool
	}{
		{
			name: "valid event",
			event: Event{
				ID:             "event-123",
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
			event: Event{
				Title:          "Will X happen?",
				Category:       "politics",
				YesProbability: 0.75,
				NoProbability:  0.25,
			},
			wantErr: true,
		},
		{
			name: "empty question",
			event: Event{
				ID:             "event-123",
				Category:       "politics",
				YesProbability: 0.75,
				NoProbability:  0.25,
			},
			wantErr: true,
		},
		{
			name: "invalid yes probability",
			event: Event{
				ID:             "event-123",
				Title:          "Will X happen?",
				Category:       "politics",
				YesProbability: 1.5,
				NoProbability:  0.25,
			},
			wantErr: true,
		},
		{
			name: "probabilities don't sum to 1",
			event: Event{
				ID:             "event-123",
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
			err := tt.event.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Event.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSnapshotValidate(t *testing.T) {
	tests := []struct {
		name     string
		snapshot Snapshot
		wantErr  bool
	}{
		{
			name: "valid snapshot",
			snapshot: Snapshot{
				ID:             "snap-123",
				EventID:        "event-123",
				YesProbability: 0.75,
				NoProbability:  0.25,
				Timestamp:      time.Now(),
				Source:         "polymarket-gamma-api",
			},
			wantErr: false,
		},
		{
			name: "empty ID",
			snapshot: Snapshot{
				EventID:        "event-123",
				YesProbability: 0.75,
				NoProbability:  0.25,
				Timestamp:      time.Now(),
				Source:         "polymarket-gamma-api",
			},
			wantErr: true,
		},
		{
			name: "invalid probability",
			snapshot: Snapshot{
				ID:             "snap-123",
				EventID:        "event-123",
				YesProbability: -0.1,
				NoProbability:  0.25,
				Timestamp:      time.Now(),
				Source:         "polymarket-gamma-api",
			},
			wantErr: true,
		},
		{
			name: "future timestamp",
			snapshot: Snapshot{
				ID:             "snap-123",
				EventID:        "event-123",
				YesProbability: 0.75,
				NoProbability:  0.25,
				Timestamp:      time.Now().Add(1 * time.Hour),
				Source:         "polymarket-gamma-api",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.snapshot.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Snapshot.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestChangeValidate(t *testing.T) {
	tests := []struct {
		name    string
		change  Change
		wantErr bool
	}{
		{
			name: "valid increase change",
			change: Change{
				ID:             "change-123",
				EventID:        "event-123",
				EventQuestion:  "Will X happen?",
				Magnitude:      0.15,
				Direction:      "increase",
				OldProbability: 0.60,
				NewProbability: 0.75,
				TimeWindow:     1 * time.Hour,
				DetectedAt:     time.Now(),
				Notified:       false,
			},
			wantErr: false,
		},
		{
			name: "valid decrease change",
			change: Change{
				ID:             "change-456",
				EventID:        "event-123",
				EventQuestion:  "Will Y happen?",
				Magnitude:      0.20,
				Direction:      "decrease",
				OldProbability: 0.80,
				NewProbability: 0.60,
				TimeWindow:     2 * time.Hour,
				DetectedAt:     time.Now(),
				Notified:       false,
			},
			wantErr: false,
		},
		{
			name: "magnitude mismatch",
			change: Change{
				ID:             "change-789",
				EventID:        "event-123",
				EventQuestion:  "Will Z happen?",
				Magnitude:      0.10,
				Direction:      "increase",
				OldProbability: 0.60,
				NewProbability: 0.80,
				TimeWindow:     1 * time.Hour,
				DetectedAt:     time.Now(),
			},
			wantErr: true,
		},
		{
			name: "invalid direction",
			change: Change{
				ID:             "change-111",
				EventID:        "event-123",
				EventQuestion:  "Test",
				Magnitude:      0.10,
				Direction:      "sideways",
				OldProbability: 0.60,
				NewProbability: 0.70,
				TimeWindow:     1 * time.Hour,
				DetectedAt:     time.Now(),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.change.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Change.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
