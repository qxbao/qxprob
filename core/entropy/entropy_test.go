package entropy

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"testing"
)

// TestProvablyFairDeterminism verifies that identical inputs and calls reproduce a stream.
func TestProvablyFairDeterminism(t *testing.T) {
	first := NewProvablyFairSource("server-seed", "client-seed", 42)
	second := NewProvablyFairSource("server-seed", "client-seed", 42)

	firstUint64, err := first.Uint64()
	if err != nil {
		t.Fatal(err)
	}
	secondUint64, err := second.Uint64()
	if err != nil {
		t.Fatal(err)
	}
	if firstUint64 != secondUint64 {
		t.Fatalf("Uint64 differs: %d != %d", firstUint64, secondUint64)
	}

	firstBits, err := first.Bits(53)
	if err != nil {
		t.Fatal(err)
	}
	secondBits, err := second.Bits(53)
	if err != nil {
		t.Fatal(err)
	}
	if firstBits != secondBits {
		t.Fatalf("Bits differs: %d != %d", firstBits, secondBits)
	}
}

// TestProvablyFairCounterBlocks verifies that the stream advances beyond one digest.
func TestProvablyFairCounterBlocks(t *testing.T) {
	const serverSeed = "server-seed"
	const clientSeed = "client-seed"
	const nonce = 42
	source := NewProvablyFairSource(serverSeed, clientSeed, nonce)

	for index := uint64(0); index < 5; index++ {
		value, err := source.Uint64()
		if err != nil {
			t.Fatal(err)
		}
		block := testHMACBlock(serverSeed, clientSeed, nonce, index/4)
		expected := binary.BigEndian.Uint64(block[(index%4)*8:])
		if value != expected {
			t.Fatalf("value %d = %x, want %x", index, value, expected)
		}
	}
}

// TestProvablyFairBits verifies supported bit widths and invalid bit-count handling.
func TestProvablyFairBits(t *testing.T) {
	for _, bitCount := range []int{1, 32, 33, 53, 64} {
		source := NewProvablyFairSource("server", "client", 1)
		value, err := source.Bits(bitCount)
		if err != nil {
			t.Fatalf("Bits(%d): %v", bitCount, err)
		}
		if bitCount < 64 && value >= uint64(1)<<bitCount {
			t.Fatalf("Bits(%d) = %d is out of range", bitCount, value)
		}
	}

	source := NewProvablyFairSource("server", "client", 1)
	for _, bitCount := range []int{-1, 0, 65} {
		if _, err := source.Bits(bitCount); !errors.Is(err, ErrInvalidBitCount) {
			t.Fatalf("Bits(%d) error = %v, want ErrInvalidBitCount", bitCount, err)
		}
	}
}

// TestProvablyFairMethods exercises all public sampling methods and their bounds.
func TestProvablyFairMethods(t *testing.T) {
	source := NewProvablyFairSource("server", "client", 9)

	value, err := source.Float64()
	if err != nil {
		t.Fatal(err)
	}
	if value < 0 || value >= 1 {
		t.Fatalf("Float64() = %f is out of range", value)
	}

	bounded, err := source.Intn(17)
	if err != nil {
		t.Fatal(err)
	}
	if bounded >= 17 {
		t.Fatalf("Intn(17) = %d is out of range", bounded)
	}

	if _, err := source.Intn(0); !errors.Is(err, ErrInvalidBound) {
		t.Fatalf("Intn(0) error = %v, want ErrInvalidBound", err)
	}
}

// TestProvablyFairReadAcrossBlockBoundary verifies that reads remain contiguous
// when one value spans two HMAC blocks.
func TestProvablyFairReadAcrossBlockBoundary(t *testing.T) {
	const serverSeed = "boundary-server"
	const clientSeed = "boundary-client"
	const nonce = 13

	source := NewProvablyFairSource(serverSeed, clientSeed, nonce)
	for i := 0; i < hmacBlockSize-1; i++ {
		if _, err := source.Bits(8); err != nil {
			t.Fatal(err)
		}
	}

	value, err := source.Uint64()
	if err != nil {
		t.Fatal(err)
	}
	first := testHMACBlock(serverSeed, clientSeed, nonce, 0)
	second := testHMACBlock(serverSeed, clientSeed, nonce, 1)
	var expectedBytes [8]byte
	expectedBytes[0] = first[hmacBlockSize-1]
	copy(expectedBytes[1:], second[:7])
	expected := binary.BigEndian.Uint64(expectedBytes[:])
	if value != expected {
		t.Fatalf("cross-block Uint64() = %x, want %x", value, expected)
	}
}

