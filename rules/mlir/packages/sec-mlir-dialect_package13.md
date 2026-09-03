# Sec MLIR Program - Implementation Package 13

## Package status

Implementation package for the Sec compiler.

Package ID: `SEC-MLIR-P13`\
Package title: `Struct Semantic Value Representation`\
Repository: `https://github.com/YoePro/sec`\
Repository branch: `main`\
Repository sync commit used for this package: `152c772`\
Repository sync date: `2026-08-09`\
Semantic IR version before package: `1`\
Semantic IR version after package: `1`\
Sec MLIR dialect schema before package: `8`\
Sec MLIR dialect schema after package: `9`\
Sec MLIR lowering specification before package: `8`\
Sec MLIR lowering specification after package: `9`

Package 13 introduces canonical Semantic IR and high-level Sec MLIR
representation for ordinary stored-field structs.

It supports:

```text
named struct type identity
declaration-order stored fields
field tags as metadata
empty structs
nested structs
concrete generic structs
fully initialized struct construction
omitted-field semantic defaults
same-type struct spread
field reads
trivial field replacement
trivial mutable local struct storage
struct parameters and returns
canonical struct-like union payload values
```

The package deliberately does not choose final physical struct layout, field
offsets, aggregate ABI, non-trivial ownership transfer, properties or methods.

---

# 1. Normative authority

Implementation follows:

```text
rules/declarations/struct.md
rules/types/default_values.md
rules/declarations/spread.md
rules/memory/layout.md
rules/memory/copy_move.md
rules/memory/ownership.md
rules/memory/destruction.md
rules/declarations/properties.md
    ↓
rules/compiler/semantic_ir.md
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

1. apply `sec_semantic_ir_struct_package13.md` to
   `rules/compiler/semantic_ir.md`;
2. update `rules/mlir/sec_mlir_dialect.md` with
   `sec_mlir_dialect_package13.md`;
3. update `rules/mlir/sec_mlir_lowering.md` with
   `sec_mlir_lowering_package13.md`.

No source-language syntax change is introduced.

---

# 2. Repository facts at baseline

At repository sync `152c772`, the frontend already provides:

```text
Type.Kind == StructType
Type.Fields []StructField
StructField.Name
StructField.Type
StructField.Token
StructField.Tags

Type.Properties []Property

CopyClassificationOf
TriviallyDestructible

DefaultValueOf(Type) -> DefaultResolution

AST StructLiteral
AST StructLiteralField
AST spread marker

Sema struct literal field validation
Sema same-type struct spread validation
Sema omitted-field default materialization
Sema member resolution
Sema field place analysis
Sema move/borrow tracking
```

The legacy MLIR backend already supports selected struct literals, reads,
mutable local storage and nested field replacement.

Package 13 does not reuse the legacy LLVM-dialect representation as the new
canonical architecture.

---

# 3. Current AST-mutation debt

Current Sema materializes omitted struct fields by appending synthesized field
expressions directly to:

```text
ast.StructLiteral.Fields
```

Current mutable declarations may likewise receive synthesized default AST
expressions.

This behavior may remain temporarily for legacy backend compatibility.

However:

```text
AST mutation is not the canonical Semantic IR contract
```

Package 13 introduces read-only resolved construction/default plans.

The new Semantic IR builder must not depend on synthesized default AST nodes.

---

# 4. Wide builtin invariant

These are active Sec builtin types:

```text
int128
int256
uint128
uint256
decimal128
```

Struct fields, defaults, nested structs, function parameters/results and
struct-like union payloads may contain those active types wherever the existing
Semantic IR supports their values.

No future/planned wording is permitted.

---

# 5. Struct semantic identity

A Sec struct:

```text
is a distinct named type
contains stored fields in declaration order
may be empty
may be generic
may be nested inside an impl
may have field tags
may have properties and methods that are not stored fields
```

Struct type identity is not structural.

Two structs with identical field lists remain different types.

---

# 6. Stored-field rule

Only stored fields participate in the canonical struct value representation.

Do not include:

```text
properties
methods
nested type declarations
nested enum declarations
nested union declarations
nested units
constants
interfaces
events that are not stored data
field tags as stored bytes
documentation
visibility metadata
```

in the stored field sequence.

Field tags remain metadata attached to stored fields.

---

# 7. Field order

Stored fields retain declaration order.

Canonical field ordinal:

```text
zero-based declaration order
```

It is semantic field identity metadata.

It is not a physical byte offset.

Do not reorder fields to reduce padding.

---

# 8. Empty structs

An empty struct is a valid struct type.

Semantic value:

```text
zero stored fields
```

Native layout later resolves to the canonical zero-sized representation.

P13 does not fake one byte of storage.

---

# 9. Generic structs

Runtime Semantic IR uses concrete generic struct instantiations.

Example:

```sec
type Pair[A, B] struct {
    first: A,
    second: B,
}
```

Concrete runtime types:

```text
Pair[int32, byte]
Pair[int128, uint256]
```

have substituted concrete field types.

No unresolved generic field type reaches a runtime `StructConstructOp`,
projection or replacement.

---

# 10. Nested structs

Nested type identity remains qualified.

Examples:

```text
Vehicle.Engine
Container.Pair[int32,uint64]
```

Do not use only the short nested type name as canonical identity.

---

# 11. Package goal

After Package 13:

1. Semantic IR has canonical StructDefinition and StructFieldDefinition;
2. fields remain declaration ordered;
3. field tags remain semantic metadata;
4. properties are excluded from stored field representation;
5. concrete generic struct instances are representable;
6. empty structs are representable;
7. Sema exposes `ResolvedStructLiteralPlan`;
8. struct construction no longer relies on AST default-field mutation;
9. explicit fields evaluate in source-entry order;
10. spread source expressions evaluate exactly once;
11. struct spread is explicit in Semantic IR;
12. later entries override earlier spread-provided fields;
13. omitted fields use canonical `DefaultResolution`;
14. omitted defaults are distinguished from explicit initialization;
15. final `StructConstructOp` contains one value for every stored field;
16. final construct operands are canonical declaration order;
17. no readable struct contains uninitialized/undef/poison fields;
18. field reads use stable field identity;
19. P13 source field reads support only copy-trivial transfer;
20. functional field replacement exists for trivial structs;
21. mutable local trivial structs use high-level storage + replacement;
22. nested trivial field assignment rebuilds aggregate values leaf-to-root;
23. function parameters/results may use high-level structs;
24. P11 struct-like union payload can materialize a canonical synthetic struct
    value;
25. P12 whole struct-like union payload binding becomes available for the
    copy-trivial subset;
26. physical field offsets/layout remain deferred;
27. properties remain distinct semantic member operations;
28. non-trivial copy/move/destruction/borrow remains deferred.

---

# 12. Explicit package boundary

## 12.1 In scope

Implement:

```text
StructFieldID
StructDefinition
StructFieldDefinition
StructTag metadata
concrete generic struct identity
synthetic struct-like union payload type identity

