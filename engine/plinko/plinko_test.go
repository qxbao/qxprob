package plinko_test

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/qxbao/qxprob/core/entropy"
	"github.com/qxbao/qxprob/core/simulation"
	"github.com/qxbao/qxprob/engine/plinko"
)

// fakeEntropySource is a deterministic fake implementing entropy.Source for tests.
type fakeEntropySource struct {
	decisions []uint64
	index     int
	err       error
	readCount int
}

func (s *fakeEntropySource) Intn(n uint64) (uint64, error) {
	s.readCount++
	if s.err != nil {
		return 0, s.err
	}
	if s.index >= len(s.decisions) {
		return 0, errors.New("fake: no more decisions")
	}
	val := s.decisions[s.index]
	s.index++
	return val % n, nil
}

func (s *fakeEntropySource) Float64() (float64, error) {
	return 0, errors.New("fake: Float64 not implemented")
}

func (s *fakeEntropySource) Bits(int) (uint64, error) {
	return 0, errors.New("fake: Bits not implemented")
}

func (s *fakeEntropySource) Uint64() (uint64, error) {
	return 0, errors.New("fake: Uint64 not implemented")
}

var _ entropy.Source = (*fakeEntropySource)(nil)

// TestEngineInterfaceCompatibility verifies that *plinko.Engine satisfies simulation.Engine[plinko.Result].
func TestEngineInterfaceCompatibility(t *testing.T) {
	fake := &fakeEntropySource{
		decisions: []uint64{0, 1, 0, 1, 0, 1, 0, 1},
	}
	eng, err := plinko.New(plinko.Config{Rows: 8, Risk: plinko.Low}, fake)
	if err != nil {
		t.Fatalf("unexpected construction failure: %v", err)
	}
	var _ simulation.Engine[plinko.Result] = eng

	var roundCount uint64
	err = simulation.Run(context.Background(), eng, 1, func(round uint64, res plinko.Result) error {
		roundCount++
		if len(res.Path) != 8 {
			t.Fatalf("expected path length 8, got %d", len(res.Path))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("simulation.Run failed: %v", err)
	}
	if roundCount != 1 {
		t.Fatalf("expected 1 round, got %d", roundCount)
	}
}

// TestConstructorValidation verifies that New validates config and source boundaries
// without consuming entropy.
func TestConstructorValidation(t *testing.T) {
	fake := &fakeEntropySource{decisions: make([]uint64, 32)}

	tests := []struct {
		name      string
		config    plinko.Config
		source    entropy.Source
		wantErr   error
		wantValid bool
	}{
		{
			name:      "nil source",
			config:    plinko.Config{Rows: 8, Risk: plinko.Low},
			source:    nil,
			wantErr:   plinko.ErrNilSource,
			wantValid: false,
		},
		{
			name:      "rows below minimum (0)",
			config:    plinko.Config{Rows: 0, Risk: plinko.Low},
			source:    fake,
			wantErr:   plinko.ErrInvalidRows,
			wantValid: false,
		},
		{
			name:      "rows below minimum (7)",
			config:    plinko.Config{Rows: 7, Risk: plinko.Low},
			source:    fake,
			wantErr:   plinko.ErrInvalidRows,
			wantValid: false,
		},
		{
			name:      "negative rows",
			config:    plinko.Config{Rows: -1, Risk: plinko.Low},
			source:    fake,
			wantErr:   plinko.ErrInvalidRows,
			wantValid: false,
		},
		{
			name:      "rows above maximum (17)",
			config:    plinko.Config{Rows: 17, Risk: plinko.Low},
			source:    fake,
			wantErr:   plinko.ErrInvalidRows,
			wantValid: false,
		},
		{
			name:      "rows above maximum (100)",
			config:    plinko.Config{Rows: 100, Risk: plinko.Low},
			source:    fake,
			wantErr:   plinko.ErrInvalidRows,
			wantValid: false,
		},
		{
			name:      "invalid risk level (3)",
			config:    plinko.Config{Rows: 8, Risk: plinko.RiskLevel(3)},
			source:    fake,
			wantErr:   plinko.ErrInvalidRisk,
			wantValid: false,
		},
		{
			name:      "invalid risk level (255)",
			config:    plinko.Config{Rows: 8, Risk: plinko.RiskLevel(255)},
			source:    fake,
			wantErr:   plinko.ErrInvalidRisk,
			wantValid: false,
		},
		{
			name:      "valid minimum rows (8, Low)",
			config:    plinko.Config{Rows: 8, Risk: plinko.Low},
			source:    fake,
			wantErr:   nil,
			wantValid: true,
		},
		{
			name:      "valid maximum rows (16, High)",
			config:    plinko.Config{Rows: 16, Risk: plinko.High},
			source:    fake,
			wantErr:   nil,
			wantValid: true,
		},
		{
			name:      "valid intermediate rows (12, Medium)",
			config:    plinko.Config{Rows: 12, Risk: plinko.Medium},
			source:    fake,
			wantErr:   nil,
			wantValid: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			beforeReads := fake.readCount
			eng, err := plinko.New(tc.config, tc.source)
			if tc.wantValid {
				if err != nil {
					t.Fatalf("expected valid construction, got: %v", err)
				}
				if eng == nil {
					t.Fatal("expected non-nil engine")
				}
			} else {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("error = %v, want %v", err, tc.wantErr)
				}
				if eng != nil {
					t.Fatal("expected nil engine on error")
				}
			}
			if fake.readCount != beforeReads {
				t.Fatalf("construction consumed entropy: before=%d after=%d", beforeReads, fake.readCount)
			}
		})
	}
}

