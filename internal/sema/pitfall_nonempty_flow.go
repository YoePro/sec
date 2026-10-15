package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// nonEmptyCollectionTypeProof consumes resolved extent and inherited contract
// facts; upper bounds and zero lower bounds cannot prove non-emptiness.
// Rules: rules/types/contracts.md — String and collection contracts, Composition;
// rules/collections/collections.md — §5.1;
// rules/analysis/pitfall_analysis.md — Final-element access requires non-empty proof.
func nonEmptyCollectionTypeProof(typ Type, fallback lexer.Token) (lexer.Token, bool) {
	if length, fixed := exactFixedArrayLength(typ); fixed && length.Sign() > 0 {
		return fallback, true
	}
	for _, contract := range typ.Contracts {
		switch c := contract.(type) {
		case MarkerContract:
			if c.Name == "notEmpty" {
				return c.Token, true
			}
		case LengthContract:
			if (c.Name == "minLen" || c.Name == "exactLen" || typ.Kind == StringType && (c.Name == "minByteLen" || c.Name == "exactByteLen")) && c.Value != nil && c.Value.Sign() > 0 {
				return c.Token, true
			}
		}
	}
	return lexer.Token{}, false
}

// mergePitfallNonEmptyProofs isolates path state and adds stronger local facts.
// Rules: rules/analysis/pitfall_analysis.md — Guards participate in pitfall reasoning.
func mergePitfallNonEmptyProofs(outer, local map[string]lexer.Token) map[string]lexer.Token {
	result := clonePitfallNonEmptyProofs(outer)
	if result == nil {
		result = map[string]lexer.Token{}
	}
	for identity, token := range local {
		result[identity] = token
	}
	return result
}

// survivingNonEmptyProofs reuses canonical guard invalidation for assignments,
// structural operations, mutable/opaque calls and nested execution. Invalidating
// before walking a statement withholds proofs when evaluation order is uncertain.
// Rules: rules/analysis/pitfall_analysis.md — Guards participate in pitfall reasoning,
// Canonical facts consumed by pitfall analysis, False-positive control.
func (b *pitfallBuilder) survivingNonEmptyProofs(proofs map[string]lexer.Token, statement ast.Statement) map[string]lexer.Token {
	result := map[string]lexer.Token{}
	for identity, token := range proofs {
		if b.guardSurvivesStatement(pitfallIndexGuard{collectionIdentity: identity}, statement) {
			result[identity] = token
		}
	}
	return result
}

// continuationNonEmptyProofs uses the owning fallthrough facts to retain the
// condition of the only continuing branch, including else exits and reversed
// or relational empty checks. Writes on the statement conservatively revoke it.
// Rules: rules/analysis/pitfall_analysis.md — Guards participate in pitfall reasoning;
// rules/control-flow/flowcontrol_if.md — §27.
func (b *pitfallBuilder) continuationNonEmptyProofs(statement ast.Statement) map[string]lexer.Token {
	conditional, ok := statement.(*ast.IfStatement)
	if !ok {
		return nil
	}
	var proofs map[string]lexer.Token
	if !b.analyzer.blockCanFallThrough(conditional.Consequence) {
		proofs = b.nonEmptyBranchProof(conditional.Condition, false)
	} else if conditional.Alternative != nil && !b.analyzer.blockCanFallThrough(conditional.Alternative) {
		proofs = b.nonEmptyBranchProof(conditional.Condition, true)
	}
	return b.survivingNonEmptyProofs(proofs, statement)
}

// clonePitfallNonEmptyProofs keeps branch-local proof state isolated across
// recursive block traversal.
// Rules: rules/analysis/pitfall_analysis.md — Guards participate in pitfall reasoning.
func clonePitfallNonEmptyProofs(proofs map[string]lexer.Token) map[string]lexer.Token {
	if len(proofs) == 0 {
		return nil
	}
	result := make(map[string]lexer.Token, len(proofs))
	for collection, token := range proofs {
		result[collection] = token
	}
	return result
}