ResolvedStructLiteralPlan
ResolvedStructMemberPlan
resolved field transfer action
resolved default origin
resolved spread origin

StructConstructOp
StructSpreadFieldsOp
StructExtractFieldOp
StructReplaceFieldOp

trivial high-level struct storage
nested trivial field replacement

Sec MLIR StructTagAttr
Sec MLIR StructFieldAttr
!sec.struct
sec.struct.construct
sec.struct.spread_fields
sec.struct.extract
sec.struct.replace_field

field/default/source provenance attributes
P6 recursive target-sized scalar resolution inside struct wrapper
P8 no-recursive signless normalization inside struct wrapper
P5 keep struct storage high-level
P11/P12 struct-like union payload materialization
full tests and regression
```

## 12.2 Explicitly out of scope

Do not implement:

```text
physical field offsets
ResolvedLayout implementation
ResolvedFieldLayout implementation
padding materialization
packing
explicit offsets
aggregate alignment contracts

physical LLVM struct
LLVM insertvalue/extractvalue lowering
aggregate ABI classification

general struct equality lowering
struct ordering

properties
property getters/setters
methods
impl dispatch

borrow field
ref field projection
ref mut field projection

move field
partial move
field reinitialization after move
semantic copy
conditional copy
non-copyable aggregate transfer

custom free
non-trivial field destruction
partial construction cleanup
cleanup regions

reference/slice-owning struct field value path where ownership is unresolved
resource-owning struct value path

general FFI struct layout
reflection
serialization

arrays/slices lowering
LLVM dialect
production backend migration
```

---

# 13. Semantic IR `StructFieldID`

Add:

```go
type StructFieldID uint32
```

Rules:

```text
zero is a valid first field ID within a struct
IDs are local to one StructDefinition
IDs follow declaration order
field identity is not a physical offset
```

A field is uniquely identified by:

```text
Struct TypeID + StructFieldID
```

---

# 14. `StructTag`

Recommended Semantic IR metadata:

```go
type StructTag struct {
    Key   string
    Value string
}
```

Tags preserve source order.

Tags are metadata.

They do not affect P13 type compatibility or physical layout.

---

# 15. `StructFieldDefinition`

Recommended:

```go
type StructFieldDefinition struct {
    ID       StructFieldID
    Name     string
    Type     TypeID
    Tags     []StructTag
    Location Location
}
```

Field names are unique after Sema.

---

# 16. `StructDefinition`

Recommended:

```go
type StructDefinition struct {
    TypeID                 TypeID
    SymbolID               SymbolID
    Name                   string
    TypeArguments          []TypeID
    Fields                 []StructFieldDefinition
    CopyClassification     CopyClassification
    TriviallyDestructible  bool
    Defaultable            bool
    LayoutRef              string
    SyntheticOrigin        StructSyntheticOrigin
    Location               Location
}
```

`LayoutRef` may be empty.

Do not invent a fake layout reference.

---

# 17. Synthetic struct origin

Recommended:

```go
type StructSyntheticOrigin string

