package slot

import (
	"encoding/json"
	"errors"
	"fmt"
)

var (
	// ErrInvalidDefinition indicates that the definition violates required constraints.
	ErrInvalidDefinition = errors.New("slot: invalid definition")
	// ErrDuplicateSymbol indicates a duplicate symbol ID in the catalog.
	ErrDuplicateSymbol = errors.New("slot: duplicate symbol ID")
	// ErrUnknownSymbol indicates a symbol ID was referenced but not declared in the catalog.
	ErrUnknownSymbol = errors.New("slot: unknown symbol ID")
	// ErrInvalidPayoutLength indicates a paytable length does not match grid dimensions.
	ErrInvalidPayoutLength = errors.New("slot: invalid payout length")
	// ErrMissingReelSet indicates a referenced reel set is not found.
	ErrMissingReelSet = errors.New("slot: missing reel set")
	// ErrMalformedReelSet indicates a reel set has mismatched reel counts or empty strips.
	ErrMalformedReelSet = errors.New("slot: malformed reel set")
	// ErrIllegalWildTarget indicates wild substitution target is illegal or not allowed.
	ErrIllegalWildTarget = errors.New("slot: illegal wild substitution target")
	// ErrInvalidRowRange indicates an invalid row range configuration.
	ErrInvalidRowRange = errors.New("slot: invalid row range")
	// ErrInvalidFeatureRef indicates a feature configuration references a missing asset or reel set.
	ErrInvalidFeatureRef = errors.New("slot: invalid feature reference")
	// ErrIncoherentCap indicates an invalid or negative max win cap.
	ErrIncoherentCap = errors.New("slot: incoherent max win cap")
	// ErrDuplicateFeatureID indicates a feature ID is declared more than once.
	ErrDuplicateFeatureID = errors.New("slot: duplicate feature ID")
)

// GridConfig specifies the physical dimensions of the slot grid.
type GridConfig struct {
	Reels        int  `json:"reels"`
	Rows         int  `json:"rows,omitempty"`
	MinRows      int  `json:"min_rows,omitempty"`
	MaxRows      int  `json:"max_rows,omitempty"`
	VariableRows bool `json:"variable_rows,omitempty"`
}

// WinMechanicConfig specifies how wins are formed and the minimum match length.
type WinMechanicConfig struct {
	Type     WinMechanicType `json:"type"`
	MinMatch int             `json:"min_match"`
}

// SymbolConfig defines identity, type, payouts, metadata, and substitution rules for a single symbol.
type SymbolConfig struct {
	ID                  SymbolID        `json:"id"`
	Name                string          `json:"name,omitempty"`
	Type                SymbolType      `json:"type"`
	Payouts             []Multiplier    `json:"payouts,omitempty"`
	Metadata            json.RawMessage `json:"metadata,omitempty"`
	SubstituteFor       []SymbolID      `json:"substitute_for,omitempty"`
	CannotSubstituteFor []SymbolID      `json:"cannot_substitute_for,omitempty"`
}

// ReelSet represents a complete set of reel strips for a grid.
type ReelSet struct {
	Reels [][]SymbolID `json:"reels"`
}

// Clone returns a deep copy of the reel set.
func (rs ReelSet) Clone() ReelSet {
	if rs.Reels == nil {
		return ReelSet{}
	}
	cp := make([][]SymbolID, len(rs.Reels))
	for r := range rs.Reels {
		if rs.Reels[r] != nil {
			cp[r] = make([]SymbolID, len(rs.Reels[r]))
			copy(cp[r], rs.Reels[r])
		}
	}
	return ReelSet{Reels: cp}
}

// WildConfig governs wild symbol substitution rules.
type WildConfig struct {
	WildSymbolIDs       []SymbolID `json:"wild_symbols,omitempty"`
	SubstituteFor       []SymbolID `json:"substitute_for,omitempty"`
	CannotSubstituteFor []SymbolID `json:"cannot_substitute_for,omitempty"`
}

// ScatterTrigger specifies payout and triggers awarded for a scatter count threshold.
type ScatterTrigger struct {
	Count    int        `json:"count"`
	Payout   Multiplier `json:"payout,omitempty"`
	Triggers []Trigger  `json:"triggers,omitempty"`
}

