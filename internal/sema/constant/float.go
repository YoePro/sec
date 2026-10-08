package constant

import (
	"math"
	"math/big"
)

// FloatExact represents a finite value at the selected binary width exactly.
// Rules: rules/types/types.md — Binary floating-point types;
// rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md — §6.
func FloatExact(value float64, bits int) (*big.Rat, bool) {
	if bits == 32 {
		value = float64(float32(value))
	} else if bits != 64 {
		return nil, false
	}
	if math.IsInf(value, 0) || math.IsNaN(value) {
		return nil, false
	}
	return new(big.Rat).SetFloat64(value), true
}

// FloatEndpoint selects the nearest finite representable value on the requested
// side of an exact range boundary; exclusive applies to an upper boundary.
// Rules: rules/types/default_values.md — Floating and decimal ranges;
// rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md — §§6.14-6.17.
func FloatEndpoint(bound *big.Rat, bits int, upper, exclusive bool) (float64, bool) {
	var v float64
	if bits == 32 {
		f, _ := bound.Float32()
		v = float64(f)
	} else if bits == 64 {
		v, _ = bound.Float64()
	} else {
		return 0, false
	}
	current, ok := FloatExact(v, bits)
	if !ok {
		return 0, false
	}
	direction := 0
	if upper && (current.Cmp(bound) > 0 || exclusive && current.Cmp(bound) == 0) {
		direction = -1
	}
	if !upper && current.Cmp(bound) < 0 {
		direction = 1
	}
	if direction != 0 {
		toward := math.Inf(direction)
		if bits == 32 {
			v = float64(math.Nextafter32(float32(v), float32(toward)))
		} else {
			v = math.Nextafter(v, toward)
		}
	}
	_, ok = FloatExact(v, bits)
	return v, ok
}
