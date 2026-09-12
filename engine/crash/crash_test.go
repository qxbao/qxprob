package crash_test

import (
	"errors"
	"math"
	"testing"

	"github.com/qxbao/qxprob/core/entropy"
	"github.com/qxbao/qxprob/core/simulation"
	"github.com/qxbao/qxprob/engine/crash"
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

// TestEngineInterfaceCompatibility verifies that *crash.Engine satisfies simulation.Engine[crash.Result].
func TestEngineInterfaceCompatibility(t *testing.T) {
	fake := &fakeEntropySource{values: []float64{0.5}}
	eng, err := crash.New(crash.Config{HouseEdge: 1.0, InstantCrashProbability: 0.0}, fake)
	if err != nil {
		t.Fatalf("unexpected construction failure: %v", err)
	}
	var _ simulation.Engine[crash.Result] = eng
}

// TestConstructorValidation verifies that New rejects invalid configurations and nil sources
// with documented sentinel errors and without consuming entropy.
func TestConstructorValidation(t *testing.T) {
	fake := &fakeEntropySource{values: []float64{0.5}}

	tests := []struct {
		name      string
		config    crash.Config
		source    entropy.Source
		wantErr   error
		wantValid bool
	}{
		{
			name:      "nil source",
			config:    crash.Config{HouseEdge: 1.0, InstantCrashProbability: 0.0},
			source:    nil,
			wantErr:   crash.ErrNilSource,
			wantValid: false,
		},
		{
			name:      "negative house edge",
			config:    crash.Config{HouseEdge: -0.01, InstantCrashProbability: 0.0},
			source:    fake,
			wantErr:   crash.ErrInvalidHouseEdge,
			wantValid: false,
		},
		{
			name:      "house edge at 100",
			config:    crash.Config{HouseEdge: 100.0, InstantCrashProbability: 0.0},
			source:    fake,
			wantErr:   crash.ErrInvalidHouseEdge,
			wantValid: false,
		},
		{
			name:      "house edge above 100",
			config:    crash.Config{HouseEdge: 105.0, InstantCrashProbability: 0.0},
			source:    fake,
			wantErr:   crash.ErrInvalidHouseEdge,
			wantValid: false,
		},
		{
			name:      "NaN house edge",
			config:    crash.Config{HouseEdge: math.NaN(), InstantCrashProbability: 0.0},
			source:    fake,
			wantErr:   crash.ErrInvalidHouseEdge,
			wantValid: false,
		},
		{
			name:      "positive infinite house edge",
			config:    crash.Config{HouseEdge: math.Inf(1), InstantCrashProbability: 0.0},
			source:    fake,
			wantErr:   crash.ErrInvalidHouseEdge,
			wantValid: false,
		},
		{
			name:      "negative infinite house edge",
			config:    crash.Config{HouseEdge: math.Inf(-1), InstantCrashProbability: 0.0},
			source:    fake,
			wantErr:   crash.ErrInvalidHouseEdge,
			wantValid: false,
		},
		{
			name:      "negative instant crash probability",
			config:    crash.Config{HouseEdge: 1.0, InstantCrashProbability: -0.01},
			source:    fake,
			wantErr:   crash.ErrInvalidInstantCrashProbability,
			wantValid: false,
		},
		{
			name:      "instant crash probability above 1",
			config:    crash.Config{HouseEdge: 1.0, InstantCrashProbability: 1.01},
			source:    fake,
			wantErr:   crash.ErrInvalidInstantCrashProbability,
			wantValid: false,
		},
		{
			name:      "NaN instant crash probability",
			config:    crash.Config{HouseEdge: 1.0, InstantCrashProbability: math.NaN()},
			source:    fake,
			wantErr:   crash.ErrInvalidInstantCrashProbability,
			wantValid: false,
		},
		{
			name:      "positive infinite instant crash probability",
			config:    crash.Config{HouseEdge: 1.0, InstantCrashProbability: math.Inf(1)},
			source:    fake,
			wantErr:   crash.ErrInvalidInstantCrashProbability,
			wantValid: false,
		},
		{
			name:      "negative infinite instant crash probability",
			config:    crash.Config{HouseEdge: 1.0, InstantCrashProbability: math.Inf(-1)},
			source:    fake,
			wantErr:   crash.ErrInvalidInstantCrashProbability,
			wantValid: false,
		},
		{
			name:      "valid zero house edge and zero instant crash",
			config:    crash.Config{HouseEdge: 0.0, InstantCrashProbability: 0.0},
			source:    fake,
			wantErr:   nil,
			wantValid: true,
		},
		{
			name:      "valid house edge boundary 99.999",
			config:    crash.Config{HouseEdge: 99.999, InstantCrashProbability: 0.5},
			source:    fake,
			wantErr:   nil,
			wantValid: true,
		},
		{
			name:      "valid instant crash probability at 1.0",
			config:    crash.Config{HouseEdge: 1.0, InstantCrashProbability: 1.0},
			source:    fake,
			wantErr:   nil,
			wantValid: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			beforeReads := fake.readCount
			engine, err := crash.New(tc.config, tc.source)
			if tc.wantValid {
				if err != nil {
					t.Fatalf("expected valid construction, got error: %v", err)
				}
				if engine == nil {
					t.Fatal("expected non-nil engine")
				}
			} else {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("error = %v, want %v", err, tc.wantErr)
				}
				if engine != nil {
					t.Fatal("expected nil engine on error")
				}
			}
			if fake.readCount != beforeReads {
				t.Fatalf("construction consumed entropy: reads before=%d, after=%d", beforeReads, fake.readCount)
			}
		})
	}
}