const (
    StructSyntheticNone         StructSyntheticOrigin = ""
    StructSyntheticUnionPayload StructSyntheticOrigin = "union-payload"
)
```

For a struct-like union variant, synthetic identity is based on:

```text
owning union TypeID
stable UnionVariantIndex
```

It is not source-name lookup identity.

---

# 18. Synthetic union payload struct

For:

```sec
type Shape union {
    Circle {
        radius: float64
    }
    Rectangle {
        width: float64
        height: float64
    }
}
```

P13 creates canonical internal struct definitions conceptually equivalent to:

```text
Shape.Circle$payload
Shape.Rectangle$payload
```

with fields matching the variant payload fields in declaration order.

The printable `$payload` spelling is illustrative only.

The canonical identity is compiler-internal.

The type is not directly nameable from Sec source unless a future rule says so.

---

# 19. Sema `ResolvedStructLiteralPlan`

Add a read-only plan keyed by:

```text
*ast.StructLiteral
```

Recommended source entry kinds:

```go
type ResolvedStructEntryKind string

const (
    StructEntryExplicit ResolvedStructEntryKind = "explicit"
    StructEntrySpread   ResolvedStructEntryKind = "spread"
)
```

---

# 20. Struct field source kinds

Recommended:

```go
type ResolvedStructFieldSourceKind string

const (
    StructFieldSourceExplicit ResolvedStructFieldSourceKind = "explicit"
    StructFieldSourceSpread   ResolvedStructFieldSourceKind = "spread"
    StructFieldSourceDefault  ResolvedStructFieldSourceKind = "default"
)
```

---

# 21. Struct field transfer actions

Recommended:

```go
type ResolvedStructFieldAction string

const (
    StructFieldConstructDirect ResolvedStructFieldAction = "construct-direct"
    StructFieldCopyTrivial     ResolvedStructFieldAction = "copy-trivial"
    StructFieldMove            ResolvedStructFieldAction = "move"
    StructFieldCopySemantic    ResolvedStructFieldAction = "copy-semantic"
    StructFieldBorrowShared    ResolvedStructFieldAction = "borrow-shared"
    StructFieldBorrowMutable   ResolvedStructFieldAction = "borrow-mutable"
)
```

P13 complete value path accepts:

```text
construct-direct
copy-trivial
```

only.

The plan records other valid source actions so later packages do not need to
reconstruct them.

---

# 22. `ResolvedStructEntry`

Recommended:

```go
type ResolvedStructEntry struct {
    SourceIndex int
    Kind        ResolvedStructEntryKind
    FieldName   string
    FieldID     uint32
    Expression  ast.Expression
    Type        Type
}
```

For spread:

```text
FieldName empty
FieldID unused
Expression is the spread source expression
Type is exact target struct type
```

AST references are source-expression handles only.

The semantic decisions are already recorded in the plan.

---

# 23. `ResolvedStructFinalField`

Recommended:

```go
type ResolvedStructFinalField struct {
    FieldID          uint32
    FieldName        string
    FieldType        Type
    SourceKind       ResolvedStructFieldSourceKind
    SourceEntryIndex int
    SpreadFieldID    uint32
    Action           ResolvedStructFieldAction
    Default          DefaultResolution
}
```

Exactly one final field record exists per stored declaration field.

Records are in declaration order.

---

# 24. `ResolvedStructLiteralPlan`

Recommended:

```go
type ResolvedStructLiteralPlan struct {
    StructType       Type
    Entries          []ResolvedStructEntry
    FinalFields      []ResolvedStructFinalField
    FullyInitialized bool
}
```

The plan must be recorded during successful Sema.

---

# 25. Read-only plan query

Recommended:

```go
func (a *Analyzer) ResolvedStructLiteralPlanOf(
    expr *ast.StructLiteral,
) (ResolvedStructLiteralPlan, bool)
```

It must not:

```text
re-run type resolution
re-run default resolution
re-run spread resolution
mutate AST
mutate Analyzer
```

---

# 26. Legacy AST default materialization

To avoid breaking the legacy backend immediately:

```text
legacy AST default materialization may remain temporarily
```

but:

```text
new Semantic IR must ignore it as semantic authority
```

Recommended refactor:

```text
Sema records ResolvedStructLiteralPlan first
optional legacy compatibility helper materializes synthesized AST defaults later
```

Tests must prove the new plan is identical whether legacy AST materialization is
enabled or disabled.

---

# 27. Default resolution

For each omitted stored field:

```text
Sema calls canonical DefaultValueOf(field.Type)
```

The plan stores the resolved default.

P13 Semantic IR construction consumes `DefaultResolution`.

It does not:

```text
assume zero
read backend undef
invent a constructor
re-evaluate type contracts
```

---

# 28. Default provenance

Semantic IR must distinguish:

```text
explicit source initialization
spread-provided initialization
omitted-field default initialization
aggregate default construction
```

Recommended value/field provenance:

```go
type StructFieldOrigin string

