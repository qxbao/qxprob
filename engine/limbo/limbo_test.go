package limbo_test

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/qxbao/qxprob/core/entropy"
	"github.com/qxbao/qxprob/core/simulation"
	"github.com/qxbao/qxprob/engine/limbo"
)

// fakeEntropySource is a deterministic fake implementing entropy.Source for tests.
type fakeEntropySource struct {
	values    []float64
	index     int
	err       error
	readCount int
}

func (s *fakeEntropySource) Float64() (float64, error) {
	s.readCount++
	if s.err != nil {
		return 0, s.err
	}
	if s.index >= len(s.values) {
		return 0, errors.New("fake: no more values")
	}
	v := s.values[s.index]
	s.index++
	return v, nil
}

func (s *fakeEntropySource) Bits(int) (uint64, error) {
	return 0, errors.New("fake: Bits not implemented")
}

func (s *fakeEntropySource) Uint64() (uint64, error) {
	return 0, errors.New("fake: Uint64 not implemented")
}

func (s *fakeEntropySource) Intn(uint64) (uint64, error) {
	return 0, errors.New("fake: Intn not implemented")
}

var _ entropy.Source = (*fakeEntropySource)(nil)

// TestEngineInterfaceCompatibility verifies that *limbo.Engine satisfies simulation.Engine[limbo.Result].
func TestEngineInterfaceCompatibility(t *testing.T) {
	fake := &fakeEntropySource{values: []float64{0.5}}
	eng, err := limbo.New(limbo.Config{}, fake)
	if err != nil {
		t.Fatalf("unexpected construction failure: %v", err)
	}
	var _ simulation.Engine[limbo.Result] = eng
}

