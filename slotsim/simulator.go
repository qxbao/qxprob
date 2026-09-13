package slotsim

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/qxbao/qxprob/core/slot"
)

var (
	// ErrNilContext indicates that a nil context was provided.
	ErrNilContext = errors.New("slotsim: context must not be nil")
	// ErrNilFactory indicates that a nil spinner factory was provided.
	ErrNilFactory = errors.New("slotsim: factory must not be nil")
	// ErrNilRunner indicates that a nil runner was provided.
	ErrNilRunner = errors.New("slotsim: runner must not be nil")
	// ErrZeroSpins indicates that the requested spin count was zero.
	ErrZeroSpins = errors.New("slotsim: spin count must be greater than zero")
	// ErrInvalidBet indicates that the bet amount was non-positive.
	ErrInvalidBet = errors.New("slotsim: bet must be greater than zero")
)

// Options holds parameters governing a Monte Carlo simulation execution.
type Options struct {
	Spins uint64
	Bet   slot.Amount
}

// Spinner defines the narrow contract required by the simulator to execute a round.
type Spinner interface {
	Spin(slot.SpinRequest) (slot.SpinResult, error)
}

// Factory constructs a Spinner instance for the given nonce round counter.
//
// Deprecated: Use Runner and RunWithRunner to avoid reconstructing the game engine per round.
type Factory func(nonce uint64) (Spinner, error)

// Runner executes a single simulation round for the specified nonce and request.
// It enables callers to reuse immutable compiled game definitions and resettable entropy sources.
type Runner func(nonce uint64, req slot.SpinRequest) (slot.SpinResult, error)

// Run executes a streaming Monte Carlo simulation across the requested number of spins.
// Nonces advance monotonically from 1 to opts.Spins. Memory consumption is bounded.
//
// Deprecated: Use RunWithRunner with a reusable compiled game and resettable entropy source instead.
func Run(ctx context.Context, opts Options, factory Factory) (Report, error) {
	if ctx == nil {
		return Report{}, ErrNilContext
	}
	if factory == nil {
		return Report{}, ErrNilFactory
	}
	if opts.Spins == 0 {
		return Report{}, ErrZeroSpins
	}
	if opts.Bet <= 0 {
		return Report{}, ErrInvalidBet
	}

	return runInternal(ctx, opts, func(nonce uint64, req slot.SpinRequest) (slot.SpinResult, error) {
		spinner, err := factory(nonce)
		if err != nil {
			return slot.SpinResult{}, fmt.Errorf("slotsim: factory error at nonce %d: %w", nonce, err)
		}

		res, err := spinner.Spin(req)
		if err != nil {
			return slot.SpinResult{}, fmt.Errorf("slotsim: spin error at nonce %d: %w", nonce, err)
		}
		return res, nil
	})
}

// RunWithRunner executes a streaming Monte Carlo simulation across the requested number of spins,
// sequentially invoking the supplied Runner for nonces 1 through opts.Spins.
// Memory consumption is bounded, and immutable game state is reused across rounds.
func RunWithRunner(ctx context.Context, opts Options, runner Runner) (Report, error) {
	if ctx == nil {
		return Report{}, ErrNilContext
	}
	if runner == nil {
		return Report{}, ErrNilRunner
	}
	if opts.Spins == 0 {
		return Report{}, ErrZeroSpins
	}
	if opts.Bet <= 0 {
		return Report{}, ErrInvalidBet
	}

	return runInternal(ctx, opts, func(nonce uint64, req slot.SpinRequest) (slot.SpinResult, error) {
		res, err := runner(nonce, req)
		if err != nil {
			return slot.SpinResult{}, fmt.Errorf("slotsim: runner error at nonce %d: %w", nonce, err)
		}
		return res, nil
	})
}

func runInternal(ctx context.Context, opts Options, runFn func(nonce uint64, req slot.SpinRequest) (slot.SpinResult, error)) (Report, error) {
	start := time.Now()
	acc := newAccumulator(opts.Bet)

	for nonce := uint64(1); nonce <= opts.Spins; nonce++ {
		if err := ctx.Err(); err != nil {
			return Report{}, fmt.Errorf("slotsim: cancelled at nonce %d: %w", nonce, err)
		}

		res, err := runFn(nonce, slot.SpinRequest{
			Bet:  opts.Bet,
			Mode: slot.PlayModeSimulation,
		})
		if err != nil {
			return Report{}, err
		}

		if err := acc.add(res); err != nil {
			return Report{}, fmt.Errorf("slotsim: accumulator error at nonce %d: %w", nonce, err)
		}
	}

	elapsed := time.Since(start).Seconds()
	return acc.finish(elapsed)
}
