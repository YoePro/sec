package sema

import (
	"math/big"
	"strconv"
	"strings"

	"sec/internal/ast"
)

var bigOne = big.NewInt(1)

// PitfallPredecessorNeedsPositiveIndex: values[i - 1] without a proof that
// i >= 1 (pitfall_analysis.md § 25.7(6)).
const PitfallPredecessorNeedsPositiveIndex PitfallRuleID = "pitfall.bounds.predecessor-needs-positive-index"

// positiveIndexCollection marks a pitfallIndexGuard that proves its index is
// at least one rather than below a collection's Len.
const positiveIndexCollection = "<positive-index>"

// positiveIndexGuards collects the facts a condition proves about an index
// binding being at least one on its true (holds) or false branch:
// `i > 0`, `i >= 1`, `0 < i`, `1 <= i`, and `i != 0` for an unsigned index.
// A false branch negates one comparison or a disjunction of comparisons.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — § 25.7(6) predecessor access, "Guards participate in pitfall reasoning"
func (b *pitfallBuilder) positiveIndexGuards(condition ast.Expression, holds bool) []pitfallIndexGuard {
	comparison, ok := condition.(*ast.InfixExpression)
	if !ok || comparison == nil {
		return nil
	}
	switch comparison.Operator {
	case "&&":
		if holds {
			return append(b.positiveIndexGuards(comparison.Left, true), b.positiveIndexGuards(comparison.Right, true)...)
		}
		return nil
	case "||":
		if !holds {
			return append(b.positiveIndexGuards(comparison.Left, false), b.positiveIndexGuards(comparison.Right, false)...)
		}
		return nil
	}
	operator := comparison.Operator
	if !holds {
		operator = map[string]string{"==": "!=", "!=": "==", "<": ">=", "<=": ">", ">": "<=", ">=": "<"}[operator]
	}
	left, right := comparison.Left, comparison.Right
	// Normalize to `index OP constant`.
	if _, constant := b.analyzer.integerConstantValue(left); constant {
		left, right = right, left
		operator = map[string]string{"<": ">", "<=": ">=", ">": "<", ">=": "<=", "==": "==", "!=": "!="}[operator]
	}
	bound, constant := b.analyzer.integerConstantValue(right)
	if !constant {
		return nil
	}
	identity, ok := b.positiveIndexIdentity(left)
	if !ok {
		return nil
	}
	positive := false
	switch operator {
	case ">":
		positive = bound.Sign() >= 0
	case ">=":
		positive = bound.Cmp(bigOne) >= 0
	case "==":
		positive = bound.Cmp(bigOne) >= 0
	case "!=":
		positive = bound.Sign() == 0 && b.unsignedIndex(left)
	}
	if !positive {
		return nil
	}
	return []pitfallIndexGuard{{indexIdentity: identity, collectionIdentity: positiveIndexCollection, source: comparison.Token}}
}

func (b *pitfallBuilder) positiveIndexIdentity(expression ast.Expression) (string, bool) {
	if _, identifier := expression.(*ast.Identifier); !identifier {
		return "", false
	}
	return b.expressionIdentity(expression)
}

func (b *pitfallBuilder) unsignedIndex(expression ast.Expression) bool {
	typ, ok := b.analyzer.ResolvedTypeOf(expression)
	return ok && typ.Kind == UintType
}

// loopPositiveIndexGuard proves a range loop binding at least one from the
// smallest value its range can produce: the start of an ascending range, the
// inclusive end of a descending `start..end step -k` range, or one past the
// exclusive end of a descending `start..<end step -k` range (flowcontrol_for.md
// §§ 25–27: descending iteration is always an explicit negative step).
func (b *pitfallBuilder) loopPositiveIndexGuard(loop *ast.ForStatement) (pitfallIndexGuard, bool) {
	rangeExpression, ok := loop.Iterable.(*ast.RangeExpression)
	if !ok || rangeExpression == nil || len(loop.Bindings) != 1 || loop.Bindings[0].Discard {
		return pitfallIndexGuard{}, false
	}
	descending := false
	if loop.Step != nil {
		step, constant := b.analyzer.integerConstantValue(loop.Step)
		if !constant || step.Sign() == 0 {
			return pitfallIndexGuard{}, false
		}
		descending = step.Sign() < 0
	}
	positive := false
	if descending {
		end, constant := b.analyzer.integerConstantValue(rangeExpression.End)
		if constant {
			positive = rangeExpression.Exclusive && end.Sign() >= 0 || !rangeExpression.Exclusive && end.Cmp(bigOne) >= 0
		}
	} else if start, constant := b.analyzer.integerConstantValue(rangeExpression.Start); constant {
		positive = start.Cmp(bigOne) >= 0
	}
	if !positive {
		return pitfallIndexGuard{}, false
	}
	binding := loop.Bindings[0].Token
	identity := strings.Join([]string{binding.File, strconv.Itoa(binding.Line), strconv.Itoa(binding.Column)}, ":")
	return pitfallIndexGuard{indexIdentity: identity, collectionIdentity: positiveIndexCollection, source: rangeExpression.Token}, true
}

