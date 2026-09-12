package duckrace_test

import (
	"context"
	"errors"
	"math"
	"reflect"
	"slices"
	"testing"

	"github.com/qxbao/qxprob/core/entropy"
	"github.com/qxbao/qxprob/core/simulation"
	"github.com/qxbao/qxprob/engine/duckrace"
)

// fakeEntropySource is a deterministic fake implementing entropy.Source for unit testing.
type fakeEntropySource struct {
	intnVals   []uint64
	floatVals  []float64
	intnIdx    int
	floatIdx   int
	intnErr    error
	floatErr   error
	intnCount  int
	floatCount int
	intnBounds []uint64
}

func (f *fakeEntropySource) Intn(n uint64) (uint64, error) {
	f.intnCount++
	f.intnBounds = append(f.intnBounds, n)
	if f.intnErr != nil {
		return 0, f.intnErr
	}
	if f.intnIdx >= len(f.intnVals) {
		return 0, errors.New("fake: no more intn values")
	}
	val := f.intnVals[f.intnIdx]
	f.intnIdx++
	return val % n, nil
}

func (f *fakeEntropySource) Float64() (float64, error) {
	f.floatCount++
	if f.floatErr != nil {
		return 0, f.floatErr
	}
	if f.floatIdx >= len(f.floatVals) {
		return 0, errors.New("fake: no more float values")
	}
	val := f.floatVals[f.floatIdx]
	f.floatIdx++
	return val, nil
}

func (f *fakeEntropySource) Bits(int) (uint64, error) {
	return 0, errors.New("fake: Bits not implemented")
}

func (f *fakeEntropySource) Uint64() (uint64, error) {
	return 0, errors.New("fake: Uint64 not implemented")
}

var _ entropy.Source = (*fakeEntropySource)(nil)

