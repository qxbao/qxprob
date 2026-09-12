package plinko_test

import (
	"errors"
	"math"
	"testing"

	"github.com/qxbao/qxprob/engine/plinko"
)

func TestCustomMultiplierTable(t *testing.T) {
	table := []float64{10, 8, 6, 4, 2, 1, 0.5, 0.25, 0}
	fake := &fakeEntropySource{decisions: make([]uint64, 8)}
	engine, err := plinko.New(plinko.Config{Rows: 8, Risk: plinko.Low, Multipliers: table}, fake)
	if err != nil {
		t.Fatal(err)
	}
	table[0] = 999
	result, err := engine.Play()
	if err != nil {
		t.Fatal(err)
	}
	if result.Multiplier != 10 {
		t.Fatalf("Multiplier = %v, want copied custom value 10", result.Multiplier)
	}
}

func TestCustomTargetRTPAndRiskAlpha(t *testing.T) {
	const target = 0.93
	fake := &fakeEntropySource{decisions: make([]uint64, 8)}
	engine, err := plinko.New(plinko.Config{Rows: 8, Risk: plinko.High, TargetRTP: target, RiskAlpha: 0.15}, fake)
	if err != nil {
		t.Fatal(err)
	}
	result, err := engine.Play()
	if err != nil {
		t.Fatal(err)
	}
	if result.Multiplier <= 0 || math.IsNaN(result.Multiplier) || math.IsInf(result.Multiplier, 0) {
		t.Fatalf("invalid custom multiplier %v", result.Multiplier)
	}
}

func TestCustomPlinkoConfigValidation(t *testing.T) {
	tests := []struct {
		config  plinko.Config
		wantErr error
	}{
		{plinko.Config{Rows: 8, Risk: plinko.Low, TargetRTP: 1.01}, plinko.ErrInvalidTargetRTP},
		{plinko.Config{Rows: 8, Risk: plinko.Low, RiskAlpha: math.NaN()}, plinko.ErrInvalidRiskAlpha},
		{plinko.Config{Rows: 8, Risk: plinko.Low, Multipliers: []float64{1}}, plinko.ErrInvalidMultipliers},
	}
	for _, tc := range tests {
		_, err := plinko.New(tc.config, &fakeEntropySource{})
		if !errors.Is(err, tc.wantErr) {
			t.Fatalf("New(%+v) error = %v, want %v", tc.config, err, tc.wantErr)
		}
	}
}

func TestPlaySeededForwardsCustomPayouts(t *testing.T) {
	table := make([]float64, 9)
	for i := range table {
		table[i] = 7
	}
	result, err := plinko.PlaySeeded(plinko.SeededInput{
		ServerSeed: "server", ClientSeed: "client", Nonce: 1,
		Rows: 8, Risk: plinko.Low, Multipliers: table,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Multiplier != 7 {
		t.Fatalf("Multiplier = %v, want 7", result.Multiplier)
	}
}
