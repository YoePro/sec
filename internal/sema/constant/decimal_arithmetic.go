package constant

import (
	"math"
	"math/big"
)

// DecimalFailure names the ArithmeticError variant of a failed decimal
// operation, or "" for success (MD-067 § 7(3)).
type DecimalFailure string

const (
	DecimalDivisionByZero     DecimalFailure = "DivisionByZero"
	DecimalScaleOverflow      DecimalFailure = "ScaleOverflow"
	DecimalPrecisionExhausted DecimalFailure = "PrecisionExhausted"
	DecimalOverflow           DecimalFailure = "Overflow"
)

// maxDecimalEvaluationExponent bounds the powers of ten exact evaluation
// materializes; operands or results beyond it are not folded, so runtime
// semantics decide them instead of an unbounded host computation.
const maxDecimalEvaluationExponent = 400

// EvaluateDecimal evaluates `a op b` for decimal operands exactly, as
// coefficient × 10^(-scale) values with a signed coefficient of the given
// width (64 for decimal, 128 for decimal128). Exact operations select the
// legal scale closest to the preferred scale (max of the operand scales, or
// their sum for `*`); `%` is the truncating remainder; an inexact quotient
// rounds once with nearest-even at the finest scale that keeps the whole
// number part. A failure reports the most specific ArithmeticError variant.
// ok is false when the operation is outside the bounded evaluation domain
// (the result is then left to runtime) or op is not arithmetic.
//
// Rules:
//   - rules/foundations/operators.md — "Decimal arithmetic", "Decimal result scale", "Decimal division"
//   - rules/corrections/applied/md066-md067-md068-decimal-correction-20261028.md — §§ 3–5, 7
func EvaluateDecimal(op string, aCoefficient *big.Int, aScale int64, bCoefficient *big.Int, bScale int64, bits int) (coefficient *big.Int, scale int64, failure DecimalFailure, ok bool) {
	if aCoefficient == nil || bCoefficient == nil || outOfEvaluationRange(aScale) || outOfEvaluationRange(bScale) {
		return nil, 0, "", false
	}
	a := decimalRat(aCoefficient, int(aScale))
	b := decimalRat(bCoefficient, int(bScale))
	preferred := max(aScale, bScale)
	var value *big.Rat
	switch op {
	case "+":
		value = new(big.Rat).Add(a, b)
	case "-":
		value = new(big.Rat).Sub(a, b)
	case "*":
		value = new(big.Rat).Mul(a, b)
		preferred = aScale + bScale
	case "/", "%":
		if b.Sign() == 0 {
			return nil, 0, DecimalDivisionByZero, true
		}
		quotient := new(big.Rat).Quo(a, b)
		if op == "%" {
			truncated := new(big.Int).Quo(quotient.Num(), quotient.Denom())
			value = new(big.Rat).Sub(a, new(big.Rat).Mul(new(big.Rat).SetInt(truncated), b))
		} else {
			value = quotient
		}
	default:
		return nil, 0, "", false
	}
	if op == "/" {
		if _, exact := minimalExactDecimalScale(value); !exact {
			return roundDecimalQuotient(value, bits)
		}
		// An exact quotient needing more digits than the coefficient holds
		// is approximated like a nonterminating one (MD-066 § 5(2)).
		if fitted, scale, failure, ok := fitExactDecimal(value, preferred, bits); !ok || failure == "" {
			return fitted, scale, failure, ok
		}
		return roundDecimalQuotient(value, bits)
	}
	return fitExactDecimal(value, preferred, bits)
}

// NegateDecimal evaluates unary minus exactly at the operand's scale.
func NegateDecimal(coefficient *big.Int, scale int64, bits int) (*big.Int, int64, DecimalFailure, bool) {
	if coefficient == nil || outOfEvaluationRange(scale) {
		return nil, 0, "", false
	}
	return fitExactDecimal(new(big.Rat).Neg(decimalRat(coefficient, int(scale))), scale, bits)
}

func outOfEvaluationRange(scale int64) bool {
	return scale > maxDecimalEvaluationExponent || scale < -maxDecimalEvaluationExponent
}

