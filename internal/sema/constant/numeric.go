package constant

import (
	"math"
	"math/big"
)

// Numeric applies resolved decimal or binary-floating arithmetic. Decimal
// operations retain exact rationals; float operations round at their selected
// width, including each intermediate operation, rather than using host width.
// Rules: rules/foundations/operators.md — Floating arithmetic, Decimal arithmetic
// and Decimal remainder; rules/compiler/compile_time_evaluation.md — §2(3).
func Numeric(operator string, left, right *big.Rat, bits int) (*big.Rat, bool) {
	if bits != 0 {
		x, _ := left.Float64()
		y, _ := right.Float64()
		if bits == 32 {
			a, _ := left.Float32()
			b, _ := right.Float32()
			x, y = float64(a), float64(b)
		} else if bits != 64 {
			return nil, false
		}
		var z float64
		if bits == 32 {
			a, b := float32(x), float32(y)
			switch operator {
			case "+":
				z = float64(a + b)
			case "-":
				z = float64(a - b)
			case "*":
				z = float64(a * b)
			case "/":
				z = float64(a / b)
			case "%":
				z = float64(float32(math.Mod(float64(a), float64(b))))
			default:
				return nil, false
			}
		} else {
			switch operator {
			case "+":
				z = x + y
			case "-":
				z = x - y
			case "*":
				z = x * y
			case "/":
				z = x / y
			case "%":
				z = math.Mod(x, y)
			default:
				return nil, false
			}
		}
		// Nonfinite constants require a represented nonfinite value; never fabricate
		// a rational value for infinity or NaN.
		if math.IsNaN(z) || math.IsInf(z, 0) {
			return nil, false
		}
		return new(big.Rat).SetFloat64(z), true
	}
	z := new(big.Rat)
	switch operator {
	case "+":
		return z.Add(left, right), true
	case "-":
		return z.Sub(left, right), true
	case "*":
		return z.Mul(left, right), true
	case "/":
		if right.Sign() != 0 {
			return z.Quo(left, right), true
		}
	case "%":
		if right.Sign() != 0 {
			q := new(big.Rat).Quo(left, right)
			integer := new(big.Int).Quo(q.Num(), q.Denom())
			return z.Sub(left, new(big.Rat).Mul(new(big.Rat).SetInt(integer), right)), true
		}
	}
	return nil, false
}

// DecimalText converts a finite base-ten rational to its shortest exact
// decimal spelling while checking the canonical signed coefficient width.
// Rules: rules/memory/layout.md — scalar representations;
// rules/mlir/dialect-versions/sec_mlir_dialect_v3.md — §22 decimal constants;
// rules/mlir/packages/sec-mlir-dialect_package6.md — §36 Decimal boundary.
func DecimalText(value *big.Rat, bits int) (string, bool) {
	if bits != 64 && bits != 128 {
		return "", false
	}
	denominator := new(big.Int).Set(value.Denom())
	two, five := big.NewInt(2), big.NewInt(5)
	twos, fives := 0, 0
	for denominator.Bit(0) == 0 {
		denominator.Quo(denominator, two)
		twos++
	}
	for new(big.Int).Mod(denominator, five).Sign() == 0 {
		denominator.Quo(denominator, five)
		fives++
	}
	if denominator.Cmp(big.NewInt(1)) != 0 {
		return "", false
	}
	scale := max(twos, fives)
	coefficient := new(big.Int).Set(value.Num())
	if twos < scale {
		coefficient.Mul(coefficient, new(big.Int).Exp(two, big.NewInt(int64(scale-twos)), nil))
	}
	if fives < scale {
		coefficient.Mul(coefficient, new(big.Int).Exp(five, big.NewInt(int64(scale-fives)), nil))
	}
	bound := new(big.Int).Lsh(big.NewInt(1), uint(bits-1))
	if coefficient.Cmp(new(big.Int).Neg(bound)) < 0 || coefficient.Cmp(new(big.Int).Sub(bound, big.NewInt(1))) > 0 {
		return "", false
	}
	return value.FloatString(scale), true
}

