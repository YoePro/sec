package sema

import (
	"math/big"

	"sec/internal/ast"
)

// intervalBound is one integer comparison of a pitfall subject against a
// literal bound, normalized to a closed half-line: subject >= Value (lower) or
// subject <= Value (upper). Op and Spelling retain the source comparison so a
// canonical range-membership replacement can reuse the written bound.
type intervalBound struct {
	Lower      bool
	Value      *big.Int
	Op         string
	Spelling   string
	Subject    ast.Expression
	Identity   string
	Comparison *ast.InfixExpression
}

// inspectIntervalCondition recognizes a `&&` or `||` of two integer
// comparisons of the same resolved subject against literal bounds. Over the
// integers the pair is either a tautology (`x >= 0 || x <= 10` covers every
// value), an impossible interval (`x > 10 && x < 5` admits none), or an exact
// interval test expressible with canonical range membership
// (`x >= 0 && x <= 10` is `x in 0..10`). Floating-point subjects are excluded
// because NaN breaks the total order the proofs rely on, and only literal
// bounds are used so the proof never depends on transient binding values.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — "Tautological interval conditions"
//   - rules/analysis/pitfall_analysis.md — "Canonical idiom guidance"
//   - rules/analysis/pitfall_analysis.md — "Meaningless comparisons from proven ranges"
//   - rules/foundations/operators.md — "Logical AND", "Logical OR", ranges and `in`
func (b *pitfallBuilder) inspectIntervalCondition(condition *ast.InfixExpression) {
	if condition == nil || (condition.Operator != "&&" && condition.Operator != "||") {
		return
	}
	left, leftOK := b.intervalBound(condition.Left)
	right, rightOK := b.intervalBound(condition.Right)
	if !leftOK || !rightOK || left.Identity != right.Identity || left.Lower == right.Lower {
		return
	}
	lower, upper := left, right
	if !lower.Lower {
		lower, upper = right, left
	}
	switch condition.Operator {
	case "||":
		// [L, +inf) ∪ (-inf, U] covers every integer exactly when L <= U + 1.
		if lower.Value.Cmp(new(big.Int).Add(upper.Value, big.NewInt(1))) <= 0 {
			b.reportTautologicalInterval(condition, lower, upper)
		}
	case "&&":
		if lower.Value.Cmp(upper.Value) > 0 {
			b.reportImpossibleInterval(condition, lower, upper)
			return
		}
		b.reportRangeMembershipIdiom(condition, lower, upper)
	}
}

// intervalBound normalizes `subject op literal` or `literal op subject`.
func (b *pitfallBuilder) intervalBound(expression ast.Expression) (intervalBound, bool) {
	comparison, ok := expression.(*ast.InfixExpression)
	if !ok {
		return intervalBound{}, false
	}
	op := comparison.Operator
	subject, literal := comparison.Left, comparison.Right
	value, spelling, literalOK := intervalLiteral(literal)
	if !literalOK {
		subject, literal = comparison.Right, comparison.Left
		value, spelling, literalOK = intervalLiteral(literal)
		if !literalOK {
			return intervalBound{}, false
		}
		op = mirroredComparison(op)
	}
	if op == "" {
		return intervalBound{}, false
	}
	if subjectType, typed := b.analyzer.expressionTypes[subject]; !typed || !isIntegerType(subjectType) {
		return intervalBound{}, false
	}
	identity, ok := b.expressionIdentity(subject)
	if !ok {
		return intervalBound{}, false
	}
	bound := intervalBound{Op: op, Spelling: spelling, Subject: subject, Identity: identity, Comparison: comparison}
	switch op {
	case ">=":
		bound.Lower, bound.Value = true, value
	case ">":
		bound.Lower, bound.Value = true, new(big.Int).Add(value, big.NewInt(1))
	case "<=":
		bound.Value = value
	case "<":
		bound.Value = new(big.Int).Sub(value, big.NewInt(1))
	default:
		return intervalBound{}, false
	}
	return bound, true
}

// mirroredComparison returns the comparison with its operands swapped, or ""
// when the operator is not an ordered comparison.
func mirroredComparison(op string) string {
	switch op {
	case "<":
		return ">"
	case "<=":
		return ">="
	case ">":
		return "<"
	case ">=":
		return "<="
	}
	return ""
}

// intervalLiteral accepts an integer literal, optionally negated.
func intervalLiteral(expression ast.Expression) (*big.Int, string, bool) {
	switch expression := expression.(type) {
	case *ast.IntegerLiteral:
		value, ok := ast.ParseIntegerLiteralLexeme(expression.Token.Lexeme)
		return value, expression.Token.Lexeme, ok
	case *ast.PrefixExpression:
		literal, ok := expression.Right.(*ast.IntegerLiteral)
		if expression.Operator != "-" || !ok {
			return nil, "", false
		}
		value, parsed := ast.ParseIntegerLiteralLexeme(literal.Token.Lexeme)
		if !parsed {
			return nil, "", false
		}
		return new(big.Int).Neg(value), "-" + literal.Token.Lexeme, true
	}
	return nil, "", false
}

// rangeMembership spells `subject in lower..upper` (both inclusive) or
// `subject in lower..<upper` when the written bounds map directly onto a
// canonical range; strict lower bounds have no direct spelling.
func rangeMembership(subject ast.Expression, lower, upper intervalBound) (string, bool) {
	if lower.Op != ">=" {
		return "", false
	}
	switch upper.Op {
	case "<=":
		return subject.String() + " in " + lower.Spelling + ".." + upper.Spelling, true
	case "<":
		return subject.String() + " in " + lower.Spelling + "..<" + upper.Spelling, true
	}
	return "", false
}

