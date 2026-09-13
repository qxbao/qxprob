package slot_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"

	"github.com/qxbao/qxprob/core/entropy"
	coreslot "github.com/qxbao/qxprob/core/slot"
	engineslot "github.com/qxbao/qxprob/engine/slot"
	"github.com/qxbao/qxprob/engine/slotfeature"
)

func testGameDef() coreslot.Definition {
	return coreslot.Definition{
		ID:      "test-slot-engine",
		Version: "1.0.0",
		Name:    "Test Slot Engine",
		Grid: coreslot.GridConfig{
			Reels: 5,
			Rows:  3,
		},
		WinMechanic: coreslot.WinMechanicConfig{
			Type:     coreslot.WinMechanicWays,
			MinMatch: 3,
		},
		Symbols: []coreslot.SymbolConfig{
			{ID: "J", Type: coreslot.SymbolRegular, Payouts: []coreslot.Multiplier{0, 0, 0, coreslot.MustMultiplier("0.2"), coreslot.MustMultiplier("0.5"), coreslot.MustMultiplier("1.0")}},
			{ID: "Q", Type: coreslot.SymbolRegular, Payouts: []coreslot.Multiplier{0, 0, 0, coreslot.MustMultiplier("0.3"), coreslot.MustMultiplier("0.8"), coreslot.MustMultiplier("1.5")}},
			{ID: "K", Type: coreslot.SymbolRegular, Payouts: []coreslot.Multiplier{0, 0, 0, coreslot.MustMultiplier("0.5"), coreslot.MustMultiplier("1.2"), coreslot.MustMultiplier("2.5")}},
			{ID: "A", Type: coreslot.SymbolRegular, Payouts: []coreslot.Multiplier{0, 0, 0, coreslot.MustMultiplier("1.0"), coreslot.MustMultiplier("2.5"), coreslot.MustMultiplier("5.0")}},
			{ID: "WILD", Type: coreslot.SymbolWild},
			{ID: "SCAT", Type: coreslot.SymbolScatter},
		},
		ReelSets: map[string]coreslot.ReelSet{
			"base": {
				Reels: [][]coreslot.SymbolID{
					{"J", "Q", "K", "A", "WILD", "J", "Q", "K"},
					{"J", "Q", "K", "A", "WILD", "J", "Q", "K"},
					{"J", "Q", "K", "A", "WILD", "J", "Q", "K"},
					{"J", "Q", "K", "A", "WILD", "J", "Q", "K"},
					{"J", "Q", "K", "A", "WILD", "J", "Q", "K"},
				},
			},
			"free_spins": {
				Reels: [][]coreslot.SymbolID{
					{"A", "K", "Q", "SCAT", "WILD", "A", "K", "Q"},
					{"A", "K", "Q", "SCAT", "WILD", "A", "K", "Q"},
					{"A", "K", "Q", "SCAT", "WILD", "A", "K", "Q"},
					{"A", "K", "Q", "SCAT", "WILD", "A", "K", "Q"},
					{"A", "K", "Q", "SCAT", "WILD", "A", "K", "Q"},
				},
			},
		},
		Wild: coreslot.WildConfig{
			WildSymbolIDs: []coreslot.SymbolID{"WILD"},
		},
		Scatter: coreslot.ScatterConfig{
			SymbolID: "SCAT",
			Thresholds: []coreslot.ScatterTrigger{
				{Count: 3, Payout: coreslot.MustMultiplier("2.0"), Triggers: []coreslot.Trigger{{Kind: coreslot.TriggerFreeSpins, Award: 5}}},
			},
		},
		Cascade: coreslot.CascadeConfig{
			Enabled:        true,
			MaxCascades:    10,
			MultiplierMode: coreslot.CascadeMultiplierTable,
			MultiplierTable: []coreslot.Multiplier{
				coreslot.MustMultiplier("1.0"),
				coreslot.MustMultiplier("2.0"),
				coreslot.MustMultiplier("3.0"),
				coreslot.MustMultiplier("5.0"),
			},
		},
		FreeSpins: coreslot.FreeSpinsConfig{
			Enabled:          true,
			ReelSet:          "free_spins",
			MaxTotalSpins:    20,
			RetriggerEnabled: true,
		},
		MaxWinMultiplier: coreslot.MustMultiplier("1000"),
		MaxWinPolicy:     coreslot.MaxWinCapOnly,
	}
}

func TestEngine_ValidationBeforeEntropy(t *testing.T) {
	def := testGameDef()
	features := []coreslot.Feature{
		slotfeature.NewCascade("cascade"),
		slotfeature.NewScatterFreeSpins("scatter_free_spins"),
	}

	t.Run("empty server seed", func(t *testing.T) {
		req := coreslot.SeededSpinRequest{
			SpinRequest: coreslot.SpinRequest{GameID: def.ID, Version: def.Version, Bet: 100},
			ServerSeed:  "",
			ClientSeed:  "client",
			Nonce:       1,
		}
		if _, err := engineslot.PlaySeeded(def, req, features, engineslot.Options{}); !errors.Is(err, engineslot.ErrEmptyServerSeed) {
			t.Errorf("expected ErrEmptyServerSeed, got %v", err)
		}
	})

	t.Run("empty client seed", func(t *testing.T) {
		req := coreslot.SeededSpinRequest{
			SpinRequest: coreslot.SpinRequest{GameID: def.ID, Version: def.Version, Bet: 100},
			ServerSeed:  "server",
			ClientSeed:  "",
			Nonce:       1,
		}
		if _, err := engineslot.PlaySeeded(def, req, features, engineslot.Options{}); !errors.Is(err, engineslot.ErrEmptyClientSeed) {
			t.Errorf("expected ErrEmptyClientSeed, got %v", err)
		}
	})

	t.Run("invalid bet", func(t *testing.T) {
		req := coreslot.SeededSpinRequest{
			SpinRequest: coreslot.SpinRequest{GameID: def.ID, Version: def.Version, Bet: 0},
			ServerSeed:  "server",
			ClientSeed:  "client",
			Nonce:       1,
		}
		if _, err := engineslot.PlaySeeded(def, req, features, engineslot.Options{}); !errors.Is(err, engineslot.ErrInvalidBet) {
			t.Errorf("expected ErrInvalidBet, got %v", err)
		}
	})

	t.Run("mismatched game id", func(t *testing.T) {
		req := coreslot.SeededSpinRequest{
			SpinRequest: coreslot.SpinRequest{GameID: "wrong-game", Version: def.Version, Bet: 100},
			ServerSeed:  "server",
			ClientSeed:  "client",
			Nonce:       1,
		}
		if _, err := engineslot.PlaySeeded(def, req, features, engineslot.Options{}); !errors.Is(err, engineslot.ErrMismatchedGameID) {
			t.Errorf("expected ErrMismatchedGameID, got %v", err)
		}
	})
}