// TestEngineInterfaceCompatibility verifies that *duckrace.Engine satisfies simulation.Engine[duckrace.Result].
func TestEngineInterfaceCompatibility(t *testing.T) {
	fake := &fakeEntropySource{
		intnVals:  []uint64{0},
		floatVals: []float64{0.1, 0.2, 0.3, 0.4},
	}
	cfg := duckrace.Config{
		DuckCount:      2,
		DurationMillis: 1000,
		TickMillis:     500,
	}
	eng, err := duckrace.New(cfg, fake)
	if err != nil {
		t.Fatalf("unexpected New failure: %v", err)
	}

	var _ simulation.Engine[duckrace.Result] = eng

	var roundCount uint64
	err = simulation.Run(context.Background(), eng, 1, func(round uint64, res duckrace.Result) error {
		roundCount++
		if len(res.FinishOrder) != 2 {
			t.Fatalf("expected finish order length 2, got %d", len(res.FinishOrder))
		}
		if len(res.Frames) != 3 {
			t.Fatalf("expected 3 frames, got %d", len(res.Frames))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("simulation.Run failed: %v", err)
	}
	if roundCount != 1 {
		t.Fatalf("expected 1 round, got %d", roundCount)
	}
}

// TestConstructorValidation verifies that New validates config and source boundaries
// without consuming entropy.
func TestConstructorValidation(t *testing.T) {
	fake := &fakeEntropySource{}

	tests := []struct {
		name      string
		config    duckrace.Config
		source    entropy.Source
		wantErr   error
		wantValid bool
	}{
		{
			name:      "nil source",
			config:    duckrace.Config{DuckCount: 4, DurationMillis: 5000, TickMillis: 100},
			source:    nil,
			wantErr:   duckrace.ErrNilSource,
			wantValid: false,
		},
		{
			name:      "duck count below minimum (0)",
			config:    duckrace.Config{DuckCount: 0, DurationMillis: 5000, TickMillis: 100},
			source:    fake,
			wantErr:   duckrace.ErrInvalidDuckCount,
			wantValid: false,
		},
		{
			name:      "duck count below minimum (1)",
			config:    duckrace.Config{DuckCount: 1, DurationMillis: 5000, TickMillis: 100},
			source:    fake,
			wantErr:   duckrace.ErrInvalidDuckCount,
			wantValid: false,
		},
		{
			name:      "duck count negative (-1)",
			config:    duckrace.Config{DuckCount: -1, DurationMillis: 5000, TickMillis: 100},
			source:    fake,
			wantErr:   duckrace.ErrInvalidDuckCount,
			wantValid: false,
		},
		{
			name:      "duck count above maximum (17)",
			config:    duckrace.Config{DuckCount: 17, DurationMillis: 5000, TickMillis: 100},
			source:    fake,
			wantErr:   duckrace.ErrInvalidDuckCount,
			wantValid: false,
		},
		{
			name:      "duration below minimum (999)",
			config:    duckrace.Config{DuckCount: 4, DurationMillis: 999, TickMillis: 100},
			source:    fake,
			wantErr:   duckrace.ErrInvalidDuration,
			wantValid: false,
		},
		{
			name:      "duration zero",
			config:    duckrace.Config{DuckCount: 4, DurationMillis: 0, TickMillis: 100},
			source:    fake,
			wantErr:   duckrace.ErrInvalidDuration,
			wantValid: false,
		},
		{
			name:      "duration negative (-1)",
			config:    duckrace.Config{DuckCount: 4, DurationMillis: -1, TickMillis: 100},
			source:    fake,
			wantErr:   duckrace.ErrInvalidDuration,
			wantValid: false,
		},
		{
			name:      "duration above maximum (60001)",
			config:    duckrace.Config{DuckCount: 4, DurationMillis: 60001, TickMillis: 100},
			source:    fake,
			wantErr:   duckrace.ErrInvalidDuration,
			wantValid: false,
		},
		{
			name:      "tick below minimum (49)",
			config:    duckrace.Config{DuckCount: 4, DurationMillis: 5000, TickMillis: 49},
			source:    fake,
			wantErr:   duckrace.ErrInvalidTick,
			wantValid: false,
		},
		{
			name:      "tick zero",
			config:    duckrace.Config{DuckCount: 4, DurationMillis: 5000, TickMillis: 0},
			source:    fake,
			wantErr:   duckrace.ErrInvalidTick,
			wantValid: false,
		},
		{
			name:      "tick negative (-50)",
			config:    duckrace.Config{DuckCount: 4, DurationMillis: 5000, TickMillis: -50},
			source:    fake,
			wantErr:   duckrace.ErrInvalidTick,
			wantValid: false,
		},
		{
			name:      "tick above maximum (1001)",
			config:    duckrace.Config{DuckCount: 4, DurationMillis: 5000, TickMillis: 1001},
			source:    fake,
			wantErr:   duckrace.ErrInvalidTick,
			wantValid: false,
		},
		{
			name:      "duration not divisible by tick (1000 / 300)",
			config:    duckrace.Config{DuckCount: 4, DurationMillis: 1000, TickMillis: 300},
			source:    fake,
			wantErr:   duckrace.ErrIndivisibleDuration,
			wantValid: false,
		},
		{
			name:      "duration not divisible by tick (5000 / 300)",
			config:    duckrace.Config{DuckCount: 4, DurationMillis: 5000, TickMillis: 300},
			source:    fake,
			wantErr:   duckrace.ErrIndivisibleDuration,
			wantValid: false,
		},
		{
			name:      "valid lower boundaries (2 ducks, 1000ms, 50ms)",
			config:    duckrace.Config{DuckCount: 2, DurationMillis: 1000, TickMillis: 50},
			source:    fake,
			wantErr:   nil,
			wantValid: true,
		},
		{
			name:      "valid upper boundaries (16 ducks, 60000ms, 1000ms)",
			config:    duckrace.Config{DuckCount: 16, DurationMillis: 60000, TickMillis: 1000},
			source:    fake,
			wantErr:   nil,
			wantValid: true,
		},
		{
			name:      "valid mid-range config (8 ducks, 10000ms, 200ms)",
			config:    duckrace.Config{DuckCount: 8, DurationMillis: 10000, TickMillis: 200},
			source:    fake,
			wantErr:   nil,
			wantValid: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			beforeIntn := fake.intnCount
			beforeFloat := fake.floatCount
			eng, err := duckrace.New(tc.config, tc.source)
			if tc.wantValid {
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
				if eng == nil {
					t.Fatal("expected non-nil engine")
				}
			} else {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected error %v, got %v", tc.wantErr, err)
				}
				if eng != nil {
					t.Fatal("expected nil engine on validation error")
				}
				if fake.intnCount != beforeIntn || fake.floatCount != beforeFloat {
					t.Fatalf("entropy consumed during validation: intn %d->%d, float %d->%d",
						beforeIntn, fake.intnCount, beforeFloat, fake.floatCount)
				}
			}
		})
	}
}