// ScatterConfig configures scatter symbol identification and threshold payouts/triggers.
type ScatterConfig struct {
	SymbolID   SymbolID         `json:"symbol_id,omitempty"`
	Thresholds []ScatterTrigger `json:"thresholds,omitempty"`
}

// CascadeMultiplierMode defines how cascade win multipliers scale with consecutive cascades.
type CascadeMultiplierMode string

const (
	// CascadeMultiplierFixed applies a fixed multiplier to all cascades.
	CascadeMultiplierFixed CascadeMultiplierMode = "fixed"
	// CascadeMultiplierIncremental increments the multiplier per cascade by a step.
	CascadeMultiplierIncremental CascadeMultiplierMode = "incremental"
	// CascadeMultiplierTable looks up the multiplier from a defined progression table.
	CascadeMultiplierTable CascadeMultiplierMode = "table"
)

// CascadeConfig controls symbol removal, collapse, refill, and multipliers.
type CascadeConfig struct {
	Enabled         bool                  `json:"enabled"`
	MaxCascades     int                   `json:"max_cascades,omitempty"`
	MultiplierMode  CascadeMultiplierMode `json:"multiplier_mode,omitempty"`
	FixedMultiplier Multiplier            `json:"fixed_multiplier,omitempty"`
	MultiplierStep  Multiplier            `json:"multiplier_step,omitempty"`
	MultiplierTable []Multiplier          `json:"multiplier_table,omitempty"`
}

// MultiplierPolicy governs how multipliers behave across free spins.
type MultiplierPolicy string

const (
	// MultiplierResetEachSpin resets the cascade/win multiplier after each spin.
	MultiplierResetEachSpin MultiplierPolicy = "reset_each_spin"
	// MultiplierPersistent preserves accumulated multipliers across free spins.
	MultiplierPersistent MultiplierPolicy = "persistent"
	// MultiplierCustom delegates multiplier handling to a registered feature module.
	MultiplierCustom MultiplierPolicy = "custom"
)

// FreeSpinsConfig configures the free spin feature mode.
type FreeSpinsConfig struct {
	Enabled          bool             `json:"enabled"`
	ReelSet          string           `json:"reel_set,omitempty"`
	MaxTotalSpins    int              `json:"max_total_spins,omitempty"`
	MultiplierPolicy MultiplierPolicy `json:"multiplier_policy,omitempty"`
	RetriggerEnabled bool             `json:"retrigger_enabled"`
}

// FeatureConfig declares an enabled or optional game feature module.
type FeatureConfig struct {
	ID      string          `json:"id"`
	Enabled bool            `json:"enabled"`
	Config  json.RawMessage `json:"config,omitempty"`
}

// MaxWinPolicy determines engine behavior when payout reaches the maximum cap.
type MaxWinPolicy string

const (
	// MaxWinCapOnly caps the payout at the max win without early termination.
	MaxWinCapOnly MaxWinPolicy = "cap_only"
	// MaxWinTerminateFeature terminates the current feature when cap is reached.
	MaxWinTerminateFeature MaxWinPolicy = "terminate_feature"
	// MaxWinTerminateSession terminates all further cascades and spins immediately.
	MaxWinTerminateSession MaxWinPolicy = "terminate_session"
)

// MathProfile holds theoretical math targets and volatility classifications.
type MathProfile struct {
	TargetRTP  float64 `json:"target_rtp,omitempty"`
	Volatility string  `json:"volatility,omitempty"`
}

// Definition is the complete, versioned, JSON-serializable specification of a slot game.
type Definition struct {
	ID               string             `json:"id"`
	Version          string             `json:"version"`
	Name             string             `json:"name"`
	Grid             GridConfig         `json:"grid"`
	WinMechanic      WinMechanicConfig  `json:"win_mechanic"`
	Symbols          []SymbolConfig     `json:"symbols"`
	ReelSets         map[string]ReelSet `json:"reel_sets"`
	BaseReelSet      string             `json:"base_reel_set,omitempty"`
	Wild             WildConfig         `json:"wild"`
	Scatter          ScatterConfig      `json:"scatter"`
	Cascade          CascadeConfig      `json:"cascade"`
	FreeSpins        FreeSpinsConfig    `json:"free_spins"`
	Features         []FeatureConfig    `json:"features,omitempty"`
	Math             MathProfile        `json:"math,omitempty"`
	MaxWinMultiplier Multiplier         `json:"max_win_multiplier"`
	MaxWinPolicy     MaxWinPolicy       `json:"max_win_policy"`
}