// TestMultiplierFormula verifies the multiplier calculation and single entropy consumption.
func TestMultiplierFormula(t *testing.T) {
	tests := []struct {
		name             string
		houseEdge        float64
		instantCrashProb float64
		u                float64
		wantMultiplier   float64
		wantInstantCrash bool
	}{
		{
			name:             "AC1: 1% house edge, u=0.5 yields 1.98",
			houseEdge:        1.0,
			instantCrashProb: 0.0,
			u:                0.5,
			wantMultiplier:   1.98,
			wantInstantCrash: false,
		},
		{
			name:             "0% house edge, u=0.5 yields 2.0",
			houseEdge:        0.0,
			instantCrashProb: 0.0,
			u:                0.5,
			wantMultiplier:   2.0,
			wantInstantCrash: false,
		},
		{
			name:             "5% house edge, u=0.5 yields 1.90",
			houseEdge:        5.0,
			instantCrashProb: 0.0,
			u:                0.5,
			wantMultiplier:   1.90,
			wantInstantCrash: false,
		},
		{
			name:             "2% house edge, u=0.8 yields 4.90",
			houseEdge:        2.0,
			instantCrashProb: 0.0,
			u:                0.8,
			wantMultiplier:   4.90,
			wantInstantCrash: false,
		},
		{
			name:             "1% house edge, u=0.99 yields 99.0",
			houseEdge:        1.0,
			instantCrashProb: 0.0,
			u:                0.99,
			wantMultiplier:   99.0,
			wantInstantCrash: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeEntropySource{values: []float64{tc.u}}
			engine, err := crash.New(crash.Config{
				HouseEdge:               tc.houseEdge,
				InstantCrashProbability: tc.instantCrashProb,
			}, fake)
			if err != nil {
				t.Fatalf("New failed: %v", err)
			}

			res, err := engine.Play()
			if err != nil {
				t.Fatalf("Play failed: %v", err)
			}

			if fake.readCount != 1 {
				t.Fatalf("expected exactly 1 read, got %d", fake.readCount)
			}
			if res.InstantCrash != tc.wantInstantCrash {
				t.Fatalf("InstantCrash = %v, want %v", res.InstantCrash, tc.wantInstantCrash)
			}
			if math.Abs(res.Multiplier-tc.wantMultiplier) > 1e-9 {
				t.Fatalf("Multiplier = %v, want %v", res.Multiplier, tc.wantMultiplier)
			}
		})
	}
}

