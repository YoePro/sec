# Sec MLIR Program - Implementation Package 16

## Package status

Implementation package for the Sec compiler.

Package ID: `SEC-MLIR-P16`  
Package title: `Slice Semantic Value Representation`  
Repository: `https://github.com/YoePro/sec`  
Repository branch: `main`  
Repository sync commit used for this package: `152c772`  
Local predecessors: `SEC-MLIR-P13`, `SEC-MLIR-P14`, `SEC-MLIR-P15`  
Repository sync date: `2026-08-09`  
Semantic IR version before package: `1`  
Semantic IR version after package: `1`  
Sec MLIR dialect schema before package: `11`  
Sec MLIR dialect schema after package: `12`  
Sec MLIR lowering specification before package: `11`  
Sec MLIR lowering specification after package: `12`

Package 16 introduces canonical Semantic IR and high-level Sec MLIR
representation for safe borrowed slices:

```text
ref T[]
ref mut T[]
```

It builds directly on Package 15 places and direct-reference facts.

A slice is modeled as:

```text
a direct safe bounded reference
to a contiguous element range
owned by another storage identity
```

Package 16 covers:

```text
shared slices
mutable slices
explicit fixed-array-to-slice borrowing
slice-to-slice reborrowing
inclusive/exclusive/open ranges
normalized half-open ranges
empty slices
slice length
slice indexing as an element place
ordinary bounds/range panic paths
fallible IndexError / RangeError paths
shared-slice copy
mutable-slice move
borrow/lifetime/origin/epoch propagation
returned slice origin summaries
P13/P14/P15 integration
```

It deliberately does not implement owning dynamic `T[]`, physical slice
descriptors, FFI slice ABI, stable/weak handles, or non-trivial element
ownership operations.

---

# 1. Normative authority

Implementation follows:

```text
rules/collections/collections.md
rules/memory/reference_model.md
rules/memory/borrowing.md
rules/errors/runtime_checks.md
rules/library/core-library.md
rules/errors/panic.md
rules/memory/layout.md
rules/memory/copy_move.md
rules/memory/ownership.md
rules/memory/raw_pointers.txt
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

1. apply `sec_slice_sync_package16.md`;
2. apply `sec_semantic_ir_slice_package16.md` to
   `rules/compiler/semantic_ir.txt`;
3. update `rules/mlir/sec_mlir_dialect.md` with
   `sec_mlir_dialect_package16.md`;
4. update `rules/mlir/sec_mlir_lowering.md` with
   `sec_mlir_lowering_package16.md`.

No new source syntax is introduced.

---

# 2. Repository and local predecessor rule

GitHub `main` remains:

```text
152c772
```

P16 additionally assumes the local semantics from:

```text
P13:
    structs

P14:
    fixed arrays
    arbitrary-precision fixed lengths
    fixed-array bounds plans

P15:
    places
    ref/ref mut
    StorageIdentity
    BorrowID/LifetimeID
    validity policy
    epoch dependency
    relocation class
    reference guards
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

They may be slice element types.

They may also participate in slice index/range expressions where the ordinary
integer-index rules permit them.

No future/planned wording is allowed.

---

# 4. Canonical slice source types

Source-level slice types are exactly:

```sec
ref T[]
ref mut T[]
```

They are not owning arrays.

They are not RawPtr values.

They are not nullable.

They are not fixed arrays.

They are not modeled as:

```text
ref T[N]
ref !sec.array<T,N>
```

A slice is a distinct bounded reference category.

---

# 5. Shared slice semantics

`ref T[]`:

```text
is non-owning
has shared read authority
is copyable
is trivially destructible
has runtime length
has bounded spatial extent
inherits storage identity/lifetime/epoch dependency
never owns or destroys elements
```

Copying it copies only the slice-reference value.

It does not copy elements.

---

# 6. Mutable slice semantics

`ref mut T[]`:

```text
is non-owning
has exclusive mutable authority
is move-only
is trivially destructible
has runtime length
has bounded spatial extent
inherits storage identity/lifetime/epoch dependency
never owns or destroys elements
```

It may be moved or safely reborrowed.

It must not be freely copied.

---

# 7. Slice representation remains abstract

The conceptual runtime information includes at least:

```text
storage identity
element type
bounded range
length
authority
provenance
lifetime/epoch dependency
address-space compatibility
relocation correctness
```

A target/profile may later represent this as:

```text
address + length
address + length + expected epoch
handle + length
capability + bounds
another semantics-preserving descriptor
```

P16 chooses none of these.

---

# 8. Empty slices

An empty slice is a valid safe slice.

It has:

```text
length == 0
no dereferenceable element
valid storage/origin semantics
```

A physical backend may later use a null-like or sentinel base internally only
when:

```text
length is zero
the base is never dereferenced
no safe ref T is reconstructed from the base alone
all operations respect zero length
```

This does not make safe references nullable.

---

# 9. No default slice value

A slice has no implicit default value.

A declaration requiring a slice value must obtain it from valid storage or
another valid slice.

The compiler must not fabricate:

```text
null slice
dangling slice
originless slice
```

---

# 10. Explicit slice creation

Slice construction remains explicit.

Canonical examples:

```sec
let all := ref values[..]
let writable := ref mut values[..]
let part := ref values[1..<4]
let inclusive := ref values[1..3]
```

There is no implicit fixed-array-to-slice conversion.

---

# 11. Runtime-check syntax synchronization

The runtime-check rulebook currently illustrates fallible slicing as:

```sec
let part := try values[start..<end]
```

while the canonical slice rule requires explicit `ref` or `ref mut`.

P16 synchronizes the fallible source shape to:

```sec
let part := try ref values[start..<end]
```

and mutable:

```sec
let part := try ref mut values[start..<end]
```

Parentheses may be used when required for clarity:

```sec
let part := try (ref values[start..<end])
```

A bare slice-range expression remains a range/place expression used by explicit
slice construction.

It is not an implicit slice value.

---

# 12. Slice creation sources in P16

Complete end-to-end P16 support:

```text
fixed T[N] storage/place
existing ref T[] slice
existing ref mut T[] slice
```

Language-valid but new-pipeline-deferred source:

```text
owning dynamic T[]
```

because canonical dynamic owning-array Semantic IR is not yet implemented.

Do not downgrade the source feature.

Report explicit `UnsupportedFeatureError` in the new IR path.

---

# 13. Slice range syntax

Supported source forms:

```sec
source[start..<end]
source[start..end]
source[..]
source[start..]
source[..end]
source[..<end]
```

Descending ranges are invalid.

An exclusive equal range is empty and valid:

```sec
source[2..<2]
```

---

# 14. Canonical normalized range

Every valid slice range is normalized semantically to a half-open range:

