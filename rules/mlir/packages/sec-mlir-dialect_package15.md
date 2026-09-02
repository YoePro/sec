# Sec MLIR Program - Implementation Package 15

## Package status

Implementation package for the Sec compiler.

Package ID: `SEC-MLIR-P15`\
Package title: `Safe Place and Direct Reference Semantic Core`\
Repository: `https://github.com/YoePro/sec`\
Repository branch: `main`\
Repository sync commit used for this package: `152c772`\
Local predecessors: `SEC-MLIR-P13`, `SEC-MLIR-P14`\
Repository sync date: `2026-08-09`\
Semantic IR version before package: `1`\
Semantic IR version after package: `1`\
Sec MLIR dialect schema before package: `10`\
Sec MLIR dialect schema after package: `11`\
Sec MLIR lowering specification before package: `10`\
Sec MLIR lowering specification after package: `11`

Package 15 introduces the canonical high-level semantic representation for:

```text
places
ref T
ref mut T
direct safe reference creation
subreferences
reborrows
reference-origin facts
storage identities
invalidation-domain dependencies
validity epochs
reference validity proof/check policy
reference reads and writes for the ownership-safe subset
reference equality
reference lifetime end markers
```

The package deliberately does not select one physical pointer/reference
representation.

It also does not implement stable handles, weak handles, slices, RawPtr-to-ref
conversion, physical pinning, physical epoch storage, or LLVM pointer lowering.

---

# 1. Normative authority

Implementation follows:

```text
rules/memory/reference_model.md
rules/memory/references.md
rules/memory/borrowing.md
rules/memory/memory_model.md
rules/memory/lifetime_analysis.md
rules/memory/storage.md
rules/memory/ownership.md
rules/memory/copy_move.md
rules/errors/runtime_checks.md
rules/errors/panic.md
rules/memory/raw_pointers.md
rules/collections/collections.md
rules/declarations/struct.md
rules/declarations/unions.md
    ↓
rules/compiler/semantic_ir.txt
    ↓
rules/mlir/sec_mlir.md
    ↓
rules/mlir/sec_mlir_dialect.md
    ↓
rules/mlir/sec_mlir_lowering.md
    ↓
implementation package
    ↓
implementation
```

Before implementation:

1. apply `sec_reference_sync_package15.md`;
2. apply `sec_semantic_ir_place_reference_package15.md` to
   `rules/compiler/semantic_ir.txt`;
3. update `rules/mlir/sec_mlir_dialect.md` with
   `sec_mlir_dialect_package15.md`;
4. update `rules/mlir/sec_mlir_lowering.md` with
   `sec_mlir_lowering_package15.md`.

No new source syntax is introduced.

---

# 2. Baseline and local predecessor rule

GitHub `main` remains:

```text
152c772
```

Package 15 additionally assumes the local Package 13 and Package 14 semantics:

```text
P13:
    canonical structs

P14:
    canonical fixed arrays
    arbitrary-precision fixed-array lengths
    fixed-array bounds plans
```

If those packages are merged under a newer HEAD before implementation, Codex
must report the new HEAD and verify semantic equivalence.

---

# 3. Canonical reference rulebook precedence

`rules/memory/reference_model.md` is the canonical Sec 0.1 reference model.

The source-level and runtime-check rules in:

```text
rules/memory/references.md
rules/errors/runtime_checks.md
```

must be interpreted consistently with the complete reference model.

In particular:

```text
safe-reference semantics are independent of physical representation
validity may be statically proven or dynamically checked
ordinary stale safe references are safety failures, not normal Result errors
generation checking does not replace borrowing, provenance, bounds, relocation
or concurrency correctness
```

---

# 4. Wide builtin invariant

These remain active builtin types:

```text
int128
int256
uint128
uint256
decimal128
```

They may be:

```text
referents
struct fields reached through references
fixed-array elements reached through references
union payload values reached through references
```

No future/planned wording is permitted.

---

# 5. Package scope: direct safe references

P15 implements:

```text
ref T
ref mut T
```

as direct safe references.

P15 does not implement:

```text
stable handle source API
weak handle source API
slot-table resolution
fallible stale-handle resolution
```

Those are separate later packages.

---

# 6. Safe reference invariants

A valid `ref T` guarantees:

```text
non-nullness
correct type/alignment semantics
initialized valid T
read authority
valid storage for the reference use
correct storage provenance
valid spatial extent
address-space compatibility
borrow compatibility
relocation correctness
```

A valid `ref mut T` additionally guarantees:

```text
write authority
exclusive mutable access
writable storage
no conflicting shared/mutable borrow
```

A generation match alone is insufficient.

---

# 7. Reference copy/move classification

Canonical:

```text
ref T
    CopyTrivial

ref mut T
    MoveOnly
```

Copying a shared reference:

```text
duplicates the non-owning reference value
does not duplicate or transfer referent ownership
preserves reference-origin semantics
```

Moving a mutable reference:

```text
transfers the reference holder/borrow obligation
does not move the referent
makes the source binding unavailable according to ordinary move rules
```

A `ref mut` copy is invalid.

---

# 8. Place model is canonical compiler analysis data

A place identifies an addressable semantic storage path.

Examples:

```text
local
parameter
value.field
array[index]
union.<variant>
reference.*
```

The existing frontend already has a working `Place` model.

P15 refactors and exposes it.

Do not introduce a second incompatible place representation.

---

# 9. Current place implementation debt

The current `internal/sema/place.go` uses:

```go
Root string
ConstantIndex int64
SliceStart int64
SliceEnd int64
```

and a boolean:

```go
PlacesOverlap(...)
```

P14 already established arbitrary-precision fixed-array length/index semantics.

P15 therefore updates the canonical place model to:

```text
stable root identity
arbitrary-precision constant indexes
symbolic dynamic indexes
structured relationship classification
```

Legacy display strings may remain derived presentation.

---

# 10. Stable place root identity

Add:

```go
type PlaceRootID uint32
```

A source name is not canonical place identity.

Recommended root kinds:

```go
type PlaceRootKind string

const (
    PlaceRootLocal      PlaceRootKind = "local"
    PlaceRootParameter  PlaceRootKind = "parameter"
    PlaceRootStatic     PlaceRootKind = "static"
    PlaceRootAddressed  PlaceRootKind = "addressed"
    PlaceRootAllocation PlaceRootKind = "allocation"
    PlaceRootForeign    PlaceRootKind = "foreign"
)
```

The exact set may reuse canonical storage-domain classifications.

---

# 11. Canonical Place

Recommended:

```go
type Place struct {
    Root               PlaceRootID
    RootKind           PlaceRootKind
    DisplayName        string
    Projections        []PlaceProjection
    Type               Type
    Mutable            bool
    Addressable        bool
    StorageIdentity    StorageIdentityID
    AddressSpace       AddressSpaceID
    AlternativeOrigins []Place
}
```

Presentation names/tokens remain diagnostics metadata.

They are not identity.

---

# 12. Place projections