// ValidatedDefinition wraps an immutable deep copy of Definition with precomputed lookup indexes.
type ValidatedDefinition struct {
	def               Definition
	symbolsByID       map[SymbolID]SymbolConfig
	reelSetsByName    map[string]ReelSet
	wildSubstitutions map[SymbolID]map[SymbolID]bool
	wildSymbols       map[SymbolID]bool
	scatterSymbols    map[SymbolID]bool
	bonusSymbols      map[SymbolID]bool
	regularSymbols    map[SymbolID]bool
	featureIDs        map[string]bool
	minRows           int
	maxRows           int
}

// ValidateDefinition validates and deep-copies a Definition, precomputing indexes.
func ValidateDefinition(def Definition) (*ValidatedDefinition, error) {
	if def.ID == "" {
		return nil, fmt.Errorf("%w: ID must not be empty", ErrInvalidDefinition)
	}
	if def.Version == "" {
		return nil, fmt.Errorf("%w: Version must not be empty", ErrInvalidDefinition)
	}

	// Grid validation
	if def.Grid.Reels <= 0 {
		return nil, fmt.Errorf("%w: Reels must be > 0", ErrInvalidDefinition)
	}
	minRows := def.Grid.Rows
	maxRows := def.Grid.Rows
	if def.Grid.VariableRows {
		if def.Grid.MinRows <= 0 || def.Grid.MaxRows < def.Grid.MinRows {
			return nil, fmt.Errorf("%w: invalid variable rows [%d, %d]", ErrInvalidRowRange, def.Grid.MinRows, def.Grid.MaxRows)
		}
		minRows = def.Grid.MinRows
		maxRows = def.Grid.MaxRows
	} else {
		if def.Grid.Rows <= 0 {
			return nil, fmt.Errorf("%w: Rows must be > 0", ErrInvalidDefinition)
		}
		def.Grid.MinRows = def.Grid.Rows
		def.Grid.MaxRows = def.Grid.Rows
	}

	// Win mechanic validation
	if def.WinMechanic.Type == "" {
		def.WinMechanic.Type = WinMechanicWays
	}
	if def.WinMechanic.Type != WinMechanicWays {
		return nil, fmt.Errorf("%w: unsupported win mechanic %q", ErrInvalidDefinition, def.WinMechanic.Type)
	}
	if def.WinMechanic.MinMatch <= 0 || def.WinMechanic.MinMatch > def.Grid.Reels {
		return nil, fmt.Errorf("%w: min match %d outside [1, %d]", ErrInvalidDefinition, def.WinMechanic.MinMatch, def.Grid.Reels)
	}

	// Symbols validation
	if len(def.Symbols) == 0 {
		return nil, fmt.Errorf("%w: symbols catalog must not be empty", ErrInvalidDefinition)
	}
	symbolsByID := make(map[SymbolID]SymbolConfig, len(def.Symbols))
	wildSymbols := make(map[SymbolID]bool)
	scatterSymbols := make(map[SymbolID]bool)
	bonusSymbols := make(map[SymbolID]bool)
	regularSymbols := make(map[SymbolID]bool)

	for _, sym := range def.Symbols {
		if sym.ID == "" {
			return nil, fmt.Errorf("%w: symbol ID must not be empty", ErrInvalidDefinition)
		}
		if _, exists := symbolsByID[sym.ID]; exists {
			return nil, fmt.Errorf("%w: %q", ErrDuplicateSymbol, sym.ID)
		}

		switch sym.Type {
		case SymbolRegular:
			regularSymbols[sym.ID] = true
			if len(sym.Payouts) != def.Grid.Reels+1 {
				return nil, fmt.Errorf("%w: symbol %q has payout length %d, want %d", ErrInvalidPayoutLength, sym.ID, len(sym.Payouts), def.Grid.Reels+1)
			}
			for match, p := range sym.Payouts {
				if p < 0 {
					return nil, fmt.Errorf("%w: symbol %q match %d payout is negative", ErrInvalidDefinition, sym.ID, match)
				}
			}
		case SymbolWild:
			wildSymbols[sym.ID] = true
			if len(sym.Payouts) > 0 {
				if len(sym.Payouts) != def.Grid.Reels+1 {
					return nil, fmt.Errorf("%w: wild symbol %q has payout length %d, want %d", ErrInvalidPayoutLength, sym.ID, len(sym.Payouts), def.Grid.Reels+1)
				}
			}
		case SymbolScatter:
			scatterSymbols[sym.ID] = true
		case SymbolBonus:
			bonusSymbols[sym.ID] = true
		default:
			return nil, fmt.Errorf("%w: invalid symbol type %q for %q", ErrInvalidDefinition, sym.Type, sym.ID)
		}

		symbolsByID[sym.ID] = sym
	}

	// Wild symbols resolution
	if len(def.Wild.WildSymbolIDs) > 0 {
		for _, wid := range def.Wild.WildSymbolIDs {
			sym, ok := symbolsByID[wid]
			if !ok {
				return nil, fmt.Errorf("%w: wild symbol %q not found", ErrUnknownSymbol, wid)
			}
			if sym.Type != SymbolWild {
				return nil, fmt.Errorf("%w: symbol %q declared in wild_symbols but type is %q", ErrInvalidDefinition, wid, sym.Type)
			}
		}
	} else {
		// Populate from catalog
		for wid := range wildSymbols {
			def.Wild.WildSymbolIDs = append(def.Wild.WildSymbolIDs, wid)
		}
	}

	// Validate SubstituteFor and CannotSubstituteFor
	for _, target := range def.Wild.SubstituteFor {
		sym, ok := symbolsByID[target]
		if !ok {
			return nil, fmt.Errorf("%w: wild substitute_for symbol %q", ErrUnknownSymbol, target)
		}
		if sym.Type == SymbolScatter || sym.Type == SymbolBonus {
			return nil, fmt.Errorf("%w: wild cannot substitute for scatter or bonus symbol %q", ErrIllegalWildTarget, target)
		}
	}
	cannotSub := make(map[SymbolID]bool)
	for _, target := range def.Wild.CannotSubstituteFor {
		if _, ok := symbolsByID[target]; !ok {
			return nil, fmt.Errorf("%w: wild cannot_substitute_for symbol %q", ErrUnknownSymbol, target)
		}
		cannotSub[target] = true
	}
	// By default, scatter and bonus symbols cannot be substituted
	for sid := range scatterSymbols {
		cannotSub[sid] = true
	}
	for sid := range bonusSymbols {
		cannotSub[sid] = true
	}

	// Validate per-wild substitution configuration
	for _, sym := range def.Symbols {
		if sym.Type == SymbolWild {
			for _, target := range sym.SubstituteFor {
				ts, ok := symbolsByID[target]
				if !ok {
					return nil, fmt.Errorf("%w: wild %q substitute_for symbol %q", ErrUnknownSymbol, sym.ID, target)
				}
				if ts.Type == SymbolScatter || ts.Type == SymbolBonus {
					return nil, fmt.Errorf("%w: wild %q cannot substitute for scatter or bonus symbol %q", ErrIllegalWildTarget, sym.ID, target)
				}
			}
			for _, target := range sym.CannotSubstituteFor {
				if _, ok := symbolsByID[target]; !ok {
					return nil, fmt.Errorf("%w: wild %q cannot_substitute_for symbol %q", ErrUnknownSymbol, sym.ID, target)
				}
			}
		}
	}

	// Build wild substitution matrix
	wildSubstitutions := make(map[SymbolID]map[SymbolID]bool, len(def.Wild.WildSymbolIDs))
	globalSubstituteAllowed := make(map[SymbolID]bool)
	if len(def.Wild.SubstituteFor) > 0 {
		for _, sid := range def.Wild.SubstituteFor {
			globalSubstituteAllowed[sid] = true
		}
	} else {
		for sid := range regularSymbols {
			globalSubstituteAllowed[sid] = true
		}
	}

	for _, wid := range def.Wild.WildSymbolIDs {
		wildSubstitutions[wid] = make(map[SymbolID]bool)
		symConf := symbolsByID[wid]

		subAllowed := make(map[SymbolID]bool)
		if len(symConf.SubstituteFor) > 0 {
			for _, sid := range symConf.SubstituteFor {
				subAllowed[sid] = true
			}
		} else {
			for sid, ok := range globalSubstituteAllowed {
				if ok {
					subAllowed[sid] = true
				}
			}
		}

		perCannotSub := make(map[SymbolID]bool)
		for sid := range cannotSub {
			perCannotSub[sid] = true
		}
		if len(symConf.CannotSubstituteFor) > 0 {
			for _, sid := range symConf.CannotSubstituteFor {
				perCannotSub[sid] = true
			}
		}

		for sid := range regularSymbols {
			if subAllowed[sid] && !perCannotSub[sid] {
				wildSubstitutions[wid][sid] = true
			}
		}
	}

	// Scatter validation
	if def.Scatter.SymbolID != "" {
		sym, ok := symbolsByID[def.Scatter.SymbolID]
		if !ok {
			return nil, fmt.Errorf("%w: scatter symbol %q", ErrUnknownSymbol, def.Scatter.SymbolID)
		}
		if sym.Type != SymbolScatter {
			return nil, fmt.Errorf("%w: symbol %q declared as scatter symbol_id but type is %q", ErrInvalidDefinition, def.Scatter.SymbolID, sym.Type)
		}
	}
	for _, th := range def.Scatter.Thresholds {
		if th.Count <= 0 {
			return nil, fmt.Errorf("%w: scatter threshold count must be > 0", ErrInvalidDefinition)
		}
		if th.Payout < 0 {
			return nil, fmt.Errorf("%w: scatter threshold payout must be >= 0", ErrInvalidDefinition)
		}
	}

	// ReelSets validation
	if len(def.ReelSets) == 0 {
		return nil, fmt.Errorf("%w: at least one reel set required", ErrMissingReelSet)
	}
	reelSetsByName := make(map[string]ReelSet, len(def.ReelSets))
	for name, rs := range def.ReelSets {
		if len(rs.Reels) != def.Grid.Reels {
			return nil, fmt.Errorf("%w: reel set %q has %d reels, want %d", ErrMalformedReelSet, name, len(rs.Reels), def.Grid.Reels)
		}
		for r, strip := range rs.Reels {
			if len(strip) < minRows {
				return nil, fmt.Errorf("%w: reel set %q reel %d strip length %d < minRows %d", ErrMalformedReelSet, name, r, len(strip), minRows)
			}
			for _, sid := range strip {
				if _, ok := symbolsByID[sid]; !ok {
					return nil, fmt.Errorf("%w: reel set %q reel %d contains unknown symbol %q", ErrUnknownSymbol, name, r, sid)
				}
			}
		}
		reelSetsByName[name] = rs.Clone()
	}

	// BaseReelSet validation
	if def.BaseReelSet == "" {
		def.BaseReelSet = "base"
	}
	if _, ok := reelSetsByName[def.BaseReelSet]; !ok {
		return nil, fmt.Errorf("%w: base reel set %q not found", ErrMissingReelSet, def.BaseReelSet)
	}

	// Cascade validation
	if def.Cascade.Enabled {
		if def.Cascade.MaxCascades <= 0 {
			def.Cascade.MaxCascades = 100
		}
		switch def.Cascade.MultiplierMode {
		case "", CascadeMultiplierFixed:
			def.Cascade.MultiplierMode = CascadeMultiplierFixed
			if def.Cascade.FixedMultiplier <= 0 {
				def.Cascade.FixedMultiplier = Multiplier(MultiplierScale)
			}
		case CascadeMultiplierIncremental:
			if def.Cascade.MultiplierStep < 0 {
				return nil, fmt.Errorf("%w: cascade multiplier step cannot be negative", ErrInvalidDefinition)
			}
		case CascadeMultiplierTable:
			if len(def.Cascade.MultiplierTable) == 0 {
				return nil, fmt.Errorf("%w: cascade multiplier table must not be empty", ErrInvalidDefinition)
			}
			for i, m := range def.Cascade.MultiplierTable {
				if m <= 0 {
					return nil, fmt.Errorf("%w: cascade multiplier table index %d is non-positive", ErrInvalidDefinition, i)
				}
			}
		default:
			return nil, fmt.Errorf("%w: unknown cascade multiplier mode %q", ErrInvalidDefinition, def.Cascade.MultiplierMode)
		}
	}

	// FreeSpins validation
	if def.FreeSpins.Enabled {
		if def.FreeSpins.ReelSet != "" {
			if _, ok := reelSetsByName[def.FreeSpins.ReelSet]; !ok {
				return nil, fmt.Errorf("%w: free spins reel set %q not found", ErrInvalidFeatureRef, def.FreeSpins.ReelSet)
			}
		}
		if def.FreeSpins.MaxTotalSpins <= 0 {
			def.FreeSpins.MaxTotalSpins = 100
		}
		if def.FreeSpins.MultiplierPolicy == "" {
			def.FreeSpins.MultiplierPolicy = MultiplierResetEachSpin
		}
		switch def.FreeSpins.MultiplierPolicy {
		case MultiplierResetEachSpin, MultiplierPersistent, MultiplierCustom:
		default:
			return nil, fmt.Errorf("%w: unknown multiplier policy %q", ErrInvalidDefinition, def.FreeSpins.MultiplierPolicy)
		}
	}

	// Features validation
	featureIDs := make(map[string]bool, len(def.Features))
	for _, feat := range def.Features {
		if feat.ID == "" {
			return nil, fmt.Errorf("%w: feature ID must not be empty", ErrInvalidDefinition)
		}
		if _, seen := featureIDs[feat.ID]; seen {
			return nil, fmt.Errorf("%w: %q", ErrDuplicateFeatureID, feat.ID)
		}
		featureIDs[feat.ID] = feat.Enabled
	}

	// Max win validation
	if def.MaxWinMultiplier < 0 {
		return nil, fmt.Errorf("%w: max win multiplier cannot be negative", ErrIncoherentCap)
	}
	if def.MaxWinMultiplier > 0 && def.MaxWinPolicy == "" {
		def.MaxWinPolicy = MaxWinCapOnly
	}
	if def.MaxWinPolicy != "" {
		switch def.MaxWinPolicy {
		case MaxWinCapOnly, MaxWinTerminateFeature, MaxWinTerminateSession:
		default:
			return nil, fmt.Errorf("%w: unknown max win policy %q", ErrInvalidDefinition, def.MaxWinPolicy)
		}
	}

	// Create deep-copied Definition
	cpSymbols := make([]SymbolConfig, len(def.Symbols))
	copiedSymbolsByID := make(map[SymbolID]SymbolConfig, len(def.Symbols))
	for i, sc := range def.Symbols {
		payoutsCp := make([]Multiplier, len(sc.Payouts))
		copy(payoutsCp, sc.Payouts)
		var metaCp json.RawMessage
		if sc.Metadata != nil {
			metaCp = make(json.RawMessage, len(sc.Metadata))
			copy(metaCp, sc.Metadata)
		}
		cpSymbols[i] = SymbolConfig{
			ID:                  sc.ID,
			Name:                sc.Name,
			Type:                sc.Type,
			Payouts:             payoutsCp,
			Metadata:            metaCp,
			SubstituteFor:       append([]SymbolID(nil), sc.SubstituteFor...),
			CannotSubstituteFor: append([]SymbolID(nil), sc.CannotSubstituteFor...),
		}
		copiedSymbolsByID[sc.ID] = cpSymbols[i]
	}
	symbolsByID = copiedSymbolsByID

	cpFeatures := make([]FeatureConfig, len(def.Features))
	for i, fc := range def.Features {
		var cfgRaw json.RawMessage
		if fc.Config != nil {
			cfgRaw = make(json.RawMessage, len(fc.Config))
			copy(cfgRaw, fc.Config)
		}
		cpFeatures[i] = FeatureConfig{
			ID:      fc.ID,
			Enabled: fc.Enabled,
			Config:  cfgRaw,
		}
	}

	cpThresholds := make([]ScatterTrigger, len(def.Scatter.Thresholds))
	for i, st := range def.Scatter.Thresholds {
		trigsCp := make([]Trigger, len(st.Triggers))
		for j, tr := range st.Triggers {
			var pl json.RawMessage
			if tr.Payload != nil {
				pl = make(json.RawMessage, len(tr.Payload))
				copy(pl, tr.Payload)
			}
			trigsCp[j] = Trigger{
				Kind:       tr.Kind,
				Count:      tr.Count,
				Award:      tr.Award,
				Multiplier: tr.Multiplier,
				Payload:    pl,
			}
		}
		cpThresholds[i] = ScatterTrigger{
			Count:    st.Count,
			Payout:   st.Payout,
			Triggers: trigsCp,
		}
	}

	defCopy := Definition{
		ID:          def.ID,
		Version:     def.Version,
		Name:        def.Name,
		Grid:        def.Grid,
		WinMechanic: def.WinMechanic,
		Symbols:     cpSymbols,
		ReelSets:    reelSetsByName,
		BaseReelSet: def.BaseReelSet,
		Wild: WildConfig{
			WildSymbolIDs:       append([]SymbolID(nil), def.Wild.WildSymbolIDs...),
			SubstituteFor:       append([]SymbolID(nil), def.Wild.SubstituteFor...),
			CannotSubstituteFor: append([]SymbolID(nil), def.Wild.CannotSubstituteFor...),
		},
		Scatter: ScatterConfig{
			SymbolID:   def.Scatter.SymbolID,
			Thresholds: cpThresholds,
		},
		Cascade: CascadeConfig{
			Enabled:         def.Cascade.Enabled,
			MaxCascades:     def.Cascade.MaxCascades,
			MultiplierMode:  def.Cascade.MultiplierMode,
			FixedMultiplier: def.Cascade.FixedMultiplier,
			MultiplierStep:  def.Cascade.MultiplierStep,
			MultiplierTable: append([]Multiplier(nil), def.Cascade.MultiplierTable...),
		},
		FreeSpins:        def.FreeSpins,
		Features:         cpFeatures,
		Math:             def.Math,
		MaxWinMultiplier: def.MaxWinMultiplier,
		MaxWinPolicy:     def.MaxWinPolicy,
	}

	return &ValidatedDefinition{
		def:               defCopy,
		symbolsByID:       symbolsByID,
		reelSetsByName:    reelSetsByName,
		wildSubstitutions: wildSubstitutions,
		wildSymbols:       wildSymbols,
		scatterSymbols:    scatterSymbols,
		bonusSymbols:      bonusSymbols,
		regularSymbols:    regularSymbols,
		featureIDs:        featureIDs,
		minRows:           minRows,
		maxRows:           maxRows,
	}, nil
}

