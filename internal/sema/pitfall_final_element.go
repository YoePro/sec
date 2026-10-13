package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// inspectFinalElementAccess recognizes collection[collection.Len - 1] through
// resolved collection identity and canonical integer facts. Resolved type
// contracts and mutation-aware dominating flow facts supply suppression
// evidence. Without a supported proof, the requirement remains unproven.
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
	proof, proven := b.activeNonEmptyProofs[lengthCollection]
	proofReason := "control flow proves the collection is non-empty before the final-element access"
	if !proven && b.pitfallPureOperand(index.Left) {
		if typ, resolved := b.analyzer.ResolvedTypeOf(index.Left); resolved {
			proof, proven = nonEmptyCollectionTypeProof(dereferenceType(typ), lengthToken)
			if proven {
				proofReason = "the resolved collection type proves a positive minimum length"
			}
		}
	}
	if proven {
		evidence := PitfallEvidence{Strength: PitfallEvidenceSuppressing, Fact: proofReason, Source: proof}
		finding.State = PitfallStateSuppressed
		finding.EvidenceAgainst = []PitfallEvidence{evidence}
		finding.Suppression = &PitfallSuppression{
			Reason:   proofReason,
			Evidence: []PitfallEvidence{evidence},
		}
	}
	b.add(finding)
}

// walkIfStatement carries inherited and condition-derived non-empty facts
// into reachable branches. walkBlock invalidates them on possible length changes.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — "Guards participate in pitfall reasoning"
//   - rules/analysis/pitfall_analysis.md — "Final-element access requires non-empty proof", "Reachability"
//   - rules/control-flow/flowcontrol_if.md — §27 "Sema and flow-analysis requirements"
func (b *pitfallBuilder) walkIfStatement(statement *ast.IfStatement) {
	if statement == nil {
		return
	}
	flow, resolved := b.analyzer.ResolvedIfFlowOf(statement)
	trueReachable := !resolved || flow.TruePathExecution != ResolvedIfPathNever
	falseReachable := !resolved || flow.FalsePathExecution != ResolvedIfPathNever
	if b.boundsOnly {
		b.walkExpression(statement.Condition)
		if trueReachable {
			b.walkBlock(statement.Consequence)
		}
		if falseReachable {
			b.walkBlock(statement.Alternative)
		}
		return
	}
	if trueReachable {
		b.inspectIneffectiveUpperBoundsGuard(statement)
	}
	b.walkExpression(statement.Condition)

	guards := b.conditionIndexGuards(statement.Condition)
	if trueReachable {
		b.inspectWrongGuardSubject(statement, guards)
	}

	outerProofs := b.activeNonEmptyProofs
	outerCapacityEqualities := b.activeCapacityEqualities
	if proofs := b.capacityEqualsLengthProof(statement.Condition); len(proofs) > 0 {
		merged := map[string]lexer.Token{}
		for collection, token := range outerCapacityEqualities {
			merged[collection] = token
		}
		for collection, token := range proofs {
			merged[collection] = token
		}
		b.activeCapacityEqualities = merged
	}
	b.activeNonEmptyProofs = mergePitfallNonEmptyProofs(outerProofs, b.nonEmptyBranchProof(statement.Condition, true))
	outerLengthConditions := b.activeLengthConditions
	b.activeLengthConditions = append(append([]ast.Expression(nil), outerLengthConditions...), statement.Condition)
	if trueReachable {
		b.withIndexGuards(guards, func() { b.walkBlock(statement.Consequence) })
	}
	b.activeLengthConditions = outerLengthConditions
	b.activeCapacityEqualities = outerCapacityEqualities
	b.activeNonEmptyProofs = mergePitfallNonEmptyProofs(outerProofs, b.nonEmptyBranchProof(statement.Condition, false))
	if falseReachable {
		b.withIndexGuards(b.falseIndexGuards(statement.Condition), func() { b.walkBlock(statement.Alternative) })
	}
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
	if comparison.Operator == "&&" && truthBranch || comparison.Operator == "||" && !truthBranch {
		proofs := mergePitfallNonEmptyProofs(b.nonEmptyBranchProof(comparison.Left, truthBranch), b.nonEmptyBranchProof(comparison.Right, truthBranch))
		return b.survivingNonEmptyProofs(proofs, &ast.ExpressionStatement{Expression: condition})
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
