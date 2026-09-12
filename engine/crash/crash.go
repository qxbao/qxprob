// Package crash implements a crash-style airplane game engine driven by an entropy source.
package crash

import (
	"errors"
	"fmt"
	"math"

	"github.com/qxbao/qxprob/core/entropy"
)

var (
	// ErrInvalidHouseEdge indicates that HouseEdge is NaN, infinite, or outside [0, 100).
	ErrInvalidHouseEdge = errors.New("crash: house edge must be a finite percentage in [0, 100)")

	// ErrInvalidInstantCrashProbability indicates that InstantCrashProbability is NaN, infinite, or outside [0, 1].
	ErrInvalidInstantCrashProbability = errors.New("crash: instant crash probability must be a finite value in [0, 1]")

	// ErrNilSource indicates that a nil entropy source was provided.
	ErrNilSource = errors.New("crash: entropy source must not be nil")

	// ErrEmptyServerSeed indicates that an empty server seed was provided.
	ErrEmptyServerSeed = errors.New("crash: server seed must not be empty")

	// ErrEmptyClientSeed indicates that an empty client seed was provided.
	ErrEmptyClientSeed = errors.New("crash: client seed must not be empty")

	// ErrInvalidMaxMultiplier indicates that MaxMultiplier is non-finite or between zero and 1.
	ErrInvalidMaxMultiplier = errors.New("crash: max multiplier must be zero (uncapped) or a finite value >= 1")
)

// Config defines configuration parameters for the crash engine.
type Config struct {
	// HouseEdge is the house edge percentage in [0, 100).
	HouseEdge float64
	// InstantCrashProbability is the probability of an instant crash in [0, 1].
	InstantCrashProbability float64
	// MaxMultiplier caps the crash result when positive. Zero leaves the result uncapped.
	MaxMultiplier float64
}

// SeededInput defines inputs required to run a deterministic, reproducible round of Crash.
type SeededInput struct {
	// ServerSeed is the secret seed controlled by the game operator.
	ServerSeed string
	// ClientSeed is the public seed provided by or visible to the player.
	ClientSeed string
	// Nonce is the round counter that prevents replay across rounds.
	Nonce uint64
	// Config contains configuration parameters for the crash engine.
	Config Config
}

// Result contains the outcome of a single crash game round.
type Result struct {
	// Multiplier is the crash multiplier reached in this round, at least 1.0.
	Multiplier float64
	// InstantCrash indicates whether the round ended immediately at 1.0x due to instant crash.
	InstantCrash bool
}

// Engine executes rounds of the crash game using an injected entropy source.
type Engine struct {
	config Config
	source entropy.Source
}

// New constructs a validated crash Engine. It rejects nil sources, NaN, infinity,
// or out-of-range configuration parameters without consuming entropy.
func New(config Config, source entropy.Source) (*Engine, error) {
	if source == nil {
		return nil, ErrNilSource
	}
	if math.IsNaN(config.HouseEdge) || math.IsInf(config.HouseEdge, 0) || config.HouseEdge < 0 || config.HouseEdge >= 100 {
		return nil, ErrInvalidHouseEdge
	}
	if math.IsNaN(config.InstantCrashProbability) || math.IsInf(config.InstantCrashProbability, 0) || config.InstantCrashProbability < 0 || config.InstantCrashProbability > 1 {
		return nil, ErrInvalidInstantCrashProbability
	}
	if math.IsNaN(config.MaxMultiplier) || math.IsInf(config.MaxMultiplier, 0) || config.MaxMultiplier < 0 || (config.MaxMultiplier > 0 && config.MaxMultiplier < 1) {
		return nil, ErrInvalidMaxMultiplier
	}
	return &Engine{
		config: config,
		source: source,
	}, nil
}

// Play executes one round of the crash game. It consumes exactly one Float64 from the
// entropy source. If U < InstantCrashProbability, an instant crash result is returned with
// a multiplier of 1.0. Otherwise, Multiplier is calculated as (100 - HouseEdge) / (100 * (1 - U)),
// floored to 1.0 if less than 1.0, then optionally capped by MaxMultiplier.
func (e *Engine) Play() (Result, error) {
	u, err := e.source.Float64()
	if err != nil {
		return Result{}, fmt.Errorf("crash: failed to sample entropy: %w", err)
	}

	if u < e.config.InstantCrashProbability {
		return Result{
			Multiplier:   1.0,
			InstantCrash: true,
		}, nil
	}

	multiplier := (100.0 - e.config.HouseEdge) / (100.0 * (1.0 - u))
	if multiplier < 1.0 {
		multiplier = 1.0
	}
	if e.config.MaxMultiplier > 0 && multiplier > e.config.MaxMultiplier {
		multiplier = e.config.MaxMultiplier
	}

	return Result{
		Multiplier:   multiplier,
		InstantCrash: false,
	}, nil
}

// PlaySeeded executes a deterministic, reproducible round of Crash derived from
// server seed, client seed, and nonce. It validates inputs before constructing
// an entropy source or consuming entropy.
func PlaySeeded(input SeededInput) (Result, error) {
	if input.ServerSeed == "" {
		return Result{}, ErrEmptyServerSeed
	}
	if input.ClientSeed == "" {
		return Result{}, ErrEmptyClientSeed
	}

	source := entropy.NewProvablyFairSource(input.ServerSeed, input.ClientSeed, input.Nonce)
	engine, err := New(input.Config, source)
	if err != nil {
		return Result{}, err
	}

	return engine.Play()
}
