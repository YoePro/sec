package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// capacityReceiver resolves `X.Capacity` through the compiler-known member
// fact and returns the collection identity and the Capacity token.
func (b *pitfallBuilder) capacityReceiver(expression ast.Expression) (string, lexer.Token, bool) {
	member, ok := expression.(*ast.MemberExpression)
	if !ok || member == nil || member.Property == nil {
		return "", lexer.Token{}, false
	}
	known, resolved := b.analyzer.compilerKnownMemberFacts[sourceTokenLocation(member.Property.Token)]
	if !resolved || known.Kind != CompilerKnownProperty || known.Name != "Capacity" {
		return "", lexer.Token{}, false
	}
	identity, ok := b.expressionIdentity(member.Object)
	return identity, member.Property.Token, ok
}

// inspectCapacityAsLength reports a range loop that ends at a collection's
// compiler-known Capacity and indexes that same collection with its loop
// binding: Capacity is the number of elements the collection can hold without
// growing, not the live length, so the traversal reaches non-live positions
// whenever Capacity exceeds Len. A proven `Len == Capacity` on the path — a
// dominating assertion or an established continuing branch —
// suppresses the finding, unless the loop body structurally mutates the
// collection and so invalidates the proof.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — "Length, capacity, and extent", "Capacity used as live length"
//   - rules/collections/collections.md — 13.4 List properties
func (b *pitfallBuilder) inspectCapacityAsLength(loop *ast.ForStatement) {
	rangeExpression, ok := loop.Iterable.(*ast.RangeExpression)
	if !ok || rangeExpression == nil || loop.Body == nil || len(loop.Bindings) != 1 || loop.Bindings[0].Discard {
		return
	}
	collection, capacityToken, ok := b.capacityReceiver(rangeExpression.End)
	if !ok {
		return
	}
	binding := loop.Bindings[0]
	var indexed *ast.IndexExpression
	for _, statement := range loop.Body.Statements {
		for _, index := range indexesInStatement(statement) {
			if identity, same := b.expressionIdentity(index.Left); same && identity == collection && b.expressionUsesBinding(index.Index, binding.Token) {
				indexed = index
				break
			}
		}
		if indexed != nil {
			break
		}
	}
	if indexed == nil {
		return
	}
	finding := PitfallFinding{
		Rule:           PitfallCapacityAsLength,
		Family:         PitfallBoundsAndRanges,
		Classification: PitfallLikelyMistake,
		Confidence:     PitfallConfidenceHigh,
		Subject:        PitfallSubject{Expression: indexed.String(), Source: indexed.Token},
		EvidenceFor: []PitfallEvidence{
			{Strength: PitfallEvidenceProof, Fact: "the range ends at Capacity, the number of elements the collection can hold without growing", Source: capacityToken},
			{Strength: PitfallEvidenceStrong, Fact: "the loop indexes the same collection by its binding, reaching non-live positions whenever Capacity exceeds Len", Source: indexed.Token},
		},
		OwningRule: "bounds",
		Actions: []PitfallSuggestedAction{{
			Kind: PitfallSuggestedEdit, Title: "traverse the live elements with Len", Replacement: "Len", Source: capacityToken,
		}},
	}
	if proof, proven := b.activeCapacityEqualities[collection]; proven {
		if len(b.indexedStructuralMutations(loop.Body, collection, binding, false)) == 0 {
			evidence := PitfallEvidence{Strength: PitfallEvidenceSuppressing, Fact: "Len == Capacity is proven on this path and the loop does not change the collection's structure", Source: proof}
			finding.State = PitfallStateSuppressed
			finding.EvidenceAgainst = []PitfallEvidence{evidence}
			finding.Suppression = &PitfallSuppression{Reason: evidence.Fact, Evidence: []PitfallEvidence{evidence}}
		}
	}
	b.add(finding)
}

// inspectDirectCapacityIndex reports a direct access `X[X.Capacity]` or
// `X[X.Capacity - k]` with a positive constant k. Because Len never exceeds
// Capacity, `X[X.Capacity]` always addresses a non-live position and is
// ProvenInvalid regardless of any equality proof. `X[X.Capacity - k]` is live
// only when Capacity - k < Len, an unproven equality assumption, and so a
// high-confidence LikelyMistake that a `Len == Capacity` proof on the path
// suppresses.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — "Capacity used as live length"
//   - rules/collections/collections.md — 13.4 List properties (Len never exceeds Capacity)
func (b *pitfallBuilder) inspectDirectCapacityIndex(index *ast.IndexExpression) {
	if index == nil {
		return
	}
	bound := index.Index
	offset := false
	if subtraction, ok := bound.(*ast.InfixExpression); ok && subtraction.Operator == "-" {
		constant, constantOK := b.analyzer.integerConstantValue(subtraction.Right)
		if !constantOK || constant.Sign() <= 0 {
			return
		}
		bound, offset = subtraction.Left, true
	}
	collection, capacityToken, ok := b.capacityReceiver(bound)
	if !ok {
		return
	}
	if indexed, same := b.expressionIdentity(index.Left); !same || indexed != collection {
		return
	}
	finding := PitfallFinding{
		Rule:           PitfallCapacityAsLength,
		Family:         PitfallBoundsAndRanges,
		Classification: PitfallProvenInvalid,
		Confidence:     PitfallConfidenceProven,
		Subject:        PitfallSubject{Expression: index.String(), Source: index.Token},
		EvidenceFor: []PitfallEvidence{
			{Strength: PitfallEvidenceProof, Fact: "the index is the Capacity of the same collection", Source: capacityToken},
			{Strength: PitfallEvidenceProof, Fact: "Len never exceeds Capacity, so Capacity is never a live zero-based index", Source: index.Token},
		},
		OwningRule: "bounds",
		Actions: []PitfallSuggestedAction{{
			Kind: PitfallSuggestedEdit, Title: "index relative to the live length Len", Replacement: "Len", Source: capacityToken,
		}},
	}
	if offset {
		finding.Classification = PitfallLikelyMistake
		finding.Confidence = PitfallConfidenceHigh
		finding.EvidenceFor = []PitfallEvidence{
			{Strength: PitfallEvidenceProof, Fact: "the index counts back from Capacity, the number of elements the collection can hold without growing", Source: capacityToken},
			{Strength: PitfallEvidenceStrong, Fact: "the position is live only when Capacity equals Len, which is not proven on this path", Source: index.Token},
		}
		if proof, proven := b.activeCapacityEqualities[collection]; proven {
			evidence := PitfallEvidence{Strength: PitfallEvidenceSuppressing, Fact: "Len == Capacity is proven on this path", Source: proof}
			finding.State = PitfallStateSuppressed
			finding.EvidenceAgainst = []PitfallEvidence{evidence}
			finding.Suppression = &PitfallSuppression{Reason: evidence.Fact, Evidence: []PitfallEvidence{evidence}}
		}
	}
	b.add(finding)
}
