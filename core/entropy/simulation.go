package entropy

import (
	"math/rand/v2"
)

var _ Source = (*SimulationSource)(nil)

// SimulationSource is a high-throughput, deterministic pseudo-random entropy source
// backed by a standard-library Permuted Congruential Generator (math/rand/v2 PCG).
//
// WARNING: SimulationSource is NOT cryptographically secure and is NOT part of the
// qxprob provably-fair protocol. It must NEVER be used for production game outcomes,
// provably-fair verification, real-money wagering, or security-sensitive decisions.
// It is provided exclusively for offline Monte Carlo simulations, high-volume statistical
// analysis, and fast unit testing where cryptographic guarantees are unnecessary.
type SimulationSource struct {
	pcg rand.PCG
}

// NewSimulationSource creates an explicit deterministic SimulationSource initialized
// with seed1 and seed2. It is strictly for simulation and testing; never use in production.
func NewSimulationSource(seed1, seed2 uint64) *SimulationSource {
	s := &SimulationSource{}
	s.pcg.Seed(seed1, seed2)
	return s
}

// Reset re-initializes the PCG generator state with seed1 and seed2.
// It is not safe for concurrent use.
func (s *SimulationSource) Reset(seed1, seed2 uint64) {
	s.pcg.Seed(seed1, seed2)
}

// Uint64 returns a uniformly distributed 64-bit unsigned integer.
func (s *SimulationSource) Uint64() (uint64, error) {
	return s.pcg.Uint64(), nil
}

// Bits returns a uniformly distributed integer in [0, 2^n).
func (s *SimulationSource) Bits(n int) (uint64, error) {
	if n < 1 || n > 64 {
		return 0, ErrInvalidBitCount
	}
	v := s.pcg.Uint64()
	if n == 64 {
		return v, nil
	}
	return v & (uint64(1)<<n - 1), nil
}

// Float64 returns one of 2^53 equally likely values in the interval [0, 1).
func (s *SimulationSource) Float64() (float64, error) {
	v, err := s.Bits(53)
	if err != nil {
		return 0, err
	}
	return float64(v) / float64(uint64(1)<<53), nil
}

// Intn returns a uniformly distributed integer in [0, n), without modulo bias,
// using the package's standard rejection sampling algorithm.
func (s *SimulationSource) Intn(n uint64) (uint64, error) {
	return intn(s, n)
}
