# Sec MLIR Dialect

## Status

Normative detailed representation specification.

Current Sec MLIR dialect schema version: `11`

Schema version 7 adds canonical high-level enum and tagged-union semantic value
representation.

It also migrates enum-shaped core errors, including ArithmeticError, to the
ordinary enum representation.

Schema version 8 additionally defines verified explicit match CFG and the
synthesized `sec.unreachable` terminator. Physical union layout remains deferred.

Schema version 9 adds canonical high-level struct semantic values. Physical
struct layout remains deferred.

Schema version 10 adds canonical high-level fixed-array semantic values,
compact construction/defaults, bounds predicates, guarded element operations
and the ordinary bounds-failure endpoint (SEC-MLIR-P14). Physical array
representation remains deferred.

Schema version 11 introduces compiler-internal semantic places and direct safe
reference values (SEC-MLIR-P15). The schema does not select one physical
pointer/reference representation.

---

# 1. Version history

```text
v1  dialect foundation
v2  Semantic IR bridge
v3  scalar/target coverage
v4  checked integer operations
v5  typed arithmetic failure and high-level Result construction
v6  Result branching and local try handlers
v7  enum and tagged-union semantic value representation
v8  verified match CFG support and synthesized unreachable
v9  canonical high-level struct semantic values
v10 fixed-array semantic values
v11 places and direct safe references
```

Compiler-generated modules carry the newest schema whose package is
implemented; schema-v11 modules carry:

```mlir
sec.dialect_version = 11 : i32
```

Until SEC-MLIR-P15 completes schema-11 emission the compiler emits schema 10.
Schema versions 1 through 10 remain valid regression inputs, and every schema
keeps its compatibility promises.

The schema-v10 and schema-v11 sections below were installed from
`rules/mlir/dialect-versions/sec_mlir_dialect_v10.md` and
`sec_mlir_dialect_v11.md` on 2026-10-11; those files remain the version-history
snapshots.

---

# 2. Enum case attribute

Canonical conceptual syntax:

```text
#sec.enum_case<ordinal, "name", "value">
```

Fields:

```text
ordinal
name
arbitrary-precision base-10 numeric value
```

Rules:

```text
ordinal >= 0
name non-empty
value parses exactly as arbitrary-precision integer
```

Duplicate numeric values are allowed.

---

# 3. Enum representation kind

Canonical values:

```text
integer
bit-backed
```

---

# 4. `!sec.enum`

Canonical conceptual syntax:

```text
!sec.enum<
    "type-id",
    underlying-type,
    representation-kind,
    bit-width,
    [cases]
>
```

Example:

```text
!sec.enum<
    "main::Color",
    si32,
    integer,
    0,
    [
        #sec.enum_case<0, "Red", "0">,
        #sec.enum_case<1, "Green", "1">
    ]
>
```

Rules:

```text
type-id non-empty
underlying type integer-compatible
integer representation bit-width = 0
bit-backed width in 1..256
cases in contiguous ordinal order
case names unique
case numeric values representable
duplicate numeric values legal
```

---

# 5. Enum identity

Two `!sec.enum` values with different type IDs are different Sec types even when
all representation data is otherwise identical.

No implicit conversion exists.

---

# 6. Enum aliases

Cases may share numeric values.

Case ordinal/name remain declaration provenance.

Runtime equality is based on the semantic enum numeric value, not case ordinal.

---

# 7. `sec.enum.constant`

No operands.

Result:

```text
!sec.enum
```

Required attribute:

```text
case ordinal
```

Verifier confirms the ordinal exists in the result enum type.

---

# 8. `sec.enum.from_integer`

Operand:

```text
integer semantic value
```

Result:

```text
!sec.enum
```

It represents an explicit conversion already validated by Sema.

The value need not correspond to a declared case.

No truncation is implied.

---

# 9. `sec.enum.to_integer`

Operand:

```text
!sec.enum
```

Result:

```text
resolved integer conversion target
```

It is an explicit enum conversion.

It is not an implicit underlying-type extraction.

---

# 10. `sec.enum.cmp`

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

No ordering predicate.

---

# 11. Union field attribute

Canonical conceptual syntax:

```text
#sec.union_field<"name", type>
```