func TestEngine_SeededDeterminism(t *testing.T) {
	def := testGameDef()
	features := []coreslot.Feature{
		slotfeature.NewCascade("cascade"),
		slotfeature.NewScatterFreeSpins("scatter_free_spins"),
	}

	req := coreslot.SeededSpinRequest{
		SpinRequest: coreslot.SpinRequest{GameID: def.ID, Version: def.Version, Bet: 100},
		ServerSeed:  "reproducible-server-seed",
		ClientSeed:  "reproducible-client-seed",
		Nonce:       42,
	}

	res1, err := engineslot.PlaySeeded(def, req, features, engineslot.Options{})
	if err != nil {
		t.Fatalf("first run failed: %v", err)
	}

	res2, err := engineslot.PlaySeeded(def, req, features, engineslot.Options{})
	if err != nil {
		t.Fatalf("second run failed: %v", err)
	}

	if res1.TotalMultiplier != res2.TotalMultiplier {
		t.Errorf("multipliers differ: %s vs %s", res1.TotalMultiplier, res2.TotalMultiplier)
	}
	if res1.Payout != res2.Payout {
		t.Errorf("payouts differ: %d vs %d", res1.Payout, res2.Payout)
	}
	if !res1.InitialGrid.Equal(res2.InitialGrid) {
		t.Errorf("grids differ: %v vs %v", res1.InitialGrid, res2.InitialGrid)
	}
	if len(res1.Events) != len(res2.Events) {
		t.Errorf("events length differ: %d vs %d", len(res1.Events), len(res2.Events))
	}

	// Different nonce must yield different source stream
	reqDiffNonce := req
	reqDiffNonce.Nonce = 43
	resDiff, err := engineslot.PlaySeeded(def, reqDiffNonce, features, engineslot.Options{})
	if err != nil {
		t.Fatalf("diff nonce run failed: %v", err)
	}
	if res1.InitialGrid.Equal(resDiff.InitialGrid) && res1.Payout == resDiff.Payout {
		t.Logf("Notice: same outcome for diff nonce (possible with short strips, but stream was different)")
	}
}

type hookRecorder struct {
	coreslot.FeatureAdapter
	calls []string
}

func (h *hookRecorder) BeforeSpin(*coreslot.FeatureContext) error {
	h.calls = append(h.calls, "BeforeSpin")
	return nil
}
func (h *hookRecorder) AfterGridGenerated(*coreslot.FeatureContext) error {
	h.calls = append(h.calls, "AfterGridGenerated")
	return nil
}
func (h *hookRecorder) BeforeEvaluation(*coreslot.FeatureContext) error {
	h.calls = append(h.calls, "BeforeEvaluation")
	return nil
}
func (h *hookRecorder) AfterEvaluation(*coreslot.FeatureContext) error {
	h.calls = append(h.calls, "AfterEvaluation")
	return nil
}
func (h *hookRecorder) AfterCascade(*coreslot.FeatureContext) error {
	h.calls = append(h.calls, "AfterCascade")
	return nil
}
func (h *hookRecorder) AfterSpin(*coreslot.FeatureContext) error {
	h.calls = append(h.calls, "AfterSpin")
	return nil
}

func TestEngine_HookOrder(t *testing.T) {
	def := testGameDef()
	def.Cascade.Enabled = false
	def.FreeSpins.Enabled = false

	recorder := &hookRecorder{
		FeatureAdapter: coreslot.FeatureAdapter{FeatureID: "recorder"},
	}

	src := entropy.NewProvablyFairSource("seed1", "seed2", 1)
	eng, err := engineslot.New(def, src, []coreslot.Feature{recorder}, engineslot.Options{})
	if err != nil {
		t.Fatal(err)
	}

	_, err = eng.Spin(coreslot.SpinRequest{
		GameID:  def.ID,
		Version: def.Version,
		Bet:     100,
	})
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{"BeforeSpin", "AfterGridGenerated", "BeforeEvaluation", "AfterEvaluation", "AfterSpin"}
	if len(recorder.calls) != len(expected) {
		t.Fatalf("expected %v calls, got %v", expected, recorder.calls)
	}
	for i, want := range expected {
		if recorder.calls[i] != want {
			t.Errorf("hook %d = %s, want %s", i, recorder.calls[i], want)
		}
	}
}

func TestEngine_PlayVsSimulationMode(t *testing.T) {
	def := testGameDef()
	features := []coreslot.Feature{
		slotfeature.NewCascade("cascade"),
		slotfeature.NewScatterFreeSpins("scatter_free_spins"),
	}

	req := coreslot.SeededSpinRequest{
		SpinRequest: coreslot.SpinRequest{GameID: def.ID, Version: def.Version, Bet: 250},
		ServerSeed:  "compare-server-seed",
		ClientSeed:  "compare-client-seed",
		Nonce:       101,
	}

	// Normal mode
	reqNormal := req
	reqNormal.Mode = coreslot.PlayModeNormal
	resNormal, err := engineslot.PlaySeeded(def, reqNormal, features, engineslot.Options{})
	if err != nil {
		t.Fatal(err)
	}

	// Simulation mode
	reqSim := req
	reqSim.Mode = coreslot.PlayModeSimulation
	resSim, err := engineslot.PlaySeeded(def, reqSim, features, engineslot.Options{})
	if err != nil {
		t.Fatal(err)
	}

	// Settlement and math must be 100% identical!
	if resNormal.TotalMultiplier != resSim.TotalMultiplier {
		t.Errorf("multiplier mismatch: %s vs %s", resNormal.TotalMultiplier, resSim.TotalMultiplier)
	}
	if resNormal.Payout != resSim.Payout {
		t.Errorf("payout mismatch: %d vs %d", resNormal.Payout, resSim.Payout)
	}
	if resNormal.NetProfit != resSim.NetProfit {
		t.Errorf("net profit mismatch: %d vs %d", resNormal.NetProfit, resSim.NetProfit)
	}
	if resNormal.CascadeCount != resSim.CascadeCount {
		t.Errorf("cascade count mismatch: %d vs %d", resNormal.CascadeCount, resSim.CascadeCount)
	}
	if resNormal.FreeSpinCount != resSim.FreeSpinCount {
		t.Errorf("free spin count mismatch: %d vs %d", resNormal.FreeSpinCount, resSim.FreeSpinCount)
	}

	// Normal mode includes presentation grids, cascades, and events
	if resNormal.InitialGrid == nil {
		t.Errorf("normal mode should include initial grid")
	}
	if len(resNormal.Events) == 0 {
		t.Errorf("normal mode should emit events, got 0")
	}

	// Simulation mode omits presentation grids, cascades, and events
	if resSim.InitialGrid != nil {
		t.Errorf("simulation mode must omit initial grid")
	}
	if len(resSim.Cascades) != 0 {
		t.Errorf("simulation mode must omit cascades, got %d", len(resSim.Cascades))
	}
	if len(resSim.FreeSpins) != 0 {
		t.Errorf("simulation mode must omit free spin sessions, got %d", len(resSim.FreeSpins))
	}
	if len(resSim.Events) != 0 {
		t.Errorf("simulation mode must suppress events, got %d", len(resSim.Events))
	}
}

