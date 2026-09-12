package crash_test

import (
	"errors"
	"math"
	"testing"

	"github.com/qxbao/qxprob/engine/crash"
)

func TestMaxMultiplier(t *testing.T) {
	fake := &fakeEntropySource{values: []float64{0.99}}
	engine, err := crash.New(crash.Config{HouseEdge: 1, MaxMultiplier: 3}, fake)
	if err != nil {
		t.Fatal(err)
	}
	result, err := engine.Play()
	if err != nil {
		t.Fatal(err)
	}
	if result.Multiplier != 3 {
		t.Fatalf("Multiplier = %v, want cap 3", result.Multiplier)
	}
}

func TestMaxMultiplierValidation(t *testing.T) {
	for _, value := range []float64{-1, 0.5, math.NaN(), math.Inf(1)} {
		_, err := crash.New(crash.Config{HouseEdge: 1, MaxMultiplier: value}, &fakeEntropySource{})
		if !errors.Is(err, crash.ErrInvalidMaxMultiplier) {
			t.Fatalf("MaxMultiplier %v error = %v, want %v", value, err, crash.ErrInvalidMaxMultiplier)
		}
	}
}

func TestPlaySeededForwardsMaxMultiplier(t *testing.T) {
	result, err := crash.PlaySeeded(crash.SeededInput{
		ServerSeed: "server", ClientSeed: "client", Nonce: 1,
		Config: crash.Config{HouseEdge: 0, MaxMultiplier: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Multiplier != 1 {
		t.Fatalf("Multiplier = %v, want cap 1", result.Multiplier)
	}
}