Field name is non-empty.

---

# 12. Union variant attribute

Canonical conceptual forms:

```text
#sec.union_variant<index, "Name", empty>

#sec.union_variant<index, "Name", single<type>>

#sec.union_variant<
    index,
    "Name",
    fields<[
        #sec.union_field<"x", type>,
        #sec.union_field<"y", type>
    ]>
>
```

Rules:

```text
index non-negative
name non-empty
empty has no payload
single has exactly one payload type
fields has one or more uniquely named fields
```

---

# 13. `!sec.union`

Canonical conceptual syntax:

```text
!sec.union<
    "type-id",
    [type-arguments],
    [variants]
>
```

Example:

```text
!sec.union<
    "main::Option",
    [i32],
    [
        #sec.union_variant<0, "Some", single<i32>>,
        #sec.union_variant<1, "None", empty>
    ]
>
```

Rules:

```text
type-id non-empty
runtime type arguments concrete
at least one variant
indices contiguous from zero
variant names unique
payload types valid
```

---

# 14. Union identity

Type identity includes:

```text
declaration identity
concrete type arguments
```

It is not the physical tag.

---

# 15. `sec.union.construct`

Operands:

```text
zero or more payload values
```

Result:

```text
one !sec.union
```

Required:

```text
variant index
payload actions
```

Struct-like variant additionally carries:

```text
field names
```

P11 compiler-generated payload action:

```text
copy-trivial
```

only.

---

# 16. Construction verifier

For the selected variant:

```text
empty:
    zero operands

single:
    one operand
    operand type equals payload type

fields:
    operand count equals field count
    field names exactly declaration order
    operand types equal field types
```

Payload action count matches operand count.

---

# 17. `sec.union.is_variant`

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

Total semantic variant test.

---

# 18. `sec.union.unwrap_payload`

Operand:

```text
!sec.union
```

Required:

```text
single-payload variant index
payload action
```

Result:

```text
declared payload type
```

Control-flow validity is checked by the union guard verifier.

---

# 19. `sec.union.unwrap_field`

Operand:

```text
!sec.union
```

Required:

```text
struct-like variant index
field name
payload action
```

Result:

```text
declared field type
```

---

# 20. Union guard verifier

Register:

```bash
--sec-verify-union-guards
```

It verifies canonical guarded union projections.

Required:

```text
same union SSA for test and projection
projection is on true/matching path
variant identity matches
projection kind matches variant shape
field exists
result type matches
dominance is valid
```

---

# 21. Enum target-sized underlying

Package 6 scalar resolution may recurse one level into `!sec.enum` solely to
resolve:

```text
!sec.int
!sec.uint
!sec.char
!sec.rune
```

where such types are valid underlying representations according to source rules.

The enum wrapper and case metadata remain.

For ordinary default enum:

```text
!sec.int -> si32 or si64
```

by active CompilationPlan.

---

# 22. P8 signless boundary

Package 8 must not normalize the underlying signedness inside `!sec.enum`.

It must not recursively normalize `!sec.union`.

Dedicated representation lowering owns that transition later.

---

# 23. ArithmeticError

Schema-v7 compiler output represents ArithmeticError as:

```text
ordinary !sec.enum
```

with cases:

```text
Overflow
DivisionByZero
InvalidShift
```

The default underlying source type follows the core enum declaration.

If no explicit underlying is declared, it is `int`.

---

# 24. `!sec.core_error` compatibility

Schema-v6 `!sec.core_error<...>` remains parseable for regression fixtures.

Schema-v7 compiler-generated output must not use it for an enum-shaped core
error.

It is not the canonical enum representation.

---

# 25. Arithmetic error conversion

`sec.arithmetic_error.from_reason` in schema v7 returns the canonical
ArithmeticError `!sec.enum`.

No physical enum integer lowering is implied.

---

# 26. P10 enum handlers

Schema-v7 compiler-generated local enum-error dispatch uses:

```text
sec.enum.constant
sec.enum.cmp eq
```

rather than:

```text
sec.core_error.is_variant
```

Catch-all binds the complete enum value.

Every schema-v7 branch that reaches a local enum-error handler merge carries:

```text
sec.try_handler_exhaustive = true
```

