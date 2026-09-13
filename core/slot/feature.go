package slot

import (
	"encoding/json"
)

// Feature defines the lifecycle hook interface for modular slot game extensions.
type Feature interface {
	// ID returns the unique identifier for this feature module.
	ID() string
	// BeforeSpin is called before any reel generation occurs for a spin round.
	BeforeSpin(ctx *FeatureContext) error
	// AfterGridGenerated is called after the grid is sampled and revealed.
	AfterGridGenerated(ctx *FeatureContext) error
	// BeforeEvaluation is called immediately prior to evaluating grid wins.
	BeforeEvaluation(ctx *FeatureContext) error
	// AfterEvaluation is called after grid evaluation completes, before cascades or payouts.
	AfterEvaluation(ctx *FeatureContext) error
	// AfterCascade is called after symbols are removed and refill grid is constructed.
	AfterCascade(ctx *FeatureContext) error
	// AfterSpin is called at the conclusion of the entire spin round.
	AfterSpin(ctx *FeatureContext) error
}

// FeatureAdapter provides no-op default implementations for all Feature hooks.
type FeatureAdapter struct {
	FeatureID string
}

// ID returns the adapter's feature ID.
func (a *FeatureAdapter) ID() string {
	return a.FeatureID
}

// BeforeSpin default no-op.
func (a *FeatureAdapter) BeforeSpin(*FeatureContext) error { return nil }

// AfterGridGenerated default no-op.
func (a *FeatureAdapter) AfterGridGenerated(*FeatureContext) error { return nil }

// BeforeEvaluation default no-op.
func (a *FeatureAdapter) BeforeEvaluation(*FeatureContext) error { return nil }

// AfterEvaluation default no-op.
func (a *FeatureAdapter) AfterEvaluation(*FeatureContext) error { return nil }

// AfterCascade default no-op.
func (a *FeatureAdapter) AfterCascade(*FeatureContext) error { return nil }

// AfterSpin default no-op.
func (a *FeatureAdapter) AfterSpin(*FeatureContext) error { return nil }

// FeatureContext carries the engine state and captures feature decisions across lifecycle hooks.
type FeatureContext struct {
	Definition        *ValidatedDefinition
	Mode              PlayMode
	IsFreeSpin        bool
	RoundIndex        int
	CascadeIndex      int
	Grid              Grid
	Evaluation        Evaluation
	FreeSpinState     *FreeSpinState
	ActiveReelSet     string
	CascadeRequested  bool
	CascadeMultiplier Multiplier
	AwardSpins        int
	RetriggerSpins    int
	AppliedMultiplier Multiplier
	TerminateFeature  bool
	TerminateSession  bool
	State             json.RawMessage
}

// RequestCascade instructs the engine to perform a cascade with the specified multiplier.
func (c *FeatureContext) RequestCascade(mult Multiplier) {
	c.CascadeRequested = true
	c.CascadeMultiplier = mult
}

// AwardFreeSpins queues free spins to be awarded to the session.
func (c *FeatureContext) AwardFreeSpins(spins int) {
	if spins > 0 {
		c.AwardSpins += spins
	}
}

// RetriggerFreeSpins queues additional retriggered free spins.
func (c *FeatureContext) RetriggerFreeSpins(spins int) {
	if spins > 0 {
		c.RetriggerSpins += spins
	}
}

// SelectReelSet sets the reel set to use for upcoming grid sampling.
func (c *FeatureContext) SelectReelSet(name string) {
	c.ActiveReelSet = name
}

// SetAppliedMultiplier applies a multiplier adjustment to the current round.
func (c *FeatureContext) SetAppliedMultiplier(mult Multiplier) {
	c.AppliedMultiplier = mult
}

// Terminate instructs the engine to terminate the current feature or entire session early.
func (c *FeatureContext) Terminate(terminateSession bool) {
	c.TerminateFeature = true
	if terminateSession {
		c.TerminateSession = true
	}
}

// ResetRoundDecisions clears transient decision flags between evaluation cycles.
func (c *FeatureContext) ResetRoundDecisions() {
	c.CascadeRequested = false
	c.CascadeMultiplier = 0
}
