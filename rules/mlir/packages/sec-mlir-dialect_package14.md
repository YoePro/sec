# Sec MLIR Program - Implementation Package 14

## Package status

Implementation package for the Sec compiler.

Package ID: `SEC-MLIR-P14`\
Package title: `Fixed Array Semantic Value Representation`\
Repository: `https://github.com/YoePro/sec`\
Repository branch: `main`\
Repository sync commit used for this package: `152c772`\
Local predecessor: `SEC-MLIR-P13`\
Repository sync date: `2026-08-09`\
Semantic IR version before package: `1`\
Semantic IR version after package: `1`\
Sec MLIR dialect schema before package: `9`\
Sec MLIR dialect schema after package: `10`\
Sec MLIR lowering specification before package: `9`\
Sec MLIR lowering specification after package: `10`

Package 14 introduces canonical Semantic IR and high-level Sec MLIR
representation for fixed arrays:

```text
T[N]
```

It covers:

```text
arbitrary-precision compile-time length identity
zero-length arrays
nested fixed arrays
array literals
fixed-array literal spread
canonical array defaults
fixed-array length
fixed-array indexing
compile-time/proven bounds
runtime bounds checks
ordinary panic-capable index access
fallible try index access through IndexError
trivial element reads
trivial element replacement
trivial mutable local fixed arrays
fixed arrays inside structs/unions/functions
```

The package deliberately does not introduce:

```text
dynamic owning T[]
slices
safe references
physical array storage lowering
LLVM array representation
non-trivial element ownership/destruction
aggregate ABI lowering
```

---

# 1. Normative authority

Implementation follows:

```text
rules/collections/collections.md
rules/declarations/spread.md
rules/types/default_values.md
rules/errors/runtime_checks.md
rules/library/core-library.md
rules/foundations/operators.md
rules/memory/layout.md
rules/memory/copy_move.md
rules/memory/ownership.md
rules/memory/destruction.txt
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

1. apply `sec_fixed_array_sync_package14.md`;
2. apply `sec_semantic_ir_fixed_array_package14.md` to
   `rules/compiler/semantic_ir.txt`;
3. update `rules/mlir/sec_mlir_dialect.md` with
   `sec_mlir_dialect_package14.md`;
4. update `rules/mlir/sec_mlir_lowering.md` with
   `sec_mlir_lowering_package14.md`.

No new source syntax is introduced.

---

# 2. Local predecessor rule

Package 13 is not yet present in repository `main`.

Package 14 must therefore be implemented against:

```text
GitHub baseline:
    152c772

plus local normative/implementation package:
    SEC-MLIR-P13
```

In particular P14 assumes P13 schema-v9 concepts exist:

```text
!sec.struct
sec.struct.construct
sec.struct.spread_fields
sec.struct.extract
sec.struct.replace_field
```

If Package 13 has been merged under a newer HEAD by implementation time, Codex
must report the newer HEAD and verify that P13 semantics are unchanged before
continuing.

---

# 3. Mandatory rule synchronization: array literal spread

`rules/collections/collections.md` still contains older wording saying:

```text
No spread ... is included in the initial implementation.
```

The newer dedicated `rules/declarations/spread.md` explicitly defines and marks implemented:

```text
fixed-size array spread in array literals
multiple fixed-array spreads
compile-time expanded length
copyability validation
```

Package 14 treats the dedicated spread rule as canonical.

Remove/synchronize the stale no-spread statement.

P14 must not regress accepted fixed-array literal spread.

---

# 4. Mandatory rule synchronization: index error type

`rules/errors/runtime_checks.md` describes fallible bounds access as producing:

```text
BoundsError or the canonical equivalent
```

The canonical core rulebook already defines:

```sec
enum IndexError {
    OutOfBounds
}
```

and identifies fundamental indexing errors as core errors.

Package 14 therefore uses:

```text
IndexError.OutOfBounds
```

as the canonical fixed-element-index failure.

Do not add a second parallel `BoundsError` type.

Range/slice range failures remain a separate concern associated with
`RangeError` and the later slice package.

---

# 5. Wide builtin invariant

These remain active Sec builtin types:

```text
int128
int256
uint128
uint256
decimal128
```

They may appear as fixed-array elements.

They may also appear in nested structs/unions that are fixed-array elements.

No Package 14 document or implementation may describe these types as future,
planned, reserved, placeholder or not-yet-active.

---

# 6. Fixed array semantic identity

For:

```text
T[N]
```

type identity contains:

```text
element type identity
exact compile-time length N
```

Examples:

```text
int32[4] != int32[5]
int32[4] != uint32[4]
Pair[int128,uint256][2] != Pair[int128,uint256][3]
```

Fixed arrays are structural aggregate types.

No declaration SymbolID is required merely for the array wrapper.

Named types whose underlying type is an array remain nominal named types under
the existing named-type rules.

---

# 7. Canonical fixed-array length

`N` is:

```text
a non-negative arbitrary-precision compile-time integer
```

It must later be validated as representable by the active target's canonical:

```text
uint
```

and its total physical layout must satisfy the selected CompilationPlan.

Do not restrict the semantic length merely because the compiler host uses
`int64`.

---

# 8. Target-dependent length validity

Because `uint` is pointer-sized:

```text
32-bit CompilationPlan:
    N <= uint32.Max

64-bit CompilationPlan:
    N <= uint64.Max
```

A source program may therefore be valid for one output target and invalid for
another.

Multi-output builds validate the same exact semantic length independently for
each CompilationPlan.

Do not truncate or reuse one target's cached validation for another target.

---

# 9. Sema array-shape correction

Current Sema uses:

```go
ArrayLength int64
```

and uses:

```go
-1
```

as the dynamic owning-array sentinel.

Package 14 removes that as the canonical model.

Recommended authoritative representation:

```go
type ArrayShapeKind string

