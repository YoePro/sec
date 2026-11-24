package semantic

import (
	"sec/internal/ast"
	"sec/internal/sema"
)

// arenaDependencyProof names the dependency legality Sema established: a
// program that analyzed without errors has no validity-preserving dependency
// crossing the Reset (arena.md §§ 37-38; P19 § 37). It is a provenance
// name, not a new analysis.
const arenaDependencyProof = "sema-dependency-check"

// buildArenaCall lowers the Arena operations whose plans are complete
// without a layout or provider: Arena.FromBuffer creation and Reset of a
// borrowed Arena created in the same function. Every other Arena operation
// stays a named package-19 boundary (owned creation: provider catalogs,
// MD-151; Release: P17 ownership termination; nested borrowed Arenas).
// New/Alloc lower on a FromBuffer Arena created here. Implicit destruction
// is the owner's compiler-known-resource cleanup plan.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 10, 36-42, 86-96, 110, 115;
// rules/memory/arena.md — §§ 10, 35-46.
func (fb *functionBuilder) buildArenaCall(call *ast.CallExpression) (builtValue, bool, error) {
	plan, ok := fb.owner.analyzer.ResolvedArenaOperationPlanOf(call)
	if !ok {
		return builtValue{}, false, nil
	}
	token := call.Token
	if member, ok := call.Callee.(*ast.MemberExpression); ok && member.Property != nil {
		token = member.Property.Token
	}
	if fb.owner.maxPackage < arenaLoweringPackage {
		return builtValue{}, true, fb.unsupported("Arena operations require package 19", token)
	}
	if len(plan.Unresolved) != 0 {
		return builtValue{}, true, fb.unsupported("Arena "+string(plan.Operation)+" with unresolved plan facts ("+plan.Unresolved[0]+")", token)
	}
	switch plan.Operation {
	case sema.ArenaOperationCreate:
		if plan.Constructor != "FromBuffer" {
			return builtValue{}, true, fb.unsupported("owned Arena creation "+plan.Constructor+" (provider catalogs, MD-151)", token)
		}
		value, err := fb.buildArenaFromBuffer(call, plan)
		return value, true, err
	case sema.ArenaOperationReset:
		value, err := fb.buildArenaReset(call, plan)
		return value, true, err
	case sema.ArenaOperationRelease:
		value, err := fb.buildArenaRelease(call, plan)
		return value, true, err
	case sema.ArenaOperationNew, sema.ArenaOperationAlloc:
		value, err := fb.buildArenaAllocation(call, plan)
		return value, true, err
	}
	return builtValue{}, true, fb.unsupported("Arena "+string(plan.Operation), token)
}

// nextArenaState advances the owner state version of a domain created in
// this function: before is the current version, after a fresh successor.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — § 121.
func (fb *functionBuilder) nextArenaState(domain sema.InvalidationDomainID) (uint32, uint32, bool) {
	before, known := fb.arenaStates[domain]
	if !known || before == 0 {
		return 0, 0, false
	}
	fb.arenaStates[domain] = before + 1
	return before, before + 1, true
}

// arenaBackingFacts is the storage classification of a borrowed Arena
// domain: Arena-origin storage borrowed from its external owner.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 9-12, 110.
func arenaBackingFacts(plan sema.ResolvedArenaOperationPlan, transition sema.BackingTransitionKind) *BackingMetadata {
	return &BackingMetadata{
		Domain: uint32(plan.InvalidationDomain), Transition: string(transition),
		Origin: string(sema.StorageOriginArena), Relation: string(sema.BackingRelationBorrowed),
		Authority: string(sema.ReclamationExternalOwner), Stability: string(sema.AddressStabilityStable), Space: "ordinary",
	}
}

