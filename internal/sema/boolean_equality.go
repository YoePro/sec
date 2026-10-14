package sema

const (
	ResolvedBoolCompareEQ ResolvedOperatorKind = "bool-compare-eq"
	ResolvedBoolCompareNE ResolvedOperatorKind = "bool-compare-ne"
)

// resolvedBooleanEquality records eager, non-fallible equality over builtin
// bool operands after Sema has validated the source expression.
// Rules: rules/foundations/operators.md — "Boolean equality", "Evaluation order".
func resolvedBooleanEquality(operator string, left, right, result Type) (ResolvedOperator, bool) {
	if left.Kind != BoolType || right.Kind != BoolType || result.Kind != BoolType || left.Name != "bool" || right.Name != "bool" {
		return ResolvedOperator{}, false
	}
	kind := ResolvedBoolCompareEQ
	switch operator {
	case "==":
	case "!=":
		kind = ResolvedBoolCompareNE
	default:
		return ResolvedOperator{}, false
	}
	return ResolvedOperator{Kind: kind, LeftType: left, RightType: &right, ResultType: result, FailureBehavior: OperatorDoesNotFail}, true
}