const (
    FixedArrayShape   ArrayShapeKind = "fixed"
    DynamicArrayShape ArrayShapeKind = "dynamic"
)

type Type struct {
    // existing fields...

    ArrayShape  ArrayShapeKind
    ArrayLength *big.Int
    Element     *Type
}
```

Rules:

```text
fixed array:
    ArrayShape == fixed
    ArrayLength != nil
    ArrayLength >= 0

dynamic owning array:
    ArrayShape == dynamic
    ArrayLength == nil

non-array:
    ArrayShape empty
    ArrayLength nil
```

Exact field names may differ.

---

# 10. No canonical dynamic sentinel

After P14 semantic refactoring, no correctness decision may depend on:

```text
ArrayLength == -1
```

A temporary legacy compatibility helper may translate:

```text
dynamic -> legacy -1
```

only inside old backend code.

The new Sema, Semantic IR and Sec MLIR path must use explicit array shape.

---

# 11. Migration from `int64`

Update all semantic operations that participate in fixed-array correctness:

```text
type identity
type display
sameConcreteType
generic substitution
recursive-layout checks
array literal length
spread length
compile-time bounds checking
default resolution
copy/destruction classification
membership typing
function parameter identity
diagnostics
```

to use canonical arbitrary-precision fixed length.

Legacy paths that fundamentally require `int64` must:

```text
check IsInt64()
return explicit legacy-backend unsupported/error if not representable
```

They must not make the source program semantically invalid for the new pipeline.

---

# 12. Plan-aware length validation

Sema resolves:

```text
compile-time integer
non-negative
exact arbitrary-precision N
```

Target-plan validation then checks:

```text
N <= max uint for active CompilationPlan
physical layout representability when layout is required
```

Recommended compiler flow:

```text
parse
Sema exact fixed-array length
CompilationPlan-specific fixed-array validation
Semantic IR
Sec MLIR
```

If the compiler architecture already supplies the target plan to Sema, the
checks may occur together.

There must still be one authoritative arbitrary-precision length.

---

# 13. Current literal-expansion debt

Current Sema expands fixed-array spread conceptually into:

```text
one Type entry per expanded element
```

and infers length from the resulting host slice length.

This is not the canonical P14 model.

It:

```text
scales linearly with spread length
depends on host container size
cannot represent valid large lengths cleanly
loses explicit spread provenance
```

Package 14 introduces a compact resolved literal plan.

---

# 14. `ResolvedArrayLiteralPlan`

Add a read-only Sema fact keyed by:

```text
*ast.ArrayLiteral
```

Recommended:

```go
type ResolvedArrayLiteralEntryKind string

const (
    ArrayLiteralElement ResolvedArrayLiteralEntryKind = "element"
    ArrayLiteralSpread  ResolvedArrayLiteralEntryKind = "spread"
)

type ResolvedArrayTransferAction string

const (
    ArrayTransferConstructDirect ResolvedArrayTransferAction = "construct-direct"
    ArrayTransferCopyTrivial     ResolvedArrayTransferAction = "copy-trivial"
    ArrayTransferMove            ResolvedArrayTransferAction = "move"
    ArrayTransferCopySemantic    ResolvedArrayTransferAction = "copy-semantic"
    ArrayTransferBorrowShared    ResolvedArrayTransferAction = "borrow-shared"
    ArrayTransferBorrowMutable   ResolvedArrayTransferAction = "borrow-mutable"
)
```

---

# 15. `ResolvedArrayLiteralEntry`

Recommended:

```go
type ResolvedArrayLiteralEntry struct {
    SourceIndex int
    Kind        ResolvedArrayLiteralEntryKind
    Type        Type
    Length      *big.Int
    Action      ResolvedArrayTransferAction
}
```

Meaning:

```text
ordinary element:
    Length = 1

spread:
    Type is source fixed-array type
    Length is exact source fixed-array length
```

The AST expression itself remains available through the original AST entry.

The plan stores semantic decisions, not duplicate AST nodes.

---

# 16. Literal plan result

Recommended:

```go
type ResolvedArrayLiteralPlan struct {
    ElementType Type
    Length      *big.Int
    Entries     []ResolvedArrayLiteralEntry
}
```

The total length is computed using checked arbitrary-precision addition.

Target `uint` validation is separate as described above.

---

# 17. Read-only literal query

Recommended:

```go
func (a *Analyzer) ResolvedArrayLiteralPlanOf(
    expr *ast.ArrayLiteral,
) (ResolvedArrayLiteralPlan, bool)
```

It:

```text
does not re-infer elements
does not expand spreads
does not mutate Analyzer
does not allocate O(total expanded length) semantic records
```

---

# 18. Literal inference without target

For an untyped non-empty array literal:

```text
all ordinary elements and all spread element types must resolve to one exact
element type
```

No common type through implicit conversion.

Result:

```text
ElementType[total exact length]
```

A spread contributes its source fixed-array element type.

A runtime-length source remains invalid.

---

# 19. Empty literal

```sec
[]
```

requires target fixed-array context.

Canonical valid case:

```sec
let empty: int32[0] := []
```

Without target element type:

```text
error
```

P14 preserves this rule.

---

# 20. Array literal source order

Array literal entries evaluate strictly left to right.

For:

```sec
[First(), source..., Last()]
```

evaluation is:

```text
First()
source
Last()
```

The spread source evaluates exactly once.

P14 does not lower spread by re-evaluating the source for each expanded element.

---

# 21. Array literal spread transfer

The current source rule requires array-literal spread element type to be
implicitly copyable.

P14 complete new-IR path accepts:

```text
copy-trivial
```

spread transfer.

If existing Sema accepts a source requiring semantic copy rather than trivial
copy:

```text
record the actual transfer action
new Semantic IR path returns UnsupportedFeatureError
```

Do not hide semantic copy inside one aggregate SSA operand.

Move-aware spread remains deferred.

---

# 22. Compact array construction

Do not represent a spread of `T[1000000]` as one million synthetic Semantic IR
operands.

Add a segmented construction operation.

Recommended Semantic IR:

```go
type ArrayConstructSegmentKind string