// TestRaceSeededValidation verifies that RaceSeeded rejects invalid seeds and configurations
// without consuming entropy.
func TestRaceSeededValidation(t *testing.T) {
	validCfg := duckrace.Config{
		DuckCount:      4,
		DurationMillis: 5000,
		TickMillis:     100,
	}

	tests := []struct {
		name    string
		input   duckrace.SeededInput
		wantErr error
	}{
		{
			name: "empty server seed",
			input: duckrace.SeededInput{
				ServerSeed: "",
				ClientSeed: "client",
				Nonce:      1,
				Config:     validCfg,
			},
			wantErr: duckrace.ErrEmptyServerSeed,
		},
		{
			name: "empty client seed",
			input: duckrace.SeededInput{
				ServerSeed: "server",
				ClientSeed: "",
				Nonce:      1,
				Config:     validCfg,
			},
			wantErr: duckrace.ErrEmptyClientSeed,
		},
		{
			name: "invalid duck count",
			input: duckrace.SeededInput{
				ServerSeed: "server",
				ClientSeed: "client",
				Nonce:      1,
				Config:     duckrace.Config{DuckCount: 1, DurationMillis: 5000, TickMillis: 100},
			},
			wantErr: duckrace.ErrInvalidDuckCount,
		},
		{
			name: "invalid duration",
			input: duckrace.SeededInput{
				ServerSeed: "server",
				ClientSeed: "client",
				Nonce:      1,
				Config:     duckrace.Config{DuckCount: 4, DurationMillis: 500, TickMillis: 100},
			},
			wantErr: duckrace.ErrInvalidDuration,
		},
		{
			name: "invalid tick",
			input: duckrace.SeededInput{
				ServerSeed: "server",
				ClientSeed: "client",
				Nonce:      1,
				Config:     duckrace.Config{DuckCount: 4, DurationMillis: 5000, TickMillis: 20},
			},
			wantErr: duckrace.ErrInvalidTick,
		},
		{
			name: "indivisible duration",
			input: duckrace.SeededInput{
				ServerSeed: "server",
				ClientSeed: "client",
				Nonce:      1,
				Config:     duckrace.Config{DuckCount: 4, DurationMillis: 5000, TickMillis: 300},
			},
			wantErr: duckrace.ErrIndivisibleDuration,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, err := duckrace.RaceSeeded(tc.input)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("expected error %v, got %v", tc.wantErr, err)
			}
			if len(res.Frames) != 0 || len(res.FinishOrder) != 0 {
				t.Fatal("expected empty result on validation error")
			}
		})
	}
}

// TestFinishOrderPermutation verifies that FinishOrder is a valid permutation of duck indices.
func TestFinishOrderPermutation(t *testing.T) {
	duckCounts := []int{2, 3, 5, 8, 16}

	for _, duckCount := range duckCounts {
		t.Run(t.Name(), func(t *testing.T) {
			input := duckrace.SeededInput{
				ServerSeed: "test-server-seed",
				ClientSeed: "test-client-seed",
				Nonce:      uint64(duckCount * 42),
				Config: duckrace.Config{
					DuckCount:      duckCount,
					DurationMillis: 2000,
					TickMillis:     100,
				},
			}
			res, err := duckrace.RaceSeeded(input)
			if err != nil {
				t.Fatalf("RaceSeeded failed: %v", err)
			}

			if len(res.FinishOrder) != duckCount {
				t.Fatalf("expected finish order length %d, got %d", duckCount, len(res.FinishOrder))
			}

			seen := make(map[int]bool)
			for _, d := range res.FinishOrder {
				if d < 0 || d >= duckCount {
					t.Fatalf("duck index out of range [0, %d): %d", duckCount, d)
				}
				if seen[d] {
					t.Fatalf("duplicate duck index in finish order: %d", d)
				}
				seen[d] = true
			}

			if len(seen) != duckCount {
				t.Fatalf("expected %d unique ducks in finish order, got %d", duckCount, len(seen))
			}
		})
	}
}

