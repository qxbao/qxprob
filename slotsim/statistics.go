// Package slotsim provides streaming Monte Carlo simulation, Welford variance tracking,
// and reporting for modular slot games.
package slotsim

import (
	"errors"
	"math"

	"github.com/qxbao/qxprob/core/slot"
)

var (
	// ErrPayoutOverflow indicates that accumulated simulation payouts exceeded int64 bounds.
	ErrPayoutOverflow = errors.New("slotsim: payout overflow")
	// ErrTotalBetOverflow indicates that accumulated simulation total bet exceeded int64 bounds.
	ErrTotalBetOverflow = errors.New("slotsim: total bet overflow")
)

// MultiplierBucket records frequency and proportion of spins falling into a payout multiplier range.
type MultiplierBucket struct {
	Label   string  `json:"label"`
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
	Count   uint64  `json:"count"`
	Percent float64 `json:"percent"`
}

// Report holds the complete aggregate statistical results of a simulation run.
type Report struct {
	GameID            string             `json:"game_id"`
	Version           string             `json:"version"`
	TotalSpins        uint64             `json:"total_spins"`
	BetPerSpin        slot.Amount        `json:"bet_per_spin"`
	TotalBet          slot.Amount        `json:"total_bet"`
	TotalPayout       slot.Amount        `json:"total_payout"`
	BasePayout        slot.Amount        `json:"base_payout"`
	FeaturePayout     slot.Amount        `json:"feature_payout"`
	TotalRTP          float64            `json:"total_rtp"`
	BaseRTP           float64            `json:"base_rtp"`
	FeatureRTP        float64            `json:"feature_rtp"`
	HitCount          uint64             `json:"hit_count"`
	HitFrequency      float64            `json:"hit_frequency"`
	FeatureCount      uint64             `json:"feature_count"`
	FeatureFrequency  float64            `json:"feature_frequency"`
	AverageWin        float64            `json:"average_win"`
	AverageFeatureWin float64            `json:"average_feature_win"`
	MaxMultiplier     slot.Multiplier    `json:"max_multiplier"`
	MaxPayout         slot.Amount        `json:"max_payout"`
	Variance          float64            `json:"variance"`
	StandardDeviation float64            `json:"standard_deviation"`
	Buckets           []MultiplierBucket `json:"buckets"`
	DurationSeconds   float64            `json:"duration_seconds,omitempty"`
}

// defaultBuckets initializes standard slot payout multiplier distribution bands.
func defaultBuckets() []MultiplierBucket {
	return []MultiplierBucket{
		{Label: "0x", Min: 0.0, Max: 0.0},
		{Label: "(0, 1x]", Min: 0.0, Max: 1.0},
		{Label: "(1, 2x]", Min: 1.0, Max: 2.0},
		{Label: "(2, 5x]", Min: 2.0, Max: 5.0},
		{Label: "(5, 10x]", Min: 5.0, Max: 10.0},
		{Label: "(10, 20x]", Min: 10.0, Max: 20.0},
		{Label: "(20, 50x]", Min: 20.0, Max: 50.0},
		{Label: "(50, 100x]", Min: 50.0, Max: 100.0},
		{Label: "(100, 500x]", Min: 100.0, Max: 500.0},
		{Label: "(500, 1000x]", Min: 500.0, Max: 1000.0},
		{Label: ">1000x", Min: 1000.0, Max: -1.0},
	}
}

// accumulator maintains online Welford statistics and distribution aggregates.
type accumulator struct {
	gameID        string
	version       string
	betPerSpin    slot.Amount
	totalSpins    uint64
	totalPayout   slot.Amount
	basePayout    slot.Amount
	featurePayout slot.Amount
	hitCount      uint64
	featureCount  uint64
	maxMultiplier slot.Multiplier
	maxPayout     slot.Amount
	buckets       []MultiplierBucket
	mean          float64
	m2            float64
}

func newAccumulator(bet slot.Amount) *accumulator {
	return &accumulator{
		betPerSpin: bet,
		buckets:    defaultBuckets(),
	}
}

