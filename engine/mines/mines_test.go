package mines_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"testing"

	"github.com/qxbao/qxprob/core/entropy"
	"github.com/qxbao/qxprob/core/simulation"
	"github.com/qxbao/qxprob/engine/mines"
)

// fakeEntropySource is a deterministic fake implementing entropy.Source for tests.
type fakeEntropySource struct {
	decisions []uint64
	index     int
	err       error
	readCount int
	bounds    []uint64
}

func (s *fakeEntropySource) Intn(n uint64) (uint64, error) {
	s.readCount++
	s.bounds = append(s.bounds, n)
	if s.err != nil {
		return 0, s.err
	}
	if s.index >= len(s.decisions) {
		return 0, errors.New("fake: no more decisions")
	}
	val := s.decisions[s.index]
	s.index++
	return val % n, nil
}

func (s *fakeEntropySource) Float64() (float64, error) {
	return 0, errors.New("fake: Float64 not implemented")
}

func (s *fakeEntropySource) Bits(int) (uint64, error) {
	return 0, errors.New("fake: Bits not implemented")
}

func (s *fakeEntropySource) Uint64() (uint64, error) {
	return 0, errors.New("fake: Uint64 not implemented")
}

var _ entropy.Source = (*fakeEntropySource)(nil)

// TestEngineInterfaceCompatibility verifies that *mines.Engine satisfies simulation.Engine[mines.Board].
func TestEngineInterfaceCompatibility(t *testing.T) {
	fake := &fakeEntropySource{
		decisions: []uint64{0, 1, 2},
	}
	eng, err := mines.New(mines.Config{MineCount: 3}, fake)
	if err != nil {
		t.Fatalf("unexpected construction failure: %v", err)
	}
	var _ simulation.Engine[mines.Board] = eng

	var roundCount uint64
	err = simulation.Run(context.Background(), eng, 1, func(round uint64, b mines.Board) error {
		roundCount++
		if len(b.MineIndices) != 3 {
			t.Fatalf("expected 3 mines, got %d", len(b.MineIndices))
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
	fake := &fakeEntropySource{decisions: make([]uint64, 32)}

	tests := []struct {
		name      string
		config    mines.Config
		source    entropy.Source
		wantErr   error
		wantValid bool
	}{
		{
			name:      "nil source",
			config:    mines.Config{MineCount: 3},
			source:    nil,
			wantErr:   mines.ErrNilSource,
			wantValid: false,
		},
		{
			name:      "mine count below minimum (0)",
			config:    mines.Config{MineCount: 0},
			source:    fake,
			wantErr:   mines.ErrInvalidMineCount,
			wantValid: false,
		},
		{
			name:      "negative mine count (-1)",
			config:    mines.Config{MineCount: -1},
			source:    fake,
			wantErr:   mines.ErrInvalidMineCount,
			wantValid: false,
		},
		{
			name:      "mine count above maximum (25)",
			config:    mines.Config{MineCount: 25},
			source:    fake,
			wantErr:   mines.ErrInvalidMineCount,
			wantValid: false,
		},
		{
			name:      "mine count above maximum (100)",
			config:    mines.Config{MineCount: 100},
			source:    fake,
			wantErr:   mines.ErrInvalidMineCount,
			wantValid: false,
		},
		{
			name:      "lower boundary (1)",
			config:    mines.Config{MineCount: mines.MinMines},
			source:    fake,
			wantErr:   nil,
			wantValid: true,
		},
		{
			name:      "upper boundary (24)",
			config:    mines.Config{MineCount: mines.MaxMines},
			source:    fake,
			wantErr:   nil,
			wantValid: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake.readCount = 0
			eng, err := mines.New(tc.config, tc.source)
			if tc.wantValid {
				if err != nil {
					t.Fatalf("unexpected construction error: %v", err)
				}
				if eng == nil {
					t.Fatal("expected non-nil Engine, got nil")
				}
			} else {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("error = %v, want %v", err, tc.wantErr)
				}
				if eng != nil {
					t.Fatalf("expected nil Engine on error, got %v", eng)
				}
			}
			if fake.readCount != 0 {
				t.Fatalf("constructor consumed %d entropy values, want 0", fake.readCount)
			}
		})
	}
}

// TestGenerateSeededValidation verifies that GenerateSeeded rejects empty seeds and
// out-of-range mine counts before consuming entropy.
func TestGenerateSeededValidation(t *testing.T) {
	tests := []struct {
		name    string
		input   mines.SeededInput
		wantErr error
	}{
		{
			name: "empty server seed",
			input: mines.SeededInput{
				ServerSeed: "",
				ClientSeed: "client-pub",
				Nonce:      1,
				MineCount:  3,
			},
			wantErr: mines.ErrEmptyServerSeed,
		},
		{
			name: "empty client seed",
			input: mines.SeededInput{
				ServerSeed: "server-sec",
				ClientSeed: "",
				Nonce:      1,
				MineCount:  3,
			},
			wantErr: mines.ErrEmptyClientSeed,
		},
		{
			name: "mine count below minimum (0)",
			input: mines.SeededInput{
				ServerSeed: "server-sec",
				ClientSeed: "client-pub",
				Nonce:      1,
				MineCount:  0,
			},
			wantErr: mines.ErrInvalidMineCount,
		},
		{
			name: "negative mine count",
			input: mines.SeededInput{
				ServerSeed: "server-sec",
				ClientSeed: "client-pub",
				Nonce:      1,
				MineCount:  -5,
			},
			wantErr: mines.ErrInvalidMineCount,
		},
		{
			name: "mine count above maximum (25)",
			input: mines.SeededInput{
				ServerSeed: "server-sec",
				ClientSeed: "client-pub",
				Nonce:      1,
				MineCount:  25,
			},
			wantErr: mines.ErrInvalidMineCount,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b, err := mines.GenerateSeeded(tc.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
			if len(b.MineIndices) != 0 {
				t.Fatalf("expected empty MineIndices on error, got %+v", b)
			}
		})
	}
}

// TestSourceErrorPropagation verifies that source errors during Play are wrapped
// and discoverable via errors.Is.
func TestSourceErrorPropagation(t *testing.T) {
	injectedErr := errors.New("entropy stream failure")

	t.Run("first call failure", func(t *testing.T) {
		fake := &fakeEntropySource{err: injectedErr}
		eng, err := mines.New(mines.Config{MineCount: 5}, fake)
		if err != nil {
			t.Fatalf("unexpected New failure: %v", err)
		}
		_, err = eng.Play()
		if err == nil {
			t.Fatal("expected Play to fail, got nil")
		}
		if !errors.Is(err, injectedErr) {
			t.Fatalf("expected wrapped injected error, got %v", err)
		}
	})

	t.Run("subsequent call failure", func(t *testing.T) {
		// Provide 2 successful decisions, then run out of decisions
		fake := &fakeEntropySource{
			decisions: []uint64{0, 1},
		}
		eng, err := mines.New(mines.Config{MineCount: 5}, fake)
		if err != nil {
			t.Fatalf("unexpected New failure: %v", err)
		}
		_, err = eng.Play()
		if err == nil {
			t.Fatal("expected Play to fail when decisions exhausted, got nil")
		}
	})
}

// TestPartialShuffleBounds verifies that partial Fisher-Yates shuffle calls Intn with
// remaining bounds TileCount down to TileCount - MineCount + 1.
func TestPartialShuffleBounds(t *testing.T) {
	tests := []struct {
		name       string
		mineCount  int
		wantBounds []uint64
	}{
		{
			name:       "1 mine",
			mineCount:  1,
			wantBounds: []uint64{25},
		},
		{
			name:       "3 mines",
			mineCount:  3,
			wantBounds: []uint64{25, 24, 23},
		},
		{
			name:       "5 mines",
			mineCount:  5,
			wantBounds: []uint64{25, 24, 23, 22, 21},
		},
		{
			name:      "24 mines",
			mineCount: 24,
			wantBounds: []uint64{
				25, 24, 23, 22, 21, 20, 19, 18, 17, 16, 15, 14,
				13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeEntropySource{
				decisions: make([]uint64, tc.mineCount),
			}
			eng, err := mines.New(mines.Config{MineCount: tc.mineCount}, fake)
			if err != nil {
				t.Fatalf("unexpected New failure: %v", err)
			}
			_, err = eng.Play()
			if err != nil {
				t.Fatalf("Play failed: %v", err)
			}
			if len(fake.bounds) != len(tc.wantBounds) {
				t.Fatalf("bounds count = %d, want %d", len(fake.bounds), len(tc.wantBounds))
			}
			for i, b := range fake.bounds {
				if b != tc.wantBounds[i] {
					t.Fatalf("step %d bound = %d, want %d", i, b, tc.wantBounds[i])
				}
			}
		})
	}
}

// TestBoardGenerationInvariants verifies that for all mine counts [1, 24], the generated
// board contains exactly MineCount mines, all in [0, 24], strictly unique, and strictly sorted.
func TestBoardGenerationInvariants(t *testing.T) {
	for count := mines.MinMines; count <= mines.MaxMines; count++ {
		source := entropy.NewProvablyFairSource("server-invariants", "client-invariants", uint64(count))
		eng, err := mines.New(mines.Config{MineCount: count}, source)
		if err != nil {
			t.Fatalf("count %d: unexpected New error: %v", count, err)
		}

		for round := 0; round < 10; round++ {
			b, err := eng.Play()
			if err != nil {
				t.Fatalf("count %d round %d: Play failed: %v", count, round, err)
			}

			if len(b.MineIndices) != count {
				t.Fatalf("count %d: got %d mines, want %d", count, len(b.MineIndices), count)
			}

			// Verify bounds and strictly sorted / unique
			for i, mine := range b.MineIndices {
				if mine < 0 || mine >= mines.TileCount {
					t.Fatalf("count %d: mine index out of range: %d", count, mine)
				}
				if i > 0 && mine <= b.MineIndices[i-1] {
					t.Fatalf("count %d: mines not strictly sorted: %v", count, b.MineIndices)
				}
			}
		}
	}
}

// TestBoardEvaluateIntegrityValidation verifies that Board.Evaluate rejects invalid board
// states before evaluating picks.
func TestBoardEvaluateIntegrityValidation(t *testing.T) {
	validPicks := []int{0, 1}

	tests := []struct {
		name        string
		mineIndices []int
		wantErr     error
	}{
		{
			name:        "nil mines",
			mineIndices: nil,
			wantErr:     mines.ErrInvalidBoard,
		},
		{
			name:        "empty mines",
			mineIndices: []int{},
			wantErr:     mines.ErrInvalidBoard,
		},
		{
			name: "too many mines (25)",
			mineIndices: []int{
				0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12,
				13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24,
			},
			wantErr: mines.ErrInvalidBoard,
		},
		{
			name:        "negative mine index",
			mineIndices: []int{-1, 2},
			wantErr:     mines.ErrInvalidBoard,
		},
		{
			name:        "out of range mine index (25)",
			mineIndices: []int{5, 25},
			wantErr:     mines.ErrInvalidBoard,
		},
		{
			name:        "duplicate mine indices",
			mineIndices: []int{3, 7, 3},
			wantErr:     mines.ErrInvalidBoard,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := mines.Board{MineIndices: tc.mineIndices}
			_, err := b.Evaluate(validPicks)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

// TestBoardEvaluatePicksValidation verifies that Board.Evaluate validates picks (range and
// duplicates) across all picks before evaluating.
func TestBoardEvaluatePicksValidation(t *testing.T) {
	b := mines.Board{MineIndices: []int{5, 10, 15}}

	tests := []struct {
		name    string
		picks   []int
		wantErr error
	}{
		{
			name:    "negative pick",
			picks:   []int{-1},
			wantErr: mines.ErrInvalidTile,
		},
		{
			name:    "pick 25 (out of range)",
			picks:   []int{25},
			wantErr: mines.ErrInvalidTile,
		},
		{
			name:    "pick 100",
			picks:   []int{1, 2, 100},
			wantErr: mines.ErrInvalidTile,
		},
		{
			name:    "duplicate pick",
			picks:   []int{0, 1, 0},
			wantErr: mines.ErrDuplicatePick,
		},
		{
			name:    "duplicate pick occurring after a mine",
			picks:   []int{5, 2, 2}, // tile 5 is a mine
			wantErr: mines.ErrDuplicatePick,
		},
		{
			name:    "out of range pick occurring after a mine",
			picks:   []int{5, 30}, // tile 5 is a mine
			wantErr: mines.ErrInvalidTile,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := b.Evaluate(tc.picks)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

// TestBoardEvaluateEmptyPicks verifies that 0 safe picks on a non-lost round return a
// multiplier of 1.0 and an empty reveals list.
func TestBoardEvaluateEmptyPicks(t *testing.T) {
	b := mines.Board{MineIndices: []int{5, 10}}

	for _, picks := range [][]int{nil, {}} {
		eval, err := b.Evaluate(picks)
		if err != nil {
			t.Fatalf("unexpected Evaluate error: %v", err)
		}
		if eval.Lost {
			t.Fatal("expected Lost to be false")
		}
		if eval.SafePicks != 0 {
			t.Fatalf("SafePicks = %d, want 0", eval.SafePicks)
		}
		if eval.Multiplier != 1.0 {
			t.Fatalf("Multiplier = %v, want 1.0", eval.Multiplier)
		}
		if len(eval.Reveals) != 0 {
			t.Fatalf("len(Reveals) = %d, want 0", len(eval.Reveals))
		}
	}
}

// TestBoardEvaluateSafeAndMinePaths verifies step-by-step evaluation on safe and mine paths.
func TestBoardEvaluateSafeAndMinePaths(t *testing.T) {
	// 3 mines at {3, 7, 15}
	b := mines.Board{MineIndices: []int{3, 7, 15}}

	t.Run("all safe picks", func(t *testing.T) {
		picks := []int{0, 1, 2}
		eval, err := b.Evaluate(picks)
		if err != nil {
			t.Fatalf("unexpected Evaluate error: %v", err)
		}
		if eval.Lost {
			t.Fatal("expected Lost to be false")
		}
		if eval.SafePicks != 3 {
			t.Fatalf("SafePicks = %d, want 3", eval.SafePicks)
		}
		if len(eval.Reveals) != 3 {
			t.Fatalf("len(Reveals) = %d, want 3", len(eval.Reveals))
		}

		// Reveal 0: safe pick 1
		// C(22, 1) / C(25, 1) = 22/25 = 0.88 -> 0.99 / 0.88 = 1.125
		if eval.Reveals[0].Tile != 0 || eval.Reveals[0].Mine || eval.Reveals[0].SafePicks != 1 {
			t.Fatalf("unexpected reveal 0: %+v", eval.Reveals[0])
		}
		if math.Abs(eval.Reveals[0].Multiplier-1.125) > 1e-12 {
			t.Fatalf("reveal 0 multiplier = %v, want 1.125", eval.Reveals[0].Multiplier)
		}

		// Reveal 1: safe pick 2
		// C(22, 2) / C(25, 2) = 231 / 300 = 0.77 -> 0.99 / 0.77 = 297/231 = 27/21 = 9/7
		expectedM2 := 0.99 / (231.0 / 300.0)
		if eval.Reveals[1].Tile != 1 || eval.Reveals[1].Mine || eval.Reveals[1].SafePicks != 2 {
			t.Fatalf("unexpected reveal 1: %+v", eval.Reveals[1])
		}
		if math.Abs(eval.Reveals[1].Multiplier-expectedM2) > 1e-12 {
			t.Fatalf("reveal 1 multiplier = %v, want %v", eval.Reveals[1].Multiplier, expectedM2)
		}

		// Reveal 2: safe pick 3
		// C(22, 3) / C(25, 3) = 1540 / 2300 = 0.6695652173913044 -> 0.99 / prob
		expectedM3 := 0.99 / (1540.0 / 2300.0)
		if eval.Reveals[2].Tile != 2 || eval.Reveals[2].Mine || eval.Reveals[2].SafePicks != 3 {
			t.Fatalf("unexpected reveal 2: %+v", eval.Reveals[2])
		}
		if math.Abs(eval.Reveals[2].Multiplier-expectedM3) > 1e-12 {
			t.Fatalf("reveal 2 multiplier = %v, want %v", eval.Reveals[2].Multiplier, expectedM3)
		}
		if math.Abs(eval.Multiplier-expectedM3) > 1e-12 {
			t.Fatalf("final multiplier = %v, want %v", eval.Multiplier, expectedM3)
		}
	})

	t.Run("first pick is a mine", func(t *testing.T) {
		picks := []int{3} // tile 3 is a mine
		eval, err := b.Evaluate(picks)
		if err != nil {
			t.Fatalf("unexpected Evaluate error: %v", err)
		}
		if !eval.Lost {
			t.Fatal("expected Lost to be true")
		}
		if eval.SafePicks != 0 {
			t.Fatalf("SafePicks = %d, want 0", eval.SafePicks)
		}
		if eval.Multiplier != 0.0 {
			t.Fatalf("Multiplier = %v, want 0.0", eval.Multiplier)
		}
		if len(eval.Reveals) != 1 {
			t.Fatalf("len(Reveals) = %d, want 1", len(eval.Reveals))
		}
		if eval.Reveals[0].Tile != 3 || !eval.Reveals[0].Mine || eval.Reveals[0].SafePicks != 0 || eval.Reveals[0].Multiplier != 0.0 {
			t.Fatalf("unexpected reveal: %+v", eval.Reveals[0])
		}
	})

	t.Run("stops at first mine", func(t *testing.T) {
		// picks: safe (0), safe (1), mine (3), safe (4)
		picks := []int{0, 1, 3, 4}
		eval, err := b.Evaluate(picks)
		if err != nil {
			t.Fatalf("unexpected Evaluate error: %v", err)
		}
		if !eval.Lost {
			t.Fatal("expected Lost to be true")
		}
		if eval.SafePicks != 2 {
			t.Fatalf("SafePicks = %d, want 2", eval.SafePicks)
		}
		if eval.Multiplier != 0.0 {
			t.Fatalf("Multiplier = %v, want 0.0", eval.Multiplier)
		}
		if len(eval.Reveals) != 3 {
			t.Fatalf("len(Reveals) = %d, want 3 (should stop at mine)", len(eval.Reveals))
		}
		if eval.Reveals[2].Tile != 3 || !eval.Reveals[2].Mine || eval.Reveals[2].SafePicks != 2 || eval.Reveals[2].Multiplier != 0.0 {
			t.Fatalf("unexpected mine reveal: %+v", eval.Reveals[2])
		}
	})

	t.Run("all safe tiles picked on board with 24 mines", func(t *testing.T) {
		// 24 mines: tiles 0..23. Tile 24 is safe.
		mines24 := make([]int, 24)
		for i := 0; i < 24; i++ {
			mines24[i] = i
		}
		board24 := mines.Board{MineIndices: mines24}
		eval, err := board24.Evaluate([]int{24})
		if err != nil {
			t.Fatalf("unexpected Evaluate error: %v", err)
		}
		if eval.Lost {
			t.Fatal("expected Lost to be false")
		}
		if eval.SafePicks != 1 {
			t.Fatalf("SafePicks = %d, want 1", eval.SafePicks)
		}
		// C(1, 1) / C(25, 1) = 1/25 = 0.04 -> 0.99 / 0.04 = 24.75
		if math.Abs(eval.Multiplier-24.75) > 1e-12 {
			t.Fatalf("Multiplier = %v, want 24.75", eval.Multiplier)
		}
	})

	t.Run("all 24 safe tiles picked on board with 1 mine", func(t *testing.T) {
		// 1 mine at tile 24. Tiles 0..23 are safe.
		board1 := mines.Board{MineIndices: []int{24}}
		picks := make([]int, 24)
		for i := 0; i < 24; i++ {
			picks[i] = i
		}
		eval, err := board1.Evaluate(picks)
		if err != nil {
			t.Fatalf("unexpected Evaluate error: %v", err)
		}
		if eval.Lost {
			t.Fatal("expected Lost to be false")
		}
		if eval.SafePicks != 24 {
			t.Fatalf("SafePicks = %d, want 24", eval.SafePicks)
		}
		// C(24, 24) / C(25, 24) = 1/25 = 0.04 -> 0.99 / 0.04 = 24.75
		if math.Abs(eval.Multiplier-24.75) > 1e-12 {
			t.Fatalf("Multiplier = %v, want 24.75", eval.Multiplier)
		}
	})
}

// TestMultiplierFormulasAndBoundaries exhaustively tests all possible MineCount and SafePicks
// configurations to verify finite, strictly increasing multipliers and exact 0.99 RTP.
func TestMultiplierFormulasAndBoundaries(t *testing.T) {
	for mineCount := mines.MinMines; mineCount <= mines.MaxMines; mineCount++ {
		safeTiles := mines.TileCount - mineCount
		// Create a board where mines are at safeTiles..24, so picks 0..s-1 are always safe
		mineIndices := make([]int, mineCount)
		for i := 0; i < mineCount; i++ {
			mineIndices[i] = safeTiles + i
		}
		b := mines.Board{MineIndices: mineIndices}

		var prevMultiplier float64 = 1.0

		for s := 1; s <= safeTiles; s++ {
			picks := make([]int, s)
			for i := 0; i < s; i++ {
				picks[i] = i
			}

			eval, err := b.Evaluate(picks)
			if err != nil {
				t.Fatalf("M=%d s=%d: Evaluate failed: %v", mineCount, s, err)
			}
			if eval.Lost {
				t.Fatalf("M=%d s=%d: expected safe round, got lost", mineCount, s)
			}
			if eval.SafePicks != s {
				t.Fatalf("M=%d s=%d: SafePicks = %d, want %d", mineCount, s, eval.SafePicks, s)
			}

			m := eval.Multiplier
			if math.IsNaN(m) || math.IsInf(m, 0) || m <= 0 {
				t.Fatalf("M=%d s=%d: invalid multiplier: %v", mineCount, s, m)
			}

			// Multiplier must strictly increase with each safe pick
			if m <= prevMultiplier {
				t.Fatalf("M=%d s=%d: multiplier %v did not strictly exceed previous %v", mineCount, s, m, prevMultiplier)
			}
			prevMultiplier = m

			// Theoretical win probability
			// P(s) = (C(25-M, s) / C(25, s))
			// Expected return = P(s) * m == 0.99
			cSafe := float64(binomialCoeff(safeTiles, s))
			cTotal := float64(binomialCoeff(mines.TileCount, s))
			prob := cSafe / cTotal
			expectedRTP := prob * m
			if math.Abs(expectedRTP-0.99) > 1e-12 {
				t.Fatalf("M=%d s=%d: expected RTP = %v, want 0.99", mineCount, s, expectedRTP)
			}
		}
	}
}

// binomialCoeff is a test oracle for combination calculation.
func binomialCoeff(n, k int) uint64 {
	if k < 0 || k > n {
		return 0
	}
	if k == 0 || k == n {
		return 1
	}
	if k > n/2 {
		k = n - k
	}
	res := uint64(1)
	for i := 1; i <= k; i++ {
		res = (res * uint64(n-i+1)) / uint64(i)
	}
	return res
}

// TestBoardImmutability verifies that evaluating a board does not mutate its MineIndices,
// and that mutating the picks slice does not affect the evaluation result.
func TestBoardImmutability(t *testing.T) {
	origMines := []int{2, 5, 8, 11}
	minesCopy := slices.Clone(origMines)
	b := mines.Board{MineIndices: minesCopy}

	picks := []int{0, 1, 2, 3}
	eval1, err := b.Evaluate(picks)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	// Verify b.MineIndices unchanged
	if !slices.Equal(b.MineIndices, origMines) {
		t.Fatalf("Board.MineIndices was mutated: got %v, want %v", b.MineIndices, origMines)
	}

	// Mutate picks slice
	picks[0] = 99
	if eval1.Reveals[0].Tile != 0 {
		t.Fatalf("Reveals slice mutated after caller modified picks: got %d, want 0", eval1.Reveals[0].Tile)
	}

	// Second evaluate produces identical result
	eval2, err := b.Evaluate([]int{0, 1, 2, 3})
	if err != nil {
		t.Fatalf("second Evaluate failed: %v", err)
	}
	if eval1.Lost != eval2.Lost || eval1.SafePicks != eval2.SafePicks || eval1.Multiplier != eval2.Multiplier {
		t.Fatalf("inconsistent evaluation results across calls: %+v != %+v", eval1, eval2)
	}
}

// TestGenerateSeededDeterminismAndReplay verifies that identical seeds and nonce reproduce
// the exact board, and varying nonce or seeds yields different boards.
func TestGenerateSeededDeterminismAndReplay(t *testing.T) {
	input := mines.SeededInput{
		ServerSeed: "server-secret-alpha",
		ClientSeed: "client-pub-beta",
		Nonce:      42,
		MineCount:  5,
	}

	b1, err := mines.GenerateSeeded(input)
	if err != nil {
		t.Fatalf("first GenerateSeeded failed: %v", err)
	}

	b2, err := mines.GenerateSeeded(input)
	if err != nil {
		t.Fatalf("replay GenerateSeeded failed: %v", err)
	}

	if !slices.Equal(b1.MineIndices, b2.MineIndices) {
		t.Fatalf("replay mismatch: %v != %v", b1.MineIndices, b2.MineIndices)
	}

	// Changing nonce separates boards
	inputNonce2 := input
	inputNonce2.Nonce = 43
	bNonce2, err := mines.GenerateSeeded(inputNonce2)
	if err != nil {
		t.Fatalf("GenerateSeeded with nonce 43 failed: %v", err)
	}
	if slices.Equal(b1.MineIndices, bNonce2.MineIndices) {
		t.Errorf("expected different boards for different nonces, got identical %v", b1.MineIndices)
	}

	// Changing server seed separates boards
	inputDiffServer := input
	inputDiffServer.ServerSeed = "server-secret-gamma"
	bDiffServer, err := mines.GenerateSeeded(inputDiffServer)
	if err != nil {
		t.Fatalf("GenerateSeeded with different server seed failed: %v", err)
	}
	if slices.Equal(b1.MineIndices, bDiffServer.MineIndices) {
		t.Errorf("expected different boards for different server seeds, got identical %v", b1.MineIndices)
	}

	// Changing client seed separates boards
	inputDiffClient := input
	inputDiffClient.ClientSeed = "client-pub-delta"
	bDiffClient, err := mines.GenerateSeeded(inputDiffClient)
	if err != nil {
		t.Fatalf("GenerateSeeded with different client seed failed: %v", err)
	}
	if slices.Equal(b1.MineIndices, bDiffClient.MineIndices) {
		t.Errorf("expected different boards for different client seeds, got identical %v", b1.MineIndices)
	}
}

// TestGenerateSeededDeterministicVector pins an exact compatibility vector for Mines replay.
func TestGenerateSeededDeterministicVector(t *testing.T) {
	input := mines.SeededInput{
		ServerSeed: "test-server-seed",
		ClientSeed: "test-client-seed",
		Nonce:      1,
		MineCount:  3,
	}

	b, err := mines.GenerateSeeded(input)
	if err != nil {
		t.Fatalf("GenerateSeeded failed: %v", err)
	}

	expectedMines := []int{7, 11, 12}
	if !slices.Equal(b.MineIndices, expectedMines) {
		t.Fatalf("MineIndices = %v, want %v", b.MineIndices, expectedMines)
	}

	// Evaluate safe pick then mine
	eval, err := b.Evaluate([]int{0, 7})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if !eval.Lost {
		t.Fatal("expected round to be lost at tile 7")
	}
	if eval.SafePicks != 1 {
		t.Fatalf("SafePicks = %d, want 1", eval.SafePicks)
	}
	if eval.Multiplier != 0.0 {
		t.Fatalf("Multiplier = %v, want 0.0", eval.Multiplier)
	}
	if len(eval.Reveals) != 2 {
		t.Fatalf("len(Reveals) = %d, want 2", len(eval.Reveals))
	}
	if eval.Reveals[0].Tile != 0 || eval.Reveals[0].Mine || eval.Reveals[0].SafePicks != 1 || math.Abs(eval.Reveals[0].Multiplier-1.125) > 1e-12 {
		t.Fatalf("unexpected reveal 0: %+v", eval.Reveals[0])
	}
	if eval.Reveals[1].Tile != 7 || !eval.Reveals[1].Mine || eval.Reveals[1].SafePicks != 1 || eval.Reveals[1].Multiplier != 0.0 {
		t.Fatalf("unexpected reveal 1: %+v", eval.Reveals[1])
	}
}

// TestMultipleSequentialRounds verifies that an engine can produce consecutive valid boards.
func TestMultipleSequentialRounds(t *testing.T) {
	source := entropy.NewProvablyFairSource("seq-server-seed", "seq-client-seed", 100)
	eng, err := mines.New(mines.Config{MineCount: 5}, source)
	if err != nil {
		t.Fatalf("unexpected New failure: %v", err)
	}

	seenBoards := make(map[string]bool)
	for round := 0; round < 20; round++ {
		b, err := eng.Play()
		if err != nil {
			t.Fatalf("round %d failed: %v", round, err)
		}
		if len(b.MineIndices) != 5 {
			t.Fatalf("round %d: got %d mines, want 5", round, len(b.MineIndices))
		}
		for i, m := range b.MineIndices {
			if m < 0 || m >= mines.TileCount {
				t.Fatalf("round %d: invalid tile %d", round, m)
			}
			if i > 0 && m <= b.MineIndices[i-1] {
				t.Fatalf("round %d: not strictly sorted: %v", round, b.MineIndices)
			}
		}
		boardKey := fmt.Sprintf("%v", b.MineIndices)
		seenBoards[boardKey] = true
	}

	if len(seenBoards) < 15 {
		t.Errorf("expected high board diversity across 20 rounds, got %d unique boards", len(seenBoards))
	}
}

// BenchmarkPlay benchmarks unseeded Board generation with a deterministic fake source.
func BenchmarkPlay(b *testing.B) {
	fake := &fakeEntropySource{
		decisions: make([]uint64, 1000000),
	}
	eng, err := mines.New(mines.Config{MineCount: 5}, fake)
	if err != nil {
		b.Fatalf("New failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if fake.index >= len(fake.decisions)-10 {
			fake.index = 0
		}
		_, _ = eng.Play()
	}
}

// BenchmarkGenerateSeeded benchmarks full provably-fair board generation from seeds.
func BenchmarkGenerateSeeded(b *testing.B) {
	input := mines.SeededInput{
		ServerSeed: "bench-server-seed",
		ClientSeed: "bench-client-seed",
		Nonce:      1,
		MineCount:  5,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		input.Nonce = uint64(i)
		_, _ = mines.GenerateSeeded(input)
	}
}

// BenchmarkEvaluate benchmarks evaluating an ordered sequence of picks.
func BenchmarkEvaluate(b *testing.B) {
	board := mines.Board{
		MineIndices: []int{20, 21, 22, 23, 24},
	}
	picks := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = board.Evaluate(picks)
	}
}
