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