// buildArenaFromBuffer creates a fixed Arena over the borrowed mutable byte
// slice: a fresh domain, state version 1 and its initial epoch; the slice
// borrow is retained by the Arena until it ends.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 10, 88, 110.
func (fb *functionBuilder) buildArenaFromBuffer(call *ast.CallExpression, plan sema.ResolvedArenaOperationPlan) (builtValue, error) {
	token := call.Token
	if len(call.Arguments) != 1 || len(plan.Parents) != 0 || plan.BackingKind != "borrowed" || plan.InvalidationDomain == 0 || plan.EpochAfter == 0 {
		return builtValue{}, fb.unsupported("Arena.FromBuffer of a nested or incompletely planned backing", token)
	}
	argumentType, typed := fb.owner.analyzer.ResolvedTypeOf(call.Arguments[0])
	if !typed {
		return builtValue{}, fb.unsupported("Arena.FromBuffer backing without a resolved type", token)
	}
	sliceType, err := fb.owner.internType(argumentType)
	if err != nil {
		return builtValue{}, err
	}
	backing, err := fb.buildExpr(call.Arguments[0], sliceType)
	if err != nil {
		return builtValue{}, err
	}
	facts := fb.references[backing.id]
	slice, _ := fb.owner.module.Types.Lookup(backing.typ)
	if slice.Kind != TypeMutableSlice || !typeHasKind(fb.owner.module.Types, slice.Element, TypeByte) || facts == nil || facts.Borrow == 0 {
		return builtValue{}, fb.unsupported("Arena.FromBuffer backing without a mutable byte-slice borrow", token)
	}
	arena := fb.owner.module.Types.InternArena()
	loc := location(token)
	result := fb.newValue(arena, OwnershipOwned, loc)
	fb.emit(Operation{
		Kind: OpArenaCreateBorrowed, Operands: []ValueID{backing.id}, Results: []Value{result}, Location: loc,
		Arena: &ArenaOperationFacts{StateAfter: uint32(plan.StateAfter), EpochAfter: uint32(plan.EpochAfter),
			BackingKind: "borrowed", GrowthKind: "fixed", RetainedBorrow: facts.Borrow},
		Backing: arenaBackingFacts(plan, sema.BackingTransitionEstablishDomain),
	})
	if fb.arenaBorrows == nil {
		fb.arenaBorrows = map[sema.InvalidationDomainID]uint32{}
	}
	fb.arenaBorrows[plan.InvalidationDomain] = facts.Borrow
	if fb.arenaStates == nil {
		fb.arenaStates = map[sema.InvalidationDomainID]uint32{}
	}
	fb.arenaStates[plan.InvalidationDomain] = uint32(plan.StateAfter)
	return builtValue{id: result.ID, typ: arena}, nil
}

// buildArenaReset advances the epoch of an Arena held in a local owner
// created in this function, through its writable Place.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 36-42, 94, 115.
func (fb *functionBuilder) buildArenaReset(call *ast.CallExpression, plan sema.ResolvedArenaOperationPlan) (builtValue, error) {
	member, _ := call.Callee.(*ast.MemberExpression)
	token := member.Property.Token
	retained, known := fb.arenaBorrows[plan.InvalidationDomain]
	backing := "borrowed"
	if fb.arenaDefaults[plan.InvalidationDomain] && plan.BackingKind == "none" {
		// MD-136 § 131(7): Reset of an empty default reclaims nothing.
		known, backing = true, "none"
	}
	if !known || plan.BackingKind != backing || plan.StateBefore == 0 || plan.EpochBefore == 0 || plan.EpochAfter == plan.EpochBefore {
		return builtValue{}, fb.unsupported("Arena Reset of an Arena not created by FromBuffer or default in this function", token)
	}
	owned, stored := fb.ownedStorageOf(member.Object)
	if !stored {
		return builtValue{}, fb.unsupported("Arena Reset of an owner without local storage", token)
	}
	stateBefore, stateAfter, numbered := fb.nextArenaState(plan.InvalidationDomain)
	if !numbered {
		return builtValue{}, fb.unsupported("Arena Reset without a numbered owner state", token)
	}
	place := fb.ownedStoragePlace(owned, token)
	fb.emit(Operation{
		Kind: OpArenaReset, Operands: []ValueID{place.id}, Location: location(token),
		Arena: &ArenaOperationFacts{StateBefore: stateBefore, StateAfter: stateAfter,
			EpochBefore: uint32(plan.EpochBefore), EpochAfter: uint32(plan.EpochAfter),
			BackingKind: backing, GrowthKind: plan.GrowthKind, RetainedBorrow: retained, DependencyProof: arenaDependencyProof,
			ProviderPlan: plan.ProviderPlan},
		Backing: arenaResetBacking(plan, backing),
	})
	void := fb.owner.module.Types.Intern(Type{Kind: TypeVoid})
	return builtValue{typ: void}, nil
}

