package simulation_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/qxbao/qxprob/core/simulation"
)

// mockEngine is a test double for simulation.Engine[T].
type mockEngine[T any] struct {
	playFunc func(callCount int) (T, error)
	calls    int
}

func (m *mockEngine[T]) Play() (T, error) {
	m.calls++
	if m.playFunc != nil {
		return m.playFunc(m.calls)
	}
	var zero T
	return zero, nil
}

// TestRunValidation verifies that nil dependencies are rejected before starting.
func TestRunValidation(t *testing.T) {
	ctx := context.Background()
	engine := &mockEngine[int]{}
	observer := func(round uint64, result int) error { return nil }

	tests := []struct {
		name     string
		ctx      context.Context
		engine   simulation.Engine[int]
		rounds   uint64
		observer simulation.Observer[int]
		wantErr  error
	}{
		{
			name:     "nil context with positive rounds",
			ctx:      nil,
			engine:   engine,
			rounds:   10,
			observer: observer,
			wantErr:  simulation.ErrNilContext,
		},
		{
			name:     "nil context with zero rounds",
			ctx:      nil,
			engine:   engine,
			rounds:   0,
			observer: observer,
			wantErr:  simulation.ErrNilContext,
		},
		{
			name:     "nil engine with positive rounds",
			ctx:      ctx,
			engine:   nil,
			rounds:   10,
			observer: observer,
			wantErr:  simulation.ErrNilEngine,
		},
		{
			name:     "nil engine with zero rounds",
			ctx:      ctx,
			engine:   nil,
			rounds:   0,
			observer: observer,
			wantErr:  simulation.ErrNilEngine,
		},
		{
			name:     "nil observer with positive rounds",
			ctx:      ctx,
			engine:   engine,
			rounds:   10,
			observer: nil,
			wantErr:  simulation.ErrNilObserver,
		},
		{
			name:     "nil observer with zero rounds",
			ctx:      ctx,
			engine:   engine,
			rounds:   0,
			observer: nil,
			wantErr:  simulation.ErrNilObserver,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := simulation.Run(tc.ctx, tc.engine, tc.rounds, tc.observer)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Run() error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

// TestRunZeroRounds verifies that zero rounds completes with nil error and no calls.
func TestRunZeroRounds(t *testing.T) {
	ctx := context.Background()
	engine := &mockEngine[string]{}
	observerCalls := 0
	observer := func(round uint64, result string) error {
		observerCalls++
		return nil
	}

	err := simulation.Run(ctx, engine, 0, observer)
	if err != nil {
		t.Fatalf("expected nil error for zero rounds, got: %v", err)
	}
	if engine.calls != 0 {
		t.Fatalf("expected 0 engine calls, got %d", engine.calls)
	}
	if observerCalls != 0 {
		t.Fatalf("expected 0 observer calls, got %d", observerCalls)
	}
}

// TestRunMultipleRoundsAndOrdering verifies that N rounds are executed sequentially
// with correct zero-based round indices and results streamed to the observer.
func TestRunMultipleRoundsAndOrdering(t *testing.T) {
	ctx := context.Background()
	values := []int{100, 200, 300, 400, 500}
	engine := &mockEngine[int]{
		playFunc: func(callCount int) (int, error) {
			return values[callCount-1], nil
		},
	}

	type observedEntry struct {
		round  uint64
		result int
	}
	var observed []observedEntry
	observer := func(round uint64, result int) error {
		observed = append(observed, observedEntry{round: round, result: result})
		return nil
	}

	rounds := uint64(len(values))
	err := simulation.Run(ctx, engine, rounds, observer)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if engine.calls != len(values) {
		t.Fatalf("engine calls = %d, want %d", engine.calls, len(values))
	}
	if len(observed) != len(values) {
		t.Fatalf("observed entries = %d, want %d", len(observed), len(values))
	}

	for i, entry := range observed {
		if entry.round != uint64(i) {
			t.Fatalf("entry %d round = %d, want %d", i, entry.round, i)
		}
		if entry.result != values[i] {
			t.Fatalf("entry %d result = %d, want %d", i, entry.result, values[i])
		}
	}
}

// TestRunPreCancelledContext verifies that a pre-cancelled context halts before round 0.
func TestRunPreCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately before Run

	engine := &mockEngine[int]{}
	observerCalled := false
	observer := func(round uint64, result int) error {
		observerCalled = true
		return nil
	}

	err := simulation.Run(ctx, engine, 5, observer)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected error wrapping context.Canceled, got: %v", err)
	}
	if !strings.Contains(err.Error(), "round 0") {
		t.Fatalf("expected error message to contain round 0, got: %s", err.Error())
	}
	if engine.calls != 0 {
		t.Fatalf("engine calls = %d, want 0", engine.calls)
	}
	if observerCalled {
		t.Fatal("observer must not be called when pre-cancelled")
	}
}

// TestRunMidRunCancellation verifies that cancellation halts the simulation immediately.
func TestRunMidRunCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	engine := &mockEngine[int]{
		playFunc: func(callCount int) (int, error) {
			return callCount, nil
		},
	}

	observerCalls := 0
	observer := func(round uint64, result int) error {
		observerCalls++
		if round == 2 {
			cancel() // cancel context during round 2 observation
		}
		return nil
	}

	err := simulation.Run(ctx, engine, 10, observer)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected error wrapping context.Canceled, got: %v", err)
	}
	if !strings.Contains(err.Error(), "round 3") {
		t.Fatalf("expected error message to mention round 3, got: %s", err.Error())
	}
	if engine.calls != 3 {
		t.Fatalf("engine calls = %d, want 3", engine.calls)
	}
	if observerCalls != 3 {
		t.Fatalf("observer calls = %d, want 3", observerCalls)
	}
}