// TestInstantCrashBoundaries verifies instant crash behavior and half-open boundary.
func TestInstantCrashBoundaries(t *testing.T) {
	threshold := 1.0 / 33.0

	tests := []struct {
		name             string
		instantCrashProb float64
		u                float64
		wantInstantCrash bool
		wantExactMult1   bool
	}{
		{
			name:             "AC2: u strictly below 1/33 triggers instant crash",
			instantCrashProb: threshold,
			u:                threshold - 1e-10,
			wantInstantCrash: true,
			wantExactMult1:   true,
		},
		{
			name:             "AC2: u=0.0 with positive instant crash prob triggers instant crash",
			instantCrashProb: threshold,
			u:                0.0,
			wantInstantCrash: true,
			wantExactMult1:   true,
		},
		{
			name:             "AC3: u exactly equal to boundary does not trigger instant crash (half-open)",
			instantCrashProb: threshold,
			u:                threshold,
			wantInstantCrash: false,
			wantExactMult1:   false,
		},
		{
			name:             "AC3: u strictly above boundary does not trigger instant crash",
			instantCrashProb: threshold,
			u:                threshold + 1e-10,
			wantInstantCrash: false,
			wantExactMult1:   false,
		},
		{
			name:             "zero instant crash prob with u=0 never triggers instant crash",
			instantCrashProb: 0.0,
			u:                0.0,
			wantInstantCrash: false,
			wantExactMult1:   false,
		},
		{
			name:             "instant crash prob 1.0 with u=0.9999 always triggers instant crash",
			instantCrashProb: 1.0,
			u:                0.9999,
			wantInstantCrash: true,
			wantExactMult1:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeEntropySource{values: []float64{tc.u}}
			engine, err := crash.New(crash.Config{
				HouseEdge:               1.0,
				InstantCrashProbability: tc.instantCrashProb,
			}, fake)
			if err != nil {
				t.Fatalf("New failed: %v", err)
			}

			res, err := engine.Play()
			if err != nil {
				t.Fatalf("Play failed: %v", err)
			}

			if res.InstantCrash != tc.wantInstantCrash {
				t.Fatalf("InstantCrash = %v, want %v", res.InstantCrash, tc.wantInstantCrash)
			}
			if tc.wantExactMult1 && res.Multiplier != 1.0 {
				t.Fatalf("expected exact multiplier 1.0, got %v", res.Multiplier)
			}
			if !tc.wantInstantCrash {
				expectedMult := (100.0 - 1.0) / (100.0 * (1.0 - tc.u))
				if expectedMult < 1.0 {
					expectedMult = 1.0
				}
				if math.Abs(res.Multiplier-expectedMult) > 1e-9 {
					t.Fatalf("expected normal multiplier %v, got %v", expectedMult, res.Multiplier)
				}
			}
		})
	}
}

// TestFloorBehavior verifies that raw multipliers below 1.0 are floored to 1.0
// without being marked as instant crashes.
func TestFloorBehavior(t *testing.T) {
	tests := []struct {
		name      string
		houseEdge float64
		u         float64
	}{
		{
			name:      "AC4: 1% house edge with u=0 gives raw 0.99, floored to 1.0",
			houseEdge: 1.0,
			u:         0.0,
		},
		{
			name:      "AC4: 5% house edge with u=0 gives raw 0.95, floored to 1.0",
			houseEdge: 5.0,
			u:         0.0,
		},
		{
			name:      "AC4: 5% house edge with u=0.04 gives raw 95/96 < 1.0, floored to 1.0",
			houseEdge: 5.0,
			u:         0.04,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeEntropySource{values: []float64{tc.u}}
			engine, err := crash.New(crash.Config{
				HouseEdge:               tc.houseEdge,
				InstantCrashProbability: 0.0,
			}, fake)
			if err != nil {
				t.Fatalf("New failed: %v", err)
			}

			res, err := engine.Play()
			if err != nil {
				t.Fatalf("Play failed: %v", err)
			}

			if res.Multiplier != 1.0 {
				t.Fatalf("Multiplier = %v, want 1.0", res.Multiplier)
			}
			if res.InstantCrash {
				t.Fatal("floored multiplier must NOT be marked as instant crash")
			}
		})
	}
}

