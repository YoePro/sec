package sema

import (
	"math/big"
	"strings"

	"sec/internal/ast"
	"sec/internal/sema/constant"
)

// ConversionFailureNonFinite is a NaN or infinity converted to a target whose
// intrinsic domain holds only finite values (integers and decimals).
const ConversionFailureNonFinite ConversionFailureKind = "NonFinite"

// numericConversionFailures lists, in runtime priority order (NonFinite,
// OutOfRange, PrecisionLoss), the ConversionError variants an exact typed
// conversion involving a binary floating or decimal type can produce. The
// second result is false when neither side is floating or decimal, so the
// integer-family rules keep the pair. minimum/maximum bound an integer
// source; they are nil for every other source.
//
// The classification is by mathematical cause:
//
//	float -> wider or equal float    exact
//	float -> narrower float          OutOfRange, PrecisionLoss (NaN and infinity map)
//	integer -> float                 exact while |value| <= 2^precision; otherwise
//	                                 PrecisionLoss, and OutOfRange once the source
//	                                 can exceed the finite float range
//	float -> integer                 NonFinite, OutOfRange, PrecisionLoss
//	float -> decimal                 NonFinite, PrecisionLoss
//	decimal -> float                 OutOfRange, PrecisionLoss
//	decimal -> integer               OutOfRange, PrecisionLoss
//	integer -> decimal               exact while the value fits the signed
//	                                 coefficient; otherwise PrecisionLoss
//	decimal128 -> decimal            PrecisionLoss
//	decimal -> decimal128            exact
//
// A decimal scale is a signed int32 for both widths, so no pair here needs an
// unrepresentable destination scale; ScaleOverflow has no source among them.
//
// Rules:
//   - rules/types/types.md — § 25(3)–(5), § 25(9), § 25(11) "Explicit conversions"
//   - rules/mlir/normative-sync/sec_float_numeric_sync_v1.md — § 12 "Error priority", § 13 "Float narrowing", § 14 "Integer to float", § 15 "Float to integer"
//   - rules/mlir/normative-sync/sec_p20_p21_conversion_compatibility_package22.md — § 2 "Required mapping"
//   - rules/mlir/semantic-ir/sec_semantic_ir_decimal_package21.md — § 7 "Conversions and modes"
//   - rules/types/types.md — "Decimal scale"; rules/memory/layout.md — decimal {i64,i32}, decimal128 {i128,i32}
func numericConversionFailures(target, source Type, minimum, maximum *big.Int) ([]ConversionFailureKind, bool) {
	targetFloat, sourceFloat := target.Kind == FloatType, source.Kind == FloatType
	targetDecimal, sourceDecimal := target.Kind == DecimalType, source.Kind == DecimalType
	if !targetFloat && !sourceFloat && !targetDecimal && !sourceDecimal {
		return nil, false
	}
	switch {
	case targetFloat && sourceFloat:
		if source.FloatBits == 0 || target.FloatBits == 0 {
			return []ConversionFailureKind{ConversionFailurePrecisionLoss}, true
		}
		if source.FloatBits > target.FloatBits {
			return []ConversionFailureKind{ConversionFailureOutOfRange, ConversionFailurePrecisionLoss}, true
		}
		return nil, true
	case targetFloat && isIntegerType(source):
		precision, exponent, ok := floatFormat(target.FloatBits)
		if !ok || minimum == nil || maximum == nil {
			return []ConversionFailureKind{ConversionFailurePrecisionLoss}, true
		}
		magnitude := new(big.Int).Abs(minimum)
		if other := new(big.Int).Abs(maximum); other.Cmp(magnitude) > 0 {
			magnitude = other
		}
		if magnitude.Cmp(new(big.Int).Lsh(big.NewInt(1), precision)) <= 0 {
			return nil, true
		}
		if magnitude.Cmp(new(big.Int).Lsh(big.NewInt(1), exponent)) >= 0 {
			return []ConversionFailureKind{ConversionFailureOutOfRange, ConversionFailurePrecisionLoss}, true
		}
		return []ConversionFailureKind{ConversionFailurePrecisionLoss}, true
	case sourceFloat && isIntegerType(target):
		return []ConversionFailureKind{ConversionFailureNonFinite, ConversionFailureOutOfRange, ConversionFailurePrecisionLoss}, true
	case sourceFloat && targetDecimal:
		return []ConversionFailureKind{ConversionFailureNonFinite, ConversionFailurePrecisionLoss}, true
	case sourceDecimal && (targetFloat || isIntegerType(target)):
		return []ConversionFailureKind{ConversionFailureOutOfRange, ConversionFailurePrecisionLoss}, true
	case targetDecimal && isIntegerType(source):
		if minimum == nil || maximum == nil {
			return []ConversionFailureKind{ConversionFailurePrecisionLoss}, true
		}
		bound := new(big.Int).Lsh(big.NewInt(1), decimalCoefficientBits(target)-1)
		if minimum.Cmp(new(big.Int).Neg(bound)) < 0 || maximum.Cmp(bound) >= 0 {
			return []ConversionFailureKind{ConversionFailurePrecisionLoss}, true
		}
		return nil, true
	case targetDecimal && sourceDecimal:
		if decimalCoefficientBits(source) > decimalCoefficientBits(target) {
			return []ConversionFailureKind{ConversionFailurePrecisionLoss}, true
		}
		return nil, true
	}
	// bool, char, rune and other families have no floating or decimal checked
	// relation here; their own rules decide.
	return nil, true
}

