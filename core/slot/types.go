// Package slot provides core types, deterministic reel sampling, evaluation,
// and state machine logic for modular slot games.
package slot

import (
	"encoding/json"
	"errors"
	"fmt"
)

var (
	// ErrInvalidGridDimensions indicates that a grid does not match the configured dimensions.
	ErrInvalidGridDimensions = errors.New("slot: invalid grid dimensions")
	// ErrEmptySymbol indicates that a grid position contains an empty symbol ID.
	ErrEmptySymbol = errors.New("slot: empty symbol ID in grid")
)

// SymbolID identifies a slot symbol in a game's catalog.
type SymbolID string

// Position identifies a reel and row cell coordinate in a slot grid.
type Position struct {
	Reel int `json:"reel"`
	Row  int `json:"row"`
}

// PlacedSymbol represents a symbol placed at a specific grid position.
type PlacedSymbol struct {
	Position Position `json:"position"`
	Symbol   SymbolID `json:"symbol"`
}

// Grid is a two-dimensional grid of symbols indexed by [reel][row].
type Grid [][]SymbolID

// Clone returns a deep copy of the grid.
func (g Grid) Clone() Grid {
	if g == nil {
		return nil
	}
	cp := make(Grid, len(g))
	for r := range g {
		if g[r] != nil {
			cp[r] = make([]SymbolID, len(g[r]))
			copy(cp[r], g[r])
		}
	}
	return cp
}

// Equal reports whether two grids contain identical dimensions and symbols.
func (g Grid) Equal(other Grid) bool {
	if len(g) != len(other) {
		return false
	}
	for r := range g {
		if len(g[r]) != len(other[r]) {
			return false
		}
		for row := range g[r] {
			if g[r][row] != other[r][row] {
				return false
			}
		}
	}
	return true
}

// Validate checks that the grid matches expected reel count and row bounds,
// and contains no empty symbol IDs.
func (g Grid) Validate(reels int, minRows, maxRows int) error {
	if len(g) != reels {
		return fmt.Errorf("%w: reel count %d, want %d", ErrInvalidGridDimensions, len(g), reels)
	}
	for r := range g {
		rowCount := len(g[r])
		if rowCount < minRows || rowCount > maxRows {
			return fmt.Errorf("%w: reel %d row count %d outside [%d, %d]", ErrInvalidGridDimensions, r, rowCount, minRows, maxRows)
		}
		for row, sym := range g[r] {
			if sym == "" {
				return fmt.Errorf("%w at reel %d row %d", ErrEmptySymbol, r, row)
			}
		}
	}
	return nil
}

// SymbolType classifies how a symbol behaves during evaluation.
type SymbolType string

const (
	// SymbolRegular denotes a standard paying symbol.
	SymbolRegular SymbolType = "regular"
	// SymbolWild denotes a wild symbol that can substitute for other symbols.
	SymbolWild SymbolType = "wild"
	// SymbolScatter denotes a scatter symbol evaluated across the full grid.
	SymbolScatter SymbolType = "scatter"
	// SymbolBonus denotes a bonus trigger symbol.
	SymbolBonus SymbolType = "bonus"
)

// WinMechanicType specifies how winning combinations are formed.
type WinMechanicType string

const (
	// WinMechanicWays pays for matching symbols on consecutive reels from left to right.
	WinMechanicWays WinMechanicType = "ways"
)

// TriggerKind indicates the kind of feature action awarded.
type TriggerKind string

const (
	// TriggerFreeSpins awards a number of free spins.
	TriggerFreeSpins TriggerKind = "free_spins"
	// TriggerRespin awards a respin round.
	TriggerRespin TriggerKind = "respin"
	// TriggerBonus awards a bonus game round.
	TriggerBonus TriggerKind = "bonus"
	// TriggerPick awards an interactive pick feature.
	TriggerPick TriggerKind = "pick"
	// TriggerJackpot awards a jackpot tier.
	TriggerJackpot TriggerKind = "jackpot"
	// TriggerCustom awards a custom game-specific feature.
	TriggerCustom TriggerKind = "custom"
)

// Trigger defines a feature action triggered by scatter or bonus evaluation.
type Trigger struct {
	Kind       TriggerKind     `json:"kind"`
	Count      int             `json:"count"`
	Award      int             `json:"award,omitempty"`
	Multiplier Multiplier      `json:"multiplier,omitempty"`
	Payload    json.RawMessage `json:"payload,omitempty"`
}

// Win represents a single winning combination.
type Win struct {
	Symbol           SymbolID   `json:"symbol"`
	MatchedReels     int        `json:"matched_reels"`
	CountPerReel     []int      `json:"count_per_reel"`
	Ways             int64      `json:"ways"`
	MultiplierPerWay Multiplier `json:"multiplier_per_way"`
	TotalMultiplier  Multiplier `json:"total_multiplier"`
	Positions        []Position `json:"positions"`
}

