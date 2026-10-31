package sema

import "fmt"

// TypePackReferenceParameterFactsOf instantiates one reference parameter grant
// from its template. Each expanded position has its own borrow/lifetime identity;
// origin storage remains conservative until element/return-origin expansion.
// The index is relative to the pack, not the source or IR fixed prefix.
// Rules: rules/declarations/functions.md §38A(3)(2–5),(8–9);
// rules/compiler/semantic_ir.md §27A(1–2); rules/memory/borrowing.md §11.
func (a *Analyzer) TypePackReferenceParameterFactsOf(param FunctionParameter, position int) (ReferenceFacts, bool) {
	facts, ok := a.ReferenceParameterFactsOf(param)
	if !ok || position < 0 || facts.Referent.Kind != GenericTypePackType || param.Type.Kind != ReferenceType || param.Type.Element == nil || facts.MutableAuthority != param.Type.ReferenceMutable {
		return ReferenceFacts{}, false
	}
	key := fmt.Sprintf("type-pack-parameter:%s:%d:%d:%d", param.Token.File, param.Token.Line, param.Token.Column, position)
	facts.Borrow = a.placeIdentities.borrow(key)
	facts.Lifetime = a.placeIdentities.lifetime(key)
	facts.Referent = semanticSnapshotType(*param.Type.Element)
	return facts, true
}
