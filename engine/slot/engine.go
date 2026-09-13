// Package slot orchestrates server-authoritative, config-driven slot game rounds,
// cascading mechanics, feature modes, and deterministic settlement.
package slot

import (
	"errors"
	"fmt"

	"github.com/qxbao/qxprob/core/entropy"
	"github.com/qxbao/qxprob/core/slot"
)

var (
	// ErrNilSource indicates that a nil entropy source was provided.
	ErrNilSource = errors.New("slot: entropy source must not be nil")
	// ErrEmptyServerSeed indicates that an empty server seed was provided.
	ErrEmptyServerSeed = errors.New("slot: server seed must not be empty")
	// ErrEmptyClientSeed indicates that an empty client seed was provided.
	ErrEmptyClientSeed = errors.New("slot: client seed must not be empty")
	// ErrMismatchedGameID indicates that the request game ID does not match definition ID.
	ErrMismatchedGameID = errors.New("slot: request game ID does not match definition")
	// ErrMismatchedVersion indicates that the request version does not match definition version.
	ErrMismatchedVersion = errors.New("slot: request version does not match definition")
	// ErrInvalidBet indicates a non-positive bet amount.
	ErrInvalidBet = errors.New("slot: bet must be greater than zero")
	// ErrUnregisteredFeature indicates an enabled feature in definition has no registered module.
	ErrUnregisteredFeature = errors.New("slot: unknown or unregistered feature enabled")
)

// Options configures optional evaluator and mode behaviors for the engine.
type Options struct {
	Mode      slot.PlayMode
	Evaluator slot.Evaluator
}

// Game represents an immutable compiled slot game definition, ordered feature modules,
// and default evaluation options. It owns only immutable-by-convention compiled data
// and retains no entropy, nonce, grid, or per-round state.
//
// Reusing a *Game sequentially across arbitrarily many rounds is safe and recommended.
// For concurrent execution across multiple goroutines, callers must supply distinct
// entropy sources per caller/goroutine. Custom Feature and Evaluator implementations
// must also be safe for concurrent use; built-in features and evaluators are stateless.
type Game struct {
	vdef     *slot.ValidatedDefinition
	def      slot.Definition
	features []slot.Feature
	opts     Options
}

// Compile validates and prepares an immutable, reusable slot Game definition and its features.
// It deep-copies definition data, verifies that all enabled features are registered, orders
// features according to Definition.Features declaration order, resolves options/evaluators,
// and rejects duplicate feature modules.
// Compilation does not consume entropy.
func Compile(def slot.Definition, features []slot.Feature, opts Options) (*Game, error) {
	vdef, err := slot.ValidateDefinition(def)
	if err != nil {
		return nil, err
	}

	registered := make(map[string]slot.Feature, len(features))
	for _, f := range features {
		if f != nil {
			id := f.ID()
			if _, exists := registered[id]; exists {
				return nil, fmt.Errorf("%w: duplicate feature module %q", slot.ErrDuplicateFeatureID, id)
			}
			registered[id] = f
		}
	}

	var orderedFeatures []slot.Feature
	if len(def.Features) > 0 {
		for _, featDecl := range def.Features {
			if featDecl.Enabled {
				mod, ok := registered[featDecl.ID]
				if !ok {
					return nil, fmt.Errorf("%w: %q", ErrUnregisteredFeature, featDecl.ID)
				}
				orderedFeatures = append(orderedFeatures, mod)
			}
		}
	} else {
		for _, f := range features {
			if f != nil {
				orderedFeatures = append(orderedFeatures, f)
			}
		}
	}

	if def.FreeSpins.MultiplierPolicy == slot.MultiplierCustom && len(features) == 0 {
		return nil, fmt.Errorf("%w: custom multiplier policy requires a registered feature", slot.ErrInvalidDefinition)
	}

	if opts.Evaluator == nil {
		opts.Evaluator = slot.WaysEvaluator{}
	}
	if opts.Mode == "" {
		opts.Mode = slot.PlayModeNormal
	}

	return &Game{
		vdef:     vdef,
		def:      vdef.Definition(),
		features: orderedFeatures,
		opts:     opts,
	}, nil
}

// Definition returns the validated slot game definition.
func (g *Game) Definition() slot.Definition {
	return g.vdef.Definition()
}