Canonical projection kinds needed by P15:

```text
field
constant-index
dynamic-index
union-payload
dereference
```

The complete frontend may retain:

```text
slice
property-storage
```

for later packages.

P15 does not make ordinary properties addressable.

---

# 13. Constant index representation

Canonical constant place index:

```go
ConstantIndex *big.Int
```

or an immutable equivalent.

No `int64` semantic limit.

This applies to place overlap/disjointness logic.

---

# 14. Dynamic index identity

A dynamic index projection must identify the evaluated index expression
semantically.

Recommended:

```go
type DynamicIndexID uint32
```

or a resolved SSA/value-plan identity.

Two unrelated runtime indexes are:

```text
PotentiallyOverlapping
```

unless analysis proves a stronger relation.

Do not compare expression text.

---

# 15. Place relationship

Add canonical relationship:

```go
type PlaceRelationship string

const (
    PlaceSame                   PlaceRelationship = "same"
    PlaceDisjoint               PlaceRelationship = "disjoint"
    PlaceContains               PlaceRelationship = "contains"
    PlaceContainedBy            PlaceRelationship = "contained-by"
    PlacePotentiallyOverlapping PlaceRelationship = "potentially-overlapping"
    PlaceUnknown                PlaceRelationship = "unknown"
)
```

Recommended query:

```go
func Relationship(left, right Place) PlaceRelationship
```

Borrowing, moves, destruction, data-race analysis and future atomics must share
this relationship logic.

---

# 16. Legacy `PlacesOverlap`

`PlacesOverlap` may remain temporarily as:

```text
Relationship != Disjoint
```

with appropriate treatment of unknown/potential overlap.

It is compatibility API only.

New correctness logic should use the richer relationship query.

---

# 17. Field disjointness

Distinct stored struct fields may be:

```text
Disjoint
```

when the semantic type representation guarantees independent subobjects.

Borrowing:

```sec
let left := ref mut pair.Left
let right := ref mut pair.Right
```

may coexist when Sema has proven this disjointness.

---

# 18. Array element disjointness

Distinct constant fixed-array indexes may be:

```text
Disjoint
```

when both are valid indexes of the same array.

Runtime indexes into the same array are:

```text
PotentiallyOverlapping
```

unless symbolic/range proof establishes disjointness.

P15 does not invent runtime borrow checking.

---

# 19. Union variant place relationship

Different union variant payload places may be treated as disjoint only under
active-variant control-flow proof.

They share representation over time.

P15 place facts must retain:

```text
union TypeID
stable variant index
active-variant proof scope
```

---

# 20. Alternative origins

The current frontend supports finite multiple possible origins through control
flow.

Preserve the existing canonical behavior:

```text
known finite origin set:
    retain path alternatives

more than implementation limit or unknown:
    degrade to unknown provenance
```

The current implementation limit of 16 alternatives may remain as an
implementation bound.

It is compile-time analysis only.

No runtime provenance tag is introduced.

---

# 21. Storage identity

Add stable compiler identity:

```go
type StorageIdentityID uint32
```

A storage identity is not a numeric address.

It represents one recognized storage domain or live incarnation relationship.

P15 must assign/propagate storage identity for place roots where known.

---

# 22. Invalidation domain identity

Add:

```go
type InvalidationDomainID uint32
```

Examples:

```text
allocation domain
arena allocation epoch domain
collection backing storage domain
object-incarnation domain
addressed storage domain
foreign storage domain when declared
```

A generation belongs to an invalidation domain, not merely an address.

---

# 23. Epoch dependency identity

Add:

```go
type EpochDependencyID uint32
```

The semantic dependency identifies:

```text
which invalidation domain
which expected live incarnation/epoch relationship
```

It does not require runtime epoch bits.

---

# 24. Default logical epoch width

CompilationPlan reference policy defaults to:

```text
64 logical epoch bits
```

including 32-bit general-purpose targets.

Pointer width and epoch width are independent.

A shorter width may be selected only:

```text
at compile time
with proof/exhaustion strategy preserving stale-reference safety
```

Runtime code never chooses epoch width dynamically.

---

# 25. Reference validity policy

Add semantic classification:

```go
type ReferenceValidityPolicy string

const (
    ReferenceValidityProven       ReferenceValidityPolicy = "proven"
    ReferenceValidityDynamicEpoch ReferenceValidityPolicy = "dynamic-epoch"
)
```

A safe operation that is neither provable nor dynamically checkable is rejected
or requires an explicit unsafe/RawPtr boundary.

---

# 26. Proven references

For:

```text
ReferenceValidityProven
```

Semantic IR records the proof provenance.

No runtime generation validation is emitted.

Typical cases include:

```text
local lexical/non-lexical stack borrow
fixed addressed storage
fixed arrays whose owner lifetime is proven
non-relocating static storage
arena borrow proven not to cross reset
```

Physical lowering may therefore use address-only representation when all other
reference guarantees permit it.

---

# 27. Dynamic epoch references

For:

```text
ReferenceValidityDynamicEpoch
```

Semantic IR records:

```text
storage identity
epoch dependency
relocation class
address space
failure behavior
```

A runtime validity operation remains explicit until target/profile lowering.

Physical representation is not fixed.

Possible later representations include:

```text
address + expected epoch
side-table lookup
hardware memory tag
capability
other semantics-preserving mechanism
```

---

# 28. Relocation class

Add semantic classification:

```go
type RelocationClass string

const (
    RelocationStableAddress RelocationClass = "stable-address"
    RelocationFixedAddress  RelocationClass = "fixed-address"
    RelocationMayMove       RelocationClass = "may-relocate"
    RelocationPinned        RelocationClass = "pinned"
    RelocationIndirect      RelocationClass = "indirect-stable"
    RelocationUnknown       RelocationClass = "unknown"
)
```

P15 direct references may not remain live across possible physical relocation
unless correctness is otherwise preserved.

---

# 29. Address space identity

Add/reuse:

```go
type AddressSpaceID uint32
```

Numerically equal addresses in different address spaces do not imply equivalent
safe references.

P15 does not lower target address spaces physically.

---

# 30. Borrow identity

Add:

```go
type BorrowID uint32
type LifetimeID uint32
```

A direct borrow creation receives a BorrowID.

Reference copies/moves/reborrows retain or derive the appropriate borrow
relationship.

The programmer never writes these identities.

---

# 31. ReferenceFacts

Recommended canonical Semantic IR analysis structure:

```go
type ReferenceFacts struct {
    Kind              ReferenceKind
    Referent          TypeID
    OriginPlaces      []PlaceID
    StorageIdentities []StorageIdentityID
    AddressSpace      AddressSpaceID

    Borrow            BorrowID
    Lifetime          LifetimeID
    MutableAuthority  bool

    Validity          ReferenceValidityPolicy
    EpochDependency   EpochDependencyID
    Relocation        RelocationClass

    Provenance        ReferenceProvenanceKind
}
```

Exact internal layout may differ.