// Evaluation contains all wins, triggers, and multipliers for a single grid.
type Evaluation struct {
	Wins             []Win      `json:"wins"`
	ScatterCount     int        `json:"scatter_count,omitempty"`
	ScatterPositions []Position `json:"scatter_positions,omitempty"`
	ScatterPayout    Multiplier `json:"scatter_payout,omitempty"`
	Triggers         []Trigger  `json:"triggers,omitempty"`
	TotalMultiplier  Multiplier `json:"total_multiplier"`
}

// CascadeResult records one step of a cascade removal and refill sequence.
type CascadeResult struct {
	Index            int            `json:"index"`
	GridBefore       Grid           `json:"grid_before"`
	Wins             []Win          `json:"wins"`
	RemovedPositions []Position     `json:"removed_positions"`
	CollapsedGrid    Grid           `json:"collapsed_grid"`
	NewSymbols       []PlacedSymbol `json:"new_symbols"`
	FinalGrid        Grid           `json:"final_grid"`
	Multiplier       Multiplier     `json:"multiplier"`
	TotalMultiplier  Multiplier     `json:"total_multiplier"`
	Triggers         []Trigger      `json:"triggers,omitempty"`
}

// FreeSpinState tracks remaining and awarded spins, multipliers, and custom state.
type FreeSpinState struct {
	RemainingSpins   int             `json:"remaining_spins"`
	AwardedSpins     int             `json:"awarded_spins"`
	TotalSpins       int             `json:"total_spins"`
	AccumulatedWin   Multiplier      `json:"accumulated_win"`
	GlobalMultiplier Multiplier      `json:"global_multiplier"`
	FeatureState     json.RawMessage `json:"feature_state,omitempty"`
}

// SpinSession encapsulates an initial grid evaluation and subsequent cascades.
type SpinSession struct {
	InitialGrid       Grid            `json:"initial_grid"`
	InitialEvaluation Evaluation      `json:"initial_evaluation"`
	Cascades          []CascadeResult `json:"cascades,omitempty"`
	TotalMultiplier   Multiplier      `json:"total_multiplier"`
	Payout            Amount          `json:"payout"`
}

// PlayMode determines whether full presentation events are generated.
type PlayMode string

const (
	// PlayModeNormal generates all semantic events and presentation metadata.
	PlayModeNormal PlayMode = "normal"
	// PlayModeSimulation suppresses presentation events for high-throughput execution.
	PlayModeSimulation PlayMode = "simulation"
)

// SpinRequest contains inputs required to execute a slot spin.
type SpinRequest struct {
	GameID  string   `json:"game_id"`
	Version string   `json:"version"`
	Bet     Amount   `json:"bet"`
	Mode    PlayMode `json:"mode,omitempty"`
}

// SeededSpinRequest contains inputs required for deterministic, reproducible spin replay.
type SeededSpinRequest struct {
	SpinRequest
	ServerSeed string `json:"server_seed"`
	ClientSeed string `json:"client_seed"`
	Nonce      uint64 `json:"nonce"`
}

// State represents the current lifecycle state of a slot spin round.
type State string

const (
	// StateIdle indicates the engine is ready for a new spin.
	StateIdle State = "IDLE"
	// StateSpinning indicates reels are sampling stops and forming grids.
	StateSpinning State = "SPINNING"
	// StateEvaluating indicates grid wins and triggers are being evaluated.
	StateEvaluating State = "EVALUATING"
	// StateCascading indicates winning symbols are being removed and refilled.
	StateCascading State = "CASCADING"
	// StateFeatureTrigger indicates feature triggers are being processed.
	StateFeatureTrigger State = "FEATURE_TRIGGER"
	// StateFreeSpins indicates free spin rounds are executing.
	StateFreeSpins State = "FREE_SPINS"
	// StateSettling indicates final payout and profit are being calculated.
	StateSettling State = "SETTLING"
	// StateCompleted indicates the round has concluded cleanly.
	StateCompleted State = "COMPLETED"
)

// EventKind classifies semantic events emitted during normal play.
type EventKind string

const (
	// EventGridReveal is emitted when a newly generated grid is revealed.
	EventGridReveal EventKind = "grid_reveal"
	// EventWins is emitted when winning combinations are detected.
	EventWins EventKind = "wins"
	// EventRemoval is emitted when winning symbols are removed from the grid.
	EventRemoval EventKind = "removal"
	// EventCascade is emitted when refilled symbols drop into place.
	EventCascade EventKind = "cascade"
	// EventScatterTrigger is emitted when scatter conditions trigger features.
	EventScatterTrigger EventKind = "scatter_trigger"
	// EventFreeSpinTrigger is emitted when free spins are awarded or retriggered.
	EventFreeSpinTrigger EventKind = "free_spin_trigger"
	// EventMaxWinReached is emitted when payout reaches the configured cap.
	EventMaxWinReached EventKind = "max_win_reached"
	// EventComplete is emitted when the spin round completes settlement.
	EventComplete EventKind = "complete"
)

