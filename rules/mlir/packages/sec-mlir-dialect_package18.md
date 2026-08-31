# Sec MLIR Program - Implementation Package 18

## Package status

Implementation package for the Sec compiler.

Package ID: `SEC-MLIR-P18`  
Package title: `Owning Dynamic Array Semantic Value Representation`  
Repository: `https://github.com/YoePro/sec`  
Repository branch: `main`  
Repository sync commit used for this package: `152c772`  
Local predecessors: `SEC-MLIR-P13`, `SEC-MLIR-P14`, `SEC-MLIR-P15`, `SEC-MLIR-P16`, `SEC-MLIR-P17`  
Repository sync date: `2026-08-10`  
Semantic IR version before package: `1`  
Semantic IR version after package: `1`  
Sec MLIR dialect schema before package: `13`  
Sec MLIR dialect schema after package: `14`  
Sec MLIR lowering specification before package: `13`  
Sec MLIR lowering specification after package: `14`

Package 18 introduces canonical Semantic IR and high-level Sec MLIR
representation for the owning runtime-sized array/sequence type:

```text
T[]
```

The package establishes:

```text
move-only owning descriptor semantics
non-allocating empty default
runtime length
internal capacity
owned backing-storage facts
allocation context
allocation/reclamation pairing
storage/invalidation domains
relocation and epoch transitions
element initialization state
deterministic destruction
index places
slice borrowing
fallible allocation
internal growth primitives
```

Package 18 consumes the public `T[]` surface fixed by `collections.md`; it does
not invent names beyond that surface. The canonical properties are `Len`,
`IsEmpty`, unsafe `Ptr`, and `SizeOf`. The canonical methods are `Append`,
`Clear`, `RemoveAt`, and `ToString`. `Push`, `Reserve`, `Resize`, `Capacity`,
`AsSlice`, `AsMutableSlice`, and `Clone` are not added.

---

# 1. Normative authority

Implementation follows:

```text
rules/collections/collections.md
rules/memory/layout.md
rules/memory/storage.md
rules/memory/allocation.md
rules/types/default_values.md
rules/library/core-library.md
rules/memory/reference_model.md
rules/memory/ownership.md
rules/memory/copy_move.md
rules/memory/destruction.md
    ↓
local P13-P17 normative amendments
    ↓
rules/compiler/semantic_ir.txt
rules/mlir/sec_mlir.md
rules/mlir/sec_mlir_dialect.md
rules/mlir/sec_mlir_lowering.md
    ↓
implementation package
    ↓
implementation
```

Before implementation:

1. apply `sec_dynamic_array_sync_package18.md`;
2. apply `sec_semantic_ir_dynamic_array_package18.md` to
   `rules/compiler/semantic_ir.txt`;
3. update `rules/mlir/sec_mlir_dialect.md` with
   `sec_mlir_dialect_package18.md`;
4. update `rules/mlir/sec_mlir_lowering.md` with
   `sec_mlir_lowering_package18.md`.

The package's C++/MLIR implementation must not begin until these prerequisite
documents and predecessor packages are present. Recording the public surface in
this package does not imply that a runtime or physical allocator ABI exists.

No new source syntax is introduced.

---

# 2. Repository and local predecessor rule

GitHub `main` remains:

```text
152c772
```

P16 and P17 are available locally but are not yet represented by a newer GitHub
sync commit.

P18 assumes the local semantics from:

```text
P13 structs
P14 fixed arrays
P15 places/direct references
P16 slices
P17 ownership/destruction/cleanup
```

If those packages are merged under a newer HEAD before implementation, Codex
must report the newer HEAD and verify semantic equivalence.

---

# 3. Wide builtin invariant

These remain active Sec builtin types:

```text
int128
int256
uint128
uint256
decimal128
```

They may be dynamic-array element types.

P18 does not change their status.

---

# 4. Canonical source type

An owning runtime-sized array/sequence uses:

```sec
T[]
```

It is distinct from:

```sec
T[N]
ref T[]
ref mut T[]
list[T]
```

`T[]` is a sized owning descriptor type.

Runtime length is not part of type identity.

---

# 5. Ownership classification

P18 locks:

```text
T[] -> MoveOnly
```

for all element types.

Reason:

```text
shallow copy would duplicate unique ownership/reclamation responsibility
deep copy may require allocation
implicit semantic copy is forbidden from hiding allocation/failure
```

Element copyability does not make the owning descriptor copyable.

---

# 6. Explicit duplication boundary

P18 defines no implicit or ordinary copy of `T[]`.

A future explicit duplication operation may:

```text
allocate
copy elements
return Result
```

but must be named and semantically explicit.

P18 does not choose that public API.

---

# 7. Empty owning default

`T[]` has a canonical non-allocating empty default:

```text
length = 0
capacity = 0
backing storage = none
initialized elements = none
allocation identity = none
```

This default exists independently of whether `T` is defaultable.

No element is constructed.

No dynamic allocation occurs.

---

# 8. Mutable default declaration

This is valid:

```sec
let mut values: byte[]
```

It creates the canonical empty owning value.

This remains invalid because immutable bindings require explicit initialization:

```sec
let values: byte[]
```

The older array rulebook example must be synchronized.

---

# 9. Empty destruction

Destroying the canonical empty owner:

```text
destroys no elements
reclaims no backing allocation
ends no non-existent allocation domain
```

The type remains non-trivially destructible in general because another value of
the same type may own backing storage.

Optimization may remove a destructor when emptiness is proven.

---

# 10. Dynamic-array semantic facts

Recommended:

```go
type DynamicArrayFacts struct {
    ElementType          TypeID
    Length               ValueID
    Capacity             ValueID

    Backing              BackingRelation
    StorageIdentity      StorageIdentityID
    AllocationIdentity   AllocationIdentityID
    AllocationContext    AllocationContextID
    ReclamationAuthority ReclamationAuthority
    ReclamationPlan      ReclamationPlanID

    InvalidationDomain   InvalidationDomainID
    EpochDependency      EpochDependencyID

    AddressStability     AddressStability
    RelocationClass      RelocationClass
    MemorySpace          MemorySpaceID

    InitializedPrefix    ValueID
}
```

Exact implementation layout may differ.

---

# 11. Dynamic-array value invariant

Every readable owning array satisfies:

```text
0 <= length <= capacity
initialized element indexes are exactly [0, length)
indexes [length, capacity) are not live T objects
```

Safe code cannot observe uninitialized capacity slots.

---

# 12. Capacity is compiler-internal in P18

Capacity is required semantic state for growth/reallocation.

It is not exposed as a source member by P18.

The current source rules explicitly provide no array/slice `capacity` member.

Do not add one.

---

# 13. Length operations

Owning `T[]` supports the existing intrinsic member:

```sec
values.Len
```

with result:

```text
uint
```

and compiler-known:

```sec
len(values)
```

with result:

```text
int
```

P18 keeps those operations distinct.

---

# 14. `len(T[])` representability gap

The same outstanding issue identified in P16 applies:

```text
.Len -> uint
len(...) -> int
```

The current language rules do not define what happens when runtime length fits
`uint` but not `int`.

P18 therefore:

```text
fully supports .Len
emits len(T[]) -> int only when representability is proven
never silently narrows
```

No truncation, wrapping, saturation, implicit panic, or silent maximum-length
reduction is invented.

---

# 15. Canonical public surface and internal growth

`collections.md` defines `Append`, `Clear`, `RemoveAt`, and `ToString` for
`T[]`. P18 must reserve and lower exactly those identities when their frontend
and allocation prerequisites are available. It must not expose its internal
capacity or growth primitives as public `Push`, `Reserve`, `Resize`,
`Capacity`, `AsSlice`, `AsMutableSlice`, or `Clone` members.

The compiler/core-internal primitives below implement the storage transitions
needed by the approved surface; they are not separately callable source APIs.

`Append(value)` first validates/prepares input ownership, then establishes any
required growth transaction, initializes the new logical slot, and increments
`Len` last. Failure preserves both the original owner and input ownership. The
allocation plan remains P18/P19-owned; public structural/resource failures are
reported as `CollectionError`, never an allocation panic.

`Clear()` destroys live elements in reverse index order, sets `Len` to zero,
retains reusable backing, and does not allocate. The retained storage domain
remains live.

`RemoveAt(index) -> Option[T]` is explicit owning extraction. A valid index
moves out the element, shifts later live elements left, decrements `Len`, leaves
the former final slot uninitialized, and returns `Some(value)`. An invalid index
returns `None` without mutation. It never allocates or introduces holes: the
initialized prefix remains `[0, Len)`.

All three mutations obey active-borrow and backing-domain invalidation rules.
Slices continue to use only `ref values[..]` and `ref mut values[..]`.

---

# 16. Allocation is explicit through the producing operation

Creating non-empty owned backing storage is allocation-capable.

Allocation must be explicit in Semantic IR.

The selected allocation context may be implicit in source according to
`allocation.md`.

No copy, move, return, escape, or borrow may introduce backing allocation.

---

# 17. Allocation failure

A potentially failing dynamic-array backing allocation uses:

```text
AllocationError
```

through ordinary `Result`/checked control flow.

Minimum error values remain:

```text
OutOfMemory
Unsupported
InvalidSize
InvalidAlignment
```

Allocation must not:

```text
return null
panic
silently shorten
silently fall back to invalid storage
```

unless a later explicitly named operation defines different behavior.

---

# 18. No-allocation profile

The canonical empty default remains valid in a no-allocation profile.

An operation that requires dynamic backing allocation is a compile-time error
when no valid allocation context exists.

The type `T[]` itself remains a valid source type.

---

# 19. Default allocation context

When allocation is permitted, P18 consumes the allocation context selected by
the canonical allocation rules.

The default model may use a compiler-selected Arena.

P18 does not assume one universal heap.

---

# 20. Value ownership versus backing reclamation

An owning `T[]` always owns the live element values.

It may or may not individually reclaim the bytes containing them.

Examples:

```text
individually allocator-backed:
    element ownership = dynamic array
    reclamation authority = OwningValue

arena-backed:
    element ownership = dynamic array
    reclamation authority = Arena
```

Destruction and reclamation remain distinct.

---

# 21. Allocation identity

P18 introduces/reuses:

```go
type AllocationIdentityID uint32
type AllocationContextID uint32
type ReclamationPlanID uint32
```

These are semantic compiler identities or references to canonical plan data.

They are not numeric addresses.

Runtime representation is deferred.

---

# 22. Empty backing relation

The canonical empty owner has:

```text
BackingRelation = None
ReclamationAuthority = None
```

until backing storage is established.

No fake allocation identity is created.

---

# 23. Backing allocation relation

Successful individually reclaimable backing allocation normally has:

```text
StorageOrigin = AllocatorBacked
BackingRelation = Owned
ReclamationAuthority = OwningValue
```

Arena-backed storage uses the canonical Arena storage/reclamation facts rather
than pretending individual bytes are owner-reclaimable.

---

# 24. Collection object versus backing domain

P18 distinguishes:

```text
dynamic-array value/object identity
backing-storage invalidation domain
allocation identity
```

They are not one identity.

Moving the owning descriptor transfers value ownership.

It does not inherently relocate backing storage or advance backing epoch.

---

# 25. Initial backing establishment

Successful first backing allocation establishes the backing invalidation domain:

```text
Absent -> Live(initial epoch)
```

using the canonical storage transition model.

