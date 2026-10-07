package sema

import (
	"sec/internal/ast"
	"strings"
)

// pitfallGuardAccess retains the live check witnesses and protection at an
// access, rather than attributing every descendant to its enclosing if.
type pitfallGuardAccess struct {
	pitfallResolvedIndex
	guards, witnesses []pitfallIndexGuard
}

// guardSurvivesStatement invalidates a relation on replacement of either
// subject, structural changes, mutable call authority or opaque execution.
// Element writes retain Len; uncertain evaluation order with a write withholds
// the entire statement's evidence. Parent-place replacement invalidates fields.
// Rules: rules/analysis/pitfall_analysis.md — "Guards participate in pitfall reasoning",
// "Canonical facts consumed by pitfall analysis", "Evidence model".
func (b *pitfallBuilder) guardSurvivesStatement(g pitfallIndexGuard, statement ast.Statement) bool {
	if b.statementAssignsIdentity(statement, g.indexIdentity) || b.statementAssignsIdentity(statement, g.collectionIdentity) {
		return false
	}
	survives := true
	visitStatementExpressions(statement, func(expression ast.Expression) {
		switch expression := expression.(type) {
		case *ast.CallExpression:
			scan := pitfallEndpointScan{builder: b, collection: g.collectionIdentity}
			if scan.callMayChangeLength(expression) {
				survives = false
			}
		case *ast.RefExpression:
			if expression.Mutable {
				survives = false
			}
		case *ast.RuntimeCallExpression, *ast.AwaitExpression:
			survives = false
		}
	})
	// A containing stored value may have been replaced even when the collection
	// itself is a field. Identity prefixes are canonical declaration identities.
	for _, identity := range []string{g.indexIdentity, g.collectionIdentity} {
		for dot := strings.LastIndex(identity, "."); dot >= 0; dot = strings.LastIndex(identity, ".") {
			identity = identity[:dot]
			if b.statementAssignsIdentity(statement, identity) {
				survives = false
			}
		}
	}
	return survives
}

// survivingIndexGuards copies path state and conservatively intersects it with
// possible writes, so sibling paths cannot share a mutable proof slice.
// Rules: rules/analysis/pitfall_analysis.md — "Guards participate in pitfall reasoning".
func (b *pitfallBuilder) survivingIndexGuards(guards []pitfallIndexGuard, statement ast.Statement) []pitfallIndexGuard {
	result := []pitfallIndexGuard{}
	for _, guard := range guards {
		if b.guardSurvivesStatement(guard, statement) {
			result = append(result, guard)
		}
	}
	return result
}