// arenaResetBacking is the storage classification of a Reset: borrowed
// backing for FromBuffer, an owned domain without backing for the MD-136
// default.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 115, 166.
func arenaResetBacking(plan sema.ResolvedArenaOperationPlan, backing string) *BackingMetadata {
	facts := arenaBackingFacts(plan, sema.BackingTransitionAdvanceEpoch)
	if backing == "none" {
		facts.Relation, facts.Authority = string(sema.BackingRelationOwned), string(sema.ReclamationNone)
	}
	return facts
}

// buildArenaRelease lowers the explicit terminal Release of an Arena owner
// created by FromBuffer or the MD-136 default in this function: the owner
// moves out of its storage, arena.release consumes it and ends its domain
// (ending the backing loan, or reclaiming nothing for the empty default),
// and the owner's cleanup registration is cancelled so implicit destruction
// never repeats it.
// Rules: rules/memory/arena.md — §§ 43–46, 131(7);
// rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 43–48, 116;
// rules/mlir/packages/sec-mlir-dialect_package17.md — §§ 38, 73, 80.
func (fb *functionBuilder) buildArenaRelease(call *ast.CallExpression, plan sema.ResolvedArenaOperationPlan) (builtValue, error) {
	member, _ := call.Callee.(*ast.MemberExpression)
	token := member.Property.Token
	identifier, named := member.Object.(*ast.Identifier)
	retained, known := fb.arenaBorrows[plan.InvalidationDomain]
	backing := "borrowed"
	if fb.arenaDefaults[plan.InvalidationDomain] && plan.BackingKind == "none" {
		known, backing = true, "none"
	}
	if !named || !known || plan.BackingKind != backing || plan.EpochBefore == 0 || plan.EpochAfter != 0 {
		return builtValue{}, fb.unsupported("Arena Release of an Arena not created by FromBuffer or default in this function", token)
	}
	fact, bound := fb.owner.analyzer.ResolvedBindingOf(identifier)
	if !bound {
		return builtValue{}, fb.unsupported("Arena Release of an owner without a resolved binding", token)
	}
	owned, stored := fb.ownedStorages[fact.ID]
	action, tracked := fb.trackedActions[fact.ID]
	if !stored || !tracked {
		return builtValue{}, fb.unsupported("Arena Release of an owner without tracked local storage", token)
	}
	if len(fb.owner.analyzer.ResolvedOwnedCleanupCancellationsAt(call.Token.File, call.Token.Line, call.Token.Column)) == 0 {
		return builtValue{}, fb.unsupported("Arena Release without a resolved cleanup cancellation", token)
	}
	stateBefore, stateAfter, numbered := fb.nextArenaState(plan.InvalidationDomain)
	if !numbered {
		return builtValue{}, fb.unsupported("Arena Release without a numbered owner state", token)
	}
	loc := location(token)
	place := fb.ownedStoragePlace(owned, token)
	arena := fb.newValue(owned.typ, OwnershipOwned, loc)
	fb.emit(Operation{Kind: OpOwnMoveFromPlace, Operands: []ValueID{place.id}, Results: []Value{arena}, Ownership: &OwnershipFacts{}, Location: loc})
	facts := arenaBackingFacts(plan, sema.BackingTransitionEndDomain)
	if backing == "none" {
		facts.Relation, facts.Authority = string(sema.BackingRelationOwned), string(sema.ReclamationNone)
	}
	fb.emit(Operation{
		Kind: OpArenaRelease, Operands: []ValueID{arena.ID}, Location: loc,
		Arena: &ArenaOperationFacts{StateBefore: stateBefore, StateAfter: stateAfter,
			EpochBefore: uint32(plan.EpochBefore), BackingKind: backing, GrowthKind: plan.GrowthKind,
			RetainedBorrow: retained, DependencyProof: arenaDependencyProof, ProviderPlan: plan.ProviderPlan},
		Backing: facts,
	})
	fb.emitAll(fb.ownedCancels(fact.ID, action, "released", false, loc))
	void := fb.owner.module.Types.Intern(Type{Kind: TypeVoid})
	return builtValue{typ: void}, nil
}