func TestEngine_InvariantsLoop(t *testing.T) {
	def := testGameDef()
	features := []coreslot.Feature{
		slotfeature.NewCascade("cascade"),
		slotfeature.NewScatterFreeSpins("scatter_free_spins"),
	}

	const spins = 100
	for nonce := uint64(1); nonce <= spins; nonce++ {
		req := coreslot.SeededSpinRequest{
			SpinRequest: coreslot.SpinRequest{GameID: def.ID, Version: def.Version, Bet: 100},
			ServerSeed:  "invariants-server-seed",
			ClientSeed:  "invariants-client-seed",
			Nonce:       nonce,
		}

		// Normal mode verification
		res, err := engineslot.PlaySeeded(def, req, features, engineslot.Options{Mode: coreslot.PlayModeNormal})
		if err != nil {
			t.Fatalf("nonce %d failed: %v", nonce, err)
		}

		// Invariant 1: Payout is non-negative
		if res.Payout < 0 {
			t.Fatalf("nonce %d payout is negative: %d", nonce, res.Payout)
		}

		// Invariant 2: Total multiplier does not exceed cap
		if res.TotalMultiplier > def.MaxWinMultiplier {
			t.Fatalf("nonce %d multiplier %s exceeds cap %s", nonce, res.TotalMultiplier, def.MaxWinMultiplier)
		}

		// Invariant 3: Net profit is exactly payout - bet
		if res.NetProfit != res.Payout-req.Bet {
			t.Fatalf("nonce %d net profit mismatch: got %d, want %d", nonce, res.NetProfit, res.Payout-req.Bet)
		}

		// Invariant 4: Initial grid is valid in normal mode
		minRows := def.Grid.Rows
		maxRows := def.Grid.Rows
		if def.Grid.VariableRows {
			minRows = def.Grid.MinRows
			maxRows = def.Grid.MaxRows
		}
		if err := res.InitialGrid.Validate(def.Grid.Reels, minRows, maxRows); err != nil {
			t.Fatalf("nonce %d initial grid invalid: %v", nonce, err)
		}

		// Invariant 5: Terminal state is StateCompleted
		if res.TerminalState != coreslot.StateCompleted {
			t.Fatalf("nonce %d terminal state = %s, want COMPLETED", nonce, res.TerminalState)
		}

		// Simulation mode must settle identically
		reqSim := req
		reqSim.Mode = coreslot.PlayModeSimulation
		resSim, err := engineslot.PlaySeeded(def, reqSim, features, engineslot.Options{})
		if err != nil {
			t.Fatalf("nonce %d sim failed: %v", nonce, err)
		}
		if resSim.Payout != res.Payout || resSim.TotalMultiplier != res.TotalMultiplier {
			t.Fatalf("nonce %d simulation settlement mismatch: normal=%d vs sim=%d", nonce, res.Payout, resSim.Payout)
		}
		if resSim.CascadeCount != res.CascadeCount || resSim.FreeSpinCount != res.FreeSpinCount {
			t.Fatalf("nonce %d summary counts mismatch", nonce)
		}
	}
}

// 1. Requirement 1 regression: Ways win AND scatter trigger on initial grid
func TestEngine_WaysWinAndScatterPreservation(t *testing.T) {
	def := testGameDef()
	def.Features = []coreslot.FeatureConfig{
		{ID: "cascade", Enabled: true},
		{ID: "scatter_free_spins", Enabled: true},
	}
	def.ReelSets["base"] = coreslot.ReelSet{
		Reels: [][]coreslot.SymbolID{
			{"J", "J", "SCAT"},
			{"J", "J", "SCAT"},
			{"J", "J", "SCAT"},
			{"Q", "Q", "Q"},
			{"K", "K", "K"},
		},
	}
	def.ReelSets["free_spins"] = coreslot.ReelSet{
		Reels: [][]coreslot.SymbolID{
			{"J", "Q", "K"},
			{"J", "Q", "K"},
			{"J", "Q", "K"},
			{"J", "Q", "K"},
			{"J", "Q", "K"},
		},
	}

	features := []coreslot.Feature{
		slotfeature.NewCascade("cascade"),
		slotfeature.NewScatterFreeSpins("scatter_free_spins"),
	}

	req := coreslot.SeededSpinRequest{
		SpinRequest: coreslot.SpinRequest{GameID: def.ID, Version: def.Version, Bet: 100, Mode: coreslot.PlayModeNormal},
		ServerSeed:  "test-ways-and-scatter-seed",
		ClientSeed:  "client",
		Nonce:       1,
	}

	res, err := engineslot.PlaySeeded(def, req, features, engineslot.Options{})
	if err != nil {
		t.Fatalf("PlaySeeded failed: %v", err)
	}

	// Verify that cascades executed
	if res.CascadeCount == 0 || len(res.Cascades) == 0 {
		t.Fatalf("expected cascades from initial ways win, got 0")
	}

	// Verify that free spins were NOT lost across cascades!
	if res.FreeSpinCount == 0 {
		t.Fatalf("free spins were lost across cascades! got FreeSpinCount=0")
	}
	if len(res.FreeSpins) == 0 {
		t.Fatalf("expected free spin sessions, got 0")
	}
	if res.FreeSpinState == nil {
		t.Fatalf("expected non-nil FreeSpinState")
	}
	if res.FreeSpinState.AwardedSpins != 5 {
		t.Errorf("expected 5 awarded free spins, got %d", res.FreeSpinState.AwardedSpins)
	}
}

// 2. Requirement 2: Cascade step internal coherence
func TestEngine_CascadeResultCoherence(t *testing.T) {
	def := testGameDef()
	features := []coreslot.Feature{
		slotfeature.NewCascade("cascade"),
		slotfeature.NewScatterFreeSpins("scatter_free_spins"),
	}

	req := coreslot.SeededSpinRequest{
		SpinRequest: coreslot.SpinRequest{GameID: def.ID, Version: def.Version, Bet: 100, Mode: coreslot.PlayModeNormal},
		ServerSeed:  "coherence-server-seed",
		ClientSeed:  "coherence-client-seed",
		Nonce:       42,
	}

	res, err := engineslot.PlaySeeded(def, req, features, engineslot.Options{})
	if err != nil {
		t.Fatalf("PlaySeeded failed: %v", err)
	}

	if len(res.Cascades) == 0 {
		t.Skip("no cascades on this seed")
	}

	for i, c := range res.Cascades {
		if c.Index != i+1 {
			t.Errorf("cascade %d: Index = %d, want %d", i, c.Index, i+1)
		}
		if len(c.Wins) == 0 {
			t.Errorf("cascade %d: empty Wins", i)
		}
		if len(c.RemovedPositions) == 0 {
			t.Errorf("cascade %d: empty RemovedPositions", i)
		}
		if c.GridBefore == nil || c.CollapsedGrid == nil || c.FinalGrid == nil {
			t.Errorf("cascade %d: nil grid", i)
		}
		if c.Multiplier <= 0 {
			t.Errorf("cascade %d: non-positive Multiplier %s", i, c.Multiplier)
		}

		// Coherence Assertion 1: All RemovedPositions must belong to Wins
		winPosSet := make(map[coreslot.Position]bool)
		var totalWinMult coreslot.Multiplier
		for _, w := range c.Wins {
			for _, pos := range w.Positions {
				winPosSet[pos] = true
			}
			var err error
			totalWinMult, err = totalWinMult.Add(w.TotalMultiplier)
			if err != nil {
				t.Fatalf("cascade %d: overflow in win sum: %v", i, err)
			}
		}
		for _, pos := range c.RemovedPositions {
			if !winPosSet[pos] {
				t.Errorf("cascade %d: RemovedPosition %v not in Wins positions", i, pos)
			}
		}

		// Coherence Assertion 2: TotalMultiplier equals sum(Wins)*Multiplier
		expectedTotal, err := totalWinMult.Mul(c.Multiplier)
		if err != nil {
			t.Fatalf("cascade %d: overflow in expected total: %v", i, err)
		}
		if c.TotalMultiplier != expectedTotal {
			t.Errorf("cascade %d: TotalMultiplier = %s, want %s", i, c.TotalMultiplier, expectedTotal)
		}

		// Coherence Assertion 3: FinalGrid dimensions match GridBefore
		if len(c.FinalGrid) != len(c.GridBefore) {
			t.Errorf("cascade %d: FinalGrid reel count %d != GridBefore %d", i, len(c.FinalGrid), len(c.GridBefore))
		}
	}
}