const (
    ArraySegmentElement ArrayConstructSegmentKind = "element"
    ArraySegmentSpread  ArrayConstructSegmentKind = "spread"
)

type ArrayConstructSegment struct {
    Kind       ArrayConstructSegmentKind
    Operand    ValueID
    Length     *big.Int
    Action     ResolvedArrayTransferAction
    Location   Location
}

type ArrayConstructOp struct {
    Result      Value
    ElementType TypeID
    Length      *big.Int
    Segments    []ArrayConstructSegment
    Location    Location
}
```

One source literal entry becomes one segment.

---

# 23. Segment semantics

Element segment:

```text
operand type == T
length == 1
action == construct-direct
```

Spread segment:

```text
operand type == T[M]
length == M
action == copy-trivial in P14
```

The result length equals the exact checked sum of segment lengths.

No hidden allocation.

No dynamic backing store.

---

# 24. Array default semantics

Canonical source rule:

```text
T[N] is defaultable when T is defaultable
```

with the important zero-length exception:

```text
T[0] is defaultable without constructing an element
```

Therefore P14 treats zero-length arrays as defaultable even when constructing a
T value would otherwise be impossible.

No element exists to initialize.

---

# 25. Compact `ArrayDefaultOp`

Do not materialize an O(N) Semantic IR value list merely to represent:

```sec
let mut values: int32[1000000]
```

Add:

```go
type ArrayDefaultOp struct {
    Result      Value
    ElementType TypeID
    Length      *big.Int
    Location    Location
}
```

Meaning:

```text
construct a fully initialized fixed array
each live element receives the canonical semantic default of T
elements are semantically constructed in increasing index order
```

For length zero:

```text
no element default is constructed
```

---

# 26. P14 default implementation gate

Emit `ArrayDefaultOp` when:

```text
N == 0
or
element type has a canonical infallible default
```

For `N > 0`, P14 complete lowering additionally requires:

```text
element construction introduces no unrepresented ownership/cleanup obligation
```

This includes the currently supported trivial scalar and recursively trivial
aggregate default subset.

If a valid source array default requires non-trivial cleanup:

```text
UnsupportedFeatureError in the P14 new IR path
```

Do not use zero or undef as a substitute.

---

# 27. Full initialization invariant

Every readable fixed array contains exactly:

```text
N initialized semantic elements
```

No P14 array value may expose:

```text
undef
poison
partially initialized safe element storage
```

---

# 28. Fixed-array length operation

Add Semantic IR:

```text
ArrayLengthOp
```

Input:

```text
T[N]
```

Output:

```text
uint
```

Semantic result:

```text
exact compile-time N
```

This records the compiler-known `.len` operation explicitly.

It does not read memory.

---

# 29. Core `len(...)` boundary

P14 implements the fixed-array member:

```sec
value.len
```

with result type:

```text
uint
```

It does not broaden or redefine the separate compiler-known `len(...) -> int`
surface for dynamic arrays/slices.

If fixed-array `len(...)` is already accepted by current frontend behavior, keep
it compatible but do not infer that semantic from P14 array member lowering.

---

# 30. Indexing is a place

Source:

```sec
values[index]
```

semantically identifies an element place.

It does not inherently mean:

```text
copy
move
borrow
```

P14 therefore records an index plan before selecting a read/update operation.

Borrowing remains deferred.

---

# 31. `ResolvedArrayIndexPlan`

Add a read-only Sema fact keyed by:

```text
*ast.IndexExpression
```

Recommended:

```go
type ArrayIndexCheckKind string

const (
    ArrayIndexProvenSafe    ArrayIndexCheckKind = "proven-safe"
    ArrayIndexRuntimeCheck  ArrayIndexCheckKind = "runtime-check"
)

type ArrayIndexUseKind string

const (
    ArrayIndexRead      ArrayIndexUseKind = "read"
    ArrayIndexWrite     ArrayIndexUseKind = "write"
    ArrayIndexBorrow    ArrayIndexUseKind = "borrow"
    ArrayIndexMutBorrow ArrayIndexUseKind = "mut-borrow"
)
```

---

# 32. Index proof kinds

Recommended:

```go
type ArrayIndexProofKind string

const (
    ArrayIndexProofConstant ArrayIndexProofKind = "constant"
    ArrayIndexProofRange    ArrayIndexProofKind = "range"
    ArrayIndexProofBranch   ArrayIndexProofKind = "branch"
    ArrayIndexProofContract ArrayIndexProofKind = "contract"
    ArrayIndexProofOther    ArrayIndexProofKind = "analysis"
)
```

The proof kind is compiler provenance.

It is not a source assertion.

---

# 33. Index plan contents

Recommended:

```go
type ResolvedArrayIndexPlan struct {
    ArrayType     Type
    ElementType   Type
    IndexType     Type
    IndexSigned   bool
    ConstantIndex *big.Int

    CheckKind     ArrayIndexCheckKind
    ProofKind     ArrayIndexProofKind
    UseKind       ArrayIndexUseKind
    Action        ResolvedArrayTransferAction

    ErrorType     Type
}
```

For fixed arrays:

```text
ErrorType == core IndexError
```

when runtime failure is possible.

---

# 34. Read-only index query

Recommended:

```go
func (a *Analyzer) ResolvedArrayIndexPlanOf(
    expr *ast.IndexExpression,
) (ResolvedArrayIndexPlan, bool)
```

The builder must not redo:

```text
integer index validation
constant bounds evaluation
range/contract proof
copy/move classification
fallibility classification
```

---

# 35. Arbitrary-precision constant index checking

Replace the current `int64`-based fixed-array constant bounds check with exact
integer arithmetic.

For a constant index `I` and fixed length `N`:

```text
valid iff
    I >= 0
    and
    I < N
