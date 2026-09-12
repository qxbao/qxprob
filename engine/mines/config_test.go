package mines_test

import (
	"errors"
	"math"
	"testing"

	"github.com/qxbao/qxprob/engine/mines"
)

func TestCustomBoardSizeAndTargetRTP(t *testing.T) {
	fake := &fakeEntropySource{decisions: []uint64{0, 0, 0}}
	engine, err := mines.New(mines.Config{BoardSize: 10, MineCount: 3, TargetRTP: 0.95}, fake)
	if err != nil {
		t.Fatal(err)
	}
	board, err := engine.Play()
	if err != nil {
		t.Fatal(err)
	}
	if board.TileCount != 10 || board.TargetRTP != 0.95 || len(board.MineIndices) != 3 {
		t.Fatalf("unexpected board: %+v", board)
	}
	wantBounds := []uint64{10, 9, 8}
	for i, want := range wantBounds {
		if fake.bounds[i] != want {
			t.Fatalf("bound %d = %d, want %d", i, fake.bounds[i], want)
		}
	}
}

func TestCustomBoardEvaluationMultiplier(t *testing.T) {
	board := mines.Board{TileCount: 10, TargetRTP: 0.95, MineIndices: []int{9}}
	result, err := board.Evaluate([]int{0})
	if err != nil {
		t.Fatal(err)
	}
	want := 0.95 / 0.9
	if math.Abs(result.Multiplier-want) > 1e-12 {
		t.Fatalf("Multiplier = %v, want %v", result.Multiplier, want)
	}
}

func TestCustomMinesConfigValidation(t *testing.T) {
	tests := []struct {
		config  mines.Config
		wantErr error
	}{
		{mines.Config{BoardSize: 1, MineCount: 1}, mines.ErrInvalidBoardSize},
		{mines.Config{BoardSize: 10, MineCount: 10}, mines.ErrInvalidMineCount},
		{mines.Config{BoardSize: 10, MineCount: 2, TargetRTP: math.NaN()}, mines.ErrInvalidTargetRTP},
	}
	for _, tc := range tests {
		_, err := mines.New(tc.config, &fakeEntropySource{})
		if !errors.Is(err, tc.wantErr) {
			t.Fatalf("New(%+v) error = %v, want %v", tc.config, err, tc.wantErr)
		}
	}
}

func TestGenerateSeededForwardsCustomConfig(t *testing.T) {
	board, err := mines.GenerateSeeded(mines.SeededInput{
		ServerSeed: "server", ClientSeed: "client", Nonce: 1,
		BoardSize: 12, MineCount: 4, TargetRTP: 0.92,
	})
	if err != nil {
		t.Fatal(err)
	}
	if board.TileCount != 12 || board.TargetRTP != 0.92 || len(board.MineIndices) != 4 {
		t.Fatalf("unexpected board: %+v", board)
	}
}
