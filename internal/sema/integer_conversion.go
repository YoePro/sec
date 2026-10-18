package sema

import (
	"math/big"

	"sec/internal/ast"
)

// ResolvedIntegerConversion is Sema's plan for an explicit conversion
// between two built-in integer types (`int(x)`, `int8(x)`, `uint(x)`, ...).
// Exact means no runtime failure branch is needed: the source domain fits
// the target domain on every supported target width, or the argument is a
// compile-time constant Sema already validated against the target
// (types.md "Explicit conversions": a proven conversion emits no runtime
// validation). A non-exact conversion needs the checked form and `try`.
//
// Rules:
//   - rules/types/types.md — "Explicit conversions"
//   - rules/mlir/packages/sec-mlir-dialect_package20.md — §§ 44, 60–63 (NumericConvertExactOp)
type ResolvedIntegerConversion struct {
	SourceType Type
	TargetType Type
	Exact      bool
}

// recordIntegerConversion publishes the plan of a built-in integer to
// built-in integer conversion call.
func (a *Analyzer) recordIntegerConversion(call *ast.CallExpression, target, source Type) {
	if call == nil || len(call.Arguments) != 1 || !isBuiltinConversionInteger(target) || !isBuiltinConversionInteger(source) {
		return
	}
	_, mayFail := conversionDomainMayFail(target, source)
	if mayFail {
		if _, constant := a.integerConstantValue(call.Arguments[0]); constant {
			mayFail = false
		}
	}
	if a.resolvedIntegerConversions == nil {
		a.resolvedIntegerConversions = map[*ast.CallExpression]ResolvedIntegerConversion{}
	}
	a.resolvedIntegerConversions[call] = ResolvedIntegerConversion{SourceType: source, TargetType: target, Exact: !mayFail}
}

// ResolvedIntegerConversionOf returns the recorded plan without inference.
func (a *Analyzer) ResolvedIntegerConversionOf(call *ast.CallExpression) (ResolvedIntegerConversion, bool) {
	if a == nil || call == nil {
		return ResolvedIntegerConversion{}, false
	}
	plan, ok := a.resolvedIntegerConversions[call]
	return plan, ok
}

// isBuiltinConversionInteger admits the built-in signed and unsigned integer
// types; named, unit-bearing, contract, char and rune types keep their own
// conversion rules.
func isBuiltinConversionInteger(typ Type) bool {
	return (typ.Kind == IntType || typ.Kind == UintType) && !typ.Named && len(typ.Contracts) == 0 && !hasUnitSemantics(typ)
}

// recordForRangeStep publishes the compile-time value of an explicit range
// step, so lowering can choose the progression direction without
// re-evaluating it (flowcontrol_for.md §§ 23, 26).
func (a *Analyzer) recordForRangeStep(stmt *ast.ForStatement) {
	if stmt == nil || stmt.Step == nil {
		return
	}
	if value, ok := a.integerConstantValue(stmt.Step); ok {
		a.resolvedForRangeSteps[stmt] = new(big.Int).Set(value)
	} else {
		delete(a.resolvedForRangeSteps, stmt)
	}
}

// ResolvedForRangeStepOf returns the compile-time value of a for loop's
// explicit integer step, when it is a compile-time constant.
func (a *Analyzer) ResolvedForRangeStepOf(stmt *ast.ForStatement) (*big.Int, bool) {
	if a == nil || stmt == nil {
		return nil, false
	}
	value, ok := a.resolvedForRangeSteps[stmt]
	if !ok {
		return nil, false
	}
	return new(big.Int).Set(value), true
}