// Definition returns a deep copy of the underlying validated Definition.
func (d *ValidatedDefinition) Definition() Definition {
	cp, _ := ValidateDefinition(d.def)
	return cp.def
}

// ID returns the validated game ID.
func (d *ValidatedDefinition) ID() string {
	return d.def.ID
}

// Version returns the validated game version.
func (d *ValidatedDefinition) Version() string {
	return d.def.Version
}

// CascadeEnabled returns whether cascades are enabled.
func (d *ValidatedDefinition) CascadeEnabled() bool {
	return d.def.Cascade.Enabled
}

// CascadeMax returns the maximum number of consecutive cascades allowed per round.
func (d *ValidatedDefinition) CascadeMax() int {
	return d.def.Cascade.MaxCascades
}

// CascadeMultiplierMode returns the cascade multiplier scaling mode.
func (d *ValidatedDefinition) CascadeMultiplierMode() CascadeMultiplierMode {
	return d.def.Cascade.MultiplierMode
}

// CascadeFixedMultiplier returns the fixed cascade multiplier.
func (d *ValidatedDefinition) CascadeFixedMultiplier() Multiplier {
	return d.def.Cascade.FixedMultiplier
}

// CascadeMultiplierStep returns the incremental step multiplier per cascade.
func (d *ValidatedDefinition) CascadeMultiplierStep() Multiplier {
	return d.def.Cascade.MultiplierStep
}