```text
[start, endExclusive)
```

with:

```text
length = endExclusive - start
```

This normalized form is used for:

```text
borrow overlap
subslice narrowing
runtime slice length
element-place origin
returned-slice range facts
debugging/verifiers
```

The original inclusive/exclusive spelling remains source provenance only after
normalization.

---

# 15. Open range normalization

Canonical:

```text
[..]
    start = 0
    endExclusive = sourceLength

[start..]
    start = start
    endExclusive = sourceLength

[..<end]
    start = 0
    endExclusive = end

[..end]
    start = 0
    endExclusive = end + 1
```

No source expression is invented for an omitted bound.

---

# 16. Inclusive range normalization

For:

```sec
source[start..end]
```

the semantic range is:

```text
[start, end + 1)
```

only after the inclusive bound has passed its range validation.

Do not perform target-width wrapping.

---

# 17. Range endpoint evaluation order

Slice construction evaluates:

```text
1. source expression
2. start expression when present
3. end expression when present
4. source-reference validity when dynamically required
5. range validation/normalization
6. slice borrow/reborrow creation
```

Every source/end-point expression evaluates exactly once.

This ordering also applies under `try`.

`try` converts the range failure.

It does not catch stale direct-reference failure.

---

# 18. Runtime RangeError mapping

P16 locks the minimum `RangeError` mapping for direct slice-range syntax:

```text
negative runtime start or end
    -> RangeError.InvalidRange

explicit start greater than explicit represented end
    -> RangeError.StartAfterEnd

start outside source extent
exclusive end > source length
inclusive end >= source length
    -> RangeError.OutOfBounds

other range-representation failure that cannot be represented safely
    -> RangeError.InvalidRange
```

Compile-time-provable invalid ranges remain compile-time errors.

---

# 19. Runtime range failure precedence

When multiple runtime range conditions could fail, use deterministic order:

```text
1. invalid endpoint representation / negative endpoint
2. explicit start-after-end
3. source-bound violation
```

This determines the exact `RangeError` value on the fallible path.

Optimization must preserve the same observable error.

---

# 20. Ordinary range failure

Ordinary dynamic slice construction that cannot be proven valid is
panic-capable.

It uses the stable panic reason:

```text
panic.bounds
```

with operation provenance:

```text
slice-range
```

P16 reuses the existing high-level:

```text
sec.fail.bounds
```

endpoint.

No new panic category is required.

---

# 21. Fallible range failure

For:

```sec
try ref source[start..<end]
```

range failure produces:

```text
RangeError
```

using the canonical ordinary enum representation from P11.

No `sec.fail.bounds` is emitted on the handled range-failure edge.

Reference-generation failure remains separate and is not converted by `try`.

---

# 22. Local range handlers

P10/P11 local handler flow applies unchanged:

```sec
let part := try ref source[start..<end] {
    Err(RangeError.StartAfterEnd) => fallback
    Err(RangeError.OutOfBounds) => fallback
    Err(RangeError.InvalidRange) => fallback
}
```

Catch-all follows the existing handler rules.

No slice-specific handler engine is introduced.

---

# 23. Naked range propagation

Inside:

```text
Result[U, RangeError]
```

naked:

```sec
let part := try ref source[start..<end]
```

may propagate the exact RangeError.

No automatic wrapper/union inference.

---

# 24. Current Sema slice debt

Current Sema:

```text
uses SliceType plus outer ReferenceType for source ref slices
stores slice Place bounds as int64
normalizes nested constant slices through int64 arithmetic
checks slice constants through integerExpressionInt64
```

P14/P15 already remove `int64` as canonical array/place precision.

P16 completes the slice-range side of that migration.

---

# 25. Canonical static slice bounds

Static normalized range endpoints use arbitrary precision.

Recommended:

```go
type StaticSliceRange struct {
    Start        *big.Int
    EndExclusive *big.Int
}
```

Values are immutable copies.

No host-width truncation.

---

# 26. Dynamic slice-range identity

Add a stable compile-time analysis identity:

```go
type SliceRangeID uint32
```

A dynamic range projection may refer to:

```text
SliceRangeID
```

rather than source text.

This identity is compile-time only.

It is not a runtime tag.

---

# 27. PlaceSlice modernization

P15 Place slice projections must use:

```text
arbitrary-precision static bounds
or
dynamic SliceRangeID
```

instead of canonical `int64` fields.

Existing static half-open normalization remains.

Dynamic ranges remain conservatively overlapping unless analysis proves
otherwise.

---

# 28. Nested slice range composition

For statically known parent and child ranges:

```text
parent = [P0, P1)
child  = [C0, C1) relative to parent

absolute child =
    [P0 + C0, P0 + C1)
```

Use arbitrary-precision checked semantic arithmetic.

For dynamic bounds:

```text
preserve symbolic range identity
do not guess disjointness
```

---

# 29. Empty range place semantics

A statically known empty slice range:

```text
[start, start)
```

borrows no elements.

Two empty ranges do not conflict merely because their owner is the same.

The slice value still retains owner/origin/lifetime facts.

---

# 30. Slice disjointness

Static half-open ranges can be proven disjoint when:

```text
left.end <= right.start
or
right.end <= left.start
```

This enables disjoint mutable slice borrows where the existing borrow rules
permit them.

Dynamic/symbolic ranges remain:

```text
PotentiallyOverlapping
```

unless a proof says otherwise.

---

# 31. Slice value facts

Add canonical Semantic IR facts:

```go
type SliceFacts struct {
    Element           TypeID
    Mutable           bool

    OriginPlaces      []PlaceID
    StorageIdentities []StorageIdentityID
    AddressSpace      AddressSpaceID

    Borrow             BorrowID
    Lifetime           LifetimeID

    Validity           ReferenceValidityPolicy
    EpochDependency    EpochDependencyID
    Relocation         RelocationClass

    RuntimeLength      ValueID

    StaticRange        *StaticSliceRange
    DynamicRange       SliceRangeID
}
```

Exact storage may be an analysis side table.

The semantic content is required.

---

# 32. Source-type to Semantic IR mapping

Source:

```text
ref T[]
```

maps to shared slice semantic value type.

Source:

```text
ref mut T[]
```

maps to mutable slice semantic value type.

The internal Sema representation may temporarily remain:

```text
ReferenceType(Element = SliceType)
```

for frontend compatibility.

The Semantic IR builder must use a read-only resolved slice plan rather than
infer source meaning from nested type-shape heuristics.

---

# 33. `ResolvedSliceCreationPlan`

Add a read-only Sema plan keyed by the source ref expression whose operand is a
slice-range expression.

Recommended:

```go
type ResolvedSliceSourceKind string

const (
    SliceSourceFixedArray   ResolvedSliceSourceKind = "fixed-array"
    SliceSourceSlice        ResolvedSliceSourceKind = "slice"
    SliceSourceDynamicArray ResolvedSliceSourceKind = "dynamic-array"
)

type ResolvedSliceMode string

const (
    SliceShared  ResolvedSliceMode = "shared"
    SliceMutable ResolvedSliceMode = "mutable"
)
```

---

# 34. Slice range check kind

Recommended:

```go
type SliceRangeCheckKind string

const (
    SliceRangeProvenSafe   SliceRangeCheckKind = "proven-safe"
    SliceRangeRuntimeCheck SliceRangeCheckKind = "runtime-check"
)
```

Proof kinds may reuse:

```text
constant
range
branch
contract
analysis
source-open-bound
```

---

# 35. Range endpoint plan

Recommended:

```go
type ResolvedSliceEndpoint struct {
    Present       bool
    Type          Type
    Signed        bool
    Constant      *big.Int
    ConstantKnown bool
}
```

The plan also records:

```text
inclusive/exclusive upper bound
```

---

# 36. Slice creation plan contents

Recommended:

```go
type ResolvedSliceCreationPlan struct {
    Mode             ResolvedSliceMode
    SourceKind       ResolvedSliceSourceKind
    ElementType      Type

    SourcePlace      Place
    SourceSliceType  Type

    Start            ResolvedSliceEndpoint
    End              ResolvedSliceEndpoint
    EndExclusive     bool

    CheckKind        SliceRangeCheckKind
    ProofKind        string

    StaticRange      *StaticSliceRange

    BorrowID         BorrowID
    LifetimeID       LifetimeID

    ReferenceFacts   ResolvedReferencePlan

    ErrorType        Type
}
```

For fallible dynamic range:

```text
ErrorType = RangeError
```

---

# 37. Read-only slice-plan query

Recommended:

```go
func (a *Analyzer) ResolvedSliceCreationPlanOf(
    expr *ast.RefExpression,
) (ResolvedSliceCreationPlan, bool)
```

It must not:

```text
register a new borrow
re-resolve the place
re-evaluate endpoint constants
recompute range proof
mutate Analyzer
```

---

# 38. Bare slice-range plan

If useful for diagnostics/tools, expose a separate read-only plan keyed by:

```text
*ast.SliceExpression
```

but reference creation remains the semantic operation that produces the
source-level slice value.

Do not let Semantic IR construct a source slice merely because a bare
SliceExpression exists.

---

# 39. Slice copy/move classification

Canonical:

```text
shared slice:
    CopyTrivial

mutable slice:
    MoveOnly
```

Both:

```text
TriviallyDestructible
```

Destruction ends borrow obligations but never destroys elements.

---

# 40. Shared slice copy

Add Semantic IR:

```text
SliceCopySharedOp
```

It:

```text
copies the bounded non-owning reference value
preserves storage identity
preserves range
preserves epoch dependency
does not copy elements
does not create mutable authority
```

Holder/lifetime metadata follows P15 shared-reference copy semantics.

---

# 41. Mutable slice move

Add:

```text
SliceMoveMutableOp
```

It:

```text
moves the exclusive slice-reference value
transfers holder obligation
preserves referent ownership externally
preserves range/storage/epoch facts
```

The source binding becomes unavailable according to Sema move rules.

---

# 42. Slice borrowing from fixed array

Add:

```text
SliceBorrowSharedOp
SliceBorrowMutableOp
```

Inputs conceptually:

```text
fixed-array place
normalized start
normalized length
```

Result:

```text
shared/mutable slice
```

Mutable borrow requires a writable place.

No element copy.

No backing allocation.

---

# 43. Slice reborrow

Add:

```text
SliceReborrowSharedOp
SliceReborrowMutableOp
```

Source:

```text
existing slice
normalized start
normalized length
```

Rules:

```text
shared source -> shared only
mutable source -> shared or mutable
lifetime cannot grow
range cannot grow
authority cannot grow
storage identity remains compatible
epoch dependency remains compatible
```

---

# 44. Slice validity interaction

A slice is a direct safe reference category.

P15 validity policy applies:

```text
proven
dynamic-epoch
```

P16 does not create a second generation system.

---

# 45. Dynamic source-slice validation order

For operations that semantically observe or derive access from a dynamically
validated source slice:

```text
evaluate all source operands first
validate source slice
then perform range/index/element operation
```

This includes:

```text
slice reborrow
slice indexing
slice element borrowing
slice length observation
```

Shared copy/mutable move/end-borrow preserve the reference value and do not by
themselves dereference backing storage.

---

# 46. `try` and stale source slices

`try` converts:

```text
slice range failure
slice index failure
```

It does not convert:

```text
stale direct slice reference
```

If the source slice requires dynamic generation validation and is stale:

```text
panic.invalid-reference-generation
```

occurs before the fallible spatial operation produces RangeError/IndexError.

---

# 47. Slice length property

Source:

```sec
view.len
```

returns:

```text
uint
```

and represents the runtime slice element count.

Add Semantic IR:

```text
SliceLengthOp
```

No backing element is read.

P16 still treats it as observing the safe slice value for dynamic-validity
guard purposes.

---

# 48. Compiler-known `len(slice)`

Source:

```sec
len(view)
```

returns:

```text
int
```

according to the existing core rule.

P16 represents this separately as:

```text
SliceLengthIntOp
```

Do not silently reinterpret it as `.len`.

Do not silently truncate `uint` length to `int`.

---

# 49. Existing `len` representability gap

Current normative files define:

```text
slice.len -> uint
len(slice) -> int
```

but do not define what happens when a runtime slice length is representable by
`uint` and not by `int`.

P16 does not invent:

```text
saturation
wrapping
implicit panic
new maximum slice length
changed return type
```

Until the normative rule is synchronized, the new P16 pipeline may emit
`SliceLengthIntOp` only when Sema/plan facts prove the result representable by
`int`.

Otherwise:

```text
UnsupportedFeatureError:
    sequence len(...) int representability rule is unresolved
```

This is an implementation guard, not a source-language semantic decision.

`.len -> uint` remains fully supported.

---

# 50. Slice indexing is a place operation

For:

```sec
view[index]
```

the semantic result before use context is:

```text
element place
```

The use context then chooses:

```text
copy-trivial read
mutable write
shared ref borrow
mutable ref borrow
```

No slice-specific ownership transfer is invented.

---

# 51. `ResolvedSliceIndexPlan`

Add a read-only Sema plan.

Recommended:

```go
type ResolvedSliceIndexPlan struct {
    SliceType      Type
    ElementType    Type
    Mutable        bool

    IndexType      Type
    IndexSigned    bool
    ConstantIndex  *big.Int

    CheckKind      ArrayIndexCheckKind
    ProofKind      ArrayIndexProofKind
    UseKind        ArrayIndexUseKind
    Action         ResolvedArrayTransferAction

    ErrorType      Type
}
```

Reuse P14 check/use/action enums when practical.

Do not create synonymous duplicate enums.

---

# 52. Slice index bounds

A valid index satisfies:

```text
0 <= index < slice.len
```

Signed negative index is invalid.

Constant/index proof uses arbitrary precision.

If slice length is statically known through local range provenance, a constant
invalid index is a compile-time error.

Otherwise runtime validation remains.

---

# 53. Slice index failure

Ordinary unproven slice index:

```text
panic.bounds
operation = slice-index
```

Fallible:

```sec
try view[index]
```

produces:

```text
IndexError.OutOfBounds
```

using the canonical P11 enum.

No parallel `BoundsError` type.

---

# 54. Slice element place

Add Semantic IR:

```text
SliceIndexInBoundsOp
SliceElementPlaceOp
```

`SliceElementPlaceOp` produces a P15 Place.

Authority:

```text
shared slice -> read-only element place
mutable slice -> read/write element place
```

---

# 55. Slice read

For:

```sec
let value := view[index]
```

P16:

```text
slice/index validity
element place
P15 PlaceReadOp
```

Complete P16 whole-value read supports:

```text
CopyTrivial element
```

only.

Moving an element out through slice indexing remains invalid/deferred.

---

# 56. Slice write

For:

```sec
writable[index] = value
```

P16:

```text
validate source slice
validate index
evaluate RHS completely
form writable element place
P15 PlaceWriteOp
```

The slice does not own the element.

The write mutates the owner's element in place.

P16 complete path requires trivial replacement semantics.

---

# 57. Slice element borrow

Shared:

```sec
let element := ref view[index]
```

Mutable:

```sec
let element := ref mut writable[index]
```

P16:

```text
slice element place
P15 ref.borrow_shared / borrow_mut
```

No whole-slice or whole-owner copy.

---

# 58. Element operation evaluation order

For:

```sec
view[NextIndex()]
```

canonical order:

```text
1. evaluate slice expression once
2. evaluate index once
3. validate dynamic slice reference when required
4. bounds validate unless proven safe
5. form element place
6. perform read/write/borrow selected by context
```

For assignment, the RHS is fully evaluated before mutation commits according to
the existing assignment rule.

---

# 59. Slice range place provenance

A slice value retains normalized range provenance relative to ultimate known
storage when statically available.

Indexing a slice with constant `i` may compose to:

```text
ultimateIndex = sliceStart + i
```

using arbitrary precision when both are statically known.

This improves disjoint-borrow reasoning without changing runtime representation.

---

# 60. Slicing a slice

For:

```sec
let part := ref view[start..<end]
```

the result is a reborrow.

It does not:

```text
copy elements
extend lifetime
change storage identity
widen range
gain mutable authority
```

---

# 61. Mutable slice reborrow

For:

```sec
let part := ref mut writable[start..<end]
```

source must have mutable slice authority.

The derived exclusive borrow applies to the normalized subrange.

The original mutable slice may not be used incompatibly during the reborrow live
range as resolved by Sema.

---

# 62. Shared reborrow from mutable slice

Allowed:

```sec
let part := ref writable[start..<end]
```

The result is shared.

Authority is narrowed.

No path exists from shared slice to mutable slice in safe code.

---

# 63. Slice lifetime end

P16 reuses/generalizes P15:

```text
ReferenceEndBorrowOp
sec.ref.end_borrow
```

for slice BorrowID/LifetimeID.

No duplicate slice-specific borrow-end operation is required.

Destroying/dropping a slice value:

```text
does not destroy elements
does not release backing storage
does not reset arena
```

It ends the relevant borrow holder/lifetime facts.

---

# 64. Slice parameter semantics

High-level function signatures may use:

```text
ref T[]
ref mut T[]
```

as P16 slice types.

Passing a fixed array requires explicit source syntax:

```sec
Process(ref values[..])
```

No implicit array-to-slice conversion.

---

# 65. Existing slice argument

Passing a shared slice to shared slice parameter follows shared slice copy or
reborrow semantics as resolved by Sema.

Passing mutable slice to mutable parameter follows move/reborrow semantics.

P16 must not silently copy mutable slice descriptors.

---

# 66. Returning slices

A returned slice must have non-local allowed origin.

P16 extends P15 returned-reference summaries with slice-specific data:

```text
element type
shared/mutable authority
source parameter/storage identity class
normalized known range when expressible
runtime range relation when representable
epoch dependency
relocation class
```

Returning a local-array slice remains invalid.

---

# 67. Returned range precision

When a returned slice range is statically expressible relative to an input
slice/array, retain it.

When not:

```text
retain the correct origin
degrade range precision conservatively
```

Do not fabricate a narrower range.

Caller-side borrow analysis may treat an unknown returned range as potentially
overlapping the full source extent.

---

# 68. Arena/epoch slices

A slice borrowed from arena storage inherits the arena invalidation dependency.

If static analysis proves the slice does not cross reset/release:

```text
validity may be proven
```

Otherwise the P15 dynamic-epoch policy applies where the selected safe profile
supports it.

P16 introduces no new arena-generation model.

---

# 69. Relocation correctness

A direct slice cannot remain valid across backing relocation unless:

```text
relocation is forbidden
the representation is updated/indirected
or another semantics-preserving mechanism exists
```

Generation matching alone is insufficient.

P16 preserves P15 relocation class.

---

# 70. Slice reference equality

Source slice `==` and `!=` remain unsupported in Sec 0.1.

Do not reuse `sec.ref.compare` to expose slice equality.

This avoids choosing between:

```text
descriptor identity
storage/range identity
content equality
```

before the language defines it.

---

# 71. Membership boundary

Source membership:

```text
value in slice
```

is already type-checked by the frontend.

P16 does not implement membership lowering.

It remains a later aggregate/operator package.

---

# 72. Iteration boundary

Slices may participate in `for`.

P16 establishes the slice value/index/place primitives required by future
iteration lowering.

It does not add the complete loop/iterator lowering here.

Mutable iteration and disjoint per-element borrows remain governed by borrow
rules.

---

# 73. Core methods boundary

P16 primitives are sufficient to support future/core implementations of:

```text
IsEmpty
First
Last
Slice
```

without introducing special representation semantics.

P16 does not require dedicated MLIR ops for each method.

`Reverse` and iterators remain outside the P16 minimum package.

---

# 74. Pointer/FFI boundary

P16 does not implement safe slice `.ptr` lowering.

Raw pointer extraction:

```text
requires unsafe
does not preserve slice safety automatically
does not create FFI-stable slice ABI
```

Slices never cross FFI directly without an explicit foreign wrapper.

---

# 75. Dynamic owning array boundary

A bare:

```text
T[]
```

is a sized owning descriptor in the canonical layout rules.

P16 does not implement it.

Do not reuse:

```text
!sec.slice<T>
```

as the owning dynamic array representation.

Owning dynamic arrays require separate ownership/allocation/capacity/destruction
semantics.

---

# 76. Semantic IR slice types

Recommended canonical semantic type kinds:

```text
SharedSliceType
MutableSliceType
```

or one slice type with explicit immutable semantic mode.

The source distinctions must remain visible.

Do not encode mutable slice merely as a boolean on an otherwise copyable shared
value if that would lose move-only classification.

---

# 77. Semantic IR slice operations

Add:

```text
SliceRangeNormalizeOp
SliceRangeCheckOp

SliceBorrowSharedOp
SliceBorrowMutableOp
SliceReborrowSharedOp
SliceReborrowMutableOp

SliceCopySharedOp
SliceMoveMutableOp

SliceLengthOp
SliceLengthIntOp

SliceIndexInBoundsOp
SliceElementPlaceOp
```

Reuse P15:

```text
ReferenceIsValidOp
ReferenceGenerationFailureOp
ReferenceEndBorrowOp
PlaceReadOp
PlaceWriteOp
ReferenceBorrowSharedOp
ReferenceBorrowMutableOp
```

---

# 78. `SliceRangeNormalizeOp`

Use when Sema has proven the range safe.

Inputs:

```text
source length
present endpoint values
```

Metadata:

```text
open/closed endpoints
inclusive/exclusive upper bound
proof provenance
endpoint signedness/types
```

Outputs:

```text
normalized start: uint
normalized length: uint
```

No failure result.

---

# 79. `SliceRangeCheckOp`

Use when runtime range validation is required.

Inputs match the source range operands.

Outputs:

```text
normalized start: uint
normalized length: uint
failed: bool
error: RangeError
```

The operation is total.

On success:

```text
normalized outputs are valid
error output is ignored
```

On failure:

```text
error is canonical RangeError
normalized outputs are not consumed
```

---

# 80. Range-check canonical CFG

```text
evaluate source/endpoints
validate dynamic source slice if required

range_check
    -> start
    -> length
    -> failed
    -> RangeError

failed true:
    ordinary:
        bounds panic endpoint
    fallible:
        Result/local handler error flow

failed false:
    borrow/reborrow slice using start + length
```

---

# 81. `SliceIndexInBoundsOp`

Input:

```text
slice value
index value
```

Result:

```text
bool
```

Meaning:

```text
signed index:
    index >= 0 && index < slice.length

unsigned index:
    index < slice.length
```

No physical address calculation.

---

# 82. `SliceElementPlaceOp`

Inputs:

```text
slice
index
```

Output:

```text
P15 Place<T, authority>
```

Runtime-check form requires guard.

Proven-safe form requires proof provenance.

The element place carries slice storage/range origin facts.

---

# 83. Slice guard verifier

Add:

```bash
--sec-verify-slice-guards
```

It verifies:

```text
range-check failure flag branch direction
normalized range outputs used only on success
RangeError used only on failure
source slice identity preserved
index guard uses same slice/index
element place is on bounds-true path
dynamic reference validity dominates slice observation/access
mutable operations require mutable slice
range narrowing
```

It does not redo Sema range analysis.

---

# 84. Reference guard verifier extension

Extend:

```text
--sec-verify-reference-guards
```

to accept P16 direct slice types as direct safe-reference values.

Dynamic-validity slice operations protected by it include:

```text
slice.len
slice.len_int
slice reborrow
slice range on existing slice
slice index
slice element place
```

Copy/move/end-borrow do not require backing-storage observation.

---

# 85. Place verifier extension

Extend:

```text
--sec-verify-places
```

for:

```text
slice element place
normalized slice range projection
read-only/shared authority
read-write/mutable authority
no authority widening
```

---

# 86. Borrow verifier extension

Extend:

```text
--sec-verify-borrow-semantics
```

for:

```text
shared slice copy
mutable slice move-only
shared reborrow
mutable reborrow
range narrowing
disjoint static mutable ranges
end-borrow
returned-slice origin
```

---

# 87. Sec MLIR schema version 12

Compiler-generated high-level Sec MLIR uses:

```mlir
sec.dialect_version = 12 : i32
```

Schema versions 1 through 11 remain regression inputs.

Schema v12 adds:

```text
!sec.slice<T>
!sec.slice_mut<T>

sec.slice.range_normalize
sec.slice.range_check

sec.slice.borrow_shared
sec.slice.borrow_mut
sec.slice.reborrow_shared
sec.slice.reborrow_mut

sec.slice.copy_shared
sec.slice.move_mut

sec.slice.len
sec.slice.len_int

sec.slice.index_in_bounds
sec.slice.element_place
```

Existing operations extended:

```text
sec.ref.is_valid accepts direct slices
sec.ref.end_borrow accepts slice borrow IDs
sec.fail.bounds operation accepts slice-index and slice-range
```

---

# 88. `!sec.slice<T>`

High-level shared slice type.

Semantic facts:

```text
shared
non-owning
bounded
runtime-length
copyable
trivially destructible
```

Length/origin/epoch are not type parameters.

---

# 89. `!sec.slice_mut<T>`

High-level mutable slice type.

Semantic facts:

```text
exclusive mutable
non-owning
bounded
runtime-length
move-only
trivially destructible
```

Length/origin/epoch are not type parameters.

---

# 90. Why length is not a type parameter

Two runtime slices with different lengths have the same source type:

```text
ref T[]
```

Length is a runtime value/fact, not type identity.

Do not encode:

```text
!sec.slice<T,N>
```

for ordinary slices.

---

# 91. Slice operation metadata

Borrow/reborrow operations carry P15-compatible semantic metadata:

```text
sec.borrow_id
sec.lifetime_id
sec.validity_policy
sec.validity_proof
sec.storage_identity
sec.epoch_dependency
sec.relocation_class
sec.address_space
sec.reference_origin
```

and P16 range provenance where useful:

```text
sec.slice_range_id
sec.slice_static_start
sec.slice_static_end_exclusive
```

Static numeric attrs use arbitrary-precision canonical decimal representation.

---

# 92. `sec.slice.range_normalize`

Operands:

```text
source_length
optional start
optional end
```

Results:

```text
uint start
uint length
```

Required attrs describe:

```text
bound presence
endpoint signedness
exclusive/inclusive end
proof kind
```

No runtime failure.

---

