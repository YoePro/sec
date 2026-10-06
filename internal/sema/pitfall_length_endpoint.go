package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// lengthMinusOne resolves `X.Len - 1` and returns the collection identity,
// the Len member expression, and the Len token.
func (b *pitfallBuilder) lengthMinusOne(expression ast.Expression) (string, ast.Expression, lexer.Token, bool) {
	subtraction, ok := expression.(*ast.InfixExpression)
	if !ok || subtraction == nil || subtraction.Operator != "-" || !b.pitfallIntegerEquals(subtraction.Right, 1) {
		return "", nil, lexer.Token{}, false
	}
	collection, token, ok := b.lengthReceiver(subtraction.Left)
	return collection, subtraction.Left, token, ok
}

// inspectLengthEndpointIntent covers the two `Len - 1` range endings of
// rules/analysis/pitfall_analysis.md:
//
//   - "Omitted-last-element advisory": a zero-based half-open loop that ends
//     at `X.Len - 1` and indexes X with its binding provably skips the final
//     element. Neighbor access `X[i + 1]` (pairwise traversal) or a separate
//     access to `X[X.Len - 1]` in the same block is intent evidence that
//     suppresses it.
//   - "Avoiding fragile `0..len - 1` workarounds": an inclusive loop ending at
//     `X.Len - 1` underflows on an empty collection. With a stable non-empty proof on
//     the path it is equivalent to the canonical `start..<X.Len`, offered as a
//     proven fix; without one it is a likely mistake whose suggested edit
//     changes behavior for the empty collection.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — "Omitted-last-element advisory", "Avoiding fragile `0..len - 1` workarounds", "Strong advisory scope"
func (b *pitfallBuilder) inspectLengthEndpointIntent(loop *ast.ForStatement, block []ast.Statement, position int, nonEmpty map[string]lexer.Token) {
	rangeExpression, ok := loop.Iterable.(*ast.RangeExpression)
	if !ok || rangeExpression == nil || loop.Step != nil || loop.Body == nil {
		return
	}
	collection, lengthExpression, lengthToken, ok := b.lengthMinusOne(rangeExpression.End)
	if !ok {
		return
	}
	if rangeExpression.Exclusive {
		b.inspectOmittedLastElement(loop, rangeExpression, collection, lengthToken, block, position)
		return
	}
	finding := PitfallFinding{
		Rule:   PitfallFragileInclusiveLength,
		Family: PitfallBoundsAndRanges,
		Subject: PitfallSubject{
			Expression: rangeExpression.String(),
			Source:     rangeExpression.Token,
		},
		EvidenceFor: []PitfallEvidence{
			{Strength: PitfallEvidenceProof, Fact: "the inclusive range ends at Len - 1 of the same collection", Source: lengthToken},
		},
		OwningRule: "range-and-collection-intent",
	}
	canonical := "..<" + lengthExpression.String()
	if rangeExpression.Start != nil {
		canonical = rangeExpression.Start.String() + canonical
	}
	if proof, proven := nonEmpty[collection]; proven {
		finding.Classification = PitfallSuspiciousIntent
		finding.Confidence = PitfallConfidenceProven
		finding.EvidenceFor = append(finding.EvidenceFor, PitfallEvidence{Strength: PitfallEvidenceProof, Fact: "control flow proves the collection is non-empty on entry to the loop", Source: proof})
		finding.Actions = []PitfallSuggestedAction{{Kind: PitfallProvenFix, Safety: b.lengthEndpointFixSafety(rangeExpression, lengthExpression, canonical), Title: "use the canonical half-open traversal", Replacement: canonical, Source: rangeExpression.Token}}
		if !finding.Actions[0].Safety.verifiedFor(finding.Rule, canonical) {
			finding.Confidence = PitfallConfidenceHigh
			finding.EvidenceAgainst = append(finding.EvidenceAgainst, PitfallEvidence{Strength: PitfallEvidenceContradicting, Fact: "start or Len receiver evaluation may invalidate the entry proof or have observable effects; equivalence is unproven", Source: rangeExpression.Token})
		}
	} else {
		finding.Classification = PitfallLikelyMistake
		finding.Confidence = PitfallConfidenceHigh
		finding.EvidenceFor = append(finding.EvidenceFor, PitfallEvidence{Strength: PitfallEvidenceStrong, Fact: "no non-empty proof dominates the loop, so Len - 1 underflows for an empty collection", Source: rangeExpression.Token})
		finding.Actions = []PitfallSuggestedAction{{Kind: PitfallSuggestedEdit, Title: "use the half-open traversal, which is empty for an empty collection", Replacement: canonical, Source: rangeExpression.Token}}
	}
	b.add(finding)
}

