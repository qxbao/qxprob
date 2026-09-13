package slot

import (
	"errors"
	"fmt"
)

// ErrInvalidStateTransition indicates an illegal transition in the spin lifecycle.
var ErrInvalidStateTransition = errors.New("slot: invalid state transition")

var legalTransitions = map[State]map[State]bool{
	StateIdle: {
		StateSpinning: true,
	},
	StateSpinning: {
		StateEvaluating: true,
	},
	StateEvaluating: {
		StateCascading:      true,
		StateFeatureTrigger: true,
		StateFreeSpins:      true,
		StateSettling:       true,
	},
	StateCascading: {
		StateEvaluating:     true,
		StateFeatureTrigger: true,
		StateFreeSpins:      true,
		StateSettling:       true,
	},
	StateFeatureTrigger: {
		StateFreeSpins: true,
		StateSettling:  true,
	},
	StateFreeSpins: {
		StateSpinning: true,
		StateSettling: true,
	},
	StateSettling: {
		StateCompleted: true,
	},
}

// StateMachine enforces the strict lifecycle transition rules for slot spins.
type StateMachine struct {
	current State
}

// NewStateMachine constructs a new StateMachine initialized to StateIdle.
func NewStateMachine() *StateMachine {
	return &StateMachine{
		current: StateIdle,
	}
}

// State returns the current State.
func (m *StateMachine) State() State {
	return m.current
}

// Transition attempts to advance the state machine to target.
// It returns ErrInvalidStateTransition if the transition is illegal.
func (m *StateMachine) Transition(target State) error {
	allowed, ok := legalTransitions[m.current]
	if !ok || !allowed[target] {
		return fmt.Errorf("%w: from %s to %s", ErrInvalidStateTransition, m.current, target)
	}
	m.current = target
	return nil
}