# 93. `sec.slice.range_check`

Operands:

```text
source_length
optional start
optional end
```

Results:

```text
uint normalized_start
uint normalized_length
i1 failed
RangeError error
```

Verifier requires canonical RangeError type identity.

---

# 94. `sec.slice.borrow_shared`

Operands:

```text
compatible contiguous source place
normalized start
normalized length
```

Result:

```text
!sec.slice<T>
```

Source P16 complete support:

```text
fixed-array place
```

No allocation.

---

# 95. `sec.slice.borrow_mut`

Requires writable contiguous fixed-array place.

Result:

```text
!sec.slice_mut<T>
```

No allocation.

---

# 96. `sec.slice.reborrow_shared`

Source:

```text
!sec.slice<T>
or
!sec.slice_mut<T>
```

plus normalized subrange.

Result:

```text
!sec.slice<T>
```

Range/lifetime/authority cannot widen.

---

# 97. `sec.slice.reborrow_mut`

Source:

```text
!sec.slice_mut<T>
```

Result:

```text
!sec.slice_mut<T>
```

No shared-source mutable reborrow.

---

# 98. `sec.slice.copy_shared`

```text
!sec.slice<T> -> !sec.slice<T>
```

Copies only the direct shared slice-reference value.

No elements copied.

---

# 99. `sec.slice.move_mut`

```text
!sec.slice_mut<T> -> !sec.slice_mut<T>
```

Moves exclusive holder semantics.

No elements moved.

---

# 100. `sec.slice.len`

```text
!sec.slice<T> or !sec.slice_mut<T>
    -> !sec.uint
```

before P6 target scalar resolution.

It returns runtime element length.

---

# 101. `sec.slice.len_int`

```text
!sec.slice<T> or !sec.slice_mut<T>
    -> !sec.int
```

It represents the distinct compiler-known:

```sec
len(slice)
```

surface.

Compiler generation currently requires a proven representability fact as
described in this package.

No silent narrowing.

---

# 102. `sec.slice.index_in_bounds`

Operands:

```text
slice
integer index
```

Result:

```text
i1
```

Required:

```text
index_signed
```

No address computation.

---

# 103. `sec.slice.element_place`

Operands:

```text
slice
index
```

Result:

```text
!sec.place<T,"ro">
```

for shared slice or:

```text
!sec.place<T,"rw">
```

for mutable slice.

Required:

```text
bounds_kind
bounds_proof
```

---

# 104. `sec.fail.bounds` extension

Canonical operation values now include:

```text
fixed-array-index
slice-index
slice-range
```

All use:

```text
panic.bounds
```

as semantic panic reason.

Do not introduce a separate physical trap selection.

---

# 105. Range guard verifier rules

For `sec.slice.range_check`:

```text
failed has exactly the canonical branch use in compiler-generated form
true edge is failure
false edge is success
start/length are consumed only under success dominance
error is consumed only under failure dominance
source/endpoints dominate the check
```

---

# 106. Index guard verifier rules

For runtime slice index:

```text
same slice SSA
same index SSA
index_in_bounds dominates element_place
element_place is on true edge
failure edge reaches bounds failure or IndexError flow
```

For proven-safe:

```text
non-empty proof provenance required
```

---

# 107. Dynamic validity plus range guard

When source is a dynamically validated slice:

```text
ref.is_valid
    false -> fail.reference_generation
    true  -> range normalization/check
```

The range failure is considered only after direct-reference validity succeeds.

---

# 108. Dynamic validity plus index guard

Canonical:

```text
evaluate slice
evaluate index
ref.is_valid
    false -> fail.reference_generation
    true  -> slice.index_in_bounds
                false -> bounds/IndexError path
                true  -> slice.element_place
```

This preserves P15 stale-reference semantics under `try`.

---

# 109. Proven source slice

When P15 proves temporal validity:

```text
no required ref.is_valid
```

Only spatial range/index checks remain when not independently proven.

---

# 110. `.len` and dynamic validity

P16 treats `.len`/`len(slice)` as semantic observation of a direct safe slice.

For a dynamically validated slice:

```text
reference validity must dominate the length observation
```

This keeps stale ordinary direct slices from remaining semantically observable
through metadata operations.

---

# 111. Copy/move without descriptor observation

Shared slice copy and mutable slice move may preserve the reference value without
performing backing-storage validation.

Any later observing/deriving operation performs the required P15 validity check.

---

# 112. P6 compatibility

P6 may resolve target-sized scalar element types through the slice wrapper while
preserving:

```text
slice wrapper
shared/mutable mode
reference facts
```

It also resolves:

```text
slice.len result !sec.uint
slice.len_int result !sec.int
```

to plan widths.

---

# 113. P8 compatibility

P8 must not recursively signless-normalize semantic element types inside:

```text
!sec.slice
!sec.slice_mut
```

Slice reference semantics remain high-level.

---

# 114. P13 compatibility

P13 structs may contain shared slices as trivial-copy fields where origin
tracking supports contained references.

A struct containing a mutable slice is move-only according to recursive copy
classification and remains outside P13's trivial-copy whole-value path where
appropriate.

No slice backing storage is embedded in the struct.

---

# 115. P14 compatibility

P16 fixed-array-to-slice construction consumes:

```text
P14 fixed-array type
P15 fixed-array place
P14/P15 arbitrary-precision bounds facts
```

It does not load/copy the fixed array.

---

# 116. P15 compatibility

P16 reuses:

```text
Place
StorageIdentity
InvalidationDomain
EpochDependency
BorrowID
LifetimeID
ReferenceValidityPolicy
RelocationClass
AddressSpace
reference failure endpoint
borrow end
place read/write
element ref borrowing
```

No parallel lifetime/provenance system.

---

# 117. P12 compatibility

Match values/union payloads may contain slice values according to ordinary
copy/move classifications.

P16 adds no slice pattern semantics.

---

# 118. Effect analysis

Ordinary unproven slice range:

```text
MayPanic
panic.bounds
```

Fallible slice range:

```text
RangeError flow
no bounds panic for that range check
```

Ordinary unproven slice index:

```text
MayPanic
panic.bounds
```

Fallible slice index:

```text
IndexError flow
no bounds panic for that index check
```

Dynamic source validity may independently contribute:

```text
panic.invalid-reference-generation
```

Operand/call effects remain.

---

# 119. `@noPanic`

A slice operation may remain `@noPanic` when:

```text
temporal validity is proven
and
range/index validity is proven
```

or when the spatial failure is converted to explicit Result and no other panic
path remains.

`try` does not remove dynamic stale-reference panic.

---

# 120. Unsafe does not disable slice checks

Inside `unsafe`:

```text
ordinary slice ranges remain checked
ordinary slice indexes remain checked
ordinary slice reference validity remains required
```

Unchecked memory access belongs to RawPtr semantics.

---

# 121. No physical descriptor lowering

P16 does not lower slices to:

```text
LLVM struct
pointer + length
pointer + length + epoch
MemRef
span struct
fat pointer ABI
handle tuple
```

Those are later plan/profile representation choices.

---

# 122. No FFI slice ABI

`!sec.slice` and `!sec.slice_mut` are not foreign ABI types.

FFI wrappers must later expose explicit foreign-compatible components and
contracts.

---

# 123. No dynamic owning array conflation

Do not represent owning:

```text
T[]
```

as:

```text
!sec.slice<T>
```

The former owns a dynamic backing store/descriptor.

The latter borrows.

---

# 124. No element ownership transfer

P16 does not implement moving an element out of a slice.

Slice element access cannot transfer backing-owner ownership merely because the
element is addressable.

Borrow the element instead.

---

# 125. Required rule synchronization tests

Verify:

```text
fallible direct slice syntax requires explicit ref/ref mut
RangeError mapping is stable
panic.bounds remains ordinary range/index panic reason
stale direct slice is not converted by try
len(slice) unresolved representability is never silently truncated
```

---

# 126. Required Sema static-range tests

```text
[..]
[start..]
[..end]
[..<end]
[start..end]
[start..<end]
exclusive empty range
inclusive one-element range
negative constant rejected
descending constant rejected
fixed-array out-of-bounds constant rejected
arbitrary-precision endpoints
static nested slice composition
empty range place
disjoint static ranges
index outside static slice range
```

---

# 127. Required Sema dynamic-range tests

```text
runtime signed endpoints
runtime unsigned endpoints
runtime negative -> InvalidRange path
runtime start-after-end -> StartAfterEnd
runtime out-of-bounds -> OutOfBounds
dynamic range identity
dynamic ranges conservatively overlap
open-end runtime slice source
source/start/end evaluation plan order
```

---

# 128. Required slice creation-plan tests

```text
fixed array shared
fixed array mutable
slice shared reborrow
mutable slice shared reborrow
mutable slice mutable reborrow
shared-to-mutable rejected
dynamic owning array source recorded but new IR unsupported
BorrowID/LifetimeID
storage identity
epoch dependency
static normalized range
runtime range
read-only plan query
```

---

# 129. Required Semantic IR slice tests

```text
shared slice type
mutable slice type
borrow shared
borrow mutable
reborrow shared
reborrow mutable
copy shared
move mutable
range normalize
range check
slice len
slice len int guarded case
index bounds
element place
empty slice
end borrow
```

---

# 130. Required ordinary range CFG tests

```text
runtime valid exclusive
runtime valid inclusive
InvalidRange failure
StartAfterEnd failure
OutOfBounds failure
failure -> BoundsFailure operation slice-range
success -> slice borrow
normalized outputs absent from failure path
```

---

# 131. Required fallible range tests

```text
Result[ref int[], RangeError]
Result[ref mut int[], RangeError]
naked propagation
local specific handler
catch-all handler
no sec.fail.bounds on handled range failure
dynamic stale-source failure remains reference-generation panic
```

---

# 132. Required slice index tests

```text
shared copy-trivial read
mutable copy-trivial read
mutable trivial write
shared write rejected
shared element ref
mutable element ref mut
shared-to-ref-mut element rejected
constant known-safe
constant known-invalid when length statically known
runtime signed index
runtime unsigned index
negative runtime
index == len
empty slice index
```

---

# 133. Required fallible slice index tests

```text
try shared slice index -> IndexError.OutOfBounds
try mutable slice index -> IndexError.OutOfBounds
naked propagation
local handler
no bounds panic on handled failure
stale source is not converted by try
```

---

# 134. Required copy/move tests

```text
shared slice copy accepted
copy does not copy elements
mutable slice copy rejected
mutable slice move accepted
mutable source binding unavailable after move
shared reborrow from shared
shared reborrow from mutable
mutable reborrow from mutable
```

---

# 135. Required borrow/disjointness tests

```text
disjoint static mutable slices coexist
overlapping static mutable slices conflict
empty static slice borrows no element
dynamic mutable ranges conservatively conflict
constant element outside static slice disjoint
nested static subrange normalization
borrow ends at last use
```

---

# 136. Required reference-validity tests

```text
proven fixed-array slice requires no generation guard
proven slice reborrow requires no generation guard
dynamic epoch slice range guarded
dynamic epoch slice index guarded
dynamic epoch len guarded
wrong slice validated rejected
failure path reaches invalid-reference-generation
64-bit logical epoch facts preserved
```

---

# 137. Required returned-slice tests

```text
return full input slice
return prefix of input slice
return static subrange
return runtime subrange with conservative range summary
return local fixed-array slice rejected
return mutable slice authority preserved
returned origin storage identity preserved
```

---

# 138. Required MLIR type/op tests

```text
!sec.slice<T> round-trip
!sec.slice_mut<T> round-trip
wide element types
nested struct element
nested fixed-array element

range_normalize
range_check
borrow_shared
borrow_mut
reborrow_shared
reborrow_mut
copy_shared
move_mut
len
len_int
index_in_bounds
element_place

schema-v11 regression
```

---

# 139. Required slice verifier tests

```text
runtime range canonical guard accepted
range failed branch reversed rejected
normalized values on failure rejected
RangeError on success rejected
wrong source slice reborrow rejected
range widening rejected
mutable reborrow from shared rejected

runtime index canonical guard accepted
element place without bounds guard rejected
wrong slice/index guard rejected
element place on false path rejected
```

---

# 140. Required P13/P14/P15 integration tests

P13:

```text
shared slice struct field
contained reference-origin preservation
mutable slice makes aggregate move-only
```

P14:

```text
fixed array -> shared slice
fixed mutable array -> mutable slice
zero-length fixed array -> empty slice
no whole-array copy
```

P15:

```text
slice element place -> shared ref
slice element place -> ref mut
slice dynamic validity
slice borrow end
slice origin identity
```

---

# 141. Required end-to-end source tests

```text
ref array[..]
ref mut array[..]
ref array[1..<4]
ref array[1..3]
empty slice
slice reborrow
mutable slice reborrow
slice .len
supported len(slice)
slice index read
slice index write
slice element borrow
try ref slice range
try slice index
return input subslice
```

No hand editing of generated IR.

---

# 142. Unsupported P16 end-to-end tests

New IR path explicitly rejects where appropriate:

```text
slice construction from owning dynamic T[]
slice physical ptr extraction
slice FFI passing
slice equality
slice ordering
slice membership lowering
slice iterator lowering
mutable Reverse lowering
move-only element read
semantic-copy element read when not yet represented
non-trivial element write/replacement
stable/weak handle-backed slice source API
RawPtr-to-slice conversion
unproven len(slice) int representability
```

