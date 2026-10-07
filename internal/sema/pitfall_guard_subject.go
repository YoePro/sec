package sema

import (
	"strconv"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// pitfallIndexGuard is an established strict upper bound `index < collection.Len`
// on the current path, from an enclosing if consequence, while body, or
// half-open `0..<collection.Len` for-loop binding.
type pitfallIndexGuard struct {
	indexIdentity      string
	collectionIdentity string
	source             lexer.Token
}

// strictIndexGuards collects the `index < X.Len` (or `X.Len > index`)
// conjuncts of condition. A disjunction or any other shape proves nothing.
//
// Rules: rules/analysis/pitfall_analysis.md — "Guards participate in pitfall reasoning".
func (b *pitfallBuilder) strictIndexGuards(condition ast.Expression) []pitfallIndexGuard {
	comparison, ok := condition.(*ast.InfixExpression)
	if !ok {
		return nil
	}
	switch comparison.Operator {
	case "&&":
		return append(b.strictIndexGuards(comparison.Left), b.strictIndexGuards(comparison.Right)...)
	case "<":
		if boundary, ok := b.resolvedLengthBoundary(comparison.Left, comparison.Right, comparison.Token); ok {
			return []pitfallIndexGuard{{indexIdentity: boundary.indexIdentity, collectionIdentity: boundary.collectionIdentity, source: comparison.Token}}
		}
	case ">":
		if boundary, ok := b.resolvedLengthBoundary(comparison.Right, comparison.Left, comparison.Token); ok {
			return []pitfallIndexGuard{{indexIdentity: boundary.indexIdentity, collectionIdentity: boundary.collectionIdentity, source: comparison.Token}}
		}
	}
	return nil
}

// loopIndexGuard returns the guard a canonical half-open `start..<X.Len` loop
// establishes for its single binding.
func (b *pitfallBuilder) loopIndexGuard(loop *ast.ForStatement) (pitfallIndexGuard, bool) {
	rangeExpression, ok := loop.Iterable.(*ast.RangeExpression)
	if !ok || rangeExpression == nil || !rangeExpression.Exclusive || len(loop.Bindings) != 1 || loop.Bindings[0].Discard {
		return pitfallIndexGuard{}, false
	}
	collection, _, ok := b.lengthReceiver(rangeExpression.End)
	if !ok {
		return pitfallIndexGuard{}, false
	}
	binding := loop.Bindings[0].Token
	identity := strings.Join([]string{binding.File, strconv.Itoa(binding.Line), strconv.Itoa(binding.Column)}, ":")
	return pitfallIndexGuard{indexIdentity: identity, collectionIdentity: collection, source: rangeExpression.Token}, true
}

// withIndexGuards runs walk with additional active guards.
func (b *pitfallBuilder) withIndexGuards(guards []pitfallIndexGuard, walk func()) {
	outer := b.activeIndexGuards
	b.activeIndexGuards = append(append([]pitfallIndexGuard(nil), outer...), guards...)
	walk()
	b.activeIndexGuards = outer
}

// pitfallResolvedIndex is one `collection[index]` access whose collection and
// index both resolve to canonical identities.
type pitfallResolvedIndex struct {
	access             *ast.IndexExpression
	indexIdentity      string
	collectionIdentity string
}

// inspectWrongGuardSubject correlates a strict `a < X.Len` guard with the
// reachable accesses it encloses, including nested control flow. When no
// access in the guarded block uses the guarded
// pair, while another resolved access `Y[b]` is protected by neither this
// guard nor any enclosing one, the check most likely tests the wrong value,
// the copy/paste shape of `if leftIndex < left.Len { right[rightIndex] }`.
// An access is never reported when any guard on its path covers it, and a
// block that uses the guarded pair at all is treated as intentional.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — "Guard checks the wrong value"
//   - rules/analysis/pitfall_analysis.md — "Strong advisory scope" (cross-value guard/use mismatch)
//   - rules/analysis/pitfall_analysis.md — "Guards participate in pitfall reasoning"
func (b *pitfallBuilder) inspectWrongGuardSubject(statement *ast.IfStatement, guards []pitfallIndexGuard) {
	if statement == nil || len(guards) == 0 {
		return
	}
	accesses := b.guardedAccesses(statement.Consequence, append(append([]pitfallIndexGuard(nil), b.activeIndexGuards...), guards...), guards)
	for _, access := range accesses {
		for _, guard := range guards {
			if access.indexIdentity == guard.indexIdentity && access.collectionIdentity == guard.collectionIdentity {
				return
			}
		}
	}
	for _, access := range accesses {
		if len(access.witnesses) == 0 || guardCoversAccess(access.guards, access.pitfallResolvedIndex) {
			continue
		}
		covered := false
		for _, guard := range guards {
			if guard.indexIdentity == access.indexIdentity && guard.collectionIdentity == access.collectionIdentity {
				covered = true
			}
		}
		if covered {
			continue
		}
		guard := access.witnesses[0]
		b.add(PitfallFinding{
			Rule:           PitfallWrongGuardSubject,
			Family:         PitfallControlFlow,
			Classification: PitfallLikelyMistake,
			Confidence:     PitfallConfidenceHigh,
			Subject:        PitfallSubject{Expression: access.access.String(), Source: access.access.Token},
			EvidenceFor: []PitfallEvidence{
				{Strength: PitfallEvidenceStrong, Fact: "the guard establishes " + statement.Condition.String() + " but nothing in the guarded block indexes with that pair", Source: guard.source},
				{Strength: PitfallEvidenceStrong, Fact: "this access is covered by no guard on its path", Source: access.access.Token},
			},
			OwningRule: "bounds",
			Actions: []PitfallSuggestedAction{{
				Kind:   PitfallSuggestedEdit,
				Title:  "guard the index and collection that are accessed",
				Source: guard.source,
			}},
		})
	}
}

// inspectCheckWithoutTransfer recognizes `if index >= X.Len { ... }` (or a
// strict `>` rejection) whose branch neither leaves the path nor assigns the
// index, followed on a reachable unchanged path by `X[index]`. The apparent
// safety check does not
// protect the access. The owning bounds analysis decides validity; no control
// transfer is suggested because the intended one cannot be inferred.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — "Safety check without control transfer"
func (b *pitfallBuilder) inspectCheckWithoutTransfer(block *ast.BlockStatement) {
	if block == nil {
		return
	}
	entryGuards := append([]pitfallIndexGuard(nil), b.activeIndexGuards...)
	for statementIndex := 0; statementIndex+1 < len(block.Statements); statementIndex++ {
		if statementIndex > 0 {
			previous := block.Statements[statementIndex-1]
			if !b.analyzer.statementCanFallThrough(previous) {
				break
			}
			entryGuards = b.survivingIndexGuards(entryGuards, previous)
			entryGuards = append(entryGuards, b.continuationIndexGuards(previous)...)
		}
		conditional, ok := block.Statements[statementIndex].(*ast.IfStatement)
		if !ok || conditional.Alternative != nil || conditional.Consequence == nil || len(conditional.Consequence.Statements) == 0 {
			continue
		}
		flow, resolved := b.analyzer.ResolvedIfFlowOf(conditional)
		if resolved && flow.TruePathExecution == ResolvedIfPathNever {
			continue
		}
		if !b.analyzer.blockCanFallThrough(conditional.Consequence) {
			continue
		}
		comparison, ok := conditional.Condition.(*ast.InfixExpression)
		if !ok {
			continue
		}
		var boundary pitfallLengthBoundary
		switch comparison.Operator {
		case ">=", ">":
			boundary, ok = b.resolvedLengthBoundary(comparison.Left, comparison.Right, comparison.Token)
		case "<=", "<":
			boundary, ok = b.resolvedLengthBoundary(comparison.Right, comparison.Left, comparison.Token)
		default:
			ok = false
		}
		if !ok || b.blockAssignsIdentity(conditional.Consequence, boundary.indexIdentity) {
			continue
		}
		witness := pitfallIndexGuard{indexIdentity: boundary.indexIdentity, collectionIdentity: boundary.collectionIdentity, source: comparison.Token}
		if !b.guardSurvivesStatement(witness, conditional) {
			continue
		}
		suffix := &ast.BlockStatement{Statements: block.Statements[statementIndex+1:]}
		for _, access := range b.guardedAccesses(suffix, entryGuards, []pitfallIndexGuard{witness}) {
			if len(access.witnesses) == 0 || guardCoversAccess(access.guards, access.pitfallResolvedIndex) || access.collectionIdentity != boundary.collectionIdentity || access.indexIdentity != boundary.indexIdentity {
				continue
			}
			b.add(PitfallFinding{
				Rule:           PitfallCheckWithoutTransfer,
				Family:         PitfallControlFlow,
				Classification: PitfallLikelyMistake,
				Confidence:     PitfallConfidenceHigh,
				Subject:        PitfallSubject{Expression: access.access.String(), Source: access.access.Token},
				EvidenceFor: []PitfallEvidence{
					{Strength: PitfallEvidenceStrong, Fact: "the check " + comparison.String() + " detects an out-of-range index", Source: comparison.Token},
					{Strength: PitfallEvidenceProof, Fact: "the checking branch neither leaves the path nor assigns the index", Source: conditional.Token},
					{Strength: PitfallEvidenceProof, Fact: "a later reachable access uses the same unchanged collection and index without a protecting guard", Source: access.access.Token},
				},
				OwningRule: "bounds",
			})
		}
	}
}

// blockAssignsIdentity reports whether block may establish a new value for
// the resolved identity anywhere inside it: an assignment (including compound
// and `++`/`--` forms) or a mutable reference a callee could write through.
func (b *pitfallBuilder) blockAssignsIdentity(block *ast.BlockStatement, identity string) bool {
	if block == nil {
		return false
	}
	for _, statement := range block.Statements {
		if b.statementAssignsIdentity(statement, identity) {
			return true
		}
	}
	return false
}

func (b *pitfallBuilder) statementAssignsIdentity(statement ast.Statement, identity string) bool {
	switch statement := statement.(type) {
	case *ast.AssignmentStatement:
		if target, ok := b.expressionIdentity(statement.Target); ok && target == identity {
			return true
		}
	case *ast.TryAssignmentStatement:
		if b.statementAssignsIdentity(statement.Assignment, identity) {
			return true
		}
	case *ast.IfStatement:
		if b.blockAssignsIdentity(statement.Consequence, identity) || b.blockAssignsIdentity(statement.Alternative, identity) {
			return true
		}
	case *ast.ForStatement:
		if b.blockAssignsIdentity(statement.Body, identity) {
			return true
		}
	case *ast.WhileStatement:
		if b.blockAssignsIdentity(statement.Body, identity) {
			return true
		}
	case *ast.UnsafeStatement:
		if b.blockAssignsIdentity(statement.Body, identity) {
			return true
		}
	case *ast.SwitchStatement:
		for _, item := range append(append([]*ast.SwitchCase(nil), statement.Cases...), statement.Default) {
			if item != nil && b.blockAssignsIdentity(item.Body, identity) {
				return true
			}
		}
	case *ast.MatchStatement, *ast.SelectStatement:
		// Arm bodies are not traversed here; conservatively assume a write.
		return true
	}
	written := false
	visitStatementExpressions(statement, func(expression ast.Expression) {
		if reference, ok := expression.(*ast.RefExpression); ok && reference.Mutable {
			if target, ok := b.expressionIdentity(reference.Value); ok && target == identity {
				written = true
			}
		}
	})
	return written
}