// TestConstructorValidation verifies that New rejects invalid configurations and nil sources
// with documented sentinel errors and without consuming entropy.
func TestConstructorValidation(t *testing.T) {
	fake := &fakeEntropySource{values: []float64{0.5}}

	tests := []struct {
		name    string
		config  limbo.Config
		source  entropy.Source
		wantErr error
	}{
		{
			name:    "nil source",
			config:  limbo.Config{},
			source:  nil,
			wantErr: limbo.ErrNilSource,
		},
		{
			name:    "negative RTP",
			config:  limbo.Config{RTP: -0.1},
			source:  fake,
			wantErr: limbo.ErrInvalidRTP,
		},
		{
			name:    "RTP greater than 1",
			config:  limbo.Config{RTP: 1.05},
			source:  fake,
			wantErr: limbo.ErrInvalidRTP,
		},
		{
			name:    "NaN RTP",
			config:  limbo.Config{RTP: math.NaN()},
			source:  fake,
			wantErr: limbo.ErrInvalidRTP,
		},
		{
			name:    "Inf RTP",
			config:  limbo.Config{RTP: math.Inf(1)},
			source:  fake,
			wantErr: limbo.ErrInvalidRTP,
		},
		{
			name:    "MinMultiplier less than 1",
			config:  limbo.Config{MinMultiplier: 0.99},
			source:  fake,
			wantErr: limbo.ErrInvalidMinMultiplier,
		},
		{
			name:    "negative MinMultiplier",
			config:  limbo.Config{MinMultiplier: -1.0},
			source:  fake,
			wantErr: limbo.ErrInvalidMinMultiplier,
		},
		{
			name:    "NaN MinMultiplier",
			config:  limbo.Config{MinMultiplier: math.NaN()},
			source:  fake,
			wantErr: limbo.ErrInvalidMinMultiplier,
		},
		{
			name:    "Inf MinMultiplier",
			config:  limbo.Config{MinMultiplier: math.Inf(1)},
			source:  fake,
			wantErr: limbo.ErrInvalidMinMultiplier,
		},
		{
			name:    "MaxMultiplier less than MinMultiplier",
			config:  limbo.Config{MinMultiplier: 2.0, MaxMultiplier: 1.5},
			source:  fake,
			wantErr: limbo.ErrInvalidMaxMultiplier,
		},
		{
			name:    "negative MaxMultiplier",
			config:  limbo.Config{MaxMultiplier: -10.0},
			source:  fake,
			wantErr: limbo.ErrInvalidMaxMultiplier,
		},
		{
			name:    "NaN MaxMultiplier",
			config:  limbo.Config{MaxMultiplier: math.NaN()},
			source:  fake,
			wantErr: limbo.ErrInvalidMaxMultiplier,
		},
		{
			name:    "Inf MaxMultiplier",
			config:  limbo.Config{MaxMultiplier: math.Inf(1)},
			source:  fake,
			wantErr: limbo.ErrInvalidMaxMultiplier,
		},
		{
			name:    "negative PayoutStep",
			config:  limbo.Config{PayoutStep: -0.01},
			source:  fake,
			wantErr: limbo.ErrInvalidPayoutStep,
		},
		{
			name:    "PayoutStep greater than MinMultiplier",
			config:  limbo.Config{MinMultiplier: 1.0, PayoutStep: 1.5},
			source:  fake,
			wantErr: limbo.ErrInvalidPayoutStep,
		},
		{
			name:    "NaN PayoutStep",
			config:  limbo.Config{PayoutStep: math.NaN()},
			source:  fake,
			wantErr: limbo.ErrInvalidPayoutStep,
		},
		{
			name:    "Inf PayoutStep",
			config:  limbo.Config{PayoutStep: math.Inf(1)},
			source:  fake,
			wantErr: limbo.ErrInvalidPayoutStep,
		},
		{
			name:    "TargetMultiplier less than MinMultiplier",
			config:  limbo.Config{MinMultiplier: 2.0, TargetMultiplier: 1.5},
			source:  fake,
			wantErr: limbo.ErrInvalidTargetMultiplier,
		},
		{
			name:    "TargetMultiplier greater than MaxMultiplier",
			config:  limbo.Config{MaxMultiplier: 10.0, TargetMultiplier: 10.5},
			source:  fake,
			wantErr: limbo.ErrInvalidTargetMultiplier,
		},
		{
			name:    "TargetMultiplier zero with MinMultiplier greater than default target",
			config:  limbo.Config{MinMultiplier: 3.0, TargetMultiplier: 0.0},
			source:  fake,
			wantErr: limbo.ErrInvalidTargetMultiplier,
		},
		{
			name:    "NaN TargetMultiplier",
			config:  limbo.Config{TargetMultiplier: math.NaN()},
			source:  fake,
			wantErr: limbo.ErrInvalidTargetMultiplier,
		},
		{
			name:    "Inf TargetMultiplier",
			config:  limbo.Config{TargetMultiplier: math.Inf(1)},
			source:  fake,
			wantErr: limbo.ErrInvalidTargetMultiplier,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake.readCount = 0
			_, err := limbo.New(tt.config, tt.source)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("New() error = %v, want %v", err, tt.wantErr)
			}
			if fake.readCount != 0 {
				t.Fatalf("entropy consumed during failed New(): readCount = %d", fake.readCount)
			}
		})
	}
}

// TestDefaultConfig verifies that zero-value fields resolve to the documented defaults.
func TestDefaultConfig(t *testing.T) {
	fake := &fakeEntropySource{values: []float64{0.495}}
	eng, err := limbo.New(limbo.Config{}, fake)
	if err != nil {
		t.Fatalf("unexpected New() error: %v", err)
	}

	res, err := eng.Play()
	if err != nil {
		t.Fatalf("unexpected Play() error: %v", err)
	}

	// Default RTP 0.99 / 0.495 = 2.0
	// Default TargetMultiplier = 2.0
	// Won = true, PayoutMultiplier = 2.0
	if res.Multiplier != 2.0 {
		t.Errorf("Multiplier = %v, want 2.0", res.Multiplier)
	}
	if res.TargetMultiplier != limbo.DefaultTargetMultiplier {
		t.Errorf("TargetMultiplier = %v, want %v", res.TargetMultiplier, limbo.DefaultTargetMultiplier)
	}
	if !res.Won {
		t.Errorf("Won = false, want true")
	}
	if res.PayoutMultiplier != 2.0 {
		t.Errorf("PayoutMultiplier = %v, want 2.0", res.PayoutMultiplier)
	}
}