// NearestDecimal projects a rational onto the nearest value of the decimal
// carrier: a signed bits-wide coefficient with a signed int32 scale, the
// value being coefficient × 10^(-scale). An exactly representable value is
// returned exactly at the scale closest to zero that fits, which may be
// negative for a large integer. Otherwise the value is rounded once at the
// finest scale whose coefficient fits, breaking an exact tie toward the even
// coefficient; projected reports that rounding. The coefficient domain is the
// real asymmetric signed one.
// Rules: rules/types/units.md — "Unit metadata value types" (7) (MD-062 § 9(7));
// rules/types/types.md — "Decimal scale"; rules/foundations/operators.md —
// "Decimal result scale" (MD-066).
func NearestDecimal(value *big.Rat, bits int) (coefficient *big.Int, scale int, projected bool, ok bool) {
	if value == nil || bits != 64 && bits != 128 {
		return nil, 0, false, false
	}
	high := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(bits-1)), big.NewInt(1))
	low := new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), uint(bits-1)))
	fits := func(c *big.Int) bool { return c.Cmp(low) >= 0 && c.Cmp(high) <= 0 }
	// at rounds value × 10^s half-even and reports whether it was exact.
	at := func(s int) (*big.Int, bool) {
		numerator, denominator := new(big.Int).Set(value.Num()), new(big.Int).Set(value.Denom())
		power := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(abs(s))), nil)
		if s >= 0 {
			numerator.Mul(numerator, power)
		} else {
			denominator.Mul(denominator, power)
		}
		q, r := new(big.Int).QuoRem(numerator, denominator, new(big.Int))
		if r.Sign() == 0 {
			return q, true
		}
		twice := new(big.Int).Abs(new(big.Int).Lsh(r, 1))
		if cmp := twice.Cmp(denominator); cmp > 0 || cmp == 0 && q.Bit(0) == 1 {
			q.Add(q, big.NewInt(int64(value.Sign())))
		}
		return q, false
	}
	inScale := func(s int) bool { return s >= math.MinInt32 && s <= math.MaxInt32 }
	if minimum, exact := minimalExactDecimalScale(value); exact {
		for s := max(minimum, 0); s >= minimum && inScale(s); s-- {
			if c, _ := at(s); fits(c) {
				return c, s, false, true
			}
			if s > 0 {
				// A positive minimal scale needs every fractional digit.
				break
			}
		}
	}
	// Round once at the finest scale whose coefficient fits.
	finest, found := 0, false
	if c, _ := at(0); fits(c) {
		for s := 0; inScale(s); s++ {
			if c, _ := at(s); !fits(c) {
				break
			}
			finest, found = s, true
		}
	} else {
		for s := -1; inScale(s); s-- {
			if c, _ := at(s); fits(c) {
				finest, found = s, true
				break
			}
		}
	}
	if !found {
		return nil, 0, false, false
	}
	best, exact := at(finest)
	// The next finer lattice still holds its extreme coefficient, which can
	// lie closer to value than the coarser nearest point.
	if next := finest + 1; inScale(next) {
		edge := high
		if value.Sign() < 0 {
			edge = low
		}
		distance := func(c *big.Int, s int) *big.Rat {
			return new(big.Rat).Abs(new(big.Rat).Sub(value, decimalRat(c, s)))
		}
		if distance(edge, next).Cmp(distance(best, finest)) < 0 {
			return new(big.Int).Set(edge), next, true, true
		}
	}
	return best, finest, !exact, true
}

// NearestDecimalText is NearestDecimal in canonical text: a plain decimal
// without redundant trailing fractional zeros for a non-negative scale, and
// `<coefficient>e<exponent>` for a negative scale.
// Rules: rules/types/units.md — "Unit metadata value types" (7) (MD-062 § 9(7)).
func NearestDecimalText(value *big.Rat, bits int) (text string, projected bool, ok bool) {
	coefficient, scale, projected, ok := NearestDecimal(value, bits)
	if !ok {
		return "", false, false
	}
	if scale < 0 {
		return coefficient.String() + "e" + big.NewInt(int64(-scale)).String(), projected, true
	}
	ten := big.NewInt(10)
	for scale > 0 {
		q, r := new(big.Int).QuoRem(coefficient, ten, new(big.Int))
		if r.Sign() != 0 {
			break
		}
		coefficient, scale = q, scale-1
	}
	return decimalRat(coefficient, scale).FloatString(scale), projected, true
}

// minimalExactDecimalScale is the smallest scale at which value has an
// integer coefficient, negative for an integer with trailing zeros, or
// false when value is not a finite decimal.
func minimalExactDecimalScale(value *big.Rat) (int, bool) {
	if value.Sign() == 0 {
		return 0, true
	}
	if value.IsInt() {
		scale := 0
		ten := big.NewInt(10)
		n := new(big.Int).Abs(value.Num())
		for {
			q, r := new(big.Int).QuoRem(n, ten, new(big.Int))
			if r.Sign() != 0 {
				return scale, true
			}
			n, scale = q, scale-1
		}
	}
	denominator := new(big.Int).Set(value.Denom())
	twos, fives := 0, 0
	for denominator.Bit(0) == 0 {
		denominator.Rsh(denominator, 1)
		twos++
	}
	five := big.NewInt(5)
	for new(big.Int).Mod(denominator, five).Sign() == 0 {
		denominator.Quo(denominator, five)
		fives++
	}
	if denominator.Cmp(big.NewInt(1)) != 0 {
		return 0, false
	}
	return max(twos, fives), true
}

// decimalRat is the exact value coefficient × 10^(-scale).
func decimalRat(coefficient *big.Int, scale int) *big.Rat {
	power := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(abs(scale))), nil)
	if scale >= 0 {
		return new(big.Rat).SetFrac(coefficient, power)
	}
	return new(big.Rat).SetInt(new(big.Int).Mul(coefficient, power))
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
