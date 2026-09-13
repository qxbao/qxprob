package slot

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// MultiplierScale is the fixed-point scaling factor (1x = 1,000,000 micro-units).
const MultiplierScale int64 = 1_000_000

var (
	// ErrNegativeMultiplier indicates a negative multiplier value was provided.
	ErrNegativeMultiplier = errors.New("slot: negative multiplier")
	// ErrMultiplierOverflow indicates an arithmetic overflow in multiplier calculation.
	ErrMultiplierOverflow = errors.New("slot: multiplier overflow")
	// ErrInvalidMultiplierFormat indicates a malformed multiplier string.
	ErrInvalidMultiplierFormat = errors.New("slot: invalid multiplier format")
	// ErrNegativeAmount indicates a negative monetary amount was provided.
	ErrNegativeAmount = errors.New("slot: negative amount")
	// ErrAmountOverflow indicates an arithmetic overflow in amount calculation.
	ErrAmountOverflow = errors.New("slot: amount overflow")
)

// Amount represents monetary value in minor units (e.g. cents) as an exact integer.
type Amount int64

// Add returns a + other with overflow checking.
func (a Amount) Add(other Amount) (Amount, error) {
	if other > 0 && a > math.MaxInt64-other {
		return 0, ErrAmountOverflow
	}
	if other < 0 && a < math.MinInt64-other {
		return 0, ErrAmountOverflow
	}
	return a + other, nil
}

// Multiplier represents an exact fixed-point multiplier scaled by MultiplierScale (1,000,000).
type Multiplier int64

// ParseMultiplier parses a decimal string (up to six fractional digits) into a Multiplier.
func ParseMultiplier(s string) (Multiplier, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, ErrInvalidMultiplierFormat
	}
	if strings.HasPrefix(s, "-") {
		return 0, ErrNegativeMultiplier
	}
	if strings.HasPrefix(s, "+") {
		s = s[1:]
		if s == "" {
			return 0, ErrInvalidMultiplierFormat
		}
	}

	parts := strings.Split(s, ".")
	if len(parts) > 2 {
		return 0, ErrInvalidMultiplierFormat
	}

	var wholeMicro int64
	if parts[0] != "" {
		for _, c := range parts[0] {
			if c < '0' || c > '9' {
				return 0, fmt.Errorf("%w: invalid character %q", ErrInvalidMultiplierFormat, c)
			}
		}
		whole, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("%w: %w", ErrMultiplierOverflow, err)
		}
		if whole > math.MaxInt64/MultiplierScale {
			return 0, ErrMultiplierOverflow
		}
		wholeMicro = whole * MultiplierScale
	}

	var fracMicro int64
	if len(parts) == 2 {
		fracStr := parts[1]
		if len(fracStr) > 6 {
			return 0, fmt.Errorf("%w: maximum 6 decimal places allowed", ErrInvalidMultiplierFormat)
		}
		for _, c := range fracStr {
			if c < '0' || c > '9' {
				return 0, fmt.Errorf("%w: invalid character %q", ErrInvalidMultiplierFormat, c)
			}
		}
		if len(fracStr) > 0 {
			// Pad with trailing zeros to 6 digits.
			padded := fracStr + strings.Repeat("0", 6-len(fracStr))
			val, err := strconv.ParseInt(padded, 10, 64)
			if err != nil {
				return 0, fmt.Errorf("%w: %w", ErrMultiplierOverflow, err)
			}
			fracMicro = val
		}
	}

	if parts[0] == "" && len(parts) == 2 && parts[1] == "" {
		return 0, ErrInvalidMultiplierFormat
	}

	if wholeMicro > math.MaxInt64-fracMicro {
		return 0, ErrMultiplierOverflow
	}

	return Multiplier(wholeMicro + fracMicro), nil
}

// MustMultiplier parses s into a Multiplier and panics on error.
func MustMultiplier(s string) Multiplier {
	m, err := ParseMultiplier(s)
	if err != nil {
		panic(fmt.Sprintf("slot.MustMultiplier(%q): %v", s, err))
	}
	return m
}

