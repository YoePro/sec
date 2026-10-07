package sema

import "sec/internal/ast"

// PitfallFixSafety records the equivalence obligations established for one
// exact replacement. Empty obligations are unknown, never proof of safety.
// This value contains no mutable collections or compiler-owned AST pointers.
// Rules: rules/analysis/pitfall_analysis.md — "Fix safety", "Corrective actions".
type PitfallFixSafety struct {
	Rule            PitfallRuleID
	Replacement     string
	EvaluationOrder string
	EvaluationCount string
	Effects         string
	Failure         string
	Ownership       string
	Borrow          string
	ControlFlow     string
}

// verifiedFor checks that every observable dimension has explicit evidence and
// that the certificate belongs to the current supported rule/replacement.
// Rules: rules/analysis/pitfall_analysis.md — "Fix safety".
func (proof PitfallFixSafety) verifiedFor(rule PitfallRuleID, replacement string) bool {
	switch rule {
	case PitfallBooleanLiteralComparison, PitfallRangeMembershipIdiom, PitfallFragileInclusiveLength,
		PitfallTautologicalInterval, PitfallImpossibleInterval, PitfallMeaninglessComparison:
	default:
		return false
	}
	return proof.Rule == rule && proof.Replacement == replacement && replacement != "" &&
		proof.EvaluationOrder != "" && proof.EvaluationCount != "" && proof.Effects != "" &&
		proof.Failure != "" && proof.Ownership != "" && proof.Borrow != "" && proof.ControlFlow != ""
}

// requirePitfallFixSafety prevents any family from publishing an automatic fix
// with missing or mismatched proof. Unverified actions remain suggested edits.
// Rules: rules/analysis/pitfall_analysis.md — "Fix safety", "Heuristic edit suggestions".
func requirePitfallFixSafety(finding *PitfallFinding) {
	for index := range finding.Actions {
		action := &finding.Actions[index]
		if action.Kind == PitfallProvenFix && !action.Safety.verifiedFor(finding.Rule, action.Replacement) {
			action.Kind = PitfallSuggestedEdit
			action.Safety = PitfallFixSafety{}
		}
	}
}

// booleanFixSafety proves a truth-table simplification that evaluates the bool
// operand exactly once, even when it is an effectful call, getter, try or move.
// The removed operand is a literal; bool equality/conversion is infallible and
// non-consuming. Numeric intent repairs have no equivalence certificate.
// Rules: rules/analysis/pitfall_analysis.md — "Boolean intent pitfalls", "Fix safety";
// rules/foundations/operators.md — "Equality", "Logical NOT".
func booleanFixSafety(intent booleanComparisonIntent) PitfallFixSafety {
	if intent.numericLiteral {
		return PitfallFixSafety{}
	}
	return PitfallFixSafety{
		Rule: PitfallBooleanLiteralComparison, Replacement: intent.replacement,
		EvaluationOrder: "the retained bool operand executes in its original position; only a literal and bool equality/conversion disappear",
		EvaluationCount: "the retained bool operand executes exactly once in both forms",
		Effects:         "all effects of the retained operand are preserved; removed bool operations have none",
		Failure:         "all failures of the retained operand are preserved; removed bool operations are infallible",
		Ownership:       "the retained operand is unchanged; bool comparison/conversion does not transfer ownership",
		Borrow:          "the retained operand's borrow behavior is unchanged",
		ControlFlow:     "the boolean truth table, including negation and short circuit inside the retained operand, is identical",
	}
}

