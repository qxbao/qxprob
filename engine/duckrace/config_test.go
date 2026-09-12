package duckrace_test

import (
	"errors"
	"math"
	"testing"

	"github.com/qxbao/qxprob/engine/duckrace"
)

func TestCustomMotionAndFinishShape(t *testing.T) {
	fake := &fakeEntropySource{
		intnVals:  []uint64{0},
		floatVals: []float64{0.1, 0.9, 0.9, 0.1},
	}
	engine, err := duckrace.New(duckrace.Config{
		DuckCount:         2,
		DurationMillis:    1000,
		TickMillis:        500,
		BaseSegmentWeight: 2,
		FinishProgress:    0.9,
		RankGap:           0.2,
	}, fake)
	if err != nil {
		t.Fatal(err)
	}
	result, err := engine.Play()
	if err != nil {
		t.Fatal(err)
	}
	last := result.Frames[len(result.Frames)-1].Positions
	if math.Abs(last[0]-0.9) > 1e-12 || math.Abs(last[1]-0.7) > 1e-12 {
		t.Fatalf("final positions = %v, want [0.9 0.7]", last)
	}
}

func TestCustomDuckRaceConfigValidation(t *testing.T) {
	base := duckrace.Config{DuckCount: 4, DurationMillis: 1000, TickMillis: 100}
	tests := []struct {
		mutate  func(*duckrace.Config)
		wantErr error
	}{
		{func(c *duckrace.Config) { c.BaseSegmentWeight = -1 }, duckrace.ErrInvalidBaseSegmentWeight},
		{func(c *duckrace.Config) { c.FinishProgress = 1.1 }, duckrace.ErrInvalidFinishProgress},
		{func(c *duckrace.Config) { c.RankGap = 0.5 }, duckrace.ErrInvalidRankGap},
	}
	for _, tc := range tests {
		config := base
		tc.mutate(&config)
		_, err := duckrace.New(config, &fakeEntropySource{})
		if !errors.Is(err, tc.wantErr) {
			t.Fatalf("New(%+v) error = %v, want %v", config, err, tc.wantErr)
		}
	}
}

func TestRaceSeededForwardsCustomConfig(t *testing.T) {
	result, err := duckrace.RaceSeeded(duckrace.SeededInput{
		ServerSeed: "server", ClientSeed: "client", Nonce: 1,
		Config: duckrace.Config{
			DuckCount: 3, DurationMillis: 1000, TickMillis: 500,
			BaseSegmentWeight: 1.5, FinishProgress: 0.8, RankGap: 0.1,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	last := result.Frames[len(result.Frames)-1]
	if math.Abs(last.Positions[result.FinishOrder[0]]-0.8) > 1e-12 || math.Abs(last.Positions[result.FinishOrder[2]]-0.6) > 1e-12 {
		t.Fatalf("unexpected custom final positions: %v", last.Positions)
	}
}
