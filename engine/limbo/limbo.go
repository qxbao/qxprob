// Package limbo implements a configurable provably-fair Limbo game engine driven by an entropy source.
package limbo

import (
	"errors"
	"fmt"
	"math"

	"github.com/qxbao/qxprob/core/entropy"
)

const (
	// DefaultRTP is the default return to player ratio (99%).
	DefaultRTP = 0.99
	// DefaultMinMultiplier is the default minimum outcome multiplier.
	DefaultMinMultiplier = 1.0
	// DefaultMaxMultiplier is the default maximum outcome multiplier.
	DefaultMaxMultiplier = 1_000_000.0
	// DefaultPayoutStep is the default step size for multiplier quantization.
	DefaultPayoutStep = 0.01
	// DefaultTargetMultiplier is the default target multiplier for winning settlement.
	DefaultTargetMultiplier = 2.0
)

var (
	// ErrNilSource indicates that a nil entropy source was provided.
	ErrNilSource = errors.New("limbo: entropy source must not be nil")

	// ErrEmptyServerSeed indicates that an empty server seed was provided.
	ErrEmptyServerSeed = errors.New("limbo: server seed must not be empty")

	// ErrEmptyClientSeed indicates that an empty client seed was provided.
	ErrEmptyClientSeed = errors.New("limbo: client seed must not be empty")

	// ErrInvalidRTP indicates that RTP is not finite or outside (0, 1].
	ErrInvalidRTP = errors.New("limbo: RTP must be a finite value in (0, 1]")

	// ErrInvalidMinMultiplier indicates that MinMultiplier is not finite or less than 1.
	ErrInvalidMinMultiplier = errors.New("limbo: minimum multiplier must be a finite value >= 1")

	// ErrInvalidMaxMultiplier indicates that MaxMultiplier is not finite or less than MinMultiplier.
	ErrInvalidMaxMultiplier = errors.New("limbo: maximum multiplier must be a finite value >= minimum multiplier")

	// ErrInvalidPayoutStep indicates that PayoutStep is not finite, non-positive, or greater than MinMultiplier.
	ErrInvalidPayoutStep = errors.New("limbo: payout step must be a finite value in (0, minimum multiplier]")

	// ErrInvalidTargetMultiplier indicates that TargetMultiplier is not finite or outside [MinMultiplier, MaxMultiplier].
	ErrInvalidTargetMultiplier = errors.New("limbo: target multiplier must be a finite value in [minimum multiplier, maximum multiplier]")

	// ErrInvalidEntropyFloat indicates that the entropy source returned a non-finite float or a float outside [0, 1).
	ErrInvalidEntropyFloat = errors.New("limbo: entropy source returned invalid float")
)

// Config defines configuration parameters for the Limbo engine.
type Config struct {
	// RTP is the theoretical return-to-player ratio in (0, 1]. Zero uses DefaultRTP.
	RTP float64
	// MinMultiplier is the lower bound for outcome multipliers, at least 1.0. Zero uses DefaultMinMultiplier.
	MinMultiplier float64
	// MaxMultiplier is the upper bound for outcome multipliers, at least MinMultiplier. Zero uses DefaultMaxMultiplier.
	MaxMultiplier float64
	// PayoutStep is the quantization step for outcome multipliers in (0, MinMultiplier]. Zero uses DefaultPayoutStep.
	PayoutStep float64
	// TargetMultiplier is the user's target win threshold in [MinMultiplier, MaxMultiplier]. Zero uses DefaultTargetMultiplier.
	TargetMultiplier float64
}

// SeededInput defines inputs required to run a deterministic, reproducible round of Limbo.
type SeededInput struct {
	// ServerSeed is the secret seed controlled by the game operator.
	ServerSeed string
	// ClientSeed is the public seed provided by or visible to the player.
	ClientSeed string
	// Nonce is the round counter that prevents replay across rounds.
	Nonce uint64
	// Config contains configuration parameters for the Limbo engine.
	Config Config
}

