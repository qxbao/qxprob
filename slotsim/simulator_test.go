package slotsim_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/qxbao/qxprob/core/slot"
	"github.com/qxbao/qxprob/slotsim"
)

type mockSpinner struct {
	results []slot.SpinResult
	idx     int
	err     error
}

func (m *mockSpinner) Spin(slot.SpinRequest) (slot.SpinResult, error) {
	if m.err != nil {
		return slot.SpinResult{}, m.err
	}
	if m.idx < len(m.results) {
		res := m.results[m.idx]
		m.idx++
		return res, nil
	}
	return slot.SpinResult{
		GameID:  "mock-game",
		Version: "1.0.0",
		Bet:     100,
	}, nil
}

func TestSimulator_Validation(t *testing.T) {
	ctx := context.Background()

	t.Run("nil context", func(t *testing.T) {
		_, err := slotsim.Run(nil, slotsim.Options{Spins: 10, Bet: 100}, func(uint64) (slotsim.Spinner, error) {
			return &mockSpinner{}, nil
		})
		if !errors.Is(err, slotsim.ErrNilContext) {
			t.Errorf("expected ErrNilContext, got %v", err)
		}
	})

	t.Run("nil factory", func(t *testing.T) {
		_, err := slotsim.Run(ctx, slotsim.Options{Spins: 10, Bet: 100}, nil)
		if !errors.Is(err, slotsim.ErrNilFactory) {
			t.Errorf("expected ErrNilFactory, got %v", err)
		}
	})

	t.Run("zero spins", func(t *testing.T) {
		_, err := slotsim.Run(ctx, slotsim.Options{Spins: 0, Bet: 100}, func(uint64) (slotsim.Spinner, error) {
			return &mockSpinner{}, nil
		})
		if !errors.Is(err, slotsim.ErrZeroSpins) {
			t.Errorf("expected ErrZeroSpins, got %v", err)
		}
	})

	t.Run("invalid bet", func(t *testing.T) {
		_, err := slotsim.Run(ctx, slotsim.Options{Spins: 10, Bet: 0}, func(uint64) (slotsim.Spinner, error) {
			return &mockSpinner{}, nil
		})
		if !errors.Is(err, slotsim.ErrInvalidBet) {
			t.Errorf("expected ErrInvalidBet, got %v", err)
		}
	})
}

func TestSimulator_Cancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := slotsim.Run(ctx, slotsim.Options{Spins: 100, Bet: 100}, func(uint64) (slotsim.Spinner, error) {
		return &mockSpinner{}, nil
	})
	if err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("expected cancellation error, got %v", err)
	}
}

