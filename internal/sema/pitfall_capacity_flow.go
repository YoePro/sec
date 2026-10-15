package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// capacityBranchProof keeps equality witnesses only on the branch that
// establishes them. Conjunctions on a true path and disjunctions on a false
// path contribute both operands; the opposite paths prove neither operand.
// Rules: rules/analysis/pitfall_analysis.md — "Capacity used as live length",
// "Guards participate in pitfall reasoning", "Suppressing evidence".
func (b *pitfallBuilder) capacityBranchProof(condition ast.Expression, truth bool) map[string]lexer.Token {
	if negation, ok := condition.(*ast.PrefixExpression); ok && negation.Operator == "!" {
		return b.capacityBranchProof(negation.Right, !truth)
	}
	comparison, ok := condition.(*ast.InfixExpression)
	if !ok {
		return nil
	}
	if comparison.Operator == "&&" && truth || comparison.Operator == "||" && !truth {
		left := b.capacityBranchProof(comparison.Left, truth)
		left = b.survivingCapacityEqualities(left, &ast.ExpressionStatement{Expression: comparison.Right})
		return mergeCapacityEqualities(left, b.capacityBranchProof(comparison.Right, truth))
	}
	if !(comparison.Operator == "==" && truth || comparison.Operator == "!=" && !truth) {
		return nil
	}
	for _, pair := range [][2]ast.Expression{{comparison.Left, comparison.Right}, {comparison.Right, comparison.Left}} {
		length, _, lengthOK := b.lengthReceiver(pair[0])
		capacity, _, capacityOK := b.capacityReceiver(pair[1])
		if lengthOK && capacityOK && length == capacity {
			return map[string]lexer.Token{length: comparison.Token}
		}
	}
	return nil
}

// mergeCapacityEqualities isolates sibling paths while retaining dominating
// witnesses. Rules: rules/analysis/pitfall_analysis.md — "Evidence model".
func mergeCapacityEqualities(outer, local map[string]lexer.Token) map[string]lexer.Token {
	if len(outer)+len(local) == 0 {
		return nil
	}
	result := make(map[string]lexer.Token, len(outer)+len(local))
	for collection, token := range outer {
		result[collection] = token
	}
	for collection, token := range local {
		result[collection] = token
	}
	return result
}

// survivingCapacityEqualities consumes canonical assignment, structural-call
// and mutable/opaque-call invalidation, including parent Place replacement.
// Element writes retain equality; proof tokens are evidence, never validity.
// Rules: rules/analysis/pitfall_analysis.md — "Guards participate in pitfall
// reasoning", "Canonical facts consumed by pitfall analysis".
func (b *pitfallBuilder) survivingCapacityEqualities(proofs map[string]lexer.Token, statement ast.Statement) map[string]lexer.Token {
	if len(proofs) == 0 {
		return nil
	}
	result := make(map[string]lexer.Token, len(proofs))
	for collection, token := range proofs {
		if b.guardSurvivesStatement(pitfallIndexGuard{collectionIdentity: collection}, statement) {
			result[collection] = token
		}
	}
	return result
}

// capacityContinuationProof uses the condition of the only continuing path.
// Condition proofs already account for evaluation order; writes in the
// continuing branch invalidate them, while an exiting sibling cannot do so.
// Rules: rules/control-flow/flowcontrol_if.md — §27;
// rules/analysis/pitfall_analysis.md — "Reachability", "Suppressing evidence".
func (b *pitfallBuilder) capacityContinuationProof(statement ast.Statement) map[string]lexer.Token {
	conditional, ok := statement.(*ast.IfStatement)
	if !ok {
		return nil
	}
	var proofs map[string]lexer.Token
	var continuing *ast.BlockStatement
	if !b.analyzer.blockCanFallThrough(conditional.Consequence) {
		proofs = b.capacityBranchProof(conditional.Condition, false)
		continuing = conditional.Alternative
	} else if conditional.Alternative != nil && !b.analyzer.blockCanFallThrough(conditional.Alternative) {
		proofs = b.capacityBranchProof(conditional.Condition, true)
		continuing = conditional.Consequence
	}
	if continuing != nil {
		for _, statement := range continuing.Statements {
			proofs = b.survivingCapacityEqualities(proofs, statement)
		}
	}
	return proofs
}

// capacityEntryProofs checks effects evaluated before entering a nested block.
// Branch bodies invalidate their own proofs; loop bodies invalidate entry
// witnesses for all iterations so a backedge cannot revive a stale equality.
// Rules: rules/analysis/pitfall_analysis.md — "Guards participate in pitfall
// reasoning", "False-positive control".
func (b *pitfallBuilder) capacityEntryProofs(proofs map[string]lexer.Token, statement ast.Statement) map[string]lexer.Token {
	switch s := statement.(type) {
	case *ast.IfStatement:
		statement = &ast.ExpressionStatement{Expression: s.Condition}
	case *ast.UnsafeStatement:
		return mergeCapacityEqualities(proofs, nil)
	}
	return b.survivingCapacityEqualities(proofs, statement)
}
