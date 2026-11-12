package sema

import "sec/internal/ast"

// tryOperand executes fallible string materialization using evaluator-owned
// storage, and checked conversions using their resolved Sec error channel.
// Resource exhaustion remains an evaluator abort outside Sec try handlers;
// explicit runtime allocator calls are still forbidden, never substituted.
// Rules: rules/compiler/compile_time_evaluation.md — §§24–25,33,46,49(5);
// rules/foundations/operators.md — String concatenation.
// rules/types/types.md — Explicit conversions.
func (x *cteExecutor) tryOperand(expr ast.Expression) (DefaultConstant, bool) {
	if plan, ok := x.a.StringConcatPlanOf(expr); ok && plan.Runtime {
		value, ok := x.eval(expr)
		if !ok {
			return DefaultConstant{}, false
		}
		if value.Kind != StringType {
			x.fail(cteImplementationGap, expressionToken(expr), "string concatenation did not establish text during compile-time evaluation")
			return DefaultConstant{}, false
		}
		typ := x.a.intrinsicGenericType("Result", x.a.types["string"], plan.Allocation.FailureType)
		return variantValue(typ, "Ok", &value), true
	}
	if call, target, ok := x.conversionTarget(expr); ok {
		value, failure, ok := x.checkedConversion(call, target)
		if !ok {
			return DefaultConstant{}, false
		}
		if failure != nil {
			return variantValue(Type{Kind: ResultType}, "Err", failure), true
		}
		return variantValue(Type{Kind: ResultType}, "Ok", &value), true
	}
	return x.eval(expr)
}