// TestFormulaAndRounding verifies the inverse formula, round-down step, u==0, and clamping.
func TestFormulaAndRounding(t *testing.T) {
	tests := []struct {
		name             string
		config           limbo.Config
		u                float64
		wantMultiplier   float64
		wantWon          bool
		wantPayout       float64
		targetMultiplier float64
	}{
		{
			name:             "u is zero returns MaxMultiplier directly",
			config:           limbo.Config{MaxMultiplier: 1_000_000, TargetMultiplier: 100},
			u:                0.0,
			wantMultiplier:   1_000_000.0,
			wantWon:          true,
			wantPayout:       100.0,
			targetMultiplier: 100.0,
		},
		{
			name:             "exact division without remainder",
			config:           limbo.Config{RTP: 0.99, PayoutStep: 0.01, TargetMultiplier: 2.0},
			u:                0.5,
			wantMultiplier:   1.98,
			wantWon:          false,
			wantPayout:       0.0,
			targetMultiplier: 2.0,
		},
		{
			name:             "round down floor step",
			config:           limbo.Config{RTP: 0.99, PayoutStep: 0.01, TargetMultiplier: 3.0},
			u:                0.329, // 0.99 / 0.329 = 3.0091185... -> 3.00
			wantMultiplier:   3.00,
			wantWon:          true,
			wantPayout:       3.0,
			targetMultiplier: 3.0,
		},
		{
			name:             "clamp to MinMultiplier when raw is below MinMultiplier",
			config:           limbo.Config{RTP: 0.99, MinMultiplier: 1.0, PayoutStep: 0.01, TargetMultiplier: 2.0},
			u:                0.999, // 0.99 / 0.999 = 0.99099... -> 0.99 < 1.0
			wantMultiplier:   1.0,
			wantWon:          false,
			wantPayout:       0.0,
			targetMultiplier: 2.0,
		},
		{
			name:             "clamp to MaxMultiplier when raw exceeds MaxMultiplier",
			config:           limbo.Config{RTP: 0.99, MaxMultiplier: 50.0, TargetMultiplier: 20.0},
			u:                0.001, // 0.99 / 0.001 = 990.0 > 50.0
			wantMultiplier:   50.0,
			wantWon:          true,
			wantPayout:       20.0,
			targetMultiplier: 20.0,
		},
		{
			name:             "custom payout step 0.05",
			config:           limbo.Config{RTP: 0.99, PayoutStep: 0.05, TargetMultiplier: 1.95},
			u:                0.5, // 0.99 / 0.5 = 1.98 -> floor(1.98/0.05)*0.05 = floor(39.6)*0.05 = 39*0.05 = 1.95
			wantMultiplier:   1.95,
			wantWon:          true,
			wantPayout:       1.95,
			targetMultiplier: 1.95,
		},
		{
			name:             "custom MinMultiplier clamping",
			config:           limbo.Config{RTP: 0.99, MinMultiplier: 5.0, MaxMultiplier: 100.0, PayoutStep: 0.01, TargetMultiplier: 5.0},
			u:                0.5, // 0.99 / 0.5 = 1.98 < 5.0 -> clamped to 5.0
			wantMultiplier:   5.0,
			wantWon:          true,
			wantPayout:       5.0,
			targetMultiplier: 5.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeEntropySource{values: []float64{tt.u}}
			eng, err := limbo.New(tt.config, fake)
			if err != nil {
				t.Fatalf("unexpected New() error: %v", err)
			}
			res, err := eng.Play()
			if err != nil {
				t.Fatalf("unexpected Play() error: %v", err)
			}
			if math.Abs(res.Multiplier-tt.wantMultiplier) > 1e-9 {
				t.Errorf("Multiplier = %v, want %v", res.Multiplier, tt.wantMultiplier)
			}
			if res.Won != tt.wantWon {
				t.Errorf("Won = %v, want %v", res.Won, tt.wantWon)
			}
			if math.Abs(res.PayoutMultiplier-tt.wantPayout) > 1e-9 {
				t.Errorf("PayoutMultiplier = %v, want %v", res.PayoutMultiplier, tt.wantPayout)
			}
			if res.TargetMultiplier != tt.targetMultiplier {
				t.Errorf("TargetMultiplier = %v, want %v", res.TargetMultiplier, tt.targetMultiplier)
			}
		})
	}
}