// Engine orchestrates slot grid generation, evaluation, feature plugins, and settlement
// bound to a specific entropy source.
//
// Deprecated: For high-volume simulations, use Compile once and call Game.Spin(req, src)
// per round with a reusable, resettable entropy source instead of constructing an Engine per nonce.
type Engine struct {
	game   *Game
	source entropy.Source
}

// New constructs a validated slot Engine. It compiles the definition and features once,
// stores the compiled game plus its legacy source, and delegates Spin calls to the compiled game.
func New(def slot.Definition, src entropy.Source, features []slot.Feature, opts Options) (*Engine, error) {
	if src == nil {
		return nil, ErrNilSource
	}

	game, err := Compile(def, features, opts)
	if err != nil {
		return nil, err
	}

	return &Engine{
		game:   game,
		source: src,
	}, nil
}

// Spin executes a single spin round using the Engine's bound entropy source.
func (e *Engine) Spin(req slot.SpinRequest) (slot.SpinResult, error) {
	return e.game.Spin(req, e.source)
}

// PlaySeeded executes a deterministic, reproducible round of a slot game using server seed,
// client seed, and nonce. It strictly validates inputs before constructing an entropy source.
//
// Note: For repeated seeded rounds, call Compile once and reuse the compiled Game with a
// ProvablyFairSource using ResetNonce(nonce) for significantly higher performance.
func PlaySeeded(def slot.Definition, req slot.SeededSpinRequest, features []slot.Feature, opts Options) (slot.SpinResult, error) {
	if req.ServerSeed == "" {
		return slot.SpinResult{}, ErrEmptyServerSeed
	}
	if req.ClientSeed == "" {
		return slot.SpinResult{}, ErrEmptyClientSeed
	}
	if req.GameID != def.ID {
		return slot.SpinResult{}, fmt.Errorf("%w: got %q, want %q", ErrMismatchedGameID, req.GameID, def.ID)
	}
	if req.Version != def.Version {
		return slot.SpinResult{}, fmt.Errorf("%w: got %q, want %q", ErrMismatchedVersion, req.Version, def.Version)
	}
	if req.Bet <= 0 {
		return slot.SpinResult{}, ErrInvalidBet
	}

	game, err := Compile(def, features, opts)
	if err != nil {
		return slot.SpinResult{}, err
	}

	src := entropy.NewProvablyFairSource(req.ServerSeed, req.ClientSeed, req.Nonce)
	return game.Spin(req.SpinRequest, src)
}