// TestSourceErrorPropagation verifies that source errors are wrapped and discoverable with errors.Is.
func TestSourceErrorPropagation(t *testing.T) {
	injectedErr := errors.New("entropy device failure")
	fake := &fakeEntropySource{err: injectedErr}

	engine, err := crash.New(crash.Config{HouseEdge: 1.0, InstantCrashProbability: 0.0}, fake)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	res, err := engine.Play()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, injectedErr) {
		t.Fatalf("error = %v, want wrapped %v", err, injectedErr)
	}
	if res.Multiplier != 0.0 || res.InstantCrash {
		t.Fatalf("expected zero Result on error, got %+v", res)
	}
}

// TestMultipleSequentialRounds verifies that successive Play calls consume one float each in order.
func TestMultipleSequentialRounds(t *testing.T) {
	uValues := []float64{0.1, 0.2, 0.3, 0.4, 0.5}
	fake := &fakeEntropySource{values: uValues}

	engine, err := crash.New(crash.Config{HouseEdge: 0.0, InstantCrashProbability: 0.0}, fake)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	for i, u := range uValues {
		res, err := engine.Play()
		if err != nil {
			t.Fatalf("round %d failed: %v", i, err)
		}
		expected := 1.0 / (1.0 - u)
		if math.Abs(res.Multiplier-expected) > 1e-9 {
			t.Fatalf("round %d multiplier = %v, want %v", i, res.Multiplier, expected)
		}
		if fake.readCount != i+1 {
			t.Fatalf("round %d readCount = %d, want %d", i, fake.readCount, i+1)
		}
	}
}

