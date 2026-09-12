// Package mines implements a configurable provably-fair Mines game engine driven by an entropy source.
package mines

import (
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/qxbao/qxprob/core/entropy"
)

const (
	// TileCount is the default number of tiles, retained for compatibility.
	TileCount = 25
	// DefaultBoardSize is the default number of tiles on a board.
	DefaultBoardSize = TileCount
	// MaxBoardSize caps per-round memory use while allowing custom board layouts.
	MaxBoardSize = 1024

	// MinMines is the minimum number of mines allowed on the board.
	MinMines = 1

	// MaxMines is the maximum mine count for the default 25-tile board, retained for compatibility.
	MaxMines = 24

	// DefaultTargetRTP is the default theoretical return-to-player ratio.
	DefaultTargetRTP = 0.99
)

var (
	// ErrInvalidMineCount indicates that MineCount is outside [1, BoardSize).
	ErrInvalidMineCount = errors.New("mines: mine count must be between 1 and BoardSize-1")

	// ErrNilSource indicates that a nil entropy source was provided.
	ErrNilSource = errors.New("mines: entropy source must not be nil")

	// ErrEmptyServerSeed indicates that an empty server seed was provided.
	ErrEmptyServerSeed = errors.New("mines: server seed must not be empty")

	// ErrEmptyClientSeed indicates that an empty client seed was provided.
	ErrEmptyClientSeed = errors.New("mines: client seed must not be empty")

	// ErrInvalidBoard indicates that the board configuration or mine placement is invalid.
	ErrInvalidBoard = errors.New("mines: board configuration is invalid")

	// ErrInvalidTile indicates that a pick is outside the configured board range.
	ErrInvalidTile = errors.New("mines: tile index out of board range")

	// ErrDuplicatePick indicates that a tile was picked more than once.
	ErrDuplicatePick = errors.New("mines: duplicate tile pick")

	// ErrInvalidBoardSize indicates that BoardSize is outside [2, MaxBoardSize].
	ErrInvalidBoardSize = errors.New("mines: board size must be between 2 and MaxBoardSize")

	// ErrInvalidTargetRTP indicates that TargetRTP is not finite or outside (0, 1].
	ErrInvalidTargetRTP = errors.New("mines: target RTP must be a finite value in (0, 1]")
)

// Config defines configuration parameters for the Mines engine.
type Config struct {
	// BoardSize is the number of tiles. Zero uses DefaultBoardSize.
	BoardSize int
	// MineCount is the number of mines on the board in [1, BoardSize).
	MineCount int
	// TargetRTP controls cash-out payouts. Zero uses DefaultTargetRTP.
	TargetRTP float64
}

// SeededInput defines inputs required to generate a deterministic, reproducible Mines board.
type SeededInput struct {
	// ServerSeed is the secret seed controlled by the game operator.
	ServerSeed string
	// ClientSeed is the public seed provided by or visible to the player.
	ClientSeed string
	// Nonce is the round counter that prevents replay across rounds.
	Nonce uint64
	// MineCount is the number of mines on the board in [1, BoardSize).
	MineCount int
	// BoardSize is the number of tiles. Zero uses DefaultBoardSize.
	BoardSize int
	// TargetRTP controls cash-out payouts. Zero uses DefaultTargetRTP.
	TargetRTP float64
}

// Board represents the immutable state and payout policy of a Mines board.
type Board struct {
	// TileCount is the number of tiles. Zero is interpreted as DefaultBoardSize for compatibility.
	TileCount int
	// TargetRTP is the payout ratio. Zero is interpreted as DefaultTargetRTP for compatibility.
	TargetRTP float64
	// MineIndices contains the sorted indices of all mines on the board.
	MineIndices []int
}

// Reveal represents the outcome of revealing a single tile.
type Reveal struct {
	// Tile is the index of the revealed tile.
	Tile int
	// Mine indicates whether the revealed tile contained a mine.
	Mine bool
	// SafePicks is the cumulative count of safe picks made up to and including this reveal.
	SafePicks int
	// Multiplier is the payout multiplier after this reveal (0.0 if a mine was hit).
	Multiplier float64
}

// Evaluation represents the complete outcome of evaluating an ordered sequence of picks.
type Evaluation struct {
	// Reveals contains the step-by-step reveal outcomes up to the first mine or end of picks.
	Reveals []Reveal
	// Lost indicates whether the round ended in a loss by hitting a mine.
	Lost bool
	// SafePicks is the total number of safe tiles successfully revealed.
	SafePicks int
	// Multiplier is the final cash-out multiplier (0.0 if lost, 1.0 if zero safe picks, or fair multiplier).
	Multiplier float64
}

// Engine generates rounds of Mines using an injected entropy source.
type Engine struct {
	config Config
	source entropy.Source
}

// New constructs a validated Mines Engine. It rejects nil sources or out-of-range
// configuration parameters without consuming entropy.
func New(config Config, source entropy.Source) (*Engine, error) {
	if source == nil {
		return nil, ErrNilSource
	}
	resolved, err := resolveConfig(config)
	if err != nil {
		return nil, err
	}
	return &Engine{
		config: resolved,
		source: source,
	}, nil
}

