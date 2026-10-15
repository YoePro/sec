package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// inspectWrongBoundSource reports a range loop bounded by one
// collection's Len whose binding indexes only a different collection while
// the bounding collection is otherwise unused in the body — the copy/paste
// shape `for i in 0..<left.Len { Process(right[i]) }`. A proven
// `right.Len >= left.Len` (or `==`) on a half-open path suppresses it;
// inclusive endpoints require strict `right.Len > left.Len`. Proofs can be: a preceding
// assert of the same block, a preceding exit guard such as
// `if left.Len != right.Len { return }`, or an enclosing if condition, unless
// either collection is structurally mutated after the proof or inside the
// loop. A loop that also uses the bounding collection is a parallel traversal
// and left to the owning bounds rule.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — "Bound source differs from indexed collection", "Parallel collection traversal"
//   - rules/analysis/pitfall_analysis.md — "Required collection-relation tests"
//   - rules/control-flow/flowcontrol_for.md — explicit step and §29 Range termination
func (b *pitfallBuilder) inspectWrongBoundSource(loop *ast.ForStatement, preceding []ast.Statement) {
	rangeExpression, ok := loop.Iterable.(*ast.RangeExpression)
	if !ok || rangeExpression == nil || loop.Body == nil || len(loop.Bindings) != 1 || loop.Bindings[0].Discard {
		return
	}
	// Inclusive normalization covers direct indexes and ascending steps.
	// Only a canonical positive constant proves an explicit step's direction;
	// runtime, zero and descending steps remain outside this domain.
	if !rangeExpression.Exclusive && loop.Step != nil {
		step, known := b.analyzer.integerConstantValue(loop.Step)
		if !known || step.Sign() <= 0 {
			return
		}
	}
	bound, lengthToken, ok := b.lengthReceiver(rangeExpression.End)
	if !ok {
		return
	}
	binding := loop.Bindings[0]
	indexed := ""
	var subject *ast.IndexExpression
	boundUsed := false
	for _, statement := range loop.Body.Statements {
		visitStatementExpressions(statement, func(expression ast.Expression) {
			if identity, known := b.expressionIdentity(expression); known && identity == bound {
				boundUsed = true
			}
			index, isIndex := expression.(*ast.IndexExpression)
			if !isIndex || !b.expressionUsesBinding(index.Index, binding.Token) {
				return
			}
			if !rangeExpression.Exclusive {
				if _, direct := index.Index.(*ast.Identifier); !direct {
					return
				}
			}
			identity, known := b.expressionIdentity(index.Left)
			switch {
			case !known:
				indexed = "\x00"
			case indexed == "":
				indexed, subject = identity, index
			case indexed != identity:
				indexed = "\x00"
			}
		})
	}
	if boundUsed || subject == nil || indexed == "\x00" || indexed == bound {
		return
	}
	finding := PitfallFinding{
		Rule:           PitfallWrongBoundSource,
		Family:         PitfallCollectionRelations,
		Classification: PitfallLikelyMistake,
		Confidence:     PitfallConfidenceHigh,
		Subject:        PitfallSubject{Expression: subject.String(), Source: subject.Token},
		EvidenceFor: []PitfallEvidence{
			{Strength: PitfallEvidenceProof, Fact: "the range is bounded by the Len of " + rangeExpression.End.(*ast.MemberExpression).Object.String(), Source: lengthToken},
			{Strength: PitfallEvidenceStrong, Fact: "the loop binding indexes only " + subject.Left.String() + ", and the bounding collection is otherwise unused in the body", Source: subject.Token},
			{Strength: PitfallEvidenceStrong, Fact: "no relation proves " + subject.Left.String() + ".Len is at least the bound", Source: subject.Token},
		},
		OwningRule: "bounds",
		Actions: []PitfallSuggestedAction{{
			Kind: PitfallSuggestedEdit, Title: "bound the loop by " + subject.Left.String() + ".Len", Replacement: subject.Left.String() + ".Len", Source: lengthToken,
		}},
	}
	if !rangeExpression.Exclusive && loop.Step != nil {
		finding.EvidenceFor = append(finding.EvidenceFor, PitfallEvidence{
			Strength: PitfallEvidenceProof,
			Fact:     "the explicit step is a proven positive integer constant, so the range progresses upward",
			Source:   expressionToken(loop.Step),
		})
	}
	if !rangeExpression.Exclusive {
		finding.EvidenceFor[2].Fact = "no relation proves " + subject.Left.String() + ".Len is strictly greater than the inclusive endpoint"
		finding.Actions[0].Title = "use a half-open range bounded by " + subject.Left.String() + ".Len"
		finding.Actions[0].Replacement = ""
	}
	if proof, proven := b.lengthRelationOnPath(indexed, bound, preceding, !rangeExpression.Exclusive); proven {
		if b.guardSurvivesStatement(pitfallIndexGuard{collectionIdentity: indexed}, &ast.UnsafeStatement{Body: loop.Body}) &&
			b.guardSurvivesStatement(pitfallIndexGuard{collectionIdentity: bound}, &ast.UnsafeStatement{Body: loop.Body}) {
			evidence := PitfallEvidence{Strength: PitfallEvidenceSuppressing, Fact: "the indexed collection's Len covers the range endpoint on this path and the loop preserves the length relation", Source: proof}
			finding.State = PitfallStateSuppressed
			finding.EvidenceAgainst = []PitfallEvidence{evidence}
			finding.Suppression = &PitfallSuppression{Reason: evidence.Fact, Evidence: []PitfallEvidence{evidence}}
		}
	}
	b.add(finding)
}