Do not emit placeholder semantics.

---

# 143. Architecture rules

Non-negotiable:

```text
A slice is a bounded direct safe reference, not an owning container.

ref T[] is shared/copyable.

ref mut T[] is exclusive/move-only.

A slice never owns or destroys its elements.

Slice creation is explicit with ref/ref mut.

Fallible slice syntax remains explicit ref/ref mut syntax.

Ranges normalize to half-open [start,endExclusive).

Range provenance uses arbitrary precision for static bounds.

Dynamic range identity is compiler analysis data.

Slice borrow facts reuse P15 StorageIdentity/BorrowID/LifetimeID/epoch facts.

A sub-slice may only narrow range/lifetime/authority.

Shared cannot become mutable.

Empty slices are valid and non-null semantically.

No default slice exists.

Slice indexing forms a P15 place.

Slice reads/writes/element refs reuse P15 place/reference operations.

Range failure uses RangeError under try.

Index failure uses IndexError under try.

Ordinary range/index failure uses panic.bounds.

Stale direct slice failure is not caught by try.

Reference validity is checked before fallible spatial failure when dynamically
required.

Slice equality remains undefined.

Dynamic owning T[] remains distinct and deferred.

len(slice) int representability must never silently truncate.

No physical pointer+length representation is selected.

No FFI slice ABI is selected.

No mandatory runtime is introduced.

No LLVM dialect is generated.
```

---

# 144. Acceptance criteria

Package 16 is complete only when:

```text
[ ] baseline documents repo 152c772 + local P13/P14/P15 or newer equivalent
[ ] previous package regressions remain green
[ ] slice normative synchronization applied
[ ] Semantic IR slice amendment applied
[ ] schema-v12 dialect rulebook installed
[ ] lowering-v12 rulebook installed
[ ] explicit fallible ref/ref-mut slice syntax synchronized
[ ] RangeError mapping implemented
[ ] static range bounds use arbitrary precision
[ ] dynamic SliceRangeID implemented
[ ] PlaceSlice migrated from canonical int64 bounds
[ ] static nested slice normalization implemented
[ ] disjoint static range reasoning preserved
[ ] ResolvedSliceCreationPlan implemented
[ ] plan query is read-only
[ ] shared/mutable slice Semantic IR types implemented
[ ] SliceFacts reuse P15 reference facts
[ ] fixed-array shared slice borrow implemented
[ ] fixed-array mutable slice borrow implemented
[ ] shared slice reborrow implemented
[ ] mutable slice reborrow implemented
[ ] shared slice copy implemented
[ ] mutable slice move implemented
[ ] empty slices implemented
[ ] SliceRangeNormalizeOp implemented
[ ] SliceRangeCheckOp implemented
[ ] ordinary range panic CFG implemented
[ ] fallible RangeError CFG implemented
[ ] local/naked try range integration works
[ ] SliceLengthOp implemented
[ ] SliceLengthIntOp preserves no-truncation guard
[ ] ResolvedSliceIndexPlan implemented
[ ] slice index bounds proof/runtime classification works
[ ] SliceIndexInBoundsOp implemented
[ ] SliceElementPlaceOp implemented
[ ] slice reads/writes reuse P15 places
[ ] slice element ref/ref-mut borrowing works
[ ] ordinary slice index panic CFG implemented
[ ] fallible IndexError CFG implemented
[ ] dynamic slice validity ordering implemented
[ ] sec.ref.is_valid accepts direct slices
[ ] sec.ref.end_borrow handles slice borrows
[ ] sec.fail.bounds accepts slice-index/slice-range
[ ] --sec-verify-slice-guards registered
[ ] place/reference/borrow verifiers extended
[ ] returned-slice origin summaries implemented
[ ] P13/P14/P15 integration passes
[ ] owning dynamic T[] remains distinct/unsupported in new IR
[ ] no slice equality lowering
[ ] no FFI slice ABI
[ ] no physical slice descriptor selected
[ ] no mandatory runtime
[ ] check-sec-mlir passes
[ ] go test ./... passes
[ ] legacy paths remain operational
```

---

# 145. Required implementation report

Codex must report:

```text
1. repository HEAD implemented against
2. local/merged P13/P14/P15 status
3. previous package status
4. slice syntax/range/error synchronization
5. len(slice) representability limitation status
6. files added
7. files modified
8. PlaceSlice arbitrary-precision migration
9. SliceRangeID implementation
10. static nested range normalization
11. ResolvedSliceCreationPlan API
12. SliceFacts representation
13. shared/mutable slice type implementation
14. fixed-array borrow implementation
15. slice reborrow implementation
16. shared copy/mutable move
17. range normalize/check operations
18. RangeError mapping/CFG
19. slice length operations
20. ResolvedSliceIndexPlan API
21. slice index bounds implementation
22. element place implementation
23. P15 read/write/ref integration
24. IndexError flow
25. reference-validity ordering
26. returned-slice origin summary
27. schema-v12 types/ops
28. slice guard verifier
29. place/reference/borrow verifier extensions
30. P6/P8 compatibility
31. P13/P14/P15 integration
32. wide-type slice tests
33. empty-slice tests
34. disjoint-range tests
35. try/noPanic tests
36. unsupported dynamic-owner/FFI/ownership tests
37. CMake commands
38. exact LLVM/MLIR version
39. check-sec-mlir result
40. go test ./... result
41. end-to-end source -> schema-v12 results
42. deviations
43. recommendations for Package 17
```

---

# 146. Package 17 boundary

Recommended Package 17:

```text
Ownership Transfer and Destruction Semantic Core
```

Reason:

P13-P16 deliberately support only trivial whole-value transfer/replacement in
many aggregate paths.

Before canonical owning dynamic `T[]` can be implemented cleanly, the pipeline
needs explicit semantic operations for:

```text
move
semantic copy
destroy
replacement with old-value destruction
initialization state
cleanup on partially completed construction
conditional generic copyability
move-only function arguments/results
aggregate-contained ownership
```

Recommended P17 scope:

```text
MoveOp
SemanticCopyOp
DestroyOp
ReplaceOp
InitializeOp
ownership transfer IDs
resolved transfer plans from Sema
cleanup regions/edges for fallible construction
partial initialization state
reverse-order aggregate destruction hooks
non-trivial struct/array/union value paths
function argument/return transfer
no physical destructor ABI yet
```

After P17, Package 18 can implement:

```text
Owning Dynamic T[] Semantic Value Representation
```

without hiding allocation, relocation, move or destruction inside an opaque
descriptor operation.
