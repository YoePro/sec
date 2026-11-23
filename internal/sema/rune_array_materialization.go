package sema

import (
	"fmt"

	"sec/internal/ast"
)

// AllocationStringToRuneArray materializes string.ToRuneArray(): a fresh
// owning rune[] grown through the active allocation context.
const AllocationStringToRuneArray AllocationOperationKind = "string-to-rune-array"

// ResolvedRuneArrayMaterializationPlan is the read-only plan of one
// `text.ToRuneArray()` whose receiver is a string Place (MD-072,
// core-library.md § 5.3; package18 §§ 33, 72-73). Source and View describe
// the shared rune-coordinate view of the receiver for the call's duration;
// Owner is the compiler-internal temporary owner the runes are appended to,
// with its own storage and backing domain; Allocation is its growth plan.
// The temporary never escapes: success moves it into Ok, failure destroys
// the initialized prefix and ends/reclaims its backing (partial
// construction, § 33) before Err(CollectionError.AllocationFailed).
type ResolvedRuneArrayMaterializationPlan struct {
	Source     Place
	View       ReferenceFacts
	Owner      DynamicArrayOwner
	Allocation ResolvedAllocationOperationPlan
	ResultType Type
}

// recordRuneArrayMaterialization publishes the plan once the call has passed
// its checks. A receiver that is not a string Place (a temporary string)
// gets no plan and stays a named Semantic IR boundary.
// Rules: rules/library/core-library.md — § 5.3 (ToRuneArray, MD-072);
// rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 20-37, 72-73.
func (a *Analyzer) recordRuneArrayMaterialization(call *ast.CallExpression, member *ast.MemberExpression, contract CompilerKnownMember) {
	if a.summaryPass || contract.ID != "CKM-STRING-TORUNEARRAY" {
		return
	}
	source, ok := a.resolvePlace(member.Object)
	if !ok || dereferenceType(source.Type).Kind != StringType {
		return
	}
	resultType := a.refinedCompilerKnownType(contract.Result)
	if resultType.Kind != ResultType {
		return
	}
	array, dynamic := ResolveDynamicArrayPlan(compilerKnownDynamicArray(builtinType("rune")))
	if !dynamic {
		return
	}
	site := member.Property.Token
	key := fmt.Sprintf("rune-array-materialization:%s", referenceSiteKey(site))
	owner := DynamicArrayOwner{
		Place: Place{
			Root: "ToRuneArray.result", RootToken: site, RootID: a.placeRootID(site), RootKind: PlaceRootLocal,
			StorageIdentity: a.placeIdentities.storageIdentity(key), StorageOrigin: StorageOriginAutomatic,
			AddressSpace: AddressSpaceOrdinary, Type: array.ArrayType, Mutable: true, Addressable: true,
		},
		OwnerOrigin: StorageOriginAutomatic,
	}
	owner.OwnerStorage = owner.Place.StorageIdentity
	owner.BackingDomain = a.placeIdentities.invalidationDomain("dynamic-array-backing:" + key)
	context := a.activeAllocationContext()
	plan := ResolvedAllocationOperationPlan{
		Operation: AllocationStringToRuneArray, ArrayType: array.ArrayType, ElementType: array.ElementType,
		MayAllocate: true, Context: context, StorageOrigin: context.Origin,
		Fallible: true, FailureType: builtinType("CollectionError"), AllocationFailureType: builtinType("AllocationError"),
		Source: site, Receiver: &owner,
	}
	a.resolveAllocationIdentities(&plan, call)
	if a.allocationOperationPlans == nil {
		a.allocationOperationPlans = map[*ast.CallExpression]ResolvedAllocationOperationPlan{}
	}
	a.allocationOperationPlans[call] = plan
	if a.runeArrayMaterializations == nil {
		a.runeArrayMaterializations = map[*ast.CallExpression]ResolvedRuneArrayMaterializationPlan{}
	}
	a.runeArrayMaterializations[call] = ResolvedRuneArrayMaterializationPlan{
		Source: clonePlace(source), View: a.referenceFactsForPlace(source, false, site),
		Owner: owner, Allocation: plan, ResultType: semanticSnapshotType(resultType),
	}
}

// ResolvedRuneArrayMaterializationOf reads the plan of a ToRuneArray call;
// it allocates and infers nothing. The result is detached.
// Rules: rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 72-73.
func (a *Analyzer) ResolvedRuneArrayMaterializationOf(call *ast.CallExpression) (ResolvedRuneArrayMaterializationPlan, bool) {
	if a == nil || call == nil {
		return ResolvedRuneArrayMaterializationPlan{}, false
	}
	plan, ok := a.runeArrayMaterializations[call]
	if !ok {
		return ResolvedRuneArrayMaterializationPlan{}, false
	}
	plan.Source = clonePlace(plan.Source)
	plan.Owner.Place = clonePlace(plan.Owner.Place)
	plan.Allocation.Receiver = &plan.Owner
	return plan, true
}

// recordCoreArrayMaterializationSite remembers the allocation site of a
// string.ToByteArray / ToCharArray / ToRuneArray call. Semantic IR either
// lowers it from its materialization plan or stops at a named boundary, so
// the allocation-readiness gate leaves the site to that decision.
// Rules: rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 72-73, 133.
func (a *Analyzer) recordCoreArrayMaterializationSite(member *ast.MemberExpression, contract CompilerKnownMember) {
	switch contract.ID {
	case "CKM-STRING-TOBYTEARRAY", "CKM-STRING-TOCHARARRAY", "CKM-STRING-TORUNEARRAY":
	default:
		return
	}
	if a.coreArrayMaterializationSites == nil {
		a.coreArrayMaterializationSites = map[sourceTokenKey]bool{}
	}
	a.coreArrayMaterializationSites[sourceTokenLocation(member.Property.Token)] = true
}
