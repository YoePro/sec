package sema

import (
	"math/big"

	"sec/internal/ast"
)

// inspectMeaninglessComparison reports a comparison of a non-constant integer
// value against an integer literal whose result the value's type already
// decides: the representable range of the type narrowed by its range
// contracts. Examples are `values.Len < 0` (Len is unsigned) or
// `percent > 100` for a type constrained to 0..100. Path facts are not used;
// conditions decided by dominating conditions belong to the S3001
// unreachable-code rules, which stay primary. The finding is optional
// information, so it runs only at Deep depth.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — "Meaningless comparisons from proven ranges"
//   - rules/analysis/pitfall_analysis.md — "Interactive, Standard, and Deep analysis"
func (b *pitfallBuilder) inspectMeaninglessComparison(comparison *ast.InfixExpression) {
	if comparison == nil {
		return
	}
	operator := comparison.Operator
	subject, literal := comparison.Left, comparison.Right
	value, spelling, ok := intervalLiteral(literal)
	if !ok {
		subject, literal = comparison.Right, comparison.Left
		value, spelling, ok = intervalLiteral(literal)
		if !ok {
			return
		}
		if operator != "==" && operator != "!=" {
			operator = mirroredComparison(operator)
		}
	}
	if _, _, literalSubject := intervalLiteral(subject); literalSubject {
		return
	}
	if _, constant := b.analyzer.constantConditionIntegerValue(subject); constant {
		return
	}
	subjectType, typed := b.analyzer.expressionTypes[subject]
	if !typed || !isIntegerType(subjectType) {
		return
	}
	minimum, maximum, ok := integerTypeInterval(subjectType)
	if !ok {
		return
	}
	result, decided := decidedComparison(operator, value, minimum, maximum)
	if !decided {
		return
	}
	outcome := "false"
	if result {
		outcome = "true"
	}
	b.add(PitfallFinding{
		Rule:           PitfallMeaninglessComparison,
		Family:         PitfallBoundsAndRanges,
		Classification: PitfallSuspiciousIntent,
		Confidence:     PitfallConfidenceProven,
		Subject:        PitfallSubject{Expression: comparison.String(), Source: comparison.Token},
		EvidenceFor: []PitfallEvidence{
			{Strength: PitfallEvidenceProof, Fact: "every value of " + typeDisplayName(subjectType) + " lies in " + minimum.String() + ".." + maximum.String(), Source: expressionToken(subject)},
			{Strength: PitfallEvidenceProof, Fact: "so comparing it with " + spelling + " is always " + outcome, Source: comparison.Token},
		},
		OwningRule: "meaningless-comparison",
	})
}

// decidedComparison evaluates `subject operator value` for every subject in
// [minimum, maximum], reporting the result when it is the same for all.
func decidedComparison(operator string, value *big.Int, minimum *big.Int, maximum *big.Int) (bool, bool) {
	below := value.Cmp(minimum) < 0 // value < every subject
	above := value.Cmp(maximum) > 0 // value > every subject
	switch operator {
	case "<":
		if maximum.Cmp(value) < 0 {
			return true, true
		}
		if minimum.Cmp(value) >= 0 {
			return false, true
		}
	case "<=":
		if maximum.Cmp(value) <= 0 {
			return true, true
		}
		if minimum.Cmp(value) > 0 {
			return false, true
		}
	case ">":
		if minimum.Cmp(value) > 0 {
			return true, true
		}
		if maximum.Cmp(value) <= 0 {
			return false, true
		}
	case ">=":
		if minimum.Cmp(value) >= 0 {
			return true, true
		}
		if maximum.Cmp(value) < 0 {
			return false, true
		}
	case "==":
		if below || above {
			return false, true
		}
		if minimum.Cmp(maximum) == 0 {
			return true, true
		}
	case "!=":
		if below || above {
			return true, true
		}
		if minimum.Cmp(maximum) == 0 {
			return false, true
		}
	}
	return false, false
}
