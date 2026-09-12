// Package plinko implements a provably-fair Plinko game engine driven by an entropy source.
package plinko

import (
	"errors"
	"fmt"
	"math"

	"github.com/qxbao/qxprob/core/entropy"
)

// Direction represents the bounce direction at each pin (Left or Right).
type Direction uint8

const (
	// Left indicates a left pin bounce.
	Left Direction = iota
	// Right indicates a right pin bounce.
	Right
)

// String returns a human-readable representation of the bounce direction.
func (d Direction) String() string {
	switch d {
	case Left:
		return "Left"
	case Right:
		return "Right"
	default:
		return fmt.Sprintf("Direction(%d)", d)
	}
}

// RiskLevel represents the risk setting governing multiplier variance.
type RiskLevel uint8

const (
	// Low risk yields narrower payouts around 1.0x.
	Low RiskLevel = iota
	// Medium risk provides balanced volatility.
	Medium
	// High risk concentrates higher multipliers on the outer tails.
	High
)

// String returns a human-readable representation of the risk level.
func (r RiskLevel) String() string {
	switch r {
	case Low:
		return "Low"
	case Medium:
		return "Medium"
	case High:
		return "High"
	default:
		return fmt.Sprintf("RiskLevel(%d)", r)
	}
}

const (
	// MinRows is the minimum supported number of pin rows.
	MinRows = 8
	// MaxRows is the maximum supported number of pin rows.
	MaxRows = 16

	// DefaultTargetRTP is the default theoretical return-to-player ratio.
	DefaultTargetRTP = 0.99
)

var (
	// ErrInvalidRows indicates that Rows is outside [8, 16].
	ErrInvalidRows = errors.New("plinko: rows must be between 8 and 16")

	// ErrInvalidRisk indicates that Risk is not Low, Medium, or High.
	ErrInvalidRisk = errors.New("plinko: risk must be Low, Medium, or High")

	// ErrNilSource indicates that a nil entropy source was provided.
	ErrNilSource = errors.New("plinko: entropy source must not be nil")

	// ErrEmptyServerSeed indicates that an empty server seed was provided.
	ErrEmptyServerSeed = errors.New("plinko: server seed must not be empty")

	// ErrEmptyClientSeed indicates that an empty client seed was provided.
	ErrEmptyClientSeed = errors.New("plinko: client seed must not be empty")

	// ErrInvalidTargetRTP indicates that TargetRTP is not finite or outside (0, 1].
	ErrInvalidTargetRTP = errors.New("plinko: target RTP must be a finite value in (0, 1]")

	// ErrInvalidRiskAlpha indicates that RiskAlpha is not finite or is negative.
	ErrInvalidRiskAlpha = errors.New("plinko: risk alpha must be a finite non-negative value")

	// ErrInvalidMultipliers indicates that a custom multiplier table has the wrong length or invalid values.
	ErrInvalidMultipliers = errors.New("plinko: multipliers must contain Rows+1 finite non-negative values")
)

// Config defines configuration parameters for the Plinko engine.
type Config struct {
	// Rows is the number of pin rows in [8, 16].
	Rows int
	// Risk is the risk level governing payout volatility.
	Risk RiskLevel
	// TargetRTP controls the expected payout of generated multipliers. Zero uses DefaultTargetRTP.
	TargetRTP float64
	// RiskAlpha controls how strongly payouts concentrate in rare buckets. Zero uses the selected Risk default.
	RiskAlpha float64
	// Multipliers optionally overrides formula-generated payouts and must contain Rows+1 values.
	Multipliers []float64
}

// SeededInput defines inputs required to run a deterministic, reproducible round of Plinko.
type SeededInput struct {
	// ServerSeed is the secret seed controlled by the game operator.
	ServerSeed string
	// ClientSeed is the public seed provided by or visible to the player.
	ClientSeed string
	// Nonce is the round counter that prevents replay across rounds.
	Nonce uint64
	// Rows is the number of pin rows in [8, 16].
	Rows int
	// Risk is the risk level governing payout volatility.
	Risk RiskLevel
	// TargetRTP controls formula-generated payouts. Zero uses DefaultTargetRTP.
	TargetRTP float64
	// RiskAlpha overrides the selected risk's default exponent when positive.
	RiskAlpha float64
	// Multipliers optionally overrides formula-generated payouts.
	Multipliers []float64
}

// Result contains the outcome of a single Plinko round.
type Result struct {
	// Path contains the sequence of Left/Right bounce decisions of length Rows.
	Path []Direction
	// BucketIndex is the landing bucket in [0, Rows], equal to the count of Right bounces.
	BucketIndex int
	// Multiplier is the unrounded payout multiplier for BucketIndex according to the binomial formula.
	Multiplier float64
}

// Engine executes rounds of Plinko using an injected entropy source.
type Engine struct {
	config      Config
	source      entropy.Source
	multipliers []float64
}

