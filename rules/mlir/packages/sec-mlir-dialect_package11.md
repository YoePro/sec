# Sec MLIR Program - Implementation Package 11

## Package status

Implementation package for the Sec compiler.

Package ID: `SEC-MLIR-P11`\
Package title: `Enum and Union Semantic Value Representation`\
Repository: `https://github.com/YoePro/sec`\
Repository branch: `main`\
Repository sync commit used for this package: `152c772`\
Repository sync date: `2026-08-09`\
Semantic IR version before package: `1`\
Semantic IR version after package: `1`\
Sec MLIR dialect schema before package: `6`\
Sec MLIR dialect schema after package: `7`\
Sec MLIR lowering specification before package: `6`\
Sec MLIR lowering specification after package: `7`

Package 11 introduces canonical Semantic IR and high-level Sec MLIR value
representations for:

```text
enums
tagged unions
```

The package preserves semantic type identity, enum numeric representation,
union active-variant identity, payload shape, stable union variant indices, and
source provenance.

It deliberately does not choose the final physical tagged-union storage layout.

General `match` CFG lowering is Package 12.

---

# 1. Normative authority

Implementation follows:

```text
rules/declarations/enums.md
rules/declarations/unions.md
rules/memory/layout.md
rules/errors/errorhandling.md
rules/memory/destruction.md
rules/types/types.md
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

1. apply the Semantic IR amendment described by
   `sec_semantic_ir_enum_union_package11.md`;
2. update `rules/mlir/sec_mlir_dialect.md` with
   `sec_mlir_dialect_package11.md`;
3. update `rules/mlir/sec_mlir_lowering.md` with
   `sec_mlir_lowering_package11.md`.

No source-language grammar change is introduced.

---

# 2. Repository facts at baseline

The current frontend already has first-class semantic enum and union kinds.

Sema `Type` contains:

```text
Kind = EnumType / UnionType

EnumValues
EnumConsts
Underlying
BitWidth

UnionVariants
TypeArgs
```

Sema also already computes:

```text
TriviallyDestructible
CopyClassificationOf
EqualityComparable
```

for enums/unions and their payloads.

Package 11 must consume these resolved facts.

It must not re-parse declarations or infer copyability from MLIR types.

---

# 3. Wide builtin invariant

The following remain active Sec builtin types:

```text
int128
int256
uint128
uint256
decimal128
```

Enum underlying types and union payloads may contain active 128-bit and 256-bit
integer types.

No implementation or documentation added by Package 11 may describe those
types as future, planned, reserved, placeholder, or not-yet-active.

---

# 4. Enum semantic facts

A Sec enum:

```text
is a distinct named type
has one resolved underlying integer representation
may have aliases
may have explicitly assigned numeric values
may have values produced by explicit integer-to-enum conversion that have no
declared variant name
```

Enum values are not implicitly interchangeable with integers.

Two enum declarations remain different types even when their underlying
representation is identical.

---

# 5. Enum aliases

Multiple declared enum names may have the same numeric value.

Example:

```sec
enum ResultCode int {
    Success = 0
    Ok = 0
}
```

Package 11 preserves both:

```text
declaration identity
numeric value
```

The two declared case identities are different metadata entries.

The runtime enum value is still represented by the enum's numeric value.

Same-enum equality therefore compares the semantic numeric value.

Declared alias provenance must not be used as hidden runtime identity.

---

# 6. Enum values without a declared name

Explicit integer-to-enum conversion may produce an enum value with no declared
variant name.

Therefore the IR model must not require every enum SSA value to carry a declared
variant identifier.

Package 11 distinguishes:

```text
declared enum constant
converted enum value
```

A declared constant may retain case provenance.

A converted value may have only:

```text
enum type
numeric semantic value
source location
```

---

# 7. Enum underlying representation

Canonical enum native layout follows `rules/memory/layout.md`.

A fieldless enum uses exactly the layout of its resolved underlying integer type.

Default underlying type:

```text
int
```

and therefore target-sized according to the active CompilationPlan.

The new Semantic IR/Sec MLIR path must not reproduce the older legacy
hard-coded default enum `i32` behavior.

Bit-backed enums preserve:

```text
bit-backed representation kind
exact semantic width 1..256
unsigned behavior
```

---

# 8. Union semantic facts

A Sec union:

```text
is a distinct named type
has exactly one active variant
has a finite closed variant set
uses stable zero-based declaration-order variant indices
may carry no payload
may carry one unnamed payload
may carry a struct-like named payload
```

Union variant indices:

```text
are compiler metadata
are not source integers
must never be exposed through an integer cast
```

---

# 9. Union layout boundary

Sec 0.1 union native storage is conceptually:

```text
tag
largest-payload storage
```

but the baseline repository does not yet have one canonical fully resolved
union layout object for every CompilationPlan.

Package 11 therefore separates:

```text
semantic union value representation
physical union storage representation
```

P11 implements the first.

It preserves an optional `LayoutRef` when a resolved layout already exists.

It does not fabricate tag width, payload offset, total size, alignment or byte
buffer representation.

---

# 10. Package goal

After Package 11:

1. Semantic IR has canonical EnumType definitions;
2. Semantic IR has canonical UnionType definitions;
3. enum declaration order and aliases are preserved;
4. enum arbitrary-precision numeric constants are preserved;
5. enum underlying type identity is preserved;
6. bit-backed enum width is preserved;
7. enum declared constants can be constructed;
8. explicit integer-to-enum conversion can be represented;
9. explicit enum-to-integer conversion can be represented;
10. same-enum equality/inequality can be represented;
11. Semantic IR has stable union variant indices;
12. union variant payload shapes are canonical;
13. payload-less union construction works;
14. single-payload union construction works for the P11 ownership-safe subset;
15. struct-like union construction works for the P11 ownership-safe subset;
16. union variant testing works;
17. guarded single-payload projection works;
18. guarded struct-like field projection works;
19. generic concrete union instances are represented after substitution;
20. nested enum/union identities remain qualified;
21. schema-v7 Sec MLIR represents the same semantics;
22. Result remains a specialized high-level semantic view;
23. Option and ordinary concrete unions may use the generic union representation;
24. ArithmeticError migrates from the temporary core-error representation to the
    ordinary enum representation;
25. P10 enum error handlers can use ordinary enum constants/equality;
26. no general enum/union match CFG is added yet;
27. no physical union layout lowering is added;
28. no ownership semantics are hidden inside union construction/projection.

---

# 11. Explicit package boundary

## 11.1 In scope

Implement:

```text
Semantic IR enum definition
Semantic IR enum case definition
Semantic IR enum constant
Semantic IR enum from-integer
Semantic IR enum to-integer
Semantic IR enum eq/ne