func resolveConfig(config Config) (Config, error) {
	if config.BoardSize == 0 {
		config.BoardSize = DefaultBoardSize
	}
	if config.TargetRTP == 0 {
		config.TargetRTP = DefaultTargetRTP
	}
	if config.BoardSize < 2 || config.BoardSize > MaxBoardSize {
		return Config{}, ErrInvalidBoardSize
	}
	if config.MineCount < MinMines || config.MineCount >= config.BoardSize {
		return Config{}, ErrInvalidMineCount
	}
	if math.IsNaN(config.TargetRTP) || math.IsInf(config.TargetRTP, 0) || config.TargetRTP <= 0 || config.TargetRTP > 1 {
		return Config{}, ErrInvalidTargetRTP
	}
	return config, nil
}

// Play executes one round of the Mines game, returning a generated Board. It consumes
// exactly MineCount Intn(remaining) calls from the entropy source using a partial
// Fisher-Yates shuffle, and returns sorted MineIndices for canonical replay output.
func (e *Engine) Play() (Board, error) {
	tiles := make([]int, e.config.BoardSize)
	for i := range tiles {
		tiles[i] = i
	}

	mineIndices := make([]int, e.config.MineCount)
	for i := 0; i < e.config.MineCount; i++ {
		remaining := uint64(e.config.BoardSize - i)
		idx, err := e.source.Intn(remaining)
		if err != nil {
			return Board{}, fmt.Errorf("mines: failed to sample mine index: %w", err)
		}
		swapIdx := i + int(idx)
		tiles[i], tiles[swapIdx] = tiles[swapIdx], tiles[i]
		mineIndices[i] = tiles[i]
	}

	slices.Sort(mineIndices)
	return Board{TileCount: e.config.BoardSize, TargetRTP: e.config.TargetRTP, MineIndices: mineIndices}, nil
}

// GenerateSeeded generates a deterministic, reproducible Mines board derived from
// server seed, client seed, and nonce. It validates inputs before constructing
// an entropy source or consuming entropy.
func GenerateSeeded(input SeededInput) (Board, error) {
	if input.ServerSeed == "" {
		return Board{}, ErrEmptyServerSeed
	}
	if input.ClientSeed == "" {
		return Board{}, ErrEmptyClientSeed
	}
	source := entropy.NewProvablyFairSource(input.ServerSeed, input.ClientSeed, input.Nonce)
	engine, err := New(Config{
		BoardSize: input.BoardSize,
		MineCount: input.MineCount,
		TargetRTP: input.TargetRTP,
	}, source)
	if err != nil {
		return Board{}, err
	}

	return engine.Play()
}

// validateBoard verifies board size, payout ratio, mine count, indices,
// and no duplicate mine indices.
func validateBoard(b Board) ([]bool, int, float64, error) {
	config, err := resolveConfig(Config{BoardSize: b.TileCount, MineCount: len(b.MineIndices), TargetRTP: b.TargetRTP})
	if err != nil {
		return nil, 0, 0, ErrInvalidBoard
	}
	isMine := make([]bool, config.BoardSize)
	for _, idx := range b.MineIndices {
		if idx < 0 || idx >= config.BoardSize || isMine[idx] {
			return nil, 0, 0, ErrInvalidBoard
		}
		isMine[idx] = true
	}
	return isMine, config.BoardSize, config.TargetRTP, nil
}

// validatePicks verifies that all picks are in range and contain no duplicates.
func validatePicks(picks []int, boardSize int) error {
	seen := make([]bool, boardSize)
	for _, tile := range picks {
		if tile < 0 || tile >= boardSize {
			return ErrInvalidTile
		}
		if seen[tile] {
			return ErrDuplicatePick
		}
		seen[tile] = true
	}
	return nil
}

// Evaluate evaluates an ordered sequence of tile picks against the board.
// It first validates board integrity and all picks (checking range and duplicates),
// then evaluates reveals sequentially without mutating the board. Evaluation stops
// at the first mine. If no safe picks are made and the round is not lost, the multiplier
// is 1.0; a lost round returns a multiplier of 0.0.
func (b Board) Evaluate(picks []int) (Evaluation, error) {
	isMine, boardSize, targetRTP, err := validateBoard(b)
	if err != nil {
		return Evaluation{}, err
	}

	if err := validatePicks(picks, boardSize); err != nil {
		return Evaluation{}, err
	}

	mineCount := len(b.MineIndices)
	reveals := make([]Reveal, 0, len(picks))
	safePicks := 0
	lost := false
	currentMultiplier := 1.0

	for _, tile := range picks {
		if isMine[tile] {
			lost = true
			currentMultiplier = 0.0
			reveals = append(reveals, Reveal{
				Tile:       tile,
				Mine:       true,
				SafePicks:  safePicks,
				Multiplier: 0.0,
			})
			break
		}

		safePicks++
		currentMultiplier = cashOutMultiplier(boardSize, mineCount, safePicks, targetRTP)
		reveals = append(reveals, Reveal{
			Tile:       tile,
			Mine:       false,
			SafePicks:  safePicks,
			Multiplier: currentMultiplier,
		})
	}

	return Evaluation{
		Reveals:    reveals,
		Lost:       lost,
		SafePicks:  safePicks,
		Multiplier: currentMultiplier,
	}, nil
}

// cashOutMultiplier computes the configured cash-out multiplier after safe picks.
func cashOutMultiplier(boardSize, mineCount, safePicks int, targetRTP float64) float64 {
	if safePicks <= 0 {
		return 1.0
	}
	safeTiles := boardSize - mineCount
	if safePicks > safeTiles {
		return 0.0
	}
	probability := 1.0
	for i := 0; i < safePicks; i++ {
		probability *= float64(safeTiles-i) / float64(boardSize-i)
	}
	return targetRTP / probability
}