// membershipFixSafety permits eliminating a repeated read only for canonical
// stable, non-observable stored-value reads. Both bounds are already proven
// compile-time constants by interval analysis; no runtime bound evaluation,
// getter, volatile access, move, or index failure may be removed/reordered.
// Rules: rules/analysis/pitfall_analysis.md — "Canonical idiom guidance", "Fix safety";
// rules/foundations/operators.md — "Membership expression", "Logical AND".
func (b *pitfallBuilder) membershipFixSafety(lower, upper intervalBound, replacement string) PitfallFixSafety {
	if !b.pitfallPureOperand(lower.Subject) || !b.pitfallPureOperand(upper.Subject) {
		return PitfallFixSafety{}
	}
	return PitfallFixSafety{
		Rule: PitfallRangeMembershipIdiom, Replacement: replacement,
		EvaluationOrder: "only stable stored-value reads and compile-time constant bounds participate",
		EvaluationCount: "one or two observationally irrelevant stored-value reads become one",
		Effects:         "canonical stored fields/bindings are read without getters, calls, or volatile/register access",
		Failure:         "stored reads and constant comparisons/membership add no panic or error operation",
		Ownership:       "scalar ordered-value reads transfer no ownership; no move is rewritten",
		Borrow:          "no reference is created or consumed by the comparisons or membership test",
		ControlFlow:     "the non-empty lower/upper interval has exactly the same truth table",
	}
}

// lengthEndpointFixSafety proves the saved inclusive Len-1 endpoint equals a
// half-open Len endpoint only when entry proves nonempty and evaluating the
// written start/receiver cannot invalidate that proof. Loop bodies remain
// untouched: their effects and ownership/borrow/failure behavior execute for
// exactly the same sequence, including break/continue and zero iterations.
// Rules: rules/analysis/pitfall_analysis.md — "Avoiding fragile `0..len - 1` workarounds", "Fix safety";
// rules/control-flow/flowcontrol_for.md — §§23,25,29,39.
func (b *pitfallBuilder) lengthEndpointFixSafety(rangeExpression *ast.RangeExpression, length ast.Expression, replacement string) PitfallFixSafety {
	member, ok := length.(*ast.MemberExpression)
	if !ok || !b.pitfallPureOperand(member.Object) {
		return PitfallFixSafety{}
	}
	if _, constant := b.analyzer.integerConstantValue(rangeExpression.Start); !constant && !b.pitfallPureOperand(rangeExpression.Start) {
		return PitfallFixSafety{}
	}
	return PitfallFixSafety{
		Rule: PitfallFragileInclusiveLength, Replacement: replacement,
		EvaluationOrder: "start and canonical Len receiver retain their order and cannot invalidate the entry nonempty proof",
		EvaluationCount: "start and Len execute once in both forms; the body receives the same ascending unit-step values",
		Effects:         "only infallible Len-1 arithmetic on a proven nonempty collection is removed",
		Failure:         "nonempty proves unsigned subtraction cannot underflow; both endpoints are representable and start/body failures remain unchanged",
		Ownership:       "start and body are unchanged; the scalar endpoint arithmetic transfers no ownership",
		Borrow:          "collection and loop body borrowing remain unchanged",
		ControlFlow:     "ascending inclusive end Len-1 and exclusive end Len select the same iterations and exits",
	}
}

// pitfallPureOperand proves that reducing repeated reads is observationally
// irrelevant. Property plans (including unqualified getters), addressed or
// volatile storage, and register fields are excluded even if their syntax
// resembles a binding/field. Missing semantic resolution cannot prove purity.
// Rules: rules/analysis/pitfall_analysis.md — "Fix safety";
// rules/memory/copy_move.md — §19.1 "Volatile is storage semantics";
// rules/declarations/properties.md — §15 "Lowering".
func (b *pitfallBuilder) pitfallPureOperand(expression ast.Expression) bool {
	if _, property := b.analyzer.ResolvedPropertyAccessOf(expression); property {
		return false
	}
	switch expression := expression.(type) {
	case *ast.Identifier:
		definitions := b.analyzer.definitionTokens[sourceTokenLocation(expression.Token)]
		if len(definitions) != 1 {
			return false
		}
		if symbol, ok := b.analyzer.symbols[expression.Value]; ok && sameSourceToken(symbol.Token, definitions[0]) {
			return !symbol.Volatile && !symbol.Addressed
		}
		_, resolved := b.analyzer.ResolvedBindingOf(expression)
		return resolved
	case *ast.MemberExpression:
		if !b.pitfallPureOperand(expression.Object) {
			return false
		}
		member, ok := b.analyzer.ResolvedStructMemberOf(expression)
		return ok && member.Kind == MemberStoredField && dereferenceType(member.OwnerType).Kind == StructType
	}
	return false
}
