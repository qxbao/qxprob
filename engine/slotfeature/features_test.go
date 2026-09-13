package slotfeature_test

import (
	"testing"

	"github.com/qxbao/qxprob/core/slot"
	"github.com/qxbao/qxprob/engine/slotfeature"
)

func baseDef() slot.Definition {
	return slot.Definition{
		ID:      "test-features",
		Version: "1.0.0",
		Name:    "Features Test",
		Grid: slot.GridConfig{
			Reels: 5,
			Rows:  3,
		},
		WinMechanic: slot.WinMechanicConfig{
			Type:     slot.WinMechanicWays,
			MinMatch: 3,
		},
		Symbols: []slot.SymbolConfig{
			{ID: "SYM", Type: slot.SymbolRegular, Payouts: []slot.Multiplier{0, 0, 0, slot.MustMultiplier("1.0"), slot.MustMultiplier("2.0"), slot.MustMultiplier("5.0")}},
			{ID: "SCAT", Type: slot.SymbolScatter},
		},
		ReelSets: map[string]slot.ReelSet{
			"base": {
				Reels: [][]slot.SymbolID{
					{"SYM", "SYM", "SYM"},
					{"SYM", "SYM", "SYM"},
					{"SYM", "SYM", "SYM"},
					{"SYM", "SYM", "SYM"},
					{"SYM", "SYM", "SYM"},
				},
			},
			"fs_reels": {
				Reels: [][]slot.SymbolID{
					{"SYM", "SYM", "SCAT"},
					{"SYM", "SYM", "SCAT"},
					{"SYM", "SYM", "SCAT"},
					{"SYM", "SYM", "SCAT"},
					{"SYM", "SYM", "SCAT"},
				},
			},
		},
		Cascade: slot.CascadeConfig{
			Enabled:        true,
			MaxCascades:    5,
			MultiplierMode: slot.CascadeMultiplierTable,
			MultiplierTable: []slot.Multiplier{
				slot.MustMultiplier("1.0"),
				slot.MustMultiplier("2.0"),
				slot.MustMultiplier("3.0"),
				slot.MustMultiplier("5.0"),
			},
		},
		FreeSpins: slot.FreeSpinsConfig{
			Enabled:          true,
			ReelSet:          "fs_reels",
			MaxTotalSpins:    20,
			RetriggerEnabled: true,
		},
	}
}

func TestCascade_MultiplierTableAndRepeat(t *testing.T) {
	d := baseDef()
	vdef, err := slot.ValidateDefinition(d)
	if err != nil {
		t.Fatal(err)
	}

	cascade := slotfeature.NewCascade("")
	if cascade.ID() != "cascade" {
		t.Errorf("expected ID 'cascade', got %s", cascade.ID())
	}

	// Case 1: wins present, cascade index 1 -> table[1] = 2.0x
	ctx := &slot.FeatureContext{
		Definition:   vdef,
		CascadeIndex: 1,
		Evaluation: slot.Evaluation{
			Wins: []slot.Win{{Symbol: "SYM", MatchedReels: 3}},
		},
	}
	if err := cascade.AfterEvaluation(ctx); err != nil {
		t.Fatal(err)
	}
	if !ctx.CascadeRequested || ctx.CascadeMultiplier != slot.MustMultiplier("2.0") {
		t.Errorf("expected 2.0x cascade, got %v, mult=%s", ctx.CascadeRequested, ctx.CascadeMultiplier)
	}

	// Case 2: cascade index 10 (beyond table length 4) -> repeats final entry table[3] = 5.0x
	ctx.ResetRoundDecisions()
	ctx.CascadeIndex = 3 // index 3 is 5.0x
	if err := cascade.AfterEvaluation(ctx); err != nil {
		t.Fatal(err)
	}
	if ctx.CascadeMultiplier != slot.MustMultiplier("5.0") {
		t.Errorf("expected final repeated entry 5.0x, got %s", ctx.CascadeMultiplier)
	}

	// Case 3: beyond max cascades (5) -> no cascade requested
	ctx.ResetRoundDecisions()
	ctx.CascadeIndex = 5
	if err := cascade.AfterEvaluation(ctx); err != nil {
		t.Fatal(err)
	}
	if ctx.CascadeRequested {
		t.Errorf("expected no cascade when index >= maxCascades")
	}

	// Case 4: no wins -> no cascade requested
	ctx.ResetRoundDecisions()
	ctx.CascadeIndex = 0
	ctx.Evaluation.Wins = nil
	if err := cascade.AfterEvaluation(ctx); err != nil {
		t.Fatal(err)
	}
	if ctx.CascadeRequested {
		t.Errorf("expected no cascade when no wins")
	}
}