// pitfallPureOperand reports whether evaluating expression twice or once is
// indistinguishable: resolved bindings and chains of stored struct fields.
// Properties may run user getters and are not pure for rewriting.
func (b *pitfallBuilder) pitfallPureOperand(expression ast.Expression) bool {
	switch expression := expression.(type) {
	case *ast.Identifier:
		return true
	case *ast.MemberExpression:
		if expression.Property == nil || !b.pitfallPureOperand(expression.Object) {
			return false
		}
		objectType, ok := b.analyzer.expressionTypes[expression.Object]
		if !ok {
			return false
		}
		for _, field := range dereferenceType(objectType).Fields {
			if field.Name == expression.Property.Value {
				return true
			}
		}
	}
	return false
}

func intervalEvidence(lower, upper intervalBound) []PitfallEvidence {
	return []PitfallEvidence{
		{Strength: PitfallEvidenceProof, Fact: "both comparisons test the same resolved integer value", Source: expressionToken(lower.Subject)},
		{Strength: PitfallEvidenceProof, Fact: "lower bound " + lower.Value.String() + " from " + lower.Comparison.String(), Source: lower.Comparison.Token},
		{Strength: PitfallEvidenceProof, Fact: "upper bound " + upper.Value.String() + " from " + upper.Comparison.String(), Source: upper.Comparison.Token},
	}
}

func (b *pitfallBuilder) reportTautologicalInterval(condition *ast.InfixExpression, lower, upper intervalBound) {
	finding := PitfallFinding{
		Rule:           PitfallTautologicalInterval,
		Family:         PitfallBoundsAndRanges,
		Classification: PitfallLikelyMistake,
		Confidence:     PitfallConfidenceProven,
		Subject:        PitfallSubject{Expression: condition.String(), Source: condition.Token},
		EvidenceFor: append(intervalEvidence(lower, upper), PitfallEvidence{
			Strength: PitfallEvidenceProof,
			Fact:     "the union of both half-lines covers every integer, so the condition is always true",
			Source:   condition.Token,
		}),
		OwningRule: "tautological-interval-condition",
	}
	// § "Tautological interval conditions": an inclusive interval written with
	// `||` most likely meant membership; prefer `in` over swapping operators.
	if lower.Value.Cmp(upper.Value) <= 0 {
		if replacement, ok := rangeMembership(lower.Subject, lower, upper); ok {
			finding.Actions = []PitfallSuggestedAction{{
				Kind:        PitfallSuggestedEdit,
				Title:       "test interval membership",
				Replacement: replacement,
				Source:      condition.Token,
			}}
		}
	}
	b.add(finding)
}

func (b *pitfallBuilder) reportImpossibleInterval(condition *ast.InfixExpression, lower, upper intervalBound) {
	b.add(PitfallFinding{
		Rule:           PitfallImpossibleInterval,
		Family:         PitfallBoundsAndRanges,
		Classification: PitfallLikelyMistake,
		Confidence:     PitfallConfidenceProven,
		Subject:        PitfallSubject{Expression: condition.String(), Source: condition.Token},
		EvidenceFor: append(intervalEvidence(lower, upper), PitfallEvidence{
			Strength: PitfallEvidenceProof,
			Fact:     "the lower bound exceeds the upper bound, so the condition is always false",
			Source:   condition.Token,
		}),
		OwningRule: "impossible-interval-condition",
	})
}

func (b *pitfallBuilder) reportRangeMembershipIdiom(condition *ast.InfixExpression, lower, upper intervalBound) {
	replacement, ok := rangeMembership(lower.Subject, lower, upper)
	if !ok {
		return
	}
	finding := PitfallFinding{
		Rule:           PitfallRangeMembershipIdiom,
		Family:         PitfallBoundsAndRanges,
		Classification: PitfallSuspiciousIntent,
		Confidence:     PitfallConfidenceProven,
		Subject:        PitfallSubject{Expression: condition.String(), Source: condition.Token},
		EvidenceFor: append(intervalEvidence(lower, upper), PitfallEvidence{
			Strength: PitfallEvidenceProof,
			Fact:     "the conjunction is exactly membership in a non-empty literal range",
			Source:   condition.Token,
		}),
		OwningRule: "canonical-range-membership",
		Actions: []PitfallSuggestedAction{{
			Kind:        PitfallProvenFix,
			Title:       "use canonical range membership",
			Replacement: replacement,
			Source:      condition.Token,
		}},
	}
	// § "Required control-flow tests": a side-effecting subject would be
	// evaluated once instead of twice, so the rewrite is not proven.
	if !b.pitfallPureOperand(lower.Subject) || !b.pitfallPureOperand(upper.Subject) {
		against := []PitfallEvidence{{
			Strength: PitfallEvidenceSuppressing,
			Fact:     "the subject is not a binding or a chain of stored fields",
			Source:   expressionToken(lower.Subject),
		}}
		finding.State = PitfallStateSuppressed
		finding.Actions = nil
		finding.EvidenceAgainst = against
		finding.Suppression = &PitfallSuppression{
			Reason:   "the compared value may have side effects, so rewriting two evaluations into one is not semantics-preserving",
			Evidence: against,
		}
	}
	b.add(finding)
}