A single `IsSafePointer` boolean is forbidden.

---

# 32. Reference kinds in P15

P15 Semantic IR reference kinds:

```text
shared-direct
mutable-direct
```

Stable/weak handles remain separate future categories.

RawPtr is not represented as either reference kind.

---

# 33. Sema resolved reference plan

Add a read-only plan for each reference creation/reborrow.

Recommended:

```go
type ResolvedReferencePlan struct {
    Kind             ReferenceKind
    ReferentType     Type
    Place            Place
    BorrowID         BorrowID
    LifetimeID       LifetimeID
    OriginPlaces     []Place

    ValidityPolicy   ReferenceValidityPolicy
    EpochDependency EpochDependency
    Relocation       RelocationClass
    AddressSpace     AddressSpace

    CopyClass        CopyClassification
}
```

---

# 34. Reference-plan query

Recommended API:

```go
func (a *Analyzer) ResolvedReferencePlanOf(
    expr ast.Expression,
) (ResolvedReferencePlan, bool)
```

or an equivalent query keyed by the exact ref/reborrow AST node.

The query:

```text
does not create a borrow
does not mutate borrow state
does not resolve the place again
does not infer origin again
```

It returns facts already established by successful Sema.

---

# 35. Place-plan query

Add a read-only query:

```go
func (a *Analyzer) ResolvedPlaceOf(
    expr ast.Expression,
) (Place, bool)
```

It exposes canonical analyzed place facts.

It must not register new borrows.

This replaces Semantic IR builder calls to private re-resolution helpers.

---

# 36. Reference use plan

Add read-only Sema facts for reference-mediated access.

Recommended:

```go
type ResolvedReferenceUseKind string

const (
    ReferenceUseRead      ResolvedReferenceUseKind = "read"
    ReferenceUseWrite     ResolvedReferenceUseKind = "write"
    ReferenceUseReborrow  ResolvedReferenceUseKind = "reborrow"
    ReferenceUseEquality  ResolvedReferenceUseKind = "equality"
)
```

A use plan records:

```text
resolved reference facts
validity proof/check policy
transfer action
write replacement/destruction requirements
```

---

# 37. Reference read ownership boundary

Reading through `ref T` does not mean that T is always copied.

P15 complete value-returning read supports:

```text
CopyTrivial referent value
```

only.

For a non-copyable/non-trivial T:

```text
field/subplace reborrow may still be possible
whole-value read remains unsupported until explicit ownership semantics exist
```

Do not silently move from shared reference.

---

# 38. Reference write ownership boundary

Writing through `ref mut T` is replacement.

P15 complete write path requires:

```text
T CopyTrivial
T TriviallyDestructible
new value already valid
destination contract already resolved
```

Non-trivial destruction/replacement remains deferred.

---

# 39. Reference to aggregate subplace

P15 enables reference derivation without copying the containing aggregate.

Examples:

```sec
ref value.field
ref mut value.field
ref values[index]
ref mut values[index]
```

The builder forms semantic places and borrows those places.

It does not:

```text
load whole struct
copy whole array
extract aggregate by value
```

merely to make a reference.

---

# 40. Fixed-array element place

P14 value indexing used:

```text
sec.array.index_in_bounds
sec.array.extract
```

P15 borrowing requires place-oriented indexing.

Add a place bounds predicate and place projection.

Semantic IR:

```text
PlaceArrayIndexInBoundsOp
PlaceArrayElementOp
```

The same P14 `ResolvedArrayIndexPlan` proof facts are reused.

---

# 41. Runtime-checked element borrow

Canonical:

```text
root array place
evaluate index once
PlaceArrayIndexInBoundsOp
branch

failure:
    existing bounds failure behavior

success:
    PlaceArrayElementOp
    reference borrow
```

For ordinary `ref values[index]`, an out-of-bounds index follows ordinary safe
index failure semantics.

It is not a raw pointer operation.

---

# 42. Proven-safe element borrow

When P14/Sema proves bounds:

```text
no runtime bounds branch
PlaceArrayElementOp(bounds = proven-safe)
reference borrow
```

Proof provenance is retained.

---

# 43. Struct field place

P13 stored-field identity maps directly to:

```text
PlaceFieldOp
```

using:

```text
Struct TypeID
StructFieldID
```

Properties do not use `PlaceFieldOp`.

---

# 44. Union payload place

P11/P12 proven active variant paths map to:

```text
PlaceUnionPayloadOp
```

The operation must remain dominated by the matching variant proof.

Borrowing the payload does not copy it.

---

# 45. Dereference place

A safe reference may produce a semantic referent place:

```text
ReferenceDerefPlaceOp
```

Authority:

```text
ref T
    readable place

ref mut T
    readable + writable place
```

For dynamic-validity references, dereference-place formation/use must be
protected by the reference validity guard.

---

# 46. Subreference narrowing

A reference derived from another reference may only:

```text
retain or shorten lifetime
retain or narrow spatial extent
retain or weaken authority
retain compatible storage identity
retain compatible epoch dependency
retain compatible address space
```

A shared reference can never become mutable through safe derivation.

---

# 47. Shared reborrow

Allowed:

```text
ref T -> ref U subplace
ref mut T -> shared ref U subplace
```

when the derived place is valid and borrow rules permit.

The mutable grant remains unavailable for conflicting access for the reborrow
live range as resolved by Sema.

---

# 48. Mutable reborrow

Allowed only from mutable authority:

```text
ref mut T -> ref mut U subplace
```

The result is a constrained exclusive reborrow.

The original mutable reference cannot be used incompatibly while the reborrow
is active.

No `ref T -> ref mut U` path exists.

---

# 49. Reference lifetime end

Add explicit Semantic IR lifetime marker:

```text
ReferenceEndBorrowOp
```

It identifies:

```text
BorrowID
LifetimeID
```

It has no runtime effect by itself.

It allows later lowering/analysis to know when:

```text
relocation restriction
pin dependency
exclusive borrow
epoch dependency
```

may cease to constrain the owner.

---

# 50. Control-flow lifetime ends

Borrow ends may occur on different CFG edges.

Sema remains authoritative for resolved liveness.

Semantic IR may emit multiple path-specific end markers for one borrow when
required.

The verifier rejects reference use after the corresponding borrow has ended on a
reachable path.

---

# 51. Reference validity check

Add total Semantic IR operation:

```text
ReferenceIsValidOp
```

Input:

```text
reference
```

Output:

```text
bool
```

Meaning:

```text
all runtime-checkable temporal/provenance validity dependencies represented by
the selected semantic reference facts currently hold
```

It is not a borrow check.

It is not a bounds check.

It is not a concurrency synchronization primitive.

---

# 52. Ordinary stale reference failure

Add high-level non-returning endpoint:

```text
ReferenceGenerationFailureOp
```

Canonical panic reason:

```text
panic.invalid-reference-generation
```

It represents ordinary stale safe-reference failure.

No `Result` is returned.

No mandatory runtime endpoint is selected.

