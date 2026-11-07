package sema

import (
	"fmt"
	"math/big"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// AllocationOperationKind names a producing operation that may create or
// grow dynamic backing storage.
type AllocationOperationKind string

const (
	// AllocationDynamicArrayAppend appends one element (`values.Append(x)`).
	AllocationDynamicArrayAppend AllocationOperationKind = "dynamic-array-append"
	// AllocationDynamicArrayAppendRange appends a compact range segment
	// (`values.Append(1..9)`) as one operation.
	AllocationDynamicArrayAppendRange AllocationOperationKind = "dynamic-array-append-range"
)

// ResolvedAllocationOperationPlan is the read-only allocation fact of one
// allocation-capable producing operation (P18-06). It consumes the canonical
// allocation context of the site; it selects no allocator and adds no
// dynamic-array-only allocator model.
//
// In Sec 0.1 `Append` is the only producer that can give an owning `T[]`
// backing storage: nonempty `T[]` literals are not defined (S1155), `T[]` has
// no WithCapacity/Reserve, and the core byte[]/rune[] conversions are owned by
// their own plans (MD-072). A failure is public `CollectionError`; the
// allocation layer underneath reports `AllocationError`, never null, a panic
// or silent shortening. A missing context is rejected by the reachable-graph
// policy (S1108 under noalloc), not by this fact.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 15–19, 56–57
//   - rules/collections/collections.md — §§ 6.4, 6.7, 5.6a
//   - rules/memory/allocation.md — §§ 17(7)–(10), 22(4)
type ResolvedAllocationOperationPlan struct {
	Operation   AllocationOperationKind
	ArrayType   Type
	ElementType Type
	// Count is the number of logical elements the operation adds when
	// compile-time known; growth beyond it is internal and not observable.
	Count *big.Int
	// MayAllocate is false only when the operation provably adds nothing
	// (an empty range), so no backing is created or grown.
	MayAllocate bool
	Context     AllocationContext
	// StorageOrigin is the origin of new backing: the context's origin.
	StorageOrigin         StorageOrigin
	Fallible              bool
	FailureType           Type
	AllocationFailureType Type
	// Receiver is the owner whose backing the operation establishes or
	// grows, when the receiver is a published Place.
	Receiver *DynamicArrayOwner
	Source   lexer.Token

	// P18-07 identities and relations (package18 §§ 20–24, 31). Allocation
	// and the backing relation exist only when the operation may allocate;
	// an empty range creates no fake allocation identity (§ 22).
	Allocation           AllocationIdentityID
	ContextIdentity      AllocationContextID
	BackingRelation      BackingRelation
	ReclamationAuthority ReclamationAuthority
	// ReclamationPlan pairs the allocation with its exact reclamation; it is
	// set only for OwningValue authority. Arena-backed bytes are reclaimed
	// by Arena policy, never individually by the array (§§ 20, 30–31).
	ReclamationPlan  ReclamationPlanID
	AddressStability AddressStability
	MemorySpace      AddressSpaceID
	// Transition is the backing transition the operation applies (P18-08).
	Transition ResolvedBackingTransitionPlan
}

// ResolvedBackingTransitionPlan is the resolved storage transition of one
// structural dynamic-array operation (P18-08; package18 §§ 25–31, 36–37,
// 66–67). Whether backing exists is runtime state, so the plan names the
// transition for each case:
//   - an owner without backing establishes its backing domain (§ 25);
//   - an append within capacity changes nothing (§§ 26–27, 35);
//   - growth relocates while preserving the logical backing domain, which
//     advances its epoch and makes every element reference and slice of the
//     previous epoch stale (§ 28); the logical domain identity is the
//     owner's BackingDomain, so replace-domain is not used (§ 29);
//   - failure leaves the owner and its backing unchanged (§ 37).
type ResolvedBackingTransitionPlan struct {
	Domain                       InvalidationDomainID
	WhenAbsent                   BackingTransitionKind
	WithinCapacity               BackingTransitionKind
	Growth                       BackingTransitionKind
	OnFailure                    BackingTransitionKind
	ReclaimsRelocatedBacking     bool
	InvalidatesElementReferences bool
}

// recordDynamicArrayAllocation publishes the plan of a resolved Append on an
// owning dynamic array.
func (a *Analyzer) recordDynamicArrayAllocation(call *ast.CallExpression, member *ast.MemberExpression, contract CompilerKnownMember) {
	if a.summaryPass || contract.ID != "CKM-DYNAMIC-ARRAY-APPEND" {
		return
	}
	receiverType, ok := a.expressionTypes[member.Object]
	if !ok {
		return
	}
	array, dynamic := ResolveDynamicArrayPlan(dereferenceType(receiverType))
	if !dynamic {
		return
	}
	context := a.activeAllocationContext()
	plan := ResolvedAllocationOperationPlan{
		Operation: AllocationDynamicArrayAppend, ArrayType: array.ArrayType, ElementType: array.ElementType,
		Count: big.NewInt(1), MayAllocate: true,
		Context: context, StorageOrigin: context.Origin,
		Fallible: true, FailureType: builtinType("CollectionError"), AllocationFailureType: builtinType("AllocationError"),
		Source: member.Property.Token,
	}
	if entry, rangeAppend := a.resolvedRangeAppends[call]; rangeAppend {
		plan.Operation, plan.Count = AllocationDynamicArrayAppendRange, nil
		if entry.Length != nil {
			plan.Count = new(big.Int).Set(entry.Length)
			plan.MayAllocate = entry.Length.Sign() != 0
		}
	}
	if owner, held := a.dynamicArrayOwners[member.Object]; held {
		owner.Place = clonePlace(owner.Place)
		plan.Receiver = &owner
	}
	a.resolveAllocationIdentities(&plan, call)
	if a.allocationOperationPlans == nil {
		a.allocationOperationPlans = map[*ast.CallExpression]ResolvedAllocationOperationPlan{}
	}
	a.allocationOperationPlans[call] = plan
}

// ResolvedAllocationPlanOf reads the allocation plan of a producing
// operation (package18 § 57), detached; it allocates and infers nothing.
func (a *Analyzer) ResolvedAllocationPlanOf(expr ast.Expression) (ResolvedAllocationOperationPlan, bool) {
	call, ok := expr.(*ast.CallExpression)
	if a == nil || !ok {
		return ResolvedAllocationOperationPlan{}, false
	}
	plan, ok := a.allocationOperationPlans[call]
	if !ok {
		return ResolvedAllocationOperationPlan{}, false
	}
	if plan.Count != nil {
		plan.Count = new(big.Int).Set(plan.Count)
	}
	if plan.Receiver != nil {
		owner := *plan.Receiver
		owner.Place = clonePlace(owner.Place)
		plan.Receiver = &owner
	}
	return plan, true
}

// resolveAllocationIdentities allocates the P18-07 identities of an
// allocation-capable operation when the plan is recorded, never when it is
// queried, and selects its backing relation, reclamation and transition.
func (a *Analyzer) resolveAllocationIdentities(plan *ResolvedAllocationOperationPlan, call *ast.CallExpression) {
	plan.ContextIdentity = a.placeIdentities.allocationContext("allocation-context:" + plan.Context.Profile + ":" + string(plan.Context.Origin))
	plan.Transition = ResolvedBackingTransitionPlan{
		WhenAbsent: BackingTransitionNone, WithinCapacity: BackingTransitionNone,
		Growth: BackingTransitionNone, OnFailure: BackingTransitionNone,
	}
	if plan.Receiver != nil {
		plan.Transition.Domain = plan.Receiver.BackingDomain
	}
	if !plan.MayAllocate {
		return
	}
	token := expressionToken(call)
	key := fmt.Sprintf("dynamic-array-allocation:%s:%d:%d", token.File, token.Line, token.Column)
	plan.Allocation = a.placeIdentities.allocation(key)
	plan.BackingRelation = BackingRelationOwned
	plan.ReclamationAuthority = reclamationAuthorityOf(plan.StorageOrigin)
	if plan.ReclamationAuthority == ReclamationOwningValue {
		plan.ReclamationPlan = a.placeIdentities.reclamation("reclamation:" + key)
	}
	plan.AddressStability = AddressStabilityMovable
	plan.MemorySpace = AddressSpaceOrdinary
	plan.Transition.WhenAbsent = BackingTransitionEstablishDomain
	plan.Transition.Growth = BackingTransitionAdvanceEpoch
	plan.Transition.ReclaimsRelocatedBacking = plan.ReclamationAuthority == ReclamationOwningValue
	plan.Transition.InvalidatesElementReferences = true
}