// guardedAccesses follows reachable nested statement domains, with branch-local
// guards and invalidation before later uses. Loops invalidate entry witnesses
// against all possible iterations; their own bounds are re-established locally.
// Callable/deferred bodies are separate execution domains and are not borrowed
// as evidence for an immediately preceding check. Unsupported arm expressions
// conservatively withhold evidence instead of inventing path protection.
// Rules: rules/analysis/pitfall_analysis.md — "Reachability", "Guard checks the wrong value",
// "Safety check without control transfer", "Guards participate in pitfall reasoning";
// rules/control-flow/flowcontrol_if.md — §27; rules/control-flow/flowcontrol_for.md — §§12, 30–31.
func (b *pitfallBuilder) guardedAccesses(block *ast.BlockStatement, guards, witnesses []pitfallIndexGuard) []pitfallGuardAccess {
	var result []pitfallGuardAccess
	if block == nil {
		return result
	}
	for _, statement := range block.Statements {
		switch s := statement.(type) {
		case *ast.IfStatement:
			// Only condition evaluation precedes either branch.
			condition := &ast.ExpressionStatement{Expression: s.Condition}
			guards = b.survivingIndexGuards(guards, condition)
			witnesses = b.survivingIndexGuards(witnesses, condition)
			flow, known := b.analyzer.ResolvedIfFlowOf(s)
			if !known || flow.TruePathExecution != ResolvedIfPathNever {
				nested := append(append([]pitfallIndexGuard(nil), guards...), b.conditionIndexGuards(s.Condition)...)
				result = append(result, b.guardedAccesses(s.Consequence, nested, witnesses)...)
			}
			if !known || flow.FalsePathExecution != ResolvedIfPathNever {
				nested := append(append([]pitfallIndexGuard(nil), guards...), b.falseIndexGuards(s.Condition)...)
				result = append(result, b.guardedAccesses(s.Alternative, nested, witnesses)...)
			}
		case *ast.UnsafeStatement:
			result = append(result, b.guardedAccesses(s.Body, guards, witnesses)...)
		case *ast.ForStatement:
			entryGuards := b.survivingIndexGuards(guards, s)
			entryWitnesses := b.survivingIndexGuards(witnesses, s)
			if g, ok := b.loopIndexGuard(s); ok && b.guardSurvivesStatement(g, s) {
				entryGuards = append(entryGuards, g)
			}
			result = append(result, b.guardedAccesses(s.Body, entryGuards, entryWitnesses)...)
		case *ast.WhileStatement:
			flow, known := b.analyzer.ResolvedWhileFlowOf(s)
			if !known || !flow.ConditionKnown || flow.ConditionValue {
				entryGuards := b.survivingIndexGuards(guards, s)
				entryGuards = append(entryGuards, b.conditionIndexGuards(s.Condition)...)
				result = append(result, b.guardedAccesses(s.Body, entryGuards, b.survivingIndexGuards(witnesses, s))...)
			}
		case *ast.SwitchStatement:
			// Explicit fallthrough can carry writes from another clause. Withhold
			// inherited witnesses if any clause changes their subjects.
			entryGuards := b.survivingIndexGuards(guards, s)
			entryWitnesses := b.survivingIndexGuards(witnesses, s)
			for _, clause := range append(append([]*ast.SwitchCase(nil), s.Cases...), s.Default) {
				if clause != nil {
					result = append(result, b.guardedAccesses(clause.Body, entryGuards, entryWitnesses)...)
				}
			}
		case *ast.SelectStatement:
			for _, branch := range s.Branches {
				if branch != nil {
					result = append(result, b.guardedAccesses(branch.Body, b.survivingIndexGuards(guards, &ast.ExpressionStatement{Expression: branch.Value}), b.survivingIndexGuards(witnesses, &ast.ExpressionStatement{Expression: branch.Value}))...)
				}
			}
		case *ast.MatchStatement:
			// Match arm identities/continuations are not implied by the guard syntax.
			if s.Match != nil {
				for _, arm := range s.Match.Arms {
					if arm != nil {
						result = append(result, b.guardedAccesses(arm.BlockBody, b.survivingIndexGuards(guards, &ast.ExpressionStatement{Expression: s.Match.Subject}), b.survivingIndexGuards(witnesses, &ast.ExpressionStatement{Expression: s.Match.Subject}))...)
					}
				}
			}
		case *ast.DeferStatement:
			// Execution occurs at scope exit, outside this proof state.
		default:
			liveGuards := b.survivingIndexGuards(guards, statement)
			liveWitnesses := b.survivingIndexGuards(witnesses, statement)
			if pitfallStraightLineStatement(statement) {
				for _, access := range b.immediateStatementIndexes(statement) {
					collection, collectionOK := b.expressionIdentity(access.Left)
					index, indexOK := b.expressionIdentity(access.Index)
					if collectionOK && indexOK {
						result = append(result, pitfallGuardAccess{pitfallResolvedIndex: pitfallResolvedIndex{access: access, indexIdentity: index, collectionIdentity: collection}, guards: liveGuards, witnesses: liveWitnesses})
					}
				}
			}
		}
		guards = b.survivingIndexGuards(guards, statement)
		guards = append(guards, b.continuationIndexGuards(statement)...)
		witnesses = b.survivingIndexGuards(witnesses, statement)
		if !b.analyzer.statementCanFallThrough(statement) {
			break
		}
	}
	return result
}

// guardCoversAccess compares canonical index/collection pairs on one live path.
// Rules: rules/analysis/pitfall_analysis.md — "Guards participate in pitfall reasoning".
func guardCoversAccess(guards []pitfallIndexGuard, access pitfallResolvedIndex) bool {
	for _, g := range guards {
		if g.indexIdentity == access.indexIdentity && g.collectionIdentity == access.collectionIdentity {
			return true
		}
	}
	return false
}