// add updates running metrics with the outcome of one simulated round.
func (a *accumulator) add(res slot.SpinResult) error {
	if a.gameID == "" {
		a.gameID = res.GameID
		a.version = res.Version
	}
	a.totalSpins++

	if res.Payout > 0 && a.totalPayout > math.MaxInt64-res.Payout {
		return ErrPayoutOverflow
	}
	a.totalPayout += res.Payout

	basePay, err := slot.Settle(res.Bet, res.BaseMultiplier)
	if err != nil {
		return err
	}
	featPay, err := slot.Settle(res.Bet, res.FeatureMultiplier)
	if err != nil {
		return err
	}

	if basePay > 0 && a.basePayout > math.MaxInt64-basePay {
		return ErrPayoutOverflow
	}
	a.basePayout += basePay

	if featPay > 0 && a.featurePayout > math.MaxInt64-featPay {
		return ErrPayoutOverflow
	}
	a.featurePayout += featPay

	if res.Payout > 0 {
		a.hitCount++
	}
	if res.FreeSpinCount > 0 || len(res.FreeSpins) > 0 || len(res.Triggers) > 0 {
		a.featureCount++
	}
	if res.TotalMultiplier > a.maxMultiplier {
		a.maxMultiplier = res.TotalMultiplier
	}
	if res.Payout > a.maxPayout {
		a.maxPayout = res.Payout
	}

	// Win ratio for Welford update
	var ratio float64
	if a.betPerSpin > 0 {
		ratio = float64(res.Payout) / float64(a.betPerSpin)
	}

	// Multiplier bucket
	a.placeInBucket(ratio)

	// Welford variance update
	k := float64(a.totalSpins)
	diff := ratio - a.mean
	a.mean += diff / k
	diff2 := ratio - a.mean
	a.m2 += diff * diff2
	return nil
}

func (a *accumulator) placeInBucket(ratio float64) {
	if ratio <= 0.0 {
		a.buckets[0].Count++
		return
	}
	for i := 1; i < len(a.buckets); i++ {
		b := &a.buckets[i]
		if b.Max < 0 {
			if ratio > b.Min {
				b.Count++
				return
			}
		} else if ratio > b.Min && ratio <= b.Max {
			b.Count++
			return
		}
	}
}

// finish compiles the finalized Report.
func (a *accumulator) finish(durationSec float64) (Report, error) {
	if a.totalSpins > 0 && uint64(a.betPerSpin) > math.MaxInt64/a.totalSpins {
		return Report{}, ErrTotalBetOverflow
	}
	totalBet := a.betPerSpin * slot.Amount(a.totalSpins)

	var totalRTP, baseRTP, featRTP float64
	if totalBet > 0 {
		totalRTP = float64(a.totalPayout) / float64(totalBet)
		baseRTP = float64(a.basePayout) / float64(totalBet)
		featRTP = float64(a.featurePayout) / float64(totalBet)
	}

	var hitFreq, featFreq float64
	if a.totalSpins > 0 {
		hitFreq = float64(a.hitCount) / float64(a.totalSpins)
		featFreq = float64(a.featureCount) / float64(a.totalSpins)
	}

	var avgWin float64
	if a.totalSpins > 0 {
		avgWin = float64(a.totalPayout) / float64(a.totalSpins)
	}

	var avgFeatWin float64
	if a.featureCount > 0 {
		avgFeatWin = float64(a.featurePayout) / float64(a.featureCount)
	}

	var variance, stdDev float64
	if a.totalSpins > 1 {
		variance = a.m2 / float64(a.totalSpins-1)
		if variance > 0 {
			stdDev = math.Sqrt(variance)
		}
	}

	// Compute bucket percentages
	finalBuckets := make([]MultiplierBucket, len(a.buckets))
	for i, b := range a.buckets {
		var pct float64
		if a.totalSpins > 0 {
			pct = float64(b.Count) / float64(a.totalSpins) * 100.0
		}
		finalBuckets[i] = MultiplierBucket{
			Label:   b.Label,
			Min:     b.Min,
			Max:     b.Max,
			Count:   b.Count,
			Percent: pct,
		}
	}

	return Report{
		GameID:            a.gameID,
		Version:           a.version,
		TotalSpins:        a.totalSpins,
		BetPerSpin:        a.betPerSpin,
		TotalBet:          totalBet,
		TotalPayout:       a.totalPayout,
		BasePayout:        a.basePayout,
		FeaturePayout:     a.featurePayout,
		TotalRTP:          totalRTP,
		BaseRTP:           baseRTP,
		FeatureRTP:        featRTP,
		HitCount:          a.hitCount,
		HitFrequency:      hitFreq,
		FeatureCount:      a.featureCount,
		FeatureFrequency:  featFreq,
		AverageWin:        avgWin,
		AverageFeatureWin: avgFeatWin,
		MaxMultiplier:     a.maxMultiplier,
		MaxPayout:         a.maxPayout,
		Variance:          variance,
		StandardDeviation: stdDev,
		Buckets:           finalBuckets,
		DurationSeconds:   durationSec,
	}, nil
}