// 3. Requirement 3: Rejection of duplicate feature IDs & declaration order execution
func TestEngine_FeatureOrderAndDuplicateRejection(t *testing.T) {
	def := testGameDef()
	def.Features = []coreslot.FeatureConfig{
		{ID: "featA", Enabled: true},
		{ID: "featB", Enabled: true},
	}

	t.Run("duplicate feature module in New", func(t *testing.T) {
		src := entropy.NewProvablyFairSource("s1", "s2", 1)
		dupFeatures := []coreslot.Feature{
			slotfeature.NewCascade("featA"),
			slotfeature.NewCascade("featA"),
		}
		_, err := engineslot.New(def, src, dupFeatures, engineslot.Options{})
		if !errors.Is(err, coreslot.ErrDuplicateFeatureID) {
			t.Errorf("expected ErrDuplicateFeatureID, got %v", err)
		}
	})

	t.Run("declaration order invocation regardless of caller order", func(t *testing.T) {
		src := entropy.NewProvablyFairSource("s1", "s2", 1)
		var execOrder []string

		modA := &orderProbeFeature{id: "featA", onBeforeSpin: func() { execOrder = append(execOrder, "featA") }}
		modB := &orderProbeFeature{id: "featB", onBeforeSpin: func() { execOrder = append(execOrder, "featB") }}

		// Caller passes [modB, modA] in reverse order
		callerFeatures := []coreslot.Feature{modB, modA}
		eng, err := engineslot.New(def, src, callerFeatures, engineslot.Options{})
		if err != nil {
			t.Fatalf("New failed: %v", err)
		}

		_, err = eng.Spin(coreslot.SpinRequest{GameID: def.ID, Version: def.Version, Bet: 100})
		if err != nil {
			t.Fatalf("Spin failed: %v", err)
		}

		if len(execOrder) < 2 {
			t.Fatalf("expected at least 2 hooks executed, got %v", execOrder)
		}
		// Def declared featA then featB, so featA must run before featB!
		if execOrder[0] != "featA" || execOrder[1] != "featB" {
			t.Errorf("execution order was %v, want [featA, featB]", execOrder)
		}
	})
}

type orderProbeFeature struct {
	coreslot.FeatureAdapter
	id           string
	onBeforeSpin func()
}

func (f *orderProbeFeature) ID() string { return f.id }
func (f *orderProbeFeature) BeforeSpin(*coreslot.FeatureContext) error {
	if f.onBeforeSpin != nil {
		f.onBeforeSpin()
	}
	return nil
}

// 4. Requirement 4: Cross-spin persistent free spin cascade multiplier
func TestEngine_PersistentFreeSpinMultiplier(t *testing.T) {
	def := testGameDef()
	def.FreeSpins.MultiplierPolicy = coreslot.MultiplierPersistent
	def.Cascade.MultiplierStep = coreslot.MustMultiplier("1.0")
	def.Features = []coreslot.FeatureConfig{
		{ID: "cascade", Enabled: true},
		{ID: "scatter_free_spins", Enabled: true},
	}

	def.ReelSets["base"] = coreslot.ReelSet{
		Reels: [][]coreslot.SymbolID{
			{"SCAT", "J", "J"},
			{"SCAT", "J", "J"},
			{"SCAT", "J", "J"},
			{"Q", "Q", "Q"},
			{"K", "K", "K"},
		},
	}
	def.ReelSets["free_spins"] = coreslot.ReelSet{
		Reels: [][]coreslot.SymbolID{
			{"J", "J", "J"},
			{"J", "J", "J"},
			{"J", "J", "J"},
			{"J", "J", "J"},
			{"J", "J", "J"},
		},
	}

	features := []coreslot.Feature{
		slotfeature.NewCascade("cascade"),
		slotfeature.NewScatterFreeSpins("scatter_free_spins"),
	}

	req := coreslot.SeededSpinRequest{
		SpinRequest: coreslot.SpinRequest{GameID: def.ID, Version: def.Version, Bet: 100, Mode: coreslot.PlayModeNormal},
		ServerSeed:  "persistent-mult-seed",
		ClientSeed:  "client",
		Nonce:       1,
	}

	res, err := engineslot.PlaySeeded(def, req, features, engineslot.Options{})
	if err != nil {
		t.Fatalf("PlaySeeded failed: %v", err)
	}

	if res.FreeSpinState == nil {
		t.Fatalf("expected FreeSpinState")
	}

	if res.FreeSpinState.GlobalMultiplier <= coreslot.MustMultiplier("1.0") {
		t.Errorf("expected GlobalMultiplier > 1.0, got %s", res.FreeSpinState.GlobalMultiplier)
	}
}

