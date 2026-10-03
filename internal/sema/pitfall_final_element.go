package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// inspectFinalElementAccess recognizes collection[collection.Len - 1] through
// resolved collection identity and canonical integer facts. A directly
// preceding empty-check that exits the current path is retained as suppression
// evidence; without it, the expression lacks the non-empty proof required by
// checked arithmetic and bounds semantics.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — "Final-element access requires non-empty proof"
//   - rules/analysis/pitfall_analysis.md — "Guards participate in pitfall reasoning"
func (b *pitfallBuilder) inspectFinalElementAccess(index *ast.IndexExpression) {
	if index == nil {
		return
	}
	subtraction, ok := index.Index.(*ast.InfixExpression)
	if !ok || subtraction.Operator != "-" || !b.pitfallIntegerEquals(subtraction.Right, 1) {
		return
	}
	lengthCollection, lengthToken, ok := b.lengthReceiver(subtraction.Left)
	if !ok {
		return
	}
	indexedCollection, ok := b.expressionIdentity(index.Left)
	if !ok || indexedCollection != lengthCollection {
		return
	}

	finding := PitfallFinding{
		Rule:           PitfallFinalElementNeedsNonEmpty,
		Family:         PitfallBoundsAndRanges,
		Classification: PitfallLikelyMistake,
		Confidence:     PitfallConfidenceHigh,
		Subject:        PitfallSubject{Expression: index.String(), Source: index.Token},
		EvidenceFor: []PitfallEvidence{
			{Strength: PitfallEvidenceProof, Fact: "the index subtracts one from the Len of the same collection", Source: lengthToken},
			{Strength: PitfallEvidenceStrong, Fact: "no non-empty proof dominates this final-element access", Source: index.Token},
		},
		OwningRule: "checked-arithmetic-and-bounds",
		Actions: []PitfallSuggestedAction{{
			Kind: PitfallSuggestedEdit, Title: "prove the collection is non-empty before indexing", Source: index.Token,
		}},
	}
	if proof, proven := b.activeNonEmptyProofs[lengthCollection]; proven {
		evidence := PitfallEvidence{Strength: PitfallEvidenceSuppressing, Fact: "control flow proves the collection is non-empty before the final-element access", Source: proof}
		finding.State = PitfallStateSuppressed
		finding.EvidenceAgainst = []PitfallEvidence{evidence}
		finding.Suppression = &PitfallSuppression{
			Reason:   "the continuing path proves the collection is non-empty",
			Evidence: []PitfallEvidence{evidence},
		}
	}
	b.add(finding)
}

// walkIfStatement applies a condition-derived non-empty fact only to the first
// straight-line statement of the branch where that fact holds. This is a
// deliberately mutation-safe initial slice: later statements must re-establish
// the fact until the general path-sensitive collection-state analysis exists.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — "Guards participate in pitfall reasoning"
//   - rules/analysis/pitfall_analysis.md — "Final-element access requires non-empty proof"
func (b *pitfallBuilder) walkIfStatement(statement *ast.IfStatement) {
	if statement == nil {
		return
	}
	b.inspectIneffectiveUpperBoundsGuard(statement)
	b.walkExpression(statement.Condition)

	guards := b.strictIndexGuards(statement.Condition)
	b.inspectWrongGuardSubject(statement, guards)

	outerProofs := b.activeNonEmptyProofs
	b.activeNonEmptyProofs = b.nonEmptyBranchProof(statement.Condition, true)
	b.withIndexGuards(guards, func() { b.walkBlock(statement.Consequence) })
	b.activeNonEmptyProofs = b.nonEmptyBranchProof(statement.Condition, false)
	b.walkBlock(statement.Alternative)
	b.activeNonEmptyProofs = outerProofs
}