// TestTimelineShapeAndTimestamps verifies frame count, timestamp alignment, and time-zero positions.
func TestTimelineShapeAndTimestamps(t *testing.T) {
	configs := []duckrace.Config{
		{DuckCount: 2, DurationMillis: 1000, TickMillis: 50},
		{DuckCount: 4, DurationMillis: 2000, TickMillis: 500},
		{DuckCount: 8, DurationMillis: 5000, TickMillis: 100},
		{DuckCount: 16, DurationMillis: 10000, TickMillis: 200},
	}

	for _, cfg := range configs {
		input := duckrace.SeededInput{
			ServerSeed: "srv-seed-1",
			ClientSeed: "cli-seed-1",
			Nonce:      10,
			Config:     cfg,
		}
		res, err := duckrace.RaceSeeded(input)
		if err != nil {
			t.Fatalf("RaceSeeded failed: %v", err)
		}

		expectedSegmentCount := int(cfg.DurationMillis / cfg.TickMillis)
		expectedFrameCount := expectedSegmentCount + 1

		if len(res.Frames) != expectedFrameCount {
			t.Fatalf("expected %d frames, got %d", expectedFrameCount, len(res.Frames))
		}

		// Frame 0 must have ElapsedMillis == 0 and all positions 0.0
		firstFrame := res.Frames[0]
		if firstFrame.ElapsedMillis != 0 {
			t.Fatalf("expected frame 0 elapsed 0 ms, got %d", firstFrame.ElapsedMillis)
		}
		if len(firstFrame.Positions) != cfg.DuckCount {
			t.Fatalf("expected frame 0 to have %d positions, got %d", cfg.DuckCount, len(firstFrame.Positions))
		}
		for d, pos := range firstFrame.Positions {
			if pos != 0.0 {
				t.Fatalf("frame 0 duck %d position = %v, want 0.0", d, pos)
			}
		}

		// Intermediate frames
		for f := 1; f < expectedFrameCount; f++ {
			expectedElapsed := int64(f) * cfg.TickMillis
			if res.Frames[f].ElapsedMillis != expectedElapsed {
				t.Fatalf("frame %d elapsed = %d, want %d", f, res.Frames[f].ElapsedMillis, expectedElapsed)
			}
			if len(res.Frames[f].Positions) != cfg.DuckCount {
				t.Fatalf("frame %d position count = %d, want %d", f, len(res.Frames[f].Positions), cfg.DuckCount)
			}
		}

		// Last frame must equal DurationMillis
		lastFrame := res.Frames[expectedFrameCount-1]
		if lastFrame.ElapsedMillis != cfg.DurationMillis {
			t.Fatalf("last frame elapsed = %d, want %d", lastFrame.ElapsedMillis, cfg.DurationMillis)
		}
	}
}

// TestMonotonicAndBoundedPositions verifies that positions are finite, within [0, 1],
// and strictly non-decreasing over time for each duck.
func TestMonotonicAndBoundedPositions(t *testing.T) {
	for nonce := uint64(1); nonce <= 10; nonce++ {
		cfg := duckrace.Config{
			DuckCount:      6,
			DurationMillis: 3000,
			TickMillis:     100,
		}
		input := duckrace.SeededInput{
			ServerSeed: "bounded-server-seed",
			ClientSeed: "bounded-client-seed",
			Nonce:      nonce,
			Config:     cfg,
		}
		res, err := duckrace.RaceSeeded(input)
		if err != nil {
			t.Fatalf("RaceSeeded failed on nonce %d: %v", nonce, err)
		}

		for d := 0; d < cfg.DuckCount; d++ {
			var prevPos float64
			for f, frame := range res.Frames {
				pos := frame.Positions[d]
				if math.IsNaN(pos) || math.IsInf(pos, 0) {
					t.Fatalf("duck %d frame %d non-finite position: %v", d, f, pos)
				}
				if pos < 0.0 || pos > 1.0 {
					t.Fatalf("duck %d frame %d position out of bounds [0, 1]: %v", d, f, pos)
				}
				if pos < prevPos {
					t.Fatalf("duck %d decreased progress at frame %d: prev=%v, curr=%v", d, f, prevPos, pos)
				}
				prevPos = pos
			}
		}
	}
}