func TestCascade_IncrementalAndFixed(t *testing.T) {
	d := baseDef()
	d.Cascade.MultiplierMode = slot.CascadeMultiplierIncremental
	d.Cascade.MultiplierStep = slot.MustMultiplier("0.5")
	vdef, err := slot.ValidateDefinition(d)
	if err != nil {
		t.Fatal(err)
	}

	cascade := slotfeature.NewCascade("my_cascade")
	ctx := &slot.FeatureContext{
		Definition:   vdef,
		CascadeIndex: 2, // 1.0 + 2 * 0.5 = 2.0x
		Evaluation: slot.Evaluation{
			Wins: []slot.Win{{Symbol: "SYM"}},
		},
	}
	if err := cascade.AfterEvaluation(ctx); err != nil {
		t.Fatal(err)
	}
	if ctx.CascadeMultiplier != slot.MustMultiplier("2.0") {
		t.Errorf("incremental multiplier got %s, want 2.0", ctx.CascadeMultiplier)
	}
}

func TestScatterFreeSpins_AwardAndRetrigger(t *testing.T) {
	d := baseDef()
	vdef, err := slot.ValidateDefinition(d)
	if err != nil {
		t.Fatal(err)
	}

	fs := slotfeature.NewScatterFreeSpins("")

	// Case 1: Trigger in base game
	ctx := &slot.FeatureContext{
		Definition: vdef,
		IsFreeSpin: false,
		Evaluation: slot.Evaluation{
			Triggers: []slot.Trigger{{Kind: slot.TriggerFreeSpins, Award: 8}},
		},
	}
	if err := fs.AfterEvaluation(ctx); err != nil {
		t.Fatal(err)
	}
	if ctx.AwardSpins != 8 {
		t.Errorf("expected 8 awarded spins, got %d", ctx.AwardSpins)
	}
	if ctx.ActiveReelSet != "fs_reels" {
		t.Errorf("expected active reel set fs_reels, got %s", ctx.ActiveReelSet)
	}

	// Case 2: BeforeSpin in free spin mode selects free spin reelset
	ctx2 := &slot.FeatureContext{
		Definition: vdef,
		IsFreeSpin: true,
	}
	if err := fs.BeforeSpin(ctx2); err != nil {
		t.Fatal(err)
	}
	if ctx2.ActiveReelSet != "fs_reels" {
		t.Errorf("BeforeSpin expected fs_reels, got %s", ctx2.ActiveReelSet)
	}

	// Case 3: Retrigger in free spin mode
	ctx3 := &slot.FeatureContext{
		Definition: vdef,
		IsFreeSpin: true,
		FreeSpinState: &slot.FreeSpinState{
			TotalSpins: 15,
		},
		Evaluation: slot.Evaluation{
			Triggers: []slot.Trigger{{Kind: slot.TriggerFreeSpins, Award: 8}},
		},
	}
	// MaxTotalSpins is 20, currently 15, so only 5 can be retriggered (20 - 15 = 5)
	if err := fs.AfterEvaluation(ctx3); err != nil {
		t.Fatal(err)
	}
	if ctx3.RetriggerSpins != 5 {
		t.Errorf("expected 5 retriggered spins capped by max 20, got %d", ctx3.RetriggerSpins)
	}
}

func TestNewBuiltins(t *testing.T) {
	vdef, err := slot.ValidateDefinition(baseDef())
	if err != nil {
		t.Fatal(err)
	}

	builtins := slotfeature.NewBuiltins(vdef)
	if len(builtins) != 2 {
		t.Fatalf("expected 2 built-in features, got %d", len(builtins))
	}
	if builtins[0].ID() != "cascade" || builtins[1].ID() != "scatter_free_spins" {
		t.Errorf("builtins mismatch: %s, %s", builtins[0].ID(), builtins[1].ID())
	}
}

