# @qxbao/qxprob

[![CI](https://github.com/qxbao/qxprob/actions/workflows/ci.yml/badge.svg)](https://github.com/qxbao/qxprob/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/qxbao/qxprob.svg)](https://pkg.go.dev/github.com/qxbao/qxprob)

`qxprob` is a dependency-free Go library for deterministic, provably-fair game outcomes and unbiased probability primitives.

It provides:

- cryptographically secure and reproducible entropy sources;
- configurable Plinko, Mines, Duck Race, Crash, and Limbo engines;
- a modular, config-driven Slot Engine with ways wins, cascades, free spins, semantic events, and fixed-point settlement;
- one-shot seeded APIs for independently replayable rounds;
- injected-source APIs for testing and high-throughput simulations;
- generic and slot-specific Monte Carlo simulation tooling.

The module uses only the Go standard library.

> [!IMPORTANT]
> qxprob generates and evaluates mathematical outcomes. It does not manage wagers, balances, server-seed rotation, commitments, persistence, compliance, or player-facing disclosure.

## Requirements

- Go 1.22 or newer.
- No third-party runtime dependencies.

Go 1.22 is required because the simulation package ranges over an integer round count.

## Installation

```bash
go get github.com/qxbao/qxprob
```

Import only the packages you need:

```go
import (
	"github.com/qxbao/qxprob/core/entropy"
	"github.com/qxbao/qxprob/core/simulation"
	coreslot "github.com/qxbao/qxprob/core/slot"
	"github.com/qxbao/qxprob/engine/crash"
	"github.com/qxbao/qxprob/engine/duckrace"
	"github.com/qxbao/qxprob/engine/limbo"
	"github.com/qxbao/qxprob/engine/mines"
	"github.com/qxbao/qxprob/engine/plinko"
	engineslot "github.com/qxbao/qxprob/engine/slot"
	"github.com/qxbao/qxprob/engine/slotgame/reference"
	"github.com/qxbao/qxprob/slotsim"
)
```

## Packages

| Package | Purpose |
|---|---|
| `core/entropy` | Secure randomness, reproducible HMAC-SHA-256 streams, bits, floats, and unbiased bounded integers |
| `core/simulation` | Generic sequential runner for any engine exposing `Play() (T, error)` |
| `core/slot` | Strongly typed slot definitions, validation, evaluators, money, grids, states, events, and feature contracts |
| `engine/plinko` | Left/right paths, landing buckets, generated or custom payout tables |
| `engine/mines` | Unique mine placement, ordered reveals, and cash-out multipliers |
| `engine/duckrace` | Finish order and per-duck positions over time |
| `engine/crash` | Crash multiplier with house edge, instant-crash probability, and optional cap |
| `engine/limbo` | Inverse-distributed multiplier and target-based settlement |
| `engine/slot` | Server-authoritative slot orchestration and reusable compiled games |
| `engine/slotfeature` | Configurable wild, scatter, cascade, and free-spin feature modules |
| `engine/slotgame/reference` | Versioned 5x4 Reference Ways Slot definition and reel strips |
| `slotsim` | Streaming slot simulation, statistics, and text/JSON/CSV reports |
| `cmd/slot-sim` | Command-line Monte Carlo runner for versioned slot definitions |

## Quick start

The shortest way to generate a verifiable round is a game's seeded helper:

```go
package main

import (
	"fmt"
	"log"

	"github.com/qxbao/qxprob/engine/plinko"
)

func main() {
	result, err := plinko.PlaySeeded(plinko.SeededInput{
		ServerSeed: "secret-server-seed",
		ClientSeed: "player-selected-seed",
		Nonce:      42,
		Rows:       12,
		Risk:       plinko.Medium,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("path:", result.Path)
	fmt.Println("bucket:", result.BucketIndex)
	fmt.Println("multiplier:", result.Multiplier)
}
```

Running the same function again with the same seeds, nonce, configuration, and qxprob protocol version returns the same result.

## Provably-fair model

### Round inputs

A reproducible round uses three inputs:

- `ServerSeed`: secret operator-controlled input, revealed only after settlement;
- `ClientSeed`: player-visible or player-selected input;
- `Nonce`: unique round number for the current seed pair.

The deterministic stream is:

```text
HMAC-SHA-256(
  key     = serverSeed,
  message = "qxprob/entropy/v1\x00"
            || uint64_be(len(clientSeed))
            || clientSeed
            || uint64_be(nonce)
            || uint64_be(counter)
)
```

Consecutive reads consume bytes from each 32-byte HMAC block. The counter increments when the current block is exhausted.

The domain, integer widths, byte order, field order, counter behavior, and each engine's entropy call order are compatibility-sensitive. Changing any of them changes deterministic results.

### Recommended commitment lifecycle

qxprob provides replayable outcomes, not a complete commitment service. An application should:

1. Generate a high-entropy server seed using a cryptographically secure source.
2. Publish a cryptographic commitment such as `SHA-256(serverSeed)` before accepting a bet.
3. Accept or publish the client seed.
4. Assign a nonce that is never reused with the same seed pair.
5. Generate and settle the round without exposing the server seed.
6. Rotate and reveal the server seed after the applicable rounds are settled.
7. Let players verify the commitment and replay each outcome.

Never log or return an unrevealed server seed.

### One-shot versus injected-source APIs

Every game supports two styles.

Use a seeded helper for a self-contained round:

```go
result, err := limbo.PlaySeeded(limbo.SeededInput{
	ServerSeed: "server",
	ClientSeed: "client",
	Nonce:      1,
	Config:     limbo.Config{TargetMultiplier: 3},
})
```

Use an injected source when the caller owns the entropy lifecycle or runs multiple rounds:

```go
source := entropy.NewProvablyFairSource("server", "client", 1)
engine, err := limbo.New(limbo.Config{TargetMultiplier: 3}, source)
if err != nil {
	return
}

first, err := engine.Play()
if err != nil {
	return
}
second, err := engine.Play() // consumes the next value from the same stream
_, _ = first, second
```

For independent production rounds, constructing one deterministic source per round remains the simplest ownership model. A sequential owner such as the slot simulator may instead call `ResetNonce` before each round. Reusing a source across calls without resetting it consumes the same nonce's continuing byte stream and is not equivalent to incrementing the nonce.

## Entropy sources

All engines depend on the following interface:

```go
type Source interface {
	Float64() (float64, error)
	Bits(n int) (uint64, error)
	Uint64() (uint64, error)
	Intn(n uint64) (uint64, error)
}
```

### `CryptoSource`

`entropy.NewCryptoSource()` reads from `crypto/rand` and is safe for concurrent use.

```go
source := entropy.NewCryptoSource()

u, err := source.Float64() // one of 2^53 values in [0, 1)
n, err := source.Intn(100) // unbiased value in [0, 100)
bits, err := source.Bits(12)
word, err := source.Uint64()

_, _, _, _ = u, n, bits, word
_ = err
```

`Intn` uses rejection sampling rather than a raw modulus, avoiding modulo bias.

### `ProvablyFairSource`

`entropy.NewProvablyFairSource(serverSeed, clientSeed, nonce)` returns a stateful reproducible stream.

```go
a := entropy.NewProvablyFairSource("server", "client", 7)
b := entropy.NewProvablyFairSource("server", "client", 7)

av, _ := a.Uint64()
bv, _ := b.Uint64()
fmt.Println(av == bv) // true
```

Important properties:

- one instance should belong to one round or one sequential simulation;
- it is not safe for concurrent use;
- method order matters;
- `Bits` consumes whole bytes even for non-byte-aligned widths;
- the low-level constructor accepts empty seeds, so application policy must validate them;
- game-level seeded helpers reject empty server and client seeds.

For a sequential simulation, resetting a source is byte-for-byte equivalent to constructing a fresh source with the same seeds and new nonce:

```go
source := entropy.NewProvablyFairSource("server", "client", 1)

source.ResetNonce(42)
value, err := source.Uint64()
_ = value
_ = err
```

`ResetNonce` resets the counter, buffered block, and cursor. It does not make `ProvablyFairSource` safe for concurrent use.

### `SimulationSource`

`entropy.NewSimulationSource(seed1, seed2)` provides a deterministic PCG source for Monte Carlo experiments:

```go
source := entropy.NewSimulationSource(1234, 5678)
value, err := source.Intn(100)
_, _ = value, err
```

> [!WARNING]
> `SimulationSource` uses `math/rand/v2`. It is neither cryptographically secure nor provably fair and must never determine production game outcomes or payouts.

## Plinko

Plinko generates one unbiased Left/Right decision per row. The number of right decisions is the landing bucket.

```go
result, err := plinko.PlaySeeded(plinko.SeededInput{
	ServerSeed: "server",
	ClientSeed: "client",
	Nonce:      10,
	Rows:       16,
	Risk:       plinko.High,
	TargetRTP:  0.97,
})
```

### Configuration

| Field | Valid values | Zero value |
|---|---:|---|
| `Rows` | 8–16 | Invalid; required |
| `Risk` | `Low`, `Medium`, `High` | `Low` |
| `TargetRTP` | `(0, 1]` | `0.99` |
| `RiskAlpha` | positive finite value | Selected risk default |
| `Multipliers` | `Rows+1` finite non-negative values | Generate from formula |

Default risk exponents are Low `0.30`, Medium `0.60`, and High `0.90`.

Without a custom table, bucket `k` uses:

```text
p(k) = C(rows, k) / 2^rows
w(k) = p(k)^(-alpha)
Z    = sum(p(j) * w(j))
multiplier(k) = TargetRTP * w(k) / Z
```

This creates symmetric payouts whose expected multiplier equals the configured RTP before external currency rounding.

Provide `Multipliers` to replace the generated table:

```go
custom := []float64{10, 4, 2, 1, 0, 1, 2, 4, 10}

engine, err := plinko.New(plinko.Config{
	Rows:        8,
	Risk:        plinko.Low,
	Multipliers: custom,
}, entropy.NewCryptoSource())
```

The engine copies the table during construction, so later caller mutations do not change payouts.

### Result

```go
type Result struct {
	Path        []Direction
	BucketIndex int
	Multiplier  float64
}
```

Entropy consumption: exactly `Rows` calls to `Intn(2)`.

## Mines

Mines generates unique mine locations using a partial Fisher-Yates shuffle. Mine indices are sorted in the returned board for canonical replay output.

```go
board, err := mines.GenerateSeeded(mines.SeededInput{
	ServerSeed: "server",
	ClientSeed: "client",
	Nonce:      11,
	BoardSize:  25,
	MineCount:  5,
	TargetRTP:  0.99,
})
if err != nil {
	log.Fatal(err)
}

evaluation, err := board.Evaluate([]int{0, 6, 12})
if err != nil {
	log.Fatal(err)
}

fmt.Println(evaluation.Lost)
fmt.Println(evaluation.SafePicks)
fmt.Println(evaluation.Multiplier)
```

> [!WARNING]
> `Board.MineIndices` contains the complete secret layout. Keep the board server-side during play. Return only the relevant reveal information to a client, and disclose the full board only during the verification phase.

### Configuration

| Field | Valid values | Zero value |
|---|---:|---|
| `BoardSize` | 2–1024 | 25 |
| `MineCount` | 1–`BoardSize-1` | Invalid; required |
| `TargetRTP` | `(0, 1]` | `0.99` |

After `s` safe picks, the cash-out multiplier is:

```text
survival = C(BoardSize - MineCount, s) / C(BoardSize, s)
multiplier = TargetRTP / survival
```

An empty pick sequence returns `1.0`. Hitting a mine returns a zero multiplier and stops evaluation. Picks are fully validated before evaluation; duplicate and out-of-range picks return errors.

### Results

```go
type Board struct {
	TileCount  int
	TargetRTP  float64
	MineIndices []int
}

type Evaluation struct {
	Reveals    []Reveal
	Lost       bool
	SafePicks  int
	Multiplier float64
}
```

Entropy consumption: exactly `MineCount` calls to `Intn`, with bounds decreasing from `BoardSize`.

## Duck Race

Duck Race returns both an unbiased finish order and a complete monotonic position timeline suitable for UI playback.

```go
race, err := duckrace.RaceSeeded(duckrace.SeededInput{
	ServerSeed: "server",
	ClientSeed: "client",
	Nonce:      12,
	Config: duckrace.Config{
		DuckCount:      8,
		DurationMillis: 10_000,
		TickMillis:     100,
	},
})
if err != nil {
	log.Fatal(err)
}

winner := race.FinishOrder[0]
for _, frame := range race.Frames {
	updateUI(frame.ElapsedMillis, frame.Positions)
}
fmt.Println("winner:", winner)
```

### Configuration

| Field | Valid values | Zero value |
|---|---:|---|
| `DuckCount` | 2–16 | Invalid; required |
| `DurationMillis` | 1,000–60,000 | Invalid; required |
| `TickMillis` | 50–1,000 and divides duration exactly | Invalid; required |
| `BaseSegmentWeight` | positive finite value | `0.25` |
| `FinishProgress` | `(0, 1]` | `1.0` |
| `RankGap` | positive and keeps last place above zero | `0.01` |

The finish order is selected first with Fisher-Yates. Each duck then receives positive per-segment weights:

```text
weight = BaseSegmentWeight + Float64()
final position(rank) = FinishProgress - RankGap * rank
```

Weights are normalized to the duck's final position. Consequently:

- positions never decrease;
- all positions remain normalized to `[0,1]`;
- intermediate lead changes are possible;
- the last frame strictly represents `FinishOrder`;
- frame position slices are independent and safe for the caller to mutate.

Entropy consumption for `D` ducks and `S = DurationMillis/TickMillis` segments:

```text
(D - 1) Intn calls, followed by D * S Float64 calls
```

Output memory is `O(D*S)`. A coarser tick reduces entropy work, allocations, payload size, and UI update frequency.

## Crash

Crash converts one uniform float into an inverse-distributed multiplier, with an optional instant-crash region.

```go
result, err := crash.PlaySeeded(crash.SeededInput{
	ServerSeed: "server",
	ClientSeed: "client",
	Nonce:      13,
	Config: crash.Config{
		HouseEdge:               1.0,
		InstantCrashProbability: 1.0 / 33.0,
		MaxMultiplier:           1_000_000,
	},
})
```

### Configuration

| Field | Valid values | Zero value |
|---|---:|---|
| `HouseEdge` | percentage in `[0,100)` | 0% |
| `InstantCrashProbability` | `[0,1]` | Disabled |
| `MaxMultiplier` | zero or finite `>= 1` | Uncapped |

For sampled `u`:

```text
if u < InstantCrashProbability:
    multiplier = 1.0
    instantCrash = true
else:
    multiplier = max((100 - HouseEdge) / (100 * (1 - u)), 1.0)
    multiplier = min(multiplier, MaxMultiplier) // when configured
```

Entropy consumption: exactly one `Float64` call.

## Limbo

Limbo generates an inverse-distributed outcome and settles it against a player-selected target.

```go
result, err := limbo.PlaySeeded(limbo.SeededInput{
	ServerSeed: "server",
	ClientSeed: "client",
	Nonce:      14,
	Config: limbo.Config{
		RTP:              0.99,
		MinMultiplier:    1,
		MaxMultiplier:    1_000_000,
		PayoutStep:       0.01,
		TargetMultiplier: 5,
	},
})

fmt.Println(result.Multiplier)
fmt.Println(result.Won)
fmt.Println(result.PayoutMultiplier)
```

### Configuration

| Field | Valid values | Zero value |
|---|---:|---|
| `RTP` | `(0,1]` | `0.99` |
| `MinMultiplier` | finite `>= 1` | `1.0` |
| `MaxMultiplier` | finite and `>= MinMultiplier` | `1,000,000` |
| `PayoutStep` | `(0, MinMultiplier]` | `0.01` |
| `TargetMultiplier` | `[MinMultiplier, MaxMultiplier]` | `2.0` |

For sampled `u`:

```text
if u == 0:
    multiplier = MaxMultiplier
else:
    raw = RTP / u
    multiplier = floor(raw / PayoutStep) * PayoutStep
    multiplier = clamp(multiplier, MinMultiplier, MaxMultiplier)

won = multiplier >= TargetMultiplier
payoutMultiplier = TargetMultiplier if won, otherwise 0
```

Rounding occurs before clamping. A target exactly equal to the outcome wins.

Entropy consumption: exactly one `Float64` call.

## Slot Engine

The Slot Engine separates versioned math configuration from entropy, settlement, presentation events, and UI timing. A result is generated and fully evaluated before a frontend consumes its semantic event stream.

The reference game demonstrates:

- five reels by four visible rows and 1,024 ways;
- left-to-right wins starting at three consecutive reels;
- configurable regular, wild, and scatter symbols without hard-coded symbol IDs;
- reel-strip sampling with wrap-around;
- cascades with a configurable multiplier table;
- retriggerable free spins using a separate reel set;
- fixed-point bet and payout accounting;
- a configurable 10,000x maximum-win policy;
- normal play with presentation events and an allocation-reduced simulation mode.

### Compile once, spin many rounds

Compile and validate immutable game math once, then inject the round's entropy source into `Game.Spin`:

```go
def := reference.Definition()
validated, err := coreslot.ValidateDefinition(def)
if err != nil {
	log.Fatal(err)
}

game, err := engineslot.Compile(
	def,
	reference.Features(validated),
	engineslot.Options{Mode: coreslot.PlayModeSimulation},
)
if err != nil {
	log.Fatal(err)
}

source := entropy.NewProvablyFairSource("server", "client", 1)
for nonce := uint64(1); nonce <= 1_000; nonce++ {
	source.ResetNonce(nonce)
	result, err := game.Spin(coreslot.SpinRequest{
		GameID:  reference.GameID,
		Version: reference.ConfigVersion,
		Bet:     coreslot.Amount(100), // minor currency units
		Mode:    coreslot.PlayModeSimulation,
	}, source)
	if err != nil {
		log.Fatal(err)
	}
	consume(result)
}
```

`Compile` deep-copies and validates the definition, resolves the evaluator, orders registered feature modules, and consumes no entropy. The resulting `Game` retains no nonce, grid, or round state. It may be reused concurrently when each goroutine supplies its own entropy source and custom evaluators/features are themselves concurrency-safe.

The legacy `slot.New(def, source, features, options)` and bound `Engine.Spin` API remains available for compatibility. Prefer `Compile` and `Game.Spin` for servers and high-volume simulations so validation and feature assembly do not repeat for every nonce. `PlaySeeded` remains useful for a self-contained, replayable round.

### Configuration and extension points

`core/slot.Definition` owns the immutable game ID/version, grid shape, symbols, reel sets, win mechanic, cascades, free spins, maximum win, and math profile. Feature behavior is supplied through ordered `Feature` hooks; presentation code receives semantic events and never chooses reel stops, wins, multipliers, or payouts.

The built-in evaluator implements Ways to Win without assuming five reels or fixed symbol names. Alternative evaluator implementations can add paylines, clusters, or other mechanics through dependency injection without coupling math to rendering.

Money uses `core/slot.Amount`, an integer count of minor currency units. Multipliers use the slot package's fixed precision; application code remains responsible for wallets, balance validation, idempotent settlement, and persistence.

## Simulation

Every game engine implements `simulation.Engine[T]`:

```go
type Engine[T any] interface {
	Play() (T, error)
}
```

Use `simulation.Run` to execute sequential rounds without storing every result:

```go
ctx := context.Background()
source := entropy.NewCryptoSource()

engine, err := crash.New(crash.Config{
	HouseEdge:     1,
	MaxMultiplier: 1_000,
}, source)
if err != nil {
	log.Fatal(err)
}

err = simulation.Run(ctx, engine, 1_000_000, func(round uint64, result crash.Result) error {
	consume(round, result)
	return nil
})
if err != nil {
	log.Fatal(err)
}
```

The runner:

- validates the context, engine, and observer;
- executes rounds sequentially;
- checks cancellation before every round;
- streams each result to the observer;
- wraps engine, observer, and context errors with the round index.

It does not start goroutines or retain results.

### Slot simulation

Use `slotsim.RunWithRunner` to reuse both a compiled game and resettable entropy source while streaming aggregate statistics:

```go
source := entropy.NewProvablyFairSource("simulation-server", "simulation-client", 1)

report, err := slotsim.RunWithRunner(ctx, slotsim.Options{
	Spins: 1_000_000,
	Bet:   coreslot.Amount(100),
}, func(nonce uint64, req coreslot.SpinRequest) (coreslot.SpinResult, error) {
	source.ResetNonce(nonce)
	return game.Spin(req, source)
})
if err != nil {
	log.Fatal(err)
}

fmt.Printf("RTP: %.4f%%\n", report.TotalRTP*100)
```

The command-line tool supports deterministic text, JSON, and CSV output:

```bash
go run ./cmd/slot-sim \
  --game reference-ways-slot \
  --version 1.0.0 \
  --spins 1000000 \
  --bet 100 \
  --format json \
  --output report.json
```

The reference paytable and reel strips are initial calibration data, not a production-certified math model. Measure RTP, volatility, feature frequency, distribution tails, and maximum wins over a statistically meaningful sample before deployment.

## Validation and errors

Constructors validate configuration before consuming entropy. Game packages expose sentinel errors for invalid inputs, so callers can use `errors.Is`:

```go
engine, err := plinko.New(
	plinko.Config{Rows: 7, Risk: plinko.Low},
	entropy.NewCryptoSource(),
)
if errors.Is(err, plinko.ErrInvalidRows) {
	// Return a validation response to the caller.
}
_ = engine
```

Entropy dependency errors are wrapped with `%w` and remain discoverable:

```go
if errors.Is(err, storageOrDeviceError) {
	// Handle the underlying failure.
}
```

The entropy package exposes `ErrInvalidBitCount`, `ErrInvalidBound`, and `ErrEntropyExhausted`, so callers can inspect validation and stream-exhaustion failures with `errors.Is`.

## Determinism and compatibility

The following behavior is part of deterministic replay:

| Component | Replay-sensitive behavior |
|---|---|
| Entropy | HMAC domain, seed encoding, nonce/counter widths, byte order, cursor behavior |
| Plinko | `Rows` ordered calls to `Intn(2)` and payout configuration |
| Mines | Partial-shuffle order, decreasing bounds, sorted output, board size and RTP |
| Duck Race | Finish shuffle first, then floats ordered by duck and segment, timeline configuration |
| Crash | One float, formula, instant-crash comparison, floor, and cap |
| Limbo | One float, inverse formula, payout step, clamp order, and target comparison |
| Slot | Immutable game/config version, reel strips, feature order, evaluator rules, entropy calls, and maximum-win policy |
| Entropy reset | `ResetNonce(n)` produces the same stream as a fresh source constructed with the same seeds and nonce `n` |

To reproduce an outcome, retain:

- qxprob module version;
- server seed and its prior commitment;
- client seed;
- nonce;
- complete game configuration;
- result or result hash for comparison.

Do not change deterministic-vector expectations merely to make a test pass. A deliberate protocol change should use an explicit new version and retain old verification support where required.

## Concurrency

- `CryptoSource` is safe for concurrent use.
- `ProvablyFairSource` is stateful and not safe for concurrent use.
- Engines hold their source and do not synchronize access.
- Use one engine/source per goroutine for deterministic rounds.
- A compiled slot `Game` is reusable; each concurrent caller must provide its own entropy source.
- Custom slot features and evaluators must be concurrency-safe when shared through a compiled game.
- `simulation.Run` is deliberately sequential.
- Returned Plinko paths and Duck Race frame slices are newly allocated for each result.

## Security considerations

- Generate production server seeds with a CSPRNG.
- Never use `math/rand` for game outcomes.
- Never use `SimulationSource` for production outcomes, verification, or settlement.
- Never reuse a nonce with the same server/client seed pair.
- Commit to the server seed before accepting play and reveal it only after settlement.
- Do not send `mines.Board.MineIndices` to an active player.
- Keep money calculations and wallet state outside this floating-point probability layer.
- Decide currency rounding and payout limits explicitly at the settlement boundary.
- Treat configuration and engine version as part of the signed/audited round record.
- Run supported Go toolchains in production even though the module's compatibility floor is Go 1.22.

Provably fair means an outcome can be replayed and verified from committed inputs. It does not by itself prove correct application code, secure seed custody, regulatory compliance, or solvent settlement.

## Performance notes

- Injected engines avoid recreating a source and are useful for simulations.
- Seeded helpers intentionally create one HMAC stream per round.
- Plinko precomputes its payout table during construction.
- Mines allocates storage proportional to board size.
- Duck Race work and output scale with duck count multiplied by frame count.
- Crash and Limbo consume one float per round.
- Compiled slot games avoid rebuilding and validating the full definition for every spin.
- `PlayModeSimulation` suppresses presentation event payloads that Monte Carlo runs do not need.
- Benchmark your actual configuration before choosing UI tick rates or simulation batch sizes.

In a local paired benchmark of the reference slot lifecycle, compile-once spins averaged about `23.18 us/op`, `20.8 KB/op`, and `282 allocs/op`, compared with about `37.55 us/op`, `36.2 KB/op`, and `391 allocs/op` for rebuilding the legacy engine per nonce. These figures describe one machine and configuration, not an API performance guarantee. The PCG-backed `SimulationSource` is optional and did not improve that end-to-end reference workload because different entropy streams can produce different cascade paths.

Run package benchmarks with:

```bash
go test -bench=. -benchmem ./engine/...
```

## Development

```bash
gofmt -w core engine
go test ./...
go test -race ./...
go vet ./...
```

Test the declared compatibility floor with an actual Go 1.22 toolchain, not only a newer compiler using the `go 1.22` language mode.

## Project layout

```text
qxprob/
├── core/
│   ├── entropy/       # secure and reproducible random sources
│   ├── simulation/    # generic sequential runner
│   └── slot/          # slot definitions, evaluators, money, state, and events
├── engine/
│   ├── crash/
│   ├── duckrace/
│   ├── limbo/
│   ├── mines/
│   ├── plinko/
│   ├── slot/          # reusable compiled slot lifecycle
│   ├── slotfeature/   # built-in feature modules
│   └── slotgame/
│       └── reference/ # Reference Ways Slot configuration
├── slotsim/           # slot Monte Carlo statistics and exporters
├── cmd/slot-sim/      # simulation CLI
└── docs/spec/         # implementation contracts and design decisions
```

## FAQ

### Does qxprob publish or verify server-seed commitments?

No. It provides deterministic entropy and game replay. Commitment publication, rotation, storage, and disclosure belong to the application.

### Are seeded helpers compatible with another casino's verifier?

Not necessarily. qxprob uses its own versioned HMAC message format and game conversion rules. Verify qxprob rounds with the same qxprob protocol version.

### Can one deterministic source be shared by multiple goroutines?

No. Construct one `ProvablyFairSource` per round and keep it owned by one goroutine.

### Can a deterministic source be reused across slot simulation rounds?

Yes, when exactly one sequential owner calls `ResetNonce` before every round. This produces the same bytes as constructing a fresh source for that nonce while avoiding per-round source setup. Do not reset or read the same source concurrently.

### Why do optional configuration fields use zero as a default?

This keeps existing composite literals source-compatible as configuration grows. It also means zero cannot always express a literal policy value; consult each configuration table.

### Why are monetary amounts not included?

The library operates on probabilities and multipliers. Currency precision, rounding, limits, accounting, and settlement policies vary by application and should use an appropriate fixed-point or decimal representation outside qxprob.

### Is a Mines board safe to return to a browser?

Only after the round is settled or when intentionally revealing it for verification. During active play, expose individual reveal results rather than the complete mine layout.

## License

qxprob is released under the [MIT License](LICENSE).