Failure establishes no backing domain.

---

# 26. In-place mutation

Replacing an existing element in place:

```text
does not change backing domain
does not advance backing epoch
```

ordinary ownership/destruction semantics still apply to the element.

---

# 27. In-place growth without relocation

An internal approved growth operation that increases usable capacity without
invalidating existing element locations:

```text
does not advance backing epoch
```

It remains an allocation/storage effect.

---

# 28. Reallocation preserving logical backing identity

If backing storage relocates while preserving the logical collection backing
domain:

```text
AdvanceEpoch
```

is emitted.

Every dependency on the previous backing epoch becomes stale.

Direct references/slices that would cross the mutation must already have been
rejected by borrowing/relocation analysis.

---

# 29. Logical backing replacement

A representation may instead model growth as:

```text
EndDomain(old)
EstablishDomain(new)
```

when logical backing identity changes.

The choice is plan/operation-defined and must be explicit before physical
lowering.

---

# 30. Destruction backing transition

Destroying an owner with live backing:

```text
1. destroy initialized elements in reverse index order
2. end the dynamic-array backing domain
3. reclaim bytes when the reclamation plan gives OwningValue authority
```

Arena-backed bytes are not individually reclaimed by the array.

---

# 31. Reclamation pairing

Every backing allocation with owner reclamation authority records the exact
matching reclamation plan.

The compiler must not infer deallocator from:

```text
pointer type
address
element type
allocation size alone
```

---

# 32. Element destruction order

Owning dynamic array destruction destroys:

```text
length - 1
length - 2
...
0
```

for initialized non-trivial elements.

Large/runtime-sized destruction lowers through a high-level loop/plan, not
unrolled element operations.

---

# 33. Partial construction

A dynamic array under compiler/core-internal construction remains valid by
maintaining:

```text
length == initialized prefix count
```

Capacity beyond `length` is raw/uninitialized typed capacity and is never
exposed as live T values.

If later construction fails:

```text
destroy [0, length) in reverse order
end/reclaim backing according to plan
propagate the original failure
```

---

# 34. Safe empty-with-capacity state

An internal allocation may successfully create:

```text
length = 0
capacity > 0
```

with no initialized T elements.

This is a valid owned internal array state.

It is not source-observable through `capacity`.

---

# 35. Internal element append primitive

P18 may use an internal primitive that:

```text
requires length < capacity
consumes/constructs one prepared T
initializes slot at old length
increments length after successful initialization
```

The primitive itself does not allocate.

It is not a public `Push` API.

---

# 36. Internal growth primitive

P18 may use an internal growth primitive that:

```text
requests minimum capacity
is fallible when allocation may fail
leaves the old owner unchanged on failure
preserves element ownership
moves/relocates physical storage only as storage semantics
reports/records the required invalidation transition
```

It is not a public `Reserve` API.

---

# 37. Transactional growth

When backing growth is fallible:

```text
failure:
    original array remains valid and unchanged

success:
    updated backing facts become active
    old backing is ended/reclaimed when applicable
```

No element ownership is lost on failure.

---

# 38. Direct references during structural mutation

A structural mutation that may relocate/rebind backing storage is invalid while
a conflicting direct:

```text
ref element
ref mut element
ref slice
ref mut slice
```

dependency is live.

P18 does not add runtime borrow checking.

Sema/P15/P16 facts enforce this statically where direct references are involved.

---

# 39. Stable handles boundary

P18 does not introduce stable or weak handles.

Future handles that deliberately survive reallocation will consume the same
backing invalidation/epoch events.

---

# 40. Dynamic-array indexing

Indexing an owning `T[]` follows the canonical array/slice index rules.

Valid index:

```text
0 <= index < runtime length
```

Indexing first forms an element Place.

Use context then selects:

```text
copy/semantic-copy read
write/replacement
shared borrow
mutable borrow
```

P17/P15 own those actions.

---

# 41. `ResolvedDynamicArrayIndexPlan`

P18 may reuse/generalize P14/P16 index plan types.

Do not create duplicate synonymous enums.

Required facts:

```text
element type
index type/signedness
constant index when known
proven-safe/runtime-check
proof provenance
use kind
ownership transfer action
IndexError type
owner/backing identity
```

---

# 42. Runtime index failure

Ordinary unproven owning-array indexing uses:

```text
panic.bounds
operation = dynamic-array-index
```

Fallible:

```sec
try values[index]
```

uses:

```text
IndexError.OutOfBounds
```

when the source language/context permits fallible indexing.

---

# 43. Element move-out remains restricted

P17 intentionally deferred runtime-indexed fixed-array partial move.

P18 likewise does not introduce:

```text
let resource := values[index]
```

as a move-out of a move-only element.

Borrow the element instead until a dedicated dynamic-index partial-ownership
model exists.

---

# 44. Element replacement

Assignment through mutable owning-array indexing:

```sec
values[index] = replacement
```

uses:

```text
element Place
P17 ReplacePlaceOp
```

The replacement value is fully prepared before old element destruction.

Backing domain does not change.

---

# 45. Element borrowing

Shared:

```sec
ref values[index]
```

Mutable:

```sec
ref mut values[index]
```

uses P15 reference borrowing from the dynamic-array element Place.

The reference depends on the current backing-storage incarnation.

---

# 46. Slice borrowing from owning `T[]`

P18 removes P16's temporary dynamic-owner source restriction.

Explicit:

```sec
ref values[..]
ref mut values[start..<end]
```

may borrow a slice from an owning dynamic array.

No element copy.

No allocation.

---

# 47. Slice source facts

A slice borrowed from `T[]` inherits:

```text
backing storage identity
current backing invalidation domain/epoch
normalized range
BorrowID/LifetimeID
relocation dependency
address space
```

The owner retains element ownership.

