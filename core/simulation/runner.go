// Package simulation provides a generic, sequential, streaming simulation runner for game engines.
package simulation

import (
	"context"
	"errors"
	"fmt"
)

var (
	// ErrNilContext indicates that a nil context was provided.
	ErrNilContext = errors.New("simulation: context must not be nil")

	// ErrNilEngine indicates that a nil engine was provided.
	ErrNilEngine = errors.New("simulation: engine must not be nil")

	// ErrNilObserver indicates that a nil observer was provided.
	ErrNilObserver = errors.New("simulation: observer must not be nil")
)

// Engine defines the contract for game engines that can be simulated.
type Engine[T any] interface {
	Play() (T, error)
}

// Observer is called for each simulated round with its zero-based index and result.
type Observer[T any] func(round uint64, result T) error

// Run executes rounds of simulation sequentially, streaming each result to observe.
// It checks ctx for cancellation before each round.
func Run[T any](ctx context.Context, engine Engine[T], rounds uint64, observe Observer[T]) error {
	if ctx == nil {
		return ErrNilContext
	}
	if engine == nil {
		return ErrNilEngine
	}
	if observe == nil {
		return ErrNilObserver
	}

	for round := range rounds {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("simulation: round %d context error: %w", round, err)
		}

		result, err := engine.Play()
		if err != nil {
			return fmt.Errorf("simulation: round %d engine error: %w", round, err)
		}

		if err := observe(round, result); err != nil {
			return fmt.Errorf("simulation: round %d observer error: %w", round, err)
		}
	}

	return nil
}
