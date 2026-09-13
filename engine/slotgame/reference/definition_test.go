package reference_test

import (
	"testing"

	coreslot "github.com/qxbao/qxprob/core/slot"
	engineslot "github.com/qxbao/qxprob/engine/slot"
	"github.com/qxbao/qxprob/engine/slotgame/reference"
)

func TestReferenceDefinition_ValidationAndCompleteness(t *testing.T) {
	def := reference.Definition()
	if def.ID != reference.GameID {
		t.Errorf("ID = %q, want %q", def.ID, reference.GameID)
	}
	if def.Version != reference.ConfigVersion {
		t.Errorf("Version = %q, want %q", def.Version, reference.ConfigVersion)
	}
	if def.Grid.Reels != 5 || def.Grid.Rows != 4 {
		t.Errorf("Grid dimensions = %dx%d, want 5x4", def.Grid.Reels, def.Grid.Rows)
	}

	// 1024 nominal ways = 4^5
	ways := 1
	for r := 0; r < def.Grid.Reels; r++ {
		ways *= def.Grid.Rows
	}
	if ways != 1024 {
		t.Errorf("nominal ways = %d, want 1024", ways)
	}

	vdef, err := coreslot.ValidateDefinition(def)
	if err != nil {
		t.Fatalf("ValidateDefinition failed: %v", err)
	}

	// Check separate reelsets exist
	if _, ok := vdef.ReelSet("base"); !ok {
		t.Errorf("base reelset missing")
	}
	if _, ok := vdef.ReelSet("free_spins"); !ok {
		t.Errorf("free_spins reelset missing")
	}

	// Features factory
	features := reference.Features(vdef)
	if len(features) != 2 {
		t.Fatalf("expected 2 feature modules, got %d", len(features))
	}
}

func TestReferenceDefinition_SeededReplay(t *testing.T) {
	def := reference.Definition()
	vdef, err := coreslot.ValidateDefinition(def)
	if err != nil {
		t.Fatal(err)
	}
	features := reference.Features(vdef)

	req := coreslot.SeededSpinRequest{
		SpinRequest: coreslot.SpinRequest{
			GameID:  reference.GameID,
			Version: reference.ConfigVersion,
			Bet:     100,
		},
		ServerSeed: "reference-test-server-seed",
		ClientSeed: "reference-test-client-seed",
		Nonce:      1,
	}

	res1, err := engineslot.PlaySeeded(def, req, features, engineslot.Options{})
	if err != nil {
		t.Fatalf("first play failed: %v", err)
	}

	res2, err := engineslot.PlaySeeded(def, req, features, engineslot.Options{})
	if err != nil {
		t.Fatalf("second play failed: %v", err)
	}

	if res1.Payout != res2.Payout || res1.TotalMultiplier != res2.TotalMultiplier {
		t.Errorf("replay mismatch: payout %d vs %d, mult %s vs %s", res1.Payout, res2.Payout, res1.TotalMultiplier, res2.TotalMultiplier)
	}
	if !res1.InitialGrid.Equal(res2.InitialGrid) {
		t.Errorf("initial grids differ across identical seeded replay")
	}
}

func TestReferenceDefinition_LightweightSimulation(t *testing.T) {
	def := reference.Definition()
	vdef, err := coreslot.ValidateDefinition(def)
	if err != nil {
		t.Fatal(err)
	}
	features := reference.Features(vdef)

	const spins = 50
	for nonce := uint64(1); nonce <= spins; nonce++ {
		req := coreslot.SeededSpinRequest{
			SpinRequest: coreslot.SpinRequest{
				GameID:  reference.GameID,
				Version: reference.ConfigVersion,
				Bet:     100,
				Mode:    coreslot.PlayModeSimulation,
			},
			ServerSeed: "simulation-test-server",
			ClientSeed: "simulation-test-client",
			Nonce:      nonce,
		}

		res, err := engineslot.PlaySeeded(def, req, features, engineslot.Options{Mode: coreslot.PlayModeSimulation})
		if err != nil {
			t.Fatalf("nonce %d failed: %v", nonce, err)
		}

		if res.Payout < 0 {
			t.Fatalf("nonce %d payout negative: %d", nonce, res.Payout)
		}
		if res.TotalMultiplier > def.MaxWinMultiplier {
			t.Fatalf("nonce %d multiplier %s exceeds cap %s", nonce, res.TotalMultiplier, def.MaxWinMultiplier)
		}
	}
}