// String formats the multiplier as a decimal string with trailing zeros trimmed.
func (m Multiplier) String() string {
	val := int64(m)
	if val < 0 {
		abs := -val
		whole := abs / MultiplierScale
		frac := abs % MultiplierScale
		if frac == 0 {
			return fmt.Sprintf("-%d", whole)
		}
		fracStr := strings.TrimRight(fmt.Sprintf("%06d", frac), "0")
		return fmt.Sprintf("-%d.%s", whole, fracStr)
	}

	whole := val / MultiplierScale
	frac := val % MultiplierScale
	if frac == 0 {
		return strconv.FormatInt(whole, 10)
	}
	fracStr := strings.TrimRight(fmt.Sprintf("%06d", frac), "0")
	return fmt.Sprintf("%d.%s", whole, fracStr)
}

// MarshalJSON encodes the Multiplier as a quoted decimal string.
func (m Multiplier) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(m.String())), nil
}

// UnmarshalJSON decodes a Multiplier from a JSON string or number.
func (m *Multiplier) UnmarshalJSON(data []byte) error {
	s := strings.TrimSpace(string(data))
	if s == "null" {
		*m = 0
		return nil
	}
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		unquoted, err := strconv.Unquote(s)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidMultiplierFormat, err)
		}
		parsed, err := ParseMultiplier(unquoted)
		if err != nil {
			return err
		}
		*m = parsed
		return nil
	}

	parsed, err := ParseMultiplier(s)
	if err != nil {
		return err
	}
	*m = parsed
	return nil
}

// Add returns m + other with overflow check.
func (m Multiplier) Add(other Multiplier) (Multiplier, error) {
	if other > 0 && int64(m) > math.MaxInt64-int64(other) {
		return 0, ErrMultiplierOverflow
	}
	if other < 0 && int64(m) < math.MinInt64-int64(other) {
		return 0, ErrMultiplierOverflow
	}
	res := m + other
	if res < 0 {
		return 0, ErrNegativeMultiplier
	}
	return res, nil
}

// Sub returns m - other with underflow check (multipliers are never negative).
func (m Multiplier) Sub(other Multiplier) (Multiplier, error) {
	if other < 0 {
		return 0, ErrNegativeMultiplier
	}
	if m < other {
		return 0, ErrNegativeMultiplier
	}
	return m - other, nil
}

// Mul multiplies two multipliers and scales down by MultiplierScale: (m * other) / MultiplierScale.
func (m Multiplier) Mul(other Multiplier) (Multiplier, error) {
	if m < 0 || other < 0 {
		return 0, ErrNegativeMultiplier
	}
	if m == 0 || other == 0 {
		return 0, nil
	}
	a := int64(m)
	b := int64(other)
	if a > math.MaxInt64/b {
		return 0, ErrMultiplierOverflow
	}
	return Multiplier((a * b) / MultiplierScale), nil
}

// MulInt multiplies m by an integer count with overflow check.
func (m Multiplier) MulInt(n int64) (Multiplier, error) {
	if n < 0 {
		return 0, ErrNegativeMultiplier
	}
	if n == 0 || m == 0 {
		return 0, nil
	}
	if int64(m) > math.MaxInt64/n {
		return 0, ErrMultiplierOverflow
	}
	return Multiplier(int64(m) * n), nil
}

// Settle calculates the floor payout in minor units: floor(bet * multiplier / MultiplierScale).
func Settle(bet Amount, mult Multiplier) (Amount, error) {
	if bet < 0 {
		return 0, ErrNegativeAmount
	}
	if mult < 0 {
		return 0, ErrNegativeMultiplier
	}
	if bet == 0 || mult == 0 {
		return 0, nil
	}
	b := int64(bet)
	m := int64(mult)
	if b > math.MaxInt64/m {
		return 0, ErrAmountOverflow
	}
	product := b * m
	payout := product / MultiplierScale
	return Amount(payout), nil
}