// CascadeTableMultiplier looks up the multiplier at the specified cascade index from the table.
// If index is past the end of the table, it returns the final entry.
// Returns false if the table is empty.
func (d *ValidatedDefinition) CascadeTableMultiplier(index int) (Multiplier, bool) {
	tbl := d.def.Cascade.MultiplierTable
	if len(tbl) == 0 {
		return 0, false
	}
	if index < 0 {
		return tbl[0], true
	}
	if index >= len(tbl) {
		return tbl[len(tbl)-1], true
	}
	return tbl[index], true
}

// CascadeMultiplierTableLen returns the number of entries in the cascade multiplier table.
func (d *ValidatedDefinition) CascadeMultiplierTableLen() int {
	return len(d.def.Cascade.MultiplierTable)
}

// FreeSpinsEnabled returns whether the free spins feature is enabled.
func (d *ValidatedDefinition) FreeSpinsEnabled() bool {
	return d.def.FreeSpins.Enabled
}

// FreeSpinsReelSet returns the reel set name used for free spins.
func (d *ValidatedDefinition) FreeSpinsReelSet() string {
	return d.def.FreeSpins.ReelSet
}

// FreeSpinsMaxTotal returns the maximum total free spins cap.
func (d *ValidatedDefinition) FreeSpinsMaxTotal() int {
	return d.def.FreeSpins.MaxTotalSpins
}