// TestPlayPathAndBucketProperties verifies path length, direction values, bucket index calculation,
// and path slice independence.
func TestPlayPathAndBucketProperties(t *testing.T) {
	for rows := 8; rows <= 16; rows++ {
		decisions := make([]uint64, rows)
		for i := range decisions {
			decisions[i] = uint64(i % 2) // alternating Left and Right
		}

		fake := &fakeEntropySource{decisions: decisions}
		eng, err := plinko.New(plinko.Config{Rows: rows, Risk: plinko.Medium}, fake)
		if err != nil {
			t.Fatalf("rows %d New failed: %v", rows, err)
		}

		res, err := eng.Play()
		if err != nil {
			t.Fatalf("rows %d Play failed: %v", rows, err)
		}

		if len(res.Path) != rows {
			t.Fatalf("rows %d: expected Path length %d, got %d", rows, rows, len(res.Path))
		}

		var rightCount int
		for i, dir := range res.Path {
			if dir != plinko.Left && dir != plinko.Right {
				t.Fatalf("rows %d step %d: invalid direction %v", rows, i, dir)
			}
			if dir == plinko.Right {
				rightCount++
			}
		}

		if res.BucketIndex != rightCount {
			t.Fatalf("rows %d: BucketIndex %d != rightCount %d", rows, res.BucketIndex, rightCount)
		}
		if res.BucketIndex < 0 || res.BucketIndex > rows {
			t.Fatalf("rows %d: BucketIndex %d out of range [0, %d]", rows, res.BucketIndex, rows)
		}

		// Verify Path slice is newly allocated and independent
		origDir := res.Path[0]
		res.Path[0] = plinko.Right
		if origDir == plinko.Right {
			res.Path[0] = plinko.Left
		}
		// Second play with same fake
		fake2 := &fakeEntropySource{decisions: decisions}
		eng2, err := plinko.New(plinko.Config{Rows: rows, Risk: plinko.Medium}, fake2)
		if err != nil {
			t.Fatalf("eng2 New failed: %v", err)
		}
		res2, err := eng2.Play()
		if err != nil {
			t.Fatalf("eng2 Play failed: %v", err)
		}
		if res2.Path[0] != origDir {
			t.Fatalf("modifying previous Result.Path affected subsequent play result")
		}
	}
}

// TestSourceErrorPropagation verifies that entropy source errors are wrapped and discoverable with errors.Is.
func TestSourceErrorPropagation(t *testing.T) {
	injectedErr := errors.New("entropy stream failure")

	t.Run("error on first step", func(t *testing.T) {
		fake := &fakeEntropySource{err: injectedErr}
		eng, err := plinko.New(plinko.Config{Rows: 8, Risk: plinko.Low}, fake)
		if err != nil {
			t.Fatalf("New failed: %v", err)
		}
		res, err := eng.Play()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, injectedErr) {
			t.Fatalf("error = %v, want wrapped %v", err, injectedErr)
		}
		if len(res.Path) != 0 || res.BucketIndex != 0 || res.Multiplier != 0 {
			t.Fatalf("expected zero Result on error, got %+v", res)
		}
	})

	t.Run("error midway through pins", func(t *testing.T) {
		fake := &fakeEntropySource{
			decisions: []uint64{0, 1, 0}, // 3 decisions then exhausted
		}
		eng, err := plinko.New(plinko.Config{Rows: 8, Risk: plinko.Low}, fake)
		if err != nil {
			t.Fatalf("New failed: %v", err)
		}
		res, err := eng.Play()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if len(res.Path) != 0 || res.BucketIndex != 0 || res.Multiplier != 0 {
			t.Fatalf("expected zero Result on error, got %+v", res)
		}
	})
}