// 5. Requirement 7: Semantic events carry typed JSON payloads
func TestEngine_SemanticEventsTypedPayloads(t *testing.T) {
	def := testGameDef()
	features := []coreslot.Feature{
		slotfeature.NewCascade("cascade"),
		slotfeature.NewScatterFreeSpins("scatter_free_spins"),
	}

	req := coreslot.SeededSpinRequest{
		SpinRequest: coreslot.SpinRequest{GameID: def.ID, Version: def.Version, Bet: 100, Mode: coreslot.PlayModeNormal},
		ServerSeed:  "typed-events-seed",
		ClientSeed:  "client",
		Nonce:       1,
	}

	res, err := engineslot.PlaySeeded(def, req, features, engineslot.Options{})
	if err != nil {
		t.Fatalf("PlaySeeded failed: %v", err)
	}

	if len(res.Events) == 0 {
		t.Fatalf("expected semantic events in normal mode")
	}

	for _, ev := range res.Events {
		if len(ev.Payload) == 0 {
			t.Errorf("event %s has empty payload", ev.Kind)
			continue
		}
		switch ev.Kind {
		case coreslot.EventGridReveal:
			var p coreslot.GridRevealPayload
			if err := json.Unmarshal(ev.Payload, &p); err != nil {
				t.Errorf("EventGridReveal unmarshal error: %v", err)
			}
			if len(p.Grid) == 0 {
				t.Errorf("EventGridReveal empty grid")
			}
		case coreslot.EventWins:
			var p coreslot.WinsPayload
			if err := json.Unmarshal(ev.Payload, &p); err != nil {
				t.Errorf("EventWins unmarshal error: %v", err)
			}
		case coreslot.EventRemoval:
			var p coreslot.RemovalPayload
			if err := json.Unmarshal(ev.Payload, &p); err != nil {
				t.Errorf("EventRemoval unmarshal error: %v", err)
			}
		case coreslot.EventCascade:
			var p coreslot.CascadePayload
			if err := json.Unmarshal(ev.Payload, &p); err != nil {
				t.Errorf("EventCascade unmarshal error: %v", err)
			}
		case coreslot.EventScatterTrigger:
			var p coreslot.ScatterTriggerPayload
			if err := json.Unmarshal(ev.Payload, &p); err != nil {
				t.Errorf("EventScatterTrigger unmarshal error: %v", err)
			}
		case coreslot.EventFreeSpinTrigger:
			var p coreslot.FreeSpinTriggerPayload
			if err := json.Unmarshal(ev.Payload, &p); err != nil {
				t.Errorf("EventFreeSpinTrigger unmarshal error: %v", err)
			}
		case coreslot.EventMaxWinReached:
			var p coreslot.MaxWinReachedPayload
			if err := json.Unmarshal(ev.Payload, &p); err != nil {
				t.Errorf("EventMaxWinReached unmarshal error: %v", err)
			}
		case coreslot.EventComplete:
			var p coreslot.CompletePayload
			if err := json.Unmarshal(ev.Payload, &p); err != nil {
				t.Errorf("EventComplete unmarshal error: %v", err)
			}
			if p.Payout != res.Payout {
				t.Errorf("EventComplete payout %d != res.Payout %d", p.Payout, res.Payout)
			}
		}
	}
}

// 6. Regression: Scatter payout & trigger appearing only on the final post-cascade grid
func TestEngine_ScatterPayoutAndTriggerOnFinalPostCascadeGrid(t *testing.T) {
	def := testGameDef()
	def.Features = []coreslot.FeatureConfig{
		{ID: "cascade", Enabled: true},
		{ID: "scatter_free_spins", Enabled: true},
	}
	// Configure base reels:
	// Initial stop 0 on each reel produces:
	// Reel 0: J, J, J
	// Reel 1: J, J, J
	// Reel 2: J, J, J
	// Reel 3: A, A, A
	// Reel 4: A, A, A
	// Initial grid has ways win on J (3 reels) and 0 SCAT.
	// When J is removed, missing cells are refilled from stops 3, 4, 5 of each reel:
	// Reel 0: SCAT, Q, K
	// Reel 1: SCAT, K, Q
	// Reel 2: SCAT, J, Q
	// Reel 3: A, A, A
	// Reel 4: A, A, A
	// Resulting post-cascade grid has 3 SCAT symbols and NO ways wins!
	def.ReelSets["base"] = coreslot.ReelSet{
		Reels: [][]coreslot.SymbolID{
			{"J", "J", "J", "SCAT", "J", "J"},
			{"J", "J", "J", "SCAT", "Q", "Q"},
			{"J", "J", "J", "SCAT", "K", "K"},
			{"A", "A", "A", "A", "A", "A"},
			{"A", "A", "A", "A", "A", "A"},
		},
	}
	// Free spins reels with no matching symbols so free spins don't cascade
	def.ReelSets["free_spins"] = coreslot.ReelSet{
		Reels: [][]coreslot.SymbolID{
			{"J", "J", "J"},
			{"Q", "Q", "Q"},
			{"K", "K", "K"},
			{"A", "A", "A"},
			{"J", "J", "J"},
		},
	}
	def.Cascade.MultiplierMode = coreslot.CascadeMultiplierTable
	def.Cascade.MultiplierTable = []coreslot.Multiplier{
		coreslot.MustMultiplier("2.0"), // step 1 multiplier = 2.0x
		coreslot.MustMultiplier("3.0"),
	}

	features := []coreslot.Feature{
		slotfeature.NewCascade("cascade"),
		slotfeature.NewScatterFreeSpins("scatter_free_spins"),
	}

	// mock source: stops 0 for initial grid, then stops 3, 4, 5 for refill
	mockSrc := &fixedMockSource{
		stops: []uint64{
			0, 0, 0, 0, 0, // initial grid reel stops: 0 for all 5 reels
			3, 4, 5, // refill reel 0 (row 2: SCAT, row 1: Q, row 0: K)
			3, 4, 5, // refill reel 1 (row 2: SCAT, row 1: K, row 0: Q)
			3, 4, 5, // refill reel 2 (row 2: SCAT, row 1: J, row 0: Q)
		},
	}

	eng, err := engineslot.New(def, mockSrc, features, engineslot.Options{})
	if err != nil {
		t.Fatalf("New engine failed: %v", err)
	}

	res, err := eng.Spin(coreslot.SpinRequest{
		GameID:  def.ID,
		Version: def.Version,
		Bet:     100,
		Mode:    coreslot.PlayModeNormal,
	})
	if err != nil {
		t.Fatalf("Spin failed: %v", err)
	}

	// 1. Initial grid must have 0 scatters
	initialScatters := 0
	for r := range res.InitialGrid {
		for _, s := range res.InitialGrid[r] {
			if s == "SCAT" {
				initialScatters++
			}
		}
	}
	if initialScatters != 0 {
		t.Fatalf("expected 0 scatters on initial grid, got %d", initialScatters)
	}

	// 2. Must have exactly 1 cascade
	if res.CascadeCount != 1 || len(res.Cascades) != 1 {
		t.Fatalf("expected 1 cascade, got count=%d len=%d", res.CascadeCount, len(res.Cascades))
	}

	// 3. Final post-cascade grid must have 3 scatters
	finalGrid := res.Cascades[0].FinalGrid
	finalScatters := 0
	for r := range finalGrid {
		for _, s := range finalGrid[r] {
			if s == "SCAT" {
				finalScatters++
			}
		}
	}
	if finalScatters != 3 {
		t.Fatalf("expected 3 scatters on final grid, got %d", finalScatters)
	}

	// 4. Scatter payout (2.0x) from final grid must be contributed to BaseMultiplier unmultiplied by cascade multiplier
	// Ways win on initial grid: J matches 3 reels, count 3x3x3=27 ways * 0.2 = 5.4x.
	// Step 1 ways multiplier was 2.0x -> ways win = 5.4 * 2.0 = 10.8x.
	// Scatter payout = 2.0x (unmultiplied!).
	// Total base multiplier = 10.8 + 2.0 = 12.8x.
	expectedBaseMult := coreslot.MustMultiplier("12.8")
	if res.BaseMultiplier != expectedBaseMult {
		t.Errorf("expected BaseMultiplier %s, got %s", expectedBaseMult, res.BaseMultiplier)
	}

	// 5. Triggers and TriggeredFeatures must aggregate scatter trigger from post-cascade grid
	if len(res.Triggers) == 0 {
		t.Errorf("expected scatter triggers to be aggregated into res.Triggers")
	} else {
		tr := res.Triggers[0]
		if tr.Kind != coreslot.TriggerFreeSpins || tr.Count != 3 || tr.Award != 5 {
			t.Errorf("unexpected trigger: %+v", tr)
		}
	}

	foundFreeSpinsFeature := false
	for _, f := range res.TriggeredFeatures {
		if f == "free_spins" {
			foundFreeSpinsFeature = true
			break
		}
	}
	if !foundFreeSpinsFeature {
		t.Errorf("expected 'free_spins' in res.TriggeredFeatures, got %v", res.TriggeredFeatures)
	}

	// 6. Semantic event for scatter trigger must be emitted for post-cascade grid
	foundScatterEvent := false
	for _, ev := range res.Events {
		if ev.Kind == coreslot.EventScatterTrigger {
			foundScatterEvent = true
			break
		}
	}
	if !foundScatterEvent {
		t.Errorf("expected EventScatterTrigger in res.Events")
	}

	// 7. Free spins triggered by post-cascade grid must be executed
	if res.FreeSpinCount != 5 {
		t.Errorf("expected FreeSpinCount=5, got %d", res.FreeSpinCount)
	}
}