// FreeSpinsMultiplierPolicy returns the multiplier policy configured for free spins.
func (d *ValidatedDefinition) FreeSpinsMultiplierPolicy() MultiplierPolicy {
	return d.def.FreeSpins.MultiplierPolicy
}

// FreeSpinsRetriggerEnabled returns whether free spins can be retriggered.
func (d *ValidatedDefinition) FreeSpinsRetriggerEnabled() bool {
	return d.def.FreeSpins.RetriggerEnabled
}

// MaxWinMultiplier returns the maximum win multiplier cap.
func (d *ValidatedDefinition) MaxWinMultiplier() Multiplier {
	return d.def.MaxWinMultiplier
}

// MaxWinPolicy returns the policy applied when reaching the maximum win cap.
func (d *ValidatedDefinition) MaxWinPolicy() MaxWinPolicy {
	return d.def.MaxWinPolicy
}

// Symbol looks up a SymbolConfig by ID from the precomputed catalog.
func (d *ValidatedDefinition) Symbol(id SymbolID) (SymbolConfig, bool) {
	sym, ok := d.symbolsByID[id]
	if !ok {
		return SymbolConfig{}, false
	}
	if sym.Payouts != nil {
		payoutsCp := make([]Multiplier, len(sym.Payouts))
		copy(payoutsCp, sym.Payouts)
		sym.Payouts = payoutsCp
	}
	return sym, true
}