Semantic IR union definition
Semantic IR union variant definition
payload-less variant
single-payload variant
struct-like payload variant
stable declaration-order index
generic concrete union identity
nested union identity
union construct
union is-variant
union unwrap single payload
union unwrap named field
union guard verifier

Sec MLIR !sec.enum
Sec MLIR EnumCaseAttr
Sec MLIR enum operations

Sec MLIR !sec.union
Sec MLIR UnionVariantAttr
Sec MLIR UnionFieldAttr
Sec MLIR union operations

schema v7 migration of ArithmeticError to ordinary enum
P10 enum-handler generalization
Package 6 enum-underlying scalar-resolution compatibility
Package 8 no-recursive-signless normalization compatibility

trivial-copy payload construction/projection
wide integer enum/union tests
generic concrete union tests
full regression
```

## 11.2 Explicitly out of scope

Do not implement:

```text
general match CFG lowering
enum match lowering
union match lowering
switch migration

physical union tag/payload lowering
canonical ResolvedLayout implementation
union tag width materialization
payload byte storage
niche optimization
LLVM union representation

union equality lowering
union ordering
enum ordering
enum arithmetic

checked dynamic integer-to-bit-enum conversion beyond Sema's existing accepted
range-proven cases

general numeric cast lowering

ownership-sensitive union payload construction
move-only payload extraction
partial move from union payload
union payload destruction
active-variant destruction dispatch
cleanup integration

general struct Semantic IR
general struct lowering
field borrowing

open/extensible unions
C unions
untagged unions
user-controlled union layout

LLVM dialect
production backend migration
```

---

# 12. Stable type identity

Every enum/union declaration receives a stable Semantic IR type identity.

Identity must distinguish:

```text
module declarations with the same short name
nested declarations
generic concrete union instantiations
```

Do not use display name alone as identity.

Recommended examples:

```text
main::Color
main::Vehicle.FuelType
main::Option<int32>
main::ResultCode
```

The exact printable identity string is not semantic authority.

Use TypeID/SymbolID internally.

---

# 13. Semantic IR enum definition

Recommended:

```go
type EnumRepresentationKind string

const (
    EnumRepresentationInteger   EnumRepresentationKind = "integer"
    EnumRepresentationBitBacked EnumRepresentationKind = "bit-backed"
)

type EnumCaseID uint32

type EnumCase struct {
    ID       EnumCaseID
    Name     string
    Value    *big.Int
    Location Location
}