const (
    StructOriginExplicit StructFieldOrigin = "explicit"
    StructOriginSpread   StructFieldOrigin = "spread"
    StructOriginDefault  StructFieldOrigin = "default"
)
```

This survives at least through high-level Sec MLIR verification.

---

# 29. Default materialization subset

P13 may materialize a resolved default when:

```text
the default is infallible
the default requires no hidden allocation
the resulting value type already has canonical Semantic IR representation
the resulting value introduces no unrepresented cleanup obligation
```

This includes the currently representable trivial scalar/default subset and
recursively trivial structs.

If a valid source default requires an unimplemented semantic value path:

```text
UnsupportedFeatureError
```

Do not substitute zero.

---

# 30. Construction source evaluation order

Struct literal source entries evaluate:

```text
left to right
```

across:

```text
explicit fields
spread entries
```

A spread source expression evaluates exactly once.

Defaults are constructed only after source entries and override resolution.

---

# 31. Spread semantics

For a spread entry:

```sec
source...
```

the source type is exactly the target struct type.

P13 Semantic IR emits one explicit:

```text
StructSpreadFieldsOp
```

for the already-evaluated source value.

The operation produces one field value per stored field in declaration order.

P13 compiler-generated spread actions are:

```text
copy-trivial
```

only.

---

# 32. Why spread stays explicit

The spread rulebook requires Semantic IR to preserve:

```text
source expression
source type
source evaluation point
field order
copy operations
override information
```

`StructSpreadFieldsOp` makes those semantics visible.

A later normalization may replace it with ordinary field extracts.

---

# 33. Spread override semantics

Entries are resolved left to right.

A later explicit field or later spread may override a value supplied by an
earlier spread according to source rules.

Duplicate explicit field declarations remain invalid.

P13 still evaluates the earlier spread source at its source position.

For the P13 copy-trivial subset, unused overridden spread results carry no
destruction/copy side effect.

Do not generalize this simplification to semantic-copy spreads.

---

# 34. Final construction order

After all source entries have been evaluated and defaults materialized:

```text
StructConstructOp operands are in stored field declaration order
```

Each field has exactly one value.

Source evaluation order and final operand order are separate concepts.

---

# 35. `StructConstructOp`

Recommended:

```go
type StructConstructOp struct {
    Result       Value
    StructType   TypeID
    Fields       []ValueID
    Origins      []StructFieldOrigin
    Actions      []ResolvedStructFieldAction
    Location     Location
}
```

Rules:

```text
one field operand per stored field
field operands in declaration order
operand types match field types
no missing field
no duplicate field
P13 actions construct-direct/copy-trivial only
```

The resulting struct is fully initialized.

---

# 36. `StructSpreadFieldsOp`

Recommended:

```go
type StructSpreadFieldsOp struct {
    Source    ValueID
    Results   []Value
    Actions   []ResolvedStructFieldAction
    Location  Location
}
```

Rules:

```text
source is exact struct type
one result per stored field
results in declaration order
result types equal field types
P13 actions copy-trivial only
```

---

# 37. `StructExtractFieldOp`

Recommended:

```go
type StructExtractFieldOp struct {
    Source    ValueID
    Field     StructFieldID
    Action    ResolvedStructFieldAction
    Result    Value
    Location  Location
}
```

P13 source field read:

```text
Action = copy-trivial
```

Result type is exact field type.

Properties do not use this operation.

---

# 38. Field read semantics

Source:

```sec
let x := value.field
```

when `field` is a stored field:

```text
Sema resolves member as field
Sema resolves read/copy action
Semantic IR emits StructExtractFieldOp
```

For P13:

```text
copy-trivial field -> supported
move-only field ordinary read -> source error or unsupported resolved path
borrow field -> deferred
semantic-copy field -> deferred
```

---

# 39. Resolved member plan

Parser member syntax is shared by:

```text
stored field
property
register field
enum value
other members
```

Add a read-only member-resolution fact.

Recommended:

```go
type ResolvedMemberKind string

const (
    MemberStoredField ResolvedMemberKind = "stored-field"
    MemberProperty    ResolvedMemberKind = "property"
    MemberOther       ResolvedMemberKind = "other"
)