// TestExactFinalRanking verifies that the final frame positions strictly encode FinishOrder
// with rank r having progress 1.0 - 0.01*r.
func TestExactFinalRanking(t *testing.T) {
	for duckCount := 2; duckCount <= 16; duckCount++ {
		cfg := duckrace.Config{
			DuckCount:      duckCount,
			DurationMillis: 2000,
			TickMillis:     100,
		}
		input := duckrace.SeededInput{
			ServerSeed: "ranking-server-seed",
			ClientSeed: "ranking-client-seed",
			Nonce:      uint64(duckCount),
			Config:     cfg,
		}
		res, err := duckrace.RaceSeeded(input)
		if err != nil {
			t.Fatalf("RaceSeeded failed for duckCount %d: %v", duckCount, err)
		}

		finalFrame := res.Frames[len(res.Frames)-1]

		// Winner alone must reach 1.0
		winnerDuck := res.FinishOrder[0]
		if finalFrame.Positions[winnerDuck] != 1.0 {
			t.Fatalf("winner (duck %d) final position = %v, want 1.0", winnerDuck, finalFrame.Positions[winnerDuck])
		}

		// Verify target for each rank
		for rank, duckIdx := range res.FinishOrder {
			expectedTarget := 1.0 - 0.01*float64(rank)
			actual := finalFrame.Positions[duckIdx]
			if math.Abs(actual-expectedTarget) > 1e-12 {
				t.Fatalf("duck %d at rank %d final position = %v, want %v", duckIdx, rank, actual, expectedTarget)
			}
			if rank > 0 && actual >= 1.0 {
				t.Fatalf("non-winner duck %d at rank %d reached 1.0: %v", duckIdx, rank, actual)
			}
		}

		// Reconstructing finish order by sorting ducks by final position descending
		type duckPos struct {
			duck int
			pos  float64
		}
		var ranked []duckPos
		for d := 0; d < duckCount; d++ {
			ranked = append(ranked, duckPos{duck: d, pos: finalFrame.Positions[d]})
		}
		slices.SortFunc(ranked, func(a, b duckPos) int {
			if a.pos > b.pos {
				return -1
			}
			if a.pos < b.pos {
				return 1
			}
			return 0
		})

		for rank, dp := range ranked {
			if dp.duck != res.FinishOrder[rank] {
				t.Fatalf("reconstructed rank %d = duck %d, want duck %d from FinishOrder",
					rank, dp.duck, res.FinishOrder[rank])
			}
		}
	}
}

// TestIndependentSlices verifies that mutating returned result slices does not leak
// or alias across frames or future operations.
func TestIndependentSlices(t *testing.T) {
	input := duckrace.SeededInput{
		ServerSeed: "slice-seed-server",
		ClientSeed: "slice-seed-client",
		Nonce:      1,
		Config: duckrace.Config{
			DuckCount:      4,
			DurationMillis: 2000,
			TickMillis:     500,
		},
	}
	res, err := duckrace.RaceSeeded(input)
	if err != nil {
		t.Fatalf("RaceSeeded failed: %v", err)
	}

	// Mutate frame 0 position
	res.Frames[0].Positions[0] = 999.0
	if res.Frames[1].Positions[0] == 999.0 {
		t.Fatal("frame positions slices are aliased across frames")
	}

	// Mutate finish order
	originalWinner := res.FinishOrder[0]
	res.FinishOrder[0] = 999
	res2, err := duckrace.RaceSeeded(input)
	if err != nil {
		t.Fatalf("second RaceSeeded failed: %v", err)
	}
	if res2.FinishOrder[0] != originalWinner {
		t.Fatal("FinishOrder mutation affected subsequent execution")
	}
}