// TestProvablyFairExhaustion verifies the final counter block and the stable
// exported error returned after the stream is exhausted.
func TestProvablyFairExhaustion(t *testing.T) {
	source := NewProvablyFairSource("server", "client", 1)
	source.counter = ^uint64(0)

	if err := source.refill(); err != nil {
		t.Fatalf("final refill: %v", err)
	}
	if !source.exhausted {
		t.Fatal("source was not marked exhausted after the final counter block")
	}

	source.offset = hmacBlockSize
	if _, err := source.Uint64(); !errors.Is(err, ErrEntropyExhausted) {
		t.Fatalf("Uint64() error = %v, want ErrEntropyExhausted", err)
	}

	for _, test := range []struct {
		name string
		call func(*ProvablyFairSource) error
	}{
		{name: "Bits", call: func(source *ProvablyFairSource) error {
			_, err := source.Bits(8)
			return err
		}},
		{name: "Float64", call: func(source *ProvablyFairSource) error {
			_, err := source.Float64()
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			exhausted := NewProvablyFairSource("server", "client", 1)
			exhausted.exhausted = true
			if err := test.call(exhausted); !errors.Is(err, ErrEntropyExhausted) {
				t.Fatalf("error = %v, want ErrEntropyExhausted", err)
			}
		})
	}
}

// TestIntnRejectsBiasedPrefix verifies rejection sampling before applying a modulus.
func TestIntnRejectsBiasedPrefix(t *testing.T) {
	source := &testUint64Source{values: []uint64{0, 5}}
	value, err := intn(source, 3)
	if err != nil {
		t.Fatal(err)
	}
	if value != 2 {
		t.Fatalf("Intn result = %d, want 2", value)
	}
	if source.index != 2 {
		t.Fatalf("source reads = %d, want 2", source.index)
	}
	if _, err := intn(source, 0); !errors.Is(err, ErrInvalidBound) {
		t.Fatalf("Intn(0) error = %v, want ErrInvalidBound", err)
	}
}

// TestIntnPropagatesSourceError verifies that rejection sampling preserves the
// underlying entropy failure.
func TestIntnPropagatesSourceError(t *testing.T) {
	wantErr := errors.New("source failed")
	if _, err := intn(errorUint64Source{err: wantErr}, 3); !errors.Is(err, wantErr) {
		t.Fatalf("intn() error = %v, want wrapped source error", err)
	}
}

// TestCryptoSource verifies the secure source's output bounds and interface conformance.
func TestCryptoSource(t *testing.T) {
	var source Source = NewCryptoSource()
	value, err := source.Float64()
	if err != nil {
		t.Fatal(err)
	}
	if value < 0 || value >= 1 {
		t.Fatalf("Float64() = %f is out of range", value)
	}
	for _, bitCount := range []int{1, 32, 53, 64} {
		value, err := source.Bits(bitCount)
		if err != nil {
			t.Fatalf("Bits(%d): %v", bitCount, err)
		}
		if bitCount < 64 && value >= uint64(1)<<bitCount {
			t.Fatalf("Bits(%d) = %d is out of range", bitCount, value)
		}
	}
	if _, err := source.Uint64(); err != nil {
		t.Fatalf("Uint64(): %v", err)
	}
	valueN, err := source.Intn(11)
	if err != nil {
		t.Fatalf("Intn(11): %v", err)
	}
	if valueN >= 11 {
		t.Fatalf("Intn(11) = %d is out of range", valueN)
	}
	for _, bitCount := range []int{-1, 0, 65} {
		if _, err := source.Bits(bitCount); !errors.Is(err, ErrInvalidBitCount) {
			t.Fatalf("Bits(%d) error = %v, want ErrInvalidBitCount", bitCount, err)
		}
	}
	if _, err := source.Intn(0); !errors.Is(err, ErrInvalidBound) {
		t.Fatalf("Intn(0) error = %v, want ErrInvalidBound", err)
	}

	var zeroValue CryptoSource
	if _, err := zeroValue.Uint64(); err != nil {
		t.Fatalf("zero-value Uint64(): %v", err)
	}
}

// TestCryptoSourceFailure verifies that crypto/rand failures are returned by
// every public method that reads entropy.
func TestCryptoSourceFailure(t *testing.T) {
	source := &CryptoSource{reader: errorReader{}}
	tests := []struct {
		name string
		call func() error
	}{
		{name: "Float64", call: func() error { _, err := source.Float64(); return err }},
		{name: "Bits", call: func() error { _, err := source.Bits(8); return err }},
		{name: "Uint64", call: func() error { _, err := source.Uint64(); return err }},
		{name: "Intn", call: func() error { _, err := source.Intn(7); return err }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); !errors.Is(err, io.ErrUnexpectedEOF) {
				t.Fatalf("error = %v, want io.ErrUnexpectedEOF", err)
			}
		})
	}
}