func TestBuiltinFeatures_ZeroAllocations(t *testing.T) {
	vdef, err := slot.ValidateDefinition(baseDef())
	if err != nil {
		t.Fatal(err)
	}

	cascade := slotfeature.NewCascade("cascade")
	sfs := slotfeature.NewScatterFreeSpins("scatter_free_spins")

	ctxCascade := &slot.FeatureContext{
		Definition:   vdef,
		CascadeIndex: 1,
		Evaluation: slot.Evaluation{
			Wins: []slot.Win{{Symbol: "SYM", MatchedReels: 3}},
		},
	}

	allocs := testing.AllocsPerRun(100, func() {
		ctxCascade.ResetRoundDecisions()
		if err := cascade.AfterEvaluation(ctxCascade); err != nil {
			t.Fatal(err)
		}
	})
	if allocs > 0 {
		t.Errorf("Cascade.AfterEvaluation allocated %.2f objects per run, want 0", allocs)
	}

	fsState := &slot.FreeSpinState{
		GlobalMultiplier: slot.MustMultiplier("1.0"),
	}
	ctxAfterCascade := &slot.FeatureContext{
		Definition:    vdef,
		IsFreeSpin:    true,
		FreeSpinState: fsState,
	}
	allocs = testing.AllocsPerRun(100, func() {
		if err := cascade.AfterCascade(ctxAfterCascade); err != nil {
			t.Fatal(err)
		}
	})
	if allocs > 0 {
		t.Errorf("Cascade.AfterCascade allocated %.2f objects per run, want 0", allocs)
	}

	ctxSFSBefore := &slot.FeatureContext{
		Definition: vdef,
		IsFreeSpin: true,
	}
	allocs = testing.AllocsPerRun(100, func() {
		ctxSFSBefore.ActiveReelSet = ""
		if err := sfs.BeforeSpin(ctxSFSBefore); err != nil {
			t.Fatal(err)
		}
	})
	if allocs > 0 {
		t.Errorf("ScatterFreeSpins.BeforeSpin allocated %.2f objects per run, want 0", allocs)
	}

	ctxSFSAfter := &slot.FeatureContext{
		Definition: vdef,
		Evaluation: slot.Evaluation{
			Triggers: []slot.Trigger{{Kind: slot.TriggerFreeSpins, Award: 10}},
		},
	}
	allocs = testing.AllocsPerRun(100, func() {
		ctxSFSAfter.AwardSpins = 0
		if err := sfs.AfterEvaluation(ctxSFSAfter); err != nil {
			t.Fatal(err)
		}
	})
	if allocs > 0 {
		t.Errorf("ScatterFreeSpins.AfterEvaluation allocated %.2f objects per run, want 0", allocs)
	}
}

func TestBuiltinFeatures_ConcurrentSafety(t *testing.T) {
	vdef, err := slot.ValidateDefinition(baseDef())
	if err != nil {
		t.Fatal(err)
	}

	cascade := slotfeature.NewCascade("cascade")
	sfs := slotfeature.NewScatterFreeSpins("scatter_free_spins")

	const goroutines = 50
	const iterations = 100

	done := make(chan bool, goroutines)
	for g := 0; g < goroutines; g++ {
		go func(gID int) {
			defer func() { done <- true }()
			for i := 0; i < iterations; i++ {
				// Cascade test
				cCtx := &slot.FeatureContext{
					Definition:   vdef,
					CascadeIndex: gID % 4,
					Evaluation: slot.Evaluation{
						Wins: []slot.Win{{Symbol: "SYM", MatchedReels: 3}},
					},
				}
				if err := cascade.AfterEvaluation(cCtx); err != nil {
					t.Errorf("goroutine %d: cascade AfterEvaluation failed: %v", gID, err)
					return
				}
				if !cCtx.CascadeRequested {
					t.Errorf("goroutine %d: expected cascade requested", gID)
					return
				}

				// ScatterFreeSpins BeforeSpin test
				fsCtx := &slot.FeatureContext{
					Definition: vdef,
					IsFreeSpin: true,
				}
				if err := sfs.BeforeSpin(fsCtx); err != nil {
					t.Errorf("goroutine %d: sfs BeforeSpin failed: %v", gID, err)
					return
				}
				if fsCtx.ActiveReelSet != "fs_reels" {
					t.Errorf("goroutine %d: expected fs_reels, got %s", gID, fsCtx.ActiveReelSet)
					return
				}

				// ScatterFreeSpins AfterEvaluation test
				fsCtx2 := &slot.FeatureContext{
					Definition: vdef,
					IsFreeSpin: false,
					Evaluation: slot.Evaluation{
						Triggers: []slot.Trigger{{Kind: slot.TriggerFreeSpins, Award: 8}},
					},
				}
				if err := sfs.AfterEvaluation(fsCtx2); err != nil {
					t.Errorf("goroutine %d: sfs AfterEvaluation failed: %v", gID, err)
					return
				}
				if fsCtx2.AwardSpins != 8 {
					t.Errorf("goroutine %d: expected 8 spins, got %d", gID, fsCtx2.AwardSpins)
					return
				}
			}
		}(g)
	}

	for g := 0; g < goroutines; g++ {
		<-done
	}
}
