package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// indexedStructuralMutation is one structural mutation of the traversed
// collection found in an indexed loop body.
type indexedStructuralMutation struct {
	call   *ast.CallExpression
	member CompilerKnownMember
	// endsTraversal records that the loop cannot continue after the mutation
	// on its path: the mutation is returned, or the next statement of its own
	// block is `return`, or `break` of this loop.
	endsTraversal bool
	// clearedThenIndexed is the straight-line index of the collection by the
	// loop binding that follows a Clear() in the same block.
	clearedThenIndexed *ast.IndexExpression
}

// inspectIndexedStructuralMutation correlates a forward indexed traversal
// whose range end was evaluated once from a collection's Len with structural
// mutations of that same collection in the loop body. Each mutation is
// classified instead of warning on every mutation inside a loop:
//
//   - proven invalid: Clear() followed on the same straight-line path by an
//     index of the collection with the loop binding — the collection is
//     empty there;
//   - likely mistake: RemoveAt, Remove, Clear, or Insert while the loop
//     addresses the collection by its binding (an index or the mutation's own
//     argument) — later elements shift, the next element is skipped or
//     revisited, and the final iterations run past the new end;
//   - proven safe (suppressed): the traversal ends right after the mutation,
//     the mutation only appends beyond the once-evaluated range, or the loop
//     never addresses the collection by its binding (a draining loop).
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — "Structural mutation during indexed iteration"
//   - rules/control-flow/flowcontrol_for.md — range bounds are evaluated exactly once
//   - rules/analysis/pitfall_analysis.md — "Suppressing evidence"
func (b *pitfallBuilder) inspectIndexedStructuralMutation(loop *ast.ForStatement) {
	rangeExpression, ok := loop.Iterable.(*ast.RangeExpression)
	if !ok || rangeExpression == nil || loop.Step != nil || loop.Body == nil || len(loop.Bindings) != 1 {
		return
	}
	collection, lengthToken, ok := b.lengthReceiver(rangeExpression.End)
	if !ok {
		return
	}
	binding := loop.Bindings[0]
	bindingUsed := !binding.Discard
	indexedByBinding := false
	if bindingUsed {
		for _, statement := range loop.Body.Statements {
			for _, index := range indexesInStatement(statement) {
				if identity, same := b.expressionIdentity(index.Left); same && identity == collection && b.expressionUsesBinding(index.Index, binding.Token) {
					indexedByBinding = true
				}
			}
		}
	}

	for _, mutation := range b.indexedStructuralMutations(loop.Body, collection, binding, false) {
		argumentUsesBinding := false
		if bindingUsed {
			for _, argument := range mutation.call.Arguments {
				if b.expressionUsesBinding(argument, binding.Token) {
					argumentUsesBinding = true
				}
			}
		}
		finding := PitfallFinding{
			Rule:           PitfallIndexedStructuralMutation,
			Family:         PitfallIterationMutation,
			Classification: PitfallLikelyMistake,
			Confidence:     PitfallConfidenceHigh,
			Subject:        PitfallSubject{Expression: mutation.call.String(), Source: mutation.call.Token},
			EvidenceFor: []PitfallEvidence{
				{Strength: PitfallEvidenceProof, Fact: "the range end is evaluated once from the collection's Len before the loop", Source: lengthToken},
				{Strength: PitfallEvidenceProof, Fact: mutation.member.Name + " structurally mutates the same collection inside the loop", Source: mutation.call.Token},
			},
			OwningRule: "bounds",
			Actions: []PitfallSuggestedAction{{
				Kind:   PitfallSuggestedEdit,
				Title:  "traverse in reverse, or use a while loop that advances the index only when no element is removed",
				Source: loop.Token,
			}},
		}
		switch {
		case mutation.clearedThenIndexed != nil:
			finding.Classification = PitfallProvenInvalid
			finding.Confidence = PitfallConfidenceProven
			finding.EvidenceFor = append(finding.EvidenceFor, PitfallEvidence{
				Strength: PitfallEvidenceProof,
				Fact:     "the collection is empty after Clear() and is then indexed by the loop binding on the same path",
				Source:   mutation.clearedThenIndexed.Token,
			})
		case mutation.endsTraversal:
			suppressIndexedStructuralMutation(&finding, "the traversal ends immediately after the structural mutation", mutation.call.Token)
		case mutation.member.Name == "Append":
			suppressIndexedStructuralMutation(&finding, "appended elements lie beyond the once-evaluated range, so every visited index stays valid", mutation.call.Token)
		case !indexedByBinding && !argumentUsesBinding:
			suppressIndexedStructuralMutation(&finding, "the loop never addresses the collection by its loop binding", loop.Token)
		default:
			fact := "later elements shift, so the next element is skipped or revisited and the final iterations run past the new end"
			if mutation.member.Name == "Clear" {
				fact = "every later iteration addresses an empty collection"
			}
			finding.EvidenceFor = append(finding.EvidenceFor, PitfallEvidence{Strength: PitfallEvidenceStrong, Fact: fact, Source: mutation.call.Token})
		}
		b.add(finding)
	}
}

