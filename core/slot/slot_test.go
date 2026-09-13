package slot_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/qxbao/qxprob/core/slot"
)

// fakeSource implements slot.RandomSource with fixed sequences.
type fakeSource struct {
	uint64Vals []uint64
	intnVals   []uint64
	err        error
	intnIdx    int
	uint64Idx  int
}

func (f *fakeSource) Uint64() (uint64, error) {
	if f.err != nil {
		return 0, f.err
	}
	if f.uint64Idx < len(f.uint64Vals) {
		val := f.uint64Vals[f.uint64Idx]
		f.uint64Idx++
		return val, nil
	}
	return 0, nil
}

func (f *fakeSource) Float64() (float64, error) {
	u, err := f.Uint64()
	if err != nil {
		return 0, err
	}
	return float64(u) / float64(uint64(1)<<53), nil
}

func (f *fakeSource) Intn(n uint64) (uint64, error) {
	if f.err != nil {
		return 0, f.err
	}
	if f.intnIdx < len(f.intnVals) {
		val := f.intnVals[f.intnIdx]
		f.intnIdx++
		return val % n, nil
	}
	return 0, nil
}

func sampleDefinition() slot.Definition {
	return slot.Definition{
		ID:      "test-game",
		Version: "1.0.0",
		Name:    "Test Slot",
		Grid: slot.GridConfig{
			Reels: 5,
			Rows:  3,
		},
		WinMechanic: slot.WinMechanicConfig{
			Type:     slot.WinMechanicWays,
			MinMatch: 3,
		},
		Symbols: []slot.SymbolConfig{
			{ID: "LOW1", Type: slot.SymbolRegular, Payouts: []slot.Multiplier{0, 0, 0, slot.MustMultiplier("0.5"), slot.MustMultiplier("1.0"), slot.MustMultiplier("2.0")}},
			{ID: "HIGH1", Type: slot.SymbolRegular, Payouts: []slot.Multiplier{0, 0, 0, slot.MustMultiplier("1.0"), slot.MustMultiplier("2.5"), slot.MustMultiplier("5.0")}},
			{ID: "WILD", Type: slot.SymbolWild},
			{ID: "SCAT", Type: slot.SymbolScatter},
		},
		ReelSets: map[string]slot.ReelSet{
			"base": {
				Reels: [][]slot.SymbolID{
					{"LOW1", "HIGH1", "WILD", "LOW1"},
					{"LOW1", "HIGH1", "WILD", "LOW1"},
					{"LOW1", "HIGH1", "WILD", "LOW1"},
					{"LOW1", "HIGH1", "WILD", "LOW1"},
					{"LOW1", "HIGH1", "WILD", "LOW1"},
				},
			},
		},
		Wild: slot.WildConfig{
			WildSymbolIDs: []slot.SymbolID{"WILD"},
		},
		Scatter: slot.ScatterConfig{
			SymbolID: "SCAT",
			Thresholds: []slot.ScatterTrigger{
				{Count: 3, Payout: slot.MustMultiplier("5.0"), Triggers: []slot.Trigger{{Kind: slot.TriggerFreeSpins, Award: 10}}},
			},
		},
		Cascade: slot.CascadeConfig{
			Enabled:        true,
			MaxCascades:    10,
			MultiplierMode: slot.CascadeMultiplierFixed,
		},
		MaxWinMultiplier: slot.MustMultiplier("5000"),
		MaxWinPolicy:     slot.MaxWinCapOnly,
	}
}

// ---------------------------------------------------------------------------
// 1. Money Tests
// ---------------------------------------------------------------------------