// nonEmptyBranchProof normalizes direct comparisons between compiler-known Len
// and zero, returning a proof only for the branch where Len must be non-zero.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — "Canonical facts consumed by pitfall analysis"
//   - rules/analysis/pitfall_analysis.md — "Guards participate in pitfall reasoning"
func (b *pitfallBuilder) nonEmptyBranchProof(condition ast.Expression, truthBranch bool) map[string]lexer.Token {
	if collection, token, emptyWhenTrue, ok := b.isEmptyCondition(condition); ok {
		if truthBranch == emptyWhenTrue {
			return nil
		}
		return map[string]lexer.Token{collection: token}
	}

	comparison, ok := condition.(*ast.InfixExpression)
	if !ok {
		return nil
	}

	var collection string
	var token lexer.Token
	var matched bool
	switch comparison.Operator {
	case "!=", "==":
		collection, token, matched = b.zeroLengthComparison(comparison.Left, comparison.Right)
		if !matched {
			collection, token, matched = b.zeroLengthComparison(comparison.Right, comparison.Left)
		}
		if matched && truthBranch != (comparison.Operator == "!=") {
			matched = false
		}
	case ">":
		collection, token, matched = b.zeroLengthComparison(comparison.Left, comparison.Right)
		if !truthBranch {
			matched = false
		}
	case "<":
		collection, token, matched = b.zeroLengthComparison(comparison.Right, comparison.Left)
		if !truthBranch {
			matched = false
		}
	case "<=":
		collection, token, matched = b.zeroLengthComparison(comparison.Left, comparison.Right)
		if truthBranch {
			matched = false
		}
	case ">=":
		collection, token, matched = b.zeroLengthComparison(comparison.Right, comparison.Left)
		if truthBranch {
			matched = false
		}
	}
	if !matched {
		return nil
	}
	return map[string]lexer.Token{collection: token}
}

// emptyCollectionExitGuard recognizes the canonical dominating proof
// `if collection.Len == 0 { return }`. Restricting the proof to an unconditional
// exit and resolved Len receiver prevents spelling-based correlation.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — "Guards participate in pitfall reasoning"
//   - rules/analysis/pitfall_analysis.md — "Final-element access requires non-empty proof"
func (b *pitfallBuilder) emptyCollectionExitGuard(statement ast.Statement) (string, lexer.Token, bool) {
	conditional, ok := statement.(*ast.IfStatement)
	if !ok || conditional.Alternative != nil || !pitfallBlockDefinitelyExits(conditional.Consequence) {
		return "", lexer.Token{}, false
	}
	if collection, token, emptyWhenTrue, ok := b.isEmptyCondition(conditional.Condition); ok && emptyWhenTrue {
		return collection, token, true
	}
	comparison, ok := conditional.Condition.(*ast.InfixExpression)
	if !ok || comparison.Operator != "==" {
		return "", lexer.Token{}, false
	}
	if collection, token, ok := b.zeroLengthComparison(comparison.Left, comparison.Right); ok {
		return collection, token, true
	}
	return b.zeroLengthComparison(comparison.Right, comparison.Left)
}

// isEmptyCondition resolves compiler-known IsEmpty and its direct boolean
// negation. The returned boolean states whether the collection is empty on the
// condition's true branch.
//
// Rules:
//   - rules/compiler/compiler_known_members.md — "IsEmpty"
//   - rules/analysis/pitfall_analysis.md — "Guards participate in pitfall reasoning"
func (b *pitfallBuilder) isEmptyCondition(condition ast.Expression) (string, lexer.Token, bool, bool) {
	emptyWhenTrue := true
	if prefix, ok := condition.(*ast.PrefixExpression); ok && prefix.Operator == "!" {
		condition = prefix.Right
		emptyWhenTrue = false
	}
	member, ok := condition.(*ast.MemberExpression)
	if !ok || member.Property == nil {
		return "", lexer.Token{}, false, false
	}
	known, resolved := b.analyzer.compilerKnownMemberFacts[sourceTokenLocation(member.Property.Token)]
	if !resolved || known.Kind != CompilerKnownProperty || known.Name != "IsEmpty" {
		return "", lexer.Token{}, false, false
	}
	identity, ok := b.expressionIdentity(member.Object)
	return identity, member.Property.Token, emptyWhenTrue, ok
}

// zeroLengthComparison correlates a compiler-known Len receiver with Sema's
// canonical integer-zero fact.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — "Semantic correlation, not spelling heuristics"
//   - rules/analysis/pitfall_analysis.md — "Canonical facts consumed by pitfall analysis"
func (b *pitfallBuilder) zeroLengthComparison(length ast.Expression, zero ast.Expression) (string, lexer.Token, bool) {
	if !b.pitfallIntegerEquals(zero, 0) {
		return "", lexer.Token{}, false
	}
	return b.lengthReceiver(length)
}