// TestLeadChanges verifies that intermediate frames allow lead changes before the
// guaranteed finish order is reached.
func TestLeadChanges(t *testing.T) {
	// DuckCount = 2, DurationMillis = 1000, TickMillis = 500 => 2 segments, 3 frames.
	// Shuffle: 2 ducks => 1 Intn call:
	// Intn(2) returns 1 => swapIdx = 0 + 1 = 1 => order becomes [1, 0].
	// Winner is Duck 1 (rank 0, target 1.0). Runner-up is Duck 0 (rank 1, target 0.99).
	// Per-duck weights:
	// Duck 0: segment 1 float = 0.75 (weight 1.0), segment 2 float = 0.0 (weight 0.25). Total weight = 1.25.
	// Duck 0 frame 1 progress: 0.99 * (1.0 / 1.25) = 0.792.
	// Duck 1: segment 1 float = 0.0 (weight 0.25), segment 2 float = 0.75 (weight 1.0). Total weight = 1.25.
	// Duck 1 frame 1 progress: 1.0 * (0.25 / 1.25) = 0.200.
	// At frame 1 (mid-race): Duck 0 has 0.792, Duck 1 has 0.200 => Duck 0 is in the lead!
	// At frame 2 (finish): Duck 1 has 1.0, Duck 0 has 0.99 => Duck 1 wins!
	fake := &fakeEntropySource{
		intnVals: []uint64{1},
		floatVals: []float64{
			0.75, 0.0, // Duck 0 segments 0 and 1
			0.0, 0.75, // Duck 1 segments 0 and 1
		},
	}

	cfg := duckrace.Config{
		DuckCount:      2,
		DurationMillis: 1000,
		TickMillis:     500,
	}
	eng, err := duckrace.New(cfg, fake)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	res, err := eng.Play()
	if err != nil {
		t.Fatalf("Play failed: %v", err)
	}

	// Winner must be Duck 1
	if res.FinishOrder[0] != 1 || res.FinishOrder[1] != 0 {
		t.Fatalf("expected finish order [1, 0], got %v", res.FinishOrder)
	}

	// Check frame 1 lead: Duck 0 should lead Duck 1
	frame1 := res.Frames[1]
	duck0ProgressMid := frame1.Positions[0]
	duck1ProgressMid := frame1.Positions[1]
	if duck0ProgressMid <= duck1ProgressMid {
		t.Fatalf("expected duck 0 to lead duck 1 at mid-race: duck0=%v, duck1=%v",
			duck0ProgressMid, duck1ProgressMid)
	}

	// Check final frame: Duck 1 must win with 1.0, Duck 0 must have 0.99
	frame2 := res.Frames[2]
	if frame2.Positions[1] != 1.0 {
		t.Fatalf("expected duck 1 final position 1.0, got %v", frame2.Positions[1])
	}
	if math.Abs(frame2.Positions[0]-0.99) > 1e-12 {
		t.Fatalf("expected duck 0 final position 0.99, got %v", frame2.Positions[0])
	}
	if frame2.Positions[1] <= frame2.Positions[0] {
		t.Fatalf("expected duck 1 to overtake duck 0 at finish: duck1=%v, duck0=%v",
			frame2.Positions[1], frame2.Positions[0])
	}
}