// ReelSet looks up a ReelSet by name.
func (d *ValidatedDefinition) ReelSet(name string) (ReelSet, bool) {
	rs, ok := d.reelSetsByName[name]
	if !ok {
		return ReelSet{}, false
	}
	return rs.Clone(), true
}

// CanWildSubstitute reports whether a wild symbol can substitute for target regular symbol.
func (d *ValidatedDefinition) CanWildSubstitute(wild, target SymbolID) bool {
	targets, ok := d.wildSubstitutions[wild]
	if !ok {
		return false
	}
	return targets[target]
}

// IsWild reports whether a symbol ID is a registered wild symbol.
func (d *ValidatedDefinition) IsWild(id SymbolID) bool {
	return d.wildSymbols[id]
}

// IsScatter reports whether a symbol ID is a registered scatter symbol.
func (d *ValidatedDefinition) IsScatter(id SymbolID) bool {
	return d.scatterSymbols[id]
}

// IsBonus reports whether a symbol ID is a registered bonus symbol.
func (d *ValidatedDefinition) IsBonus(id SymbolID) bool {
	return d.bonusSymbols[id]
}

// IsRegular reports whether a symbol ID is a registered regular symbol.
func (d *ValidatedDefinition) IsRegular(id SymbolID) bool {
	return d.regularSymbols[id]
}

// BaseReelSet returns the configured default base reel set name.
func (d *ValidatedDefinition) BaseReelSet() string {
	return d.def.BaseReelSet
}
