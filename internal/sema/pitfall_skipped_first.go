package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// inspectSkippedFirstElement recognizes the canonical start-at-one traversal
// over a collection's Len. It reports only when the same resolved induction
// binding directly indexes that collection. A predecessor access in the loop
// or a dominating direct access to element zero suppresses the advisory.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — "Skipped-first-element advisory"
//   - rules/analysis/pitfall_analysis.md — "Evidence against a finding"
//   - rules/analysis/pitfall_analysis.md — "Suppressing evidence"
func (b *pitfallBuilder) inspectSkippedFirstElement(loop *ast.ForStatement, preceding []ast.Statement) {
	rangeExpression, ok := loop.Iterable.(*ast.RangeExpression)
	if !ok || rangeExpression == nil || !rangeExpression.Exclusive || loop.Step != nil || loop.Body == nil || len(loop.Bindings) != 1 || loop.Bindings[0].Discard {
		return
	}
	if !b.pitfallIntegerEquals(rangeExpression.Start, 1) {
		return
	}
	collection, lengthToken, ok := b.lengthReceiver(rangeExpression.End)
	if !ok {
		return
	}

	binding := loop.Bindings[0].Token
	var direct *ast.IndexExpression
	var predecessor *ast.IndexExpression
	for _, statement := range loop.Body.Statements {
		for _, index := range indexesInStatement(statement) {
			indexedCollection, sameCollection := b.expressionIdentity(index.Left)
			if !sameCollection || indexedCollection != collection {
				continue
			}
			if b.expressionUsesBinding(index.Index, binding) && direct == nil {
				direct = index
			}
			if offset, resolved := b.resolvedLoopBindingOffset(index.Index, binding); resolved && offset == -1 && predecessor == nil {
				predecessor = index
			}
		}
	}
	if direct == nil {
		return
	}

	finding := PitfallFinding{
		Rule:           PitfallSkippedFirstElement,
		Family:         PitfallBoundsAndRanges,
		Classification: PitfallLikelyMistake,
		Confidence:     PitfallConfidenceHigh,
		Subject:        PitfallSubject{Expression: direct.String(), Source: direct.Token},
		EvidenceFor: []PitfallEvidence{
			{Strength: PitfallEvidenceProof, Fact: "the half-open traversal starts at index 1", Source: rangeExpression.Token},
			{Strength: PitfallEvidenceProof, Fact: "the same collection supplies Len and is indexed directly by the loop binding", Source: lengthToken},
		},
		OwningRule: "range-and-collection-intent",
		Actions: []PitfallSuggestedAction{{
			Kind: PitfallSuggestedEdit, Title: "start complete traversal at index 0", Source: rangeExpression.Token,
		}},
	}

	if predecessor != nil {
		b.suppressSkippedFirstElement(&finding, "a predecessor access explains the start-at-one traversal", predecessor.Token)
	} else if zero := b.dominatingZeroElementAccess(preceding, collection); zero != nil {
		b.suppressSkippedFirstElement(&finding, "element zero is handled on the preceding dominating path", zero.Token)
	}
	b.add(finding)
}

func (b *pitfallBuilder) suppressSkippedFirstElement(finding *PitfallFinding, reason string, source lexer.Token) {
	evidence := PitfallEvidence{Strength: PitfallEvidenceSuppressing, Fact: reason, Source: source}
	finding.State = PitfallStateSuppressed
	finding.EvidenceAgainst = []PitfallEvidence{evidence}
	finding.Suppression = &PitfallSuppression{Reason: reason, Evidence: []PitfallEvidence{evidence}}
}

// dominatingZeroElementAccess deliberately accepts only straight-line
// statements in the same block. An access nested in a conditional does not
// establish that element zero is handled on every path reaching the loop.
func (b *pitfallBuilder) dominatingZeroElementAccess(preceding []ast.Statement, collection string) *ast.IndexExpression {
	for _, statement := range preceding {
		if !pitfallStraightLineStatement(statement) {
			continue
		}
		for _, index := range indexesInStatement(statement) {
			indexedCollection, sameCollection := b.expressionIdentity(index.Left)
			if sameCollection && indexedCollection == collection && b.pitfallIntegerEquals(index.Index, 0) {
				return index
			}
		}
	}
	return nil
}