func suppressIndexedStructuralMutation(finding *PitfallFinding, reason string, source lexer.Token) {
	evidence := PitfallEvidence{Strength: PitfallEvidenceSuppressing, Fact: reason, Source: source}
	finding.State = PitfallStateSuppressed
	finding.EvidenceAgainst = []PitfallEvidence{evidence}
	finding.Suppression = &PitfallSuppression{Reason: reason, Evidence: []PitfallEvidence{evidence}}
}

// indexedStructuralMutations collects the structural mutations of collection
// in block and its nested control flow. nestedLoop is true inside a nested
// loop, where `break` no longer ends the traversal under inspection.
func (b *pitfallBuilder) indexedStructuralMutations(block *ast.BlockStatement, collection string, binding ast.ForBinding, nestedLoop bool) []indexedStructuralMutation {
	if block == nil {
		return nil
	}
	result := []indexedStructuralMutation{}
	for index, statement := range block.Statements {
		if parameterUsageNodeIsNil(statement) {
			continue
		}
		switch statement := statement.(type) {
		case *ast.IfStatement:
			result = append(result, b.mutationsInExpression(statement.Condition, collection, false)...)
			result = append(result, b.indexedStructuralMutations(statement.Consequence, collection, binding, nestedLoop)...)
			result = append(result, b.indexedStructuralMutations(statement.Alternative, collection, binding, nestedLoop)...)
			continue
		case *ast.WhileStatement:
			result = append(result, b.mutationsInExpression(statement.Condition, collection, false)...)
			result = append(result, b.indexedStructuralMutations(statement.Body, collection, binding, true)...)
			continue
		case *ast.ForStatement:
			result = append(result, b.mutationsInExpression(statement.Iterable, collection, false)...)
			result = append(result, b.indexedStructuralMutations(statement.Body, collection, binding, true)...)
			continue
		case *ast.SwitchStatement:
			result = append(result, b.mutationsInExpression(statement.Subject, collection, false)...)
			for _, item := range append(append([]*ast.SwitchCase(nil), statement.Cases...), statement.Default) {
				if item != nil {
					result = append(result, b.indexedStructuralMutations(item.Body, collection, binding, nestedLoop)...)
				}
			}
			continue
		}
		if !pitfallStraightLineStatement(statement) {
			continue
		}
		_, returned := statement.(*ast.ReturnStatement)
		ends := returned
		if index+1 < len(block.Statements) {
			switch block.Statements[index+1].(type) {
			case *ast.ReturnStatement:
				ends = true
			case *ast.BreakStatement:
				ends = ends || !nestedLoop
			}
		}
		found := []indexedStructuralMutation{}
		visitStatementExpressions(statement, func(expression ast.Expression) {
			found = append(found, b.mutationsInExpression(expression, collection, ends)...)
		})
		for position := range found {
			if found[position].member.Name != "Clear" || ends {
				continue
			}
			found[position].clearedThenIndexed = b.followingBindingIndex(block.Statements[index+1:], collection, binding)
		}
		result = append(result, found...)
	}
	return result
}

// mutationsInExpression returns the structural mutation when expression is a
// direct compiler-known structural call on collection, such as
// values.RemoveAt(i). Nested expressions are visited by the caller.
func (b *pitfallBuilder) mutationsInExpression(expression ast.Expression, collection string, ends bool) []indexedStructuralMutation {
	call, ok := expression.(*ast.CallExpression)
	if !ok || call == nil {
		return nil
	}
	member, ok := call.Callee.(*ast.MemberExpression)
	if !ok || member == nil || member.Property == nil {
		return nil
	}
	known, resolved := b.analyzer.compilerKnownMemberFacts[sourceTokenLocation(member.Property.Token)]
	if !resolved || !known.StructuralMutation {
		return nil
	}
	identity, same := b.expressionIdentity(member.Object)
	if !same || identity != collection {
		return nil
	}
	return []indexedStructuralMutation{{call: call, member: known, endsTraversal: ends}}
}

// followingBindingIndex returns the first straight-line index of collection
// by the loop binding among statements, which follow a Clear() on the same
// path. Control flow before such an access stops the search.
func (b *pitfallBuilder) followingBindingIndex(statements []ast.Statement, collection string, binding ast.ForBinding) *ast.IndexExpression {
	if binding.Discard {
		return nil
	}
	for _, statement := range statements {
		if !pitfallStraightLineStatement(statement) {
			return nil
		}
		switch statement.(type) {
		case *ast.ReturnStatement, *ast.BreakStatement, *ast.ContinueStatement:
			return nil
		}
		for _, index := range indexesInStatement(statement) {
			if identity, same := b.expressionIdentity(index.Left); same && identity == collection && b.expressionUsesBinding(index.Index, binding.Token) {
				return index
			}
		}
	}
	return nil
}
