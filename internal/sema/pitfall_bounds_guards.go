package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

type pitfallLengthBoundary struct {
	indexIdentity      string
	collectionIdentity string
	comparisonToken    lexer.Token
	lengthToken        lexer.Token
}

// inspectIneffectiveUpperBoundsGuard recognizes a direct `index <= values.Len`
// branch whose first statement indexes the same collection with the same
// resolved binding. Equality remains executable and therefore does not prove
// the strict upper bound required by indexing.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — "Ineffective upper bounds guard"
//   - rules/analysis/pitfall_analysis.md — "Guards participate in pitfall reasoning"
func (b *pitfallBuilder) inspectIneffectiveUpperBoundsGuard(statement *ast.IfStatement) {
	if statement == nil || statement.Condition == nil || statement.Consequence == nil || len(statement.Consequence.Statements) == 0 {
		return
	}
	comparison, ok := statement.Condition.(*ast.InfixExpression)
	if !ok {
		return
	}
	boundary, ok := b.permissiveLengthBoundary(comparison)
	if !ok {
		return
	}
	b.reportIneffectiveLengthGuardIndexes(statement.Consequence.Statements[0], boundary, "the <= guard permits index == Len", "require index < Len before indexing")
}

// inspectIneffectiveRejectionGuards recognizes the canonical early-exit form
// `if index > values.Len { return }` followed immediately by an access. The
// continuing edge still admits equality; `>=` is required to reject it.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — "Ineffective rejection guard"
//   - rules/analysis/pitfall_analysis.md — "Required control-flow tests"
func (b *pitfallBuilder) inspectIneffectiveRejectionGuards(block *ast.BlockStatement) {
	if block == nil {
		return
	}
	for statementIndex := 0; statementIndex+1 < len(block.Statements); statementIndex++ {
		conditional, ok := block.Statements[statementIndex].(*ast.IfStatement)
		if !ok || conditional.Alternative != nil || !pitfallBlockDefinitelyExits(conditional.Consequence) {
			continue
		}
		comparison, ok := conditional.Condition.(*ast.InfixExpression)
		if !ok {
			continue
		}
		boundary, ok := b.nonRejectingLengthBoundary(comparison)
		if !ok {
			continue
		}
		b.reportIneffectiveLengthGuardIndexes(block.Statements[statementIndex+1], boundary, "the > rejection guard permits index == Len", "reject index >= Len before indexing")
	}
}

// permissiveLengthBoundary normalizes the two operand orders that admit the
// invalid equality boundary into index-versus-Len form.
//
// Rules: rules/analysis/pitfall_analysis.md — "Ineffective upper bounds guard".
func (b *pitfallBuilder) permissiveLengthBoundary(comparison *ast.InfixExpression) (pitfallLengthBoundary, bool) {
	if comparison == nil {
		return pitfallLengthBoundary{}, false
	}
	switch comparison.Operator {
	case "<=":
		return b.resolvedLengthBoundary(comparison.Left, comparison.Right, comparison.Token)
	case ">=":
		return b.resolvedLengthBoundary(comparison.Right, comparison.Left, comparison.Token)
	default:
		return pitfallLengthBoundary{}, false
	}
}

// nonRejectingLengthBoundary normalizes strict-greater rejection conditions
// whose continuing edge still includes equality with Len.
//
// Rules: rules/analysis/pitfall_analysis.md — "Ineffective rejection guard".
func (b *pitfallBuilder) nonRejectingLengthBoundary(comparison *ast.InfixExpression) (pitfallLengthBoundary, bool) {
	if comparison == nil {
		return pitfallLengthBoundary{}, false
	}
	switch comparison.Operator {
	case ">":
		return b.resolvedLengthBoundary(comparison.Left, comparison.Right, comparison.Token)
	case "<":
		return b.resolvedLengthBoundary(comparison.Right, comparison.Left, comparison.Token)
	default:
		return pitfallLengthBoundary{}, false
	}
}

// resolvedLengthBoundary requires compiler-resolved identities for both the
// guarded index expression and the receiver of compiler-known Len.
//
// Rules: rules/analysis/pitfall_analysis.md — "Semantic correlation, not spelling heuristics".
func (b *pitfallBuilder) resolvedLengthBoundary(indexExpression ast.Expression, lengthExpression ast.Expression, comparisonToken lexer.Token) (pitfallLengthBoundary, bool) {
	indexIdentity, ok := b.expressionIdentity(indexExpression)
	if !ok {
		return pitfallLengthBoundary{}, false
	}
	collectionIdentity, lengthToken, ok := b.lengthReceiver(lengthExpression)
	if !ok {
		return pitfallLengthBoundary{}, false
	}
	return pitfallLengthBoundary{
		indexIdentity:      indexIdentity,
		collectionIdentity: collectionIdentity,
		comparisonToken:    comparisonToken,
		lengthToken:        lengthToken,
	}, true
}

// reportIneffectiveLengthGuardIndexes publishes a finding only when the guarded
// binding indexes the exact collection whose Len supplied the boundary.
//
// Rules:
//   - rules/analysis/pitfall_analysis.md — "Ineffective upper bounds guard"
//   - rules/analysis/pitfall_analysis.md — "Ineffective rejection guard"
func (b *pitfallBuilder) reportIneffectiveLengthGuardIndexes(statement ast.Statement, boundary pitfallLengthBoundary, boundaryEvidence string, action string) {
	for _, index := range indexesInStatement(statement) {
		collectionIdentity, collectionOK := b.expressionIdentity(index.Left)
		indexIdentity, indexOK := b.expressionIdentity(index.Index)
		if !collectionOK || !indexOK || collectionIdentity != boundary.collectionIdentity || indexIdentity != boundary.indexIdentity {
			continue
		}
		b.add(PitfallFinding{
			Rule:           PitfallIneffectiveLengthGuard,
			Family:         PitfallBoundsAndRanges,
			Classification: PitfallProvenInvalid,
			Confidence:     PitfallConfidenceProven,
			Subject:        PitfallSubject{Expression: index.String(), Source: index.Token},
			EvidenceFor: []PitfallEvidence{
				{Strength: PitfallEvidenceProof, Fact: boundaryEvidence, Source: boundary.comparisonToken},
				{Strength: PitfallEvidenceProof, Fact: "the guarded binding indexes the same collection whose Len forms the boundary", Source: boundary.lengthToken},
			},
			OwningRule: "bounds",
			Actions: []PitfallSuggestedAction{{
				Kind: PitfallSuggestedEdit, Title: action, Source: boundary.comparisonToken,
			}},
		})
	}
}