type ResolvedStructMemberPlan struct {
    Kind       ResolvedMemberKind
    OwnerType  Type
    MemberType Type
    FieldID    uint32
    FieldName  string
    Action     ResolvedStructFieldAction
}
```

---

# 40. Member query

Recommended:

```go
func (a *Analyzer) ResolvedStructMemberOf(
    expr *ast.MemberExpression,
) (ResolvedStructMemberPlan, bool)
```

The Semantic IR builder must not infer field-versus-property from spelling.

P13 accepts:

```text
MemberStoredField
```

Property access remains explicit unsupported in the new IR path until property
operations are implemented.

---

# 41. `StructReplaceFieldOp`

Internal functional aggregate update:

```go
type StructReplaceFieldOp struct {
    Source     ValueID
    Field      StructFieldID
    NewValue   ValueID
    Result     Value
    Location   Location
}
```

Meaning:

```text
produce a new semantic struct value equal to Source except selected field is
NewValue
```

No physical copy algorithm is implied.

---

# 42. Replace-field safety boundary

P13 `StructReplaceFieldOp` is compiler-generated only when:

```text
whole struct CopyClassification == CopyTrivial
whole struct TriviallyDestructible == true
replaced field CopyClassification == CopyTrivial
replaced field TriviallyDestructible == true
new value is already fully evaluated
```

No old-field destruction occurs because P13 accepts only trivial destruction.

Non-trivial replacement belongs to ownership/destruction lowering.

---

# 43. Mutable local trivial struct storage

Extend high-level Semantic IR storage eligibility for:

```text
trivial struct values
```

Requirements:

```text
CopyTrivial
TriviallyDestructible
no unrepresented reference-origin obligation
all field value types supported by the new IR path
```

Use existing semantic storage operations:

```text
storage.declare
storage.init
storage.load
storage.store
```

with struct value type.

---

# 44. P5 storage lowering boundary

Package 5 must not lower:

```text
!sec.storage<!sec.struct<...>>
```

to MemRef merely because scalar storage is lowerable.

Struct storage remains high-level after P13.

A later physical struct-storage pass uses canonical resolved layout.

---

# 45. Field assignment to local struct

For:

```sec
value.field = expression
```

on a supported mutable local trivial struct:

```text
1. evaluate expression completely
2. load current whole struct from semantic storage
3. StructReplaceFieldOp
4. store replacement whole struct
```

This preserves safe source-before-destination update ordering.

---

# 46. Nested trivial field assignment

For:

```sec
root.outer.inner = expression
```

P13 may:

```text
evaluate expression
load root
extract outer
replace inner in outer
replace outer in root
store root once
```

Rebuild from leaf to root.

Every involved struct/field must satisfy P13 trivial safety requirements.

---

# 47. No field-place borrowing yet

P13 does not represent:

```sec
ref value.field
ref mut value.field
```

even when place analysis has validated it.

The member plan records the fact for future use.

The builder reports explicit unsupported until reference Semantic IR exists.

---

# 48. Struct parameters and returns

`func.func` may use high-level:

```text
!sec.struct<...>
```

parameters/results.

No ABI decision is implied.

Function call operands/results use the same high-level type identity.

Foreign ABI lowering remains deferred.

---

# 49. Struct equality boundary

Current Sema may prove ordinary struct equality-comparable when all stored
fields are comparable.

P13 records:

```text
EqualityComparable
```

in type metadata if useful.

It does not lower:

```text
==
!=
```

for structs.

A later aggregate-operator package may recursively compare fields according to
source rules.

---

# 50. Physical layout boundary

P13 does not encode:

```text
field byte offset
field alignment
padding before field
tail padding
aggregate size
aggregate alignment
packing
endianness
```

inside `!sec.struct`.

Those belong to canonical resolved layout metadata.

---

# 51. Layout reference

When a canonical resolved layout is available, Semantic IR may attach/reference:

```text
LayoutRef
```

and MLIR may carry:

```text
sec.layout_ref
```

according to the existing metadata convention.

If no canonical layout exists:

```text
leave it unresolved
```

Do not derive offsets independently in the P13 backend.

---

# 52. Sec MLIR schema version 9

Compiler-generated high-level Sec MLIR uses:

```mlir
sec.dialect_version = 9 : i32
```

Schema versions 1 through 8 remain regression inputs.

Schema v9 adds:

```text
StructTagAttr
StructFieldAttr

!sec.struct

sec.struct.construct
sec.struct.spread_fields
sec.struct.extract
sec.struct.replace_field
```

No physical struct op is introduced.

---

# 53. `StructTagAttr`

Canonical conceptual syntax:

```text
#sec.struct_tag<"key", "value">
```

Rules:

```text
key non-empty
value arbitrary string
source tag order preserved
```

Tags are metadata only.

---

# 54. `StructFieldAttr`

Canonical conceptual syntax:

```text
#sec.struct_field<
    ordinal,
    "name",
    type,
    [tags]
>
```

Rules:

```text
ordinal non-negative
name non-empty
field type valid
tag list valid
```

---

# 55. `!sec.struct`

Canonical conceptual syntax:

```text
!sec.struct<
    "type-id",
    [type-arguments],
    [
        #sec.struct_field<...>,
        ...
    ]