type EnumDefinition struct {
    TypeID              TypeID
    SymbolID            SymbolID
    Name                string
    Underlying          TypeID
    RepresentationKind  EnumRepresentationKind
    BitWidth            uint16
    Cases               []EnumCase
    Location            Location
}
```

`EnumCaseID` is declaration identity.

It is not the enum numeric value.

---

# 14. Enum case ordering

Cases remain in declaration order.

Case ID:

```text
monotonic declaration-order ID local to the enum
```

Recommended first case ID:

```text
0
```

Duplicate numeric values are valid.

Duplicate case names are impossible after Sema.

---

# 15. Enum numeric values

Use arbitrary precision.

Never store enum constants only as:

```text
int64
uint64
```

because enum underlyings may include:

```text
int128
int256
uint128
uint256
bit[N] up to 256
```

The Semantic IR enum constant retains exact numeric value.

---

# 16. Semantic IR enum constant

Recommended:

```go
type EnumConstantOp struct {
    Result   Value
    EnumType TypeID
    Case     EnumCaseID
    Location Location
}
```

The numeric value is obtained from the canonical EnumDefinition.

Do not duplicate the numeric value as independently mutable operation metadata.

The result has the enum type.

---

# 17. Integer-to-enum semantic conversion

Add:

```text
EnumFromIntegerOp
```

Input:

```text
resolved integer value
```

Output:

```text
enum value
```

This is an explicit source conversion already validated by Sema.

It may produce a numeric enum value with no declared case name.

For bit-backed enums, P11 trusts Sema's accepted range proof.

Do not insert truncation.

---

# 18. Enum-to-integer semantic conversion

Add:

```text
EnumToIntegerOp
```

Input:

```text
enum
```

Output:

```text
the exact integer target type selected by Sema
```

This operation represents only the explicit enum/integer conversion.

It is not a generic numeric cast operation.

Physical extension/truncation is deferred.

---

# 19. Enum comparison

Add:

```text
EnumCompareOp
```

Allowed predicates:

```text
eq
ne
```

Inputs:

```text
same enum type
```

Output:

```text
bool
```

Comparison uses the semantic numeric enum value.

Alias declaration provenance does not make equal numeric values unequal.

No ordering predicate is allowed.

---

# 20. Enum provenance

A declared enum constant may retain:

```text
EnumCaseID
case source name
case source location
```

as provenance.

An arbitrary converted enum value does not need a case ID.

Optimization must not infer that a value with a numeric value equal to one case
was necessarily constructed from that named case.

---

# 21. Semantic IR union variant kinds

Define:

```go
type UnionVariantKind string

const (
    UnionVariantEmpty  UnionVariantKind = "empty"
    UnionVariantSingle UnionVariantKind = "single"
    UnionVariantFields UnionVariantKind = "fields"
)
```

---

# 22. Semantic IR union fields

Recommended:

```go
type UnionPayloadField struct {
    Name     string
    Type     TypeID
    Location Location
}
```

Fields remain in declared payload-field order.

---

# 23. Semantic IR union variant definition

Recommended:

```go
type UnionVariantIndex uint32

type UnionVariantDefinition struct {
    Index         UnionVariantIndex
    Name          string
    Kind          UnionVariantKind
    Payload       TypeID
    PayloadFields []UnionPayloadField
    Location      Location
}
```

Rules:

```text
Index is stable declaration order
empty variant has no Payload and no fields
single variant has one Payload and no fields
fields variant has no single Payload and one or more fields
```

---

# 24. Semantic IR union definition

Recommended:

```go
type UnionDefinition struct {
    TypeID               TypeID
    SymbolID             SymbolID
    Name                 string
    TypeArguments        []TypeID
    Variants             []UnionVariantDefinition
    CopyClassification   CopyClassification
    TriviallyDestructible bool
    LayoutRef            string
    Location             Location
}
```

`LayoutRef` may be empty until canonical layout resolution exists.

Do not synthesize a fake layout reference.

---

# 25. Concrete generic unions

Runtime Semantic IR values use concrete union instantiations.

Example source:

```sec
type Option[T] union {
    Some(T)
    None
}
```

Concrete type:

```text
Option[int32]
```

has a concrete P11 UnionDefinition:

```text
Some payload -> int32
None -> empty
```

No unresolved GenericType may reach a runtime union construction/projection op.

Generic template declarations may remain in higher module metadata.

---

# 26. Result relationship

The source language treats Result as the normal two-variant result abstraction.

P9/P10 already introduced specialized:

```text
!sec.result<T,E>
```

for high-level error-flow reasoning.

Package 11 does not remove that abstraction.

Instead:

```text
!sec.result<T,E>
```

is treated as a specialized semantic view whose eventual materialized
representation must obey the canonical Result union semantics.

Do not create a second incompatible physical Result representation.

Physical Result-to-union lowering remains later work.

---

# 27. Option relationship

`Option[T]` may use the generic P11 union representation after concrete
substitution:

```text
Some(T)
None
```

No special `!sec.option` type is required in Package 11.

If an existing high-level Option abstraction exists, keep it only until a
dedicated compatibility migration is implemented.

---

# 28. Union construction action

Union construction must not hide copy/move semantics.

Add per-payload action metadata.

Package 11 allows compiler-generated action:

```text
copy-trivial
```

only.

Recommended:

```go
type UnionPayloadAction string

