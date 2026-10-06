package sema

import (
	"math/big"

	"sec/internal/ast"
)

// intervalBound is one ordered comparison of a pitfall subject against a
// constant bound, normalized to a closed half-line: subject >= Value (lower) or
// subject <= Value (upper). Op and Spelling retain the source comparison so a
// canonical range-membership replacement can reuse the written bound; Named
// marks a bound written as a named compile-time constant.
type intervalBound struct {
	Lower      bool
	Value      *big.Int
	Op         string
	Spelling   string
	Named      bool
	Subject    ast.Expression
	Identity   string
	Comparison *ast.InfixExpression
}

// inspectIntervalCondition recognizes a `&&` or `||` chain whose operands
// include ordered comparisons of one resolved subject against constant bounds.
// The chain is either a tautology (`x >= 0 || x <= 10`
// covers every value), an impossible interval (`x > 10 && x < 5` admits
// none), or, for exactly one lower and one upper bound, an interval test
// expressible with canonical range membership (`x >= 0 && x <= 10` is
// `x in 0..10`). Other operands of a chain only add disjuncts or conjuncts, so
// they never weaken the proof. Subjects are integers ordered by value, `char`
// ordered by its one-byte value, and `rune` ordered by Unicode scalar value;
// floating-point subjects
// are excluded because NaN breaks the total order, and enums have no ordered
// comparison in Sec 0.1. Bounds are literals or named compile-time constants
// whose value Sema proved in the comparison's own scope, so no proof depends
// on transient binding values. A constrained type domain never decides a
// chain the integer proof leaves open: Sema rejects every constant bound
// outside the subject's domain [min, max], so an empty conjunction would need
// L = max+1 <= U and a covering disjunction U = max with L > max+1, and
// neither bound exists.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — "Tautological interval conditions"
//   - rules/analysis/pitfall_analysis.md — "Canonical idiom guidance"
//   - rules/analysis/pitfall_analysis.md — "Meaningless comparisons from proven ranges"
//   - rules/foundations/operators.md — "Logical AND", "Logical OR", ranges and `in`
//   - rules/foundations/operators.md — "Character ordering", "Rune ordering", "Enum equality"
func (b *pitfallBuilder) inspectIntervalCondition(condition *ast.InfixExpression) {
	if condition == nil || (condition.Operator != "&&" && condition.Operator != "||") || b.handledIntervalChains[condition] {
		return
	}
	operands := b.intervalChainOperands(condition)
	order := []string{}
	groups := map[string][]intervalBound{}
	for _, operand := range operands {
		bound, ok := b.intervalBound(operand)
		if !ok {
			continue
		}
		if _, seen := groups[bound.Identity]; !seen {
			order = append(order, bound.Identity)
		}
		groups[bound.Identity] = append(groups[bound.Identity], bound)
	}
	for _, identity := range order {
		bounds := groups[identity]
		switch condition.Operator {
		case "||":
			// The loosest half-lines decide whether the union covers the domain.
			lower, upper, ok := extremeIntervalBounds(bounds, false)
			if !ok {
				continue
			}
			// [L, +inf) ∪ (-inf, U] covers every value exactly when L <= U + 1.
			if lower.Value.Cmp(new(big.Int).Add(upper.Value, big.NewInt(1))) <= 0 {
				b.reportTautologicalInterval(condition, lower, upper, len(operands) == 2)
				return
			}
		case "&&":
			// The tightest half-lines decide whether the intersection is empty.
			lower, upper, ok := extremeIntervalBounds(bounds, true)
			if !ok {
				continue
			}
			if lower.Value.Cmp(upper.Value) > 0 {
				b.reportImpossibleInterval(condition, lower, upper)
				return
			}
			if len(operands) == 2 && len(bounds) == 2 {
				b.reportRangeMembershipIdiom(condition, lower, upper)
				return
			}
		}
	}
}