>
```

Rules:

```text
type-id non-empty
type arguments concrete for runtime values
field ordinals contiguous from zero
field names unique
fields in declaration order
empty field list valid
properties absent
```

---

# 56. Struct type identity

Two `!sec.struct` types with different canonical type identities are different
Sec types even when field metadata is identical.

No structural typing is introduced.

---

# 57. `sec.struct.construct`

Operands:

```text
one value per stored field
```

Result:

```text
one !sec.struct
```

Required arrays:

```text
field origins
field actions
```

Canonical field origins:

```text
explicit
spread
default
```

Canonical P13 actions:

```text
construct-direct
copy-trivial
```

Verifier requires declaration-order operand/type alignment.

---

# 58. `sec.struct.spread_fields`

Operand:

```text
one !sec.struct source
```

Results:

```text
one result per stored field in declaration order
```

Required actions:

```text
copy-trivial
```

for P13 compiler output.

Source expression evaluation has already happened before this operation.

---

# 59. `sec.struct.extract`

Operand:

```text
!sec.struct
```

Required:

```text
field ordinal
action
```

Result:

```text
field type
```

P13 compiler output action:

```text
copy-trivial
```

This is a semantic field read.

It is not physical extractvalue yet.

---

# 60. `sec.struct.replace_field`

Operands:

```text
source struct
new field value
```

Required:

```text
field ordinal
```

Result:

```text
same !sec.struct type
```

Verifier:

```text
new field type matches
source/result exact struct type match
```

P13 compiler generator additionally enforces trivial safety.

---

# 61. Operation effects

Pure SSA struct operations:

```text
sec.struct.construct
sec.struct.spread_fields
sec.struct.extract
sec.struct.replace_field
```

do not themselves imply memory effects.

They do imply semantic value transfer actions.

Do not mark ownership-sensitive future variants speculatable merely because the
P13 trivial forms are pure.

P13 actions are specifically safe to treat as pure value operations.

---

# 62. Default provenance in MLIR

Struct construction preserves field origin.

For omitted default fields:

```text
origin = default
```

Compiler-generated default values should also preserve existing source/default
provenance metadata when the producing operation supports discardable Sec
attributes.

Recommended:

```text
sec.default_kind
sec.default_synthesized = true
```

where useful.

No backend `undef`.

---

# 63. Struct spread in MLIR

Example conceptual shape:

```mlir
%source = ...
%a, %b, %c = "sec.struct.spread_fields"(%source)
    {actions = ["copy-trivial", "copy-trivial", "copy-trivial"]}
    : (!sec.struct<...>) -> (T0, T1, T2)

%explicit = ...

%result = "sec.struct.construct"(%a, %explicit, %c)
    {
        field_origins = ["spread", "explicit", "spread"],
        field_actions = ["copy-trivial", "construct-direct", "copy-trivial"]
    }
    : (T0, T1, T2) -> !sec.struct<...>
