package constant

import (
	"math"
	"math/big"
)

// FloatAffineIntervalExact proves that every representable source value in the
// finite interval maps exactly through factor*x+offset at the selected width.
// Each binary binade is an integer lattice. Its image must lie on the coarsest
// destination lattice it spans; a single source value needs only an exact image.
// Unknown proofs are rejected, including subnormal loss and finite overflow.
// Rules: rules/types/units.md — Exact fixed conversions; No hidden precision loss;
// rules/types/types.md — Binary floating-point types.
func FloatAffineIntervalExact(lower, upper, factor, offset *big.Rat, bits int) bool {
	if lower == nil || upper == nil || factor == nil || offset == nil || lower.Cmp(upper) > 0 || factor.Sign() <= 0 {
		return false
	}
	precision, minExponent, maxExponent := 53, -1022, 1023
	if bits == 32 {
		precision, minExponent, maxExponent = 24, -126, 127
	} else if bits != 64 {
		return false
	}
	image := func(x *big.Rat) *big.Rat { return new(big.Rat).Add(new(big.Rat).Mul(x, factor), offset) }
	exact := func(x *big.Rat) bool {
		var f float64
		if bits == 32 {
			v, _ := x.Float32()
			f = float64(v)
		} else {
			f, _ = x.Float64()
		}
		r, ok := FloatExact(f, bits)
		return ok && r.Cmp(x) == 0
	}
	if !exact(image(lower)) || !exact(image(upper)) {
		return false
	}
	if lower.Cmp(upper) == 0 {
		return true
	}
	check := func(step *big.Rat, first, last *big.Int) bool {
		lo := new(big.Rat).Quo(lower, step)
		hi := new(big.Rat).Quo(upper, step)
		ceil := ratFloor(lo)
		if new(big.Rat).SetInt(ceil).Cmp(lo) < 0 {
			ceil.Add(ceil, big.NewInt(1))
		}
		floor := ratFloor(hi)
		if ceil.Cmp(first) > 0 {
			first = ceil
		}
		if floor.Cmp(last) < 0 {
			last = floor
		}
		if first.Cmp(last) > 0 {
			return true
		}
		a := image(new(big.Rat).Mul(new(big.Rat).SetInt(first), step))
		b := image(new(big.Rat).Mul(new(big.Rat).SetInt(last), step))
		if !exact(a) || !exact(b) {
			return false
		}
		if first.Cmp(last) == 0 {
			return true
		}
		magnitude := new(big.Rat).Abs(a)
		if other := new(big.Rat).Abs(b); other.Cmp(magnitude) > 0 {
			magnitude = other
		}
		f, _ := magnitude.Float64()
		_, exponent := math.Frexp(f)
		spacingExponent := exponent - precision
		if spacingExponent < minExponent-precision+1 {
			spacingExponent = minExponent - precision + 1
		}
		spacing := binaryPower(spacingExponent)
		return new(big.Rat).Quo(a, spacing).IsInt() && new(big.Rat).Quo(new(big.Rat).Mul(step, factor), spacing).IsInt()
	}
	normalFirst := new(big.Int).Lsh(big.NewInt(1), uint(precision-1))
	normalLast := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(precision)), big.NewInt(1))
	subnormalLast := new(big.Int).Sub(new(big.Int).Set(normalFirst), big.NewInt(1))
	if !check(binaryPower(minExponent-precision+1), new(big.Int).Neg(subnormalLast), subnormalLast) {
		return false
	}
	for exponent := minExponent; exponent <= maxExponent; exponent++ {
		step := binaryPower(exponent - precision + 1)
		if !check(step, normalFirst, normalLast) || !check(step, new(big.Int).Neg(normalLast), new(big.Int).Neg(normalFirst)) {
			return false
		}
	}
	return true
}

// binaryPower builds an exact binary lattice spacing without host rounding.
// Rules: rules/types/types.md — Binary floating-point types.
func binaryPower(exponent int) *big.Rat {
	if exponent >= 0 {
		return new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), uint(exponent)))
	}
	return new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), uint(-exponent)))
}

// ratFloor rounds a rational index toward negative infinity for lattice bounds.
// Rules: rules/types/units.md — No hidden precision loss.
func ratFloor(value *big.Rat) *big.Int {
	q, r := new(big.Int), new(big.Int)
	q.QuoRem(value.Num(), value.Denom(), r)
	if r.Sign() < 0 {
		q.Sub(q, big.NewInt(1))
	}
	return q
}