This boolean records Sema's exhaustiveness proof. The MLIR verifier consumes
that proof instead of inferring a closed error domain from hard-coded enum case
names. Schema-v6 compatibility input retains its legacy ArithmeticError check.

---

# 27. Result with enum error

Valid:

```text
!sec.result<T, !sec.enum<...>>
```

Result guard/unwrapping operations from schema v6 remain unchanged.

---

# 28. Result remains specialized high-level form

`!sec.result<T,E>` is retained because high-level error-flow operations benefit
from the semantic abstraction.

Its future physical representation must obey the canonical Result tagged-union
semantics.

P11 does not lower Result to `!sec.union`.

---

# 29. Option

Concrete Option values may use `!sec.union` with:

```text
Some(T)
None
```

No dedicated new Option type is required.

---

# 30. Layout boundary

`!sec.union` does not encode:

```text
tag physical type
tag offset
payload offset
payload size
payload alignment
total size
total alignment
```

Those properties belong to resolved layout metadata and representation lowering.

---

# 31. No tag exposure

`UnionVariantAttr.index` is not a source numeric value.

No schema-v7 operation converts it to an integer.

---

# 32. Effects

Enum and union semantic construction/test/projection operations introduced here
have no memory effect by themselves.

This does not mean payload ownership effects are absent.

P11 compiler output permits only trivial-copy payload actions.

Future ownership-aware forms may require stronger effects/traits.

---

# 33. No speculatable invalid projection

`sec.union.unwrap_payload` and `sec.union.unwrap_field` must not be marked
speculatable across variant guards.

They rely on a proven active-variant path.

---

# 34. Schema-v7 completion

Schema v7 is complete when:

```text
enum attrs/type/ops parse, print and verify
aliases remain valid
wide/bit-backed enums verify
union attrs/type/ops parse, print and verify
generic concrete unions verify
guarded union projection verifier works
ArithmeticError uses ordinary enum output
P10 enum handlers use ordinary enum operations
schema-v6 regression fixtures remain valid
no physical union layout is selected
```

## 34.1 Generic concreteness

Compiler-generated executable Sec MLIR follows the generic-concreteness
boundary in `rules/compiler/generics_lowering.md`. Generic-dependent nominal
types preserve declaration identity and complete ordered concrete arguments;
generic enum instances use ordinary `!sec.enum` semantics with distinct
concrete nominal identities. The executable dialect does not require runtime
generic parameters, dictionaries, type descriptors, or generic call
operations. Generic provenance may remain as non-runtime metadata.

---

# 35. Schema-v8 match CFG

Schema v8 intentionally adds no monolithic match operation. Match lowering uses
`cf.cond_br`, `cf.br`, block arguments and ordinary function returns.

`sec.unreachable` is a no-operand, no-result, no-successor terminator requiring
`sec.synthesized = true` and a non-empty `reason`. Exhaustive match residuals
use reason `exhaustive-match-fallthrough`.

Compiler-generated CFG may carry positive `sec.match_id`, non-negative
`sec.match_arm_index`, `sec.match_stage` (`pattern`, `guard`, `body-exit`,
`merge`, `residual`) and `sec.match_pattern_kind` provenance. Provenance aids
verification but does not define semantics. `--sec-verify-match-cfg` validates
source order, false/true edges, guarded projections, merge types and exhaustive
residual termination.

---

# 36. Schema-v9 struct attributes

`#sec.struct_tag<key, value>` preserves open, source-order field metadata and
does not affect type compatibility. `#sec.struct_field<ordinal, name, type,
tags>` identifies one stored field. Ordinals are non-negative and contiguous
from zero, names are non-empty and unique, types are valid non-void types, and
every tag is a `StructTagAttr`. Properties, methods and other non-storage
members must not appear as fields.

# 37. `!sec.struct`

The canonical form is:

```text
!sec.struct<identity, [concrete-type-arguments], [stored-fields]>
```

Identity is non-empty and nominal. Empty field lists are valid. Runtime type
arguments are concrete. Stored fields remain in declaration order; names,
ordinals and tags are semantic metadata, not physical byte offsets. Nested
structs and wide active builtins (`int128`, `int256`, `uint128`, `uint256`,
`decimal128`) are valid field types. Compiler-internal identities derived from
union identity plus variant index may represent struct-like union payloads and
are not separately source-nameable.