func TestParseMultiplier_Valid(t *testing.T) {
	tests := []struct {
		input string
		want  slot.Multiplier
	}{
		{"0", 0},
		{"1", 1_000_000},
		{"1.5", 1_500_000},
		{"0.25", 250_000},
		{"10.000001", 10_000_001},
		{"0.000001", 1},
		{"100", 100_000_000},
		{"+2.5", 2_500_000},
		{"  3.75  ", 3_750_000},
	}

	for _, tc := range tests {
		got, err := slot.ParseMultiplier(tc.input)
		if err != nil {
			t.Fatalf("ParseMultiplier(%q) unexpected error: %v", tc.input, err)
		}
		if got != tc.want {
			t.Errorf("ParseMultiplier(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

func TestParseMultiplier_Invalid(t *testing.T) {
	tests := []string{
		"",
		"-1",
		"-0.5",
		"abc",
		"1.2.3",
		"1.1234567", // > 6 decimals
		"+",
	}

	for _, input := range tests {
		_, err := slot.ParseMultiplier(input)
		if err == nil {
			t.Errorf("ParseMultiplier(%q) expected error, got nil", input)
		}
	}
}

func TestMustMultiplier_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("MustMultiplier invalid string did not panic")
		}
	}()
	slot.MustMultiplier("invalid")
}

func TestMultiplier_String(t *testing.T) {
	tests := []struct {
		m    slot.Multiplier
		want string
	}{
		{0, "0"},
		{1_000_000, "1"},
		{1_500_000, "1.5"},
		{250_000, "0.25"},
		{1, "0.000001"},
		{10_000_001, "10.000001"},
	}

	for _, tc := range tests {
		got := tc.m.String()
		if got != tc.want {
			t.Errorf("Multiplier(%d).String() = %q, want %q", tc.m, got, tc.want)
		}
	}
}

func TestMultiplier_JSON(t *testing.T) {
	type wrapper struct {
		M slot.Multiplier `json:"m"`
	}

	// Marshal
	w := wrapper{M: slot.MustMultiplier("2.5")}
	data, err := json.Marshal(w)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Unmarshal from string format
	var w2 wrapper
	if err := json.Unmarshal(data, &w2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if w2.M != w.M {
		t.Errorf("JSON roundtrip got %d, want %d", w2.M, w.M)
	}

	// Unmarshal from number format
	jsonNum := []byte(`{"m": 3.75}`)
	var w3 wrapper
	if err := json.Unmarshal(jsonNum, &w3); err != nil {
		t.Fatalf("Unmarshal number failed: %v", err)
	}
	if w3.M != slot.MustMultiplier("3.75") {
		t.Errorf("Unmarshal number got %d, want %d", w3.M, slot.MustMultiplier("3.75"))
	}
}

func TestMultiplier_CheckedArithmetic(t *testing.T) {
	m1 := slot.MustMultiplier("2.5")
	m2 := slot.MustMultiplier("1.5")

	sum, err := m1.Add(m2)
	if err != nil || sum != slot.MustMultiplier("4.0") {
		t.Errorf("Add failed: got %d, err %v", sum, err)
	}

	product, err := m1.MulInt(3)
	if err != nil || product != slot.MustMultiplier("7.5") {
		t.Errorf("MulInt failed: got %d, err %v", product, err)
	}
}

func TestSettle(t *testing.T) {
	tests := []struct {
		bet  slot.Amount
		mult slot.Multiplier
		want slot.Amount
	}{
		{100, slot.MustMultiplier("1.5"), 150},
		{100, slot.MustMultiplier("0.5"), 50},
		{100, slot.MustMultiplier("0.0"), 0},
		{100, slot.MustMultiplier("2.345678"), 234}, // floor 234.5678 -> 234
		{0, slot.MustMultiplier("10.0"), 0},
	}

	for _, tc := range tests {
		got, err := slot.Settle(tc.bet, tc.mult)
		if err != nil {
			t.Fatalf("Settle(%d, %d) unexpected error: %v", tc.bet, tc.mult, err)
		}
		if got != tc.want {
			t.Errorf("Settle(%d, %d) = %d, want %d", tc.bet, tc.mult, got, tc.want)
		}
	}

	// Negative amount rejected
	if _, err := slot.Settle(-10, slot.MustMultiplier("1.0")); !errors.Is(err, slot.ErrNegativeAmount) {
		t.Errorf("Settle negative bet want ErrNegativeAmount, got %v", err)
	}
	if _, err := slot.Settle(10, -1); !errors.Is(err, slot.ErrNegativeMultiplier) {
		t.Errorf("Settle negative mult want ErrNegativeMultiplier, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// 2. Config & Validation Tests
// ---------------------------------------------------------------------------

func TestValidateDefinition_Valid(t *testing.T) {
	def := sampleDefinition()
	vdef, err := slot.ValidateDefinition(def)
	if err != nil {
		t.Fatalf("ValidateDefinition failed: %v", err)
	}
	if vdef.Definition().ID != def.ID {
		t.Errorf("got ID %s, want %s", vdef.Definition().ID, def.ID)
	}

	// Check symbol lookup
	sym, ok := vdef.Symbol("HIGH1")
	if !ok || sym.Type != slot.SymbolRegular {
		t.Errorf("Symbol HIGH1 lookup failed")
	}

	// Check reelset lookup
	rs, ok := vdef.ReelSet("base")
	if !ok || len(rs.Reels) != 5 {
		t.Errorf("ReelSet base lookup failed")
	}

	// Check wild substitution
	if !vdef.CanWildSubstitute("WILD", "HIGH1") {
		t.Errorf("WILD should substitute for HIGH1")
	}
	if vdef.CanWildSubstitute("WILD", "SCAT") {
		t.Errorf("WILD must not substitute for SCAT")
	}
}

func TestValidateDefinition_DeepCopyIsolation(t *testing.T) {
	def := sampleDefinition()
	vdef, err := slot.ValidateDefinition(def)
	if err != nil {
		t.Fatalf("ValidateDefinition failed: %v", err)
	}

	// Mutate caller definition
	def.Symbols[0].Payouts[3] = slot.MustMultiplier("999")
	sym, _ := vdef.Symbol("LOW1")
	if sym.Payouts[3] == slot.MustMultiplier("999") {
		t.Errorf("mutating original definition affected validated definition")
	}
}

func TestValidateDefinition_Invalid(t *testing.T) {
	t.Run("empty ID", func(t *testing.T) {
		d := sampleDefinition()
		d.ID = ""
		if _, err := slot.ValidateDefinition(d); !errors.Is(err, slot.ErrInvalidDefinition) {
			t.Errorf("expected ErrInvalidDefinition, got %v", err)
		}
	})

	t.Run("duplicate symbol", func(t *testing.T) {
		d := sampleDefinition()
		d.Symbols = append(d.Symbols, d.Symbols[0])
		if _, err := slot.ValidateDefinition(d); !errors.Is(err, slot.ErrDuplicateSymbol) {
			t.Errorf("expected ErrDuplicateSymbol, got %v", err)
		}
	})

	t.Run("invalid payout length", func(t *testing.T) {
		d := sampleDefinition()
		d.Symbols[0].Payouts = []slot.Multiplier{0, 1}
		if _, err := slot.ValidateDefinition(d); !errors.Is(err, slot.ErrInvalidPayoutLength) {
			t.Errorf("expected ErrInvalidPayoutLength, got %v", err)
		}
	})

	t.Run("wild substituting for scatter", func(t *testing.T) {
		d := sampleDefinition()
		d.Wild.SubstituteFor = []slot.SymbolID{"SCAT"}
		if _, err := slot.ValidateDefinition(d); !errors.Is(err, slot.ErrIllegalWildTarget) {
			t.Errorf("expected ErrIllegalWildTarget, got %v", err)
		}
	})

	t.Run("unknown symbol in reelset", func(t *testing.T) {
		d := sampleDefinition()
		d.ReelSets["base"].Reels[0][0] = "GHOST"
		if _, err := slot.ValidateDefinition(d); !errors.Is(err, slot.ErrUnknownSymbol) {
			t.Errorf("expected ErrUnknownSymbol, got %v", err)
		}
	})

	t.Run("negative max win", func(t *testing.T) {
		d := sampleDefinition()
		d.MaxWinMultiplier = -1
		if _, err := slot.ValidateDefinition(d); !errors.Is(err, slot.ErrIncoherentCap) {
			t.Errorf("expected ErrIncoherentCap, got %v", err)
		}
	})

	t.Run("duplicate feature ID", func(t *testing.T) {
		d := sampleDefinition()
		d.Features = []slot.FeatureConfig{
			{ID: "cascade", Enabled: true},
			{ID: "cascade", Enabled: true},
		}
		if _, err := slot.ValidateDefinition(d); !errors.Is(err, slot.ErrDuplicateFeatureID) {
			t.Errorf("expected ErrDuplicateFeatureID, got %v", err)
		}
	})

	t.Run("missing base reel set", func(t *testing.T) {
		d := sampleDefinition()
		d.BaseReelSet = "nonexistent_set"
		if _, err := slot.ValidateDefinition(d); !errors.Is(err, slot.ErrMissingReelSet) {
			t.Errorf("expected ErrMissingReelSet, got %v", err)
		}
	})

	t.Run("per-wild substitution and metadata", func(t *testing.T) {
		d := sampleDefinition()
		d.Symbols = append(d.Symbols,
			slot.SymbolConfig{
				ID:                  "WILD2",
				Type:                slot.SymbolWild,
				Metadata:            []byte(`{"color":"gold"}`),
				SubstituteFor:       []slot.SymbolID{"HIGH1"},
				CannotSubstituteFor: []slot.SymbolID{"LOW1"},
			},
		)
		d.Wild.WildSymbolIDs = append(d.Wild.WildSymbolIDs, "WILD2")
		vdef, err := slot.ValidateDefinition(d)
		if err != nil {
			t.Fatalf("ValidateDefinition failed: %v", err)
		}
		if !vdef.CanWildSubstitute("WILD2", "HIGH1") {
			t.Errorf("expected WILD2 to substitute for HIGH1")
		}
		if vdef.CanWildSubstitute("WILD2", "LOW1") {
			t.Errorf("expected WILD2 NOT to substitute for LOW1")
		}
		sym, ok := vdef.Symbol("WILD2")
		if !ok || string(sym.Metadata) != `{"color":"gold"}` {
			t.Errorf("metadata not preserved: %s", sym.Metadata)
		}
	})

	t.Run("per-wild illegal target scatter", func(t *testing.T) {
		d := sampleDefinition()
		d.Symbols = append(d.Symbols,
			slot.SymbolConfig{
				ID:            "WILD_BAD",
				Type:          slot.SymbolWild,
				SubstituteFor: []slot.SymbolID{"SCAT"},
			},
		)
		d.Wild.WildSymbolIDs = append(d.Wild.WildSymbolIDs, "WILD_BAD")
		if _, err := slot.ValidateDefinition(d); !errors.Is(err, slot.ErrIllegalWildTarget) {
			t.Errorf("expected ErrIllegalWildTarget, got %v", err)
		}
	})
}

func TestDimensionDriven_LargeGrid(t *testing.T) {
	// Test grid with 17 reels and 18 rows (no hidden [16] dimension limit)
	numReels := 17
	numRows := 18

	payouts := make([]slot.Multiplier, numReels+1)
	for i := range payouts {
		payouts[i] = slot.MustMultiplier("1.0")
	}

	symbols := make([]slot.SymbolConfig, numRows)
	for i := 0; i < numRows; i++ {
		sid := slot.SymbolID(fmt.Sprintf("SYM%d", i))
		symbols[i] = slot.SymbolConfig{ID: sid, Type: slot.SymbolRegular, Payouts: payouts}
	}

	reels := make([][]slot.SymbolID, numReels)
	for r := 0; r < numReels; r++ {
		reels[r] = make([]slot.SymbolID, numRows)
		for row := 0; row < numRows; row++ {
			reels[r][row] = slot.SymbolID(fmt.Sprintf("SYM%d", row))
		}
	}

	d := slot.Definition{
		ID:      "large-grid-test",
		Version: "1.0.0",
		Name:    "Large Grid Test",
		Grid: slot.GridConfig{
			Reels: numReels,
			Rows:  numRows,
		},
		WinMechanic: slot.WinMechanicConfig{
			Type:     slot.WinMechanicWays,
			MinMatch: 3,
		},
		Symbols: symbols,
		ReelSets: map[string]slot.ReelSet{
			"base": {Reels: reels},
		},
		MaxWinMultiplier: slot.MustMultiplier("1000"),
	}

	vdef, err := slot.ValidateDefinition(d)
	if err != nil {
		t.Fatalf("ValidateDefinition failed: %v", err)
	}

	grid := make(slot.Grid, numReels)
	for r := 0; r < numReels; r++ {
		grid[r] = make([]slot.SymbolID, numRows)
		for row := 0; row < numRows; row++ {
			grid[r][row] = slot.SymbolID(fmt.Sprintf("SYM%d", row))
		}
	}

	evaluator := slot.WaysEvaluator{}
	eval, err := evaluator.Evaluate(vdef, grid)
	if err != nil {
		t.Fatalf("large grid evaluate failed: %v", err)
	}
	if len(eval.Wins) != numRows || eval.Wins[0].MatchedReels != numReels {
		t.Errorf("expected %d wins matching %d reels, got %v", numRows, numReels, len(eval.Wins))
	}

	// Test CollapseAndRefill on 17x18 grid
	removed := []slot.Position{
		{Reel: 16, Row: 17},
		{Reel: 0, Row: 0},
	}
	src := &fakeSource{intnVals: []uint64{0, 0}}
	collapsed, added, final, err := slot.CollapseAndRefill(vdef, "base", grid, removed, src)
	if err != nil {
		t.Fatalf("CollapseAndRefill on 17x18 failed: %v", err)
	}
	if len(added) != 2 {
		t.Errorf("expected 2 added symbols, got %d", len(added))
	}
	if len(collapsed) != numReels || len(final) != numReels {
		t.Errorf("dimensions mismatch in collapsed or final grid")
	}
}

// ---------------------------------------------------------------------------
// 3. Reel Sampling & Collapse Tests
// ---------------------------------------------------------------------------

func TestGenerateGrid_Fixed(t *testing.T) {
	vdef, err := slot.ValidateDefinition(sampleDefinition())
	if err != nil {
		t.Fatal(err)
	}

	src := &fakeSource{intnVals: []uint64{0, 1, 2, 3, 0}}
	gen, err := slot.GenerateGrid(vdef, "base", src)
	if err != nil {
		t.Fatalf("GenerateGrid failed: %v", err)
	}

	if len(gen.Grid) != 5 {
		t.Fatalf("expected 5 reels, got %d", len(gen.Grid))
	}
	for r := 0; r < 5; r++ {
		if len(gen.Grid[r]) != 3 {
			t.Errorf("reel %d expected 3 rows, got %d", r, len(gen.Grid[r]))
		}
	}
	if gen.Stops[1] != 1 {
		t.Errorf("reel 1 stop = %d, want 1", gen.Stops[1])
	}
}

func TestCollapseAndRefill(t *testing.T) {
	vdef, err := slot.ValidateDefinition(sampleDefinition())
	if err != nil {
		t.Fatal(err)
	}

	// 5x3 grid
	grid := slot.Grid{
		{"LOW1", "LOW1", "HIGH1"},
		{"HIGH1", "LOW1", "LOW1"},
		{"LOW1", "HIGH1", "WILD"},
		{"LOW1", "LOW1", "HIGH1"},
		{"HIGH1", "LOW1", "LOW1"},
	}

	// Remove row 1 in reel 0, row 2 in reel 0
	// In reel 0: "LOW1" (row 0) survives, falls to row 2.
	// Empty cells in reel 0 are rows 0 and 1. Refilled bottom-to-top (row 1, then row 0).
	removed := []slot.Position{
		{Reel: 0, Row: 1},
		{Reel: 0, Row: 2},
	}

	// Refill symbols from reelset "base": indices 1, 2
	src := &fakeSource{intnVals: []uint64{1, 2}}
	collapsed, added, final, err := slot.CollapseAndRefill(vdef, "base", grid, removed, src)
	if err != nil {
		t.Fatalf("CollapseAndRefill failed: %v", err)
	}

	// In collapsed grid: reel 0 row 2 is survivor "LOW1"
	if collapsed[0][2] != "LOW1" {
		t.Errorf("survivor not collapsed to bottom: got %s", collapsed[0][2])
	}
	if collapsed[0][0] != "" || collapsed[0][1] != "" {
		t.Errorf("expected empty upper cells in collapsed grid")
	}

	// In final grid: row 1 was filled first (index 1 = "HIGH1"), row 0 was filled next (index 2 = "WILD")
	if len(added) != 2 {
		t.Fatalf("expected 2 added symbols, got %d", len(added))
	}
	if added[0].Position.Row != 1 || added[1].Position.Row != 0 {
		t.Errorf("expected bottom-to-top refill order: %v, %v", added[0].Position, added[1].Position)
	}
	if final[0][2] != "LOW1" {
		t.Errorf("final grid row 2 = %s, want LOW1", final[0][2])
	}
}

func TestChooseWeighted(t *testing.T) {
	src := &fakeSource{intnVals: []uint64{5}} // sum = 10; weights [2, 5, 3]; val 5 falls in idx 1
	idx, err := slot.ChooseWeighted(src, []uint64{2, 5, 3})
	if err != nil || idx != 1 {
		t.Errorf("ChooseWeighted got %d, err %v", idx, err)
	}

	if _, err := slot.ChooseWeighted(src, nil); !errors.Is(err, slot.ErrEmptyWeights) {
		t.Errorf("expected ErrEmptyWeights, got %v", err)
	}
	if _, err := slot.ChooseWeighted(src, []uint64{0, 0}); !errors.Is(err, slot.ErrZeroTotalWeight) {
		t.Errorf("expected ErrZeroTotalWeight, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// 4. Evaluator Tests
// ---------------------------------------------------------------------------

func TestWaysEvaluator_WinsAndPositions(t *testing.T) {
	vdef, err := slot.ValidateDefinition(sampleDefinition())
	if err != nil {
		t.Fatal(err)
	}

	evaluator := slot.WaysEvaluator{}

	// Grid with 3 reels of LOW1:
	// Reel 0: 2 LOW1
	// Reel 1: 1 LOW1, 1 WILD
	// Reel 2: 1 LOW1
	// Reel 3: 0 LOW1 (stops here)
	// Reel 4: 1 LOW1
	// Ways: 2 * 2 * 1 = 4 ways of 3-of-a-kind LOW1 (payout 0.5x). Total = 2.0x
	grid := slot.Grid{
		{"LOW1", "LOW1", "SCAT"},
		{"LOW1", "WILD", "HIGH1"},
		{"LOW1", "HIGH1", "HIGH1"},
		{"HIGH1", "HIGH1", "HIGH1"},
		{"LOW1", "HIGH1", "HIGH1"},
	}

	eval, err := evaluator.Evaluate(vdef, grid)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if len(eval.Wins) != 1 {
		t.Fatalf("expected 1 win, got %d", len(eval.Wins))
	}
	win := eval.Wins[0]
	if win.Symbol != "LOW1" || win.MatchedReels != 3 || win.Ways != 4 {
		t.Errorf("win mismatch: symbol %s, reels %d, ways %d", win.Symbol, win.MatchedReels, win.Ways)
	}
	if win.TotalMultiplier != slot.MustMultiplier("2.0") {
		t.Errorf("total multiplier got %s, want 2.0", win.TotalMultiplier)
	}
	if len(win.Positions) != 5 { // 2 + 2 + 1 = 5 positions
		t.Errorf("expected 5 participating positions, got %d", len(win.Positions))
	}
}

func TestWaysEvaluator_ScatterTriggers(t *testing.T) {
	vdef, err := slot.ValidateDefinition(sampleDefinition())
	if err != nil {
		t.Fatal(err)
	}

	evaluator := slot.WaysEvaluator{}

	// Grid with 3 SCAT symbols
	grid := slot.Grid{
		{"SCAT", "LOW1", "LOW1"},
		{"HIGH1", "SCAT", "LOW1"},
		{"HIGH1", "HIGH1", "SCAT"},
		{"HIGH1", "LOW1", "LOW1"},
		{"HIGH1", "LOW1", "LOW1"},
	}

	eval, err := evaluator.Evaluate(vdef, grid)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if eval.ScatterCount != 3 {
		t.Errorf("scatter count = %d, want 3", eval.ScatterCount)
	}
	if eval.ScatterPayout != slot.MustMultiplier("5.0") {
		t.Errorf("scatter payout = %s, want 5.0", eval.ScatterPayout)
	}
	if len(eval.Triggers) != 1 || eval.Triggers[0].Kind != slot.TriggerFreeSpins || eval.Triggers[0].Award != 10 {
		t.Errorf("triggers mismatch: %v", eval.Triggers)
	}
}

// ---------------------------------------------------------------------------
// 5. State Machine Tests
// ---------------------------------------------------------------------------

func TestStateMachine_ValidSequence(t *testing.T) {
	sm := slot.NewStateMachine()
	if sm.State() != slot.StateIdle {
		t.Fatalf("initial state = %s, want IDLE", sm.State())
	}

	seq := []slot.State{
		slot.StateSpinning,
		slot.StateEvaluating,
		slot.StateCascading,
		slot.StateEvaluating,
		slot.StateFeatureTrigger,
		slot.StateFreeSpins,
		slot.StateSpinning,
		slot.StateEvaluating,
		slot.StateSettling,
		slot.StateCompleted,
	}

	for _, next := range seq {
		if err := sm.Transition(next); err != nil {
			t.Fatalf("transition to %s failed: %v", next, err)
		}
		if sm.State() != next {
			t.Errorf("current state = %s, want %s", sm.State(), next)
		}
	}
}

func TestStateMachine_InvalidTransition(t *testing.T) {
	sm := slot.NewStateMachine()
	if err := sm.Transition(slot.StateCompleted); !errors.Is(err, slot.ErrInvalidStateTransition) {
		t.Errorf("expected ErrInvalidStateTransition, got %v", err)
	}
}