// intervalChainOperands flattens a chain of one logical operator, so
// `a && b && c` yields its three operands; nested links are marked handled so
// the walk does not inspect a sub-chain again as its own condition.
func (b *pitfallBuilder) intervalChainOperands(condition *ast.InfixExpression) []ast.Expression {
	operands := []ast.Expression{}
	var collect func(ast.Expression)
	collect = func(expression ast.Expression) {
		if link, ok := expression.(*ast.InfixExpression); ok && link.Operator == condition.Operator {
			if link != condition {
				b.handledIntervalChains[link] = true
			}
			collect(link.Left)
			collect(link.Right)
			return
		}
		operands = append(operands, expression)
	}
	collect(condition)
	return operands
}

// extremeIntervalBounds selects one lower and one upper bound: the tightest
// pair (greatest lower, least upper) for a conjunction, or the loosest pair
// (least lower, greatest upper) for a disjunction. The first written bound
// wins ties so evidence follows source order.
func extremeIntervalBounds(bounds []intervalBound, tightest bool) (intervalBound, intervalBound, bool) {
	var lower, upper *intervalBound
	for index := range bounds {
		bound := &bounds[index]
		if bound.Lower {
			if lower == nil || (tightest && bound.Value.Cmp(lower.Value) > 0) || (!tightest && bound.Value.Cmp(lower.Value) < 0) {
				lower = bound
			}
			continue
		}
		if upper == nil || (tightest && bound.Value.Cmp(upper.Value) < 0) || (!tightest && bound.Value.Cmp(upper.Value) > 0) {
			upper = bound
		}
	}
	if lower == nil || upper == nil {
		return intervalBound{}, intervalBound{}, false
	}
	return *lower, *upper, true
}