// TestTargetSettlement verifies win/loss boundary conditions including ties.
func TestTargetSettlement(t *testing.T) {
	cfg := limbo.Config{RTP: 0.99, TargetMultiplier: 2.0}

	// Case 1: Exact tie: 0.99 / 0.495 = 2.00
	t.Run("exact tie is a win", func(t *testing.T) {
		fake := &fakeEntropySource{values: []float64{0.495}}
		eng, err := limbo.New(cfg, fake)
		if err != nil {
			t.Fatal(err)
		}
		res, err := eng.Play()
		if err != nil {
			t.Fatal(err)
		}
		if !res.Won {
			t.Errorf("expected Won == true for tie, got false")
		}
		if res.PayoutMultiplier != 2.0 {
			t.Errorf("PayoutMultiplier = %v, want 2.0", res.PayoutMultiplier)
		}
	})

	// Case 2: Multiplier strictly above target: 0.99 / 0.33 = 3.00 > 2.00
	t.Run("outcome strictly above target is a win", func(t *testing.T) {
		fake := &fakeEntropySource{values: []float64{0.33}}
		eng, err := limbo.New(cfg, fake)
		if err != nil {
			t.Fatal(err)
		}
		res, err := eng.Play()
		if err != nil {
			t.Fatal(err)
		}
		if !res.Won {
			t.Errorf("expected Won == true, got false")
		}
		if res.PayoutMultiplier != 2.0 {
			t.Errorf("PayoutMultiplier = %v, want 2.0", res.PayoutMultiplier)
		}
	})

	// Case 3: Multiplier strictly below target: 0.99 / 0.50 = 1.98 < 2.00
	t.Run("outcome strictly below target is a loss", func(t *testing.T) {
		fake := &fakeEntropySource{values: []float64{0.50}}
		eng, err := limbo.New(cfg, fake)
		if err != nil {
			t.Fatal(err)
		}
		res, err := eng.Play()
		if err != nil {
			t.Fatal(err)
		}
		if res.Won {
			t.Errorf("expected Won == false, got true")
		}
		if res.PayoutMultiplier != 0.0 {
			t.Errorf("PayoutMultiplier = %v, want 0.0", res.PayoutMultiplier)
		}
	})
}

// TestEntropyValidation verifies that Play rejects out-of-range floats, NaNs, Infs, and wrapped errors.
func TestEntropyValidation(t *testing.T) {
	invalidFloats := []float64{
		-0.0001,
		-1.0,
		1.0,
		1.0001,
		2.0,
		math.NaN(),
		math.Inf(1),
		math.Inf(-1),
	}

	for _, val := range invalidFloats {
		t.Run("invalid float", func(t *testing.T) {
			fake := &fakeEntropySource{values: []float64{val}}
			eng, err := limbo.New(limbo.Config{}, fake)
			if err != nil {
				t.Fatalf("unexpected New() error: %v", err)
			}
			_, err = eng.Play()
			if !errors.Is(err, limbo.ErrInvalidEntropyFloat) {
				t.Fatalf("Play() error = %v, want %v", err, limbo.ErrInvalidEntropyFloat)
			}
		})
	}

	t.Run("source dependency failure is wrapped", func(t *testing.T) {
		cause := errors.New("entropy stream exhausted")
		fake := &fakeEntropySource{err: cause}
		eng, err := limbo.New(limbo.Config{}, fake)
		if err != nil {
			t.Fatalf("unexpected New() error: %v", err)
		}
		_, err = eng.Play()
		if !errors.Is(err, cause) {
			t.Fatalf("Play() error = %v, want wrapped cause %v", err, cause)
		}
	})
}