func TestSimulator_ScriptedAggregates(t *testing.T) {
	// 4 scripted spins with bet = 100:
	// Round 1: loss (payout 0, mult 0)
	// Round 2: 2.0x win base (payout 200, base mult 2.0)
	// Round 3: 5.0x win feature (payout 500, feat mult 5.0, 1 feature)
	// Round 4: 1.0x win base (payout 100, base mult 1.0)
	// Total bet = 400
	// Total payout = 800
	// Total RTP = 800 / 400 = 200% (2.0)
	// Base payout = 300 (RTP 75%)
	// Feature payout = 500 (RTP 125%)
	// Hit count = 3 (75% hit frequency)
	// Feature count = 1 (25% feature frequency)
	// Max multiplier = 5.0x
	scripted := []slot.SpinResult{
		{GameID: "test-game", Version: "1.0.0", Bet: 100, Payout: 0, TotalMultiplier: 0},
		{GameID: "test-game", Version: "1.0.0", Bet: 100, Payout: 200, BaseMultiplier: slot.MustMultiplier("2.0"), TotalMultiplier: slot.MustMultiplier("2.0")},
		{GameID: "test-game", Version: "1.0.0", Bet: 100, Payout: 500, FeatureMultiplier: slot.MustMultiplier("5.0"), TotalMultiplier: slot.MustMultiplier("5.0"), FreeSpins: []slot.SpinSession{{}}},
		{GameID: "test-game", Version: "1.0.0", Bet: 100, Payout: 100, BaseMultiplier: slot.MustMultiplier("1.0"), TotalMultiplier: slot.MustMultiplier("1.0")},
	}

	spinner := &mockSpinner{results: scripted}
	report, err := slotsim.Run(context.Background(), slotsim.Options{Spins: 4, Bet: 100}, func(uint64) (slotsim.Spinner, error) {
		return spinner, nil
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if report.TotalSpins != 4 {
		t.Errorf("TotalSpins = %d, want 4", report.TotalSpins)
	}
	if report.TotalBet != 400 {
		t.Errorf("TotalBet = %d, want 400", report.TotalBet)
	}
	if report.TotalPayout != 800 {
		t.Errorf("TotalPayout = %d, want 800", report.TotalPayout)
	}
	if report.TotalRTP != 2.0 {
		t.Errorf("TotalRTP = %f, want 2.0", report.TotalRTP)
	}
	if report.BasePayout != 300 {
		t.Errorf("BasePayout = %d, want 300", report.BasePayout)
	}
	if report.FeaturePayout != 500 {
		t.Errorf("FeaturePayout = %d, want 500", report.FeaturePayout)
	}
	if report.HitCount != 3 {
		t.Errorf("HitCount = %d, want 3", report.HitCount)
	}
	if report.FeatureCount != 1 {
		t.Errorf("FeatureCount = %d, want 1", report.FeatureCount)
	}
	if report.MaxMultiplier != slot.MustMultiplier("5.0") {
		t.Errorf("MaxMultiplier = %s, want 5.0", report.MaxMultiplier)
	}

	// Exports test
	var bufJSON, bufCSV, bufText bytes.Buffer
	if err := slotsim.WriteJSON(&bufJSON, report); err != nil {
		t.Fatalf("WriteJSON error: %v", err)
	}
	if !strings.Contains(bufJSON.String(), `"total_rtp": 2`) {
		t.Errorf("JSON missing total_rtp")
	}

	if err := slotsim.WriteCSV(&bufCSV, report); err != nil {
		t.Fatalf("WriteCSV error: %v", err)
	}
	if !strings.Contains(bufCSV.String(), "Total RTP (%)") {
		t.Errorf("CSV missing header")
	}

	if err := slotsim.WriteText(&bufText, report); err != nil {
		t.Fatalf("WriteText error: %v", err)
	}
	if !strings.Contains(bufText.String(), "Total RTP:         200.0000%") {
		t.Errorf("Text missing total RTP summary")
	}
}

func TestSimulator_SettlementErrorHandling(t *testing.T) {
	// Settle error (negative multiplier) should be caught and returned
	badResult := slot.SpinResult{
		GameID:         "test",
		Version:        "1.0.0",
		Bet:            100,
		BaseMultiplier: -1,
	}
	spinner := &mockSpinner{results: []slot.SpinResult{badResult}}
	_, err := slotsim.Run(context.Background(), slotsim.Options{Spins: 1, Bet: 100}, func(uint64) (slotsim.Spinner, error) {
		return spinner, nil
	})
	if err == nil {
		t.Fatalf("expected settlement error from negative multiplier, got nil")
	}
}

func TestSimulator_SimulationModeSummaryCount(t *testing.T) {
	// When FreeSpins slice is omitted (simulation mode), FreeSpinCount > 0 must increment FeatureCount
	simResult := slot.SpinResult{
		GameID:        "test",
		Version:       "1.0.0",
		Bet:           100,
		FreeSpinCount: 8,
		FreeSpins:     nil, // omitted in simulation mode
	}
	spinner := &mockSpinner{results: []slot.SpinResult{simResult}}
	report, err := slotsim.Run(context.Background(), slotsim.Options{Spins: 1, Bet: 100}, func(uint64) (slotsim.Spinner, error) {
		return spinner, nil
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if report.FeatureCount != 1 {
		t.Errorf("expected FeatureCount = 1 from FreeSpinCount, got %d", report.FeatureCount)
	}
}

func TestSimulator_RunWithRunner_Validation(t *testing.T) {
	ctx := context.Background()
	dummyRunner := func(uint64, slot.SpinRequest) (slot.SpinResult, error) {
		return slot.SpinResult{}, nil
	}

	t.Run("nil context", func(t *testing.T) {
		_, err := slotsim.RunWithRunner(nil, slotsim.Options{Spins: 10, Bet: 100}, dummyRunner)
		if !errors.Is(err, slotsim.ErrNilContext) {
			t.Errorf("expected ErrNilContext, got %v", err)
		}
	})

	t.Run("nil runner", func(t *testing.T) {
		_, err := slotsim.RunWithRunner(ctx, slotsim.Options{Spins: 10, Bet: 100}, nil)
		if !errors.Is(err, slotsim.ErrNilRunner) {
			t.Errorf("expected ErrNilRunner, got %v", err)
		}
	})

	t.Run("zero spins", func(t *testing.T) {
		_, err := slotsim.RunWithRunner(ctx, slotsim.Options{Spins: 0, Bet: 100}, dummyRunner)
		if !errors.Is(err, slotsim.ErrZeroSpins) {
			t.Errorf("expected ErrZeroSpins, got %v", err)
		}
	})

	t.Run("invalid bet", func(t *testing.T) {
		_, err := slotsim.RunWithRunner(ctx, slotsim.Options{Spins: 10, Bet: 0}, dummyRunner)
		if !errors.Is(err, slotsim.ErrInvalidBet) {
			t.Errorf("expected ErrInvalidBet, got %v", err)
		}
	})
}

func TestSimulator_RunWithRunner_Cancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := slotsim.RunWithRunner(ctx, slotsim.Options{Spins: 100, Bet: 100}, func(uint64, slot.SpinRequest) (slot.SpinResult, error) {
		return slot.SpinResult{}, nil
	})
	if err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("expected cancellation error, got %v", err)
	}
}

func TestSimulator_RunWithRunner_SequentialNoncesAndRequests(t *testing.T) {
	const totalSpins = 15
	const testBet = slot.Amount(250)

	var seenNonces []uint64
	var seenModes []slot.PlayMode
	var seenBets []slot.Amount

	runner := func(nonce uint64, req slot.SpinRequest) (slot.SpinResult, error) {
		seenNonces = append(seenNonces, nonce)
		seenModes = append(seenModes, req.Mode)
		seenBets = append(seenBets, req.Bet)
		return slot.SpinResult{
			GameID:  "test-runner-game",
			Version: "1.0.0",
			Bet:     req.Bet,
		}, nil
	}

	report, err := slotsim.RunWithRunner(context.Background(), slotsim.Options{Spins: totalSpins, Bet: testBet}, runner)
	if err != nil {
		t.Fatalf("RunWithRunner failed: %v", err)
	}

	if report.TotalSpins != totalSpins {
		t.Errorf("TotalSpins = %d, want %d", report.TotalSpins, totalSpins)
	}
	if len(seenNonces) != totalSpins {
		t.Fatalf("seen nonces length = %d, want %d", len(seenNonces), totalSpins)
	}

	for i := 0; i < totalSpins; i++ {
		expectedNonce := uint64(i + 1)
		if seenNonces[i] != expectedNonce {
			t.Errorf("nonce[%d] = %d, want %d", i, seenNonces[i], expectedNonce)
		}
		if seenModes[i] != slot.PlayModeSimulation {
			t.Errorf("mode[%d] = %s, want %s", i, seenModes[i], slot.PlayModeSimulation)
		}
		if seenBets[i] != testBet {
			t.Errorf("bet[%d] = %d, want %d", i, seenBets[i], testBet)
		}
	}
}

func TestSimulator_RunWithRunner_ErrorPropagation(t *testing.T) {
	spinErr := errors.New("custom spin failure")
	runner := func(nonce uint64, req slot.SpinRequest) (slot.SpinResult, error) {
		if nonce == 3 {
			return slot.SpinResult{}, spinErr
		}
		return slot.SpinResult{Bet: req.Bet}, nil
	}

	_, err := slotsim.RunWithRunner(context.Background(), slotsim.Options{Spins: 5, Bet: 100}, runner)
	if err == nil || !errors.Is(err, spinErr) {
		t.Fatalf("expected wrapped spinErr, got %v", err)
	}
	if !strings.Contains(err.Error(), "nonce 3") {
		t.Errorf("expected error message to mention nonce 3, got %v", err)
	}
}

func TestSimulator_RunWithRunner_LegacyEquivalence(t *testing.T) {
	scripted := []slot.SpinResult{
		{GameID: "test-game", Version: "1.0.0", Bet: 100, Payout: 0, TotalMultiplier: 0},
		{GameID: "test-game", Version: "1.0.0", Bet: 100, Payout: 200, BaseMultiplier: slot.MustMultiplier("2.0"), TotalMultiplier: slot.MustMultiplier("2.0")},
		{GameID: "test-game", Version: "1.0.0", Bet: 100, Payout: 500, FeatureMultiplier: slot.MustMultiplier("5.0"), TotalMultiplier: slot.MustMultiplier("5.0"), FreeSpins: []slot.SpinSession{{}}},
		{GameID: "test-game", Version: "1.0.0", Bet: 100, Payout: 100, BaseMultiplier: slot.MustMultiplier("1.0"), TotalMultiplier: slot.MustMultiplier("1.0")},
	}

	opts := slotsim.Options{Spins: uint64(len(scripted)), Bet: 100}

	// Legacy Run with Factory
	spinnerLegacy := &mockSpinner{results: scripted}
	reportLegacy, err := slotsim.Run(context.Background(), opts, func(uint64) (slotsim.Spinner, error) {
		return spinnerLegacy, nil
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// RunWithRunner with reusable runner
	runnerIdx := 0
	runner := func(nonce uint64, req slot.SpinRequest) (slot.SpinResult, error) {
		res := scripted[runnerIdx]
		runnerIdx++
		return res, nil
	}
	reportRunner, err := slotsim.RunWithRunner(context.Background(), opts, runner)
	if err != nil {
		t.Fatalf("RunWithRunner failed: %v", err)
	}

	// Compare aggregate statistics (ignoring ElapsedSeconds which is timing-dependent)
	if reportLegacy.TotalSpins != reportRunner.TotalSpins {
		t.Errorf("TotalSpins: legacy %d != runner %d", reportLegacy.TotalSpins, reportRunner.TotalSpins)
	}
	if reportLegacy.TotalBet != reportRunner.TotalBet {
		t.Errorf("TotalBet: legacy %d != runner %d", reportLegacy.TotalBet, reportRunner.TotalBet)
	}
	if reportLegacy.TotalPayout != reportRunner.TotalPayout {
		t.Errorf("TotalPayout: legacy %d != runner %d", reportLegacy.TotalPayout, reportRunner.TotalPayout)
	}
	if reportLegacy.TotalRTP != reportRunner.TotalRTP {
		t.Errorf("TotalRTP: legacy %f != runner %f", reportLegacy.TotalRTP, reportRunner.TotalRTP)
	}
	if reportLegacy.BasePayout != reportRunner.BasePayout {
		t.Errorf("BasePayout: legacy %d != runner %d", reportLegacy.BasePayout, reportRunner.BasePayout)
	}
	if reportLegacy.FeaturePayout != reportRunner.FeaturePayout {
		t.Errorf("FeaturePayout: legacy %d != runner %d", reportLegacy.FeaturePayout, reportRunner.FeaturePayout)
	}
	if reportLegacy.HitCount != reportRunner.HitCount {
		t.Errorf("HitCount: legacy %d != runner %d", reportLegacy.HitCount, reportRunner.HitCount)
	}
	if reportLegacy.FeatureCount != reportRunner.FeatureCount {
		t.Errorf("FeatureCount: legacy %d != runner %d", reportLegacy.FeatureCount, reportRunner.FeatureCount)
	}
	if reportLegacy.MaxMultiplier != reportRunner.MaxMultiplier {
		t.Errorf("MaxMultiplier: legacy %s != runner %s", reportLegacy.MaxMultiplier, reportRunner.MaxMultiplier)
	}
	if len(reportLegacy.Buckets) != len(reportRunner.Buckets) {
		t.Fatalf("Buckets length mismatch")
	}
	for i := range reportLegacy.Buckets {
		if reportLegacy.Buckets[i] != reportRunner.Buckets[i] {
			t.Errorf("Bucket[%d] mismatch: legacy %+v != runner %+v", i, reportLegacy.Buckets[i], reportRunner.Buckets[i])
		}
	}
}

func TestSimulator_RunWithRunner_StateReuse(t *testing.T) {
	// Verify that stateful caller objects (like a reusable runner struct) persist across calls
	type statefulRunner struct {
		calls int
	}
	sr := &statefulRunner{}

	runner := func(nonce uint64, req slot.SpinRequest) (slot.SpinResult, error) {
		sr.calls++
		return slot.SpinResult{Bet: req.Bet}, nil
	}

	opts := slotsim.Options{Spins: 50, Bet: 100}
	_, err := slotsim.RunWithRunner(context.Background(), opts, runner)
	if err != nil {
		t.Fatal(err)
	}
	if sr.calls != 50 {
		t.Errorf("expected stateful runner to be called 50 times, got %d", sr.calls)
	}
}