---

# 48. Slice and structural mutation conflict

A live slice prevents structural owner mutation that may invalidate its backing
dependency.

Element replacement in place may be allowed according to shared/mutable borrow
authority.

Relocating growth is not allowed across the live direct slice.

---

# 49. `T[]` length property

Add Semantic IR:

```text
DynamicArrayLengthOp -> uint
```

Pure value observation.

No allocation.

No consumption.

---

# 50. `len(T[])`

Add:

```text
DynamicArrayLengthIntOp -> int
```

with the same no-truncation representability guard as P16.

---

# 51. Public capacity remains absent

P18 may represent internal capacity with:

```text
DynamicArrayCapacityInternalOp
```

for compiler/core lowering.

That operation is not exposed through ordinary source member lookup.

---

# 52. Dynamic-array type in Semantic IR

Add canonical type kind:

```text
DynamicArrayType(Element TypeID)
```

Source identity:

```text
T[]
```

Length/capacity/allocation facts are value facts, not type parameters.

---

# 53. Dynamic-array move

Whole owning array movement uses P17:

```text
MoveValueOp
MoveFromPlaceOp
```

No dedicated ownership semantics are invented.

Moving the descriptor transfers:

```text
element ownership
backing relation
reclamation responsibility
storage/invalidation facts
cleanup responsibility
```

It does not necessarily relocate backing bytes.

---

# 54. Dynamic-array destruction plan

Every concrete `T[]` has a non-trivial P17 destruction plan:

```text
DestroyDynamicArray
```

even when `T` itself is trivially destructible.

Reason:

```text
the value may own or depend on backing reclamation
```

The empty no-backing case is a runtime/analysis no-op specialization.

---

# 55. `ResolvedDynamicArrayPlan`

Recommended read-only Sema/analysis fact:

```go
type ResolvedDynamicArrayPlan struct {
    ArrayType             Type
    ElementType           Type

    AllocationCapable     bool
    AllocationContext     AllocationContext
    StorageOrigin         StorageOrigin
    BackingRelation       BackingRelation
    ReclamationAuthority  ReclamationAuthority
    ReclamationPlan       ReclamationPlan

    StorageIdentity       StorageIdentity
    InvalidationDomain    InvalidationDomain
    EpochPolicy           EpochPolicy
    RelocationClass       RelocationClass
    MemorySpace           MemorySpace

    CopyClass             CopyClassification
    DestructionPlan       DestructionPlanID
}
```

---

# 56. Allocation operation plan

For each compiler/core allocation-capable producer, record:

```text
requested capacity/count
element type
alignment
allocation context
fallibility
AllocationError
storage origin
backing relation
reclamation plan
memory space
domain establishment
```

The builder does not select an allocator.

---

# 57. Read-only allocation query

Recommended:

```go
ResolvedAllocationPlanOf(ast.Node)
```

or reuse the canonical allocation-plan API if one is introduced globally.

P18 must not add a dynamic-array-only competing allocator model.

---

# 58. Storage transition Semantic IR

P18 is the first package that requires explicit general invalidation-domain
transitions in the new pipeline.

Add/reuse:

```text
StorageEstablishDomainOp
StorageAdvanceEpochOp
StorageEndDomainOp
StorageReclaimOp
```

These are general storage operations, not dynamic-array-only semantics.

---

# 59. `StorageEstablishDomainOp`

Represents:

```text
Absent -> Live(initial epoch)
```

Required facts include:

```text
storage identity/domain
origin
backing relation
reclamation authority
address stability
memory space
runtime-generation requirement
```

No physical address is required.

---

# 60. `StorageAdvanceEpochOp`

Represents:

```text
Live(old) -> Live(new)
```

for preserved logical domain identity with invalidated prior incarnation.

No silent epoch wrap.

---

# 61. `StorageEndDomainOp`

Represents:

```text
Live(epoch) -> Ended
```

Every dependency on the domain becomes invalid.

It is distinct from physical reclamation.

---

# 62. `StorageReclaimOp`

Represents the matching physical reclamation responsibility when applicable.

It consumes the resolved reclamation plan.

It does not imply destruction of T elements.

Destruction has already occurred.

---

# 63. Allocation backing operation

Add Semantic IR:

```text
DynamicArrayAllocateOp
```

Inputs/facts:

```text
element type
minimum capacity
allocation context
alignment/layout requirements
```

Outputs:

```text
candidate empty owner
failed
AllocationError
```

On success:

```text
length = 0
capacity >= requested minimum
backing domain established
```

On failure:

```text
candidate owner is not consumed as a valid result
no backing domain is established
```

---

# 64. Checked allocation convention

Compiler-generated allocation CFG follows:

```text
failed == true -> AllocationError path
failed == false -> owner success path
```

The error is a canonical core enum value.

No allocation panic endpoint is introduced.

---

# 65. Internal append operation

Add:

```text
DynamicArrayAppendPreparedOp
```

Inputs:

```text
writable dynamic-array Place
owned prepared element
```

Precondition:

```text
length < capacity
no conflicting borrow
```

Semantics:

```text
initialize element at old length
increment length
transfer element ownership into array
```

No allocation.

---

# 66. Internal growth operation

Add:

```text
DynamicArrayGrowBackingOp
```

Inputs:

```text
writable dynamic-array Place
minimum capacity
resolved allocation context/plan
```

Outputs:

```text
failed
AllocationError
backing_transition
```

Canonical transition values:

```text
none
advance-epoch
replace-domain
```

Failure leaves the owner unchanged.

---

# 67. Growth transition application

After successful internal growth:

```text
none:
    no invalidation transition

advance-epoch:
    StorageAdvanceEpochOp

replace-domain:
    StorageEndDomainOp(old)
    StorageEstablishDomainOp(new)
```