const (
    UnionPayloadCopyTrivial UnionPayloadAction = "copy-trivial"
)
```

Future ownership packages may add explicit move/borrow/semantic-copy forms.

---

# 29. P11 ownership-safe payload subset

End-to-end union value construction/projection in Package 11 requires every
payload component used by the operation to satisfy:

```text
Sema CopyClassificationOf(type) == CopyTrivial
and
the type already has a canonical Semantic IR value representation
```

If not:

```text
UnsupportedFeatureError
```

Do not silently copy a move-only or ownership-sensitive payload.

The union type definition itself may still describe such a variant.

---

# 30. Payload-less union construction

Add:

```text
UnionConstructOp
```

with:

```text
variant kind = empty
zero payload operands
```

Result:

```text
union type
```

No allocation.

No physical tag write is implied yet.

---

# 31. Single-payload union construction

`UnionConstructOp`:

```text
variant kind = single
one payload operand
payload action = copy-trivial
```

The operand type exactly matches the concrete variant payload type.

Result:

```text
union type
```

---

# 32. Struct-like union construction

For a struct-like union variant:

```sec
Shape.Rectangle {
    width: First()
    height: Second()
}
```

source expressions are evaluated in source order.

After evaluation, the construction operation receives operands in canonical
payload-field declaration order.

This permits deterministic semantic evaluation without making source field order
the physical layout order.

The operation carries:

```text
field names
payload actions
```

and the verifier requires declaration-order canonicalization.

---

# 33. Union variant test

Add:

```text
UnionIsVariantOp
```

Input:

```text
union value
```

Attribute:

```text
stable UnionVariantIndex
```

Output:

```text
bool
```

This is a semantic active-variant test.

It does not read a physical tag yet.

---

# 34. Union payload projection

Add:

```text
UnionUnwrapPayloadOp
UnionUnwrapFieldOp
```

These are internal projections.

They are valid only on a control-flow path proven to have the matching active
variant.

They do not perform a runtime check themselves.

Invalid projection is malformed compiler IR.

---

# 35. Canonical union guard

```text
%is = union.is_variant %value, Variant
cond_br %is, matchingBlock, otherBlock

matchingBlock:
    %payload = union.unwrap_payload %value, Variant
```

or for a field:

```text
%field = union.unwrap_field %value, Variant, field
```

No unwrap before the guard.

---

# 36. Union guard verifier

Add:

```bash
--sec-verify-union-guards
```

It validates compiler-generated guarded projections.

Required checks:

```text
same union SSA value for test/projection
variant index exists in union type
true successor is the matching-variant path
projection variant equals tested variant
payload projection kind matches variant kind
field projection field exists and type matches
no opposite-variant projection in canonical matching block
union value dominates test and projection
```

---

# 37. No general match in P11

P11 provides the operations required by future match lowering:

```text
enum constants/comparison
union variant tests
union guarded projections
```

It does not generate the complete source `match` arm CFG.

Package 12 owns:

```text
arm ordering
catch-all
guards
exhaustiveness-to-CFG mapping
match expression merge
```

Sema already owns source exhaustiveness.

---

# 38. Sec MLIR schema version 7

Compiler-generated high-level Sec MLIR uses:

```mlir
sec.dialect_version = 7 : i32
```

Schema versions 1 through 6 remain regression inputs.

Schema 7 adds:

```text
EnumCaseAttr
UnionFieldAttr
UnionVariantAttr

!sec.enum
!sec.union

sec.enum.constant
sec.enum.from_integer
sec.enum.to_integer
sec.enum.cmp

sec.union.construct
sec.union.is_variant
sec.union.unwrap_payload
sec.union.unwrap_field
```

---

# 39. `EnumCaseAttr`

Canonical conceptual form:

```text
#sec.enum_case<
    ordinal,
    "name",
    "decimal-numeric-value"