// intervalBound normalizes `subject op bound` or `bound op subject`. Exactly
// one operand may be a constant; a comparison of two constants is a constant
// condition owned by the unreachable-code rules.
func (b *pitfallBuilder) intervalBound(expression ast.Expression) (intervalBound, bool) {
	comparison, ok := expression.(*ast.InfixExpression)
	if !ok {
		return intervalBound{}, false
	}
	op := comparison.Operator
	subject, constant := comparison.Left, comparison.Right
	value, spelling, named, constantOK := b.intervalBoundValue(constant)
	if !constantOK {
		subject, constant = comparison.Right, comparison.Left
		value, spelling, named, constantOK = b.intervalBoundValue(constant)
		if !constantOK {
			return intervalBound{}, false
		}
		op = mirroredComparison(op)
	}
	if op == "" {
		return intervalBound{}, false
	}
	if _, _, _, subjectConstant := b.intervalBoundValue(subject); subjectConstant {
		return intervalBound{}, false
	}
	subjectType, typed := b.analyzer.expressionTypes[subject]
	if !typed || !isIntervalSubjectType(subjectType) {
		return intervalBound{}, false
	}
	if subjectType.Kind == CharType && !isCharByteBound(constant, value) {
		return intervalBound{}, false
	}
	identity, ok := b.expressionIdentity(subject)
	if !ok {
		return intervalBound{}, false
	}
	bound := intervalBound{Op: op, Spelling: spelling, Named: named, Subject: subject, Identity: identity, Comparison: comparison}
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

// isIntervalSubjectType accepts the totally ordered discrete scalars: integers
// by value, char (one byte) by byte value, and rune by Unicode scalar value.
func isIntervalSubjectType(typ Type) bool {
	return isIntegerType(typ) || typ.Kind == CharType || typ.Kind == RuneType
}

// isCharByteBound reports whether a bound has a proven byte value for a char
// subject. A char is one byte, so only an ASCII character literal (whose
// scalar equals its byte) or a value in 0..255 is a byte; a non-ASCII
// character literal names no single byte and yields no proof.
func isCharByteBound(bound ast.Expression, value *big.Int) bool {
	limit := int64(0xFF)
	if _, character := bound.(*ast.CharLiteral); character {
		limit = 0x7F
	}
	return value.Sign() >= 0 && value.Cmp(big.NewInt(limit)) <= 0
}

// intervalBoundValue accepts a literal bound or a named compile-time constant
// whose value Sema recorded for this comparison operand.
func (b *pitfallBuilder) intervalBoundValue(expression ast.Expression) (*big.Int, string, bool, bool) {
	if value, spelling, ok := intervalLiteral(expression); ok {
		return value, spelling, false, true
	}
	if value, ok := b.analyzer.comparisonConstantOperands[expression]; ok {
		return new(big.Int).Set(value), expression.String(), true, true
	}
	return nil, "", false, false
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

// intervalLiteral accepts an integer literal, optionally negated, or a
// character literal valued by its Unicode scalar; for a char subject only an
// ASCII scalar is also its byte value (see isCharByteBound).
func intervalLiteral(expression ast.Expression) (*big.Int, string, bool) {
	switch expression := expression.(type) {
	case *ast.CharLiteral:
		scalars := []rune(expression.Value)
		if len(scalars) != 1 {
			return nil, "", false
		}
		return big.NewInt(int64(scalars[0])), expression.Token.Lexeme, true
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

func intervalEvidence(lower, upper intervalBound) []PitfallEvidence {
	evidence := []PitfallEvidence{
		{Strength: PitfallEvidenceProof, Fact: "both comparisons test the same resolved, totally ordered value", Source: expressionToken(lower.Subject)},
		{Strength: PitfallEvidenceProof, Fact: "lower bound " + lower.Value.String() + " from " + lower.Comparison.String(), Source: lower.Comparison.Token},
		{Strength: PitfallEvidenceProof, Fact: "upper bound " + upper.Value.String() + " from " + upper.Comparison.String(), Source: upper.Comparison.Token},
	}
	for _, bound := range []intervalBound{lower, upper} {
		if bound.Named {
			evidence = append(evidence, PitfallEvidence{
				Strength: PitfallEvidenceProof,
				Fact:     bound.Spelling + " is a named compile-time constant",
				Source:   bound.Comparison.Token,
			})
		}
	}
	return evidence
}

// reportTautologicalInterval reports a chain proven true. suggestMembership is set only
// when the whole condition is the one bound pair, so the replacement covers
// exactly the written condition.
func (b *pitfallBuilder) reportTautologicalInterval(condition *ast.InfixExpression, lower, upper intervalBound, suggestMembership bool) {
	finding := PitfallFinding{
		Rule:           PitfallTautologicalInterval,
		Family:         PitfallBoundsAndRanges,
		Classification: PitfallLikelyMistake,
		Confidence:     PitfallConfidenceProven,
		Subject:        PitfallSubject{Expression: condition.String(), Source: condition.Token},
		EvidenceFor: append(intervalEvidence(lower, upper), PitfallEvidence{
			Strength: PitfallEvidenceProof,
			Fact:     "the union of both half-lines covers every value, so the condition is always true",
			Source:   condition.Token,
		}),
		OwningRule: "tautological-interval-condition",
	}
	// § "Tautological interval conditions": an inclusive interval written with
	// `||` most likely meant membership; prefer `in` over swapping operators.
	if suggestMembership && lower.Value.Cmp(upper.Value) <= 0 {
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

// reportRangeMembershipIdiom offers a canonical interval rewrite only with
// complete observable-equivalence evidence; effectful/unknown reads suppress it.
// Rules: rules/analysis/pitfall_analysis.md — "Canonical idiom guidance", "Fix safety".
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
			Fact:     "the conjunction is exactly membership in a non-empty constant range",
			Source:   condition.Token,
		}),
		OwningRule: "canonical-range-membership",
		Actions: []PitfallSuggestedAction{{
			Kind:        PitfallProvenFix,
			Safety:      b.membershipFixSafety(lower, upper, replacement),
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
			Fact:     "the subject lacks a proven non-observable stored-value read (getters, volatile/register access and unresolved reads are excluded)",
			Source:   expressionToken(lower.Subject),
		}}
		finding.State = PitfallStateSuppressed
		finding.Actions = nil
		finding.EvidenceAgainst = against
		finding.Suppression = &PitfallSuppression{
			Reason:   "the compared value may have observable effects or failures, so rewriting two evaluations into one is not proven semantics-preserving",
			Evidence: against,
		}
	}
	b.add(finding)
}