```

Source evaluation order remains encoded in SSA definition order.

---

# 64. P6 compatibility

Extend target scalar resolution recursively through the high-level struct field
type list while preserving the struct wrapper.

Examples:

```text
field !sec.int  -> si32/si64
field !sec.uint -> ui32/ui64
nested high-level structs recurse
```

Do not change:

```text
struct type identity
field ordinal
field name
tags
```

---

# 65. P8 compatibility

Checked-integer signless normalization must not recurse into:

```text
!sec.struct
```

Dedicated aggregate representation lowering owns that transition.

High-level field signedness/type identity remains available.

---

# 66. P11 union payload compatibility

For a P11 struct-like union variant:

```text
fields<[name:type, ...]>
```

P13 creates the canonical synthetic payload struct definition.

The union type remains a union.

The synthetic struct is a semantic view of that variant's whole payload.

It is not a second physical payload layout.

---

# 67. P12 whole-payload match binding

P12 may now support:

```sec
Circle(circle) => Use(circle.radius)
```

for a struct-like union variant when:

```text
all payload fields are representable
whole payload transfer is copy-trivial
no borrow/move semantics are required
```

Canonical lowering on the proven matching path:

```text
unwrap each union payload field using P11 guarded field projection
construct synthetic P13 payload struct
bind the synthetic struct value
```

Do not add a second union payload representation.

---

# 68. P12 struct-like payload guard safety

Every `sec.union.unwrap_field` used to create the synthetic payload struct
remains dominated by the matching `sec.union.is_variant` true path.

The existing union guard verifier remains authoritative.

After field projections:

```text
sec.struct.construct
```

assembles the whole semantic payload value.

---

# 69. Properties remain separate

A property uses ordinary member syntax in source, but Sema distinguishes it from
a stored field.

P13 must not emit:

```text
sec.struct.extract
sec.struct.replace_field
```

for properties.

Property reads/writes belong to explicit property Semantic IR operations in a
later package.

---

# 70. Field tags

P13 preserves tags through:

```text
Sema Type
Semantic IR StructDefinition
Sec MLIR StructFieldAttr
```

Tags do not affect:

```text
type compatibility
field read semantics
field replacement semantics
P13 layout
```

Do not hard-code tag keys.

---

# 71. Source construction order tests

Required source cases:

```sec
Pair {
    second: Second(),
    first: First(),
}
```

Expected evaluation:

```text
Second()
First()
```

Final `StructConstructOp` operands:

```text
first
second
```

in declaration order.

Do not reorder calls to simplify aggregate construction.

---

# 72. Spread evaluation tests

For:

```sec
User {
    FirstSource()...
    Name: ExplicitName()
    SecondSource()...
}
```

required expression evaluation:

```text
FirstSource()
ExplicitName()
SecondSource()
```

Each spread source evaluates once.

Each spread op expands fields in declaration order.

Final fields follow left-to-right override rules.

---

# 73. Default ordering tests

For:

```sec
Config {
    explicit: BuildExplicit()
}
```

required:

```text
BuildExplicit()
then omitted semantic defaults
then final struct construct
```

Omitted defaults are not source expressions and do not move ahead of explicit
source evaluation.

Within recursive struct default construction:

```text
fields construct in declaration order
```

---

# 74. Required Semantic IR type tests

```text
empty struct
one-field struct
nested struct
generic Pair[int32,uint64]
generic Pair[int128,uint256]
nested impl struct identity
field tags
duplicate field ID rejected
duplicate field name rejected
non-contiguous field ID rejected
property not present in stored fields
optional LayoutRef
```

---

# 75. Required ResolvedStructLiteralPlan tests

```text
explicit-only literal
empty default literal
partial default literal
nested default
named constrained default
explicit type default
non-defaultable omission rejected
one spread
explicit overrides spread
multiple spreads
later spread overrides earlier spread
duplicate explicit field rejected
source entries remain source ordered
final fields are declaration ordered
read-only query does not mutate Analyzer
new IR does not depend on AST synthesized fields
```

---

# 76. Required struct construction tests

```text
fully explicit fields
source order differs from declaration order
omitted default field
all-default empty literal
nested trivial struct
int128 field
uint256 field
decimal128 field when current value representation supports it
empty struct
field-origin metadata
construct-direct action
copy-trivial action
unsupported move action rejected
unsupported semantic-copy action rejected
```

---

# 77. Required spread tests

```text
same exact struct type
source evaluated once
copy-trivial field results in declaration order
explicit override
multiple spread override
wrong struct source remains Sema error
move-only/semantic-copy spread explicit unsupported in new IR
no hidden allocation
```

---

# 78. Required field read tests

```text
scalar field
int128 field
uint256 field
nested trivial struct field
field through function parameter
field from function return value
property with same member syntax does not lower as field
copy-trivial field succeeds
move-only field ordinary read not faked as copy
```

---

# 79. Required mutable local tests

```text
mutable trivial struct default initialization
mutable trivial struct explicit initialization
top-level field assignment
nested field assignment
new RHS evaluated before load/replace/store commit
root stored once after nested rebuild
P5 does not memref-lower struct storage
```

---

# 80. Required struct-like union integration

```text
P11 struct-like variant gets synthetic StructDefinition
synthetic fields match union payload declaration order
synthetic identity stable by union TypeID + variant index
P12 whole-payload binding works for trivial payload fields
whole-payload binding field read works
int128 payload field
uint256 payload field
non-trivial payload whole binding rejected
union guard still dominates every field projection
```

---

# 81. Required dialect tests

```text
StructTagAttr round-trip
StructFieldAttr round-trip
empty !sec.struct round-trip
nested !sec.struct round-trip
generic !sec.struct round-trip
wide field types round-trip
field tags round-trip
construct round-trip
spread_fields variadic results round-trip
extract round-trip
replace_field round-trip
bad field ordinal rejected
bad operand type rejected
bad origin/action count rejected
schema-v8 regression accepted
```

---

# 82. Required P6/P8 tests

P6:

```text
!sec.int field resolves on 32-bit plan
!sec.int field resolves on 64-bit plan
!sec.uint field resolves
nested struct scalar resolution
wrapper/field identity preserved
```

P8:

```text
does not recursively convert struct siN/uiN field types to signless
does not lower struct operations
```

---

# 83. Unsupported end-to-end tests

The new Semantic IR path must explicitly reject:

```text
resource-owning struct construction
move-only source field transfer
semantic-copy field transfer
borrowed field construction requiring new borrow semantics
ordinary read of move-only field when source semantics require move
partial move
field borrow/ref/ref mut
non-trivial field replacement
custom-free struct value path
struct equality
property read/write
method call requiring new struct receiver semantics
foreign struct ABI
```

Do not emit placeholder IR.

---

# 84. No physical struct lowering

Package 13 does not lower:

```text
!sec.struct
```

to:

```text
LLVM struct
builtin tuple
memref byte buffer
array of bytes
individual field allocas
```

The representation stays semantic.

---

# 85. No `undef` construction

No P13 generated readable struct may be created by leaving omitted fields as:

```text
undef
poison
uninitialized bytes
```

All stored fields have semantic values before `StructConstructOp` completes.

---

# 86. No mandatory runtime

Struct construction, spread, extraction and trivial replacement require no
runtime service.

No heap allocation is introduced by P13 itself.

---

# 87. Compiler commands

`sec emit-ir`:

```text
prints StructDefinition
prints explicit spread/default/construction semantics
prints struct storage/replacement where present
```

`sec emit-sec-mlir`:

```text
emits schema-v9 high-level struct operations
```

Legacy commands retain their previous stage meanings.

---

# 88. Verification pipeline

Compiler-generated schema-v9 high-level MLIR runs:

```text
normal MLIR verification
existing applicable package verifiers
union guard verifier when struct-like union payload is materialized
match verifier when payload struct is used by match
```

No physical layout verifier is invented in P13.

---

# 89. Architecture rules

Non-negotiable:

```text
Struct identity is nominal, not structural.

Only stored fields belong to struct value representation.