>
```

Fields:

```text
ordinal: i32-compatible non-negative integer
name: non-empty string
value: arbitrary-precision base-10 integer string
```

The ordinal is declaration identity/provenance.

The numeric value is enum semantics.

Aliases may share numeric value.

---

# 40. `!sec.enum`

Canonical conceptual form:

```text
!sec.enum<
    "type-id",
    underlying-type,
    representation-kind,
    bit-width,
    [enum-case-attrs]
>
```

Representation kinds:

```text
integer
bit-backed
```

Rules:

```text
type-id non-empty
underlying is integer semantic type
integer representation uses bit-width 0
bit-backed uses width 1..256
case ordinals contiguous in declaration order
case names unique
case values representable by underlying/bit width
duplicate case numeric values allowed
```

---

# 41. Enum underlying type before/after scalar resolution

Before Package 6 target resolution:

```text
default enum may contain !sec.int underlying
```

After resolution:

```text
!sec.int -> si32 or si64 according to CompilationPlan
```

inside the enum type.

Package 6 must be extended to resolve the enum underlying semantic scalar while
retaining the enum wrapper.

Package 8 must not recursively normalize the enum underlying to signless.

Enum physical lowering is a later dedicated conversion.

---

# 42. `sec.enum.constant`

No operands.

Result:

```text
!sec.enum<...>
```

Required:

```text
case ordinal
```

Verifier:

```text
ordinal exists
result enum matches case table
```

No runtime lookup table.

---

# 43. `sec.enum.from_integer`

Operand:

```text
integer value
```

Result:

```text
!sec.enum<...>
```

This represents an explicit conversion already approved by Sema.

It does not require the runtime value to equal a declared case.

No hidden range truncation.

---

# 44. `sec.enum.to_integer`

Operand:

```text
!sec.enum<...>
```

Result:

```text
resolved integer target type
```

The target type is the explicit conversion target already resolved by Sema.

No implicit conversion.

---

# 45. `sec.enum.cmp`

Operands:

```text
same !sec.enum type
```

Predicate:

```text
eq
ne
```

Result:

```text
i1
```

No ordered predicates.

---

# 46. `UnionFieldAttr`

Canonical conceptual form:

```text
#sec.union_field<"name", type>
```

Field names are unique within one struct-like variant.

---

# 47. `UnionVariantAttr`

Canonical conceptual forms:

```text
#sec.union_variant<index, "Name", empty>

#sec.union_variant<index, "Name", single<type>>

