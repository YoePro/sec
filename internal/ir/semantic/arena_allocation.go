package semantic

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/sema"
)

// buildArenaAllocation lowers Arena New[T]() and Alloc[T](count) on a
// borrowed FromBuffer Arena created in this function. The checked
// arena.new/arena.alloc yields (candidate, failed, AllocationError); the
// candidate is a dynamic-epoch reference/slice of the current domain epoch
// and is published only on the success edge as Ok, the failure edge builds
// Err. The layout of T comes from the canonical resolver, the default from
// Sema's canonical initialization plan, and the capacity from the fixed
// borrowed backing (arena.md §§ 16-31; P19 §§ 20-35, 113-114).
// Rules: rules/memory/arena.md — §§ 16–31; rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 20–35, 90–93, 113–114.
func (fb *functionBuilder) buildArenaAllocation(call *ast.CallExpression, plan sema.ResolvedArenaOperationPlan) (builtValue, error) {
	member, _ := call.Callee.(*ast.MemberExpression)
	token := member.Property.Token
	retained, known := fb.arenaBorrows[plan.InvalidationDomain]
	if !known || plan.BackingKind != "borrowed" || plan.ElementLayout == nil || plan.Reference == nil || plan.DefaultPlan == "" || plan.CapacityPlan == "" || !plan.Fallible {
		return builtValue{}, fb.unsupported("Arena "+string(plan.Operation)+" outside a FromBuffer Arena created in this function", token)
	}
	owned, stored := fb.ownedStorageOf(member.Object)
	if !stored {
		return builtValue{}, fb.unsupported("Arena "+string(plan.Operation)+" on an owner without local storage", token)
	}
	resultSema, ok := fb.owner.analyzer.ResolvedTypeOf(call)
	if !ok {
		return builtValue{}, fmt.Errorf("Arena %s has no resolved result type", plan.Operation)
	}
	resultType, err := fb.owner.internType(resultSema)
	if err != nil {
		return builtValue{}, err
	}
	candidateType, err := fb.owner.internType(plan.SuccessType)
	if err != nil {
		return builtValue{}, err
	}
	errorType, err := fb.owner.internType(plan.ErrorType)
	if err != nil {
		return builtValue{}, err
	}
	boolType, err := fb.owner.internType(sema.Type{Name: "bool", Kind: sema.BoolType})
	if err != nil {
		return builtValue{}, err
	}
	place := fb.ownedStoragePlace(owned, token)
	operands := []ValueID{place.id}
	if plan.Operation == sema.ArenaOperationAlloc {
		countSema, typed := fb.owner.analyzer.ResolvedTypeOf(call.Arguments[0])
		if !typed {
			return builtValue{}, fb.unsupported("Arena Alloc count without a resolved type", token)
		}
		countType, err := fb.owner.internType(countSema)
		if err != nil {
			return builtValue{}, err
		}
		count, err := fb.buildExpr(call.Arguments[0], countType)
		if err != nil {
			return builtValue{}, err
		}
		operands = append(operands, count.id)
	}
	stateBefore, stateAfter, numbered := fb.nextArenaState(plan.InvalidationDomain)
	if !numbered {
		return builtValue{}, fb.unsupported("Arena "+string(plan.Operation)+" without a numbered owner state", token)
	}
	loc := location(token)
	kind := OpArenaNew
	if plan.Operation == sema.ArenaOperationAlloc {
		kind = OpArenaAlloc
	}
	candidate := fb.newValue(candidateType, OwnershipMutableReference, loc)
	failed := fb.newValue(boolType, OwnershipImmediate, loc)
	allocationError := fb.newValue(errorType, OwnershipImmediate, loc)
	metadata := referenceMetadataOf(*plan.Reference)
	fb.emit(Operation{
		Kind: kind, Operands: operands, Results: []Value{candidate, failed, allocationError}, Location: loc,
		Arena: &ArenaOperationFacts{
			StateBefore: stateBefore, StateAfter: stateAfter,
			EpochBefore: uint32(plan.EpochBefore), EpochAfter: uint32(plan.EpochAfter),
			BackingKind: plan.BackingKind, GrowthKind: plan.GrowthKind, RetainedBorrow: retained,
			CapacityPlan: plan.CapacityPlan, DefaultPlan: plan.DefaultPlan,
			LayoutPlan:    arenaLayoutPlan(plan.ElementLayout),
			FailureAtomic: plan.FailureAtomic,
		},
		Backing:   arenaBackingFacts(plan, sema.BackingTransitionNone),
		Reference: metadata,
	})
	success, failure, join := fb.newBlock(), fb.newBlock(), fb.newBlock()
	failureError := fb.newValue(errorType, OwnershipImmediate, loc)
	failure.Parameters = []Value{failureError}
	joined := fb.newValue(resultType, OwnershipImmediate, loc)
	join.Parameters = []Value{joined}
	fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{failed.ID}, Successors: []BranchTarget{{Block: failure.ID, Arguments: []ValueID{allocationError.ID}}, {Block: success.ID}}, Location: loc})

	fb.current = success
	fb.recordReference(candidate.ID, metadata)
	okResult := fb.result(Operation{Kind: OpResultOk, Operands: []ValueID{candidate.ID}, Location: loc}, resultType)
	fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: join.ID, Arguments: []ValueID{okResult.id}}}, Location: loc})

	fb.current = failure
	errResult := fb.result(Operation{Kind: OpResultErr, Operands: []ValueID{failureError.ID}, Location: loc}, resultType)
	fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: join.ID, Arguments: []ValueID{errResult.id}}}, Location: loc})

	fb.current = join
	if fb.okReferences == nil {
		fb.okReferences = map[ValueID]*ReferenceMetadata{}
	}
	fb.okReferences[joined.ID] = metadata
	return builtValue{id: joined.ID, typ: resultType}, nil
}