// lengthRelationOnPath finds a proof that indexed.Len >= bound.Len before the
// loop: a preceding assert or exit guard of the same block, or else an
// enclosing if condition. A structural mutation of either collection between
// the proof and the loop invalidates it. Inclusive endpoints set strict to
// require indexed.Len > bound.Len. Canonical mutable effects revoke proofs.
// Rules: rules/analysis/pitfall_analysis.md — "Bound source differs from indexed collection", "Suppressing evidence".
func (b *pitfallBuilder) lengthRelationOnPath(indexed string, bound string, preceding []ast.Statement, strict bool) (lexer.Token, bool) {
	for position := len(preceding) - 1; position >= 0; position-- {
		statement := preceding[position]
		switch statement := statement.(type) {
		case *ast.AssertStatement:
			if statement != nil && b.lengthRelationProves(statement.Condition, indexed, bound, true, strict) {
				return statement.Token, true
			}
		case *ast.IfStatement:
			if statement != nil && statement.Alternative == nil && pitfallBlockDefinitelyExits(statement.Consequence) &&
				b.lengthRelationProves(statement.Condition, indexed, bound, false, strict) {
				return statement.Token, true
			}
		}
		if !b.guardSurvivesStatement(pitfallIndexGuard{collectionIdentity: indexed}, statement) ||
			!b.guardSurvivesStatement(pitfallIndexGuard{collectionIdentity: bound}, statement) {
			return lexer.Token{}, false
		}
	}
	for _, condition := range b.activeLengthConditions {
		if b.lengthRelationProves(condition, indexed, bound, true, strict) {
			return expressionToken(condition), true
		}
	}
	return lexer.Token{}, false
}

// lengthRelationProves reports that condition, when it evaluates to holds,
// proves indexed.Len >= bound.Len: a direct Len comparison in either operand
// order, a conjunct of a true `&&`, or a disjunct of a false `||`.
// Strict endpoint coverage accepts only >; effectful conditions prove nothing.
// Rules: rules/analysis/pitfall_analysis.md — "Bound source differs from indexed collection", "False-positive control".
func (b *pitfallBuilder) lengthRelationProves(condition ast.Expression, indexed string, bound string, holds, strict bool) bool {
	if !b.guardSurvivesStatement(pitfallIndexGuard{collectionIdentity: indexed}, &ast.ExpressionStatement{Expression: condition}) ||
		!b.guardSurvivesStatement(pitfallIndexGuard{collectionIdentity: bound}, &ast.ExpressionStatement{Expression: condition}) {
		return false
	}
	infix, ok := condition.(*ast.InfixExpression)
	if !ok || infix == nil {
		return false
	}
	switch {
	case infix.Operator == "&&" && holds, infix.Operator == "||" && !holds:
		return b.lengthRelationProves(infix.Left, indexed, bound, holds, strict) || b.lengthRelationProves(infix.Right, indexed, bound, holds, strict)
	}
	left, _, leftOK := b.lengthReceiver(infix.Left)
	right, _, rightOK := b.lengthReceiver(infix.Right)
	if !leftOK || !rightOK {
		return false
	}
	operator := infix.Operator
	if !holds {
		operator = map[string]string{"<": ">=", "<=": ">", ">": "<=", ">=": "<", "==": "!=", "!=": "=="}[operator]
	}
	switch {
	case left == indexed && right == bound:
	case left == bound && right == indexed:
		operator = mirroredRelation(operator)
	default:
		return false
	}
	return operator == ">" || !strict && (operator == ">=" || operator == "==")
}