#sec.union_variant<
    index,
    "Name",
    fields<[#sec.union_field<...>, ...]>
>
```

Index is stable declaration-order metadata.

It is not a source integer.

---

# 48. `!sec.union`

Canonical conceptual form:

```text
!sec.union<
    "type-id",
    [type-arguments],
    [union-variant-attrs]
>
```

Rules:

```text
type-id non-empty
type arguments concrete
at least one variant
variant indices contiguous from zero
variant names unique
all payload types valid
no unresolved generic type in runtime concrete union
```

---

# 49. `sec.union.construct`

Operands:

```text
zero or more payload values
```

Result:

```text
one !sec.union type
```

Required attributes:

```text
variant index
payload actions
```

For struct-like payload:

```text
field names in declaration order
```

Verifier consults the union type's self-contained variant metadata.

---

# 50. `sec.union.is_variant`

Operand:

```text
!sec.union
```

Required:

```text
variant index
```

Result:

```text
i1
```

Total.

---

# 51. `sec.union.unwrap_payload`

Operand:

```text
!sec.union
```

Required:

```text
single-payload variant index
payload action = copy-trivial in P11 compiler output
```

Result:

```text
declared single payload type
```

Path safety checked by union guard verifier.

---

# 52. `sec.union.unwrap_field`

Operand:

```text
!sec.union
```

Required:

```text
struct-like variant index
field name
payload action = copy-trivial in P11 compiler output
```

Result:

```text
declared field type
```

---

# 53. ArithmeticError migration

Package 9/10 temporarily modeled:

```text
!sec.core_error<"core::ArithmeticError">
```

Schema v7 new compiler output instead uses ordinary enum representation:

```text
!sec.enum<
    "core::ArithmeticError",
    resolved-int-underlying,
    integer,
    0,
    [
        Overflow = 0,
        DivisionByZero = 1,
        InvalidShift = 2
    ]
>
```

using canonical enum metadata syntax.

`!sec.core_error` remains parseable for schema-v6 regression fixtures only.

Do not emit it for enum-shaped core errors in schema v7.

---

# 54. `sec.arithmetic_error.from_reason` migration

Schema-v7 result becomes:

```text
ordinary ArithmeticError !sec.enum
```

The operation may remain as a high-level convenience operation until reason
lowering is implemented.

It no longer produces `!sec.core_error`.

---

# 55. P10 handler generalization

P10's temporary:

```text
sec.core_error.is_variant
```

is not emitted in new schema-v7 compiler output.

For any enum error E:

```text
construct the declared enum case constant
compare using sec.enum.cmp eq
```

Specific error handler:

```sec
Err(IOError.InvalidValue) => ...
```

can therefore lower when `IOError` has a canonical P11 enum representation.

Catch-all continues to bind the complete enum error value.

Sema remains responsible for handler exhaustiveness/order.

---

# 56. P10 error-enum scope after P11

P10 local handler lowering may now support:

```text
ArithmeticError
user-defined fieldless enum error types
compiler-known enum error types
```

provided:

```text
the error type is canonically represented as !sec.enum
handler patterns are the currently supported qualified enum constants/catch-all
payload ownership is irrelevant because enum values are trivial
```

No special whitelist of ArithmeticError variants remains in the handler engine.

---

# 57. Union errors and try handlers

P11 provides generic union variant test/projection primitives.

However `rules/errors/errorhandling.md` currently defines the initial specific local
error pattern in terms of a qualified error value and specifically describes
static exhaustiveness for enum error types.

Do not silently extend source try-handler semantics to payload-bearing union
patterns in P11.

That extension belongs with Package 12/general pattern lowering unless already
resolved by Sema in a later repository revision.

---

# 58. Result error type after P11

Example:

```text
!sec.result<int32, !sec.enum<"main::IOError", ...>>
```

is valid high-level schema-v7 IR.

Result guard operations remain unchanged.

P10 handler CFG can unwrap the enum error and compare it with enum constants.

---

# 59. Enum storage boundary

P11 does not lower `!sec.enum` to its physical integer representation.

This is deliberate even though enum layout is known.

Reason:

```text
enum nominal identity
safe integer conversion boundaries
error-handler verification
future match lowering
debug provenance
```

remain useful at the high-level stage.

A later enum representation pass may lower it to the underlying integer while
preserving required provenance.

---

# 60. Union storage boundary

P11 does not lower `!sec.union` to:

```text
integer tag
byte buffer
LLVM struct
memref
```

even if layout metadata is partially available.

High-level active-variant identity remains explicit.

---

# 61. LayoutRef behavior

Semantic IR enum/union types may retain:

```text
LayoutRef
```

when the canonical layout layer provides one.

Sec MLIR operations may carry existing:

```text
sec.layout_ref
```

metadata where useful.

A missing canonical union layout reference is not invented.

Package 11 does not complete the layout subsystem.

---

# 62. Union copy/destruction boundary

P11 type metadata preserves Sema's:

```text
CopyClassification
TriviallyDestructible
```

The compiler-generated value-operation subset requires trivial payload transfer.

Non-trivial union values remain valid Sec language values.

They are merely unsupported by the P11 new Semantic IR value path until
ownership/copy/move/destruction operations are implemented.

Do not downgrade source-language support.

---

# 63. Enum destruction

Fieldless enums are trivially destructible.

No destroy operation is emitted for enum values.

---

# 64. Union destruction

If every possible payload is trivially destructible:

```text
union destruction is trivial
```

for Package 11 purposes.

If any possible payload requires non-trivial destruction:

```text
P11 may describe the type
P11 must reject value paths that would require unrepresented cleanup
```

Do not emit an incomplete union destructor.

---

# 65. Nested enum/union identity

Examples:

```text
Vehicle.FuelType
Owner.NestedUnion
```

retain fully qualified semantic identity outside the owning impl.

The IR does not store the short lookup alias as type identity.

---

# 66. Source location policy

Retain:

```text
type declaration location
variant/case declaration location
payload-field location
construction location
conversion location
projection location
```

Normal MLIR `Location` remains operation source provenance.

Type metadata may retain declaration locations in Semantic IR even when MLIR
type syntax does not encode them.

---

# 67. Deterministic printing

Semantic IR printer order:

```text
enum definitions by TypeID/order
enum cases in declaration order
union definitions by TypeID/order
union variants by stable index
union fields in declaration order
```

Do not print Go map iteration order.

Arbitrary-precision enum values print canonical base-10.

---

# 68. Semantic IR verifier tests

Required:

```text
duplicate EnumCaseID rejected
duplicate enum case name rejected
duplicate numeric alias accepted
out-of-range enum numeric value rejected
bit-backed width 1 accepted
bit-backed width 256 accepted
bit-backed width 0/257 rejected
enum from integer type validated
enum compare requires same enum type

union requires at least one variant
union indices contiguous from zero
duplicate union variant name rejected
empty/single/fields shape invariants
generic runtime union has no unresolved generic payload
construct operand count/type validation
struct-like canonical field order
non-trivial payload action rejected in P11
union variant test valid/invalid index
unwrap payload wrong variant kind rejected
unwrap field unknown field rejected
```

---

# 69. MLIR dialect tests

Required:

```text
!sec.enum round-trip
enum aliases round-trip
enum int128/uint256 underlying round-trip
bit[1] enum round-trip
bit[256] enum round-trip
enum constant round-trip
enum from/to integer round-trip
enum cmp round-trip

!sec.union empty-only round-trip
single-payload union round-trip
mixed union round-trip
struct-like payload union round-trip
concrete generic Option[int32] round-trip
nested union identity round-trip
union construct round-trip
union is_variant round-trip
union unwrap payload/field round-trip
schema-v6 regression round-trip
```

---

# 70. Union guard verifier tests

Required:

```text
canonical guard accepted
projection without guard rejected
projection from false path rejected
projection variant differs from tested variant rejected
projection from different union SSA rejected
single projection on fields variant rejected
field projection on single variant rejected
unknown field rejected
matching block projection accepted
```

---

# 71. Enum integration tests

Required source cases:

```text
default-underlying enum on 32-bit target
default-underlying enum on 64-bit target
explicit int128 enum
explicit uint256 enum
bit[2] enum
bit[256] enum
enum aliases
enum equality/inequality
explicit integer-to-enum conversion
explicit enum-to-integer conversion
nested enum
enum function parameter/return
enum local mutable storage where current storage subset permits it
```

No hard-coded default enum i32 in the new pipeline.

---

# 72. Union integration tests

Required source cases:

```text
payload-less State union
single int32 payload
single int128 payload
single enum payload
mixed empty/single variants
struct-like payload with trivial fields
generic Option[int32]
generic Option[int128]
nested union
union parameter/return where current ABI-independent high-level path supports it
```

Do not require physical union layout.

---

# 73. Evaluation-order test for struct-like union construction

Example:

```sec
let value := Message.Move {
    y: Second(),
    x: First(),
}
```

Expected source evaluation:

```text
Second()
First()
```

if that is source order.

The final canonical construction operands may then be arranged in declaration
field order:

```text
x
y
```

using the already-evaluated SSA values.

Do not reorder source expression evaluation to simplify payload layout.

Use the language's established left-to-right/source-order evaluation rule.

---

# 74. P10 enum-handler integration tests

Required:

```text
Result[int32, IOError] local handler with specific enum cases
Result[int128, IOError] naked propagation
Result[uint256, IOError] catch-all handler
ArithmeticError handler using ordinary enum representation
no sec.core_error in schema-v7 compiler output
no sec.core_error.is_variant in schema-v7 compiler output
```

Only use an IOError-like test enum already representable in the test source.

---

# 75. Package 6 compatibility

Extend scalar-layout resolution so it may resolve the underlying semantic
integer type inside:

```text
!sec.enum
```

while preserving:

```text
enum type identity
case table
representation kind
bit width
```

Do not alter union variant payload semantics except normal recursive type mapping
where a concrete payload type itself is already independently supported and the
wrapper remains intact.

---

# 76. Package 8 compatibility

Package 8 signless normalization must not recurse into:

```text
!sec.enum
!sec.union
```

Enum signedness/underlying representation remains available until dedicated enum
representation lowering.

Union payload semantic types remain high-level.

---

# 77. P9/P10 compatibility

Schema-v7 compiler output uses:

```text
ArithmeticError as !sec.enum
Result error E as ordinary enum type where applicable
```

Existing schema-v6:

```text
!sec.core_error
sec.core_error.is_variant
```

fixtures remain parseable for regression.

Do not rewrite old test files in place merely to hide compatibility failures.

---

# 78. No source-level variant integer leakage

For unions, never expose:

```text
UnionVariantIndex
```

through:

```text
integer casts
debugger source value
reflection
serialization
user-visible enum-like API
```

It is implementation metadata.

Enum numeric values are different: they are explicit language-level underlying
values and may be converted through the language's enum/integer conversion
rules.

Keep these concepts separate.

---

# 79. Architecture rules

Non-negotiable:

```text
Enum identity is not its underlying integer type.

Enum aliases preserve declaration provenance but compare by semantic numeric
enum value.

Integer-to-enum conversion may produce a value with no declared case provenance.

Default enum underlying int is target-sized in the new pipeline.

Bit-backed enum width remains exact through 256 bits.

Union identity is not its tag.

Union variant index is not a source integer.

Union active variant remains semantic information.

Union payload projection is legal only on a proven matching path.

Union physical layout is not invented by P11.

Payload copy/move/destruction is not hidden inside union operations.

Non-trivial payloads wait for ownership semantics.

ArithmeticError uses the ordinary enum representation in schema v7.

Result remains a specialized high-level semantic abstraction with future
representation constrained by canonical union semantics.

General match lowering remains separate.

No mandatory runtime is introduced.

No LLVM dialect is generated.
```

---

# 80. Acceptance criteria

Package 11 is complete only when:

```text
[ ] repository baseline 152c772 or newer sync documented
[ ] previous package regressions remain green
[ ] wide-builtin invariant remains
[ ] Semantic IR enum/union amendment applied
[ ] schema-v7 dialect rulebook installed
[ ] lowering-v7 rulebook installed
[ ] EnumDefinition implemented
[ ] EnumCaseID distinct from numeric value
[ ] enum constants use arbitrary precision
[ ] duplicate numeric aliases supported
[ ] enum from-integer implemented
[ ] enum to-integer implemented
[ ] enum eq/ne implemented
[ ] bit-backed enum width preserved through 256
[ ] default enum int resolves by target plan
[ ] UnionDefinition implemented
[ ] stable zero-based union indices implemented
[ ] empty/single/fields variant kinds implemented
[ ] concrete generic union instances implemented
[ ] payload-less construction implemented
[ ] trivial single-payload construction implemented
[ ] trivial struct-like payload construction implemented
[ ] source field evaluation order preserved
[ ] union is-variant implemented
[ ] guarded payload unwrap implemented
[ ] guarded field unwrap implemented
[ ] union guard verifier registered
[ ] non-trivial payload transfer explicitly rejected in P11
[ ] !sec.enum implemented
[ ] EnumCaseAttr implemented
[ ] !sec.union implemented
[ ] UnionVariantAttr implemented
[ ] UnionFieldAttr implemented
[ ] enum ops implemented
[ ] union ops implemented
[ ] ArithmeticError emitted as ordinary enum
[ ] schema-v7 output contains no new !sec.core_error ArithmeticError
[ ] P10 enum handlers use enum constants/comparison
[ ] user fieldless enum error handlers work
[ ] Result[*, enum-error] works at high level
[ ] no general match CFG added
[ ] no physical union layout selected
[ ] no union equality lowering added
[ ] no LLVM dialect generated
[ ] check-sec-mlir passes
[ ] go test ./... passes
[ ] legacy compiler paths remain operational
```

---

# 81. Required implementation report

Codex must report:

```text
1. repository HEAD implemented against
2. previous package status
3. files added
4. files modified
5. Semantic IR enum representation
6. Semantic IR union representation
7. enum arbitrary-precision constant storage
8. enum alias handling
9. default/bit-backed underlying handling
10. union stable variant-index handling
11. generic concrete union canonicalization
12. union construction algorithms
13. struct-like source evaluation-order handling
14. ownership-safe payload gate
15. union guard verifier
16. schema-v7 types/attrs/ops
17. Package 6 enum-underlying compatibility
18. Package 8 wrapper-preservation compatibility
19. ArithmeticError migration
20. P10 enum-handler generalization
21. wide enum tests
22. wide union payload tests
23. CMake commands
24. exact LLVM/MLIR version
25. check-sec-mlir result
26. go test ./... result
27. end-to-end source -> schema-v7 results
28. unsupported non-trivial payload cases
29. deviations
30. recommendations for Package 12
```

---

# 82. Package 12 boundary

Recommended Package 12:

```text
Enum and Union Match CFG
```

Scope:

```text
resolved MatchPlan from Sema
subject evaluated once
arms in source order
first matching arm
enum constant patterns
enum alias-pattern behavior from Sema
union variant patterns
payload-less union arms
single-payload binding
struct-like whole-payload/initial supported binding
catch-all
guards
exhaustiveness
unreachable-arm invariants
match statement
match expression SSA merge
Result/Option match integration
P10 try-handler reuse where appropriate
```

Package 12 should still defer:

```text
physical union layout
ownership-sensitive payload moves
field-level union destructuring beyond source-supported forms
LLVM
```

After Package 12, P10 error handling can share one canonical pattern-dispatch
engine with general `match`.