Old physical backing reclamation follows the resolved plan.

---

# 68. Growth and element ownership

Backing growth preserves ownership of all existing initialized elements.

Physical relocation of their bytes is not a source-level `move` of each element.

P17 ownership responsibility remains with the same logical array owner.

Do not emit element `MoveValueOp` solely because storage relocated.

---

# 69. Growth of non-relocatable elements

If element/storage semantics prohibit relocation:

```text
growth requiring relocation is invalid/fails according to the approved
allocation operation contract
```

P18 does not silently bit-move pinned or non-relocatable objects.

---

# 70. Address stability

Dynamic-array backing is normally:

```text
Movable
```

unless the selected allocation/storage plan proves another class.

The owning descriptor itself may move independently.

---

# 71. Allocation effects

Effect analysis must represent as applicable:

```text
AllocateStorage
EstablishDomain
RelocateStorage
AdvanceEpoch
ReclaimStorage
EndDomain
```

Element append without growth has no allocation effect.

---

# 72. Construction from existing compiler/core producers

P18 permits compiler/core operations that already semantically return:

```text
byte[]
rune[]
other T[]
```

to lower through the new dynamic-array primitives once their public failure and
allocation signatures are themselves finalized.

P18 does not silently change those source signatures.

---

# 73. `ToByteArray` / `ToRuneArray` boundary

The core rulebook already identifies allocation-aware materialization as pending.

P18 supplies the owning-array storage/lifecycle substrate.

It does not decide whether a specific conversion returns:

```text
T[]
or
Result[T[], E]
```

when that core API remains normatively unresolved.

---

# 74. Dynamic-array place

P15 Place may address an owning dynamic-array binding/storage.

P18 adds element-place derivation without exposing backing pointer arithmetic.

---

# 75. `DynamicArrayIndexInBoundsOp`

Total Semantic IR predicate:

```text
0 <= index < runtime length
```

with signedness semantics identical to P14/P16.

---

# 76. `DynamicArrayElementPlaceOp`

Inputs:

```text
dynamic-array Place
index
```

Output:

```text
P15 Place<T, authority>
```

Runtime-check form requires the matching bounds guard.

The result carries current backing-domain dependency.

---

# 77. Direct element reference epoch dependency

A direct reference or slice derived from the array depends on the current backing
incarnation.

Reallocation/replacement invalidates that dependency unless a future indirect
stable mechanism explicitly preserves it.

---

# 78. Empty owner slicing

Borrowing:

```sec
ref values[..]
```

from an empty owning `T[]` yields a valid empty slice.

It requires no backing allocation.

The empty slice still has a valid owner/origin relationship.

---

# 79. Empty owner indexing

No index is valid when `length == 0`.

Runtime bounds checking fails normally.

No element Place is produced.

---

# 80. No null owning-array semantic value

The empty owner is a valid initialized owning value.

It is not a null/invalid descriptor.

A physical null-like backing field may be used internally only as part of a
valid empty representation.

---

# 81. MLIR schema version 14

Compiler-generated high-level Sec MLIR uses:

```mlir
sec.dialect_version = 14 : i32
```

Schema versions 1 through 13 remain regression inputs.

Schema v14 adds:

```text
!sec.dynamic_array<T>

sec.dynamic_array.empty
sec.dynamic_array.allocate
sec.dynamic_array.len
sec.dynamic_array.len_int
sec.dynamic_array.capacity_internal
sec.dynamic_array.index_in_bounds
sec.dynamic_array.element_place
sec.dynamic_array.append_prepared
sec.dynamic_array.grow_backing

sec.storage.establish_domain
sec.storage.advance_epoch
sec.storage.end_domain
sec.storage.reclaim
```

---

# 82. `!sec.dynamic_array<T>`

High-level source `T[]`.

Semantic properties:

```text
owned
move-only
runtime length
internal capacity
non-trivial destruction
possibly separate backing storage
```

No physical descriptor fields are type parameters.

---

# 83. `sec.dynamic_array.empty`

No operands.

Result:

```text
!sec.dynamic_array<T>
```

Semantics:

```text
length 0
capacity 0
backing none
no allocation
```

---

# 84. `sec.dynamic_array.allocate`

Inputs include minimum capacity and compiler-selected allocation context.

Results:

```text
!sec.dynamic_array<T>
i1 failed
AllocationError
```

The operation is high-level and total.

The array result is usable only on success.

---

# 85. `sec.dynamic_array.len`

```text
!sec.dynamic_array<T> -> !sec.uint
```

before P6 target scalar resolution.

---

# 86. `sec.dynamic_array.len_int`

```text
!sec.dynamic_array<T> -> !sec.int
```

Requires the current proven representability rule.

---

# 87. `sec.dynamic_array.capacity_internal`

```text
!sec.dynamic_array<T> -> !sec.uint
```

Compiler/core-internal only.

It must not be surfaced by ordinary member lookup.

---

# 88. `sec.dynamic_array.index_in_bounds`

Operands:

```text
array owner/place
integer index
```

Result:

```text
i1
```

Total bounds predicate.

---

# 89. `sec.dynamic_array.element_place`

Operands:

```text
dynamic-array Place
index
```

Result:

```text
!sec.place<T,"ro|rw">
```

depending on owner-place authority.

Required:

```text
bounds_kind
bounds_proof
backing dependency
```

---

# 90. `sec.dynamic_array.append_prepared`

Operands:

```text
writable dynamic-array Place
owned T
```

No ordinary result.

Requires:

```text
capacity available
no conflicting borrow
resolved ownership action
```

No allocation effect.

---

# 91. `sec.dynamic_array.grow_backing`

Operands:

```text
writable dynamic-array Place
minimum capacity
```

Results:

```text
i1 failed
AllocationError
backing transition enum/attr value
```