// New constructs a validated Plinko Engine. It rejects nil sources or out-of-range
// configuration parameters without consuming entropy.
func New(config Config, source entropy.Source) (*Engine, error) {
	if source == nil {
		return nil, ErrNilSource
	}
	if config.Rows < MinRows || config.Rows > MaxRows {
		return nil, ErrInvalidRows
	}
	if config.Risk > High {
		return nil, ErrInvalidRisk
	}

	targetRTP := config.TargetRTP
	if targetRTP == 0 {
		targetRTP = DefaultTargetRTP
	}
	if math.IsNaN(targetRTP) || math.IsInf(targetRTP, 0) || targetRTP <= 0 || targetRTP > 1 {
		return nil, ErrInvalidTargetRTP
	}

	alpha := config.RiskAlpha
	if alpha == 0 {
		var err error
		alpha, err = riskAlpha(config.Risk)
		if err != nil {
			return nil, err
		}
	}
	if math.IsNaN(alpha) || math.IsInf(alpha, 0) || alpha < 0 {
		return nil, ErrInvalidRiskAlpha
	}

	var multipliers []float64
	var err error
	if config.Multipliers != nil {
		multipliers, err = validateAndCopyMultipliers(config.Rows, config.Multipliers)
	} else {
		multipliers, err = computeMultipliers(config.Rows, targetRTP, alpha)
	}
	if err != nil {
		return nil, err
	}
	config.TargetRTP = targetRTP
	config.RiskAlpha = alpha
	config.Multipliers = append([]float64(nil), multipliers...)

	return &Engine{
		config:      config,
		source:      source,
		multipliers: multipliers,
	}, nil
}

// Play executes one round of the Plinko game. It consumes exactly Rows Intn(2) calls
// from the entropy source, constructs a newly allocated Path slice, determines the
// landing bucket index, and looks up the unrounded binomial multiplier.
func (e *Engine) Play() (Result, error) {
	path := make([]Direction, e.config.Rows)
	var bucketIndex int

	for i := 0; i < e.config.Rows; i++ {
		decision, err := e.source.Intn(2)
		if err != nil {
			return Result{}, fmt.Errorf("plinko: failed to sample path decision: %w", err)
		}
		if decision == 0 {
			path[i] = Left
		} else {
			path[i] = Right
			bucketIndex++
		}
	}

	return Result{
		Path:        path,
		BucketIndex: bucketIndex,
		Multiplier:  e.multipliers[bucketIndex],
	}, nil
}

// PlaySeeded executes a deterministic, reproducible round of Plinko derived from
// server seed, client seed, and nonce. It validates inputs before constructing
// an entropy source or consuming entropy.
func PlaySeeded(input SeededInput) (Result, error) {
	if input.ServerSeed == "" {
		return Result{}, ErrEmptyServerSeed
	}
	if input.ClientSeed == "" {
		return Result{}, ErrEmptyClientSeed
	}
	if input.Rows < MinRows || input.Rows > MaxRows {
		return Result{}, ErrInvalidRows
	}
	if input.Risk > High {
		return Result{}, ErrInvalidRisk
	}

	source := entropy.NewProvablyFairSource(input.ServerSeed, input.ClientSeed, input.Nonce)
	engine, err := New(Config{
		Rows:        input.Rows,
		Risk:        input.Risk,
		TargetRTP:   input.TargetRTP,
		RiskAlpha:   input.RiskAlpha,
		Multipliers: input.Multipliers,
	}, source)
	if err != nil {
		return Result{}, err
	}

	return engine.Play()
}

// riskAlpha returns the risk exponent alpha for the given RiskLevel.
func riskAlpha(risk RiskLevel) (float64, error) {
	switch risk {
	case Low:
		return 0.30, nil
	case Medium:
		return 0.60, nil
	case High:
		return 0.90, nil
	default:
		return 0, ErrInvalidRisk
	}
}

// binomialCoefficient computes C(n, k) for n <= 16.
func binomialCoefficient(n, k int) uint64 {
	if k < 0 || k > n {
		return 0
	}
	if k == 0 || k == n {
		return 1
	}
	if k > n/2 {
		k = n - k
	}
	res := uint64(1)
	for i := 1; i <= k; i++ {
		res = (res * uint64(n-i+1)) / uint64(i)
	}
	return res
}

// computeMultipliers calculates the unrounded binomial multipliers for all buckets in [0, rows].
func computeMultipliers(rows int, targetRTP, alpha float64) ([]float64, error) {
	twoPowRows := math.Pow(2, float64(rows))
	p := make([]float64, rows+1)
	w := make([]float64, rows+1)
	var z float64

	for k := 0; k <= rows; k++ {
		p[k] = float64(binomialCoefficient(rows, k)) / twoPowRows
		w[k] = math.Pow(p[k], -alpha)
		if math.IsNaN(w[k]) || math.IsInf(w[k], 0) {
			return nil, ErrInvalidRiskAlpha
		}
		z += p[k] * w[k]
	}

	multipliers := make([]float64, rows+1)
	for k := 0; k <= rows; k++ {
		multipliers[k] = targetRTP * w[k] / z
		if math.IsNaN(multipliers[k]) || math.IsInf(multipliers[k], 0) {
			return nil, ErrInvalidRiskAlpha
		}
	}

	return multipliers, nil
}

func validateAndCopyMultipliers(rows int, values []float64) ([]float64, error) {
	if len(values) != rows+1 {
		return nil, ErrInvalidMultipliers
	}
	result := append([]float64(nil), values...)
	for _, value := range result {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			return nil, ErrInvalidMultipliers
		}
	}
	return result, nil
}