// TestRunEngineError verifies that an engine error halts the simulation and wraps the error.
func TestRunEngineError(t *testing.T) {
	injectedErr := errors.New("injected engine fault")
	engine := &mockEngine[int]{
		playFunc: func(callCount int) (int, error) {
			if callCount == 3 { // fail on 3rd call (round 2)
				return 0, injectedErr
			}
			return callCount, nil
		},
	}

	observerCalls := 0
	observer := func(round uint64, result int) error {
		observerCalls++
		return nil
	}

	err := simulation.Run(context.Background(), engine, 10, observer)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, injectedErr) {
		t.Fatalf("expected error wrapping injectedErr, got: %v", err)
	}
	if !strings.Contains(err.Error(), "round 2") {
		t.Fatalf("expected error message to mention round 2, got: %s", err.Error())
	}
	if engine.calls != 3 {
		t.Fatalf("engine calls = %d, want 3", engine.calls)
	}
	// Observer should not be called for the failed round
	if observerCalls != 2 {
		t.Fatalf("observer calls = %d, want 2", observerCalls)
	}
}

// TestRunObserverError verifies that an observer error halts the simulation and wraps the error.
func TestRunObserverError(t *testing.T) {
	injectedErr := errors.New("injected observer fault")
	engine := &mockEngine[int]{
		playFunc: func(callCount int) (int, error) {
			return callCount, nil
		},
	}

	observerCalls := 0
	observer := func(round uint64, result int) error {
		observerCalls++
		if round == 2 {
			return injectedErr
		}
		return nil
	}

	err := simulation.Run(context.Background(), engine, 10, observer)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, injectedErr) {
		t.Fatalf("expected error wrapping injectedErr, got: %v", err)
	}
	if !strings.Contains(err.Error(), "round 2") {
		t.Fatalf("expected error message to mention round 2, got: %s", err.Error())
	}
	if engine.calls != 3 {
		t.Fatalf("engine calls = %d, want 3", engine.calls)
	}
	if observerCalls != 3 {
		t.Fatalf("observer calls = %d, want 3", observerCalls)
	}
}

// TestRunDeadlineExceeded verifies that deadline expiration is wrapped and inspectable.
func TestRunDeadlineExceeded(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Hour))
	defer cancel()

	engine := &mockEngine[int]{}
	observer := func(round uint64, result int) error { return nil }

	err := simulation.Run(ctx, engine, 5, observer)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected error wrapping context.DeadlineExceeded, got: %v", err)
	}
}