// continuationPositiveIndexGuards keeps a positivity fact after an if whose
// other branch cannot continue (`if i == 0 { return }`).
func (b *pitfallBuilder) continuationPositiveIndexGuards(statement ast.Statement) []pitfallIndexGuard {
	conditional, ok := statement.(*ast.IfStatement)
	if !ok || conditional.OptionBinding != nil {
		return nil
	}
	var result []pitfallIndexGuard
	if !b.analyzer.blockCanFallThrough(conditional.Consequence) {
		for _, guard := range b.positiveIndexGuards(conditional.Condition, false) {
			if conditional.Alternative == nil || b.guardSurvivesStatement(guard, &ast.UnsafeStatement{Body: conditional.Alternative}) {
				result = append(result, guard)
			}
		}
	}
	if conditional.Alternative != nil && !b.analyzer.blockCanFallThrough(conditional.Alternative) {
		for _, guard := range b.positiveIndexGuards(conditional.Condition, true) {
			if b.guardSurvivesStatement(guard, &ast.UnsafeStatement{Body: conditional.Consequence}) {
				result = append(result, guard)
			}
		}
	}
	return result
}

func (b *pitfallBuilder) hasPositiveIndexGuard(identity string) bool {
	for _, guard := range b.activePositiveIndexes {
		if guard.indexIdentity == identity {
			return true
		}
	}
	return false
}

// inspectPredecessorAccess reports `values[i - 1]` when no dominating guard,
// loop domain, constant or type contract proves i at least one, and explains
// the hazard: checked unsigned underflow before indexing for an unsigned i,
// or index -1, a bounds violation, for a signed i. The final-element form
// `values[values.Len - 1]` and zero-based loop neighbors have their own
// findings.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — § 25.7(5)–(6)
func (b *pitfallBuilder) inspectPredecessorAccess(index *ast.IndexExpression) {
	if index == nil {
		return
	}
	subtraction, ok := index.Index.(*ast.InfixExpression)
	if !ok || subtraction.Operator != "-" || !b.pitfallIntegerEquals(subtraction.Right, 1) {
		return
	}
	if _, _, length := b.lengthReceiver(subtraction.Left); length {
		return
	}
	identity, ok := b.positiveIndexIdentity(subtraction.Left)
	if !ok || b.hasPositiveIndexGuard(identity) || b.reportedLowerNeighbor(index) {
		return
	}
	if value, constant := b.analyzer.integerConstantValue(subtraction.Left); constant && value.Cmp(bigOne) >= 0 {
		return
	}
	if typ, resolved := b.analyzer.ResolvedTypeOf(subtraction.Left); resolved && typ.MinInteger != nil && typ.MinInteger.Cmp(bigOne) >= 0 {
		return
	}
	name := subtraction.Left.String()
	hazard := "for " + name + " == 0 the index " + name + " - 1 is -1, a bounds violation"
	if b.unsignedIndex(subtraction.Left) {
		hazard = "for " + name + " == 0 the subtraction " + name + " - 1 is checked unsigned underflow and fails before any indexing"
	}
	b.add(PitfallFinding{
		Rule:           PitfallPredecessorNeedsPositiveIndex,
		Family:         PitfallBoundsAndRanges,
		Classification: PitfallLikelyMistake,
		Confidence:     PitfallConfidenceHigh,
		Subject:        PitfallSubject{Expression: index.String(), Source: index.Token},
		EvidenceFor: []PitfallEvidence{
			{Strength: PitfallEvidenceProof, Fact: "the index subtracts one from " + subtraction.Left.String(), Source: subtraction.Token},
			{Strength: PitfallEvidenceStrong, Fact: "no guard, loop domain, constant or contract proves " + name + " >= 1 here; " + hazard, Source: index.Token},
		},
		OwningRule: "checked-arithmetic-and-bounds",
		Actions: []PitfallSuggestedAction{{
			Kind: PitfallSuggestedEdit, Title: "guard the predecessor with `if " + subtraction.Left.String() + " > 0` or start the traversal at 1", Source: index.Token,
		}},
	})
}

// reportedLowerNeighbor reports an access the proven lower-neighbor finding
// already owns (diagnostic ownership: no overlapping report).
func (b *pitfallBuilder) reportedLowerNeighbor(index *ast.IndexExpression) bool {
	for _, finding := range b.result.results {
		if finding.Rule == PitfallLowerNeighborIndex && finding.Subject.Source == index.Token {
			return true
		}
	}
	return false
}
