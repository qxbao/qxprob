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
