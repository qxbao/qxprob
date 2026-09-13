// Package slotfeature provides modular slot engine feature plugins such as cascades
// and scatter-triggered free spins.
package slotfeature

import (
	"github.com/qxbao/qxprob/core/slot"
)

// Cascade implements a hook-driven cascading wins module.
type Cascade struct {
	slot.FeatureAdapter
}

// NewCascade constructs a Cascade feature with the specified or default ID.
func NewCascade(id string) *Cascade {
	if id == "" {
		id = "cascade"
	}
	return &Cascade{
		FeatureAdapter: slot.FeatureAdapter{FeatureID: id},
	}
}

// AfterEvaluation checks for winning positions and requests a cascade refill
// with the configured multiplier mode (fixed, incremental, or table).
func (c *Cascade) AfterEvaluation(ctx *slot.FeatureContext) error {
	if ctx == nil || ctx.Definition == nil {
		return nil
	}
	vdef := ctx.Definition
	if !vdef.CascadeEnabled() {
		return nil
	}
	if len(ctx.Evaluation.Wins) == 0 {
		return nil
	}
	if ctx.CascadeIndex >= vdef.CascadeMax() {
		return nil
	}

	var mult slot.Multiplier
	switch vdef.CascadeMultiplierMode() {
	case slot.CascadeMultiplierFixed, "":
		mult = vdef.CascadeFixedMultiplier()
		if mult <= 0 {
			mult = slot.MustMultiplier("1.0")
		}
	case slot.CascadeMultiplierIncremental:
		base := slot.MustMultiplier("1.0")
		if ctx.CascadeIndex > 0 {
			step, err := vdef.CascadeMultiplierStep().MulInt(int64(ctx.CascadeIndex))
			if err != nil {
				return err
			}
			var err2 error
			mult, err2 = base.Add(step)
			if err2 != nil {
				return err2
			}
		} else {
			mult = base
		}
	case slot.CascadeMultiplierTable:
		if m, ok := vdef.CascadeTableMultiplier(ctx.CascadeIndex); ok {
			mult = m
		} else {
			mult = slot.MustMultiplier("1.0")
		}
	}

	if ctx.IsFreeSpin && vdef.FreeSpinsMultiplierPolicy() == slot.MultiplierPersistent && ctx.FreeSpinState != nil && ctx.FreeSpinState.GlobalMultiplier > slot.MustMultiplier("1.0") {
		var err error
		mult, err = mult.Mul(ctx.FreeSpinState.GlobalMultiplier)
		if err != nil {
			return err
		}
	}

	ctx.RequestCascade(mult)
	return nil
}

// AfterCascade increments the global persistent multiplier when in free spins with MultiplierPersistent policy.
func (c *Cascade) AfterCascade(ctx *slot.FeatureContext) error {
	if ctx == nil || ctx.Definition == nil {
		return nil
	}
	vdef := ctx.Definition
	if ctx.IsFreeSpin && ctx.FreeSpinState != nil && vdef.FreeSpinsMultiplierPolicy() == slot.MultiplierPersistent {
		step := vdef.CascadeMultiplierStep()
		if step <= 0 {
			step = slot.MustMultiplier("1.0")
		}
		newGlobal, err := ctx.FreeSpinState.GlobalMultiplier.Add(step)
		if err != nil {
			return err
		}
		ctx.FreeSpinState.GlobalMultiplier = newGlobal
	}
	return nil
}

// ScatterFreeSpins implements scatter-triggered free spins, retriggers, and reel switching.
type ScatterFreeSpins struct {
	slot.FeatureAdapter
}

// NewScatterFreeSpins constructs a ScatterFreeSpins feature with the specified or default ID.
func NewScatterFreeSpins(id string) *ScatterFreeSpins {
	if id == "" {
		id = "scatter_free_spins"
	}
	return &ScatterFreeSpins{
		FeatureAdapter: slot.FeatureAdapter{FeatureID: id},
	}
}

// BeforeSpin selects the configured free spins reel set when a free spin executes.
func (s *ScatterFreeSpins) BeforeSpin(ctx *slot.FeatureContext) error {
	if ctx == nil || ctx.Definition == nil {
		return nil
	}
	if ctx.IsFreeSpin {
		if rs := ctx.Definition.FreeSpinsReelSet(); rs != "" {
			ctx.SelectReelSet(rs)
		}
	}
	return nil
}

// AfterEvaluation inspects scatter triggers and awards initial or retriggered free spins.
func (s *ScatterFreeSpins) AfterEvaluation(ctx *slot.FeatureContext) error {
	if ctx == nil || ctx.Definition == nil {
		return nil
	}
	vdef := ctx.Definition
	if !vdef.FreeSpinsEnabled() {
		return nil
	}

	totalAwarded := 0
	for _, tr := range ctx.Evaluation.Triggers {
		if tr.Kind == slot.TriggerFreeSpins {
			totalAwarded += tr.Award
		}
	}
	if totalAwarded == 0 {
		return nil
	}

	maxSpins := vdef.FreeSpinsMaxTotal()
	reelSet := vdef.FreeSpinsReelSet()

	if !ctx.IsFreeSpin {
		if ctx.AwardSpins > 0 {
			return nil
		}
		award := totalAwarded
		if maxSpins > 0 && award > maxSpins {
			award = maxSpins
		}
		ctx.AwardFreeSpins(award)
		if reelSet != "" {
			ctx.SelectReelSet(reelSet)
		}
	} else {
		if !vdef.FreeSpinsRetriggerEnabled() {
			return nil
		}
		currentTotal := 0
		if ctx.FreeSpinState != nil {
			currentTotal = ctx.FreeSpinState.TotalSpins
		}
		if maxSpins > 0 && currentTotal >= maxSpins {
			return nil
		}
		award := totalAwarded
		if maxSpins > 0 && currentTotal+award > maxSpins {
			award = maxSpins - currentTotal
		}
		if award > 0 {
			ctx.RetriggerFreeSpins(award)
		}
	}
	return nil
}

// NewBuiltins creates the built-in feature modules enabled by the definition.
func NewBuiltins(def *slot.ValidatedDefinition) []slot.Feature {
	if def == nil {
		return nil
	}
	var features []slot.Feature

	if def.CascadeEnabled() {
		features = append(features, NewCascade("cascade"))
	}
	if def.FreeSpinsEnabled() {
		features = append(features, NewScatterFreeSpins("scatter_free_spins"))
	}

	return features
}
