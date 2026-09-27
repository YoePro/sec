package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// inspectNeighborIndexes recognizes the canonical zero-based half-open
// traversal whose first straight-line body statement indexes the same
// collection at i+1 or i-1. Restricting the initial slice avoids looking
// through guards that may prove the neighbor access safe.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — "Upper neighbor access"
//   - rules/analysis/pitfall_analysis.md — "Lower neighbor access"
//   - rules/analysis/pitfall_analysis.md — "Guards participate in pitfall reasoning"
func (b *pitfallBuilder) inspectNeighborIndexes(loop *ast.ForStatement) {
	rangeExpression, ok := loop.Iterable.(*ast.RangeExpression)
	if !ok || rangeExpression == nil || !rangeExpression.Exclusive || loop.Step != nil || loop.Body == nil || len(loop.Body.Statements) == 0 || len(loop.Bindings) != 1 || loop.Bindings[0].Discard {
		return
	}
	start, ok := b.analyzer.integerConstantValue(rangeExpression.Start)
	if !ok || start.Sign() != 0 {
		return
	}
	collectionIdentity, lengthToken, ok := b.lengthReceiver(rangeExpression.End)
	if !ok || !pitfallStraightLineStatement(loop.Body.Statements[0]) {
		return
	}

	binding := loop.Bindings[0].Token
	for _, index := range indexesInStatement(loop.Body.Statements[0]) {
		indexedCollection, sameCollection := b.expressionIdentity(index.Left)
		if !sameCollection || indexedCollection != collectionIdentity {
			continue
		}
		offset, ok := b.resolvedLoopBindingOffset(index.Index, binding)
		if !ok || (offset != 1 && offset != -1) {
			continue
		}
		b.reportNeighborIndex(index, rangeExpression, lengthToken, offset)
	}
}

// resolvedLoopBindingOffset normalizes i+1, 1+i, and i-1 using the resolved
// loop binding rather than identifier spelling.
//
// Rules: rules/analysis/pitfall_analysis.md — "Semantic correlation, not spelling heuristics".
func (b *pitfallBuilder) resolvedLoopBindingOffset(expression ast.Expression, binding lexer.Token) (int64, bool) {
	infix, ok := expression.(*ast.InfixExpression)
	if !ok {
		return 0, false
	}
	switch infix.Operator {
	case "+":
		if b.expressionUsesBinding(infix.Left, binding) && b.pitfallIntegerEquals(infix.Right, 1) {
			return 1, true
		}
		if b.expressionUsesBinding(infix.Right, binding) && b.pitfallIntegerEquals(infix.Left, 1) {
			return 1, true
		}
	case "-":
		if b.expressionUsesBinding(infix.Left, binding) && b.pitfallIntegerEquals(infix.Right, 1) {
			return -1, true
		}
	}
	return 0, false
}

// reportNeighborIndex records the exact violated endpoint and keeps correction
// advice heuristic because changing the traversal domain can change intent.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — "Upper neighbor access"
//   - rules/analysis/pitfall_analysis.md — "Lower neighbor access"
func (b *pitfallBuilder) reportNeighborIndex(index *ast.IndexExpression, domain *ast.RangeExpression, lengthToken lexer.Token, offset int64) {
	rule := PitfallUpperNeighborIndex
	evidence := "the half-open domain permits i == Len - 1, making i + 1 equal Len"
	action := "shorten the traversal domain or guard the upper neighbor"
	if offset < 0 {
		rule = PitfallLowerNeighborIndex
		evidence = "the zero-based domain permits i == 0, making i - 1 invalid"
		action = "start predecessor traversal at 1 or guard the lower neighbor"
	}
	b.add(PitfallFinding{
		Rule:           rule,
		Family:         PitfallBoundsAndRanges,
		Classification: PitfallProvenInvalid,
		Confidence:     PitfallConfidenceProven,
		Subject:        PitfallSubject{Expression: index.String(), Source: index.Token},
		EvidenceFor: []PitfallEvidence{
			{Strength: PitfallEvidenceProof, Fact: evidence, Source: domain.Token},
			{Strength: PitfallEvidenceProof, Fact: "the offset loop binding indexes the same collection whose Len bounds the domain", Source: lengthToken},
		},
		OwningRule: "bounds",
		Actions: []PitfallSuggestedAction{{
			Kind: PitfallSuggestedEdit, Title: action, Source: index.Token,
		}},
	})
}

// pitfallIntegerEquals uses Sema's canonical integer-constant facts, including
// explicit integer conversions and constant bindings.
//
// Rules: rules/analysis/pitfall_analysis.md — "Canonical facts consumed by pitfall analysis".
func (b *pitfallBuilder) pitfallIntegerEquals(expression ast.Expression, value int64) bool {
	constant, ok := b.analyzer.integerConstantValue(expression)
	return ok && constant.IsInt64() && constant.Int64() == value
}

// pitfallStraightLineStatement excludes nested control-flow constructs because
// their guards may remove the invalid endpoint before the indexed access.
//
// Rules: rules/analysis/pitfall_analysis.md — "Guards participate in pitfall reasoning".
func pitfallStraightLineStatement(statement ast.Statement) bool {
	switch statement.(type) {
	case *ast.IfStatement, *ast.SwitchStatement, *ast.SelectStatement, *ast.ForStatement, *ast.WhileStatement, *ast.MatchStatement:
		return false
	default:
		return true
	}
}