Stored fields remain declaration ordered.

Field ID is not field offset.

Tags are metadata, not stored bytes.

Every readable struct is fully initialized.

Omitted fields use canonical semantic defaults.

New Semantic IR does not depend on AST default mutation.

Struct source entries evaluate left to right.

Spread source evaluates exactly once.

Spread remains explicit before normalization.

Final struct operands are declaration ordered.

Source evaluation order must not be changed to match field order.

Field versus property is resolved by Sema.

P13 field reads/replacements are copy-trivial/trivial-destruction only.

Move/borrow/semantic-copy/destruction is never hidden as SSA behavior.

Mutable trivial struct assignment rebuilds semantic aggregates, not physical
bytes.

Struct-like union whole payload uses canonical synthetic struct value identity.

Physical layout, offsets and ABI remain separate.

No mandatory runtime is introduced.

No LLVM dialect is generated.
```

---

# 90. Acceptance criteria

Package 13 is complete only when:

```text
[ ] repository baseline 152c772 or newer sync documented
[ ] previous package regressions remain green
[ ] wide builtin invariant remains
[ ] Semantic IR struct amendment applied
[ ] schema-v9 dialect rulebook installed
[ ] lowering-v9 rulebook installed
[ ] StructFieldID implemented
[ ] StructDefinition implemented
[ ] field tags preserved
[ ] stored fields exclude properties/non-storage members
[ ] empty structs supported
[ ] concrete generic structs supported
[ ] nested struct identity supported
[ ] ResolvedStructLiteralPlan implemented
[ ] plan query read-only
[ ] Semantic IR does not rely on AST default mutation
[ ] explicit/default/spread origins preserved
[ ] canonical DefaultResolution consumed
[ ] explicit source order preserved
[ ] spread source evaluated once
[ ] StructSpreadFieldsOp implemented
[ ] override rules preserved
[ ] StructConstructOp fully initializes every field
[ ] construction operands declaration ordered
[ ] StructExtractFieldOp implemented
[ ] field/property distinction consumed from Sema
[ ] StructReplaceFieldOp implemented for trivial subset
[ ] mutable local trivial struct storage supported
[ ] nested trivial replacement rebuild works
[ ] P5 leaves struct storage high-level
[ ] !sec.struct implemented
[ ] StructTagAttr implemented
[ ] StructFieldAttr implemented
[ ] sec.struct.construct implemented
[ ] sec.struct.spread_fields implemented
[ ] sec.struct.extract implemented
[ ] sec.struct.replace_field implemented
[ ] P6 resolves target-sized struct fields preserving wrapper
[ ] P8 does not recursively normalize struct fields
[ ] synthetic struct-like union payload type implemented
[ ] P12 trivial whole-payload struct binding enabled
[ ] non-trivial ownership paths reject explicitly
[ ] no physical field offsets/layout selected
[ ] no backend undef/poison default
[ ] no LLVM dialect generated
[ ] check-sec-mlir passes
[ ] go test ./... passes
[ ] legacy paths remain operational
```

---

# 91. Required implementation report

Codex must report:

```text
1. repository HEAD implemented against
2. previous package status
3. files added
4. files modified
5. StructDefinition/StructFieldID implementation
6. field-tag preservation
7. generic/nested type identity
8. ResolvedStructLiteralPlan API
9. AST default-mutation compatibility strategy
10. default-resolution lowering
11. explicit/spread/default origin model
12. source evaluation-order algorithm
13. StructSpreadFieldsOp
14. final construction algorithm
15. field member-resolution API
16. StructExtractFieldOp
17. StructReplaceFieldOp
18. mutable local struct storage extension
19. nested replacement algorithm
20. schema-v9 attrs/types/ops
21. P5 storage compatibility
22. P6 scalar-resolution compatibility
23. P8 wrapper-preservation compatibility
24. synthetic union payload struct representation
25. P12 whole-payload match integration
26. wide struct tests
27. default tests
28. spread tests
29. ownership-boundary unsupported tests
30. CMake commands
31. exact LLVM/MLIR version
32. check-sec-mlir result
33. go test ./... result
34. end-to-end source -> schema-v9 results
35. deviations
36. recommendations for Package 14
```

---

# 92. Package 14 boundary

Recommended Package 14:

```text
Fixed Array Semantic Value Representation
```

Reason:

After scalar, enum/union, match and struct values, fixed arrays are the next
major aggregate whose semantics are already well specified and whose layout is
compile-time finite.

Recommended scope:

```text
Semantic IR fixed-array type
compile-time length including zero
array literal construction
array default construction
array spread normalization
constant/dynamic indexing semantics
checked versus proven-safe index classification
array field/struct nesting
trivial-copy element subset
Sec MLIR !sec.array
array construct/extract/update operations
no physical memref/LLVM layout yet
```

Package 14 should still defer:

```text
owning dynamic arrays
slices/references
move-out from fixed arrays
non-trivial element destruction
aggregate ABI
LLVM
```

That package then prepares the path to slices/references and the ownership-heavy
aggregate work.
