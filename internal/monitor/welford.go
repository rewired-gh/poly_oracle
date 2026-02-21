package monitor

import (
	"math"

	"github.com/rewired-gh/polyoracle/internal/models"
)

const (
	Epsilon = 1e-9
	Delta   = 0.005
)

func UpdateWelford(state *models.MarketState, price float64) {
	state.WelfordCount++
	delta := price - state.WelfordMean
	state.WelfordMean += delta / float64(state.WelfordCount)
	delta2 := price - state.WelfordMean
	state.WelfordM2 += delta * delta2
}

func GetSigma(state *models.MarketState) float64 {
	if state.WelfordCount < 2 {
		return 0.01
	}
	variance := state.WelfordM2 / float64(state.WelfordCount-1)
	return math.Max(math.Sqrt(variance), Delta)
}

func UpdateTCBuffer(state *models.MarketState, value float64, windowSize int) {
	if len(state.TCBuffer) < windowSize {
		state.TCBuffer = append(state.TCBuffer, value)
	} else {
		state.TCBuffer[state.TCIndex] = value
	}
	state.TCIndex = (state.TCIndex + 1) % windowSize
}