# 38. `sec.struct.construct`

The operation consumes exactly one value per stored field and produces one
`!sec.struct`. Required `field_origins` entries are `explicit`, `spread` or
`default`; required schema-v9 `field_actions` entries are `construct-direct` or
`copy-trivial`. Counts must equal the field count and operand types must match
the declaration-order field types exactly. It always represents a fully
initialized semantic value; `undef`, poison and omitted operands are invalid.

# 39. `sec.struct.spread_fields`

The operation consumes one already evaluated `!sec.struct` and produces one
result per stored field in declaration order. Its `actions` count equals the
result count, every schema-v9 compiler action is `copy-trivial`, and result
types exactly match field types. The operation never re-evaluates its source.

# 40. Field projection and replacement

`sec.struct.extract` consumes a struct plus a valid field ordinal and
`copy-trivial` action, and returns the exact selected field type.
`sec.struct.replace_field` consumes a source struct and replacement value and
returns the exact same struct type. The replacement type must match the
selected field. Replacement is functional semantic aggregation; it does not
imply an offset, store, destruction, `insertvalue` or `memcpy`.

# 41. Effects and ownership boundary

Schema-v9 construct, spread, extract and replacement forms are pure because
their executable transfer subset is limited to `construct-direct` and
`copy-trivial`. This purity must not be generalized to later move,
semantic-copy or ownership-aware forms. Properties never lower through these
field operations merely because their source syntax is member access.

# 42. Storage and pass compatibility

High-level `!sec.storage<!sec.struct<...>>` is legal for the P13 trivial subset.
`--sec-lower-trivial-core` must leave it high-level. Target-sized scalar
resolution recurses through struct type arguments and fields while preserving
the struct wrapper, identity, field ordinals, names and tags, including nested
structs. Checked-integer signless normalization must not recurse through the
wrapper.

# 43. Layout and schema-v9 completion

An optional `sec.layout_ref` may name separately resolved layout information,
but schema 9 embeds no field offsets, size, alignment, padding, packing, LLVM
struct body or ABI classification. Completion requires tag/field/type
parse-print-verify, empty/nested/concrete-generic/wide types, all four struct
operation verifiers, scalar-wrapper preservation, high-level struct storage,
synthetic union payload structs, schema-8 regression compatibility and no
physical layout selection.

---

# Schema version 10 — fixed-array semantic values

## v10.2 `!sec.array`

Canonical conceptual syntax:

```text
!sec.array<element-type, "length">
```

Examples:

```text
!sec.array<i32, "4">
!sec.array<i128, "2">
!sec.array<!sec.struct<...>, "0">
```

---

## v10.3 Length syntax

The length parameter is a StringAttr containing canonical base-10
arbitrary-precision unsigned text.

Valid:

```text
"0"
"1"
"4294967295"
"18446744073709551615"
```

Invalid:

```text
"-1"
"+1"
"00"
"01"
" 4 "
```

---

## v10.4 Why the length is textual

The semantic length is not assigned an arbitrary MLIR integer bit width.

Canonical decimal text:

```text
preserves arbitrary precision
prints deterministically
avoids host-width dependence
remains independent of physical index type
```

---

## v10.5 Array type identity

Type identity includes:

```text
element type
exact length
```

`!sec.array<i32,"4">` differs from:

```text
!sec.array<i32,"5">
!sec.array<ui32,"4">
```

---

## v10.6 No dynamic-array use

`!sec.array` in schema v10 means only:

```text
fixed T[N]
```

It never means:

```text
dynamic T[]
slice
```

Those receive separate future representations.

---

## v10.7 `sec.array.construct`

Variadic operands represent compact literal segments.

Required arrays:

```text
segment_kinds
segment_lengths
segment_actions
```

Each array length equals operand count.

Allowed segment kinds:

```text
element
spread
```

---

## v10.8 Element segment

For an element segment:

```text
operand type == result element type
segment length == "1"
segment action == "construct-direct"
```

---

## v10.9 Spread segment

For a spread segment:

```text
operand type == !sec.array<T,"M">
segment length == "M"
segment action == "copy-trivial"
```

P14 compiler output does not emit move/semantic-copy spread.

---

## v10.10 Construct result

Result:

```text
!sec.array<T,"N">
```

Verifier performs arbitrary-precision checked sum:

```text
sum(segment_lengths) == N
```

No expansion into N operands.

---

## v10.11 `sec.array.default`

No operands.

Result:

```text
!sec.array<T,"N">
```

Meaning:

```text
fully initialized canonical array default
```

For `N == 0` no element default is constructed.

Builder/Semantic IR verification owns defaultability and ownership checks.

---

## v10.12 `sec.array.len`

Operand:

```text
!sec.array<T,"N">
```

Result:

```text
!sec.uint
```

before scalar target resolution, or the plan-resolved unsigned scalar after
P6.

The semantic value is exactly N.

Pure and foldable.

---

## v10.13 `sec.array.index_in_bounds`

Operands:

```text
array: !sec.array<T,"N">
index: integer semantic value
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

No physical address computation.

---

## v10.14 `sec.array.extract`

Operands:

```text
array
index
```

Result:

```text
T
```

Required:

```text
bounds_kind
bounds_proof
action
```

Allowed P14 values:

```text
bounds_kind:
    proven-safe
    runtime-check

action:
    copy-trivial
```

---

## v10.15 `sec.array.replace`

Operands:

```text
array
index
new_value
```

Result:

```text
same !sec.array<T,"N">
```

Required:

```text
bounds_kind
bounds_proof
```

The new value type must exactly equal T.

P14 generator separately requires trivial safe replacement semantics.

---

## v10.16 Proven bounds metadata

Canonical proof strings:

```text
constant
range
branch
contract
analysis
```

For runtime-guarded operations:

```text
bounds_proof = "guarded"
```

These strings are compiler provenance.

They are not source promises.

---

## v10.17 `sec.fail.bounds`

No operands.

No results.

No successors.

Terminator.

Required:

```text
operation = "fixed-array-index"
```

Semantic panic reason:

```text
panic.bounds
```

It does not select the physical panic endpoint.

---

## v10.18 Index guard verifier

Register:

```text
--sec-verify-array-index-guards
```

Runtime-check extract/replace requires:

```text
dominating sec.array.index_in_bounds
same array SSA
same index SSA
true-edge dominance
```

Proven-safe operations require valid proof provenance.

---

## v10.19 Ordinary runtime check shape

Canonical:

```text
index_in_bounds
cf.cond_br

false:
    sec.fail.bounds

true:
    sec.array.extract or sec.array.replace
```

No direct backend trap.

---

## v10.20 Fallible check shape

Fallible indexing:

```text
index_in_bounds
cf.cond_br

false:
    canonical IndexError.OutOfBounds enum value
    Result/handler flow

true:
    sec.array.extract / replacement path