// TestExactFakePathsAndBuckets verifies landing in every bucket k in [0, rows] with predetermined paths.
func TestExactFakePathsAndBuckets(t *testing.T) {
	const rows = 8
	for k := 0; k <= rows; k++ {
		decisions := make([]uint64, rows)
		// k rights followed by (rows - k) lefts
		for i := 0; i < k; i++ {
			decisions[i] = 1 // Right
		}
		for i := k; i < rows; i++ {
			decisions[i] = 0 // Left
		}

		fake := &fakeEntropySource{decisions: decisions}
		eng, err := plinko.New(plinko.Config{Rows: rows, Risk: plinko.Medium}, fake)
		if err != nil {
			t.Fatalf("bucket %d New failed: %v", k, err)
		}

		res, err := eng.Play()
		if err != nil {
			t.Fatalf("bucket %d Play failed: %v", k, err)
		}

		if res.BucketIndex != k {
			t.Fatalf("decisions for k=%d resulted in BucketIndex=%d", k, res.BucketIndex)
		}
		if fake.readCount != rows {
			t.Fatalf("expected %d reads, got %d", rows, fake.readCount)
		}
	}
}

// comb computes binomial coefficient C(n, k) for test assertions.
func testComb(n, k int) float64 {
	if k < 0 || k > n {
		return 0
	}
	if k == 0 || k == n {
		return 1
	}
	if k > n/2 {
		k = n - k
	}
	res := uint64(1)
	for i := 1; i <= k; i++ {
		res = (res * uint64(n-i+1)) / uint64(i)
	}
	return float64(res)
}

// getMultipliersForConfig collects unrounded multipliers for all buckets in [0, rows].
func getMultipliersForConfig(t *testing.T, rows int, risk plinko.RiskLevel) []float64 {
	t.Helper()
	mults := make([]float64, rows+1)
	for k := 0; k <= rows; k++ {
		decisions := make([]uint64, rows)
		for i := 0; i < k; i++ {
			decisions[i] = 1
		}
		fake := &fakeEntropySource{decisions: decisions}
		eng, err := plinko.New(plinko.Config{Rows: rows, Risk: risk}, fake)
		if err != nil {
			t.Fatalf("rows=%d risk=%v New failed: %v", rows, risk, err)
		}
		res, err := eng.Play()
		if err != nil {
			t.Fatalf("rows=%d risk=%v Play(k=%d) failed: %v", rows, risk, k, err)
		}
		if res.BucketIndex != k {
			t.Fatalf("expected bucket %d, got %d", k, res.BucketIndex)
		}
		mults[k] = res.Multiplier
	}
	return mults
}

