package sema

import "sec/internal/ast"

// dominatingZeroElementAccesses finds the nearest unconditional element-zero
// witnesses on every continuing path in this block. Canonical replacement,
// structural mutation and mutable call facts revoke older evidence; a fresh access after a mutation can establish
// new evidence. Reads and element writes preserve the collection's shape.
// Rules: rules/analysis/pitfall_analysis.md — "Skipped-first-element advisory",
// "Guards participate in pitfall reasoning", "Canonical facts consumed by pitfall analysis".
func (b *pitfallBuilder) dominatingZeroElementAccesses(preceding []ast.Statement, collection string) []*ast.IndexExpression {
	for position := len(preceding) - 1; position >= 0; position-- {
		statement := preceding[position]
		if proof := b.zeroElementStatementProof(statement, collection); len(proof) != 0 {
			return proof
		}
		if !b.guardSurvivesStatement(pitfallIndexGuard{collectionIdentity: collection}, statement) {
			return nil
		}
	}
	return nil
}

// unconditionalZeroElementAccess withholds conditional RHS, match arms and
// independently executed bodies from dominating intent evidence. A logical
// LHS still executes before its RHS and can supply an unconditional witness.
// Control-flow statements do not establish new witnesses in this bounded slice.
// Rules: rules/analysis/pitfall_analysis.md — "Skipped-first-element advisory",
// "Reachability", "False-positive control";
// rules/foundations/operators.md — "Design summary" (lazy logical evaluation).
func (b *pitfallBuilder) unconditionalZeroElementAccess(statement ast.Statement, collection string) *ast.IndexExpression {
	if !pitfallStraightLineStatement(statement) {
		return nil
	}
	switch statement.(type) {
	case *ast.DeferStatement, *ast.UnsafeStatement:
		return nil
	}
	excluded := map[*ast.IndexExpression]bool{}
	visitStatementExpressions(statement, func(expression ast.Expression) {
		var conditional ast.Expression
		switch expression := expression.(type) {
		case *ast.InfixExpression:
			if expression.Operator == "&&" || expression.Operator == "||" {
				conditional = expression.Right
			}
		case *ast.MatchExpression, *ast.LambdaExpression, *ast.SpawnExpression:
			conditional = expression
		}
		if !parameterUsageNodeIsNil(conditional) {
			for _, index := range indexesInStatement(&ast.ExpressionStatement{Expression: conditional}) {
				excluded[index] = true
			}
		}
	})
	for _, index := range indexesInStatement(statement) {
		if excluded[index] {
			continue
		}
		identity, known := b.expressionIdentity(index.Left)
		if known && identity == collection && b.pitfallIntegerEquals(index.Index, 0) {
			return index
		}
	}
	return nil
}

// zeroElementStatementProof joins witnesses from every branch that can reach
// the following statement, using Sema's resolved reachability. Missing else
// paths cannot establish a witness; exiting branches do not need one. Each
// continuing branch searches past its own mutations only after fresh handling.
// All contributing access tokens are retained for diagnostic evidence.
// Rules: rules/analysis/pitfall_analysis.md — "Skipped-first-element advisory",
// "Reachability", "Suppressing evidence";
// rules/control-flow/flowcontrol_if.md — §27 "Sema and flow-analysis requirements".
func (b *pitfallBuilder) zeroElementStatementProof(statement ast.Statement, collection string) []*ast.IndexExpression {
	switch statement := statement.(type) {
	case *ast.IfStatement:
		trueContinues := b.analyzer.blockCanFallThrough(statement.Consequence)
		falseContinues := b.analyzer.blockCanFallThrough(statement.Alternative)
		if flow, resolved := b.analyzer.ResolvedIfFlowOf(statement); resolved {
			trueContinues, falseContinues = flow.TruePathContinues, flow.FalsePathContinues
		}
		var witnesses []*ast.IndexExpression
		for _, branch := range []struct {
			body      *ast.BlockStatement
			continues bool
		}{
			{statement.Consequence, trueContinues}, {statement.Alternative, falseContinues},
		} {
			if !branch.continues {
				continue
			}
			if branch.body == nil {
				return nil
			}
			proof := b.dominatingZeroElementAccesses(branch.body.Statements, collection)
			if len(proof) == 0 {
				return nil
			}
			witnesses = append(witnesses, proof...)
		}
		return witnesses
	case *ast.UnsafeStatement:
		if statement.Body != nil {
			return b.dominatingZeroElementAccesses(statement.Body.Statements, collection)
		}
		return nil
	default:
		if !b.guardSurvivesStatement(pitfallIndexGuard{collectionIdentity: collection}, statement) {
			return nil
		}
		if index := b.unconditionalZeroElementAccess(statement, collection); index != nil {
			return []*ast.IndexExpression{index}
		}
		return nil
	}
}