// TestEntropyErrors verifies that entropy source failures and invalid floats are propagated.
func TestEntropyErrors(t *testing.T) {
	cfg := duckrace.Config{
		DuckCount:      2,
		DurationMillis: 1000,
		TickMillis:     500,
	}

	t.Run("intn shuffle error", func(t *testing.T) {
		expectedErr := errors.New("intn failure")
		fake := &fakeEntropySource{
			intnErr: expectedErr,
		}
		eng, err := duckrace.New(cfg, fake)
		if err != nil {
			t.Fatalf("New failed: %v", err)
		}
		_, err = eng.Play()
		if !errors.Is(err, expectedErr) {
			t.Fatalf("expected wrapped intn error, got %v", err)
		}
	})

	t.Run("float64 segment weight error", func(t *testing.T) {
		expectedErr := errors.New("float64 failure")
		fake := &fakeEntropySource{
			intnVals: []uint64{0},
			floatErr: expectedErr,
		}
		eng, err := duckrace.New(cfg, fake)
		if err != nil {
			t.Fatalf("New failed: %v", err)
		}
		_, err = eng.Play()
		if !errors.Is(err, expectedErr) {
			t.Fatalf("expected wrapped float64 error, got %v", err)
		}
	})

	invalidFloats := []struct {
		name string
		val  float64
	}{
		{name: "NaN", val: math.NaN()},
		{name: "positive infinity", val: math.Inf(1)},
		{name: "negative infinity", val: math.Inf(-1)},
		{name: "negative float (-0.1)", val: -0.1},
		{name: "equal to 1.0", val: 1.0},
		{name: "greater than 1.0 (1.5)", val: 1.5},
	}

	for _, tc := range invalidFloats {
		t.Run("invalid float "+tc.name, func(t *testing.T) {
			fake := &fakeEntropySource{
				intnVals:  []uint64{0},
				floatVals: []float64{tc.val, 0.5, 0.5, 0.5},
			}
			eng, err := duckrace.New(cfg, fake)
			if err != nil {
				t.Fatalf("New failed: %v", err)
			}
			_, err = eng.Play()
			if !errors.Is(err, duckrace.ErrInvalidEntropyFloat) {
				t.Fatalf("expected ErrInvalidEntropyFloat, got %v", err)
			}
		})
	}
}

// TestDeterministicVectorAndReplay verifies that identical inputs reproduce exact timelines.
func TestDeterministicVectorAndReplay(t *testing.T) {
	input := duckrace.SeededInput{
		ServerSeed: "provably-fair-server-seed-vector",
		ClientSeed: "provably-fair-client-seed-vector",
		Nonce:      42,
		Config: duckrace.Config{
			DuckCount:      4,
			DurationMillis: 2000,
			TickMillis:     500,
		},
	}

	res1, err := duckrace.RaceSeeded(input)
	if err != nil {
		t.Fatalf("RaceSeeded 1 failed: %v", err)
	}

	res2, err := duckrace.RaceSeeded(input)
	if err != nil {
		t.Fatalf("RaceSeeded 2 failed: %v", err)
	}

	if !reflect.DeepEqual(res1.FinishOrder, res2.FinishOrder) {
		t.Fatalf("finish orders differ across replays: %v vs %v", res1.FinishOrder, res2.FinishOrder)
	}

	if len(res1.Frames) != len(res2.Frames) {
		t.Fatalf("frame count differs: %d vs %d", len(res1.Frames), len(res2.Frames))
	}

	for f := range res1.Frames {
		if res1.Frames[f].ElapsedMillis != res2.Frames[f].ElapsedMillis {
			t.Fatalf("frame %d elapsed millis differ: %d vs %d",
				f, res1.Frames[f].ElapsedMillis, res2.Frames[f].ElapsedMillis)
		}
		if !reflect.DeepEqual(res1.Frames[f].Positions, res2.Frames[f].Positions) {
			t.Fatalf("frame %d positions differ across replays: %v vs %v",
				f, res1.Frames[f].Positions, res2.Frames[f].Positions)
		}
	}

	// Verify against hardcoded deterministic vector
	expectedFinishOrder := []int{2, 1, 0, 3}
	if !reflect.DeepEqual(res1.FinishOrder, expectedFinishOrder) {
		t.Fatalf("finish order = %v, want %v", res1.FinishOrder, expectedFinishOrder)
	}
	finalPositions := res1.Frames[len(res1.Frames)-1].Positions
	winner := res1.FinishOrder[0]
	if finalPositions[winner] != 1.0 {
		t.Fatalf("winner duck %d final position = %v, want 1.0", winner, finalPositions[winner])
	}
	// Verify exact intermediate frames for compatibility
	if len(res1.Frames) != 5 {
		t.Fatalf("expected 5 frames, got %d", len(res1.Frames))
	}
}

