package monitor

import (
	"math"
	"sort"
	"time"

	"github.com/rewired-gh/polyoracle/internal/logger"
	"github.com/rewired-gh/polyoracle/internal/models"
	"github.com/rewired-gh/polyoracle/internal/storage"
)

type Config struct {
	WindowSize         int
	Alpha              float64
	Ceiling            float64
	Threshold          float64
	Volume24hrMin      float64
	Volume1wkMin       float64
	Volume1moMin       float64
	TopK               int
	CooldownMultiplier int
	CheckpointInterval int
}

func DefaultConfig() Config {
	return Config{
		WindowSize:         3,
		Alpha:              0.1,
		Ceiling:            10.0,
		Threshold:          3.0,
		Volume24hrMin:      25000,
		Volume1wkMin:       100000,
		Volume1moMin:       500000,
		TopK:               10,
		CooldownMultiplier: 5,
		CheckpointInterval: 12,
	}
}

type notifiedRecord struct {
	Direction string
	NewProb   float64
	SentAt    time.Time
}

type Monitor struct {
	storage         *storage.Storage
	states          map[string]*models.MarketState
	notifiedMarkets map[string]notifiedRecord
	config          Config
	cycleCount      int
}

func New(s *storage.Storage, config Config) *Monitor {
	m := &Monitor{
		storage:         s,
		states:          make(map[string]*models.MarketState),
		notifiedMarkets: make(map[string]notifiedRecord),
		config:          config,
	}

	persisted, err := s.LoadAllStates()
	if err != nil {
		logger.Warn("Failed to load persisted states: %v", err)
	} else {
		m.states = persisted
		logger.Info("Loaded %d persisted market states", len(persisted))
	}

	return m
}

func (m *Monitor) shouldProcessMarket(market models.Market) bool {
	if !market.Active || market.Closed {
		return false
	}
	return market.Volume24hr >= m.config.Volume24hrMin ||
		market.Volume1wk >= m.config.Volume1wkMin ||
		market.Volume1mo >= m.config.Volume1moMin
}

func (m *Monitor) getOrCreateState(marketID string) *models.MarketState {
	if state, exists := m.states[marketID]; exists {
		return state
	}

	state := &models.MarketState{
		MarketID:  marketID,
		LastSigma: 0.01,
	}
	m.states[marketID] = state
	return state
}

func erf(x float64) float64 {
	const (
		a1 = 0.254829592
		a2 = -0.284496736
		a3 = 1.421413741
		a4 = -1.453152027
		a5 = 1.061405429
		p  = 0.3275911
	)

	sign := 1.0
	if x < 0 {
		sign = -1.0
	}
	x = math.Abs(x)

	t := 1.0 / (1.0 + p*x)
	y := 1.0 - (((((a5*t+a4)*t)+a3)*t+a2)*t+a1)*t*math.Exp(-x*x)

	return sign * y
}

func (m *Monitor) processMarket(state *models.MarketState, market models.Market, volumeDelta float64) models.Alert {
	p0, p1 := state.LastPrice, market.YesProbability

	hDist := math.Sqrt(1 - (math.Sqrt(p1*p0) + math.Sqrt((1-p1)*(1-p0))))

	priceDelta := math.Abs(p1 - p0)
	// Use daily turnover ratio (Volume24hr / AvgDepth) instead of volumeDelta.
	// Volume24hr is a rolling 24h window: between polls the delta ≈ 0 (new trades ≈ trades
	// rolling off), collapsing liqPressure to 0 and silencing all scores.
	// Turnover ratio stays positive for any active market and discriminates by liquidity quality.
	loadRatio := market.Volume24hr / (state.AvgDepth + Epsilon)
	liqPressure := erf(loadRatio)

	instEnergy := (hDist * liqPressure) / (state.LastSigma + Epsilon)

	direction := 1.0
	if p1 < p0 {
		direction = -1.0
	} else if p1 == p0 {
		direction = 0.0
	}

	UpdateTCBuffer(state, instEnergy*direction, m.config.WindowSize)

	var tc float64
	for _, v := range state.TCBuffer {
		tc += v
	}
	tc = math.Abs(tc)

	finalScore := instEnergy * math.Sqrt(tc+Epsilon)

	if finalScore < m.config.Ceiling {
		UpdateWelford(state, p1)
		state.AvgDepth = (1-m.config.Alpha)*state.AvgDepth + m.config.Alpha*market.Liquidity
	}

	return models.Alert{
		MarketID:       market.ID,
		EventTitle:     market.Title,
		EventURL:       market.EventURL,
		MarketQuestion: market.MarketQuestion,
		FinalScore:     finalScore,
		IsAlert:        finalScore > m.config.Threshold,
		HellingerDist:  hDist,
		LiqPressure:    liqPressure,
		InstEnergy:     instEnergy,
		TC:             tc,
		OldProb:        p0,
		NewProb:        p1,
		PriceDelta:     priceDelta,
		VolumeDelta:    volumeDelta,
		Depth:          market.Liquidity,
		DetectedAt:     time.Now(),
	}
}

