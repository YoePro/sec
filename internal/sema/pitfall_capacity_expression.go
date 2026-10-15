package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// capacityExpressionGuards derives strict live-length guards from the branch
// required to evaluate a short-circuit RHS. Only facts surviving subsequent
// operand effects are retained; ! and nested logical expressions preserve
// source evaluation order rather than treating all comparisons as simultaneous.
// Rules: rules/analysis/pitfall_analysis.md — "Guards participate in pitfall
// reasoning", "Canonical facts consumed by pitfall analysis";
// rules/foundations/operators.md — short-circuit boolean operators.
func (b *pitfallBuilder) capacityExpressionGuards(condition ast.Expression, truth bool) []pitfallIndexGuard {
	if parameterUsageNodeIsNil(condition) {
		return nil
	}
	if prefix, ok := condition.(*ast.PrefixExpression); ok && prefix.Operator == "!" {
		return b.capacityExpressionGuards(prefix.Right, !truth)
	}
	if infix, ok := condition.(*ast.InfixExpression); ok && (infix.Operator == "&&" && truth || infix.Operator == "||" && !truth) {
		left := b.capacityExpressionGuards(infix.Left, truth)
		left = b.survivingIndexGuards(left, &ast.ExpressionStatement{Expression: infix.Right})
		return append(left, b.capacityExpressionGuards(infix.Right, truth)...)
	}
	if truth {
		return b.conditionIndexGuards(condition)
	}
	return b.falseIndexGuards(condition)
}

// capacityWhileConditionalProofs maps exact access nodes to canonical guards
// active only on their enclosing short-circuit RHS. Unprotected conditional
// accesses remain unknown, and an outer proof is never erased by a nested
// unrelated flag. Mutations anywhere in the RHS conservatively withhold its
// inherited proof. Deferred/callable nodes are still excluded by the owning
// block guard walker, so these witnesses cannot cross execution domains.
// Equality witnesses are kept separate: they cannot protect inclusive endpoints.
// Rules: rules/analysis/pitfall_analysis.md — "Capacity used as live length",
// "False-positive control", "Suppressing evidence";
// rules/control-flow/flowcontrol_while.md — §5.
func (b *pitfallBuilder) capacityWhileConditionalProofs(body *ast.BlockStatement) (map[*ast.IndexExpression]bool, map[*ast.IndexExpression][]pitfallIndexGuard, map[*ast.IndexExpression]lexer.Token) {
	conditional := map[*ast.IndexExpression]bool{}
	protected := map[*ast.IndexExpression][]pitfallIndexGuard{}
	equalities := map[*ast.IndexExpression]lexer.Token{}
	visitStatementExpressions(&ast.UnsafeStatement{Body: body}, func(expression ast.Expression) {
		infix, ok := expression.(*ast.InfixExpression)
		if !ok || infix == nil || infix.Operator != "&&" && infix.Operator != "||" {
			return
		}
		proofs := b.capacityBranchProof(infix.Left, infix.Operator == "&&")
		proofs = b.survivingCapacityEqualities(proofs, &ast.ExpressionStatement{Expression: infix.Right})
		guards := b.capacityExpressionGuards(infix.Left, infix.Operator == "&&")
		guards = b.survivingIndexGuards(guards, &ast.ExpressionStatement{Expression: infix.Right})
		for _, index := range indexesInStatement(&ast.ExpressionStatement{Expression: infix.Right}) {
			conditional[index] = true
			collection, collectionOK := b.expressionIdentity(index.Left)
			identity, indexOK := b.expressionIdentity(index.Index)
			if !collectionOK || !indexOK {
				continue
			}
			if proof, ok := proofs[collection]; ok {
				equalities[index] = proof
			}
			for _, guard := range guards {
				if guard.collectionIdentity == collection && guard.indexIdentity == identity {
					protected[index] = append(protected[index], guard)
				}
			}
		}
	})
	return conditional, protected, equalities
}

// walkCapacityInfix scopes equality witnesses to the RHS actually guarded by
// a true && or false || LHS. Subsequent RHS effects conservatively invalidate
// inherited witnesses; fresh nested guards can re-establish them. Siblings do
// not retain expression-local proofs. Owning bounds analysis keeps its existing
// constant-short-circuit reachability and receives no optional proof state.
// Rules: rules/analysis/pitfall_analysis.md — "Capacity used as live length",
// "Suppressing evidence", "Guards participate in pitfall reasoning";
// rules/foundations/operators.md — "Design summary" (lazy logical evaluation).
func (b *pitfallBuilder) walkCapacityInfix(expression *ast.InfixExpression) {
	b.walkExpression(expression.Left)
	if b.boundsOnly {
		if value, known := b.analyzer.constantBooleanValue(expression.Left); known && (expression.Operator == "&&" && !value || expression.Operator == "||" && value) {
			return
		}
		b.walkExpression(expression.Right)
		return
	}
	outer := b.activeCapacityEqualities
	proofs := outer
	if expression.Operator == "&&" || expression.Operator == "||" {
		proofs = mergeCapacityEqualities(proofs, b.capacityBranchProof(expression.Left, expression.Operator == "&&"))
	}
	b.activeCapacityEqualities = b.survivingCapacityEqualities(proofs, &ast.ExpressionStatement{Expression: expression.Right})
	b.walkExpression(expression.Right)
	b.activeCapacityEqualities = outer
}