// TestFormulaInvariantsAll27Configurations exhaustively tests symmetry, finite positivity,
// expected value 0.99 within 1e-12, and higher tail concentration by risk across all 27 configurations.
func TestFormulaInvariantsAll27Configurations(t *testing.T) {
	risks := []plinko.RiskLevel{plinko.Low, plinko.Medium, plinko.High}

	for rows := 8; rows <= 16; rows++ {
		var riskMults [3][]float64

		for rIdx, risk := range risks {
			mults := getMultipliersForConfig(t, rows, risk)
			riskMults[rIdx] = mults

			// 1. Finite positive outputs
			for k, m := range mults {
				if math.IsNaN(m) || math.IsInf(m, 0) || m <= 0 {
					t.Fatalf("rows=%d risk=%v bucket=%d: invalid multiplier %v", rows, risk, k, m)
				}
			}

			// 2. Exact symmetry
			for k := 0; k <= rows/2; k++ {
				symK := rows - k
				if mults[k] != mults[symK] {
					t.Fatalf("rows=%d risk=%v: asymmetry at bucket %d (%v) != bucket %d (%v)",
						rows, risk, k, mults[k], symK, mults[symK])
				}
			}

			// 3. Expected value 0.99 within 1e-12
			twoPowRows := math.Pow(2, float64(rows))
			var ev float64
			for k := 0; k <= rows; k++ {
				pk := testComb(rows, k) / twoPowRows
				ev += pk * mults[k]
			}
			diff := math.Abs(ev - 0.99)
			if diff > 1e-12 {
				t.Fatalf("rows=%d risk=%v: expected value diff %e exceeds 1e-12 (EV=%v)", rows, risk, diff, ev)
			}
		}

		// 4. Higher tail concentration by risk:
		// Edge multiplier (k=0 and k=rows) strictly increases with risk: Low < Medium < High
		lowEdge := riskMults[0][0]
		medEdge := riskMults[1][0]
		highEdge := riskMults[2][0]
		if !(lowEdge < medEdge && medEdge < highEdge) {
			t.Fatalf("rows=%d: edge multipliers not strictly increasing with risk: Low=%v Med=%v High=%v",
				rows, lowEdge, medEdge, highEdge)
		}

		// Center multiplier (k=rows/2) strictly decreases with risk: Low > Medium > High
		centerIdx := rows / 2
		lowCenter := riskMults[0][centerIdx]
		medCenter := riskMults[1][centerIdx]
		highCenter := riskMults[2][centerIdx]
		if !(lowCenter > medCenter && medCenter > highCenter) {
			t.Fatalf("rows=%d: center multipliers not strictly decreasing with risk: Low=%v Med=%v High=%v",
				rows, lowCenter, medCenter, highCenter)
		}
	}
}

// TestPlaySeededValidation verifies that PlaySeeded validates empty seeds and invalid configs
// with documented sentinel errors before consuming entropy.
func TestPlaySeededValidation(t *testing.T) {
	tests := []struct {
		name    string
		input   plinko.SeededInput
		wantErr error
	}{
		{
			name: "empty server seed",
			input: plinko.SeededInput{
				ServerSeed: "",
				ClientSeed: "client",
				Nonce:      1,
				Rows:       8,
				Risk:       plinko.Low,
			},
			wantErr: plinko.ErrEmptyServerSeed,
		},
		{
			name: "empty client seed",
			input: plinko.SeededInput{
				ServerSeed: "server",
				ClientSeed: "",
				Nonce:      1,
				Rows:       8,
				Risk:       plinko.Low,
			},
			wantErr: plinko.ErrEmptyClientSeed,
		},
		{
			name: "rows below minimum (7)",
			input: plinko.SeededInput{
				ServerSeed: "server",
				ClientSeed: "client",
				Nonce:      1,
				Rows:       7,
				Risk:       plinko.Low,
			},
			wantErr: plinko.ErrInvalidRows,
		},
		{
			name: "rows above maximum (17)",
			input: plinko.SeededInput{
				ServerSeed: "server",
				ClientSeed: "client",
				Nonce:      1,
				Rows:       17,
				Risk:       plinko.Low,
			},
			wantErr: plinko.ErrInvalidRows,
		},
		{
			name: "invalid risk (3)",
			input: plinko.SeededInput{
				ServerSeed: "server",
				ClientSeed: "client",
				Nonce:      1,
				Rows:       8,
				Risk:       plinko.RiskLevel(3),
			},
			wantErr: plinko.ErrInvalidRisk,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, err := plinko.PlaySeeded(tc.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
			if len(res.Path) != 0 || res.BucketIndex != 0 || res.Multiplier != 0 {
				t.Fatalf("expected zero Result on error, got %+v", res)
			}
		})
	}
}

