// Package duckrace implements a provably-fair Duck Race game engine driven by an entropy source.
package duckrace

import (
	"errors"
	"fmt"
	"math"

	"github.com/qxbao/qxprob/core/entropy"
)

const (
	// MinDucks is the minimum supported number of ducks in a race.
	MinDucks = 2
	// MaxDucks is the maximum supported number of ducks in a race.
	MaxDucks = 16

	// MinDurationMillis is the minimum supported race duration in milliseconds.
	MinDurationMillis = 1000
	// MaxDurationMillis is the maximum supported race duration in milliseconds.
	MaxDurationMillis = 60000

	// MinTickMillis is the minimum supported tick interval in milliseconds.
	MinTickMillis = 50
	// MaxTickMillis is the maximum supported tick interval in milliseconds.
	MaxTickMillis = 1000

	// DefaultBaseSegmentWeight is the default positive motion bias added to each segment.
	DefaultBaseSegmentWeight = 0.25
	// DefaultFinishProgress is the default winner progress at the final frame.
	DefaultFinishProgress = 1.0
	// DefaultRankGap is the default final progress gap between consecutive ranks.
	DefaultRankGap = 0.01
)

var (
	// ErrInvalidDuckCount indicates that DuckCount is outside [2, 16].
	ErrInvalidDuckCount = errors.New("duckrace: duck count must be between 2 and 16")

	// ErrInvalidDuration indicates that DurationMillis is outside [1000, 60000].
	ErrInvalidDuration = errors.New("duckrace: duration must be between 1000 and 60000 ms")

	// ErrInvalidTick indicates that TickMillis is outside [50, 1000].
	ErrInvalidTick = errors.New("duckrace: tick must be between 50 and 1000 ms")

	// ErrIndivisibleDuration indicates that DurationMillis is not exactly divisible by TickMillis.
	ErrIndivisibleDuration = errors.New("duckrace: duration must be exactly divisible by tick")

	// ErrNilSource indicates that a nil entropy source was provided.
	ErrNilSource = errors.New("duckrace: entropy source must not be nil")

	// ErrEmptyServerSeed indicates that an empty server seed was provided.
	ErrEmptyServerSeed = errors.New("duckrace: server seed must not be empty")

	// ErrEmptyClientSeed indicates that an empty client seed was provided.
	ErrEmptyClientSeed = errors.New("duckrace: client seed must not be empty")

	// ErrInvalidEntropyFloat indicates that the entropy source returned a non-finite float or a float outside [0, 1).
	ErrInvalidEntropyFloat = errors.New("duckrace: entropy source returned invalid float")

	// ErrInvalidBaseSegmentWeight indicates that BaseSegmentWeight is not finite or positive.
	ErrInvalidBaseSegmentWeight = errors.New("duckrace: base segment weight must be finite and positive")

	// ErrInvalidFinishProgress indicates that FinishProgress is not finite or outside (0, 1].
	ErrInvalidFinishProgress = errors.New("duckrace: finish progress must be finite and in (0, 1]")

	// ErrInvalidRankGap indicates that RankGap cannot produce positive, distinct final positions.
	ErrInvalidRankGap = errors.New("duckrace: rank gap must be finite, positive, and keep last place above zero")
)

// Config defines configuration parameters for the Duck Race engine.
type Config struct {
	// DuckCount is the number of competing ducks in [2, 16].
	DuckCount int
	// DurationMillis is the total race duration in milliseconds in [1000, 60000].
	DurationMillis int64
	// TickMillis is the timeline tick interval in milliseconds in [50, 1000], exactly dividing DurationMillis.
	TickMillis int64
	// BaseSegmentWeight controls motion smoothness. Zero uses DefaultBaseSegmentWeight.
	BaseSegmentWeight float64
	// FinishProgress is the winner's final normalized position. Zero uses DefaultFinishProgress.
	FinishProgress float64
	// RankGap is the final normalized position gap between ranks. Zero uses DefaultRankGap.
	RankGap float64
}

// SeededInput defines inputs required to run a deterministic, reproducible Duck Race.
type SeededInput struct {
	// ServerSeed is the secret seed controlled by the game operator.
	ServerSeed string
	// ClientSeed is the public seed provided by or visible to the player.
	ClientSeed string
	// Nonce is the round counter that prevents replay across rounds.
	Nonce uint64
	// Config contains the race configuration parameters.
	Config Config
}

// Frame represents the state of all ducks at a specific point in time.
type Frame struct {
	// ElapsedMillis is the elapsed time since race start in milliseconds.
	ElapsedMillis int64
	// Positions contains progress values in [0, 1] for each duck in input index order.
	Positions []float64
}

// Result contains the complete timeline and outcome of a single Duck Race round.
type Result struct {
	// Frames contains timeline frames from time zero through exact duration, inclusive.
	Frames []Frame
	// FinishOrder contains the duck indices in finish order from 1st (winner) to last.
	FinishOrder []int
}

// Engine executes rounds of Duck Race using an injected entropy source.
type Engine struct {
	config Config
	source entropy.Source
}