---

# 53. Dynamic-validity use CFG

For a reference use with:

```text
ValidityPolicy = dynamic-epoch
```

canonical shape:

```text
ReferenceIsValidOp
conditional branch

false:
    ReferenceGenerationFailureOp

true:
    dereference/read/write/equality/reborrow operation
```

The same reference SSA value must be used by the validator and guarded operation.

---

# 54. Proven-valid use

For:

```text
ValidityPolicy = proven
```

no runtime reference validity branch is emitted.

The use retains proof provenance.

---

# 55. Ordinary references are not fallible Result checks

P15 does not implement:

```sec
try reference.value
```

to recover from a stale ordinary direct reference.

A stale ordinary safe reference is a violated safety guarantee.

The non-panicking path is:

```text
static proof
compile-time rejection
or choosing an explicitly fallible handle abstraction when the source model
requires stale resolution as normal behavior
```

Stable/weak handle fallibility is deferred.

---

# 56. Reference equality

Add Semantic IR:

```text
ReferenceCompareOp
```

Allowed:

```text
eq
ne
```

Same reference kind/referent compatibility according to Sema.

Semantic equality means:

```text
same live storage identity
same referenced location within that storage
```

It is not raw numeric address equality.

---

# 57. Dynamic equality validity

If either direct reference requires dynamic validity validation:

```text
validate the relevant reference before semantic equality comparison
```

because safe-reference equality is defined over live storage identity/location.

A stale ordinary reference use follows ordinary stale-reference failure.

---

# 58. RawPtr boundary

`RawPtr[T]` remains distinct.

P15 does not lower:

```text
RawPtr -> ref
ref -> RawPtr
```

Conversions.

RawPtr-to-safe-reference conversion is unsafe and must establish additional
guarantees defined by the reference model.

That belongs to a later FFI/unsafe reference package.

---

# 59. Reference parameters

High-level function parameters may use:

```text
ref T
ref mut T
```

through canonical Semantic IR/Sec MLIR reference types.

Passing an owned place to a ref/ref-mut parameter may create an implicit borrow
only when Sema has already resolved and validated that call behavior.

The Semantic IR call site records the reference-producing operation explicitly.

---

# 60. Passing an existing reference

Passing:

```text
ref T to ref T
```

uses shared reference semantics.

Passing:

```text
ref mut T to ref mut T
```

uses the resolved move/reborrow semantics from Sema.

P15 must not silently copy `ref mut`.

---

# 61. Returning references

A returned safe reference must have a valid non-local origin.

P15 preserves existing Sema returned-reference summaries.

Reference results may originate from:

```text
reference parameters
projected fields/elements beneath allowed external owners
static/addressed storage
other source-approved external storage
```

References to local function storage remain invalid.

---

# 62. Reference origin summary

Semantic IR function metadata retains returned-reference origin summaries.

The summary must identify enough information to instantiate caller-side facts,
such as:

```text
source parameter/root class
projection path
possible origin set
mutability authority
epoch dependency class
```

Do not serialize source variable names as the only semantic identity.

---

# 63. Place/Reference MLIR strategy

Schema v11 introduces a compiler-internal high-level place type:

```text
!sec.place<T, "ro">
!sec.place<T, "rw">
```

and source-level reference types:

```text
!sec.ref<T>
!sec.ref_mut<T>
```

A place value:

```text
cannot be returned as a Sec value
cannot be stored as ordinary data
cannot cross a normal function call as a user value
```

It exists only to make addressable semantics explicit before physical lowering.

---

# 64. Place root from semantic storage

Add:

```text
sec.place.storage
```

Operand:

```text
!sec.storage<T>
```

Result:

```text
!sec.place<T,"ro" or "rw">
```

Attributes preserve:

```text
place_root_id
storage_identity
address_space
```

The result authority cannot exceed storage mutability.

---

# 65. Place root from immutable SSA binding

Add:

```text
sec.place.value
```

Operand:

```text
T
```

Result:

```text
!sec.place<T,"ro">
```

Required compiler provenance:

```text
place_root_id
storage_identity
source binding identity
```

This is a logical addressable binding.

Physical lowering may materialize storage only if required.

It does not allocate at the semantic stage.

---

# 66. Address-taken mutable values

A mutable binding that may be borrowed must remain represented by high-level
semantic storage.

P5 already excludes borrowed/reference storage from its trivial MemRef lowering
contract.

P15 makes that exclusion mechanically enforceable.

---

# 67. P5 address-taken guard

Add/require storage provenance:

```text
sec.address_taken = true
```

or an equivalent canonical analysis fact.

`--sec-lower-trivial-core` must not lower a storage declaration whose semantic
identity is consumed by:

```text
sec.place.storage
reference borrowing
reference-origin dependent operation
```

until reference representation lowering has discharged the place/reference
semantics.

---

# 68. `sec.place.field`

Operand:

```text
!sec.place<Struct,"authority">
```

Required:

```text
field ordinal
```

Result:

```text
!sec.place<FieldType,"same-or-narrower-authority">
```

Properties are invalid here.

---

# 69. `sec.place.array_index_in_bounds`

Operands:

```text
array place
index
```

Result:

```text
i1
```

Uses the fixed-array length from `!sec.array`.

Required:

```text
index_signed
```

No physical address calculation.

---

# 70. `sec.place.array_element`

Operands:

```text
array place
index
```

Required:

```text
bounds_kind
bounds_proof
```

Result:

```text
!sec.place<ElementType, authority>
```

Runtime-check form must be protected by
`sec.place.array_index_in_bounds`.

---

# 71. `sec.place.union_payload`

Operand:

```text
union place
```

Required:

```text
variant index
```

Result:

```text
!sec.place<PayloadType, authority>
```

Valid only on a proven matching active-variant path.

Struct-like payload whole-value place support depends on P13 synthetic payload
identity.

---

# 72. `sec.place.deref`

Operand:

```text
!sec.ref<T>
or
!sec.ref_mut<T>
```

Result:

```text
!sec.place<T,"ro">
or
!sec.place<T,"rw">
```

Dynamic-validity form must be inside the reference-valid true region.

---

# 73. `sec.place.read`

Operand:

```text
!sec.place<T, ...>
```

Result:

```text
T
```

Required:

```text
action
```

P15 compiler output supports:

```text
copy-trivial
```

only for whole-value reads.

---

# 74. `sec.place.write`

Operands:

```text
!sec.place<T,"rw">
new value T
```

No result.

P15 compiler-generation requires:

```text
T CopyTrivial
T TriviallyDestructible
resolved write/contract semantics already valid
```

Non-trivial replacement remains high-level unsupported.

---

# 75. Reference borrow operations

Add:

```text
sec.ref.borrow_shared
sec.ref.borrow_mut
```

Operand:

```text
compatible !sec.place
```

Results:

```text
!sec.ref<T>
!sec.ref_mut<T>
```

Required semantic attributes:

```text
borrow_id
lifetime_id
validity_policy
storage_identity or structured origin facts
relocation_class
address_space
epoch_dependency when applicable
```

No physical address or epoch field is selected.

---

# 76. Shared reference copy

Add:

```text
sec.ref.copy_shared
```

Operand/result:

```text
!sec.ref<T> -> !sec.ref<T>
```

Meaning:

```text
copy the non-owning shared reference value
preserve referent ownership
preserve compatible origin/epoch facts
```

No new mutable authority.

---

# 77. Reference move

Add:

```text
sec.ref.move
```

Allowed:

```text
!sec.ref_mut<T>
```

and explicit moves of shared refs when source move syntax requires.

It preserves referent identity.

For mutable references it transfers the borrow-holder obligation.

The source Semantic IR value may not be used through a source binding after the
resolved move.

---

# 78. Reborrow operations

Add:

```text
sec.ref.reborrow_shared
sec.ref.reborrow_mut
```

Inputs may be:

```text
existing reference
derived subplace
```

according to builder architecture.

Required:

```text
new BorrowID
new LifetimeID
origin is same/narrower
authority does not increase
epoch dependency compatible
```

---

# 79. `sec.ref.is_valid`

Operand:

```text
!sec.ref<T> or !sec.ref_mut<T>
```

Result:

```text
i1
```

Total semantic validity check.

It may conceptually inspect:

```text
storage identity
expected epoch dependency
live invalidation domain
representation validation
```

It does not perform borrow checking.

---

# 80. `sec.fail.reference_generation`

No operands.

No results.

No successors.

Terminator.

Canonical panic reason:

```text
panic.invalid-reference-generation
```

Required provenance may include:

```text
operation
reference source
storage identity
epoch dependency
```

No physical panic sink is selected.

---

# 81. `sec.ref.compare`

Operands:

```text
compatible direct safe references
```

Required:

```text
predicate = eq | ne
```

Result:

```text
i1
```

Meaning is semantic live-storage/location equality.

Do not lower as integer/pointer equality in P15.

---

# 82. `sec.ref.end_borrow`

No results.

Required:

```text
borrow_id
lifetime_id
```

May optionally consume the last holder SSA value for stronger local verification.

It is compiler-generated lifetime metadata represented as an operation.

No runtime action is implied.

---

# 83. Reference guard verifier

Register:

```bash
--sec-verify-reference-guards
```

For dynamic-validity use:

```text
same reference SSA validated
true edge dominates dereference/read/write/reborrow/compare
false edge reaches sec.fail.reference_generation
```

For proven-valid use:

```text
proof provenance must be present
```

It does not redo lifetime analysis.

---

# 84. Place verifier

Register:

```bash
--sec-verify-places
```

It validates:

```text
place types
root identity
authority narrowing
field identity/type
array index proof/guard
union variant guard
deref authority
no place return/store/call escape
no property pretending to be field
```

---

# 85. Borrow verifier

Register:

```bash
--sec-verify-borrow-semantics
```

It validates compiler-generated reference structure against Sema-resolved
metadata:

```text
ref mut is never copied
shared ref copy does not gain authority
shared reborrow never gains mutable authority
mutable reborrow originates from mutable authority
BorrowID/LifetimeID consistency
end-borrow has no later reachable use for that borrow
returned ref has non-local allowed origin summary
place/reference types agree
```

It does not replace Sema's full borrow checker.

---

# 86. Why MLIR does not re-run the borrow checker

Sema already performs source-level:

```text
borrow conflicts
branch joins
loop joins
origin propagation
field/index overlap
move prevention while borrowed
returned-reference analysis
```

P15 MLIR verification checks that emitted IR faithfully reflects those facts.

It must not maintain an independent source-language borrow algorithm.

---

# 87. Reference parameter ABI boundary

`!sec.ref<T>` and `!sec.ref_mut<T>` may appear in:

```text
func.func
sec.call.direct
func.call
```

signatures.

This does not choose:

```text
one machine pointer
fat pointer
address + epoch
capability
hidden metadata arguments
```

ABI lowering is deferred.

---

# 88. Reference representation plan metadata

CompilationPlan may carry reference policy such as:

```text
default logical epoch width
whether a given reference requires runtime epoch support
whether a shorter width is proven
supported address space
supported hardening mechanism
```

The package must not assume one global runtime reference shape.

---

# 89. No universal epoch field

Do not encode epoch width into:

```text
!sec.ref<T>
!sec.ref_mut<T>
```

source-level type identity.

Two builds may select different physical representation while preserving the
same Sec type and semantics.

---

# 90. Constrained bare-metal behavior

For a proven lexical reference:

```text
runtime generation metadata may be absent
```

For fixed addressed storage:

```text
allocation generation is normally absent
```

If a constrained profile cannot prove or dynamically preserve safe semantics:

```text
reject the safe operation
or require explicit RawPtr/unsafe boundary
```

Never silently weaken `ref` to raw address semantics.

---

# 91. Hosted behavior

An optimized hosted profile may lower proven short-lived references to:

```text
address only
```

A hardened profile may later lower selected references to:

```text
address + epoch
side-table checked address
capability
```

P15 does not choose among them.

---

# 92. Reference equality physical boundary

Future lowering may compare:

```text
address
storage identity + offset
capability identity
domain/offset metadata
```

only when that representation is proven equivalent to semantic live
storage/location equality.

P15 leaves `sec.ref.compare` high-level.

---

# 93. Effect analysis

A proven-valid reference use:

```text
adds no reference-generation panic effect
```

A dynamic generation validation whose failure remains reachable contributes:

```text
MayPanic
panic.invalid-reference-generation
```

Generation comparison itself does not imply:

```text
allocation
blocking
I/O
```

Effects from referent access remain separate.

---

# 94. `@noPanic`

A function using only proven references may remain `@noPanic`.

A function whose required dynamic reference-generation check can reach
`sec.fail.reference_generation` is not `@noPanic`.

Optional hardening that the compiler has also statically proven impossible to
fail must not create a semantic panic effect merely because a debug/profile
representation retains a redundant assertion.

Semantic effects follow the proof classification, not optional instrumentation.

---

# 95. Invalidation facts

P15 Semantic IR introduces canonical invalidation metadata sufficient to name:

```text
owner destruction
allocation free
arena reset
arena release
collection backing replacement
physical relocation
foreign invalidation
target domain reset
```

Package 15 does not require every producer operation to exist yet.

Future packages must emit/attach these events rather than invent another
invalidation system.

---

# 96. Generation transitions boundary

P15 does not implement physical:

```text
epoch counter allocation
epoch increment
slot generation
domain retirement
```

Those belong to allocation/arena/collection/handle representation packages.

The semantic IDs/dependencies are established now so later packages plug into
one model.

---

# 97. P13 integration

P13 struct fields may now be borrowed directly:

```text
struct place
sec.place.field
sec.ref.borrow_shared / borrow_mut
```

No whole-struct copy is required.

P13 stored-field identity remains authoritative.

Properties remain non-field member semantics.