// TestRepeatedRounds verifies that successive Play calls consume one float per round
// and produce independent, correct results.
func TestRepeatedRounds(t *testing.T) {
	fake := &fakeEntropySource{
		values: []float64{0.5, 0.33, 0.495},
	}
	eng, err := limbo.New(limbo.Config{TargetMultiplier: 2.0}, fake)
	if err != nil {
		t.Fatalf("unexpected New() error: %v", err)
	}

	// Round 1: 0.5 -> 1.98 (loss)
	res1, err := eng.Play()
	if err != nil {
		t.Fatalf("round 1 error: %v", err)
	}
	if res1.Won || math.Abs(res1.Multiplier-1.98) > 1e-9 {
		t.Errorf("round 1 result mismatch: %+v", res1)
	}

	// Round 2: 0.33 -> 3.00 (win)
	res2, err := eng.Play()
	if err != nil {
		t.Fatalf("round 2 error: %v", err)
	}
	if !res2.Won || math.Abs(res2.Multiplier-3.00) > 1e-9 {
		t.Errorf("round 2 result mismatch: %+v", res2)
	}

	// Round 3: 0.495 -> 2.00 (win tie)
	res3, err := eng.Play()
	if err != nil {
		t.Fatalf("round 3 error: %v", err)
	}
	if !res3.Won || math.Abs(res3.Multiplier-2.00) > 1e-9 {
		t.Errorf("round 3 result mismatch: %+v", res3)
	}

	if fake.readCount != 3 {
		t.Errorf("fake.readCount = %d, want 3", fake.readCount)
	}
}

// TestSimulationRunnerIntegration verifies that limbo.Engine works seamlessly with simulation.Run.
func TestSimulationRunnerIntegration(t *testing.T) {
	fake := &fakeEntropySource{
		values: []float64{0.5, 0.33, 0.495, 0.1, 0.2},
	}
	eng, err := limbo.New(limbo.Config{}, fake)
	if err != nil {
		t.Fatalf("unexpected New() error: %v", err)
	}

	var observedRounds []uint64
	var observedResults []limbo.Result

	err = simulation.Run(context.Background(), eng, 5, func(round uint64, res limbo.Result) error {
		observedRounds = append(observedRounds, round)
		observedResults = append(observedResults, res)
		return nil
	})
	if err != nil {
		t.Fatalf("simulation.Run error: %v", err)
	}

	if len(observedRounds) != 5 {
		t.Fatalf("observed %d rounds, want 5", len(observedRounds))
	}
	for i, r := range observedRounds {
		if r != uint64(i) {
			t.Errorf("round index mismatch: got %d, want %d", r, i)
		}
	}
}