```

Use arbitrary precision.

This applies to:

```text
int128
int256
uint128
uint256
named integer constants
```

where source typing permits them as indexes.

---

# 36. Dynamic index classification

When Sema proves:

```text
0 <= index < N
```

from:

```text
constant
named-type range contract
branch refinement
assertion refinement
other accepted analysis
```

record:

```text
CheckKind = proven-safe
```

Otherwise:

```text
CheckKind = runtime-check
```

Safe indexing remains checked even inside `unsafe`.

---

# 37. `ArrayIndexInBoundsOp`

Add a total Semantic IR operation:

```go
type ArrayIndexInBoundsOp struct {
    Array     ValueID
    Index     ValueID
    Signed    bool
    Result    Value
    Location  Location
}
```

Result:

```text
bool
```

Meaning for fixed `T[N]`:

```text
signed index:
    index >= 0 && index < N

unsigned index:
    index < N
```

No physical address computation.

---

# 38. `ArrayExtractOp`

Add:

```go
type ArrayExtractOp struct {
    Array      ValueID
    Index      ValueID
    CheckKind  ArrayIndexCheckKind
    ProofKind  ArrayIndexProofKind
    Action     ResolvedArrayTransferAction
    Result     Value
    Location   Location
}
```

P14 source read requires:

```text
Action == copy-trivial
```

Move-out remains invalid/deferred.

Borrow requires later reference IR.

---

# 39. Extract safety

For:

```text
CheckKind = proven-safe
```

the Semantic IR verifier requires recorded Sema proof provenance.

For:

```text
CheckKind = runtime-check
```

the extract must occur only on a control-flow path proven true by
`ArrayIndexInBoundsOp` for the same:

```text
array SSA value
index SSA value
```

---

# 40. `ArrayReplaceOp`

Add semantic functional replacement:

```go
type ArrayReplaceOp struct {
    Array      ValueID
    Index      ValueID
    NewValue   ValueID
    CheckKind  ArrayIndexCheckKind
    ProofKind  ArrayIndexProofKind
    Result     Value
    Location   Location
}
```

Meaning:

```text
new array value equals old array except element index is NewValue
```

No physical store is implied.

---

# 41. Replace safety boundary

P14 replacement requires:

```text
array type CopyTrivial
array type TriviallyDestructible
element type CopyTrivial
element type TriviallyDestructible
```

The new value is fully evaluated before the replacement commits.

Non-trivial old-element destruction remains deferred.

---

# 42. Index evaluation order

For a read:

```sec
values[NextIndex()]
```

preserve:

```text
1. evaluate values
2. evaluate NextIndex()
3. perform bounds validation
4. access
```

The array and index are each evaluated exactly once.

---

# 43. Indexed assignment order

For:

```sec
values[NextIndex()] = NextValue()
```

P14 must preserve exact-once target-place evaluation.

Canonical P14 flow:

```text
1. evaluate values/place root
2. evaluate NextIndex() exactly once
3. validate the element place/bounds
4. evaluate NextValue() completely
5. replace the element
6. commit the new whole array to storage
```

No destination mutation occurs before the replacement value is valid.

---

# 44. Mutable local trivial fixed arrays

Extend high-level semantic storage eligibility to:

```text
T[N]
```

when:

```text
array CopyClassification == CopyTrivial
array TriviallyDestructible == true
element value representation supported
```

Use existing semantic storage:

```text
storage.declare
storage.init
storage.load
storage.store
```

with the high-level fixed-array type.

---

# 45. P5 storage boundary

P5 must not lower:

```text
!sec.storage<!sec.array<...>>
```

to MemRef.

Fixed-array storage remains high-level until canonical physical aggregate layout
lowering exists.

Do not reuse the legacy direct `!llvm.array` representation as the P14 storage
model.

---

# 46. Indexed local replacement

For a mutable local trivial array:

```text
bounds validation
evaluate RHS
storage.load current whole array
ArrayReplaceOp
storage.store replacement whole array
```

If the array root/index expression itself has observable evaluation, evaluate it
once before the update sequence.

---

# 47. Nested fixed-array replacement

For:

```sec
matrix[row][column] = value
```

on the supported trivial subset:

```text
evaluate matrix root once
evaluate row once
validate row
evaluate column once on the selected inner-array place
validate column
evaluate RHS
load/recover root aggregate
replace inner element
replace inner array in outer array
store root once
```

The exact source-order place semantics must be preserved.

Do not generate duplicated index evaluation.

---

# 48. Ordinary runtime bounds failure

When:

```text
CheckKind == runtime-check
```

and source access is ordinary, not `try`:

```text
ArrayIndexInBoundsOp
cf.cond_br
```

Failure path ends in a high-level panic-capable endpoint:

```text
BoundsFailureOp
```

Semantic panic reason:

```text
panic.bounds
```

No backend trap is selected in P14.

---

# 49. `BoundsFailureOp`

Add Semantic IR terminator:

```go
type BoundsFailureOp struct {
    Operation string
    Location  Location
}
```

Canonical P14 operation:

```text
fixed-array-index
```

It:

```text
has no successor
is panic-capable
does not return Result
does not imply a mandatory runtime
```

---

# 50. Fallible `try` index

Source:

```sec
let value := try values[index]
```

uses the same bounds predicate.

Failure produces:

```text
IndexError.OutOfBounds
```

through the canonical enum representation from P11.

Success performs the same guarded element projection.

No panic endpoint exists on the fallible bounds-failure path.

---

# 51. Naked bounds propagation

Inside:

```text
Result[U, IndexError]
```

naked:

```sec
let value := try values[index]
```

propagates:

```text
Err(IndexError.OutOfBounds)
```

using existing Result construction/return flow.

No automatic wrapping into another error type.

---

# 52. Local bounds handlers

P14 extends the existing P10/P11 local handler engine to:

```text
IndexError
```

Example:

```sec
let value := try values[index] {
    Err(IndexError.OutOfBounds) => fallback
}
```

Specific enum handler uses canonical:

```text
enum constant
enum equality
```

Catch-all behavior follows the synchronized try-handler rules.

---

# 53. `try` resolved fact

Extend the existing resolved try-source classification with bounds indexing.

Recommended additional value:

```go
TryBoundsPropagation ResolvedTryKind = "bounds-propagation"
```

or equivalent normalized source-kind representation.

Resolved metadata records:

```text
error type = IndexError
operation = fixed-array-index
local handlers if any
propagation compatibility
```

Do not infer fallible index behavior from the presence of the token alone.

---

# 54. Effect analysis

Ordinary runtime-checked fixed-array access contributes:

```text
MayBoundsPanic
```

or the repository's canonical equivalent.

Proven-safe fixed-array access contributes no bounds panic effect.

Fallible `try` access contributes:

```text
IndexError flow
```

and no bounds panic effect for the index check itself.

Effects from:

```text
array expression
index expression
handler body
```

remain.

---

# 55. `@noPanic`

Conceptual valid cases:

```sec
type Index4 int range 0..3