// TestPlaySeededValidation verifies that PlaySeeded validates empty seeds and invalid configs
// with documented sentinel errors before consuming entropy.
func TestPlaySeededValidation(t *testing.T) {
	tests := []struct {
		name    string
		input   crash.SeededInput
		wantErr error
	}{
		{
			name: "empty server seed",
			input: crash.SeededInput{
				ServerSeed: "",
				ClientSeed: "client-seed",
				Nonce:      1,
				Config:     crash.Config{HouseEdge: 1.0, InstantCrashProbability: 0.0},
			},
			wantErr: crash.ErrEmptyServerSeed,
		},
		{
			name: "empty client seed",
			input: crash.SeededInput{
				ServerSeed: "server-seed",
				ClientSeed: "",
				Nonce:      1,
				Config:     crash.Config{HouseEdge: 1.0, InstantCrashProbability: 0.0},
			},
			wantErr: crash.ErrEmptyClientSeed,
		},
		{
			name: "negative house edge",
			input: crash.SeededInput{
				ServerSeed: "server-seed",
				ClientSeed: "client-seed",
				Nonce:      1,
				Config:     crash.Config{HouseEdge: -0.01, InstantCrashProbability: 0.0},
			},
			wantErr: crash.ErrInvalidHouseEdge,
		},
		{
			name: "house edge at 100",
			input: crash.SeededInput{
				ServerSeed: "server-seed",
				ClientSeed: "client-seed",
				Nonce:      1,
				Config:     crash.Config{HouseEdge: 100.0, InstantCrashProbability: 0.0},
			},
			wantErr: crash.ErrInvalidHouseEdge,
		},
		{
			name: "house edge above 100",
			input: crash.SeededInput{
				ServerSeed: "server-seed",
				ClientSeed: "client-seed",
				Nonce:      1,
				Config:     crash.Config{HouseEdge: 105.0, InstantCrashProbability: 0.0},
			},
			wantErr: crash.ErrInvalidHouseEdge,
		},
		{
			name: "NaN house edge",
			input: crash.SeededInput{
				ServerSeed: "server-seed",
				ClientSeed: "client-seed",
				Nonce:      1,
				Config:     crash.Config{HouseEdge: math.NaN(), InstantCrashProbability: 0.0},
			},
			wantErr: crash.ErrInvalidHouseEdge,
		},
		{
			name: "positive infinite house edge",
			input: crash.SeededInput{
				ServerSeed: "server-seed",
				ClientSeed: "client-seed",
				Nonce:      1,
				Config:     crash.Config{HouseEdge: math.Inf(1), InstantCrashProbability: 0.0},
			},
			wantErr: crash.ErrInvalidHouseEdge,
		},
		{
			name: "negative infinite house edge",
			input: crash.SeededInput{
				ServerSeed: "server-seed",
				ClientSeed: "client-seed",
				Nonce:      1,
				Config:     crash.Config{HouseEdge: math.Inf(-1), InstantCrashProbability: 0.0},
			},
			wantErr: crash.ErrInvalidHouseEdge,
		},
		{
			name: "negative instant crash probability",
			input: crash.SeededInput{
				ServerSeed: "server-seed",
				ClientSeed: "client-seed",
				Nonce:      1,
				Config:     crash.Config{HouseEdge: 1.0, InstantCrashProbability: -0.01},
			},
			wantErr: crash.ErrInvalidInstantCrashProbability,
		},
		{
			name: "instant crash probability above 1",
			input: crash.SeededInput{
				ServerSeed: "server-seed",
				ClientSeed: "client-seed",
				Nonce:      1,
				Config:     crash.Config{HouseEdge: 1.0, InstantCrashProbability: 1.01},
			},
			wantErr: crash.ErrInvalidInstantCrashProbability,
		},
		{
			name: "NaN instant crash probability",
			input: crash.SeededInput{
				ServerSeed: "server-seed",
				ClientSeed: "client-seed",
				Nonce:      1,
				Config:     crash.Config{HouseEdge: 1.0, InstantCrashProbability: math.NaN()},
			},
			wantErr: crash.ErrInvalidInstantCrashProbability,
		},
		{
			name: "positive infinite instant crash probability",
			input: crash.SeededInput{
				ServerSeed: "server-seed",
				ClientSeed: "client-seed",
				Nonce:      1,
				Config:     crash.Config{HouseEdge: 1.0, InstantCrashProbability: math.Inf(1)},
			},
			wantErr: crash.ErrInvalidInstantCrashProbability,
		},
		{
			name: "negative infinite instant crash probability",
			input: crash.SeededInput{
				ServerSeed: "server-seed",
				ClientSeed: "client-seed",
				Nonce:      1,
				Config:     crash.Config{HouseEdge: 1.0, InstantCrashProbability: math.Inf(-1)},
			},
			wantErr: crash.ErrInvalidInstantCrashProbability,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, err := crash.PlaySeeded(tc.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
			if res.Multiplier != 0.0 || res.InstantCrash {
				t.Fatalf("expected zero Result on error, got %+v", res)
			}
		})
	}
}

// TestPlaySeededDeterminismAndReplay verifies that identical seeds and nonce reproduce
// the exact round result.
func TestPlaySeededDeterminismAndReplay(t *testing.T) {
	input := crash.SeededInput{
		ServerSeed: "server-secret-alpha",
		ClientSeed: "client-pub-beta",
		Nonce:      42,
		Config: crash.Config{
			HouseEdge:               1.0,
			InstantCrashProbability: 0.03,
		},
	}

	res1, err := crash.PlaySeeded(input)
	if err != nil {
		t.Fatalf("first PlaySeeded failed: %v", err)
	}

	res2, err := crash.PlaySeeded(input)
	if err != nil {
		t.Fatalf("replay PlaySeeded failed: %v", err)
	}

	if res1.Multiplier != res2.Multiplier {
		t.Fatalf("Multiplier mismatch on replay: %v != %v", res1.Multiplier, res2.Multiplier)
	}
	if res1.InstantCrash != res2.InstantCrash {
		t.Fatalf("InstantCrash mismatch on replay: %v != %v", res1.InstantCrash, res2.InstantCrash)
	}
}