```

No `sec.fail.bounds` on the handled failure path.

---

## v10.21 Array storage

High-level:

```text
!sec.storage<!sec.array<T,"N">>
```

may exist for the P14 trivial subset.

Schema v10 does not define a physical MemRef/LLVM aggregate representation.

---

## v10.22 P5 boundary

`sec-lower-trivial-core` does not lower struct or fixed-array aggregate storage
to MemRef.

Scalar storage behavior remains unchanged.

---

## v10.23 P6 target resolution

P6 may resolve target-sized scalar types recursively inside the array element
type while preserving:

```text
array wrapper
exact length
nested shape
```

It also resolves `sec.array.len` result representation.

---

## v10.24 P8 boundary

P8 does not recursively erase integer signedness inside `!sec.array`.

Dedicated aggregate representation lowering owns that transition later.

---

## v10.25 P13 integration

`!sec.struct` fields may contain `!sec.array`.

`!sec.array` elements may contain `!sec.struct`.

Both wrappers remain high-level.

---

## v10.26 Zero length

Schema v10 permits:

```text
!sec.array<T,"0">
```

It has no element value.

`sec.array.default` is valid for zero length without requiring one T default.

`sec.array.extract` still requires a proof/guard; no valid safe proof can exist
for any actual index into length zero.

---

## v10.27 No physical layout

Schema v10 does not define:

```text
element stride
byte size
alignment
element pointer
LLVM array
MemRef
GEP
```

---

## v10.28 Schema-v10 completion

Schema v10 is complete when:

```text
array type parses/prints/verifies
large exact lengths round-trip
construct segments verify without expansion
default/len operations verify
index predicate verifies
guarded extract/replace verify
ordinary bounds endpoint verifies
schema-v9 regressions remain valid
no dynamic/slice type is represented as !sec.array
no physical array layout is selected
```

---

# Schema version 11 — places and direct safe references

## v11.2 `!sec.place`

Conceptual syntax:

```text
!sec.place<T, "ro">
!sec.place<T, "rw">
```

`!sec.place` is compiler-internal.

It is not a source-visible type.

---

## v11.3 Place authority

Allowed:

```text
ro
rw
```

A place derivation may preserve or reduce authority.

It may not increase authority.

---

## v11.4 `!sec.ref<T>`

High-level source safe shared reference.

Guarantees:

```text
non-null
typed
non-owning
shared read authority
```

The type does not encode physical generation bits.

---

## v11.5 `!sec.ref_mut<T>`

High-level source safe mutable reference.

Guarantees:

```text
non-null
typed
non-owning referent
exclusive mutable authority
```

The reference value is move-only by semantic classification.

---

## v11.6 Place root metadata

Root place operations carry canonical semantic metadata such as:

```text
sec.place_root_id
sec.storage_identity
sec.address_space
sec.source_name
```

`sec.source_name` is provenance only.

---

## v11.7 Reference metadata

Reference-producing operations carry:

```text
sec.borrow_id
sec.lifetime_id
sec.validity_policy
sec.validity_proof
sec.epoch_dependency
sec.relocation_class
sec.reference_origin
```

where applicable.

These attributes are semantic.

They do not force physical fields.

---

## v11.8 `sec.place.value`

Operand:

```text
T
```

Result:

```text
!sec.place<T,"ro">
```

Required root/storage identity metadata.

Logical place materialization only.

---

## v11.9 `sec.place.storage`

Operand:

```text
!sec.storage<T>
```

Result:

```text
!sec.place<T,"ro">
or
!sec.place<T,"rw">
```

Authority must match the storage mutability contract.

---

## v11.10 `sec.place.field`

Operand:

```text
!sec.place<!sec.struct<...>, A>
```

Required:

```text
field ordinal
```

Result:

```text
!sec.place<FieldType, A-or-narrower>
```

The field must be a stored field.

---

## v11.11 `sec.place.array_index_in_bounds`

Operands:

```text
array place
integer index
```

Result:

```text
i1
```

The array place referent is fixed:

```text
!sec.array<T,N>
```

Required:

```text
index_signed
```

No physical address computation.

---

## v11.12 `sec.place.array_element`

Operands:

```text
array place
index
```

Result:

```text
!sec.place<T, same authority>
```

Required:

```text
bounds_kind
bounds_proof
```

Runtime-check form must be dominated by the matching place bounds predicate.

---

## v11.13 `sec.place.union_payload`

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
payload place
```

Only valid on a matching active-variant path.

---

## v11.14 `sec.place.deref`

Operand:

```text
!sec.ref<T>
```

returns:

```text
!sec.place<T,"ro">
```

Operand:

```text
!sec.ref_mut<T>
```

returns:

```text
!sec.place<T,"rw">
```

Dynamic-validity reference requires a dominating valid branch.

---

## v11.15 `sec.place.read`

Operand:

```text
!sec.place<T,A>
```

Result:

```text
T
```

Required action:

```text
copy-trivial
```

for schema-v11 compiler output.

---

## v11.16 `sec.place.write`

Operands:

```text
!sec.place<T,"rw">
T
```

No result.

The compiler-generation layer enforces trivial replacement safety.

---

## v11.17 `sec.ref.borrow_shared`

Operand:

```text
!sec.place<T,"ro|rw">
```

Result:

```text
!sec.ref<T>
```

Required reference semantic metadata.

---

## v11.18 `sec.ref.borrow_mut`

Operand:

```text
!sec.place<T,"rw">
```

Result:

```text
!sec.ref_mut<T>
```

