// Package entropy provides unbiased random values for probability-game engines.
package entropy

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"io"
)

const (
	// hmacDomain separates this stream format from other HMAC uses of a seed.
	hmacDomain = "qxprob/entropy/v1\x00"

	// hmacBlockSize is the number of bytes emitted by SHA-256 per counter block.
	hmacBlockSize = sha256.Size
)

var (
	// ErrEntropyExhausted indicates that a deterministic stream has emitted all counter blocks.
	ErrEntropyExhausted = errors.New("entropy stream exhausted")

	// ErrInvalidBitCount indicates that a requested bit count is outside [1, 64].
	ErrInvalidBitCount = errors.New("bit count must be between 1 and 64")

	// ErrInvalidBound indicates that Intn was called with a zero upper bound.
	ErrInvalidBound = errors.New("upper bound must be greater than zero")
)

// Source supplies uniformly distributed random values. Implementations with mutable
// state are not safe for concurrent use unless their documentation says otherwise.
type Source interface {
	// Float64 returns one of 2^53 equally likely values in the interval [0, 1).
	Float64() (float64, error)
	// Bits returns a uniformly distributed integer in [0, 2^n).
	Bits(n int) (uint64, error)
	// Uint64 returns a uniformly distributed 64-bit unsigned integer.
	Uint64() (uint64, error)
	// Intn returns a uniformly distributed integer in [0, n), without modulo bias.
	Intn(n uint64) (uint64, error)
}

// CryptoSource obtains entropy from crypto/rand.Reader. It is safe for concurrent use.
// Its zero value is ready for use.
type CryptoSource struct {
	reader io.Reader
}

// NewCryptoSource creates a cryptographically secure entropy source.
func NewCryptoSource() *CryptoSource {
	return &CryptoSource{reader: rand.Reader}
}

// Float64 returns one of 2^53 equally likely values in [0, 1).
func (c *CryptoSource) Float64() (float64, error) {
	value, err := c.Bits(53)
	if err != nil {
		return 0, err
	}
	return float64(value) / float64(uint64(1)<<53), nil
}

// Bits returns a uniformly distributed integer in [0, 2^n).
func (c *CryptoSource) Bits(n int) (uint64, error) {
	if n < 1 || n > 64 {
		return 0, ErrInvalidBitCount
	}
	value, err := c.Uint64()
	if err != nil {
		return 0, err
	}
	if n == 64 {
		return value, nil
	}
	return value & (uint64(1)<<n - 1), nil
}

// Uint64 returns a uniformly distributed 64-bit unsigned integer.
func (c *CryptoSource) Uint64() (uint64, error) {
	var bytes [8]byte
	reader := c.reader
	if reader == nil {
		reader = rand.Reader
	}
	if _, err := io.ReadFull(reader, bytes[:]); err != nil {
		return 0, fmt.Errorf("crypto source failure: %w", err)
	}
	return binary.BigEndian.Uint64(bytes[:]), nil
}

// Intn returns a uniformly distributed integer in [0, n), without modulo bias.
func (c *CryptoSource) Intn(n uint64) (uint64, error) {
	return intn(c, n)
}

// ProvablyFairSource derives a deterministic HMAC-SHA-256 byte stream from a
// server seed, client seed, and nonce. Use one instance per game round; it is not
// safe for concurrent use.
type ProvablyFairSource struct {
	serverSeed []byte
	clientSeed []byte
	prefix     []byte
	hash       hash.Hash
	nonce      uint64
	counter    uint64
	block      [hmacBlockSize]byte
	fields     [16]byte
	offset     int
	exhausted  bool
}

// NewProvablyFairSource creates a reproducible entropy source. Its byte format is
// HMAC-SHA-256(serverSeed, "qxprob/entropy/v1" || 0x00 || len(clientSeed) ||
// clientSeed || nonce || counter), where integer fields are unsigned big-endian.
func NewProvablyFairSource(serverSeed, clientSeed string, nonce uint64) *ProvablyFairSource {
	sBytes := []byte(serverSeed)
	cBytes := []byte(clientSeed)
	prefix := make([]byte, len(hmacDomain)+8+len(cBytes))
	copy(prefix, hmacDomain)
	binary.BigEndian.PutUint64(prefix[len(hmacDomain):], uint64(len(cBytes)))
	copy(prefix[len(hmacDomain)+8:], cBytes)

	return &ProvablyFairSource{
		serverSeed: sBytes,
		clientSeed: cBytes,
		prefix:     prefix,
		hash:       hmac.New(sha256.New, sBytes),
		nonce:      nonce,
		offset:     hmacBlockSize,
	}
}

