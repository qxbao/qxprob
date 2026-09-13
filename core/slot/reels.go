package slot

import (
	"errors"
	"fmt"
	"math"
)

var (
	// ErrNilRandomSource indicates that a nil RandomSource was provided.
	ErrNilRandomSource = errors.New("slot: random source must not be nil")
	// ErrNilDefinition indicates that a nil ValidatedDefinition was provided.
	ErrNilDefinition = errors.New("slot: definition must not be nil")
	// ErrEmptyWeights indicates that a weights slice was empty.
	ErrEmptyWeights = errors.New("slot: weights slice must not be empty")
	// ErrZeroTotalWeight indicates that the sum of weights is zero.
	ErrZeroTotalWeight = errors.New("slot: total weight must be greater than zero")
	// ErrPositionOutOfBounds indicates a position is outside the grid bounds.
	ErrPositionOutOfBounds = errors.New("slot: position out of grid bounds")
)

// RandomSource supplies unbiased bounded pseudo-random values.
// Existing entropy.Source satisfies this interface.
type RandomSource interface {
	Uint64() (uint64, error)
	Float64() (float64, error)
	Intn(uint64) (uint64, error)
}

// GeneratedGrid holds the sampled grid, reel stop positions, and visible row counts.
type GeneratedGrid struct {
	Grid  Grid
	Stops []int
	Rows  []int
}

// GenerateGrid samples reel stops left-to-right (reels 0..N-1) according to protocol v1.
// For variable rows, it samples row count first, then stop position for each reel.
func GenerateGrid(def *ValidatedDefinition, reelSetName string, src RandomSource) (GeneratedGrid, error) {
	if def == nil {
		return GeneratedGrid{}, ErrNilDefinition
	}
	if src == nil {
		return GeneratedGrid{}, ErrNilRandomSource
	}

	rs, ok := def.ReelSet(reelSetName)
	if !ok {
		return GeneratedGrid{}, fmt.Errorf("%w: %q", ErrMissingReelSet, reelSetName)
	}

	numReels := def.def.Grid.Reels
	grid := make(Grid, numReels)
	stops := make([]int, numReels)
	rows := make([]int, numReels)

	for r := 0; r < numReels; r++ {
		// 1. Variable row sample if configured
		if def.def.Grid.VariableRows {
			span := uint64(def.def.Grid.MaxRows - def.def.Grid.MinRows + 1)
			rowSample, err := src.Intn(span)
			if err != nil {
				return GeneratedGrid{}, fmt.Errorf("slot: failed to sample row count for reel %d: %w", r, err)
			}
			rows[r] = def.def.Grid.MinRows + int(rowSample)
		} else {
			rows[r] = def.def.Grid.Rows
		}

		// 2. Stop sample
		strip := rs.Reels[r]
		stripLen := len(strip)
		stopSample, err := src.Intn(uint64(stripLen))
		if err != nil {
			return GeneratedGrid{}, fmt.Errorf("slot: failed to sample stop for reel %d: %w", r, err)
		}
		stops[r] = int(stopSample)

		// 3. Populate visible symbols with wrap-around
		grid[r] = make([]SymbolID, rows[r])
		for row := 0; row < rows[r]; row++ {
			grid[r][row] = strip[(stops[r]+row)%stripLen]
		}
	}

	return GeneratedGrid{
		Grid:  grid,
		Stops: stops,
		Rows:  rows,
	}, nil
}

// CollapseAndRefill removes winning positions, collapses surviving symbols downward,
// and refills missing cells bottom-to-top within reels left-to-right according to protocol v1.
func CollapseAndRefill(def *ValidatedDefinition, reelSetName string, grid Grid, removed []Position, src RandomSource) (collapsed Grid, added []PlacedSymbol, final Grid, err error) {
	if def == nil {
		return nil, nil, nil, ErrNilDefinition
	}
	if src == nil {
		return nil, nil, nil, ErrNilRandomSource
	}

	rs, ok := def.ReelSet(reelSetName)
	if !ok {
		return nil, nil, nil, fmt.Errorf("%w: %q", ErrMissingReelSet, reelSetName)
	}

	numReels := len(grid)
	if len(removed) == 0 {
		return grid.Clone(), nil, grid.Clone(), nil
	}

	// Validate and deduplicate removed positions
	isRemoved := make([][]bool, numReels)
	for r := 0; r < numReels; r++ {
		isRemoved[r] = make([]bool, len(grid[r]))
	}
	for _, pos := range removed {
		if pos.Reel < 0 || pos.Reel >= numReels || pos.Row < 0 || pos.Row >= len(grid[pos.Reel]) {
			return nil, nil, nil, fmt.Errorf("%w: reel %d row %d", ErrPositionOutOfBounds, pos.Reel, pos.Row)
		}
		isRemoved[pos.Reel][pos.Row] = true
	}

	collapsed = make(Grid, numReels)
	final = make(Grid, numReels)
	emptyCounts := make([]int, numReels)

	// Step 1: Collapse survivors downward for each reel
	for r := 0; r < numReels; r++ {
		R := len(grid[r])
		collapsed[r] = make([]SymbolID, R)
		final[r] = make([]SymbolID, R)

		var survivors []SymbolID
		for row := 0; row < R; row++ {
			if !isRemoved[r][row] {
				survivors = append(survivors, grid[r][row])
			}
		}

		S := len(survivors)
		emptyCount := R - S
		emptyCounts[r] = emptyCount

		// Place survivors at bottom
		for i := 0; i < S; i++ {
			targetRow := emptyCount + i
			collapsed[r][targetRow] = survivors[i]
			final[r][targetRow] = survivors[i]
		}
	}

	// Step 2: Refill missing cells bottom-to-top within reels left-to-right
	for r := 0; r < numReels; r++ {
		emptyCount := emptyCounts[r]
		if emptyCount == 0 {
			continue
		}
		strip := rs.Reels[r]
		stripLen := uint64(len(strip))

		// Refill from bottommost empty cell up to row 0
		for row := emptyCount - 1; row >= 0; row-- {
			sample, err := src.Intn(stripLen)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("slot: failed to sample refill symbol at reel %d row %d: %w", r, row, err)
			}
			sym := strip[sample]
			final[r][row] = sym
			added = append(added, PlacedSymbol{
				Position: Position{Reel: r, Row: row},
				Symbol:   sym,
			})
		}
	}

	return collapsed, added, final, nil
}

// ChooseWeighted selects an index from weights proportional to its weight without bias.
func ChooseWeighted(src RandomSource, weights []uint64) (int, error) {
	if src == nil {
		return -1, ErrNilRandomSource
	}
	if len(weights) == 0 {
		return -1, ErrEmptyWeights
	}

	var total uint64
	for _, w := range weights {
		if w > math.MaxUint64-total {
			return -1, errors.New("slot: weights sum overflow")
		}
		total += w
	}
	if total == 0 {
		return -1, ErrZeroTotalWeight
	}

	val, err := src.Intn(total)
	if err != nil {
		return -1, err
	}

	var cumulative uint64
	for i, w := range weights {
		cumulative += w
		if val < cumulative {
			return i, nil
		}
	}

	return len(weights) - 1, nil
}