---

# 98. P14 integration

P14 fixed-array indexing now supports direct element places for borrowing.

P14:

```text
ResolvedArrayIndexPlan
arbitrary-precision constant index
bounds proof/runtime classification
```

is reused.

P15 must not reintroduce `int64` place indexes.

---

# 99. P12 integration

P12 match payload bindings using:

```text
ref
ref mut
```

can now be enabled for representable union/Result payload places when Sema's
existing resolved match binding action is:

```text
borrow-shared
borrow-mutable
```

The reference remains branch-scoped according to existing Sema rules.

It must not escape through:

```text
match result
outer assignment
return
captured lambda
```

when Sema classifies the origin match-scoped.

---

# 100. P11 integration

Union payload reference derivation uses:

```text
active variant proof
semantic payload place
reference borrow
```

It does not construct/copy the payload first.

---

# 101. Reference-containing aggregates

P13/P14 aggregate types may contain reference values.

P15 preserves contained origin facts per canonical aggregate path as already
tracked by Sema.

Whole-aggregate transfer continues to use existing copy/move classification.

A struct containing `ref mut` is therefore move-only unless source type rules
state otherwise.

---

# 102. Dynamic origin joins

A reference SSA value may represent a finite compile-time set of possible
origins after control-flow merge.

P15 stores this as analysis/provenance metadata.

It does not add a runtime origin tag merely for borrow checking.

Dereference uses the actual runtime reference representation while compile-time
alias analysis conservatively considers every possible origin.

---

# 103. Unknown provenance

When canonical origin becomes unknown:

```text
ordinary existing reference use may remain valid if its own facts prove safe
creating a new reborrow requiring precise owner tracking is rejected
```

P15 follows existing frontend behavior.

Do not fabricate one origin.

---

# 104. No mandatory runtime

P15 introduces no mandatory:

```text
garbage collector
reference counting
global generation manager
handle table
runtime borrow counter
runtime borrow lock
universal side table
```

Dynamic validity may later lower to inline metadata or target/profile support
only when required.

---

# 105. No RawPtr collapse

Never lower:

```text
!sec.ref<T>
!sec.ref_mut<T>
```

to `RawPtr[T]` at the semantic stage.

Raw pointers lack safe reference guarantees.

---

# 106. No physical address arithmetic

Place projections remain semantic.

P15 does not calculate:

```text
field byte offset
array element stride
GEP indexes
numeric pointer addition
```

Those require resolved physical aggregate layouts later.

---

# 107. No slice type in P15

P15 place infrastructure may retain slice projection metadata already used by
Sema.

However source:

```text
ref T[]
```

slice value representation is Package 16.

Do not model slice as `!sec.ref<!sec.array<...>>`.

---

# 108. Sec MLIR schema version 11

Compiler-generated high-level Sec MLIR uses:

```mlir
sec.dialect_version = 11 : i32
```

Schema v11 adds:

```text
!sec.place<T, authority>
!sec.ref<T>
!sec.ref_mut<T>

sec.place.value
sec.place.storage
sec.place.field
sec.place.array_index_in_bounds
sec.place.array_element
sec.place.union_payload
sec.place.deref
sec.place.read
sec.place.write

sec.ref.borrow_shared
sec.ref.borrow_mut
sec.ref.copy_shared
sec.ref.move
sec.ref.reborrow_shared
sec.ref.reborrow_mut
sec.ref.is_valid
sec.ref.compare
sec.ref.end_borrow

sec.fail.reference_generation
```

---

# 109. `!sec.place`

Canonical conceptual syntax:

```text
!sec.place<T, "ro">
!sec.place<T, "rw">
```

A place is compiler-internal.

It is not a source type.

---

# 110. Place authority

Allowed:

```text
ro
rw
```

Derivation may preserve or reduce authority.

It may never increase authority.

---

# 111. `!sec.ref`

Canonical:

```text
!sec.ref<T>
```

Safe:

```text
non-null
shared read authority
non-owning
```

No physical epoch/address metadata in the type.

---

# 112. `!sec.ref_mut`

Canonical:

```text
!sec.ref_mut<T>
```

Safe:

```text
non-null
exclusive mutable authority
non-owning referent
move-only reference value
```

---

# 113. Place root IDs in MLIR

Place-producing root operations carry deterministic compiler identities:

```text
sec.place_root_id
sec.storage_identity
sec.address_space
```

as integer/string/custom attrs according to existing dialect conventions.

The source display name is optional provenance.

---

# 114. Epoch metadata in MLIR

Reference-producing operations may carry semantic attrs:

```text
sec.validity_policy
sec.epoch_dependency
sec.relocation_class
sec.borrow_id
sec.lifetime_id
sec.reference_origin
```

These are semantic metadata.

They do not force runtime fields.

---

# 115. Reference validity proof strings

Canonical proof categories may include:

```text
local-lifetime
parameter-origin
static-storage
addressed-storage
arena-no-reset
no-invalidation-path
no-relocation
analysis
```

The exact internal enum may be richer.

Compiler output must not use arbitrary free-form source text as proof authority.

---

# 116. `sec.place.value`

Operand:

```text
T
```

Result:

```text
!sec.place<T,"ro">
```

Required root/storage identity attrs.

---

# 117. `sec.place.storage`

Operand:

```text
!sec.storage<T>
```

Result:

```text
!sec.place<T,"ro|rw">
```

Authority matches storage mutability.

---

# 118. `sec.place.field`

Operand:

```text
!sec.place<Struct,...>
```

Field ordinal attribute.

Result field place.

---

# 119. `sec.place.array_index_in_bounds`

Operands:

```text
!sec.place<!sec.array<T,N>,...>
index
```

Result:

```text
i1
```

Total bounds predicate.

---

# 120. `sec.place.array_element`

Operands:

```text
array place
index
```

Result:

```text
element place
```

Requires P14-compatible bounds proof/guard attrs.

---

# 121. `sec.place.union_payload`

Operand:

```text
union place
```

Required stable variant index.

Result payload place.

Variant guard verifier integration required.

---

# 122. `sec.place.deref`

Operand:

```text
!sec.ref<T> or !sec.ref_mut<T>
```

Result place authority derived from reference kind.

Dynamic validity guard required when applicable.

---

# 123. `sec.place.read`

Operand:

```text
place
```

Result:

```text
T
```

P15 action:

```text
copy-trivial
```

---

# 124. `sec.place.write`

Operands:

```text
rw place
T
```

No result.

P15 generation requires trivial replacement safety.

---

# 125. Borrow operations

`sec.ref.borrow_shared`:

```text
ro/rw place -> !sec.ref<T>
```

`sec.ref.borrow_mut`:

```text
rw place -> !sec.ref_mut<T>
```

Required semantic reference facts.

---

# 126. Reborrow authority

`sec.ref.reborrow_shared`:

```text
shared or mutable source authority -> shared result
```

`sec.ref.reborrow_mut`:

```text
mutable source authority -> mutable result
```