// ResetNonce resets the stream position, counter, and exhaustion state for a new round
// with the given nonce, while retaining the existing seed material and HMAC keying.
// After ResetNonce, the stream produces bytes identical to NewProvablyFairSource with the same nonce.
// It is not safe for concurrent use.
func (p *ProvablyFairSource) ResetNonce(nonce uint64) {
	p.nonce = nonce
	p.counter = 0
	p.offset = hmacBlockSize
	p.exhausted = false
	clear(p.block[:])
}

// Float64 returns one of 2^53 equally likely values in [0, 1).
func (p *ProvablyFairSource) Float64() (float64, error) {
	value, err := p.Bits(53)
	if err != nil {
		return 0, err
	}
	return float64(value) / float64(uint64(1)<<53), nil
}

// Bits returns a uniformly distributed integer in [0, 2^n).
func (p *ProvablyFairSource) Bits(n int) (uint64, error) {
	if n < 1 || n > 64 {
		return 0, ErrInvalidBitCount
	}

	byteCount := (n + 7) / 8
	var bytes [8]byte
	if err := p.read(bytes[8-byteCount:]); err != nil {
		return 0, err
	}
	value := binary.BigEndian.Uint64(bytes[:])
	if n == 64 {
		return value, nil
	}
	return value & (uint64(1)<<n - 1), nil
}

// Uint64 returns a uniformly distributed 64-bit unsigned integer.
func (p *ProvablyFairSource) Uint64() (uint64, error) {
	var bytes [8]byte
	if err := p.read(bytes[:]); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(bytes[:]), nil
}

// Intn returns a uniformly distributed integer in [0, n), without modulo bias.
func (p *ProvablyFairSource) Intn(n uint64) (uint64, error) {
	return intn(p, n)
}

// read fills dst with consecutive bytes from the HMAC counter stream.
func (p *ProvablyFairSource) read(dst []byte) error {
	for len(dst) > 0 {
		if p.offset == len(p.block) {
			if err := p.refill(); err != nil {
				return err
			}
		}
		copied := copy(dst, p.block[p.offset:])
		p.offset += copied
		dst = dst[copied:]
	}
	return nil
}

// refill generates the next HMAC-SHA-256 counter block.
func (p *ProvablyFairSource) refill() error {
	if p.exhausted {
		return ErrEntropyExhausted
	}

	if p.hash == nil {
		p.hash = hmac.New(sha256.New, p.serverSeed)
	}
	if p.prefix == nil {
		p.prefix = make([]byte, len(hmacDomain)+8+len(p.clientSeed))
		copy(p.prefix, hmacDomain)
		binary.BigEndian.PutUint64(p.prefix[len(hmacDomain):], uint64(len(p.clientSeed)))
		copy(p.prefix[len(hmacDomain)+8:], p.clientSeed)
	}

	p.hash.Reset()
	p.hash.Write(p.prefix)
	binary.BigEndian.PutUint64(p.fields[:8], p.nonce)
	binary.BigEndian.PutUint64(p.fields[8:], p.counter)
	p.hash.Write(p.fields[:])
	p.hash.Sum(p.block[:0])
	p.offset = 0
	if p.counter == ^uint64(0) {
		p.exhausted = true
	} else {
		p.counter++
	}
	return nil
}

// uint64Source is the minimum entropy source required for unbiased bounded sampling.
type uint64Source interface {
	Uint64() (uint64, error)
}

// intn samples a bound with rejection sampling and avoids modulo bias.
func intn(source uint64Source, n uint64) (uint64, error) {
	if n == 0 {
		return 0, ErrInvalidBound
	}
	threshold := -n % n
	for {
		value, err := source.Uint64()
		if err != nil {
			return 0, err
		}
		if value >= threshold {
			return value % n, nil
		}
	}
}