The operation is compiler/core-internal.

---

# 92. Backing transition representation

Canonical transition values:

```text
none
advance-epoch
replace-domain
```

Use a typed custom enum attribute/type where practical.

Do not encode semantics by operation name strings.

---

# 93. `sec.storage.establish_domain`

High-level storage transition operation.

Carries the canonical storage-domain facts.

No physical allocation address is required.

---

# 94. `sec.storage.advance_epoch`

High-level invalidation operation.

Preserves logical domain identity.

Invalidates previous epoch dependencies.

---

# 95. `sec.storage.end_domain`

Ends one storage/invalidation domain.

No physical deallocation implied by itself.

---

# 96. `sec.storage.reclaim`

Consumes the resolved reclamation plan for physical backing storage.

It is separate from:

```text
element destruction
domain ending
```

---

# 97. P17 destruction-plan extension

Add:

```text
DestroyDynamicArray
```

to destruction plan kinds.

The plan contains:

```text
element destruction plan
reverse runtime iteration
backing domain end
optional reclamation plan
```

---

# 98. P17 cleanup integration

Every owned `T[]` local participates in normal P17 cleanup because the type is
non-trivially destructible.

A moved owner cancels source cleanup and transfers destruction responsibility.

The canonical empty owner may be optimized to no runtime cleanup.

---

# 99. P17 replacement integration

Replacing a `T[]` owner uses P17 transactional ownership replacement.

The old owner is destroyed/reclaimed only after the replacement value is fully
prepared.

---

# 100. P15 reference integration

Element references use the backing invalidation domain.

Backing growth/reallocation is constrained by active direct references.

No new reference model.

---

# 101. P16 slice integration

Extend:

```text
sec.slice.borrow_shared
sec.slice.borrow_mut
```

to accept a dynamic-array owner Place as a contiguous source.

Range length comes from runtime dynamic-array length.

The resulting slice retains backing dependency.

---

# 102. P14 relationship

`T[N]` remains inline fixed storage.

`T[]` remains runtime-sized owned backing storage.

Do not lower one as the other.

---

# 103. P13 nesting

Struct fields may contain `T[]`.

Such a struct is move-only because the field is move-only.

Its P17 derived destruction plan destroys the dynamic-array field in reverse
field order with other fields.

---

# 104. Union/Result/Option nesting

A dynamic array may be an owned payload.

Variant construction/move/destruction uses P17 ownership operations.

No array-specific payload semantics.

---

# 105. Function parameters/results

High-level functions may accept/return:

```text
!sec.dynamic_array<T>
```

by value.

By-value ordinary transfer is move.

No implicit copy.

No physical ABI classification is selected.

---

# 106. FFI boundary

`T[]` is not an FFI-stable descriptor.

Do not pass it directly to C or another foreign ABI without an explicit wrapper
contract.

P18 defines no foreign descriptor layout.

---

# 107. Raw pointer boundary

P18 exposes the compiler-known unsafe property `values.Ptr`, yielding the first
element address for FFI and unsafe integration. It also exposes `values.SizeOf`
as initialized payload bytes and `values.IsEmpty` as `values.Len == 0`.

No ownership is transferred and no lifetime is extended merely by obtaining a
`RawPtr`. The pointer is not a stable descriptor ABI.

---

# 108. Iterator boundary

P18 provides length/index/place semantics needed by later iteration lowering.

It does not complete dynamic-array `for` lowering in this package.

---

# 109. Equality boundary

P18 does not define owning dynamic-array equality.

Do not infer:

```text
descriptor equality
backing identity equality
elementwise equality
```

without a source-language rule.

---

# 110. Membership boundary

Existing frontend membership rules are not extended here unless they already
explicitly include owning `T[]`.

No new membership lowering is introduced.

---

# 111. Growth policy is not source-observable

Because P18 exposes no public capacity member or reserve API:

```text
exact growth factor
exact physical capacity
allocator over-allocation
```

are not source semantics.

They may vary by profile while preserving allocation/failure/relocation rules.

---

# 112. Zero-sized element handling

`T[]` may contain zero-sized T.

Element identity remains:

```text
owner/backing identity + logical index
```

not numeric address.

Length, destruction count, borrow overlap and index semantics still apply.

---

# 113. Effect analysis

P18 integrates:

```text
AllocateStorage
ReclaimStorage
RelocateStorage
EstablishDomain
AdvanceEpoch
EndDomain
```

with call/operation summaries.

Ordinary element replacement does not have those effects.

---

# 114. `@noPanic`

Allocation failure is `Result`-based and does not inherently panic.

Indexing may panic unless proven/fallible.

Destruction/reclamation must follow their own no-panic contracts.

P18 introduces no allocation panic.

---

# 115. No hidden allocation

Forbidden:

```text
copying T[]
passing T[] by value
returning T[]
moving T[]
borrowing T[]
taking len
indexing
creating a slice
```

must not allocate merely because of those operations.

Only an operation defined as allocation-capable may allocate.

---

# 116. No mandatory heap/runtime

P18 does not require:

```text
global heap
GC
reference counting
global allocator singleton
global collection registry
global generation manager
```

Allocation may use Arena, target allocator, pool, or another approved context.

---

# 117. No physical descriptor lowering

P18 does not lower `!sec.dynamic_array<T>` to:

```text
LLVM struct
pointer+length+capacity
pointer+length+capacity+allocator
fat pointer
MemRef
std::vector-like ABI
```

The active plan selects representation later.

---

# 118. No physical allocator lowering

P18 does not choose:

```text
malloc/free
new/delete
mmap
platform heap ABI
specific Arena memory layout
```

The allocation/reclamation plan remains high-level.

---

# 119. Required Sema/type tests