@noPanic
fn ReadKnown(values: int32[4], index: Index4) int32 {
    return values[index]
}
```

when Sema proves the range.

And:

```sec
@noPanic
fn ReadFallible(
    values: int32[4],
    index: int,
) Result[int32, IndexError] {
    let value := try values[index]
    return Ok(value)
}
```

when operand evaluation is otherwise noPanic.

---

# 56. No `unsafe` bypass

Ordinary fixed-array indexing inside:

```sec
unsafe {
    ...
}
```

still requires the same safe bounds semantics.

Unchecked access belongs to later explicit RawPtr operations.

P14 emits no unchecked-array indexing mode.

---

# 57. Semantic array length representation

Recommended Semantic IR array type record:

```go
type FixedArrayTypeData struct {
    Element TypeID
    Length  *big.Int
}
```

Interning key:

```text
element TypeID + canonical decimal length
```

Length values are immutable deep copies.

Do not expose mutable shared `big.Int` pointers from type interning APIs.

---

# 58. Sec MLIR schema version 10

Compiler-generated high-level Sec MLIR uses:

```mlir
sec.dialect_version = 10 : i32
```

Schema versions 1 through 9 remain regression inputs.

Schema v10 adds:

```text
!sec.array

sec.array.construct
sec.array.default
sec.array.len
sec.array.index_in_bounds
sec.array.extract
sec.array.replace

sec.fail.bounds
```

and the fixed-array index verifier.

---

# 59. `!sec.array`

Canonical conceptual syntax:

```text
!sec.array<element-type, "N">
```

Examples:

```text
!sec.array<i32, "4">
!sec.array<!sec.struct<...>, "0">
!sec.array<i128, "18446744073709551615">
```

The length is a canonical non-negative base-10 arbitrary-precision StringAttr.

Do not encode it as a host integer.

---

# 60. Array type verifier

Verify:

```text
element type valid and sized-semantic candidate
length string canonical decimal
length >= 0
no leading plus sign
no unnecessary leading zeroes except "0"
```

Plan-specific `uint` and physical-layout checks happen in the appropriate
CompilationPlan validation pass.

---

# 61. Why MLIR length is a string

Using a canonical decimal StringAttr:

```text
preserves arbitrary precision
does not choose an arbitrary MLIR integer bit width
does not depend on host integer width
prints deterministically
```

It is a semantic parameter, not physical index type.

---

# 62. `sec.array.construct`

Variadic operands correspond one-to-one with source literal segments.

Required attributes:

```text
segment_kinds
segment_lengths
segment_actions
```

Allowed kinds:

```text
element
spread
```

P14 actions:

```text
element -> construct-direct
spread  -> copy-trivial
```

Result:

```text
!sec.array<T,N>
```

---

# 63. Construct verifier

For each segment:

```text
element:
    operand type == T
    segment length == 1

spread:
    operand type == !sec.array<T,M>
    segment length == M
```

Verify:

```text
exact arbitrary-precision sum(segment lengths) == result N
attribute counts == operand count
actions valid
```

No expanded element list is required.

---

# 64. `sec.array.default`

No operands.

Result:

```text
!sec.array<T,N>
```

Meaning:

```text
canonical semantic default construction for every live element
```

`N == 0` constructs no element.

The compiler-generation layer must have validated defaultability and P14
ownership/cleanup restrictions.

No `undef`.

---

# 65. `sec.array.len`

Operand:

```text
!sec.array<T,N>
```

Result:

```text
!sec.uint
```

before target scalar resolution, or the corresponding resolved unsigned
pointer-width representation after P6.

Semantic value:

```text
N
```

The operation is pure and compile-time foldable.

---

# 66. `sec.array.index_in_bounds`

Operands:

```text
array: !sec.array<T,N>
index: resolved integer semantic type
```

Required:

```text
index_signed: BoolAttr
```

Result:

```text
i1
```

Total operation.

Meaning:

```text
signed:
    index >= 0 && index < N

unsigned:
    index < N
```

---

# 67. `sec.array.extract`

Operands:

```text
array
index
```

Required attributes:

```text
bounds_kind
bounds_proof
action
```

Allowed P14:

```text
bounds_kind:
    proven-safe
    runtime-check

action:
    copy-trivial
