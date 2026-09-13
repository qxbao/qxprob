package slotsim_test

import (
	"context"
	"testing"

	"github.com/qxbao/qxprob/core/entropy"
	coreslot "github.com/qxbao/qxprob/core/slot"
	engineslot "github.com/qxbao/qxprob/engine/slot"
	"github.com/qxbao/qxprob/engine/slotgame/reference"
	"github.com/qxbao/qxprob/slotsim"
)

func BenchmarkFullSimulation_ProvablyFair(b *testing.B) {
	def := reference.Definition()
	vdef, err := coreslot.ValidateDefinition(def)
	if err != nil {
		b.Fatal(err)
	}
	features := reference.Features(vdef)

	game, err := engineslot.Compile(def, features, engineslot.Options{
		Mode: coreslot.PlayModeSimulation,
	})
	if err != nil {
		b.Fatal(err)
	}

	opts := slotsim.Options{
		Spins: 1000,
		Bet:   100,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		src := entropy.NewProvablyFairSource("sim-server-seed", "sim-client-seed", 1)
		runner := func(nonce uint64, req coreslot.SpinRequest) (coreslot.SpinResult, error) {
			src.ResetNonce(nonce)
			return game.Spin(req, src)
		}
		if _, err := slotsim.RunWithRunner(context.Background(), opts, runner); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFullSimulation_FastRNG(b *testing.B) {
	def := reference.Definition()
	vdef, err := coreslot.ValidateDefinition(def)
	if err != nil {
		b.Fatal(err)
	}
	features := reference.Features(vdef)

	game, err := engineslot.Compile(def, features, engineslot.Options{
		Mode: coreslot.PlayModeSimulation,
	})
	if err != nil {
		b.Fatal(err)
	}

	opts := slotsim.Options{
		Spins: 1000,
		Bet:   100,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		src := entropy.NewSimulationSource(12345, 67890)
		runner := func(nonce uint64, req coreslot.SpinRequest) (coreslot.SpinResult, error) {
			return game.Spin(req, src)
		}
		if _, err := slotsim.RunWithRunner(context.Background(), opts, runner); err != nil {
			b.Fatal(err)
		}
	}
}