// 7. Regression: Verify cascade ways multiplier does not multiply scatter payout
func TestEngine_CascadeMultiplierDoesNotMultiplyScatter(t *testing.T) {
	def := testGameDef()
	def.FreeSpins.Enabled = false // disable free spins so outcome is purely base spin
	def.Features = []coreslot.FeatureConfig{
		{ID: "cascade", Enabled: true},
	}
	// Initial grid has ways win on J (reels 0..2) and 3 scatters:
	// Reel 0: J, J, SCAT, Q, K
	// Reel 1: J, J, SCAT, K, Q
	// Reel 2: J, J, SCAT, A, A
	// Reel 3: Q, Q, Q, Q, Q
	// Reel 4: K, K, K, K, K
	// Stop 0 produces:
	// Reel 0: J, J, SCAT (2 J, 1 SCAT)
	// Reel 1: J, J, SCAT (2 J, 1 SCAT)
	// Reel 2: J, J, SCAT (2 J, 1 SCAT)
	// Reel 3: Q, Q, Q
	// Reel 4: K, K, K
	// Initial evaluation:
	// J matches reels 0..2: count 2x2x2=8 ways * 0.2 = 1.6x.
	// SCAT count 3: payout = 2.0x.
	// Step 1 multiplier is 5.0x.
	// Winning J positions (6 positions) are removed.
	// Refill from stop 3 produces:
	// Reel 0: Q, K
	// Reel 1: K, Q
	// Reel 2: A, A
	// Refilled grid has Q/K on reel 0, K/Q on reel 1, A on reel 2 -> NO ways wins!
	// Surviving SCATs are still at row 2 on reels 0, 1, 2!
	// Terminal grid has 3 SCATs -> contributes 2.0x.
	def.ReelSets["base"] = coreslot.ReelSet{
		Reels: [][]coreslot.SymbolID{
			{"J", "J", "SCAT", "Q", "K"},
			{"J", "J", "SCAT", "K", "Q"},
			{"J", "J", "SCAT", "A", "A"},
			{"Q", "Q", "Q", "Q", "Q"},
			{"K", "K", "K", "K", "K"},
		},
	}
	def.Cascade.MultiplierMode = coreslot.CascadeMultiplierTable
	def.Cascade.MultiplierTable = []coreslot.Multiplier{
		coreslot.MustMultiplier("5.0"), // 5x multiplier for step 1
	}

	features := []coreslot.Feature{
		slotfeature.NewCascade("cascade"),
	}

	mockSrc := &fixedMockSource{
		stops: []uint64{
			0, 0, 0, 0, 0, // initial grid
			3, 3, // refill reel 0
			3, 3, // refill reel 1
			3, 3, // refill reel 2
		},
	}

	eng, err := engineslot.New(def, mockSrc, features, engineslot.Options{})
	if err != nil {
		t.Fatalf("New engine failed: %v", err)
	}

	res, err := eng.Spin(coreslot.SpinRequest{
		GameID:  def.ID,
		Version: def.Version,
		Bet:     100,
		Mode:    coreslot.PlayModeSimulation,
	})
	if err != nil {
		t.Fatalf("Spin failed: %v", err)
	}

	// Ways win on initial grid: 1.6x.
	// Step 1 multiplier is 5.0x -> multiplied ways win = 1.6 * 5.0 = 8.0x.
	// Scatter payout on initial grid = 2.0x (unmultiplied!).
	// Scatter payout on terminal grid = 2.0x (unmultiplied!).
	// Total base multiplier = 2.0 + 8.0 + 2.0 = 12.0x.
	// If scatter were multiplied by cascade multiplier 5.0x, the initial scatter would be 2.0*5 = 10.0x
	// and total would be at least (1.6+2.0)*5 = 18.0x.
	expectedBaseMult := coreslot.MustMultiplier("12.0")
	if res.BaseMultiplier != expectedBaseMult {
		t.Errorf("expected BaseMultiplier %s (scatter unmultiplied), got %s", expectedBaseMult, res.BaseMultiplier)
	}
}

type fixedMockSource struct {
	stops []uint64
	idx   int
}

func (s *fixedMockSource) Uint64() (uint64, error)    { return 0, nil }
func (s *fixedMockSource) Float64() (float64, error)  { return 0, nil }
func (s *fixedMockSource) Bits(n int) (uint64, error) { return 0, nil }
func (s *fixedMockSource) Intn(n uint64) (uint64, error) {
	if s.idx < len(s.stops) {
		val := s.stops[s.idx]
		s.idx++
		return val % n, nil
	}
	return 0, nil
}

type countingSource struct {
	source entropy.Source
	reads  int
}

func (c *countingSource) Float64() (float64, error) {
	c.reads++
	return c.source.Float64()
}

func (c *countingSource) Bits(n int) (uint64, error) {
	c.reads++
	return c.source.Bits(n)
}

func (c *countingSource) Uint64() (uint64, error) {
	c.reads++
	return c.source.Uint64()
}

func (c *countingSource) Intn(n uint64) (uint64, error) {
	c.reads++
	return c.source.Intn(n)
}