// testHMACBlock independently encodes one documented provably-fair HMAC block.
func testHMACBlock(serverSeed, clientSeed string, nonce, counter uint64) []byte {
	hash := hmac.New(sha256.New, []byte(serverSeed))
	hash.Write([]byte(hmacDomain))
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(clientSeed)))
	hash.Write(length[:])
	hash.Write([]byte(clientSeed))
	var fields [16]byte
	binary.BigEndian.PutUint64(fields[:8], nonce)
	binary.BigEndian.PutUint64(fields[8:], counter)
	hash.Write(fields[:])
	return hash.Sum(nil)
}

// testUint64Source provides predetermined values for rejection-sampling tests.
type testUint64Source struct {
	values []uint64
	index  int
}

type errorUint64Source struct {
	err error
}

func (s errorUint64Source) Uint64() (uint64, error) {
	return 0, s.err
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}

// Uint64 returns the next predetermined value.
func (s *testUint64Source) Uint64() (uint64, error) {
	value := s.values[s.index]
	s.index++
	return value, nil
}

// TestProvablyFairResetNonceEquivalence verifies that ResetNonce produces streams
// byte-for-byte identical to fresh instances across all public sampling methods and the HMAC oracle.
func TestProvablyFairResetNonceEquivalence(t *testing.T) {
	const serverSeed = "reusable-server-seed"
	const clientSeed = "reusable-client-seed"

	reused := NewProvablyFairSource(serverSeed, clientSeed, 0)

	testNonces := []uint64{1, 2, 42, 100, 1000, 99999, ^uint64(0) - 50}

	for _, nonce := range testNonces {
		// Advance reused source with some random calls to leave dirty state
		_, _ = reused.Uint64()
		_, _ = reused.Bits(17)

		// Reset to target nonce
		reused.ResetNonce(nonce)
		fresh := NewProvablyFairSource(serverSeed, clientSeed, nonce)

		// 1. Verify against independent HMAC oracle
		reusedU64, err := reused.Uint64()
		if err != nil {
			t.Fatalf("nonce %d reused Uint64: %v", nonce, err)
		}
		freshU64, err := fresh.Uint64()
		if err != nil {
			t.Fatalf("nonce %d fresh Uint64: %v", nonce, err)
		}
		oracleBlock := testHMACBlock(serverSeed, clientSeed, nonce, 0)
		expectedOracleU64 := binary.BigEndian.Uint64(oracleBlock[:8])
		if reusedU64 != freshU64 || reusedU64 != expectedOracleU64 {
			t.Fatalf("nonce %d: reused=%x fresh=%x oracle=%x", nonce, reusedU64, freshU64, expectedOracleU64)
		}

		// 2. Verify Uint64 sequence
		for i := 0; i < 8; i++ {
			rVal, err := reused.Uint64()
			if err != nil {
				t.Fatal(err)
			}
			fVal, err := fresh.Uint64()
			if err != nil {
				t.Fatal(err)
			}
			if rVal != fVal {
				t.Fatalf("nonce %d Uint64[%d]: reused %d != fresh %d", nonce, i, rVal, fVal)
			}
		}

		// 3. Verify Bits sequence with varying bit widths
		bitWidths := []int{1, 5, 8, 16, 31, 32, 53, 64}
		for _, w := range bitWidths {
			rVal, err := reused.Bits(w)
			if err != nil {
				t.Fatal(err)
			}
			fVal, err := fresh.Bits(w)
			if err != nil {
				t.Fatal(err)
			}
			if rVal != fVal {
				t.Fatalf("nonce %d Bits(%d): reused %d != fresh %d", nonce, w, rVal, fVal)
			}
		}

		// 4. Verify Float64 sequence
		for i := 0; i < 5; i++ {
			rVal, err := reused.Float64()
			if err != nil {
				t.Fatal(err)
			}
			fVal, err := fresh.Float64()
			if err != nil {
				t.Fatal(err)
			}
			if rVal != fVal {
				t.Fatalf("nonce %d Float64[%d]: reused %f != fresh %f", nonce, i, rVal, fVal)
			}
		}

		// 5. Verify Intn sequence
		bounds := []uint64{2, 6, 10, 37, 100, 10000}
		for _, b := range bounds {
			rVal, err := reused.Intn(b)
			if err != nil {
				t.Fatal(err)
			}
			fVal, err := fresh.Intn(b)
			if err != nil {
				t.Fatal(err)
			}
			if rVal != fVal {
				t.Fatalf("nonce %d Intn(%d): reused %d != fresh %d", nonce, b, rVal, fVal)
			}
		}
	}
}