// TestPlaySeededValidation verifies that PlaySeeded validates seeds and configuration.
func TestPlaySeededValidation(t *testing.T) {
	validConfig := limbo.Config{}

	tests := []struct {
		name    string
		input   limbo.SeededInput
		wantErr error
	}{
		{
			name: "empty server seed",
			input: limbo.SeededInput{
				ServerSeed: "",
				ClientSeed: "client",
				Nonce:      1,
				Config:     validConfig,
			},
			wantErr: limbo.ErrEmptyServerSeed,
		},
		{
			name: "empty client seed",
			input: limbo.SeededInput{
				ServerSeed: "server",
				ClientSeed: "",
				Nonce:      1,
				Config:     validConfig,
			},
			wantErr: limbo.ErrEmptyClientSeed,
		},
		{
			name: "invalid config in seeded input",
			input: limbo.SeededInput{
				ServerSeed: "server",
				ClientSeed: "client",
				Nonce:      1,
				Config:     limbo.Config{RTP: -1.0},
			},
			wantErr: limbo.ErrInvalidRTP,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := limbo.PlaySeeded(tt.input)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("PlaySeeded() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// TestPlaySeededDeterminismAndReplay verifies that PlaySeeded produces deterministic,
// reproducible results for identical inputs.
func TestPlaySeededDeterminismAndReplay(t *testing.T) {
	input := limbo.SeededInput{
		ServerSeed: "sample-server-seed",
		ClientSeed: "sample-client-seed",
		Nonce:      42,
		Config:     limbo.Config{},
	}

	res1, err := limbo.PlaySeeded(input)
	if err != nil {
		t.Fatalf("unexpected error on res1: %v", err)
	}

	res2, err := limbo.PlaySeeded(input)
	if err != nil {
		t.Fatalf("unexpected error on res2: %v", err)
	}

	if res1 != res2 {
		t.Fatalf("results differ: res1 = %+v, res2 = %+v", res1, res2)
	}
}

// TestPlaySeededSeparation verifies that changing any seed or nonce changes the outcome.
func TestPlaySeededSeparation(t *testing.T) {
	base := limbo.SeededInput{
		ServerSeed: "server-seed-alpha",
		ClientSeed: "client-seed-beta",
		Nonce:      100,
		Config:     limbo.Config{},
	}

	baseRes, err := limbo.PlaySeeded(base)
	if err != nil {
		t.Fatalf("base run error: %v", err)
	}

	// Change ServerSeed
	diffServer := base
	diffServer.ServerSeed = "server-seed-different"
	resDiffServer, err := limbo.PlaySeeded(diffServer)
	if err != nil {
		t.Fatalf("diffServer error: %v", err)
	}
	if resDiffServer.Multiplier == baseRes.Multiplier {
		t.Errorf("changing ServerSeed did not change multiplier: %v", baseRes.Multiplier)
	}

	// Change ClientSeed
	diffClient := base
	diffClient.ClientSeed = "client-seed-different"
	resDiffClient, err := limbo.PlaySeeded(diffClient)
	if err != nil {
		t.Fatalf("diffClient error: %v", err)
	}
	if resDiffClient.Multiplier == baseRes.Multiplier {
		t.Errorf("changing ClientSeed did not change multiplier: %v", baseRes.Multiplier)
	}

	// Change Nonce
	diffNonce := base
	diffNonce.Nonce = 101
	resDiffNonce, err := limbo.PlaySeeded(diffNonce)
	if err != nil {
		t.Fatalf("diffNonce error: %v", err)
	}
	if resDiffNonce.Multiplier == baseRes.Multiplier {
		t.Errorf("changing Nonce did not change multiplier: %v", baseRes.Multiplier)
	}
}

// TestPlaySeededDeterministicVector pins a literal deterministic result vector.
func TestPlaySeededDeterministicVector(t *testing.T) {
	input := limbo.SeededInput{
		ServerSeed: "limbo-spec-server-seed",
		ClientSeed: "limbo-spec-client-seed",
		Nonce:      1,
		Config: limbo.Config{
			RTP:              0.99,
			MinMultiplier:    1.0,
			MaxMultiplier:    1_000_000.0,
			PayoutStep:       0.01,
			TargetMultiplier: 2.0,
		},
	}

	res, err := limbo.PlaySeeded(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const (
		wantMultiplier       = 2.09
		wantTargetMultiplier = 2.0
		wantWon              = true
		wantPayoutMultiplier = 2.0
	)

	if math.Abs(res.Multiplier-wantMultiplier) > 1e-9 {
		t.Errorf("Multiplier = %v, want %v", res.Multiplier, wantMultiplier)
	}
	if res.TargetMultiplier != wantTargetMultiplier {
		t.Errorf("TargetMultiplier = %v, want %v", res.TargetMultiplier, wantTargetMultiplier)
	}
	if res.Won != wantWon {
		t.Errorf("Won = %v, want %v", res.Won, wantWon)
	}
	if math.Abs(res.PayoutMultiplier-wantPayoutMultiplier) > 1e-9 {
		t.Errorf("PayoutMultiplier = %v, want %v", res.PayoutMultiplier, wantPayoutMultiplier)
	}
}

func BenchmarkEnginePlay(b *testing.B) {
	source := entropy.NewCryptoSource()
	eng, err := limbo.New(limbo.Config{}, source)
	if err != nil {
		b.Fatalf("New() failed: %v", err)
	}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = eng.Play()
	}
}

func BenchmarkPlaySeeded(b *testing.B) {
	input := limbo.SeededInput{
		ServerSeed: "benchmark-server-seed",
		ClientSeed: "benchmark-client-seed",
		Nonce:      1,
		Config:     limbo.Config{},
	}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		input.Nonce = uint64(i)
		_, _ = limbo.PlaySeeded(input)
	}
}