// floatFormat returns the significand precision and the binary exponent of
// the first power of two beyond the finite range of a binary format.
func floatFormat(bits int) (precision, exponent uint, ok bool) {
	switch bits {
	case 32:
		return 24, 128, true
	case 64:
		return 53, 1024, true
	}
	return 0, 0, false
}

// platformFloatWidths lists the widths a type must be judged at. Plain float
// is target-sized (MD-014): with a selected CompilationPlan its resolved
// width is already in the type; without one the conversion is proved only
// when it holds at both supported widths, as for int and uint (MD-104).
//
// Rules:
//   - rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md — §§ 6.3–6.9
//   - rules/types/types.md — § 25.1(2) "Exact integer conversion proof"
func (a *Analyzer) platformFloatWidths(typ Type) []Type {
	if a.scalarPlanWidthBits != 0 || typ.Kind != FloatType || (typ.Name != "float" && typ.Underlying != "float") {
		return []Type{typ}
	}
	narrow, wide := typ, typ
	narrow.FloatBits, wide.FloatBits = 32, 64
	return []Type{narrow, wide}
}

// numericConversionMayFail applies numericConversionFailures to one explicit
// conversion call: a compile-time-known floating or decimal argument is
// proved by its exact value, an integer argument by its proven interval, and
// every other argument by the complete source domain. The union over the
// platform widths of plain float is returned in priority order.
//
// Rules:
//   - rules/types/types.md — § 25(5), § 25(7) "Explicit conversions"
//   - rules/corrections/applied/md012-md022-conversion-errors-ffi-literals-correction-20261008.md — § 4.8
func (a *Analyzer) numericConversionMayFail(target, source Type, argument ast.Expression) ([]ConversionFailureKind, bool) {
	if _, handled := numericConversionFailures(target, source, nil, nil); !handled {
		return nil, false
	}
	var minimum, maximum *big.Int
	if isIntegerType(source) {
		if low, high, known := a.scalarValueInterval(argument, source); known {
			minimum, maximum = low, high
		}
	}
	found := map[ConversionFailureKind]bool{}
	for _, targetAt := range a.platformFloatWidths(target) {
		for _, sourceAt := range a.platformFloatWidths(source) {
			if a.constantNumericConversionExact(targetAt, sourceAt, argument) {
				continue
			}
			failures, _ := numericConversionFailures(targetAt, sourceAt, minimum, maximum)
			for _, failure := range failures {
				found[failure] = true
			}
		}
	}
	var ordered []ConversionFailureKind
	for _, failure := range []ConversionFailureKind{ConversionFailureNonFinite, ConversionFailureOutOfRange, ConversionFailurePrecisionLoss} {
		if found[failure] {
			ordered = append(ordered, failure)
		}
	}
	return ordered, true
}

// constantNumericConversionExact reports that the argument is a compile-time
// known floating or decimal value the target represents exactly, so the
// conversion needs no runtime check. A known value the target cannot
// represent is not proved here and keeps its try requirement.
//
// Rules:
//   - rules/types/types.md — § 25(7) "Explicit conversions"
func (a *Analyzer) constantNumericConversionExact(target, source Type, argument ast.Expression) bool {
	var value *big.Rat
	switch source.Kind {
	case FloatType:
		known, ok := a.floatingConstantValue(argument, source)
		if !ok {
			return false
		}
		value = known
	case DecimalType:
		known, ok := a.decimalConstantValue(argument)
		if !ok {
			return false
		}
		value = known
	default:
		return false
	}
	switch {
	case isIntegerType(target):
		if !value.IsInt() || target.MinInteger == nil || target.MaxInteger == nil {
			return false
		}
		return value.Num().Cmp(target.MinInteger) >= 0 && value.Num().Cmp(target.MaxInteger) <= 0
	case target.Kind == FloatType:
		return ratExactInFloat(value, target.FloatBits)
	case target.Kind == DecimalType:
		_, _, projected, ok := constant.NearestDecimal(value, int(decimalCoefficientBits(target)))
		return ok && !projected
	}
	return false
}

// ratExactInFloat reports whether a rational is a finite value of the binary
// format, subnormals included.
func ratExactInFloat(value *big.Rat, bits int) bool {
	switch bits {
	case 32:
		nearest, exact := value.Float32()
		return exact && !isInfinite(float64(nearest))
	case 64:
		nearest, exact := value.Float64()
		return exact && !isInfinite(nearest)
	}
	return false
}

func isInfinite(value float64) bool {
	return value > maxFiniteFloat64 || value < -maxFiniteFloat64
}

const maxFiniteFloat64 = 0x1p1023 * (1 + (1 - 0x1p-52))

// conversionFailureList spells the variants of a checked conversion for a
// diagnostic: "ConversionError.A", "ConversionError.A or ConversionError.B",
// or "ConversionError.A, ConversionError.B or ConversionError.C".
func conversionFailureList(failures []ConversionFailureKind) string {
	names := make([]string, len(failures))
	for index, failure := range failures {
		names[index] = "ConversionError." + string(failure)
	}
	if len(names) <= 1 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " or " + names[len(names)-1]
}