func TestEngine_CompileValidation(t *testing.T) {
	def := testGameDef()
	features := []coreslot.Feature{
		slotfeature.NewCascade("cascade"),
		slotfeature.NewScatterFreeSpins("scatter_free_spins"),
	}

	t.Run("successful compile", func(t *testing.T) {
		game, err := engineslot.Compile(def, features, engineslot.Options{})
		if err != nil {
			t.Fatalf("unexpected Compile error: %v", err)
		}
		if game == nil {
			t.Fatal("expected non-nil Game")
		}
		if game.Definition().ID != def.ID {
			t.Errorf("got def ID %q, want %q", game.Definition().ID, def.ID)
		}
	})

	t.Run("duplicate feature module in Compile", func(t *testing.T) {
		dupFeatures := []coreslot.Feature{
			slotfeature.NewCascade("cascade"),
			slotfeature.NewCascade("cascade"),
		}
		_, err := engineslot.Compile(def, dupFeatures, engineslot.Options{})
		if !errors.Is(err, coreslot.ErrDuplicateFeatureID) {
			t.Errorf("expected ErrDuplicateFeatureID, got %v", err)
		}
	})

	t.Run("unregistered enabled feature in Compile", func(t *testing.T) {
		defWithUnregistered := testGameDef()
		defWithUnregistered.Features = []coreslot.FeatureConfig{
			{ID: "unknown_feature", Enabled: true},
		}
		_, err := engineslot.Compile(defWithUnregistered, features, engineslot.Options{})
		if !errors.Is(err, engineslot.ErrUnregisteredFeature) {
			t.Errorf("expected ErrUnregisteredFeature, got %v", err)
		}
	})

	t.Run("custom multiplier policy without features in Compile", func(t *testing.T) {
		defCustom := testGameDef()
		defCustom.FreeSpins.MultiplierPolicy = coreslot.MultiplierCustom
		_, err := engineslot.Compile(defCustom, nil, engineslot.Options{})
		if !errors.Is(err, coreslot.ErrInvalidDefinition) {
			t.Errorf("expected ErrInvalidDefinition, got %v", err)
		}
	})

	t.Run("invalid definition in Compile", func(t *testing.T) {
		invalidDef := testGameDef()
		invalidDef.ID = ""
		_, err := engineslot.Compile(invalidDef, features, engineslot.Options{})
		if err == nil {
			t.Fatal("expected error for invalid definition, got nil")
		}
	})
}

func TestEngine_ValidationNoEntropyConsumption(t *testing.T) {
	def := testGameDef()
	features := []coreslot.Feature{
		slotfeature.NewCascade("cascade"),
		slotfeature.NewScatterFreeSpins("scatter_free_spins"),
	}

	game, err := engineslot.Compile(def, features, engineslot.Options{})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("nil source returns ErrNilSource without consuming", func(t *testing.T) {
		req := coreslot.SpinRequest{GameID: def.ID, Version: def.Version, Bet: 100}
		_, err := game.Spin(req, nil)
		if !errors.Is(err, engineslot.ErrNilSource) {
			t.Errorf("expected ErrNilSource, got %v", err)
		}
	})

	t.Run("invalid bet does not consume entropy", func(t *testing.T) {
		cntSrc := &countingSource{source: entropy.NewProvablyFairSource("s", "c", 1)}
		req := coreslot.SpinRequest{GameID: def.ID, Version: def.Version, Bet: 0}
		_, err := game.Spin(req, cntSrc)
		if !errors.Is(err, engineslot.ErrInvalidBet) {
			t.Errorf("expected ErrInvalidBet, got %v", err)
		}
		if cntSrc.reads != 0 {
			t.Errorf("expected 0 entropy reads, got %d", cntSrc.reads)
		}
	})

	t.Run("mismatched game ID does not consume entropy", func(t *testing.T) {
		cntSrc := &countingSource{source: entropy.NewProvablyFairSource("s", "c", 1)}
		req := coreslot.SpinRequest{GameID: "wrong-id", Version: def.Version, Bet: 100}
		_, err := game.Spin(req, cntSrc)
		if !errors.Is(err, engineslot.ErrMismatchedGameID) {
			t.Errorf("expected ErrMismatchedGameID, got %v", err)
		}
		if cntSrc.reads != 0 {
			t.Errorf("expected 0 entropy reads, got %d", cntSrc.reads)
		}
	})

	t.Run("mismatched version does not consume entropy", func(t *testing.T) {
		cntSrc := &countingSource{source: entropy.NewProvablyFairSource("s", "c", 1)}
		req := coreslot.SpinRequest{GameID: def.ID, Version: "9.9.9", Bet: 100}
		_, err := game.Spin(req, cntSrc)
		if !errors.Is(err, engineslot.ErrMismatchedVersion) {
			t.Errorf("expected ErrMismatchedVersion, got %v", err)
		}
		if cntSrc.reads != 0 {
			t.Errorf("expected 0 entropy reads, got %d", cntSrc.reads)
		}
	})
}

func TestEngine_CompiledReuseAcrossNonces(t *testing.T) {
	def := testGameDef()
	features := []coreslot.Feature{
		slotfeature.NewCascade("cascade"),
		slotfeature.NewScatterFreeSpins("scatter_free_spins"),
	}

	game, err := engineslot.Compile(def, features, engineslot.Options{Mode: coreslot.PlayModeNormal})
	if err != nil {
		t.Fatal(err)
	}

	const serverSeed = "reuse-server-seed"
	const clientSeed = "reuse-client-seed"
	reusableSrc := entropy.NewProvablyFairSource(serverSeed, clientSeed, 1)

	for nonce := uint64(1); nonce <= 100; nonce++ {
		req := coreslot.SpinRequest{GameID: def.ID, Version: def.Version, Bet: 100, Mode: coreslot.PlayModeNormal}

		// Legacy engine creation per nonce
		legacySrc := entropy.NewProvablyFairSource(serverSeed, clientSeed, nonce)
		legacyEngine, err := engineslot.New(def, legacySrc, features, engineslot.Options{Mode: coreslot.PlayModeNormal})
		if err != nil {
			t.Fatalf("nonce %d legacy New: %v", nonce, err)
		}
		legacyRes, err := legacyEngine.Spin(req)
		if err != nil {
			t.Fatalf("nonce %d legacy Spin: %v", nonce, err)
		}

		// Reused compiled game with ResetNonce
		reusableSrc.ResetNonce(nonce)
		compiledRes, err := game.Spin(req, reusableSrc)
		if err != nil {
			t.Fatalf("nonce %d compiled Spin: %v", nonce, err)
		}

		if legacyRes.TotalMultiplier != compiledRes.TotalMultiplier {
			t.Fatalf("nonce %d TotalMultiplier: legacy %s != compiled %s", nonce, legacyRes.TotalMultiplier, compiledRes.TotalMultiplier)
		}
		if legacyRes.Payout != compiledRes.Payout {
			t.Fatalf("nonce %d Payout: legacy %d != compiled %d", nonce, legacyRes.Payout, compiledRes.Payout)
		}
		if legacyRes.NetProfit != compiledRes.NetProfit {
			t.Fatalf("nonce %d NetProfit: legacy %d != compiled %d", nonce, legacyRes.NetProfit, compiledRes.NetProfit)
		}
		if legacyRes.CascadeCount != compiledRes.CascadeCount {
			t.Fatalf("nonce %d CascadeCount: legacy %d != compiled %d", nonce, legacyRes.CascadeCount, compiledRes.CascadeCount)
		}
		if legacyRes.FreeSpinCount != compiledRes.FreeSpinCount {
			t.Fatalf("nonce %d FreeSpinCount: legacy %d != compiled %d", nonce, legacyRes.FreeSpinCount, compiledRes.FreeSpinCount)
		}
		if !legacyRes.InitialGrid.Equal(compiledRes.InitialGrid) {
			t.Fatalf("nonce %d InitialGrid mismatch", nonce)
		}
		if len(legacyRes.Events) != len(compiledRes.Events) {
			t.Fatalf("nonce %d Events length mismatch: %d vs %d", nonce, len(legacyRes.Events), len(compiledRes.Events))
		}
	}
}

