package sema

import (
	"math/big"
	"sort"

	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// TryFailureConversion is an explicit scalar conversion whose target cannot
// represent every value the source may hold (ConversionError).
const TryFailureConversion TryFailureKind = "conversion"

// ConversionFailureKind names the intrinsic ConversionError variant a
// checked scalar conversion can produce.
type ConversionFailureKind string

const (
	ConversionFailureOutOfRange    ConversionFailureKind = "OutOfRange"
	ConversionFailureInvalidScalar ConversionFailureKind = "InvalidScalar"
	ConversionFailurePrecisionLoss ConversionFailureKind = "PrecisionLoss"
)

// checkedConversion records one explicit conversion that is legal but may
// fail at run time. It is accepted only under try, which converts the
// failure into ConversionError.
type checkedConversion struct {
	source  Type
	target  Type
	failure ConversionFailureKind
}

// recordCheckedConversion classifies an explicit conversion call once its
// target type is known. Integer-family conversions (including byte, char,
// and rune) compare the compiler-proven value interval of the argument with
// the target's intrinsic domain, so constants and values refined by
// dominating conditions need no runtime check; rune additionally excludes
// surrogate code points. Floating conversions involving a c:: scalar compare
// formats. Declared contracts of constrained named types and unit
// conversions keep their own paths.
//
// Rules:
//   - rules/types/types.md — "Explicit conversions": a conversion that may fail requires try
//   - rules/platform/ffi.md — §8 "C-to-Sec and Sec-to-C scalar conversions"
//   - rules/corrections/applied/md012-md022-conversion-errors-ffi-literals-correction-20261008.md — §2.5, §4.1–4.8
func (a *Analyzer) recordCheckedConversion(call *ast.CallExpression, target, source Type) {
	if call == nil || len(call.Arguments) != 1 || len(target.Contracts) > 0 ||
		hasUnitSemantics(target) || hasUnitSemantics(source) {
		return
	}
	argument := call.Arguments[0]
	switch argument.(type) {
	case *ast.IntegerLiteral, *ast.FloatLiteral, *ast.BooleanLiteral, *ast.CharLiteral, *ast.StringLiteral:
		return
	}
	if failure, domainMayFail := conversionDomainMayFail(target, source); domainMayFail {
		// The relation can fail for some source values, so try may protect
		// it even where this target's proof shows the value fits; source stays
		// valid across targets with different int and uint widths.
		a.fallibleConversionRelations[call] = failure
	} else {
		delete(a.fallibleConversionRelations, call)
	}
	// Only conversions involving a c:: scalar must be protected today
	// (ffi.md §8). For Sec-only scalars the same rule in types.md cannot be
	// enforced yet: platform syscall wrappers pass signed values in unsigned
	// registers, which needs the bit-reinterpretation operation that
	// missing-decisions.yaml MD-056 tracks. Their try is already accepted.
	if !isForeignCScalar(target) && !isForeignCScalar(source) {
		delete(a.checkedConversions, call)
		return
	}
	failure, mayFail := a.conversionMayFail(target, source, argument)
	if !mayFail {
		// A body can be analyzed more than once (summary passes see fewer
		// path facts); the latest analysis of the call decides.
		delete(a.checkedConversions, call)
		return
	}
	a.checkedConversions[call] = checkedConversion{source: source, target: target, failure: failure}
}

func (a *Analyzer) conversionMayFail(target, source Type, argument ast.Expression) (ConversionFailureKind, bool) {
	if sameConcreteType(target, source) {
		return "", false
	}
	targetMin, targetMax, targetIsInteger := scalarIntegerDomain(target)
	if _, _, sourceIsInteger := scalarIntegerDomain(source); targetIsInteger && sourceIsInteger {
		valueMin, valueMax, known := a.scalarValueInterval(argument, source)
		if !known {
			return ConversionFailureOutOfRange, true
		}
		if valueMin.Cmp(targetMin) < 0 || valueMax.Cmp(targetMax) > 0 {
			if target.Kind == RuneType {
				return ConversionFailureInvalidScalar, true
			}
			return ConversionFailureOutOfRange, true
		}
		if target.Kind == RuneType && intervalIntersectsSurrogates(valueMin, valueMax) {
			return ConversionFailureInvalidScalar, true
		}
		return "", false
	}
	if target.Kind == FloatType && source.Kind == FloatType && (isForeignCScalar(target) || isForeignCScalar(source)) {
		if source.FloatBits == 0 || target.FloatBits == 0 || source.FloatBits > target.FloatBits {
			return ConversionFailurePrecisionLoss, true
		}
	}
	return "", false
}

// scalarIntegerDomain returns the intrinsic value domain of an integer-family
// scalar, ignoring declared contracts: built-in and c:: integers, byte,
// char (0..255), and rune (0..0x10FFFF; surrogates are excluded separately).
func scalarIntegerDomain(typ Type) (*big.Int, *big.Int, bool) {
	switch typ.Kind {
	case CharType:
		return big.NewInt(0), big.NewInt(255), true
	case RuneType:
		return big.NewInt(0), big.NewInt(0x10FFFF), true
	case IntType, UintType:
		if typ.MinInteger == nil || typ.MaxInteger == nil {
			return nil, nil, false
		}
		return new(big.Int).Set(typ.MinInteger), new(big.Int).Set(typ.MaxInteger), true
	}
	return nil, nil, false
}

// scalarValueInterval is the proven value interval of a conversion argument:
// the value-aware integer interval where Sema has one, otherwise the source
// type's domain. A rune source never holds a surrogate.
func (a *Analyzer) scalarValueInterval(argument ast.Expression, source Type) (*big.Int, *big.Int, bool) {
	domainMin, domainMax, ok := scalarIntegerDomain(source)
	if !ok {
		return nil, nil, false
	}
	if isIntegerType(source) {
		// Checked arithmetic never yields a value outside its type, so the
		// proven interval is clamped to the source domain.
		if minimum, maximum, known := a.integerInterval(argument); known {
			raiseMin(minimum, domainMin)
			lowerMax(maximum, domainMax)
			return minimum, maximum, true
		}
	}
	return domainMin, domainMax, true
}

func intervalIntersectsSurrogates(minimum, maximum *big.Int) bool {
	return minimum.Cmp(big.NewInt(0xDFFF)) <= 0 && maximum.Cmp(big.NewInt(0xD800)) >= 0
}

// conversionDomainMayFail compares the complete source and target domains,
// ignoring any value proof. Target-sized int and uint are compared at every
// supported width (32 and 64 bits), so the relation, and therefore whether
// try may protect it, does not depend on the selected target.
func conversionDomainMayFail(target, source Type) (ConversionFailureKind, bool) {
	if sameConcreteType(target, source) {
		return "", false
	}
	for _, width := range []uint16{32, 64} {
		if failure, mayFail := conversionDomainMayFailAt(portableDomainType(target, width), portableDomainType(source, width)); mayFail {
			return failure, true
		}
	}
	return "", false
}

// portableDomainType gives target-sized int and uint the domain of one
// supported pointer width; every other type keeps its own domain.
func portableDomainType(typ Type, width uint16) Type {
	switch typ.Name {
	case "int":
		return targetSignedIntegerType("int", width)
	case "uint":
		return targetUnsignedIntegerType("uint", width)
	}
	return typ
}

func conversionDomainMayFailAt(target, source Type) (ConversionFailureKind, bool) {
	targetMin, targetMax, targetOK := scalarIntegerDomain(target)
	sourceMin, sourceMax, sourceOK := scalarIntegerDomain(source)
	if !targetOK || !sourceOK {
		return "", false
	}
	if sourceMin.Cmp(targetMin) < 0 || sourceMax.Cmp(targetMax) > 0 {
		if target.Kind == RuneType {
			return ConversionFailureInvalidScalar, true
		}
		return ConversionFailureOutOfRange, true
	}
	if target.Kind == RuneType && source.Kind != RuneType && source.Kind != CharType && intervalIntersectsSurrogates(sourceMin, sourceMax) {
		return ConversionFailureInvalidScalar, true
	}
	return "", false
}

// appendCheckedConversionPoint adds the ConversionError failure point of a
// conversion protected by a try and marks it protected: an enforced checked
// conversion anywhere in the protected expression, or, as the try's own
// operand, any conversion whose relation can fail (even where this target
// proves the value fits). Nested Sec-only conversions stay outside the
// failure set until MD-056 allows their try requirement to be enforced.
func (a *Analyzer) appendCheckedConversionPoint(points []TryFailurePoint, call ast.Expression, root bool) []TryFailurePoint {
	_, checked := a.checkedConversions[call]
	_, relation := a.fallibleConversionRelations[call]
	if !checked && !(relation && root) {
		return points
	}
	a.protectedCheckedConversions[call] = true
	return append(points, TryFailurePoint{Kind: TryFailureConversion, ErrorType: a.types["ConversionError"], Expression: call, Token: expressionToken(call)})
}

// CheckedConversionOf returns the intrinsic failure a checked explicit
// conversion can produce, for lowering and tooling.
func (a *Analyzer) CheckedConversionOf(call *ast.CallExpression) (ConversionFailureKind, bool) {
	if a == nil {
		return "", false
	}
	conversion, ok := a.checkedConversions[call]
	return conversion.failure, ok
}

// reportCheckedConversions rejects every checked conversion that no try
// protects, once all bodies are analyzed.
func (a *Analyzer) reportCheckedConversions() {
	calls := make([]ast.Expression, 0, len(a.checkedConversions))
	for call := range a.checkedConversions {
		if !a.protectedCheckedConversions[call] {
			calls = append(calls, call)
		}
	}
	sort.SliceStable(calls, func(i, j int) bool {
		left, right := expressionToken(calls[i]), expressionToken(calls[j])
		if left.File != right.File {
			return left.File < right.File
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		return left.Column < right.Column
	})
	for _, call := range calls {
		conversion := a.checkedConversions[call]
		a.addErrorAtTokenWithMetadata(expressionToken(call), diagnostics.CheckedConversionRequiresTry,
			"Write try before the conversion to propagate ConversionError, handle it with try ... { Err(_) => ... }, or prove the value range first (for example with an if condition).",
			"converting %s to %s may fail with ConversionError.%s and requires try", typeDisplayName(conversion.source), typeDisplayName(conversion.target), conversion.failure)
	}
}