// inspectOmittedLastElement recommends the canonical full half-open traversal
// when source facts show an unexplained shortened domain. This is an intent edit,
// not an equivalence proof, and explicit neighbor/final handling suppresses it.
// Rules: rules/analysis/pitfall_analysis.md — "Omitted-last-element advisory",
// "Canonical idiom guidance", "Fix safety".
func (b *pitfallBuilder) inspectOmittedLastElement(loop *ast.ForStatement, rangeExpression *ast.RangeExpression, collection string, lengthToken lexer.Token, block []ast.Statement, position int) {
	if len(loop.Bindings) != 1 || loop.Bindings[0].Discard || !b.pitfallIntegerEquals(rangeExpression.Start, 0) {
		return
	}
	binding := loop.Bindings[0].Token
	var direct, neighbor *ast.IndexExpression
	for _, statement := range loop.Body.Statements {
		for _, index := range indexesInStatement(statement) {
			identity, same := b.expressionIdentity(index.Left)
			if !same || identity != collection {
				continue
			}
			if b.expressionUsesBinding(index.Index, binding) && direct == nil {
				direct = index
			}
			if offset, resolved := b.resolvedLoopBindingOffset(index.Index, binding); resolved && offset == 1 && neighbor == nil {
				neighbor = index
			}
		}
	}
	if direct == nil {
		return
	}
	_, length, _, resolved := b.lengthMinusOne(rangeExpression.End)
	if !resolved {
		return
	}
	finding := PitfallFinding{
		Rule:           PitfallOmittedLastElement,
		Family:         PitfallBoundsAndRanges,
		Classification: PitfallLikelyMistake,
		Confidence:     PitfallConfidenceHigh,
		Subject:        PitfallSubject{Expression: direct.String(), Source: direct.Token},
		EvidenceFor: []PitfallEvidence{
			{Strength: PitfallEvidenceProof, Fact: "the zero-based half-open traversal stops before Len - 1, the final element", Source: lengthToken},
			{Strength: PitfallEvidenceProof, Fact: "the same collection is indexed directly by the loop binding", Source: direct.Token},
		},
		OwningRule: "range-and-collection-intent",
		Actions: []PitfallSuggestedAction{{
			Kind: PitfallSuggestedEdit, Title: "traverse every element with ..<Len",
			Replacement: rangeExpression.Start.String() + "..<" + length.String(), Source: rangeExpression.Token,
		}},
	}
	suppress := func(reason string, source lexer.Token) {
		evidence := PitfallEvidence{Strength: PitfallEvidenceSuppressing, Fact: reason, Source: source}
		finding.State = PitfallStateSuppressed
		finding.EvidenceAgainst = []PitfallEvidence{evidence}
		finding.Suppression = &PitfallSuppression{Reason: reason, Evidence: []PitfallEvidence{evidence}}
	}
	if neighbor != nil {
		suppress("a successor access explains the shortened pairwise traversal", neighbor.Token)
	} else if final := b.separateFinalElementAccess(block, position, collection); final != nil {
		suppress("the final element is handled separately in the same block", final.Token)
	}
	b.add(finding)
}

// separateFinalElementAccess finds `X[X.Len - 1]` in another straight-line
// statement of the loop's block, the evidence that the final element is
// processed on its own.
func (b *pitfallBuilder) separateFinalElementAccess(block []ast.Statement, position int, collection string) *ast.IndexExpression {
	for index, statement := range block {
		if index == position || !pitfallStraightLineStatement(statement) {
			continue
		}
		for _, access := range indexesInStatement(statement) {
			identity, same := b.expressionIdentity(access.Left)
			if !same || identity != collection {
				continue
			}
			if indexed, _, _, ok := b.lengthMinusOne(access.Index); ok && indexed == collection {
				return access
			}
		}
	}
	return nil
}