```

Result:

```text
T
```

---

# 68. `sec.array.replace`

Operands:

```text
array
index
new_value
```

Required:

```text
bounds_kind
bounds_proof
```

Result:

```text
same !sec.array<T,N>
```

Compiler-generation layer enforces P14 trivial replacement safety.

---

# 69. `sec.fail.bounds`

No operands.

No successors.

Terminator.

Panic-capable high-level endpoint.

Required:

```text
operation = "fixed-array-index"
```

Optional source/provenance metadata may include:

```text
sec.source_name
sec.index_expression
```

The semantic panic reason is:

```text
panic.bounds
```

No runtime symbol or trap is selected.

---

# 70. Fixed-array index guard verifier

Register:

```bash
--sec-verify-array-index-guards
```

For `runtime-check` extraction/replacement, require:

```text
dominating sec.array.index_in_bounds
same array SSA
same index SSA
projection/update occurs only on true edge
failure edge does not reach extraction/update
```

For `proven-safe`:

```text
bounds_proof must be non-empty compiler proof provenance
```

The pass does not redo Sema range analysis.

---

# 71. Ordinary runtime index CFG

Canonical:

```mlir
%ok = "sec.array.index_in_bounds"(%array, %index)
    {index_signed = true}
    : (!sec.array<T,"N">, I) -> i1

cf.cond_br %ok, ^success, ^failure

^failure:
    "sec.fail.bounds"()
        {operation = "fixed-array-index"}
        : () -> ()

^success:
    %value = "sec.array.extract"(%array, %index)
        {
            bounds_kind = "runtime-check",
            bounds_proof = "guarded",
            action = "copy-trivial"
        }
        : (!sec.array<T,"N">, I) -> T
```

---

# 72. Fallible index CFG

Canonical failure path:

```text
IndexError.OutOfBounds
Result Err / local handler
```

Use the existing schema-v7+ ordinary enum representation.

Do not create:

```text
!sec.bounds_error
special index-error payload type
```

---

# 73. Proven-safe index

When Sema proves bounds:

```text
do not emit runtime branch
```

Emit:

```text
sec.array.extract / replace
bounds_kind = proven-safe
bounds_proof = constant/range/branch/contract/analysis
```

Optimization is not required to rediscover the proof.

---

# 74. Index type preservation

P14 does not normalize all index values to one hard-coded:

```text
i64
```

The legacy backend currently has such target-specific shortcuts.

New P14 semantics retain the resolved index type/signedness.

Later scalar lowering may convert representation according to the active target
and operation semantics.

---

# 75. Array storage

High-level storage may contain:

```text
!sec.array<T,N>
```

for the P14 trivial subset.

P5 does not lower it to MemRef.

No stack-byte layout is chosen in P14.

---

# 76. P6 compatibility

P6 scalar resolution may recurse through the array element type while preserving:

```text
array wrapper
exact length string
nested array shape
```

Example:

```text
!sec.array<!sec.int,"4">
    -> !sec.array<si32,"4">
