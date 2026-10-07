package sema

import "sec/internal/ast"

// constantBooleanAction separates a proven boolean result from permission to
// omit the expression's evaluation. Canonical intent repairs remain independent
// suggested edits; this action simplifies only the already-proven result.
// Rules: rules/analysis/pitfall_analysis.md — "Corrective actions", "Fix safety",
// "Tautological interval conditions", "Meaningless comparisons from proven ranges".
func (b *pitfallBuilder) constantBooleanAction(expression ast.Expression, rule PitfallRuleID, outcome string) PitfallSuggestedAction {
	action := PitfallSuggestedAction{Kind: PitfallSuggestedEdit, Title: "replace with " + outcome + " only if evaluating the original expression may be omitted", Replacement: outcome, Source: expressionToken(expression)}
	if !b.constantBooleanEvaluationPure(expression) {
		return action
	}
	action.Kind = PitfallProvenFix
	action.Title = "simplify the proven result to " + outcome
	action.Safety = PitfallFixSafety{
		Rule: rule, Replacement: outcome,
		EvaluationOrder: "only non-observable stored scalar reads and compile-time constants are removed; surrounding evaluation order is unchanged",
		EvaluationCount: "removed scalar reads have no observable evaluation-count effect, including on short-circuit paths",
		Effects:         "no calls, getters, volatile/addressed storage, mutations or other effectful expressions are removed",
		Failure:         "only built-in scalar comparisons and logical operations are removed; no checked arithmetic, indexing, try or panic operation participates",
		Ownership:       "stored scalar reads and comparisons transfer no ownership; no move or consuming operation is removed",
		Borrow:          "no reference is created, consumed or retained by the removed scalar comparisons",
		ControlFlow:     "the owning interval or type-range proof establishes the same boolean result on every path; surrounding branches and exits are unchanged",
	}
	return action
}

// constantBooleanEvaluationPure checks the complete expression, including
// unrelated operands of a proven interval chain. A truth proof for two bounds
// does not authorize dropping other short-circuited effects or failures.
// Rules: rules/analysis/pitfall_analysis.md — "Fix safety";
// rules/foundations/operators.md — "Logical AND", "Logical OR", "Equality".
func (b *pitfallBuilder) constantBooleanEvaluationPure(expression ast.Expression) bool {
	if _, constant := b.analyzer.comparisonConstantOperands[expression]; constant {
		return true
	}
	if _, _, literal := intervalLiteral(expression); literal {
		return true
	}
	switch expression := expression.(type) {
	case *ast.BooleanLiteral:
		return true
	case *ast.InfixExpression:
		switch expression.Operator {
		case "&&", "||", "<", "<=", ">", ">=", "==", "!=":
			return b.constantBooleanEvaluationPure(expression.Left) && b.constantBooleanEvaluationPure(expression.Right)
		}
		return false
	default:
		typ, ok := b.analyzer.expressionTypes[expression]
		typ = dereferenceType(typ)
		return ok && (isIntegerType(typ) || typ.Kind == BoolType || typ.Kind == CharType || typ.Kind == RuneType) && b.pitfallPureOperand(expression)
	}
}