// Result contains the outcome of a single Limbo game round.
type Result struct {
	// Multiplier is the final outcome multiplier reached in this round.
	Multiplier float64
	// TargetMultiplier is the target multiplier threshold for this round.
	TargetMultiplier float64
	// Won indicates whether Multiplier was greater than or equal to TargetMultiplier.
	Won bool
	// PayoutMultiplier is TargetMultiplier when Won is true, or 0.0 on a loss.
	PayoutMultiplier float64
}

// Engine executes rounds of the Limbo game using an injected entropy source.
type Engine struct {
	config Config
	source entropy.Source
}

// New constructs a validated Limbo Engine. It rejects nil sources, NaN, infinity,
// or out-of-range configuration parameters without consuming entropy.
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

func resolveConfig(cfg Config) (Config, error) {
	if cfg.RTP == 0 {
		cfg.RTP = DefaultRTP
	}
	if cfg.MinMultiplier == 0 {
		cfg.MinMultiplier = DefaultMinMultiplier
	}
	if cfg.MaxMultiplier == 0 {
		cfg.MaxMultiplier = DefaultMaxMultiplier
	}
	if cfg.PayoutStep == 0 {
		cfg.PayoutStep = DefaultPayoutStep
	}
	if cfg.TargetMultiplier == 0 {
		cfg.TargetMultiplier = DefaultTargetMultiplier
	}

	if math.IsNaN(cfg.RTP) || math.IsInf(cfg.RTP, 0) || cfg.RTP <= 0 || cfg.RTP > 1 {
		return Config{}, ErrInvalidRTP
	}
	if math.IsNaN(cfg.MinMultiplier) || math.IsInf(cfg.MinMultiplier, 0) || cfg.MinMultiplier < 1 {
		return Config{}, ErrInvalidMinMultiplier
	}
	if math.IsNaN(cfg.MaxMultiplier) || math.IsInf(cfg.MaxMultiplier, 0) || cfg.MaxMultiplier < cfg.MinMultiplier {
		return Config{}, ErrInvalidMaxMultiplier
	}
	if math.IsNaN(cfg.PayoutStep) || math.IsInf(cfg.PayoutStep, 0) || cfg.PayoutStep <= 0 || cfg.PayoutStep > cfg.MinMultiplier {
		return Config{}, ErrInvalidPayoutStep
	}
	if math.IsNaN(cfg.TargetMultiplier) || math.IsInf(cfg.TargetMultiplier, 0) || cfg.TargetMultiplier < cfg.MinMultiplier || cfg.TargetMultiplier > cfg.MaxMultiplier {
		return Config{}, ErrInvalidTargetMultiplier
	}
	return cfg, nil
}

func calculateMultiplier(u float64, cfg Config) float64 {
	if u == 0 {
		return cfg.MaxMultiplier
	}
	raw := cfg.RTP / u
	multiplier := math.Floor(raw/cfg.PayoutStep) * cfg.PayoutStep
	if math.IsNaN(multiplier) || multiplier > cfg.MaxMultiplier {
		multiplier = cfg.MaxMultiplier
	}
	if multiplier < cfg.MinMultiplier {
		multiplier = cfg.MinMultiplier
	}
	return multiplier
}

// Play executes one round of the Limbo game. It consumes exactly one Float64
// from the entropy source, calculates the outcome multiplier, clamps it to
// [MinMultiplier, MaxMultiplier], and evaluates win settlement against TargetMultiplier.
func (e *Engine) Play() (Result, error) {
	u, err := e.source.Float64()
	if err != nil {
		return Result{}, fmt.Errorf("limbo: failed to sample entropy: %w", err)
	}
	if math.IsNaN(u) || math.IsInf(u, 0) || u < 0 || u >= 1 {
		return Result{}, fmt.Errorf("limbo: %w: %v", ErrInvalidEntropyFloat, u)
	}

	multiplier := calculateMultiplier(u, e.config)
	won := multiplier >= e.config.TargetMultiplier
	var payoutMultiplier float64
	if won {
		payoutMultiplier = e.config.TargetMultiplier
	}

	return Result{
		Multiplier:       multiplier,
		TargetMultiplier: e.config.TargetMultiplier,
		Won:              won,
		PayoutMultiplier: payoutMultiplier,
	}, nil
}

// PlaySeeded executes a deterministic, reproducible round of Limbo derived from
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