// Event represents a semantic domain event emitted during a spin round.
type Event struct {
	Kind        EventKind       `json:"kind"`
	Description string          `json:"description,omitempty"`
	Payload     json.RawMessage `json:"payload,omitempty"`
}

// GridRevealPayload describes the grid revealed in a spin or cascade.
type GridRevealPayload struct {
	Grid Grid `json:"grid"`
}

// WinsPayload describes the winning combinations detected on a grid.
type WinsPayload struct {
	Wins []Win `json:"wins"`
}

// RemovalPayload describes the winning positions removed from the grid.
type RemovalPayload struct {
	Positions []Position `json:"positions"`
}

// CascadePayload describes the refill step following symbol removal.
type CascadePayload struct {
	Index         int            `json:"index"`
	CollapsedGrid Grid           `json:"collapsed_grid"`
	NewSymbols    []PlacedSymbol `json:"new_symbols"`
	FinalGrid     Grid           `json:"final_grid"`
	Multiplier    Multiplier     `json:"multiplier"`
}

// ScatterTriggerPayload describes scatter symbols and resulting triggers/payout.
type ScatterTriggerPayload struct {
	Count     int        `json:"count"`
	Positions []Position `json:"positions"`
	Payout    Multiplier `json:"payout,omitempty"`
	Triggers  []Trigger  `json:"triggers,omitempty"`
}

// FreeSpinTriggerPayload describes awarded or retriggered free spins.
type FreeSpinTriggerPayload struct {
	AwardedSpins int  `json:"awarded_spins"`
	TotalSpins   int  `json:"total_spins"`
	IsRetrigger  bool `json:"is_retrigger"`
}

// MaxWinReachedPayload describes a payout cap event.
type MaxWinReachedPayload struct {
	CapMultiplier Multiplier   `json:"cap_multiplier"`
	Policy        MaxWinPolicy `json:"policy"`
}

// CompletePayload describes the final round outcome.
type CompletePayload struct {
	TotalMultiplier Multiplier `json:"total_multiplier"`
	Payout          Amount     `json:"payout"`
	NetProfit       Amount     `json:"net_profit"`
}

// EventPayload is implemented by compile-time typed event payloads.
type EventPayload interface {
	isEventPayload()
}

func (GridRevealPayload) isEventPayload()      {}
func (WinsPayload) isEventPayload()            {}
func (RemovalPayload) isEventPayload()         {}
func (CascadePayload) isEventPayload()         {}
func (ScatterTriggerPayload) isEventPayload()  {}
func (FreeSpinTriggerPayload) isEventPayload() {}
func (MaxWinReachedPayload) isEventPayload()   {}
func (CompletePayload) isEventPayload()        {}

// MarshalPayload serializes a compile-time typed event payload into json.RawMessage,
// returning an error if marshaling fails rather than swallowing errors.
func MarshalPayload(payload EventPayload) (json.RawMessage, error) {
	if payload == nil {
		return nil, nil
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("slot: marshal event payload: %w", err)
	}
	return json.RawMessage(b), nil
}

// SpinResult contains the full outcome, settlement, cascades, features, and events of a spin.
type SpinResult struct {
	GameID            string          `json:"game_id"`
	Version           string          `json:"version"`
	Bet               Amount          `json:"bet"`
	BaseMultiplier    Multiplier      `json:"base_multiplier"`
	FeatureMultiplier Multiplier      `json:"feature_multiplier"`
	TotalMultiplier   Multiplier      `json:"total_multiplier"`
	Payout            Amount          `json:"payout"`
	NetProfit         Amount          `json:"net_profit"`
	InitialGrid       Grid            `json:"initial_grid,omitempty"`
	Cascades          []CascadeResult `json:"cascades,omitempty"`
	FreeSpins         []SpinSession   `json:"free_spins,omitempty"`
	FreeSpinState     *FreeSpinState  `json:"free_spin_state,omitempty"`
	Triggers          []Trigger       `json:"triggers,omitempty"`
	TriggeredFeatures []string        `json:"triggered_features,omitempty"`
	CascadeCount      int             `json:"cascade_count"`
	FreeSpinCount     int             `json:"free_spin_count"`
	TerminalState     State           `json:"terminal_state"`
	Events            []Event         `json:"events,omitempty"`
}