// falseIndexGuards normalizes a rejected upper-bound comparison on the false
// path; disjunctions and unrelated operands cannot establish protection.
// Rules: rules/analysis/pitfall_analysis.md — "Guards participate in pitfall reasoning".
func (b *pitfallBuilder) falseIndexGuards(condition ast.Expression) []pitfallIndexGuard {
	comparison, ok := condition.(*ast.InfixExpression)
	if !ok {
		return nil
	}
	inverse := map[string]string{">=": "<", "<=": ">"}[comparison.Operator]
	if inverse == "" {
		return nil
	}
	normalized := *comparison
	normalized.Operator = inverse
	return b.strictIndexGuards(&normalized)
}

// continuationIndexGuards retains a relation only when the opposite branch
// cannot continue, and the surviving branch has not invalidated that relation.
// Rules: rules/analysis/pitfall_analysis.md — "Reachability", "Guards participate in pitfall reasoning";
// rules/control-flow/flowcontrol_if.md — §27.
func (b *pitfallBuilder) continuationIndexGuards(statement ast.Statement) []pitfallIndexGuard {
	conditional, ok := statement.(*ast.IfStatement)
	if !ok {
		return nil
	}
	var result []pitfallIndexGuard
	if !b.analyzer.blockCanFallThrough(conditional.Consequence) {
		for _, guard := range b.falseIndexGuards(conditional.Condition) {
			if !b.blockAssignsIdentity(conditional.Alternative, guard.indexIdentity) && !b.blockAssignsIdentity(conditional.Alternative, guard.collectionIdentity) {
				alternative := &ast.UnsafeStatement{Body: conditional.Alternative}
				if b.guardSurvivesStatement(guard, alternative) {
					result = append(result, guard)
				}
			}
		}
	}
	if conditional.Alternative != nil && !b.analyzer.blockCanFallThrough(conditional.Alternative) {
		for _, guard := range b.conditionIndexGuards(conditional.Condition) {
			if b.guardSurvivesStatement(guard, &ast.UnsafeStatement{Body: conditional.Consequence}) {
				result = append(result, guard)
			}
		}
	}
	return result
}

// immediateStatementIndexes excludes nested callable execution domains from
// generic source traversal using the parsed body identity.
// Rules: rules/analysis/pitfall_analysis.md — "Reachability", "Canonical facts consumed by pitfall analysis".
func (b *pitfallBuilder) immediateStatementIndexes(statement ast.Statement) []*ast.IndexExpression {
	excluded := map[*ast.IndexExpression]bool{}
	visitStatementExpressions(statement, func(expression ast.Expression) {
		var body *ast.BlockStatement
		switch expression := expression.(type) {
		case *ast.LambdaExpression:
			body = expression.Body
		case *ast.SpawnExpression:
			body = expression.Body
		}
		if body != nil {
			for _, statement := range body.Statements {
				for _, index := range indexesInStatement(statement) {
					excluded[index] = true
				}
			}
		}
	})
	var result []*ast.IndexExpression
	for _, index := range indexesInStatement(statement) {
		if !excluded[index] {
			result = append(result, index)
		}
	}
	return result
}

// conditionIndexGuards withholds a bound when evaluating another operand of
// the same condition may already invalidate it.
// Rules: rules/analysis/pitfall_analysis.md — "Guards participate in pitfall reasoning";
// rules/foundations/operators.md — "Logical operators".
func (b *pitfallBuilder) conditionIndexGuards(condition ast.Expression) []pitfallIndexGuard {
	return b.survivingIndexGuards(b.strictIndexGuards(condition), &ast.ExpressionStatement{Expression: condition})
}

// withLoopIndexGuards excludes stale inherited proofs across a backedge while
// preserving guards that the loop condition establishes on each iteration.
// Rules: rules/analysis/pitfall_analysis.md — "Guards participate in pitfall reasoning";
// rules/control-flow/flowcontrol_for.md — §§12, 39.
func (b *pitfallBuilder) withLoopIndexGuards(statement ast.Statement, guards []pitfallIndexGuard, walk func()) {
	outer := b.activeIndexGuards
	b.activeIndexGuards = b.survivingIndexGuards(outer, statement)
	b.withIndexGuards(guards, walk)
	b.activeIndexGuards = outer
}

// walkIndependentGuardBody separates deferred and callable execution from the
// creation site's temporary path guards.
// Rules: rules/analysis/pitfall_analysis.md — "Reachability", "Guards participate in pitfall reasoning".
func (b *pitfallBuilder) walkIndependentGuardBody(body *ast.BlockStatement) {
	outer := b.activeIndexGuards
	b.activeIndexGuards = nil
	b.walkBlock(body)
	b.activeIndexGuards = outer
}