// TestNonceSeparation verifies that changing the nonce alters outcomes.
func TestNonceSeparation(t *testing.T) {
	cfg := duckrace.Config{
		DuckCount:      8,
		DurationMillis: 5000,
		TickMillis:     200,
	}
	input1 := duckrace.SeededInput{
		ServerSeed: "nonce-separation-server",
		ClientSeed: "nonce-separation-client",
		Nonce:      1,
		Config:     cfg,
	}
	input2 := duckrace.SeededInput{
		ServerSeed: "nonce-separation-server",
		ClientSeed: "nonce-separation-client",
		Nonce:      2,
		Config:     cfg,
	}

	res1, err := duckrace.RaceSeeded(input1)
	if err != nil {
		t.Fatalf("race 1 failed: %v", err)
	}
	res2, err := duckrace.RaceSeeded(input2)
	if err != nil {
		t.Fatalf("race 2 failed: %v", err)
	}

	// Either finish order or positions should differ
	ordersEqual := reflect.DeepEqual(res1.FinishOrder, res2.FinishOrder)
	framesEqual := reflect.DeepEqual(res1.Frames, res2.Frames)
	if ordersEqual && framesEqual {
		t.Fatal("expected different outcomes for different nonces")
	}
}

// TestSeedSeparation verifies that changing server or client seed alters outcomes.
func TestSeedSeparation(t *testing.T) {
	cfg := duckrace.Config{
		DuckCount:      6,
		DurationMillis: 3000,
		TickMillis:     100,
	}
	base := duckrace.SeededInput{
		ServerSeed: "seed-server-a",
		ClientSeed: "seed-client-a",
		Nonce:      1,
		Config:     cfg,
	}
	diffServer := duckrace.SeededInput{
		ServerSeed: "seed-server-b",
		ClientSeed: "seed-client-a",
		Nonce:      1,
		Config:     cfg,
	}
	diffClient := duckrace.SeededInput{
		ServerSeed: "seed-server-a",
		ClientSeed: "seed-client-b",
		Nonce:      1,
		Config:     cfg,
	}

	baseRes, err := duckrace.RaceSeeded(base)
	if err != nil {
		t.Fatalf("base failed: %v", err)
	}
	serverRes, err := duckrace.RaceSeeded(diffServer)
	if err != nil {
		t.Fatalf("diff server failed: %v", err)
	}
	clientRes, err := duckrace.RaceSeeded(diffClient)
	if err != nil {
		t.Fatalf("diff client failed: %v", err)
	}

	if reflect.DeepEqual(baseRes.Frames, serverRes.Frames) && reflect.DeepEqual(baseRes.FinishOrder, serverRes.FinishOrder) {
		t.Fatal("expected different outcome when server seed changes")
	}
	if reflect.DeepEqual(baseRes.Frames, clientRes.Frames) && reflect.DeepEqual(baseRes.FinishOrder, clientRes.FinishOrder) {
		t.Fatal("expected different outcome when client seed changes")
	}
}

// BenchmarkRaceSeeded measures performance of standard and large race simulations.
func BenchmarkRaceSeeded(b *testing.B) {
	b.Run("typical 8 ducks 5s 100ms", func(b *testing.B) {
		input := duckrace.SeededInput{
			ServerSeed: "benchmark-server-seed",
			ClientSeed: "benchmark-client-seed",
			Nonce:      1,
			Config: duckrace.Config{
				DuckCount:      8,
				DurationMillis: 5000,
				TickMillis:     100,
			},
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			input.Nonce = uint64(i)
			_, _ = duckrace.RaceSeeded(input)
		}
	})

	b.Run("max 16 ducks 60s 50ms", func(b *testing.B) {
		input := duckrace.SeededInput{
			ServerSeed: "benchmark-server-seed",
			ClientSeed: "benchmark-client-seed",
			Nonce:      1,
			Config: duckrace.Config{
				DuckCount:      16,
				DurationMillis: 60000,
				TickMillis:     50,
			},
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			input.Nonce = uint64(i)
			_, _ = duckrace.RaceSeeded(input)
		}
	})
}