No mutable result from shared source.

---

# 127. `sec.ref.is_valid`

Returns:

```text
i1
```

for direct reference runtime-validity dependencies.

It does not imply one concrete check implementation.

---

# 128. `sec.fail.reference_generation`

Terminator.

Required panic ID:

```text
panic.invalid-reference-generation
```

No Result conversion.

---

# 129. `sec.ref.compare`

Predicate:

```text
eq
ne
```

Result:

```text
i1
```

High-level semantic reference equality.

---

# 130. `sec.ref.end_borrow`

Compiler-generated lifetime marker.

Required:

```text
borrow_id
lifetime_id
```

No runtime side effect is implied.

---

# 131. Place escape restrictions

MLIR verifier rejects `!sec.place` values used as:

```text
func.return operand
ordinary func.call argument
ordinary storage value
aggregate stored field
Result payload
union payload
array element
```

A place may only feed recognized place/reference operations.

---

# 132. Reference guard verifier

Dynamic-validity operations require correct guard.

Recognized guarded uses include:

```text
place.deref
place.read/write beneath dereference
reborrow
reference comparison
```

The false path reaches:

```text
sec.fail.reference_generation
```

---

# 133. Reference storage lowering boundary

P5 does not lower high-level storage while:

```text
address-taken/reference place dependencies remain
```

A later reference representation pass decides when the root may become:

```text
memref
LLVM pointer
target address
capability
other lower object
```

---

# 134. P6 compatibility

Target scalar resolution may recurse through referent type arguments while
preserving:

```text
!sec.ref
!sec.ref_mut
!sec.place
```

wrappers.

Example:

```text
!sec.ref<!sec.int>
    -> !sec.ref<si32>
```

on a 32-bit target.

No reference representation selected.

---

# 135. P8 compatibility

Signless integer normalization must not recursively erase signedness/type
semantics inside:

```text
!sec.ref
!sec.ref_mut
!sec.place
```

Dedicated referent/aggregate representation lowering owns that stage.

---

# 136. P13/P14 compatibility

Struct/array high-level wrappers remain.

Place operations identify subobjects semantically without physical offsets.

No aggregate copy is required for borrowing.

---

# 137. Effect-analysis tests

Required:

```text
proven local ref use -> no invalid-reference panic effect
dynamic epoch validation -> invalid-reference panic effect
reference generation check -> no allocation effect by itself
reference generation check -> no blocking effect by itself
shared/mutable borrow creation -> no runtime allocation
borrow end -> no runtime effect
```

---

# 138. Required Sema place tests

```text
stable root ID independent of source display
field relationship Same/Disjoint/Contains
constant fixed-array indexes use arbitrary precision
distinct constant indexes disjoint
equal constant index same
dynamic indexes potentially overlap
different roots disjoint when storage identity proves it
union variant path proof
deref path origin
alternative origin set
unknown-origin degradation
legacy PlacesOverlap compatibility
```

---

# 139. Required Sema reference tests

```text
ref local
ref mut local
shared copy
mutable copy rejected
mutable move transfers holder
shared reborrow
mutable-to-shared reborrow
mutable-to-mutable reborrow
shared-to-mutable rejected
field subreference
constant array-index subreference
dynamic array-index subreference
returned parameter ref
returned local ref rejected
match-scoped borrowed payload escape rejected
origin joins preserved
```

---

# 140. Required epoch/reference-plan tests

```text
proven local reference
proven addressed storage reference
arena dependency fact
dynamic epoch policy fact
64-bit logical default on 32-bit plan
64-bit logical default on 64-bit plan
shorter width requires proof
no runtime-selected width
unknown uncheckable safe reference rejected
```

No physical runtime epoch allocation is required by these tests.

---

# 141. Required Semantic IR place tests

```text
root value place
root storage place
field place
fixed-array constant element place
fixed-array dynamic guarded element place
union payload place
reference deref place
authority narrowing
no authority widening
place deterministic printer
```

---

# 142. Required Semantic IR reference tests

```text
borrow shared
borrow mutable
copy shared
move mutable
reborrow shared
reborrow mutable
proven validity
dynamic validity
reference compare
end borrow
no use after borrow end
no ref mut copy
```

---

# 143. Required MLIR dialect tests

```text
!sec.place ro/rw round-trip
!sec.ref round-trip
!sec.ref_mut round-trip
place root ops
field place
array bounds/place ops
union payload place
deref place
place read/write
borrow ops
copy/move
reborrow
is_valid
compare
end_borrow
fail.reference_generation
schema-v10 regression
```

---

# 144. Required place verifier tests

```text
field ordinal/type mismatch rejected
rw from ro rejected
property encoded as field rejected
array element runtime-check without guard rejected
array element different index guard rejected
union payload wrong variant path rejected
place returned from function rejected
place stored as ordinary value rejected
place passed as ordinary user argument rejected
```

---

# 145. Required reference guard tests

```text
proven ref use without runtime check accepted
dynamic ref use with guard accepted
dynamic ref use without guard rejected
wrong reference validated rejected
false-path deref rejected
failure path missing fail.reference_generation rejected
comparison on unguarded dynamic ref rejected
reborrow on unguarded dynamic ref rejected
```

---

# 146. Required borrow verifier tests

```text
shared copy accepted
ref mut copy rejected
ref mut move accepted
shared-to-shared reborrow
mutable-to-shared reborrow
mutable-to-mutable reborrow
shared-to-mutable rejected
borrow ID mismatch rejected
lifetime ID mismatch rejected
use after end-borrow rejected
return local-origin ref rejected
```

---

# 147. Required P13 integration tests

```text
ref struct field
ref mut distinct struct fields coexist when proven disjoint
whole struct borrow conflicts with field mutable borrow
property is not a place-field borrow
nested field reborrow
wide int128/uint256 field referents
```

---

# 148. Required P14 integration tests

```text
ref fixed-array constant element
ref mut fixed-array constant element
distinct constant element mutable borrows
equal index conflicts
dynamic indexes conservatively overlap
dynamic element borrow bounds checked once
proven-safe element borrow no runtime bounds branch
int128/uint256 index proof metadata
zero-length has no valid element borrow
```

---

# 149. Required P12 match integration tests

```text
union single payload ref binding
union single payload ref mut binding
Result Ok ref binding when source rules permit
Result Err ref binding when source rules permit
match-scoped ref cannot escape
guard uses borrowed payload
guard false ends branch-local borrow correctly
```

---

# 150. Required function integration tests

```text
ref parameter
ref mut parameter
owned local passed to ref parameter
owned mutable local passed to ref mut parameter
returned input ref
returned input subfield ref
local ref return rejected by Sema
shared ref direct call copy
ref mut direct call move/reborrow according to resolved parameter mode
```

---

# 151. Unsupported P15 end-to-end tests

The new IR path explicitly rejects:

```text
stable-handle source operations
weak-handle source operations
handle resolution
slice values
RawPtr-to-ref conversion
ref-to-RawPtr FFI conversion
physical pinning
physical allocation generation
physical arena epoch storage
physical collection epoch storage
concurrent invalidation protocol
move-only whole-value read through ref
semantic-copy whole-value read through ref
non-trivial write/replacement through ref
```

Do not emit placeholder semantics.

---

# 152. No physical lowering

P15 does not choose:

```text
LLVM pointer
MemRef reference representation
address + epoch struct
fat pointer
side table
capability
slot handle
pointer tag
hidden ABI argument list
```

---

# 153. No mandatory runtime

P15 must continue to permit a program using only proven stack/fixed references
to require no generation runtime support at all.

---

# 154. Architecture rules

Non-negotiable:

```text
Places are semantic storage paths, not numeric addresses.

Place root identity is not source name.

Constant place indexes are arbitrary precision.

Borrow overlap uses one canonical place relationship model.

Safe references are non-null and non-owning.

ref is shared/copyable.

ref mut is exclusive/move-only.

Reference validity is independent of physical representation.

A validity epoch belongs to an invalidation domain.

Default logical epoch width is 64 bits, independent of pointer width.

Shorter epoch width is compile-time only and proof dependent.

Proven references need no runtime generation check.

Dynamic epoch validation remains high-level until profile lowering.

Generation checking never replaces borrow checking.

Generation checking never replaces bounds checking.

Generation checking never replaces relocation or concurrency proof.

Ordinary stale direct references panic/trap; they are not normal Result errors.

Reference equality is live storage identity + location, not raw address equality.

Subreferences may only narrow lifetime/bounds/authority.

A shared reference cannot become mutable through safe derivation.

P13 field borrowing does not copy the containing struct.

P14 element borrowing does not copy the containing array.

P5 may not erase address-taken reference roots before reference lowering.

Places may not escape as source values.

RawPtr remains distinct.

Stable/weak handles remain distinct.

Slices remain Package 16.

No physical pointer/reference representation is selected.

No mandatory runtime is introduced.

No LLVM dialect is generated.
```

---

# 155. Acceptance criteria

Package 15 is complete only when:

```text
[ ] baseline documents repo 152c772 + local P13/P14 or newer merged equivalent
[ ] previous package regressions remain green
[ ] reference/runtime-check synchronization applied
[ ] Semantic IR place/reference amendment applied
[ ] schema-v11 dialect rulebook installed
[ ] lowering-v11 rulebook installed
[ ] stable PlaceRootID implemented
[ ] canonical StorageIdentityID implemented/reused
[ ] canonical InvalidationDomainID implemented/reused
[ ] canonical EpochDependencyID implemented/reused
[ ] arbitrary-precision constant place indexes implemented
[ ] canonical PlaceRelationship implemented
[ ] legacy PlacesOverlap delegates to canonical relationship
[ ] finite alternative origins preserved
[ ] ResolvedPlaceOf is read-only
[ ] ResolvedReferencePlanOf is read-only
[ ] ReferenceFacts implemented
[ ] validity policy proven/dynamic implemented
[ ] 64-bit logical epoch default represented in CompilationPlan facts
[ ] shorter widths require compile-time proof
[ ] relocation class represented
[ ] address space represented
[ ] BorrowID/LifetimeID represented
[ ] shared ref copy classification preserved
[ ] ref mut move-only preserved
[ ] Semantic IR place operations implemented
[ ] Semantic IR ref borrow/reborrow operations implemented
[ ] Semantic IR reference validity op implemented
[ ] Semantic IR invalid-generation failure endpoint implemented
[ ] Semantic IR reference compare implemented
[ ] Semantic IR borrow-end marker implemented
[ ] !sec.place implemented
[ ] !sec.ref implemented
[ ] !sec.ref_mut implemented
[ ] schema-v11 place/ref operations implemented
[ ] sec.fail.reference_generation implemented
[ ] --sec-verify-places registered
[ ] --sec-verify-reference-guards registered
[ ] --sec-verify-borrow-semantics registered
[ ] P5 address-taken storage guard enforced
[ ] P6 preserves ref/place wrappers
[ ] P8 preserves ref/place wrappers
[ ] P13 field borrowing works
[ ] P14 fixed-array element borrowing works
[ ] P12 ref/ref-mut payload match binding works for supported payloads
[ ] returned-reference origin summaries preserved
[ ] ordinary stale direct ref is not lowered to Result
[ ] panic.invalid-reference-generation used for reachable dynamic failure
[ ] no stable/weak handle source semantics added
[ ] no slices added
[ ] no RawPtr collapse
[ ] no physical reference layout selected
[ ] no mandatory generation manager/runtime
[ ] check-sec-mlir passes
[ ] go test ./... passes
[ ] legacy paths remain operational
```

---

# 156. Required implementation report

Codex must report:

```text
1. repository HEAD implemented against
2. local/merged P13 and P14 status
3. previous package status
4. reference/runtime-check rule synchronization
5. files added
6. files modified
7. PlaceRootID migration
8. arbitrary-precision place-index migration
9. PlaceRelationship implementation
10. legacy PlacesOverlap compatibility
11. StorageIdentity/InvalidationDomain/EpochDependency representation
12. ResolvedPlaceOf API
13. ResolvedReferencePlanOf API
14. ReferenceFacts representation
15. validity-policy implementation
16. CompilationPlan epoch-width facts
17. relocation/address-space facts
18. BorrowID/LifetimeID implementation
19. Semantic IR place operations
20. Semantic IR reference operations
21. shared-copy/mutable-move implementation
22. reborrow implementation
23. validity guard/failure CFG
24. reference equality
25. end-borrow representation
26. schema-v11 types/ops
27. place verifier
28. reference guard verifier
29. borrow verifier
30. P5 address-taken storage changes
31. P6/P8 compatibility
32. P13 field-borrow integration
33. P14 array-element-borrow integration
34. P12 match-borrow integration
35. returned-reference integration
36. proven-reference tests
37. dynamic-epoch tests
38. 32/64-bit profile tests
39. unsupported handle/slice/raw-conversion tests
40. CMake commands
41. exact LLVM/MLIR version
42. check-sec-mlir result
43. go test ./... result
44. end-to-end source -> schema-v11 results
45. deviations
46. recommendations for Package 16
```

---

# 157. Package 16 boundary

Recommended Package 16:

```text
Slice Semantic Value Representation
```

P15 now provides the required place/reference core.

P16 can therefore define:

```text
ref T[]
mutable slice form according to canonical syntax
slice base reference
slice length
slice origin/storage identity
slice epoch dependency
array-to-slice borrow
slice-to-subslice narrowing
empty slice
slice index place
slice bounds/range checks
IndexError/RangeError flow
slice .len
slice equality rules if defined
slice reference escape rules
high-level !sec.slice / slice operations
```

without inventing a second alias/lifetime/provenance system.

P16 should still defer:

```text
dynamic owning T[]
physical pointer+length descriptor lowering
general collection backing stores
stable/weak handles
LLVM
```