// validateConfig verifies that the configuration parameters are within supported ranges.
func resolveConfig(config Config) (Config, error) {
	if config.DuckCount < MinDucks || config.DuckCount > MaxDucks {
		return Config{}, ErrInvalidDuckCount
	}
	if config.DurationMillis < MinDurationMillis || config.DurationMillis > MaxDurationMillis {
		return Config{}, ErrInvalidDuration
	}
	if config.TickMillis < MinTickMillis || config.TickMillis > MaxTickMillis {
		return Config{}, ErrInvalidTick
	}
	if config.DurationMillis%config.TickMillis != 0 {
		return Config{}, ErrIndivisibleDuration
	}
	if config.BaseSegmentWeight == 0 {
		config.BaseSegmentWeight = DefaultBaseSegmentWeight
	}
	if config.FinishProgress == 0 {
		config.FinishProgress = DefaultFinishProgress
	}
	if config.RankGap == 0 {
		config.RankGap = DefaultRankGap
	}
	if math.IsNaN(config.BaseSegmentWeight) || math.IsInf(config.BaseSegmentWeight, 0) || config.BaseSegmentWeight <= 0 {
		return Config{}, ErrInvalidBaseSegmentWeight
	}
	segmentCount := float64(config.DurationMillis / config.TickMillis)
	if config.BaseSegmentWeight > math.MaxFloat64/segmentCount {
		return Config{}, ErrInvalidBaseSegmentWeight
	}
	if math.IsNaN(config.FinishProgress) || math.IsInf(config.FinishProgress, 0) || config.FinishProgress <= 0 || config.FinishProgress > 1 {
		return Config{}, ErrInvalidFinishProgress
	}
	if math.IsNaN(config.RankGap) || math.IsInf(config.RankGap, 0) || config.RankGap <= 0 || config.FinishProgress-config.RankGap*float64(config.DuckCount-1) <= 0 {
		return Config{}, ErrInvalidRankGap
	}
	return config, nil
}

// New constructs a validated Duck Race Engine. It rejects nil sources or out-of-range
// configuration parameters without consuming entropy.
func New(config Config, source entropy.Source) (*Engine, error) {
	if source == nil {
		return nil, ErrNilSource
	}
	resolved, err := resolveConfig(config)
	if err != nil {
		return nil, err
	}
	return &Engine{
		config: resolved,
		source: source,
	}, nil
}

// Play executes one round of the Duck Race game. It first samples the finish order
// permutation using an unbiased Fisher-Yates shuffle, then samples positive per-segment
// weights (BaseSegmentWeight + Float64) by duck index and segment, normalizes them
// to configurable rank targets, and transposes them into independent frame slices.
func (e *Engine) Play() (Result, error) {
	order := make([]int, e.config.DuckCount)
	for i := range order {
		order[i] = i
	}

	for i := 0; i < e.config.DuckCount-1; i++ {
		remaining := uint64(e.config.DuckCount - i)
		idx, err := e.source.Intn(remaining)
		if err != nil {
			return Result{}, fmt.Errorf("duckrace: failed to sample finish order: %w", err)
		}
		swapIdx := i + int(idx)
		order[i], order[swapIdx] = order[swapIdx], order[i]
	}

	finishOrder := make([]int, e.config.DuckCount)
	copy(finishOrder, order)

	targets := make([]float64, e.config.DuckCount)
	for rank, duckIdx := range finishOrder {
		targets[duckIdx] = e.config.FinishProgress - e.config.RankGap*float64(rank)
	}

	segmentCount := int(e.config.DurationMillis / e.config.TickMillis)
	frameCount := segmentCount + 1

	duckProgress := make([][]float64, e.config.DuckCount)
	for d := 0; d < e.config.DuckCount; d++ {
		progress, err := e.generateDuckProgress(segmentCount, frameCount, targets[d])
		if err != nil {
			return Result{}, err
		}
		duckProgress[d] = progress
	}

	frames := make([]Frame, frameCount)
	for f := 0; f < frameCount; f++ {
		positions := make([]float64, e.config.DuckCount)
		for d := 0; d < e.config.DuckCount; d++ {
			positions[d] = duckProgress[d][f]
		}
		frames[f] = Frame{
			ElapsedMillis: int64(f) * e.config.TickMillis,
			Positions:     positions,
		}
	}

	return Result{
		Frames:      frames,
		FinishOrder: finishOrder,
	}, nil
}

// generateDuckProgress samples segment weights for a single duck and computes
// cumulative normalized progress values across all frames up to target.
func (e *Engine) generateDuckProgress(segmentCount, frameCount int, target float64) ([]float64, error) {
	progress := make([]float64, frameCount)
	progress[0] = 0.0

	weights := make([]float64, segmentCount)
	var totalWeight float64

	for s := 0; s < segmentCount; s++ {
		u, err := e.source.Float64()
		if err != nil {
			return nil, fmt.Errorf("duckrace: failed to sample segment weight: %w", err)
		}
		if math.IsNaN(u) || math.IsInf(u, 0) || u < 0 || u >= 1 {
			return nil, fmt.Errorf("duckrace: %w: %v", ErrInvalidEntropyFloat, u)
		}
		w := e.config.BaseSegmentWeight + u
		weights[s] = w
		totalWeight += w
	}

	var cumWeight float64
	for s := 0; s < segmentCount; s++ {
		cumWeight += weights[s]
		if s == segmentCount-1 {
			progress[s+1] = target
		} else {
			progress[s+1] = target * (cumWeight / totalWeight)
		}
	}

	return progress, nil
}

// RaceSeeded executes a deterministic, reproducible round of Duck Race derived from
// server seed, client seed, and nonce. It validates inputs before constructing
// an entropy source or consuming entropy.
func RaceSeeded(input SeededInput) (Result, error) {
	if input.ServerSeed == "" {
		return Result{}, ErrEmptyServerSeed
	}
	if input.ClientSeed == "" {
		return Result{}, ErrEmptyClientSeed
	}
	if _, err := resolveConfig(input.Config); err != nil {
		return Result{}, err
	}

	source := entropy.NewProvablyFairSource(input.ServerSeed, input.ClientSeed, input.Nonce)
	engine, err := New(input.Config, source)
	if err != nil {
		return Result{}, err
	}

	return engine.Play()
}