// Spin executes a single spin round, handling base grid generation, cascading collapses,
// free spin sessions, max win limits, and fixed-point settlement.
func (g *Game) Spin(req slot.SpinRequest, src entropy.Source) (slot.SpinResult, error) {
	if src == nil {
		return slot.SpinResult{}, ErrNilSource
	}

	def := &g.def
	if req.GameID == "" {
		req.GameID = def.ID
	} else if req.GameID != def.ID {
		return slot.SpinResult{}, fmt.Errorf("%w: got %q, want %q", ErrMismatchedGameID, req.GameID, def.ID)
	}
	if req.Version == "" {
		req.Version = def.Version
	} else if req.Version != def.Version {
		return slot.SpinResult{}, fmt.Errorf("%w: got %q, want %q", ErrMismatchedVersion, req.Version, def.Version)
	}
	if req.Bet <= 0 {
		return slot.SpinResult{}, ErrInvalidBet
	}

	mode := req.Mode
	if mode == "" {
		mode = g.opts.Mode
	}

	sm := slot.NewStateMachine()
	var events []slot.Event

	emitEvent := func(kind slot.EventKind, desc string, payload slot.EventPayload) error {
		if mode == slot.PlayModeSimulation {
			return nil
		}
		raw, err := slot.MarshalPayload(payload)
		if err != nil {
			return err
		}
		events = append(events, slot.Event{
			Kind:        kind,
			Description: desc,
			Payload:     raw,
		})
		return nil
	}

	var allTriggers []slot.Trigger
	var triggeredFeatures []string

	addTriggers := func(triggers []slot.Trigger) {
		for _, tr := range triggers {
			duplicate := false
			for _, existing := range allTriggers {
				if existing.Kind == tr.Kind && existing.Count == tr.Count && existing.Award == tr.Award && existing.Multiplier == tr.Multiplier {
					duplicate = true
					break
				}
			}
			if !duplicate {
				allTriggers = append(allTriggers, tr)
			}
			featureName := string(tr.Kind)
			hasFeature := false
			for _, f := range triggeredFeatures {
				if f == featureName {
					hasFeature = true
					break
				}
			}
			if !hasFeature {
				triggeredFeatures = append(triggeredFeatures, featureName)
			}
		}
	}

	activeReelSet := g.vdef.BaseReelSet()

	ctx := &slot.FeatureContext{
		Definition:    g.vdef,
		Mode:          mode,
		IsFreeSpin:    false,
		RoundIndex:    0,
		CascadeIndex:  0,
		ActiveReelSet: activeReelSet,
	}

	runHook := func(hook func(slot.Feature, *slot.FeatureContext) error) error {
		for _, f := range g.features {
			if err := hook(f, ctx); err != nil {
				return err
			}
		}
		return nil
	}

	// 1. Begin spin
	if err := sm.Transition(slot.StateSpinning); err != nil {
		return slot.SpinResult{}, err
	}
	if err := runHook(func(f slot.Feature, c *slot.FeatureContext) error { return f.BeforeSpin(c) }); err != nil {
		return slot.SpinResult{}, err
	}

	// 2. Generate initial grid
	if err := sm.Transition(slot.StateEvaluating); err != nil {
		return slot.SpinResult{}, err
	}
	genGrid, err := slot.GenerateGrid(g.vdef, ctx.ActiveReelSet, src)
	if err != nil {
		return slot.SpinResult{}, err
	}
	ctx.Grid = genGrid.Grid
	if err := emitEvent(slot.EventGridReveal, "Initial grid revealed", slot.GridRevealPayload{Grid: ctx.Grid.Clone()}); err != nil {
		return slot.SpinResult{}, err
	}

	if err := runHook(func(f slot.Feature, c *slot.FeatureContext) error { return f.AfterGridGenerated(c) }); err != nil {
		return slot.SpinResult{}, err
	}
	if err := runHook(func(f slot.Feature, c *slot.FeatureContext) error { return f.BeforeEvaluation(c) }); err != nil {
		return slot.SpinResult{}, err
	}

	// 3. Initial evaluation
	eval, err := g.opts.Evaluator.Evaluate(g.vdef, ctx.Grid)
	if err != nil {
		return slot.SpinResult{}, err
	}
	ctx.Evaluation = eval
	if len(eval.Wins) > 0 {
		if err := emitEvent(slot.EventWins, fmt.Sprintf("%d winning combinations", len(eval.Wins)), slot.WinsPayload{Wins: eval.Wins}); err != nil {
			return slot.SpinResult{}, err
		}
	}

	if err := runHook(func(f slot.Feature, c *slot.FeatureContext) error { return f.AfterEvaluation(c) }); err != nil {
		return slot.SpinResult{}, err
	}

	if eval.ScatterPayout > 0 || len(eval.Triggers) > 0 {
		if err := emitEvent(slot.EventScatterTrigger, "Scatter conditions triggered", slot.ScatterTriggerPayload{
			Count:     eval.ScatterCount,
			Positions: eval.ScatterPositions,
			Payout:    eval.ScatterPayout,
			Triggers:  eval.Triggers,
		}); err != nil {
			return slot.SpinResult{}, err
		}
		addTriggers(eval.Triggers)
	}

	maxCap := def.MaxWinMultiplier
	maxPolicy := def.MaxWinPolicy
	capReached := false

	// Calculate base ways multiplier and scatter payout
	waysWinMult := eval.TotalMultiplier
	if eval.ScatterPayout > 0 {
		var err error
		waysWinMult, err = waysWinMult.Sub(eval.ScatterPayout)
		if err != nil {
			return slot.SpinResult{}, err
		}
	}
	accumulatedBaseMult := eval.ScatterPayout

	var baseCascades []slot.CascadeResult
	totalCascades := 0

	// If no cascades requested or cascades disabled, add initial ways win directly
	if !ctx.CascadeRequested {
		if waysWinMult > 0 {
			var err error
			accumulatedBaseMult, err = accumulatedBaseMult.Add(waysWinMult)
			if err != nil {
				return slot.SpinResult{}, err
			}
		}
	} else {
		// 4. Cascades loop (base spin)
		// On step 1, ctx.Grid is InitialGrid with ctx.Evaluation.Wins.
		// Its multiplier is ctx.CascadeMultiplier (e.g. 1.0x).
		for ctx.CascadeRequested {
			if maxCap > 0 && accumulatedBaseMult >= maxCap {
				accumulatedBaseMult = maxCap
				capReached = true
				if err := emitEvent(slot.EventMaxWinReached, "Max win cap reached", slot.MaxWinReachedPayload{CapMultiplier: maxCap, Policy: maxPolicy}); err != nil {
					return slot.SpinResult{}, err
				}
				if maxPolicy != slot.MaxWinCapOnly {
					break
				}
			}
			if ctx.TerminateSession || ctx.TerminateFeature {
				break
			}

			cascMult := ctx.CascadeMultiplier
			if cascMult <= 0 {
				cascMult = slot.MustMultiplier("1.0")
			}

			// Payout for current winning grid (ways win only, scatter payout was contributed on grid evaluation)
			currWaysMult := ctx.Evaluation.TotalMultiplier
			if ctx.Evaluation.ScatterPayout > 0 {
				var err error
				currWaysMult, err = currWaysMult.Sub(ctx.Evaluation.ScatterPayout)
				if err != nil {
					return slot.SpinResult{}, err
				}
			}
			stepWinMult, err := currWaysMult.Mul(cascMult)
			if err != nil {
				return slot.SpinResult{}, err
			}

			var addErr error
			accumulatedBaseMult, addErr = accumulatedBaseMult.Add(stepWinMult)
			if addErr != nil {
				return slot.SpinResult{}, addErr
			}

			if maxCap > 0 && accumulatedBaseMult >= maxCap {
				accumulatedBaseMult = maxCap
				capReached = true
				if err := emitEvent(slot.EventMaxWinReached, "Max win cap reached", slot.MaxWinReachedPayload{CapMultiplier: maxCap, Policy: maxPolicy}); err != nil {
					return slot.SpinResult{}, err
				}
				if maxPolicy != slot.MaxWinCapOnly {
					// Cap reached with terminate policy: record this final cascade step if desired, then stop
				}
			}

			if err := sm.Transition(slot.StateCascading); err != nil {
				return slot.SpinResult{}, err
			}

			var removePositions []slot.Position
			for _, w := range ctx.Evaluation.Wins {
				removePositions = append(removePositions, w.Positions...)
			}
			if err := emitEvent(slot.EventRemoval, "Winning positions removed", slot.RemovalPayload{Positions: removePositions}); err != nil {
				return slot.SpinResult{}, err
			}

			gridBefore := ctx.Grid.Clone()
			collapsedGrid, newSymbols, finalGrid, err := slot.CollapseAndRefill(g.vdef, ctx.ActiveReelSet, ctx.Grid, removePositions, src)
			if err != nil {
				return slot.SpinResult{}, err
			}

			totalCascades++
			ctx.CascadeIndex++
			if err := emitEvent(slot.EventCascade, "Grid refilled", slot.CascadePayload{
				Index:         ctx.CascadeIndex,
				CollapsedGrid: collapsedGrid,
				NewSymbols:    newSymbols,
				FinalGrid:     finalGrid,
				Multiplier:    cascMult,
			}); err != nil {
				return slot.SpinResult{}, err
			}

			if mode != slot.PlayModeSimulation {
				cascResult := slot.CascadeResult{
					Index:            ctx.CascadeIndex,
					GridBefore:       gridBefore,
					Wins:             ctx.Evaluation.Wins,
					RemovedPositions: removePositions,
					CollapsedGrid:    collapsedGrid,
					NewSymbols:       newSymbols,
					FinalGrid:        finalGrid,
					Multiplier:       cascMult,
					TotalMultiplier:  stepWinMult,
					Triggers:         ctx.Evaluation.Triggers,
				}
				baseCascades = append(baseCascades, cascResult)
			}

			ctx.Grid = finalGrid
			if err := runHook(func(f slot.Feature, c *slot.FeatureContext) error { return f.AfterCascade(c) }); err != nil {
				return slot.SpinResult{}, err
			}

			if capReached && maxPolicy != slot.MaxWinCapOnly {
				break
			}
			if ctx.TerminateSession || ctx.TerminateFeature {
				break
			}

			if err := sm.Transition(slot.StateEvaluating); err != nil {
				return slot.SpinResult{}, err
			}
			if err := runHook(func(f slot.Feature, c *slot.FeatureContext) error { return f.BeforeEvaluation(c) }); err != nil {
				return slot.SpinResult{}, err
			}

			nextEval, err := g.opts.Evaluator.Evaluate(g.vdef, ctx.Grid)
			if err != nil {
				return slot.SpinResult{}, err
			}
			ctx.Evaluation = nextEval

			// Contribute scatter payout from refilled grid (unmultiplied by cascade ways multiplier)
			if nextEval.ScatterPayout > 0 {
				var err error
				accumulatedBaseMult, err = accumulatedBaseMult.Add(nextEval.ScatterPayout)
				if err != nil {
					return slot.SpinResult{}, err
				}
			}

			// Emit scatter trigger event and aggregate triggers from refilled grid
			if nextEval.ScatterPayout > 0 || len(nextEval.Triggers) > 0 {
				if err := emitEvent(slot.EventScatterTrigger, "Scatter conditions triggered", slot.ScatterTriggerPayload{
					Count:     nextEval.ScatterCount,
					Positions: nextEval.ScatterPositions,
					Payout:    nextEval.ScatterPayout,
					Triggers:  nextEval.Triggers,
				}); err != nil {
					return slot.SpinResult{}, err
				}
				addTriggers(nextEval.Triggers)
			}

			if len(nextEval.Wins) > 0 {
				if err := emitEvent(slot.EventWins, fmt.Sprintf("%d winning combinations", len(nextEval.Wins)), slot.WinsPayload{Wins: nextEval.Wins}); err != nil {
					return slot.SpinResult{}, err
				}
			}

			// Reset cascade decision and ask feature modules if another cascade should follow
			ctx.ResetRoundDecisions()
			if err := runHook(func(f slot.Feature, c *slot.FeatureContext) error { return f.AfterEvaluation(c) }); err != nil {
				return slot.SpinResult{}, err
			}
		}
	}

	if maxCap > 0 && accumulatedBaseMult >= maxCap {
		accumulatedBaseMult = maxCap
		capReached = true
	}

	// 5. Free spins mode
	var freeSpinSessions []slot.SpinSession
	var fsState *slot.FreeSpinState
	var accumulatedFeatureMult slot.Multiplier
	totalFreeSpins := 0

	// Note: ctx.AwardSpins has been preserved across all cascades
	totalAwardedSpins := ctx.AwardSpins
	if totalAwardedSpins > 0 && (!capReached || maxPolicy == slot.MaxWinCapOnly) && !ctx.TerminateSession {
		if err := sm.Transition(slot.StateFeatureTrigger); err != nil {
			return slot.SpinResult{}, err
		}
		if err := emitEvent(slot.EventFreeSpinTrigger, fmt.Sprintf("Awarded %d free spins", totalAwardedSpins), slot.FreeSpinTriggerPayload{
			AwardedSpins: totalAwardedSpins,
			TotalSpins:   totalAwardedSpins,
			IsRetrigger:  false,
		}); err != nil {
			return slot.SpinResult{}, err
		}

		if err := sm.Transition(slot.StateFreeSpins); err != nil {
			return slot.SpinResult{}, err
		}

		fsState = &slot.FreeSpinState{
			RemainingSpins:   totalAwardedSpins,
			AwardedSpins:     totalAwardedSpins,
			TotalSpins:       totalAwardedSpins,
			GlobalMultiplier: slot.MustMultiplier("1.0"),
		}
		ctx.FreeSpinState = fsState
		ctx.IsFreeSpin = true
		ctx.AwardSpins = 0 // consumed

		for fsState.RemainingSpins > 0 {
			if capReached && maxPolicy != slot.MaxWinCapOnly {
				break
			}
			if ctx.TerminateSession || ctx.TerminateFeature {
				break
			}

			fsState.RemainingSpins--
			totalFreeSpins++
			ctx.RoundIndex++
			ctx.CascadeIndex = 0
			ctx.ResetRoundDecisions()

			if err := sm.Transition(slot.StateSpinning); err != nil {
				return slot.SpinResult{}, err
			}
			if err := runHook(func(f slot.Feature, c *slot.FeatureContext) error { return f.BeforeSpin(c) }); err != nil {
				return slot.SpinResult{}, err
			}

			if err := sm.Transition(slot.StateEvaluating); err != nil {
				return slot.SpinResult{}, err
			}
			fsGrid, err := slot.GenerateGrid(g.vdef, ctx.ActiveReelSet, src)
			if err != nil {
				return slot.SpinResult{}, err
			}
			ctx.Grid = fsGrid.Grid
			if err := emitEvent(slot.EventGridReveal, "Free spin grid revealed", slot.GridRevealPayload{Grid: ctx.Grid.Clone()}); err != nil {
				return slot.SpinResult{}, err
			}

			if err := runHook(func(f slot.Feature, c *slot.FeatureContext) error { return f.AfterGridGenerated(c) }); err != nil {
				return slot.SpinResult{}, err
			}
			if err := runHook(func(f slot.Feature, c *slot.FeatureContext) error { return f.BeforeEvaluation(c) }); err != nil {
				return slot.SpinResult{}, err
			}

			fsEval, err := g.opts.Evaluator.Evaluate(g.vdef, ctx.Grid)
			if err != nil {
				return slot.SpinResult{}, err
			}
			ctx.Evaluation = fsEval
			if len(fsEval.Wins) > 0 {
				if err := emitEvent(slot.EventWins, fmt.Sprintf("%d free spin winning combinations", len(fsEval.Wins)), slot.WinsPayload{Wins: fsEval.Wins}); err != nil {
					return slot.SpinResult{}, err
				}
			}

			if fsEval.ScatterPayout > 0 || len(fsEval.Triggers) > 0 {
				if err := emitEvent(slot.EventScatterTrigger, "Free spin scatter conditions triggered", slot.ScatterTriggerPayload{
					Count:     fsEval.ScatterCount,
					Positions: fsEval.ScatterPositions,
					Payout:    fsEval.ScatterPayout,
					Triggers:  fsEval.Triggers,
				}); err != nil {
					return slot.SpinResult{}, err
				}
			}

			if err := runHook(func(f slot.Feature, c *slot.FeatureContext) error { return f.AfterEvaluation(c) }); err != nil {
				return slot.SpinResult{}, err
			}

			// Retrigger handling
			if ctx.RetriggerSpins > 0 {
				fsState.RemainingSpins += ctx.RetriggerSpins
				fsState.AwardedSpins += ctx.RetriggerSpins
				fsState.TotalSpins += ctx.RetriggerSpins
				if err := emitEvent(slot.EventFreeSpinTrigger, fmt.Sprintf("Retriggered %d free spins", ctx.RetriggerSpins), slot.FreeSpinTriggerPayload{
					AwardedSpins: ctx.RetriggerSpins,
					TotalSpins:   fsState.TotalSpins,
					IsRetrigger:  true,
				}); err != nil {
					return slot.SpinResult{}, err
				}
				ctx.RetriggerSpins = 0
			}

			fsRoundMult := fsEval.ScatterPayout
			fsWaysMult := fsEval.TotalMultiplier
			if fsEval.ScatterPayout > 0 {
				var err error
				fsWaysMult, err = fsWaysMult.Sub(fsEval.ScatterPayout)
				if err != nil {
					return slot.SpinResult{}, err
				}
			}

			// Apply global persistent multiplier to initial free spin grid if > 1x
			if def.FreeSpins.MultiplierPolicy == slot.MultiplierPersistent && fsState.GlobalMultiplier > slot.MustMultiplier("1.0") {
				var err error
				fsWaysMult, err = fsWaysMult.Mul(fsState.GlobalMultiplier)
				if err != nil {
					return slot.SpinResult{}, err
				}
			}

			var fsCascades []slot.CascadeResult

			if !ctx.CascadeRequested {
				if fsWaysMult > 0 {
					var err error
					fsRoundMult, err = fsRoundMult.Add(fsWaysMult)
					if err != nil {
						return slot.SpinResult{}, err
					}
				}
			} else {
				// Cascades within free spin
				for ctx.CascadeRequested {
					totalSoFar, err := accumulatedBaseMult.Add(accumulatedFeatureMult)
					if err != nil {
						return slot.SpinResult{}, err
					}
					totalWithCurrentRound, err := totalSoFar.Add(fsRoundMult)
					if err != nil {
						return slot.SpinResult{}, err
					}
					if maxCap > 0 && totalWithCurrentRound >= maxCap {
						capReached = true
						if err := emitEvent(slot.EventMaxWinReached, "Max win cap reached", slot.MaxWinReachedPayload{CapMultiplier: maxCap, Policy: maxPolicy}); err != nil {
							return slot.SpinResult{}, err
						}
						if maxPolicy != slot.MaxWinCapOnly {
							break
						}
					}
					if ctx.TerminateSession || ctx.TerminateFeature {
						break
					}

					cMult := ctx.CascadeMultiplier
					if cMult <= 0 {
						cMult = slot.MustMultiplier("1.0")
					}

					currFsWays := ctx.Evaluation.TotalMultiplier
					if ctx.Evaluation.ScatterPayout > 0 {
						var err error
						currFsWays, err = currFsWays.Sub(ctx.Evaluation.ScatterPayout)
						if err != nil {
							return slot.SpinResult{}, err
						}
					}
					cWinMult, err := currFsWays.Mul(cMult)
					if err != nil {
						return slot.SpinResult{}, err
					}

					var addErr error
					fsRoundMult, addErr = fsRoundMult.Add(cWinMult)
					if addErr != nil {
						return slot.SpinResult{}, addErr
					}

					if err := sm.Transition(slot.StateCascading); err != nil {
						return slot.SpinResult{}, err
					}

					var fsRemovePositions []slot.Position
					for _, w := range ctx.Evaluation.Wins {
						fsRemovePositions = append(fsRemovePositions, w.Positions...)
					}
					if err := emitEvent(slot.EventRemoval, "Free spin winning positions removed", slot.RemovalPayload{Positions: fsRemovePositions}); err != nil {
						return slot.SpinResult{}, err
					}

					fsGridBefore := ctx.Grid.Clone()
					fsCollapsed, fsAdded, fsFinal, err := slot.CollapseAndRefill(g.vdef, ctx.ActiveReelSet, ctx.Grid, fsRemovePositions, src)
					if err != nil {
						return slot.SpinResult{}, err
					}

					totalCascades++
					ctx.CascadeIndex++
					if err := emitEvent(slot.EventCascade, "Free spin grid refilled", slot.CascadePayload{
						Index:         ctx.CascadeIndex,
						CollapsedGrid: fsCollapsed,
						NewSymbols:    fsAdded,
						FinalGrid:     fsFinal,
						Multiplier:    cMult,
					}); err != nil {
						return slot.SpinResult{}, err
					}

					if mode != slot.PlayModeSimulation {
						fsCascades = append(fsCascades, slot.CascadeResult{
							Index:            ctx.CascadeIndex,
							GridBefore:       fsGridBefore,
							Wins:             ctx.Evaluation.Wins,
							RemovedPositions: fsRemovePositions,
							CollapsedGrid:    fsCollapsed,
							NewSymbols:       fsAdded,
							FinalGrid:        fsFinal,
							Multiplier:       cMult,
							TotalMultiplier:  cWinMult,
							Triggers:         ctx.Evaluation.Triggers,
						})
					}

					ctx.Grid = fsFinal
					if err := runHook(func(f slot.Feature, c *slot.FeatureContext) error { return f.AfterCascade(c) }); err != nil {
						return slot.SpinResult{}, err
					}

					if err := sm.Transition(slot.StateEvaluating); err != nil {
						return slot.SpinResult{}, err
					}
					if err := runHook(func(f slot.Feature, c *slot.FeatureContext) error { return f.BeforeEvaluation(c) }); err != nil {
						return slot.SpinResult{}, err
					}

					nextFsEval, err := g.opts.Evaluator.Evaluate(g.vdef, ctx.Grid)
					if err != nil {
						return slot.SpinResult{}, err
					}
					ctx.Evaluation = nextFsEval

					// Contribute scatter payout from refilled free-spin grid
					if nextFsEval.ScatterPayout > 0 {
						var err error
						fsRoundMult, err = fsRoundMult.Add(nextFsEval.ScatterPayout)
						if err != nil {
							return slot.SpinResult{}, err
						}
					}

					// Emit scatter semantic event during free-spin cascade
					if nextFsEval.ScatterPayout > 0 || len(nextFsEval.Triggers) > 0 {
						if err := emitEvent(slot.EventScatterTrigger, "Free spin scatter conditions triggered", slot.ScatterTriggerPayload{
							Count:     nextFsEval.ScatterCount,
							Positions: nextFsEval.ScatterPositions,
							Payout:    nextFsEval.ScatterPayout,
							Triggers:  nextFsEval.Triggers,
						}); err != nil {
							return slot.SpinResult{}, err
						}
					}

					if len(nextFsEval.Wins) > 0 {
						if err := emitEvent(slot.EventWins, fmt.Sprintf("%d free spin winning combinations", len(nextFsEval.Wins)), slot.WinsPayload{Wins: nextFsEval.Wins}); err != nil {
							return slot.SpinResult{}, err
						}
					}

					ctx.ResetRoundDecisions()
					if err := runHook(func(f slot.Feature, c *slot.FeatureContext) error { return f.AfterEvaluation(c) }); err != nil {
						return slot.SpinResult{}, err
					}

					if ctx.RetriggerSpins > 0 {
						fsState.RemainingSpins += ctx.RetriggerSpins
						fsState.AwardedSpins += ctx.RetriggerSpins
						fsState.TotalSpins += ctx.RetriggerSpins
						if err := emitEvent(slot.EventFreeSpinTrigger, fmt.Sprintf("Retriggered %d free spins", ctx.RetriggerSpins), slot.FreeSpinTriggerPayload{
							AwardedSpins: ctx.RetriggerSpins,
							TotalSpins:   fsState.TotalSpins,
							IsRetrigger:  true,
						}); err != nil {
							return slot.SpinResult{}, err
						}
						ctx.RetriggerSpins = 0
					}
				}
			}

			fsPayout, err := slot.Settle(req.Bet, fsRoundMult)
			if err != nil {
				return slot.SpinResult{}, err
			}

			if mode != slot.PlayModeSimulation {
				freeSpinSessions = append(freeSpinSessions, slot.SpinSession{
					InitialGrid:       fsGrid.Grid,
					InitialEvaluation: fsEval,
					Cascades:          fsCascades,
					TotalMultiplier:   fsRoundMult,
					Payout:            fsPayout,
				})
			}

			var addErr error
			accumulatedFeatureMult, addErr = accumulatedFeatureMult.Add(fsRoundMult)
			if addErr != nil {
				return slot.SpinResult{}, addErr
			}
			fsState.AccumulatedWin = accumulatedFeatureMult

			// Check cap across base + feature
			totalSoFar, err := accumulatedBaseMult.Add(accumulatedFeatureMult)
			if err != nil {
				return slot.SpinResult{}, err
			}
			if maxCap > 0 && totalSoFar >= maxCap {
				capReached = true
				if err := emitEvent(slot.EventMaxWinReached, "Max win cap reached", slot.MaxWinReachedPayload{CapMultiplier: maxCap, Policy: maxPolicy}); err != nil {
					return slot.SpinResult{}, err
				}
				if maxPolicy != slot.MaxWinCapOnly {
					break
				}
			}

			if err := sm.Transition(slot.StateFreeSpins); err != nil {
				return slot.SpinResult{}, err
			}
		}
	}

	// 6. Settle
	if err := sm.Transition(slot.StateSettling); err != nil {
		return slot.SpinResult{}, err
	}

	if err := runHook(func(f slot.Feature, c *slot.FeatureContext) error { return f.AfterSpin(c) }); err != nil {
		return slot.SpinResult{}, err
	}

	totalMult, err := accumulatedBaseMult.Add(accumulatedFeatureMult)
	if err != nil {
		return slot.SpinResult{}, err
	}
	if maxCap > 0 && totalMult > maxCap {
		totalMult = maxCap
	}

	payout, err := slot.Settle(req.Bet, totalMult)
	if err != nil {
		return slot.SpinResult{}, err
	}
	netProfit := payout - req.Bet

	if err := sm.Transition(slot.StateCompleted); err != nil {
		return slot.SpinResult{}, err
	}
	if err := emitEvent(slot.EventComplete, "Spin completed", slot.CompletePayload{
		TotalMultiplier: totalMult,
		Payout:          payout,
		NetProfit:       netProfit,
	}); err != nil {
		return slot.SpinResult{}, err
	}

	var retInitialGrid slot.Grid
	if mode != slot.PlayModeSimulation {
		retInitialGrid = genGrid.Grid
	}

	return slot.SpinResult{
		GameID:            req.GameID,
		Version:           req.Version,
		Bet:               req.Bet,
		BaseMultiplier:    accumulatedBaseMult,
		FeatureMultiplier: accumulatedFeatureMult,
		TotalMultiplier:   totalMult,
		Payout:            payout,
		NetProfit:         netProfit,
		InitialGrid:       retInitialGrid,
		Cascades:          baseCascades,
		FreeSpins:         freeSpinSessions,
		FreeSpinState:     fsState,
		Triggers:          allTriggers,
		TriggeredFeatures: triggeredFeatures,
		CascadeCount:      totalCascades,
		FreeSpinCount:     totalFreeSpins,
		TerminalState:     sm.State(),
		Events:            events,
	}, nil
}
