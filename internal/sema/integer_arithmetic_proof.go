package sema

import "sec/internal/ast"

// proveIntegerArithmeticSafe proves that every admitted operand combination
// fits the operation's intrinsic integer carrier. It uses arbitrary-precision
// intervals, never host-width arithmetic, and does not claim that nested
// operations, calls or contracts are safe merely because this result fits.
// Checked lowering is retained; this is evidence for the possible failure set.
// Rules: rules/errors/errorhandling.md — §§11,13.1;
// rules/errors/runtime_checks.md — Static proof and check elimination;
// rules/foundations/operators.md — Integer arithmetic failure.
func (a *Analyzer) proveIntegerArithmeticSafe(expr *ast.InfixExpression, resolved ResolvedOperator) bool {
	switch resolved.Kind {
	case ResolvedIntegerAddChecked, ResolvedIntegerSubtractChecked, ResolvedIntegerMultiplyChecked:
	default:
		return false
	}
	if !isBuiltinIntegerOperatorType(resolved.LeftType) || resolved.RightType == nil || !isBuiltinIntegerOperatorType(*resolved.RightType) {
		return false
	}
	minimum, maximum, known := a.integerInterval(expr)
	representation, _, represented := a.integerRepresentation(resolved.ResultType)
	return known && represented && minimum.Cmp(maximum) <= 0 &&
		representation.MinInteger != nil && representation.MaxInteger != nil &&
		minimum.Cmp(representation.MinInteger) >= 0 && maximum.Cmp(representation.MaxInteger) <= 0
}