func TestEngine_ConcurrentGameSpin(t *testing.T) {
	def := testGameDef()
	features := []coreslot.Feature{
		slotfeature.NewCascade("cascade"),
		slotfeature.NewScatterFreeSpins("scatter_free_spins"),
	}

	game, err := engineslot.Compile(def, features, engineslot.Options{Mode: coreslot.PlayModeSimulation})
	if err != nil {
		t.Fatal(err)
	}

	const goroutines = 8
	const spinsPerGoroutine = 50

	var wg sync.WaitGroup
	errCh := make(chan error, goroutines*spinsPerGoroutine)

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			serverSeed := fmt.Sprintf("worker-%d-server", workerID)
			clientSeed := fmt.Sprintf("worker-%d-client", workerID)
			src := entropy.NewProvablyFairSource(serverSeed, clientSeed, 1)

			for s := 0; s < spinsPerGoroutine; s++ {
				src.ResetNonce(uint64(s + 1))
				req := coreslot.SpinRequest{
					GameID:  def.ID,
					Version: def.Version,
					Bet:     100,
					Mode:    coreslot.PlayModeSimulation,
				}
				res, err := game.Spin(req, src)
				if err != nil {
					errCh <- fmt.Errorf("worker %d spin %d error: %w", workerID, s, err)
					return
				}
				if res.Payout < 0 {
					errCh <- fmt.Errorf("worker %d spin %d negative payout: %d", workerID, s, res.Payout)
					return
				}
			}
		}(g)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Fatal(err)
	}
}

func TestEngine_10000NonceEquivalence(t *testing.T) {
	def := testGameDef()
	features := []coreslot.Feature{
		slotfeature.NewCascade("cascade"),
		slotfeature.NewScatterFreeSpins("scatter_free_spins"),
	}

	game, err := engineslot.Compile(def, features, engineslot.Options{})
	if err != nil {
		t.Fatal(err)
	}

	const serverSeed = "equivalence-10000-server"
	const clientSeed = "equivalence-10000-client"
	reusableSrc := entropy.NewProvablyFairSource(serverSeed, clientSeed, 1)

	const totalNonces = 10000
	for nonce := uint64(1); nonce <= totalNonces; nonce++ {
		// Test in simulation mode for all nonces, spot-checking normal mode every 500 nonces
		mode := coreslot.PlayModeSimulation
		if nonce%500 == 0 {
			mode = coreslot.PlayModeNormal
		}

		req := coreslot.SpinRequest{
			GameID:  def.ID,
			Version: def.Version,
			Bet:     100,
			Mode:    mode,
		}

		// Legacy path: New engine per nonce
		legacySrc := entropy.NewProvablyFairSource(serverSeed, clientSeed, nonce)
		legacyEngine, err := engineslot.New(def, legacySrc, features, engineslot.Options{Mode: mode})
		if err != nil {
			t.Fatalf("nonce %d legacy New: %v", nonce, err)
		}
		legacyRes, err := legacyEngine.Spin(req)
		if err != nil {
			t.Fatalf("nonce %d legacy Spin: %v", nonce, err)
		}

		// Reusable path: ResetNonce and Game.Spin
		reusableSrc.ResetNonce(nonce)
		compiledRes, err := game.Spin(req, reusableSrc)
		if err != nil {
			t.Fatalf("nonce %d compiled Spin: %v", nonce, err)
		}

		// Assert complete mathematical and deterministic equivalence
		if legacyRes.TotalMultiplier != compiledRes.TotalMultiplier {
			t.Fatalf("nonce %d TotalMultiplier: legacy %s != compiled %s", nonce, legacyRes.TotalMultiplier, compiledRes.TotalMultiplier)
		}
		if legacyRes.BaseMultiplier != compiledRes.BaseMultiplier {
			t.Fatalf("nonce %d BaseMultiplier: legacy %s != compiled %s", nonce, legacyRes.BaseMultiplier, compiledRes.BaseMultiplier)
		}
		if legacyRes.FeatureMultiplier != compiledRes.FeatureMultiplier {
			t.Fatalf("nonce %d FeatureMultiplier: legacy %s != compiled %s", nonce, legacyRes.FeatureMultiplier, compiledRes.FeatureMultiplier)
		}
		if legacyRes.Payout != compiledRes.Payout {
			t.Fatalf("nonce %d Payout: legacy %d != compiled %d", nonce, legacyRes.Payout, compiledRes.Payout)
		}
		if legacyRes.NetProfit != compiledRes.NetProfit {
			t.Fatalf("nonce %d NetProfit: legacy %d != compiled %d", nonce, legacyRes.NetProfit, compiledRes.NetProfit)
		}
		if legacyRes.CascadeCount != compiledRes.CascadeCount {
			t.Fatalf("nonce %d CascadeCount: legacy %d != compiled %d", nonce, legacyRes.CascadeCount, compiledRes.CascadeCount)
		}
		if legacyRes.FreeSpinCount != compiledRes.FreeSpinCount {
			t.Fatalf("nonce %d FreeSpinCount: legacy %d != compiled %d", nonce, legacyRes.FreeSpinCount, compiledRes.FreeSpinCount)
		}
		if legacyRes.TerminalState != compiledRes.TerminalState {
			t.Fatalf("nonce %d TerminalState: legacy %s != compiled %s", nonce, legacyRes.TerminalState, compiledRes.TerminalState)
		}
		if len(legacyRes.Triggers) != len(compiledRes.Triggers) {
			t.Fatalf("nonce %d Triggers length: legacy %d != compiled %d", nonce, len(legacyRes.Triggers), len(compiledRes.Triggers))
		}
		if !reflect.DeepEqual(legacyRes.TriggeredFeatures, compiledRes.TriggeredFeatures) {
			t.Fatalf("nonce %d TriggeredFeatures: legacy %v != compiled %v", nonce, legacyRes.TriggeredFeatures, compiledRes.TriggeredFeatures)
		}

		if mode == coreslot.PlayModeNormal {
			if !legacyRes.InitialGrid.Equal(compiledRes.InitialGrid) {
				t.Fatalf("nonce %d InitialGrid mismatch", nonce)
			}
			if len(legacyRes.Cascades) != len(compiledRes.Cascades) {
				t.Fatalf("nonce %d Cascades count: legacy %d != compiled %d", nonce, len(legacyRes.Cascades), len(compiledRes.Cascades))
			}
			if len(legacyRes.Events) != len(compiledRes.Events) {
				t.Fatalf("nonce %d Events count: legacy %d != compiled %d", nonce, len(legacyRes.Events), len(compiledRes.Events))
			}
		}
	}
}
