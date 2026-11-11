package sema

import "sec/internal/ast"

// endTemporaryArenaBacking ends a direct constructor temporary's retained
// backing when legal explicit/implicit discard destroys that Arena value.
// No dependency is removed from a named owner or a source passed into a call.
// Rules: rules/memory/arena.md §§10,43,51(6–7);
// rules/control-flow/discard.md §§7,10–13; rules/memory/destruction.md §6.1.
func (a *Analyzer) endTemporaryArenaBacking(expression ast.Expression) {
	call, ok := expression.(*ast.CallExpression)
	if !ok {
		return
	}
	if typ, known := a.expressionTypes[expression]; known && typ.Name == "Arena" {
		a.endBorrowsHeldBy(arenaConstructorBorrowHolder(call))
	}
}

// retainNestedArenaBacking keeps the backing's parent-domain dependencies even
// when no source binding holds an Alloc result. Existing borrow-holder moves,
// branch joins, explicit Release and scope destruction carry/end these facts.
// This is not an exclusive borrow of the parent owner: disjoint allocations
// and owner moves preserve its ArenaDomain and remain legal.
// Rules: rules/memory/arena.md §§4.2,7(6),10,36,43–44,51(1–7).
func (a *Analyzer) retainNestedArenaBacking(call *ast.CallExpression, backing Type) {
	for _, origin := range referenceOriginAlternatives(backing) {
		if origin.Storage != StorageOriginArena || origin.Name == "" {
			continue
		}
		record := borrowRecord{Root: origin.Name, Place: Place{Root: origin.Name}, Holder: arenaConstructorBorrowHolder(call), Kind: arenaParentDependency, Token: call.Token, ArenaOwner: origin.DisplayName}
		if !containsBorrowRecord(a.borrows[origin.Name], record) {
			a.borrows[origin.Name] = append(a.borrows[origin.Name], record)
		}
	}
}

// nestedArenaDependency chooses a deterministic live child witness. Domain
// identity survives owner moves; invalidation cannot advance the epoch while
// this strong dependency lives. Stale backing is rejected before construction.
// Rules: rules/memory/arena.md §§38,44,51(4–7).
func (a *Analyzer) nestedArenaDependency(domain string) (borrowRecord, bool) {
	var first borrowRecord
	found := false
	for _, record := range a.borrows[domain] {
		if record.Kind != arenaParentDependency {
			continue
		}
		if _, moved := a.moved[record.Holder]; moved {
			continue
		}
		if !found || record.Token.File < first.Token.File || record.Token.File == first.Token.File && (record.Token.Line < first.Token.Line || record.Token.Line == first.Token.Line && record.Token.Column < first.Token.Column) {
			first, found = record, true
		}
	}
	return first, found
}

// arenaBackingDisplay keeps compiler-internal domain IDs out of repair text.
// Rules: rules/memory/arena.md §119; rules/tooling/diagnostics.md §§9–10.
func arenaBackingDisplay(record borrowRecord) string {
	if record.ArenaOwner != "" {
		return record.ArenaOwner
	}
	if record.Kind == arenaParentDependency {
		return "parent Arena storage"
	}
	return record.Root
}