// arenaLayoutPlan names the resolved element layout: the canonical type
// identity, its size and alignment in bytes, and the plan key it holds under.
// Rules: rules/memory/layout.md — § 47.5; rules/memory/arena.md — §§ 16–17.
func arenaLayoutPlan(layout *sema.ArenaElementLayout) string {
	return fmt.Sprintf("resolved:%s:size=%s:align=%s:plan=%s", layout.TypeIdentity, layout.SizeBytes, layout.AlignBytes, layout.PlanKey)
}

// buildArenaDefault lowers the MD-136 default creation: an owned Arena with a
// fresh domain, its initial epoch, state version 1, no backing, zero
// capacity and the provider plan frozen by Sema ("absent", or the interim
// hosted mapping pending MD-151). It allocates nothing and cannot fail.
// Rules: rules/memory/arena.md — §§ 131(1)–(5), 131(8)–(15);
// rules/mlir/packages/sec-mlir-dialect_package19.md — § 166.
func (fb *functionBuilder) buildArenaDefault(literal *ast.StructLiteral, plan sema.ResolvedArenaOperationPlan) (builtValue, error) {
	if fb.owner.maxPackage < arenaLoweringPackage {
		return builtValue{}, fb.unsupported("Arena operations require package 19", literal.Token)
	}
	if plan.Constructor != sema.ArenaDefaultConstructor || plan.InvalidationDomain == 0 || plan.EpochAfter == 0 || len(plan.Unresolved) != 0 {
		return builtValue{}, fb.unsupported("default Arena creation with unresolved plan facts", literal.Token)
	}
	arena := fb.owner.module.Types.InternArena()
	loc := location(literal.Token)
	result := fb.newValue(arena, OwnershipOwned, loc)
	fb.emit(Operation{
		Kind: OpArenaCreateDefault, Results: []Value{result}, Location: loc,
		Arena: &ArenaOperationFacts{StateAfter: uint32(plan.StateAfter), EpochAfter: uint32(plan.EpochAfter),
			BackingKind: "none", GrowthKind: plan.GrowthKind, CapacityPlan: plan.CapacityPlan, ProviderPlan: plan.ProviderPlan},
		Backing: &BackingMetadata{
			Domain: uint32(plan.InvalidationDomain), Transition: string(sema.BackingTransitionEstablishDomain),
			Origin: string(sema.StorageOriginArena), Relation: string(sema.BackingRelationOwned),
			Authority: string(sema.ReclamationNone), Stability: string(sema.AddressStabilityStable), Space: "ordinary",
		},
	})
	if fb.arenaStates == nil {
		fb.arenaStates = map[sema.InvalidationDomainID]uint32{}
	}
	fb.arenaStates[plan.InvalidationDomain] = uint32(plan.StateAfter)
	if fb.arenaDefaults == nil {
		fb.arenaDefaults = map[sema.InvalidationDomainID]bool{}
	}
	fb.arenaDefaults[plan.InvalidationDomain] = true
	return builtValue{id: result.ID, typ: arena}, nil
}