func (m *Monitor) ProcessPoll(markets []models.Market) []models.Alert {
	var alerts []models.Alert
	var processed int
	var maxScore float64
	var nearThreshold int

	for _, market := range markets {
		if !m.shouldProcessMarket(market) {
			continue
		}

		state := m.getOrCreateState(market.ID)

		if state.WelfordCount == 0 {
			state.LastPrice = market.YesProbability
			state.LastVolume = market.Volume24hr
			state.WelfordCount = 1
			state.LastSigma = 0.01
			state.AvgDepth = market.Liquidity
			continue
		}

		volumeDelta := market.Volume24hr - state.LastVolume
		if volumeDelta < 0 {
			volumeDelta = 0
		}

		alert := m.processMarket(state, market, volumeDelta)
		processed++

		if alert.FinalScore > maxScore {
			maxScore = alert.FinalScore
		}
		if alert.FinalScore >= m.config.Threshold*0.5 {
			nearThreshold++
			logger.Debug("High-score market %s: score=%.3f hDist=%.4f liqP=%.3f instE=%.3f tc=%.3f %.3f→%.3f vol24h=%.0f depth=%.0f",
				market.ID, alert.FinalScore, alert.HellingerDist, alert.LiqPressure, alert.InstEnergy, alert.TC,
				alert.OldProb, alert.NewProb, market.Volume24hr, market.Liquidity)
		}

		state.LastPrice = market.YesProbability
		state.LastVolume = market.Volume24hr
		state.LastSigma = GetSigma(state)
		state.UpdatedAt = time.Now()

		if alert.IsAlert {
			alerts = append(alerts, alert)
		}
	}

	logger.Debug("Processed %d active markets: max_score=%.3f, %d near-threshold (≥%.1f), %d above threshold",
		processed, maxScore, nearThreshold, m.config.Threshold*0.5, len(alerts))

	m.cycleCount++
	if m.cycleCount%m.config.CheckpointInterval == 0 {
		m.checkpoint()
	}

	return alerts
}

func (m *Monitor) checkpoint() {
	for marketID, state := range m.states {
		if err := m.storage.SaveState(marketID, state); err != nil {
			logger.Warn("Failed to checkpoint state for %s: %v", marketID, err)
		}
	}
}

func (m *Monitor) Shutdown() {
	logger.Info("Checkpointing %d market states before shutdown", len(m.states))
	m.checkpoint()
}

func extractEventID(marketID string) string {
	for i := len(marketID) - 1; i >= 0; i-- {
		if marketID[i] == ':' {
			return marketID[:i]
		}
	}
	return marketID
}

func (m *Monitor) GroupByEvent(alerts []models.Alert) []models.EventGroup {
	groups := make(map[string]*models.EventGroup)

	for _, alert := range alerts {
		eventID := extractEventID(alert.MarketID)

		if _, exists := groups[eventID]; !exists {
			groups[eventID] = &models.EventGroup{
				EventID:    eventID,
				EventTitle: alert.EventTitle,
				EventURL:   alert.EventURL,
			}
		}

		groups[eventID].Markets = append(groups[eventID].Markets, alert)
		if alert.FinalScore > groups[eventID].BestScore {
			groups[eventID].BestScore = alert.FinalScore
		}
	}

	for _, group := range groups {
		sort.Slice(group.Markets, func(i, j int) bool {
			return group.Markets[i].FinalScore > group.Markets[j].FinalScore
		})
	}

	result := make([]models.EventGroup, 0, len(groups))
	for _, group := range groups {
		result = append(result, *group)
	}

	return result
}

func isDeterministicZone(p float64) bool {
	return p > 0.90 || p < 0.10
}

func getDirection(oldProb, newProb float64) string {
	switch {
	case newProb > oldProb:
		return "increase"
	case newProb < oldProb:
		return "decrease"
	default:
		return "no_change"
	}
}

func (m *Monitor) FilterRecentlySent(groups []models.EventGroup, cooldown time.Duration) []models.EventGroup {
	now := time.Now()
	var result []models.EventGroup

	for _, group := range groups {
		var filtered []models.Alert

		for _, alert := range group.Markets {
			rec, exists := m.notifiedMarkets[alert.MarketID]

			if exists && now.Sub(rec.SentAt) < cooldown {
				sameDirection := rec.Direction == getDirection(alert.OldProb, alert.NewProb)
				enteringDetZone := isDeterministicZone(alert.NewProb) && !isDeterministicZone(rec.NewProb)

				if sameDirection && !enteringDetZone {
					continue
				}
			}

			filtered = append(filtered, alert)
		}

		if len(filtered) > 0 {
			newGroup := group
			newGroup.Markets = filtered
			newGroup.BestScore = filtered[0].FinalScore
			result = append(result, newGroup)
		}
	}

	return result
}

func (m *Monitor) RecordNotified(groups []models.EventGroup) {
	now := time.Now()
	for _, group := range groups {
		for _, alert := range group.Markets {
			m.notifiedMarkets[alert.MarketID] = notifiedRecord{
				Direction: getDirection(alert.OldProb, alert.NewProb),
				NewProb:   alert.NewProb,
				SentAt:    now,
			}
		}
	}
}

func (m *Monitor) PostProcessAlerts(alerts []models.Alert, pollInterval time.Duration) []models.EventGroup {
	groups := m.GroupByEvent(alerts)

	sort.Slice(groups, func(i, j int) bool {
		return groups[i].BestScore > groups[j].BestScore
	})

	if len(groups) > m.config.TopK {
		groups = groups[:m.config.TopK]
	}

	cooldown := time.Duration(m.config.CooldownMultiplier) * pollInterval
	groups = m.FilterRecentlySent(groups, cooldown)

	return groups
}