```text
T[] distinct from T[N]
T[] distinct from ref T[]
T[] runtime length not in type identity
T[] MoveOnly regardless of element copyability
T[] non-trivially destructible
mutable T[] omitted initializer -> empty default
immutable T[] omitted initializer rejected
empty default independent of T defaultability
no hidden allocation in empty default
struct containing T[] becomes move-only
```

---

# 120. Required allocation-plan tests

```text
allocation-capable producer has explicit plan
no allocation context -> compile-time error when allocation required
empty default works in noalloc profile
AllocationError identity
alignment derived from T
invalid size mapping
provider/reclamation pairing
Arena context
individually reclaimable allocator context
no allocator selection in backend
```

---

# 121. Required dynamic-array invariant tests

```text
length zero/capacity zero/no backing
length zero/capacity nonzero/internal allocated state
length <= capacity
initialized prefix equals length
uninitialized capacity not readable
append increments length only after element initialization
construction failure destroys initialized prefix
```

---

# 122. Required indexing tests

```text
copyable element read
move-only element read rejected
element replacement
shared element ref
mutable element ref
runtime signed index
runtime unsigned index
IndexError fallible path
ordinary panic.bounds path
empty owner index
same array/index guard dominance
```

---

# 123. Required slice integration tests

```text
full shared slice from T[]
full mutable slice from mutable T[]
subslice
empty owner -> empty slice
slice shares backing identity
slice prevents relocating structural mutation
slice ends then growth allowed
element replacement under legal borrow mode
```

---

# 124. Required storage transition tests

```text
first allocation establishes domain
in-place mutation no transition
in-place capacity growth no invalidation transition
reallocation preserving logical identity -> AdvanceEpoch
logical backing replacement -> EndDomain + EstablishDomain
destruction -> EndDomain
reclaim distinct from EndDomain
same-address reuse not same old domain identity
```

---

# 125. Required destruction tests

```text
empty owner
trivial element backing owner
non-trivial elements reverse runtime order
Arena-backed owner destroys elements but does not individually reclaim bytes
allocator-backed owner destroys then reclaims
moved owner source not destroyed
replacement destroys old owner after RHS success
```

---

# 126. Required growth tests

Internal/compiler tests:

```text
capacity already sufficient -> no growth
successful in-place growth
successful relocating growth
allocation failure leaves owner unchanged
relocation transition recorded
non-relocatable element/storage rejects relocating growth
live direct slice/reference blocks invalidating growth
growth does not source-move each element
```

---

# 127. Required MLIR dialect tests

Schema v14:

```text
!sec.dynamic_array<T> round-trip
wide element type
nested struct element
nested dynamic-array element where type rules allow

empty
allocate
len
len_int
capacity_internal
index_in_bounds
element_place
append_prepared
grow_backing

storage.establish_domain
storage.advance_epoch
storage.end_domain
storage.reclaim

schema-v13 regression
```

---

# 128. Required verifier extensions

Extend/register as applicable:

```text
--sec-verify-ownership
--sec-verify-destruction-plans
--sec-verify-cleanups
--sec-verify-places
--sec-verify-reference-guards
--sec-verify-slice-guards
```

Add:

```bash
--sec-verify-dynamic-arrays
--sec-verify-storage-transitions
```

---

# 129. Dynamic-array verifier

Checks:

```text
type is move-only
length/capacity invariant
allocation success/failure use
initialized-prefix invariant
append capacity precondition
index guard
element-place backing dependency
slice-borrow origin
growth borrow exclusion
growth transactional failure
no public use of internal capacity op
```

---

# 130. Storage-transition verifier

Checks:

```text
domain established before dependent use
no double establishment of one live incarnation
AdvanceEpoch only on live domain
EndDomain only on live domain
no dependent access after EndDomain
reclamation plan matches allocation/provider
reclamation occurs after owned element destruction
no reclaim for Arena authority through owning array
```

---

# 131. Required end-to-end source tests

Where the current source surface exists:

```text
let mut values: byte[]
move values into another owner
T[] parameter consumption
T[] return ownership
values.Len
values.IsEmpty
values.SizeOf
unsafe values.Ptr
supported len(values)
values[index]
values[index] = replacement
ref values[..]
ref mut values[..]
values.Append(value)
values.Clear()
values.RemoveAt(index)
values.ToString()
T[] field in struct
T[] payload in Result/Option/union
```

Allocation-producing calls are included only when their source API is already
normatively finalized.

---

# 132. Unsupported/new-surface tests

P18 must not silently accept invented source operations:

```text
values.Push(...)
values.Append(...)
values.Reserve(...)
values.Capacity()
values.capacity
values.Clone()
dynamic-array literal syntax not defined by the parser/rulebook
implicit fixed-array-to-owning-array conversion
implicit slice-to-owning-array conversion
```

A later rulebook/package may add explicit APIs.

---

# 133. Core materialization integration tests

When testing compiler-internal primitives, cover:

```text
byte[] materialization substrate
rune[] materialization substrate
allocation failure
partial initialization cleanup
returned move-only owner
```

Do not claim `ToByteArray`/`ToRuneArray` source signatures are finalized merely
because the substrate works.

---

# 134. Architecture rules

Non-negotiable:

```text
T[] is an owning runtime-sized descriptor.

T[] is MoveOnly regardless of element copyability.

Implicit copy of T[] is forbidden.

Deep duplication may not hide allocation/failure.

The canonical empty T[] default allocates nothing.

Immutable bindings still require explicit initializer.

Length is runtime state, not type identity.

Capacity is internal state and is not a P18 source member.

Every live logical element is initialized.

Capacity beyond length contains no live T object.

Allocation is explicit in Semantic IR.

Allocation failure uses AllocationError, not panic/null.

No-allocation profiles still support empty T[] values.

Value ownership and backing reclamation authority are separate.

Arena-backed T[] destroys elements without individually reclaiming Arena bytes.

Allocator-backed T[] records matching reclamation plan.

Move transfers descriptor/backing/cleanup ownership but does not imply relocation.

Element replacement does not invalidate backing storage.

Relocating growth emits an explicit invalidation-domain transition.

Direct references/slices may not cross invalidating structural mutation.

Slice borrowing reuses P15/P16 storage/epoch facts.

Element indexing produces a P15 Place.

Implicit move-only element move-out through ordinary runtime indexing remains
deferred; `RemoveAt` is the explicit structural extraction operation.

Destruction is reverse logical element order, then domain end/reclamation.

No public Push/Reserve/Resize/Capacity/AsSlice/AsMutableSlice API is invented.

No physical descriptor layout is selected.

No physical allocator ABI is selected.

No mandatory heap, GC, reference counting, or runtime registry is introduced.

No LLVM dialect is generated by P18.
```

---

# 135. Acceptance criteria

Package 18 is complete only when:

```text
[ ] baseline documents repo 152c772 + local P13-P17 or newer equivalent
[ ] previous package regressions remain green
[ ] dynamic-array/default/copy synchronization applied
[ ] Semantic IR dynamic-array amendment applied
[ ] schema-v14 dialect rulebook installed
[ ] lowering-v14 rulebook installed
[ ] T[] canonical type is separate from fixed arrays/slices/lists
[ ] T[] classified MoveOnly
[ ] T[] non-trivial destruction plan exists
[ ] empty non-allocating default implemented
[ ] immutable no-initializer example corrected
[ ] .Len, .IsEmpty, unsafe .Ptr, and payload-byte .SizeOf implemented
[ ] len(T[]) no-truncation guard preserved
[ ] internal capacity state represented
[ ] no public capacity member added
[ ] allocation context plan represented
[ ] AllocationError flow represented
[ ] noalloc profile rejects only allocating operations
[ ] allocation/reclamation pairing represented
[ ] value ownership/reclamation authority separated
[ ] DynamicArrayAllocateOp implemented
[ ] internal append-prepared implemented
[ ] internal grow-backing implemented
[ ] initialized-prefix invariant verified
[ ] P17 partial construction cleanup integrated
[ ] P17 move/replacement/cleanup integrated
[ ] runtime index/place lowering implemented
[ ] ordinary move-only indexed extraction remains rejected
[ ] Append, Clear, RemoveAt, and ToString implement the canonical public surface
[ ] P15 element reference integration implemented
[ ] P16 slice borrowing from T[] implemented
[ ] storage establish/advance/end/reclaim ops implemented
[ ] relocating growth emits explicit invalidation transition
[ ] direct borrow blocks invalidating growth
[ ] Arena backing destruction/reclamation distinction works
[ ] individually reclaimable backing works
[ ] --sec-verify-dynamic-arrays registered
[ ] --sec-verify-storage-transitions registered
[ ] existing ownership/place/reference/slice verifiers extended
[ ] no public Push/Reserve/Resize/Capacity/AsSlice/AsMutableSlice API invented
[ ] no physical descriptor selected
[ ] no allocator ABI selected
[ ] no mandatory runtime
[ ] check-sec-mlir passes
[ ] go test ./... passes
[ ] legacy paths remain operational
```

---

# 136. Required implementation report

Codex must report:

```text
1. repository HEAD implemented against
2. local/merged P13-P17 status
3. previous package status
4. dynamic-array normative synchronization
5. files added
6. files modified
7. T[] type-model changes
8. MoveOnly classification
9. empty-default implementation
10. dynamic-array destruction plan
11. DynamicArrayFacts representation
12. allocation-context integration
13. AllocationError CFG
14. empty allocation-free state
15. allocated empty-capacity state
16. initialized-prefix tracking
17. DynamicArrayAllocateOp
18. internal append-prepared implementation
19. internal grow-backing implementation
20. transactional growth
21. storage-domain establishment
22. epoch advancement/replacement
23. reclamation-plan implementation
24. Arena versus owning-value reclamation
25. dynamic-array length operations
26. index/place implementation
27. P15 element borrow integration
28. P16 slice-borrow integration
29. P17 move/replacement/cleanup integration
30. schema-v14 types/ops
31. storage transition ops
32. dynamic-array verifier
33. storage-transition verifier
34. existing verifier extensions
35. noalloc profile tests
36. wide-element tests
37. non-trivial element destruction tests
38. relocation/borrow tests
39. unsupported public-growth API tests
40. CMake commands
41. exact LLVM/MLIR version
42. check-sec-mlir result
43. go test ./... result
44. end-to-end source -> schema-v14 results
45. deviations
46. recommendations for Package 19
```

---

# 137. Package 19 boundary

Recommended Package 19:

```text
Allocation Context and Arena Semantic Lowering
```

Reason:

P18 now consumes allocation-context and storage-domain semantics, but physical
Arena/allocation behavior is still only represented abstractly.

P19 should make the existing `allocation.md` and Arena frontend semantics real
in the new pipeline without yet selecting a universal heap.

Recommended scope:

```text
AllocationContext
compiler-selected context propagation
explicit ref mut Arena selection
Arena.Alloc[T]
AllocationError lowering
Arena storage domain
Arena generation/epoch
Arena.Reset
Arena release/destruction
allocation effects
sized/alignment/default checks
owned values living in Arena storage
reference/slice generation dependencies
noalloc profile
high-level !sec.arena and allocation ops
```

P19 should still defer:

```text
physical Arena data structure
one mandatory allocator ABI
malloc/free choice
general list/map/set implementation
stable handles
LLVM
```

After P19, the compiler has a concrete allocation path that can materialize P18
owning arrays and the allocation-aware string/core operations under the canonical
no-hidden-allocation rules.
