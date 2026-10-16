package sema

import "sec/internal/ast"

// collectionGrowthContract classifies backing growth by canonical member ID,
// never by the spelling of a user method. Bounded collection mutation does not
// grow backing; independent element/callback effects remain separate.
// Rules: rules/collections/collections.md — §§6.4,6.7,13.2,13.10,13.12,14.8,15.3-15.5;
// rules/memory/allocation.md — §§3(1),24(2)-(3).
func collectionGrowthContract(typ Type, member CompilerKnownMember) AllocationKnowledge {
	switch member.ID {
	case "CKM-DYNAMIC-ARRAY-APPEND":
		return AllocationMayAllocate
	case "CKM-LIST-APPEND", "CKM-LIST-INSERT", "CKM-SET-ADD":
		if dynamicCollection(dereferenceType(typ)) {
			return AllocationMayAllocate
		}
		return AllocationFree
	case "CKM-LIST-RESERVE", "CKM-MAP-RESERVE", "CKM-SET-RESERVE",
		"CKM-LIST-WITHCAPACITY", "CKM-MAP-WITHCAPACITY", "CKM-SET-WITHCAPACITY":
		return AllocationMayAllocate
	}
	return member.AllocationBehavior
}

// recordCollectionGrowth records a potential backing allocation after successful
// resolution. Proven empty requests need no backing, but argument evaluation
// retains its own effects. No capacity is guessed for an existing collection.
// Rules: rules/collections/collections.md — §§6.7,13.12,14.8,15.5;
// rules/memory/allocation.md — §§3(2a),24(1)-(6).
func (a *Analyzer) recordCollectionGrowth(call *ast.CallExpression) {
	member, ok := call.Callee.(*ast.MemberExpression)
	if !ok || member.Property == nil {
		return
	}
	contract, known := a.CompilerKnownMemberAt(member.Property.Token.File, member.Property.Token.Line, member.Property.Token.Column)
	if !known || contract.AllocationBehavior != AllocationMayAllocate {
		return
	}
	if (contract.Name == "Reserve" || contract.Name == "WithCapacity") && len(call.Arguments) == 1 {
		if count, constant := a.integerConstantValue(call.Arguments[0]); constant && count.Sign() == 0 {
			return
		}
	}
	if entry, rangeAppend := a.resolvedRangeAppends[call]; rangeAppend && entry.Length != nil && entry.Length.Sign() == 0 {
		return
	}
	a.recordArenaEffect(ArenaEffectCollectionGrowth, "", "", member.Property.Token, true)
}

// inferCompilerKnownMemberCall publishes allocation effects only after the
// compiler-owned member call has passed its argument and receiver checks.
// Rules: rules/memory/allocation.md — §§3(1),24(1)-(6);
// rules/collections/collections.md — §§6.5,19.
func (a *Analyzer) inferCompilerKnownMemberCall(expr *ast.CallExpression) (Type, expressionValue, bool) {
	typ, value, handled := a.inferValidatedCompilerKnownMemberCall(expr)
	if handled && typ.Kind != InvalidType {
		if a.checkDynamicArrayMutationBorrow(expr) {
			delete(a.resolvedRangeAppends, expr)
			return Type{Kind: InvalidType}, value, true
		}
		a.recordCollectionGrowth(expr)
	}
	return typ, value, handled
}