```

on a 32-bit plan.

It also resolves the result of:

```text
sec.array.len
```

from `!sec.uint` to the plan-selected unsigned width.

---

# 77. P8 compatibility

P8 signless integer normalization must not recurse into:

```text
!sec.array
```

The element's source signedness/identity remains high-level until dedicated
aggregate representation lowering.

Index scalar values outside the wrapper may still be normalized according to
their already-resolved semantic operations.

---

# 78. P13 struct compatibility

Struct fields may use:

```text
!sec.array<T,N>
```

P13 struct identity/field order remains unchanged.

Struct default may contain `sec.array.default`.

Struct extract/replace remains semantic.

No physical nested aggregate layout is selected.

---

# 79. P11 union compatibility

A union payload may be a fixed array when its value path satisfies P14
ownership restrictions.

The union wrapper remains high-level.

No variant layout is selected.

---

# 80. P12 match compatibility

Match payload/result values may contain fixed arrays when:

```text
copy-trivial transfer is sufficient
```

P12 does not gain array patterns from P14.

This package adds no literal/range array pattern matching.

---

# 81. Whole-array function values

High-level:

```text
func.func
sec.call.direct
func.call
```

may carry `!sec.array<T,N>` parameter/result types where the existing
ownership-safe value subset permits.

No aggregate ABI classification is implied.

Foreign array ABI remains deferred.

---

# 82. Equality boundary

The source language supports fixed-array:

```text
==
!=
```

when element equality is defined.

P14 does not implement aggregate equality lowering.

Preserve Sema comparability metadata.

Array equality belongs to a later aggregate-operator package.

---

# 83. Membership boundary

The source language type-checks array membership:

```text
value in array
```

but exact-once short-circuit Semantic IR/lowering remains a separate operator
task.

P14 does not add membership lowering.

---

# 84. Slice boundary

P14 must not accidentally lower:

```text
ref T[]
ref mut T[]
T[] dynamic owning sequence
```

through `!sec.array`.

These are different semantic types.

Fixed `T[N]` only.

---

# 85. Physical fixed-array layout remains separate

P14 does not materialize:

```text
element stride
array byte size
array alignment
element address
LLVM array
MemRef
```

The canonical layout rule remains:

```text
elementStride = RoundUp(SizeOf(T), AlignOf(T))
arraySize = CheckedMultiply(N, elementStride)
```

but this belongs to the later plan-resolved physical aggregate lowering stage.

---

# 86. Zero-length array semantics

`T[0]`:

```text
has zero live elements
may be constructed as []
may be defaulted without constructing T
has .len == 0
has no valid index
every runtime index check evaluates false
has no element destruction
```

Its later physical alignment remains `AlignOf(T)` according to canonical layout.

P14 does not fake element zero.

---

# 87. Zero-sized element semantics

An array may have:

```text
N > 0
SizeOf(T) == 0
```

Element identity remains:

```text
array identity + index
```

Index bounds remain semantic even if physical addresses later coincide.

P14's semantic indexing naturally preserves this.

---

# 88. Array destruction boundary

The language destroys owned array elements in reverse index order.

P14 complete value/storage path is restricted to trivially destructible arrays.

No destruction loop is generated.

Non-trivial array destruction belongs to the ownership/destruction packages.

---

# 89. Array copy boundary

The language defines whole-array copy recursively from element copyability.

P14 permits ordinary aggregate SSA/value propagation only for:

```text
CopyTrivial
```

in the complete new path.

Semantic-copy or move-only arrays remain valid language types but require future
explicit transfer operations.

---

# 90. Required Sema length tests

```text
T[0]
T[4]
compile-time expression length
negative rejected
runtime-dependent rejected
non-integer rejected
exact uint boundary on 32-bit plan
one above uint boundary rejected on 32-bit plan
exact uint boundary on 64-bit plan
one above uint boundary rejected on 64-bit plan
length > int64 but <= uint64 accepted on 64-bit semantic path
multi-output 32/64 validates independently
type identity uses exact decimal length
dynamic T[] no longer identified by -1 sentinel
```

Do not allocate an array object of enormous test length.

Only type/plan validation is required.

---

# 91. Required literal-plan tests

```text
ordinary literal
target-typed literal
inferred literal
empty target literal
empty inferred literal rejected
one spread
multiple spreads
spread + ordinary entries
runtime-length spread rejected
element-type mismatch rejected
checked exact total length
large spread plan remains O(number of source entries)
query read-only
```

---

# 92. Required construction tests

```text
empty array construct
int32[4]
int128[2]
uint256[2]
nested int32[2][3]
struct[2]
enum[4]
segment length verification
spread segment verification
no per-expanded-element Semantic IR explosion
```

---

# 93. Required default tests

```text
int32[4] default
int128[3] default
nested trivial struct array default
nested fixed-array default
zero-length non-defaultable-element array default
nonzero non-defaultable-element array rejected
non-trivial cleanup-required default explicit unsupported in new IR
no undef/poison
IR size does not scale with N for ArrayDefaultOp
```

---

# 94. Required length tests

```text
array.len returns uint
array.len 0
array.len ordinary N
array.len wide exact N
P6 32-bit result type
P6 64-bit result type
compile-time foldability
```

---

# 95. Required constant index tests

Use arbitrary precision:

```text
0 in T[1]
N-1 valid
N invalid
negative invalid
int128 constant index
uint128 constant index
uint256 constant index
large fixed length > int64 on 64-bit semantic path
```

Compile-time invalid index is a source error.

No runtime failure op is emitted for a compile-time invalid program.

---

# 96. Required proven-safe index tests

```text
constant valid index
named range type proven in bounds
branch-refined index
assertion-refined index
zero-length has no proven-valid index
proven-safe emits no sec.fail.bounds
proven-safe carries proof provenance
```

---

# 97. Required runtime index tests

```text
signed dynamic index
unsigned dynamic index
negative signed runtime case
index == N
index > N
zero-length dynamic index
target/index evaluated once
same array/index used by guard and extraction
failure ends in sec.fail.bounds for ordinary access
success performs extraction
```

---

# 98. Required fallible index tests

```text
try array[index] returns/propagates IndexError.OutOfBounds
Result[int32, IndexError]
Result[int128, IndexError]
local Err(IndexError.OutOfBounds) fallback
catch-all handler
no sec.fail.bounds on fallible bounds-failure path
operand panic effects remain
@noPanic fallible index case
```

---

# 99. Required replacement tests

```text
mutable int32[4]
mutable int128[4]
proven-safe replace
runtime-checked replace
RHS evaluated once
RHS completely evaluated before replacement
root stored once
nested array replacement
non-trivial element replacement explicit unsupported
P5 leaves array storage high-level
```

---

# 100. Required dialect tests

```text
!sec.array round-trip
zero length
large decimal length
non-canonical decimal rejected
negative length rejected

sec.array.construct round-trip
element segment
spread segment
multiple segments
bad segment sum rejected
bad spread type rejected

sec.array.default round-trip
sec.array.len round-trip
sec.array.index_in_bounds round-trip
sec.array.extract round-trip
sec.array.replace round-trip
sec.fail.bounds round-trip/terminator verification

schema-v9 regression accepted
```

---

# 101. Required index-guard verifier tests

```text
canonical guarded extract accepted
canonical guarded replace accepted
extract without guard rejected for runtime-check
replace without guard rejected for runtime-check
false-path extract rejected
different array SSA rejected
different index SSA rejected
guard does not dominate rejected
proven-safe with proof provenance accepted
proven-safe without proof provenance rejected
```

---

# 102. Required integration tests

Source to schema-v10:

```text
fixed array literal
array literal spread
zero-length array
array default
array inside P13 struct
P13 struct inside array
nested array
array function parameter/result
array .len
constant index
dynamic ordinary index
dynamic try index
local try handler
mutable trivial element assignment
nested array assignment
```

No hand editing of generated IR.

---

# 103. Unsupported integration tests

The new P14 IR path must explicitly reject where required:

```text
move-only array literal spread
semantic-copy spread
move-only element read
ref element borrow
ref mut element borrow
non-trivial element replacement
non-trivial array destruction
dynamic owning T[] value lowering
slice lowering
array-to-slice creation
array equality lowering
array membership lowering
foreign fixed-array ABI lowering
```

Do not emit placeholder or unsound IR.

---

# 104. No physical backend shortcut

Do not define the new canonical array path using the legacy backend's:

```text
!llvm.array
llvm.mlir.undef
llvm.insertvalue
hard-coded i64 indexes
direct llvm.intr.trap
GEP-based semantic model
```

Those may remain legacy implementation details until migration.

P14 stays above physical representation.

---

# 105. Determinism

Use:

```text
canonical decimal strings
deterministic source segment order
deterministic nested type printing
arbitrary-precision arithmetic
```

No host-width-dependent printing or hash-order iteration.

---

# 106. Architecture rules

Non-negotiable:

```text
Fixed array length is exact arbitrary precision.

Fixed and dynamic arrays are not distinguished by a magic length sentinel in the
canonical semantic model.

Target uint representability is plan-specific.

Array literal spread is current supported Sec syntax.

