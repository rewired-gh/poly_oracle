package models

import (
	"time"
)

type MarketState struct {
	MarketID string

	WelfordCount int
	WelfordMean  float64
	WelfordM2    float64

	AvgDepth float64

	TCBuffer []float64
	TCIndex  int

	LastPrice  float64
	LastSigma  float64
	LastVolume float64

	UpdatedAt time.Time
}

type Alert struct {
	MarketID       string
	EventTitle     string
	EventURL       string
	MarketQuestion string

	FinalScore    float64
	IsAlert       bool
	HellingerDist float64
	LiqPressure   float64
	InstEnergy    float64
	TC            float64

	OldProb     float64
	NewProb     float64
	PriceDelta  float64
	VolumeDelta float64
	Depth       float64

	DetectedAt time.Time
	Notified   bool
}

type EventGroup struct {
	EventID    string
	EventTitle string
	EventURL   string
	BestScore  float64
	Markets    []Alert
}