// TestProvablyFairResetNonceAfterPartialReadsAndBoundaries verifies that resetting
// after partial-block consumption and cross-block reads properly discards previous position.
func TestProvablyFairResetNonceAfterPartialReadsAndBoundaries(t *testing.T) {
	const serverSeed = "partial-server"
	const clientSeed = "partial-client"

	source := NewProvablyFairSource(serverSeed, clientSeed, 1)

	// Partial read (consume 7 bits = 1 byte read, offset = 1)
	if _, err := source.Bits(7); err != nil {
		t.Fatal(err)
	}

	// Reset to nonce 2
	source.ResetNonce(2)
	fresh := NewProvablyFairSource(serverSeed, clientSeed, 2)

	rBytes := make([]byte, 8)
	fBytes := make([]byte, 8)
	rVal, err := source.Uint64()
	if err != nil {
		t.Fatal(err)
	}
	fVal, err := fresh.Uint64()
	if err != nil {
		t.Fatal(err)
	}
	binary.BigEndian.PutUint64(rBytes, rVal)
	binary.BigEndian.PutUint64(fBytes, fVal)
	if !bytesEqual(rBytes, fBytes) {
		t.Fatalf("after partial read: reused %x != fresh %x", rBytes, fBytes)
	}

	// Cross-block boundary read (consume 35 bytes -> crosses into block 1)
	for i := 0; i < 35; i++ {
		if _, err := source.Bits(8); err != nil {
			t.Fatal(err)
		}
	}

	// Reset to nonce 3
	source.ResetNonce(3)
	fresh3 := NewProvablyFairSource(serverSeed, clientSeed, 3)
	rVal3, err := source.Uint64()
	if err != nil {
		t.Fatal(err)
	}
	fVal3, err := fresh3.Uint64()
	if err != nil {
		t.Fatal(err)
	}
	if rVal3 != fVal3 {
		t.Fatalf("after cross-block: reused %x != fresh %x", rVal3, fVal3)
	}
}

// TestProvablyFairResetNonceAfterExhaustion verifies that ResetNonce recovers from
// exhausted state and restores full counter capacity.
func TestProvablyFairResetNonceAfterExhaustion(t *testing.T) {
	source := NewProvablyFairSource("server", "client", 1)
	source.counter = ^uint64(0)
	if err := source.refill(); err != nil {
		t.Fatalf("refill failed: %v", err)
	}
	source.offset = hmacBlockSize

	// Must return exhausted error
	if _, err := source.Uint64(); !errors.Is(err, ErrEntropyExhausted) {
		t.Fatalf("expected ErrEntropyExhausted, got %v", err)
	}

	// ResetNonce must clear exhausted and reset counter
	source.ResetNonce(10)
	fresh := NewProvablyFairSource("server", "client", 10)

	got, err := source.Uint64()
	if err != nil {
		t.Fatalf("unexpected error after ResetNonce: %v", err)
	}
	want, err := fresh.Uint64()
	if err != nil {
		t.Fatalf("fresh error: %v", err)
	}
	if got != want {
		t.Fatalf("got %x, want %x", got, want)
	}
}

// TestProvablyFairResetNonceSeedIsolation verifies that independent sources do not leak state.
func TestProvablyFairResetNonceSeedIsolation(t *testing.T) {
	sourceA := NewProvablyFairSource("server-A", "client-A", 1)
	sourceB := NewProvablyFairSource("server-B", "client-B", 1)

	valA, err := sourceA.Uint64()
	if err != nil {
		t.Fatal(err)
	}
	valB, err := sourceB.Uint64()
	if err != nil {
		t.Fatal(err)
	}
	if valA == valB {
		t.Fatalf("distinct seeds produced identical Uint64: %x", valA)
	}

	sourceA.ResetNonce(2)
	sourceB.ResetNonce(2)

	valA2, err := sourceA.Uint64()
	if err != nil {
		t.Fatal(err)
	}
	valB2, err := sourceB.Uint64()
	if err != nil {
		t.Fatal(err)
	}
	if valA2 == valB2 {
		t.Fatalf("distinct seeds produced identical Uint64 after ResetNonce: %x", valA2)
	}
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func BenchmarkProvablyFairSource_Intn(b *testing.B) {
	src := NewProvablyFairSource("benchmark-server-seed", "benchmark-client-seed", 1)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := src.Intn(100); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkProvablyFairSource_ResetNonce(b *testing.B) {
	src := NewProvablyFairSource("benchmark-server-seed", "benchmark-client-seed", 1)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		src.ResetNonce(uint64(i))
		if _, err := src.Uint64(); err != nil {
			b.Fatal(err)
		}
	}
}