Required reference semantic metadata.

---

## v11.19 `sec.ref.copy_shared`

Operand/result:

```text
!sec.ref<T> -> !sec.ref<T>
```

No ownership transfer of the referent.

No mutable authority.

---

## v11.20 `sec.ref.move`

Operand/result:

```text
!sec.ref_mut<T> -> !sec.ref_mut<T>
```

The operation represents semantic reference-value move.

It does not move referent storage.

---

## v11.21 `sec.ref.reborrow_shared`

Produces a new shared direct reference whose authority/lifetime/extent do not
exceed the source grant.

The source may be:

```text
shared
mutable
```

according to verified Sema facts.

---

## v11.22 `sec.ref.reborrow_mut`

Produces a constrained mutable reborrow.

Source authority must be mutable.

No shared-to-mutable reborrow.

---

## v11.23 `sec.ref.is_valid`

Operand:

```text
direct safe reference
```

Result:

```text
i1
```

Total abstract runtime-validity predicate.

No physical validation strategy is selected.

---

## v11.24 `sec.ref.compare`

Operands:

```text
compatible direct safe references
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

Semantic equality is live storage identity + referenced location.

---

## v11.25 `sec.ref.end_borrow`

No results.

Required:

```text
borrow_id
lifetime_id
```

Compile-time semantic lifetime marker.

---

## v11.26 `sec.fail.reference_generation`

No operands.

No results.

No successors.

Terminator.

Canonical panic reason:

```text
panic.invalid-reference-generation
```

No Result value is constructed.

---

## v11.27 Reference validity policy

Allowed compiler-generated policy values:

```text
proven
dynamic-epoch
```

A proven policy requires proof provenance.

A dynamic policy requires reference-guard validation at protected uses.

---

## v11.28 Epoch width is not a type parameter

Do not define:

```text
!sec.ref<T, epoch=64>
```

or equivalent source type identity.

Epoch width belongs to CompilationPlan representation policy.

---

## v11.29 Place escape rule

A `!sec.place` value may not be:

```text
returned
stored as ordinary data
placed in Result
placed in union/array/struct
passed as ordinary user function argument
```

Only recognized place/reference operations may consume it.

---

## v11.30 Reference function signatures

`!sec.ref<T>` and `!sec.ref_mut<T>` may appear in high-level function
parameters/results.

This does not select ABI representation.

---

## v11.31 P5 storage interaction

Storage that participates in a live place/reference identity remains high-level.

P5 must not lower it to MemRef before reference representation has been
discharged.

---

## v11.32 P6 interaction

Target scalar type resolution may recurse through the referent type while
preserving:

```text
place/ref wrapper
borrow metadata
reference semantics
```

---

## v11.33 P8 interaction

Signless normalization must not recursively erase semantic integer distinction
inside reference/place wrappers.

---

## v11.34 Verifiers

Schema v11 registers:

```text
--sec-verify-places
--sec-verify-reference-guards
--sec-verify-borrow-semantics
```

They verify emitted semantic structure.

They do not replace Sema source analysis.

---

## v11.35 `--sec-verify-places`

Checks:

```text
root/place type consistency
authority
field identity
array bounds guard
union active-variant guard
deref authority
place non-escape
```

---

## v11.36 `--sec-verify-reference-guards`

Checks dynamic-validity protected use:

```text
same reference SSA
true-edge dominance
false edge reaches invalid-generation failure
```

Proven uses require proof metadata.

---

## v11.37 `--sec-verify-borrow-semantics`

Checks:

```text
no ref-mut copy
reborrow authority
borrow/lifetime IDs
reference move structure
end-borrow use ordering
returned origin metadata
```

---

## v11.38 No physical representation

Schema v11 does not define:

```text
pointer field
epoch field
side-table key
capability bits
fat-reference struct
slot identity
physical address-space number
```

---

## v11.39 Schema-v11 completion

Schema v11 is complete when:

```text
place/ref types parse/print/verify
root/subplace operations verify
shared/mutable borrowing verifies
reference copy/move/reborrow verifies
dynamic validity guard verifies
invalid-generation endpoint verifies
reference equality remains semantic
place escape is rejected
schema-v10 regression remains valid
no physical representation is selected
```
