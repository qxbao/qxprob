package slot

import (
	"errors"
	"fmt"
	"math"
)

var (
	// ErrNilEvaluator indicates a nil Evaluator was supplied.
	ErrNilEvaluator = errors.New("slot: evaluator must not be nil")
)

// Evaluator defines the interface for evaluating wins and feature triggers on a slot grid.
type Evaluator interface {
	Evaluate(def *ValidatedDefinition, grid Grid) (Evaluation, error)
}

// WaysEvaluator evaluates left-to-right ways wins, wild substitutions, and scatter triggers.
type WaysEvaluator struct{}

// Evaluate inspects the grid, counts symbol matches from reel 0, calculates ways products,
// evaluates wild substitutions, and processes scatter thresholds according to def.
func (e WaysEvaluator) Evaluate(def *ValidatedDefinition, grid Grid) (Evaluation, error) {
	if def == nil {
		return Evaluation{}, ErrNilDefinition
	}
	if grid == nil || len(grid) != def.def.Grid.Reels {
		return Evaluation{}, ErrInvalidGridDimensions
	}
	for r := range grid {
		rowCount := len(grid[r])
		if rowCount < def.minRows || rowCount > def.maxRows {
			return Evaluation{}, fmt.Errorf("%w: reel %d row count %d outside [%d, %d]", ErrInvalidGridDimensions, r, rowCount, def.minRows, def.maxRows)
		}
	}

	var wins []Win

	// 1. Regular symbol ways evaluation
	for _, symConfig := range def.def.Symbols {
		if symConfig.Type != SymbolRegular {
			continue
		}
		sid := symConfig.ID
		matchedReels := 0
		numReels := def.def.Grid.Reels
		countPerReel := make([]int, numReels)

		for r := 0; r < numReels; r++ {
			for row := 0; row < len(grid[r]); row++ {
				cellSym := grid[r][row]
				if cellSym == sid || (def.IsWild(cellSym) && def.CanWildSubstitute(cellSym, sid)) {
					countPerReel[r]++
				}
			}
			if countPerReel[r] == 0 {
				break
			}
			matchedReels++
		}

		if matchedReels >= def.def.WinMechanic.MinMatch && matchedReels < len(symConfig.Payouts) {
			payout := symConfig.Payouts[matchedReels]
			if payout > 0 {
				ways := int64(1)
				for r := 0; r < matchedReels; r++ {
					cnt := int64(countPerReel[r])
					if cnt > 0 && ways > math.MaxInt64/cnt {
						return Evaluation{}, ErrMultiplierOverflow
					}
					ways *= cnt
				}

				totMult, err := payout.MulInt(ways)
				if err != nil {
					return Evaluation{}, err
				}

				var winPositions []Position
				for r := 0; r < matchedReels; r++ {
					for row := 0; row < len(grid[r]); row++ {
						cellSym := grid[r][row]
						if cellSym == sid || (def.IsWild(cellSym) && def.CanWildSubstitute(cellSym, sid)) {
							winPositions = append(winPositions, Position{Reel: r, Row: row})
						}
					}
				}

				matchedCounts := make([]int, matchedReels)
				copy(matchedCounts, countPerReel[:matchedReels])

				wins = append(wins, Win{
					Symbol:           sid,
					MatchedReels:     matchedReels,
					CountPerReel:     matchedCounts,
					Ways:             ways,
					MultiplierPerWay: payout,
					TotalMultiplier:  totMult,
					Positions:        winPositions,
				})
			}
		}
	}

	// 2. Wild explicit payout evaluation (only if payouts are defined)
	for _, symConfig := range def.def.Symbols {
		if symConfig.Type != SymbolWild || len(symConfig.Payouts) == 0 {
			continue
		}
		wid := symConfig.ID
		matchedReels := 0
		numReels := def.def.Grid.Reels
		countPerReel := make([]int, numReels)

		for r := 0; r < numReels; r++ {
			for row := 0; row < len(grid[r]); row++ {
				cellSym := grid[r][row]
				if cellSym == wid {
					countPerReel[r]++
				}
			}
			if countPerReel[r] == 0 {
				break
			}
			matchedReels++
		}

		if matchedReels >= def.def.WinMechanic.MinMatch && matchedReels < len(symConfig.Payouts) {
			payout := symConfig.Payouts[matchedReels]
			if payout > 0 {
				ways := int64(1)
				for r := 0; r < matchedReels; r++ {
					cnt := int64(countPerReel[r])
					if cnt > 0 && ways > math.MaxInt64/cnt {
						return Evaluation{}, ErrMultiplierOverflow
					}
					ways *= cnt
				}

				totMult, err := payout.MulInt(ways)
				if err != nil {
					return Evaluation{}, err
				}

				var winPositions []Position
				for r := 0; r < matchedReels; r++ {
					for row := 0; row < len(grid[r]); row++ {
						cellSym := grid[r][row]
						if cellSym == wid {
							winPositions = append(winPositions, Position{Reel: r, Row: row})
						}
					}
				}

				matchedCounts := make([]int, matchedReels)
				copy(matchedCounts, countPerReel[:matchedReels])

				wins = append(wins, Win{
					Symbol:           wid,
					MatchedReels:     matchedReels,
					CountPerReel:     matchedCounts,
					Ways:             ways,
					MultiplierPerWay: payout,
					TotalMultiplier:  totMult,
					Positions:        winPositions,
				})
			}
		}
	}

	// 3. Scatter evaluation across full grid
	var scatterPositions []Position
	for r := 0; r < len(grid); r++ {
		for row := 0; row < len(grid[r]); row++ {
			sym := grid[r][row]
			if (def.def.Scatter.SymbolID != "" && sym == def.def.Scatter.SymbolID) || (def.def.Scatter.SymbolID == "" && def.IsScatter(sym)) {
				scatterPositions = append(scatterPositions, Position{Reel: r, Row: row})
			}
		}
	}

	scatterCount := len(scatterPositions)
	var scatterPayout Multiplier
	var triggers []Trigger

	var bestThreshold *ScatterTrigger
	for i := range def.def.Scatter.Thresholds {
		th := &def.def.Scatter.Thresholds[i]
		if scatterCount >= th.Count {
			if bestThreshold == nil || th.Count > bestThreshold.Count {
				bestThreshold = th
			}
		}
	}

	if bestThreshold != nil {
		scatterPayout = bestThreshold.Payout
		for _, tr := range bestThreshold.Triggers {
			if tr.Count == 0 {
				tr.Count = bestThreshold.Count
			}
			triggers = append(triggers, tr)
		}
	}

	// 4. Calculate total multiplier
	var totalMult Multiplier
	for _, w := range wins {
		var err error
		totalMult, err = totalMult.Add(w.TotalMultiplier)
		if err != nil {
			return Evaluation{}, err
		}
	}
	if scatterPayout > 0 {
		var err error
		totalMult, err = totalMult.Add(scatterPayout)
		if err != nil {
			return Evaluation{}, err
		}
	}

	return Evaluation{
		Wins:             wins,
		ScatterCount:     scatterCount,
		ScatterPositions: scatterPositions,
		ScatterPayout:    scatterPayout,
		Triggers:         triggers,
		TotalMultiplier:  totalMult,
	}, nil
}