// TestPlaySeededDeterminismAndReplay verifies that identical seeds and nonce reproduce
// the exact round result, and varying nonce or seeds yields different streams.
func TestPlaySeededDeterminismAndReplay(t *testing.T) {
	input := plinko.SeededInput{
		ServerSeed: "server-secret-alpha",
		ClientSeed: "client-pub-beta",
		Nonce:      42,
		Rows:       12,
		Risk:       plinko.High,
	}

	res1, err := plinko.PlaySeeded(input)
	if err != nil {
		t.Fatalf("first PlaySeeded failed: %v", err)
	}

	res2, err := plinko.PlaySeeded(input)
	if err != nil {
		t.Fatalf("replay PlaySeeded failed: %v", err)
	}

	if res1.BucketIndex != res2.BucketIndex {
		t.Fatalf("BucketIndex mismatch on replay: %d != %d", res1.BucketIndex, res2.BucketIndex)
	}
	if res1.Multiplier != res2.Multiplier {
		t.Fatalf("Multiplier mismatch on replay: %v != %v", res1.Multiplier, res2.Multiplier)
	}
	if len(res1.Path) != len(res2.Path) {
		t.Fatalf("Path length mismatch on replay: %d != %d", len(res1.Path), len(res2.Path))
	}
	for i := range res1.Path {
		if res1.Path[i] != res2.Path[i] {
			t.Fatalf("step %d mismatch on replay: %v != %v", i, res1.Path[i], res2.Path[i])
		}
	}

	// Changing nonce separates streams
	inputNonce2 := input
	inputNonce2.Nonce = 43
	resNonce2, err := plinko.PlaySeeded(inputNonce2)
	if err != nil {
		t.Fatalf("PlaySeeded with different nonce failed: %v", err)
	}
	samePath := true
	for i := range res1.Path {
		if res1.Path[i] != resNonce2.Path[i] {
			samePath = false
			break
		}
	}
	if samePath && res1.BucketIndex == resNonce2.BucketIndex {
		t.Logf("Notice: path happened to match across nonces 42 and 43 (unlikely for 12 rows)")
	}

	// Changing server seed separates streams
	inputDiffServer := input
	inputDiffServer.ServerSeed = "server-secret-different"
	resDiffServer, err := plinko.PlaySeeded(inputDiffServer)
	if err != nil {
		t.Fatalf("PlaySeeded with different server seed failed: %v", err)
	}
	diffServerMatches := true
	for i := range res1.Path {
		if res1.Path[i] != resDiffServer.Path[i] {
			diffServerMatches = false
			break
		}
	}
	if diffServerMatches && res1.BucketIndex == resDiffServer.BucketIndex {
		t.Errorf("expected different stream with different server seed")
	}
}

// TestPlaySeededDeterministicVector pins an exact compatibility vector for Plinko replay.
func TestPlaySeededDeterministicVector(t *testing.T) {
	input := plinko.SeededInput{
		ServerSeed: "test-server-seed",
		ClientSeed: "test-client-seed",
		Nonce:      1,
		Rows:       8,
		Risk:       plinko.Medium,
	}

	res, err := plinko.PlaySeeded(input)
	if err != nil {
		t.Fatalf("PlaySeeded failed: %v", err)
	}

	expectedPath := []plinko.Direction{
		plinko.Right,
		plinko.Right,
		plinko.Right,
		plinko.Right,
		plinko.Right,
		plinko.Right,
		plinko.Left,
		plinko.Right,
	}
	expectedBucket := 7
	const expectedMultiplier = 2.4541602403852725

	if len(res.Path) != len(expectedPath) {
		t.Fatalf("Path length = %d, want %d", len(res.Path), len(expectedPath))
	}
	for i, dir := range res.Path {
		if dir != expectedPath[i] {
			t.Fatalf("Path[%d] = %v, want %v", i, dir, expectedPath[i])
		}
	}

	if res.BucketIndex != expectedBucket {
		t.Fatalf("BucketIndex = %d, want %d", res.BucketIndex, expectedBucket)
	}

	if math.Abs(res.Multiplier-expectedMultiplier) > 1e-15 {
		t.Fatalf("Multiplier = %v, want %v", res.Multiplier, expectedMultiplier)
	}
}

// TestStringers verifies Direction and RiskLevel string formatting.
func TestStringers(t *testing.T) {
	if plinko.Left.String() != "Left" {
		t.Fatalf("Left.String() = %q, want Left", plinko.Left.String())
	}
	if plinko.Right.String() != "Right" {
		t.Fatalf("Right.String() = %q, want Right", plinko.Right.String())
	}
	if plinko.Direction(99).String() != "Direction(99)" {
		t.Fatalf("Direction(99).String() = %q", plinko.Direction(99).String())
	}

	if plinko.Low.String() != "Low" {
		t.Fatalf("Low.String() = %q, want Low", plinko.Low.String())
	}
	if plinko.Medium.String() != "Medium" {
		t.Fatalf("Medium.String() = %q, want Medium", plinko.Medium.String())
	}
	if plinko.High.String() != "High" {
		t.Fatalf("High.String() = %q, want High", plinko.High.String())
	}
	if plinko.RiskLevel(99).String() != "RiskLevel(99)" {
		t.Fatalf("RiskLevel(99).String() = %q", plinko.RiskLevel(99).String())
	}
}