Spread source evaluates exactly once.

Array spread remains compact in IR and does not expand O(N) semantic operands.

Array default remains compact and fully initialized.

T[0] constructs no elements and has no valid index.

Indexing is a place semantic before read/write/borrow selection.

Constant bounds checks use arbitrary precision.

Proven-safe access has no runtime check.

Unproven safe access has deterministic bounds checking.

Unsafe does not disable ordinary bounds checks.

Fallible fixed indexing uses canonical IndexError.OutOfBounds.

Ordinary bounds failure remains a high-level panic-capable endpoint.

Array/index expressions evaluate exactly once.

Trivial element extraction/replacement does not hide ownership semantics.

Move/borrow/non-trivial destruction remains deferred.

Fixed array wrapper remains high-level through P14.

No MemRef, LLVM array, GEP or physical element address is selected.

No mandatory runtime is introduced.

No LLVM dialect is generated.
```

---

# 107. Acceptance criteria

Package 14 is complete only when:

```text
[ ] implementation baseline documents repo 152c772 + local P13 or newer merged equivalent
[ ] previous package regressions remain green
[ ] wide builtin invariant remains
[ ] stale no-array-spread wording synchronized
[ ] IndexError.OutOfBounds locked as canonical fixed-index error
[ ] Semantic IR fixed-array amendment applied
[ ] schema-v10 dialect rulebook installed
[ ] lowering-v10 rulebook installed
[ ] canonical array shape no longer relies on -1 sentinel
[ ] canonical fixed length uses arbitrary precision
[ ] fixed length type identity uses exact value
[ ] target uint validation is plan-specific
[ ] >int64 semantic fixed length can survive on valid 64-bit plan
[ ] ResolvedArrayLiteralPlan implemented
[ ] literal plan does not expand spread O(N)
[ ] array literal source order preserved
[ ] multiple fixed-array spreads preserved
[ ] ArrayConstructOp implemented
[ ] ArrayDefaultOp implemented
[ ] zero-length default constructs no element
[ ] no readable partial/undef array
[ ] ArrayLengthOp implemented
[ ] ResolvedArrayIndexPlan implemented
[ ] constant bounds use arbitrary precision
[ ] proven-safe/runtime-check classification implemented
[ ] ArrayIndexInBoundsOp implemented
[ ] ArrayExtractOp implemented
[ ] ArrayReplaceOp implemented for trivial subset
[ ] BoundsFailureOp implemented
[ ] ordinary bounds path panic-capable
[ ] fallible bounds path produces IndexError.OutOfBounds
[ ] naked/local try integration works
[ ] bounds effect analysis integrated
[ ] @noPanic proven/fallible cases work
[ ] mutable trivial fixed-array storage works
[ ] nested trivial replacement works
[ ] P5 leaves array storage high-level
[ ] !sec.array implemented
[ ] sec.array.construct implemented
[ ] sec.array.default implemented
[ ] sec.array.len implemented
[ ] sec.array.index_in_bounds implemented
[ ] sec.array.extract implemented
[ ] sec.array.replace implemented
[ ] sec.fail.bounds implemented
[ ] --sec-verify-array-index-guards registered
[ ] P6 preserves wrapper and resolves nested target scalars
[ ] P8 does not recursively normalize array wrapper
[ ] P13 struct nesting integration passes
[ ] no dynamic array/slice semantics accidentally use !sec.array
[ ] non-trivial ownership paths reject explicitly
[ ] no physical array layout selected
[ ] no LLVM dialect generated
[ ] check-sec-mlir passes
[ ] go test ./... passes
[ ] legacy paths remain operational
```

---

# 108. Required implementation report

Codex must report:

```text
1. repository HEAD implemented against
2. whether local P13 or merged P13 was used
3. previous package status
4. array/spread rule synchronization
5. IndexError synchronization
6. files added
7. files modified
8. Sema fixed/dynamic array-shape refactor
9. arbitrary-precision length representation
10. target uint validation strategy
11. legacy int64 compatibility strategy
12. ResolvedArrayLiteralPlan API
13. compact spread implementation
14. ArrayConstructOp
15. ArrayDefaultOp
16. ArrayLengthOp
17. ResolvedArrayIndexPlan API
18. compile-time/proven/runtime bounds classification
19. ArrayIndexInBoundsOp
20. ArrayExtractOp
21. ArrayReplaceOp
22. ordinary bounds failure CFG
23. fallible IndexError CFG
24. local/naked try integration
25. effect-analysis changes
26. schema-v10 types/ops
27. index guard verifier
28. P5/P6/P8 compatibility
29. P13 struct nesting integration
30. wide array tests
31. large-length no-expansion tests
32. zero-length tests
33. bounds tests
34. unsupported ownership tests
35. CMake commands
36. exact LLVM/MLIR version
37. check-sec-mlir result
38. go test ./... result
39. end-to-end source -> schema-v10 results
40. deviations
41. recommendations for Package 15
```

---

# 109. Package 15 boundary

Recommended Package 15:

```text
Safe Place and Reference Semantic Core
```

Reason:

P14 intentionally stops before:

```text
ref array[index]
ref mut array[index]
array-to-slice borrowing
borrowed struct fields
borrowed union payloads
```

Those features all need one canonical place/reference layer.

Recommended P15 scope:

```text
Semantic IR PlaceID and place paths
local-storage places
struct-field places
fixed-array constant/dynamic element places
place mutability
shared ref
exclusive ref mut
reference origin identity
generation/epoch semantic metadata
borrow creation
reborrow
reference read/write boundary
reference equality boundary
reference verifier
high-level !sec.ref / !sec.ref_mut
no physical pointer/reference descriptor yet
```

P15 should then make Package 16:

```text
Slice Semantic Value Representation
```

much cleaner, because slices can be defined as borrowed contiguous views over
already-canonical places/references rather than inventing their own alias model.