// fitExactDecimal places an exactly representable finite decimal value at
// the legal scale closest to preferred, or classifies why none exists.
func fitExactDecimal(value *big.Rat, preferred int64, bits int) (*big.Int, int64, DecimalFailure, bool) {
	if value.Sign() == 0 {
		fitted, scale, _ := FitDecimal(new(big.Int), preferred, bits)
		return fitted, int64(scale), "", true
	}
	minimal, exact := minimalExactDecimalScale(value)
	if !exact || outOfEvaluationRange(int64(minimal)) {
		return nil, 0, "", false
	}
	at := max(preferred, int64(minimal))
	if at-int64(minimal) > maxDecimalEvaluationExponent {
		at = int64(minimal) + maxDecimalEvaluationExponent
	}
	coefficient := ratAtScale(value, at)
	if fitted, scale, ok := FitDecimal(coefficient, at, bits); ok {
		return fitted, int64(scale), "", true
	}
	return nil, 0, classifyUnrepresentable(value, minimal, bits), true
}

// classifyUnrepresentable applies MD-067 § 7(4): the minimal exact
// coefficient decides PrecisionExhausted, otherwise the scale domain
// decides ScaleOverflow.
func classifyUnrepresentable(value *big.Rat, minimal int, bits int) DecimalFailure {
	if !coefficientFits(ratAtScale(value, int64(minimal)), bits) {
		return DecimalPrecisionExhausted
	}
	if int64(minimal) > math.MaxInt32 || int64(minimal) < math.MinInt32 {
		return DecimalScaleOverflow
	}
	return DecimalOverflow
}

// roundDecimalQuotient rounds an inexact quotient once with nearest-even at
// the finest non-negative scale whose rounded coefficient fits (MD-066 § 5).
// A quotient whose whole-number part cannot be kept fails.
func roundDecimalQuotient(value *big.Rat, bits int) (*big.Int, int64, DecimalFailure, bool) {
	whole := new(big.Int).Quo(new(big.Int).Abs(value.Num()), value.Denom())
	if !coefficientFits(whole, bits) && !coefficientFits(new(big.Int).Neg(whole), bits) {
		return nil, 0, DecimalPrecisionExhausted, true
	}
	fitsAt := func(scale int64) (*big.Int, bool) {
		rounded := roundNearestEven(new(big.Rat).Mul(value, new(big.Rat).SetInt(pow10(scale))))
		return rounded, coefficientFits(rounded, bits)
	}
	// Grow the scale while the rounded coefficient still fits, then step
	// back over a rounding carry; scale 0 keeps the whole-number part.
	scale := int64(0)
	if _, ok := fitsAt(0); !ok {
		return nil, 0, DecimalPrecisionExhausted, true
	}
	for scale < maxDecimalEvaluationExponent {
		if _, ok := fitsAt(scale + 1); !ok {
			break
		}
		scale++
	}
	if scale == maxDecimalEvaluationExponent {
		return nil, 0, "", false
	}
	rounded, _ := fitsAt(scale)
	return rounded, scale, "", true
}

func roundNearestEven(value *big.Rat) *big.Int {
	quotient, remainder := new(big.Int).QuoRem(value.Num(), value.Denom(), new(big.Int))
	twice := new(big.Int).Mul(new(big.Int).Abs(remainder), big.NewInt(2))
	switch twice.Cmp(value.Denom()) {
	case 1:
		return quotient.Add(quotient, big.NewInt(int64(value.Sign())))
	case 0:
		if quotient.Bit(0) == 1 {
			return quotient.Add(quotient, big.NewInt(int64(value.Sign())))
		}
	}
	return quotient
}

func ratAtScale(value *big.Rat, scale int64) *big.Int {
	scaled := new(big.Rat).Set(value)
	if scale >= 0 {
		scaled.Mul(scaled, new(big.Rat).SetInt(pow10(scale)))
	} else {
		scaled.Quo(scaled, new(big.Rat).SetInt(pow10(-scale)))
	}
	return new(big.Int).Quo(scaled.Num(), scaled.Denom())
}

func pow10(exponent int64) *big.Int {
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(exponent), nil)
}

func coefficientFits(coefficient *big.Int, bits int) bool {
	limit := new(big.Int).Lsh(big.NewInt(1), uint(bits-1))
	return coefficient.Cmp(new(big.Int).Neg(limit)) >= 0 && coefficient.Cmp(limit) < 0
}
