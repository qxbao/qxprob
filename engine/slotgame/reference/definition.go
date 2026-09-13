// Package reference provides the versioned 5x4 1024-ways Reference Ways Slot definition
// and feature factory.
package reference

import (
	"github.com/qxbao/qxprob/core/slot"
	"github.com/qxbao/qxprob/engine/slotfeature"
)

// GameID is the canonical identifier for the Reference Ways Slot game.
const GameID = "reference-ways-slot"

// ConfigVersion identifies the current configuration revision of the reference game.
const ConfigVersion = "1.0.0"

// Definition returns a fresh, unaliased Definition for the Reference Ways Slot.
// Note: Paytable and reel strips represent math tuning inputs, not certified production math.
func Definition() slot.Definition {
	return slot.Definition{
		ID:      GameID,
		Version: ConfigVersion,
		Name:    "Reference Ways Slot",
		Grid: slot.GridConfig{
			Reels: 5,
			Rows:  4,
		},
		WinMechanic: slot.WinMechanicConfig{
			Type:     slot.WinMechanicWays,
			MinMatch: 3,
		},
		Symbols: []slot.SymbolConfig{
			{ID: "J", Name: "Jack", Type: slot.SymbolRegular, Payouts: []slot.Multiplier{0, 0, 0, slot.MustMultiplier("0.0036"), slot.MustMultiplier("0.0075"), slot.MustMultiplier("0.0187")}},
			{ID: "Q", Name: "Queen", Type: slot.SymbolRegular, Payouts: []slot.Multiplier{0, 0, 0, slot.MustMultiplier("0.0055"), slot.MustMultiplier("0.0138"), slot.MustMultiplier("0.0327")}},
			{ID: "K", Name: "King", Type: slot.SymbolRegular, Payouts: []slot.Multiplier{0, 0, 0, slot.MustMultiplier("0.0075"), slot.MustMultiplier("0.0168"), slot.MustMultiplier("0.0416")}},
			{ID: "A", Name: "Ace", Type: slot.SymbolRegular, Payouts: []slot.Multiplier{0, 0, 0, slot.MustMultiplier("0.012"), slot.MustMultiplier("0.0258"), slot.MustMultiplier("0.0605")}},
			{ID: "Gem", Name: "Gem", Type: slot.SymbolRegular, Payouts: []slot.Multiplier{0, 0, 0, slot.MustMultiplier("0.0227"), slot.MustMultiplier("0.0515"), slot.MustMultiplier("0.1209")}},
			{ID: "Crown", Name: "Crown", Type: slot.SymbolRegular, Payouts: []slot.Multiplier{0, 0, 0, slot.MustMultiplier("0.0466"), slot.MustMultiplier("0.102"), slot.MustMultiplier("0.2606")}},
			{ID: "Dragon", Name: "Dragon", Type: slot.SymbolRegular, Payouts: []slot.Multiplier{0, 0, 0, slot.MustMultiplier("0.0931"), slot.MustMultiplier("0.2328"), slot.MustMultiplier("0.6043")}},
			{ID: "Wild", Name: "Wild", Type: slot.SymbolWild},
			{ID: "Scatter", Name: "Scatter", Type: slot.SymbolScatter},
		},
		BaseReelSet: "base",
		ReelSets: map[string]slot.ReelSet{
			"base": {
				Reels: [][]slot.SymbolID{
					// Reel 0 (48 symbols, 0 Wild, 1 Scatter)
					{"J", "Q", "K", "J", "A", "Gem", "Q", "K", "J", "Crown", "Q", "A", "Dragon", "K", "J", "Q", "Gem", "K", "A", "J", "Scatter", "Q", "Crown", "K", "A", "J", "Q", "Gem", "K", "J", "Q", "A", "Dragon", "K", "J", "Q", "Gem", "Crown", "J", "A", "K", "Q", "J", "A", "K", "Gem", "Q", "J"},
					// Reel 1 (48 symbols, 1 Wild, 1 Scatter)
					{"J", "Q", "K", "J", "A", "Gem", "Q", "K", "J", "Crown", "Q", "A", "Dragon", "K", "J", "Wild", "Q", "Gem", "K", "A", "J", "Scatter", "Q", "Crown", "K", "A", "J", "Q", "Gem", "K", "J", "Q", "A", "Dragon", "K", "J", "Q", "Gem", "Crown", "Q", "K", "J", "A", "Q", "K", "Gem", "J", "Q"},
					// Reel 2 (48 symbols, 1 Wild, 1 Scatter)
					{"Q", "K", "J", "Q", "A", "Gem", "K", "J", "Q", "Crown", "A", "K", "Dragon", "J", "Q", "Wild", "K", "Gem", "A", "J", "Q", "Scatter", "K", "Crown", "A", "J", "Q", "Gem", "K", "Q", "J", "A", "Dragon", "J", "Q", "K", "Gem", "Crown", "K", "J", "A", "K", "Q", "J", "A", "Gem", "K", "J"},
					// Reel 3 (48 symbols, 1 Wild, 1 Scatter)
					{"K", "A", "Q", "K", "J", "Gem", "A", "Q", "K", "Crown", "J", "A", "Dragon", "Q", "K", "Wild", "A", "Gem", "J", "Q", "K", "Scatter", "A", "Crown", "J", "Q", "K", "Gem", "A", "K", "Q", "J", "Dragon", "Q", "K", "A", "Gem", "Crown", "A", "Q", "J", "A", "K", "Q", "J", "Gem", "A", "K"},
					// Reel 4 (48 symbols, 1 Wild, 1 Scatter)
					{"A", "J", "K", "A", "Q", "Gem", "J", "K", "A", "Crown", "Q", "J", "Dragon", "K", "A", "Wild", "J", "Gem", "Q", "K", "A", "Scatter", "J", "Crown", "Q", "K", "A", "Gem", "J", "A", "K", "Q", "Dragon", "K", "A", "J", "Gem", "Crown", "J", "K", "Q", "J", "A", "K", "Q", "Gem", "J", "K"},
				},
			},
			"free_spins": {
				Reels: [][]slot.SymbolID{
					// Free spin reel 0 (48 symbols, 1 Scatter, 0 Wild)
					{"K", "A", "Gem", "Crown", "Dragon", "Scatter", "K", "A", "Gem", "Crown", "Q", "K", "A", "Gem", "Dragon", "K", "A", "Crown", "Gem", "Q", "K", "A", "Dragon", "Crown", "Gem", "K", "A", "Q", "Gem", "Crown", "K", "A", "Q", "J", "K", "J", "Q", "Dragon", "J", "A", "Crown", "Gem", "K", "A", "Q", "Gem", "Crown", "K"},
					// Free spin reel 1 (48 symbols, 1 Wild, 1 Scatter)
					{"Gem", "Wild", "Crown", "Dragon", "A", "Scatter", "Gem", "Crown", "K", "Dragon", "A", "Gem", "Crown", "K", "Q", "Gem", "Dragon", "A", "J", "Crown", "K", "Q", "Gem", "Dragon", "A", "J", "Crown", "Gem", "K", "Q", "J", "A", "K", "Q", "J", "Crown", "Gem", "K", "Dragon", "A", "Gem", "Crown", "K", "Q", "J", "A", "K", "Q"},
					// Free spin reel 2 (48 symbols, 1 Wild, 1 Scatter)
					{"Crown", "Wild", "Dragon", "Gem", "A", "Scatter", "Crown", "Dragon", "K", "Gem", "A", "Crown", "Dragon", "K", "Q", "Crown", "Gem", "A", "J", "Dragon", "K", "Q", "Crown", "Gem", "A", "J", "Dragon", "Crown", "K", "Q", "J", "A", "K", "Q", "J", "Crown", "Dragon", "K", "Gem", "A", "Crown", "Dragon", "K", "Q", "J", "A", "K", "Q"},
					// Free spin reel 3 (48 symbols, 1 Wild, 1 Scatter)
					{"Dragon", "Wild", "Gem", "Crown", "A", "Scatter", "Dragon", "Gem", "K", "Crown", "A", "Dragon", "Gem", "K", "Q", "Dragon", "Crown", "A", "J", "Gem", "K", "Q", "Dragon", "Crown", "A", "J", "Gem", "Dragon", "K", "Q", "J", "A", "K", "Q", "J", "Crown", "Gem", "A", "Dragon", "K", "Crown", "A", "Gem", "K", "Q", "J", "A", "K"},
					// Free spin reel 4 (48 symbols, 1 Wild, 1 Scatter)
					{"Gem", "Wild", "Crown", "Dragon", "A", "Scatter", "Gem", "Crown", "K", "Dragon", "A", "Gem", "Crown", "K", "Q", "Gem", "Dragon", "A", "J", "Crown", "K", "Q", "Gem", "Dragon", "A", "J", "Crown", "Gem", "K", "Q", "J", "A", "K", "Q", "J", "Crown", "Dragon", "A", "Gem", "K", "Crown", "Dragon", "A", "K", "Q", "J", "A", "K"},
				},
			},
		},
		Wild: slot.WildConfig{
			WildSymbolIDs: []slot.SymbolID{"Wild"},
		},
		Scatter: slot.ScatterConfig{
			SymbolID: "Scatter",
			Thresholds: []slot.ScatterTrigger{
				{Count: 3, Payout: slot.MustMultiplier("1.0"), Triggers: []slot.Trigger{{Kind: slot.TriggerFreeSpins, Award: 8}}},
				{Count: 4, Payout: slot.MustMultiplier("5.0"), Triggers: []slot.Trigger{{Kind: slot.TriggerFreeSpins, Award: 12}}},
				{Count: 5, Payout: slot.MustMultiplier("20.0"), Triggers: []slot.Trigger{{Kind: slot.TriggerFreeSpins, Award: 15}}},
			},
		},
		Cascade: slot.CascadeConfig{
			Enabled:        true,
			MaxCascades:    20,
			MultiplierMode: slot.CascadeMultiplierTable,
			MultiplierTable: []slot.Multiplier{
				slot.MustMultiplier("1.0"),
				slot.MustMultiplier("2.0"),
				slot.MustMultiplier("3.0"),
				slot.MustMultiplier("5.0"),
				slot.MustMultiplier("10.0"),
			},
		},
		FreeSpins: slot.FreeSpinsConfig{
			Enabled:          true,
			ReelSet:          "free_spins",
			MaxTotalSpins:    50,
			MultiplierPolicy: slot.MultiplierResetEachSpin,
			RetriggerEnabled: true,
		},
		Features: []slot.FeatureConfig{
			{ID: "cascade", Enabled: true},
			{ID: "scatter_free_spins", Enabled: true},
		},
		Math: slot.MathProfile{
			TargetRTP:  0.96,
			Volatility: "medium-high",
		},
		MaxWinMultiplier: slot.MustMultiplier("10000.0"),
		MaxWinPolicy:     slot.MaxWinCapOnly,
	}
}

// Features returns the ordered feature modules required by the reference definition.
func Features(def *slot.ValidatedDefinition) []slot.Feature {
	return slotfeature.NewBuiltins(def)
}
