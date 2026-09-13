package slot_test

import (
	"testing"

	"github.com/qxbao/qxprob/core/entropy"
	coreslot "github.com/qxbao/qxprob/core/slot"
	engineslot "github.com/qxbao/qxprob/engine/slot"
	"github.com/qxbao/qxprob/engine/slotfeature"
)

func BenchmarkEngineNew(b *testing.B) {
	def := testGameDef()
	features := []coreslot.Feature{
		slotfeature.NewCascade("cascade"),
		slotfeature.NewScatterFreeSpins("scatter_free_spins"),
	}
	src := entropy.NewProvablyFairSource("benchmark-server", "benchmark-client", 1)
	opts := engineslot.Options{Mode: coreslot.PlayModeSimulation}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		eng, err := engineslot.New(def, src, features, opts)
		if err != nil {
			b.Fatal(err)
		}
		_ = eng
	}
}

func BenchmarkCompile(b *testing.B) {
	def := testGameDef()
	features := []coreslot.Feature{
		slotfeature.NewCascade("cascade"),
		slotfeature.NewScatterFreeSpins("scatter_free_spins"),
	}
	opts := engineslot.Options{Mode: coreslot.PlayModeSimulation}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		game, err := engineslot.Compile(def, features, opts)
		if err != nil {
			b.Fatal(err)
		}
		_ = game
	}
}

func BenchmarkGameSpin(b *testing.B) {
	def := testGameDef()
	features := []coreslot.Feature{
		slotfeature.NewCascade("cascade"),
		slotfeature.NewScatterFreeSpins("scatter_free_spins"),
	}
	opts := engineslot.Options{Mode: coreslot.PlayModeSimulation}
	game, err := engineslot.Compile(def, features, opts)
	if err != nil {
		b.Fatal(err)
	}

	src := entropy.NewProvablyFairSource("benchmark-server", "benchmark-client", 1)
	req := coreslot.SpinRequest{
		GameID:  def.ID,
		Version: def.Version,
		Bet:     100,
		Mode:    coreslot.PlayModeSimulation,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		src.ResetNonce(uint64(i + 1))
		res, err := game.Spin(req, src)
		if err != nil {
			b.Fatal(err)
		}
		_ = res
	}
}

func BenchmarkFullRoundLegacy(b *testing.B) {
	def := testGameDef()
	features := []coreslot.Feature{
		slotfeature.NewCascade("cascade"),
		slotfeature.NewScatterFreeSpins("scatter_free_spins"),
	}
	opts := engineslot.Options{Mode: coreslot.PlayModeSimulation}
	src := entropy.NewProvablyFairSource("benchmark-server", "benchmark-client", 1)
	req := coreslot.SpinRequest{
		GameID:  def.ID,
		Version: def.Version,
		Bet:     100,
		Mode:    coreslot.PlayModeSimulation,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		src.ResetNonce(uint64(i + 1))
		eng, err := engineslot.New(def, src, features, opts)
		if err != nil {
			b.Fatal(err)
		}
		res, err := eng.Spin(req)
		if err != nil {
			b.Fatal(err)
		}
		_ = res
	}
}

func BenchmarkFullRoundCompiled(b *testing.B) {
	def := testGameDef()
	features := []coreslot.Feature{
		slotfeature.NewCascade("cascade"),
		slotfeature.NewScatterFreeSpins("scatter_free_spins"),
	}
	opts := engineslot.Options{Mode: coreslot.PlayModeSimulation}
	game, err := engineslot.Compile(def, features, opts)
	if err != nil {
		b.Fatal(err)
	}
	src := entropy.NewProvablyFairSource("benchmark-server", "benchmark-client", 1)
	req := coreslot.SpinRequest{
		GameID:  def.ID,
		Version: def.Version,
		Bet:     100,
		Mode:    coreslot.PlayModeSimulation,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		src.ResetNonce(uint64(i + 1))
		res, err := game.Spin(req, src)
		if err != nil {
			b.Fatal(err)
		}
		_ = res
	}
}