// TestPlaySeededNonceSeparation verifies that changing the nonce alters the outcome.
func TestPlaySeededNonceSeparation(t *testing.T) {
	cfg := crash.Config{
		HouseEdge:               1.0,
		InstantCrashProbability: 0.03,
	}
	input1 := crash.SeededInput{
		ServerSeed: "nonce-separation-server",
		ClientSeed: "nonce-separation-client",
		Nonce:      1,
		Config:     cfg,
	}
	input2 := crash.SeededInput{
		ServerSeed: "nonce-separation-server",
		ClientSeed: "nonce-separation-client",
		Nonce:      2,
		Config:     cfg,
	}

	res1, err := crash.PlaySeeded(input1)
	if err != nil {
		t.Fatalf("round 1 failed: %v", err)
	}
	res2, err := crash.PlaySeeded(input2)
	if err != nil {
		t.Fatalf("round 2 failed: %v", err)
	}

	if res1.Multiplier == res2.Multiplier && res1.InstantCrash == res2.InstantCrash {
		t.Fatal("expected different outcomes for different nonces")
	}
}

// TestPlaySeededSeedSeparation verifies that changing server or client seed alters the outcome.
func TestPlaySeededSeedSeparation(t *testing.T) {
	cfg := crash.Config{
		HouseEdge:               1.0,
		InstantCrashProbability: 0.03,
	}
	base := crash.SeededInput{
		ServerSeed: "seed-server-a",
		ClientSeed: "seed-client-a",
		Nonce:      1,
		Config:     cfg,
	}
	diffServer := crash.SeededInput{
		ServerSeed: "seed-server-b",
		ClientSeed: "seed-client-a",
		Nonce:      1,
		Config:     cfg,
	}
	diffClient := crash.SeededInput{
		ServerSeed: "seed-server-a",
		ClientSeed: "seed-client-b",
		Nonce:      1,
		Config:     cfg,
	}

	baseRes, err := crash.PlaySeeded(base)
	if err != nil {
		t.Fatalf("base failed: %v", err)
	}
	serverRes, err := crash.PlaySeeded(diffServer)
	if err != nil {
		t.Fatalf("diff server failed: %v", err)
	}
	clientRes, err := crash.PlaySeeded(diffClient)
	if err != nil {
		t.Fatalf("diff client failed: %v", err)
	}

	if baseRes.Multiplier == serverRes.Multiplier && baseRes.InstantCrash == serverRes.InstantCrash {
		t.Fatal("expected different outcome when server seed changes")
	}
	if baseRes.Multiplier == clientRes.Multiplier && baseRes.InstantCrash == clientRes.InstantCrash {
		t.Fatal("expected different outcome when client seed changes")
	}
}

// TestPlaySeededDeterministicVector pins an exact compatibility vector for Crash replay.
func TestPlaySeededDeterministicVector(t *testing.T) {
	input := crash.SeededInput{
		ServerSeed: "provably-fair-server-seed-vector",
		ClientSeed: "provably-fair-client-seed-vector",
		Nonce:      42,
		Config: crash.Config{
			HouseEdge:               1.0,
			InstantCrashProbability: 0.03,
		},
	}

	res, err := crash.PlaySeeded(input)
	if err != nil {
		t.Fatalf("PlaySeeded failed: %v", err)
	}

	const expectedMultiplier = 2.5604331624143573
	if res.InstantCrash {
		t.Fatalf("InstantCrash = true, want false")
	}
	if math.Abs(res.Multiplier-expectedMultiplier) > 1e-15 {
		t.Fatalf("Multiplier = %v, want %v", res.Multiplier, expectedMultiplier)
	}
}

// BenchmarkPlaySeeded measures performance of deterministic seeded Crash rounds.
func BenchmarkPlaySeeded(b *testing.B) {
	input := crash.SeededInput{
		ServerSeed: "benchmark-server-seed",
		ClientSeed: "benchmark-client-seed",
		Nonce:      1,
		Config: crash.Config{
			HouseEdge:               1.0,
			InstantCrashProbability: 0.03,
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		input.Nonce = uint64(i)
		_, _ = crash.PlaySeeded(input)
	}
}
