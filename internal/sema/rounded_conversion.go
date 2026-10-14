package sema

import (
	"math/big"

	"sec/internal/ast"
)

// RoundedConversionPlan is the compiler-resolved explicit numeric or unit
// conversion with a RoundingMode (MD-047): result = round(value * Factor +
// Offset) by Mode, applied exactly once to the exact mathematical value, then
// checked against the target carrier. Proof says why no runtime check is
// needed; a Checked plan fails with ConversionError.OutOfRange under try.
//
// Rules:
//   - rules/types/units.md — "Explicit rounding"
//   - rules/library/core-library.md — § 10.3 "Rounding modes"
type RoundedConversionPlan struct {
	Source, Target Type
	Mode           string
	Factor, Offset *big.Rat
	// Constant is the rounded result of a compile-time constant source.
	Constant *big.Int
	// Minimum and Maximum bound a proven integer-domain conversion.
	Minimum, Maximum *big.Int
	Checked          bool
}

// RoundedConversionOf returns the rounding plan of an explicit conversion.
func (a *Analyzer) RoundedConversionOf(call ast.Expression) (RoundedConversionPlan, bool) {
	if a == nil || call == nil {
		return RoundedConversionPlan{}, false
	}
	plan, ok := a.roundedConversions[call]
	return plan, ok
}

// isRoundingModeArgument recognizes the second argument of a rounded
// conversion syntactically, so a factor-provided unit conversion keeps its
// own path.
func (a *Analyzer) isRoundingModeArgument(expr ast.Expression) bool {
	member, ok := expr.(*ast.MemberExpression)
	if !ok {
		return false
	}
	path, ok := typePathFromExpression(member.Object)
	if !ok {
		return false
	}
	_, declared := a.types["RoundingMode"]
	return declared && a.resolveTypeName(path) == "RoundingMode"
}

// inferRoundedConversion implements `T(value, RoundingMode.M)` and
// `T<U>(value, RoundingMode.M)` for an integer target carrier; expr is the
// call or conversion node that keys the plan.
func (a *Analyzer) inferRoundedConversion(expr ast.Expression, valueExpr, modeExpr ast.Expression, target Type) (Type, expressionValue) {
	display := expressionValue{Display: expr.String()}
	invalid := Type{Kind: InvalidType}
	token := expressionToken(expr)
	mode, ok := a.roundingModeArgument(modeExpr)
	source, _ := a.inferExpression(valueExpr)
	if !ok || source.Kind == InvalidType {
		return invalid, display
	}
	if !isNumericType(source) || !isNumericType(target) || !a.canExplicitConvert(target, source) {
		a.addErrorAtToken(token, "cannot convert %s to %s", typeDisplayName(source), typeDisplayName(target))
		return invalid, display
	}
	if (hasUnitSemantics(target) || hasUnitSemantics(source)) && !a.validateExplicitUnitConversion(token, target, source) {
		return invalid, display
	}
	if !isIntegerType(target) {
		a.addErrorAtTokenWithMetadata(token, "",
			"This compiler applies an explicit RoundingMode to integer target carriers; rounding to a decimal or binary floating carrier is not implemented yet.",
			"rounded conversion to %s is not implemented yet", typeDisplayName(target))
		return invalid, display
	}
	factor, offset := big.NewRat(1, 1), big.NewRat(0, 1)
	if hasUnitSemantics(target) && hasUnitSemantics(source) {
		f, o, coordinates := unitConversionCoordinates(effectiveUnitSemantics(source), effectiveUnitSemantics(target))
		if !coordinates {
			a.addErrorAtToken(token, "cannot convert %s to %s: the units have no fixed linear relation", typeDisplayName(source), typeDisplayName(target))
			return invalid, display
		}
		factor, offset = f, o
	}
	plan := RoundedConversionPlan{Source: source, Target: target, Mode: mode, Factor: factor, Offset: offset}
	low, high, bounded := scalarIntegerDomain(target)
	convert := func(value *big.Rat) *big.Int {
		rounded, _ := roundRat(new(big.Rat).Add(new(big.Rat).Mul(value, factor), offset), mode)
		return rounded
	}
	if value, constant := a.exactQuantityValue(valueExpr); constant {
		result := convert(value)
		if bounded && (result.Cmp(low) < 0 || result.Cmp(high) > 0) {
			a.addErrorAtToken(token, "%s rounds to %s, which is outside %s", valueExpr.String(), result.String(), typeDisplayName(target))
			return invalid, display
		}
		plan.Constant = result
	} else if minimum, maximum, known := a.scalarValueInterval(valueExpr, source); known && isIntegerType(source) {
		first, second := convert(new(big.Rat).SetInt(minimum)), convert(new(big.Rat).SetInt(maximum))
		if first.Cmp(second) > 0 {
			first, second = second, first
		}
		if bounded && first.Cmp(low) >= 0 && second.Cmp(high) <= 0 {
			plan.Minimum, plan.Maximum = first, second
		} else {
			plan.Checked = true
		}
	} else {
		plan.Checked = true
	}
	a.roundedConversions[expr] = plan
	if plan.Checked {
		a.checkedConversions[expr] = checkedConversion{source: source, target: target, failure: ConversionFailureOutOfRange}
		a.fallibleConversionRelations[expr] = ConversionFailureOutOfRange
	} else {
		delete(a.checkedConversions, expr)
		delete(a.fallibleConversionRelations, expr)
	}
	return target, display
}
