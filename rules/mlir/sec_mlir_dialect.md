# Sec MLIR Dialect

## Status

Normative detailed representation specification.

Current Sec MLIR dialect specification version: `12`

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
v12 borrowed shared/mutable slice semantic values
```

Version 12 adds high-level borrowed shared/mutable slice values and their
range/index operations (SEC-MLIR-P16), without choosing a physical descriptor.
The complete v12 contract is installed below from
`rules/mlir/dialect-versions/sec_mlir_dialect_v12.md`; that input remains a
historical version snapshot, interpreted with the applied corrections.

Specification version and compiler capability are separate. As of 2026-10-19,
default compiler emission remains schema 11; explicit schema-12 selection and
partial slice type/local-operation verification exist. Full slice emission and
package acceptance remain unfinished in `lowering.sec-mlir-package16`.
A schema-12 module declares:

```mlir
sec.dialect_version = 12 : i32
```

Schemas 1–11 remain readable regression inputs with their compatibility
promises. Schema 12 inherits the schema-11 early admissions listed in v11.41;
installation neither removes those operations nor completes their owning
packages. No new compilation route or automatic default promotion is created.

The schema-v10/v11 parts below were installed on 2026-10-11; v12 was installed
on 2026-10-19. Version snapshots preserve their original numbering and history.

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
sec.relocation_class   (superseded by address_stability + pinned; MD-084 amendment below)
sec.reference_origin
```

where applicable.

These attributes are semantic.

They do not force physical fields.

**Amendment MD-084 (2026-10-21; `rules/corrections/applied/md084-schema11-reference-metadata-correction-20261021.md`).** The canonical schema-11 serialization of resolved reference facts is shared unchanged by schema 12:

- **`sec.reference` dictionary.** It sits on a `func.func` argument of type `!sec.ref<T>` / `!sec.ref_mut<T>`, and on a direct call that produces a single reference result, where it describes that result. Its exact keys are `borrow_id`, `lifetime_id`, `validity_policy` (`proven` | `dynamic-epoch`), `validity_proof`, `invalidation_domain`, `epoch_dependency`, `address_stability`, `provenance`, `source_borrow`, `origin_storages`, `pinned` and `address_space`.
  - No placeholder IDs, fabricated proofs or made-up provenance are allowed.
  - `source_borrow` may be omitted only on a copy or move where it equals `borrow_id`.
  - An empty `origin_storages`, `pinned = false` and the ordinary address space may be omitted only where the verifier reconstructs exactly that default.
- **Operation properties.** Reference-producing operations keep their operation properties. These must agree with any dictionary that states the same fact; neither form replaces the other.
- **`sec.relocation_class` (listed above).** It is superseded for current emission by `address_stability` (`Movable` | `Stable` | `Fixed` | `Unknown`). Pinning is a separate fact, and no `pinned == Fixed` inference is permitted. A reader that accepts the legacy name needs an explicit compatibility path; emitters never write it.
- **Discardable attributes.** `sec.origin_storages`, `sec.pinned` and `sec.reference_address_space` appear only on operations defining exactly one reference result.
- **`sec.return_origins`.** A `func.func` attribute, required and non-empty on a function with a reference result and absent otherwise. It is a set of strings in the canonical grammar:

  ```text
  origin := root projection*;  root := "param#" N | "receiver" | "static@" N
  projection := ".field#" N | "[" N "]" | "[*]";  N := "0" | [1-9][0-9]*
  ```

  The grammar is case-sensitive and whitespace-free, and allows no duplicates. Malformed entries are rejected, never normalized. `param#N` is the zero-based source parameter ordinal. `receiver` requires a real receiver. `static@N` names a resolved static root and does not by itself authorize the return. The order is deterministic, following the Semantic IR origin summary. New root or projection kinds need a schema amendment.
- **`sec.address_taken = true`.** A boolean `true` on `sec.storage.declare` for reference-addressable storage.
- **Calls and dynamic indices.** Reference call operands carry the argument actions `shared-reference` / `mutable-reference`. Dynamic Place-index identity is the SSA index operand.
- **Names and address spaces.** `sec.source_name` is emitted only for a real source binding: compiler-materialized storage has an empty name plus `sec.synthesized = true`. Target-defined `target:` address spaces are rejected as an unsupported lowering.

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

**Amendment MD-087 (2026-10-21; `rules/corrections/applied/md087-p5-p6-p8-reference-boundaries-correction-20261021.md`).** The single authoritative query is `sec::isReferenceDependentStorage`: a declaration carrying `sec.address_taken = true` or consumed by `sec.place.storage`. `--sec-lower-trivial-core` keeps such storage as `sec.storage`, and `--sec-resolve-scalar-layout` resolves only its element type (for example `!sec.storage<i1>`). Storage without a reference dependency lowers as before. No competing predicate may be introduced.

---

## v11.32 P6 interaction

Target scalar type resolution may recurse through the referent type while
preserving:

```text
place/ref wrapper
borrow metadata
reference semantics
```

**Amendment MD-087 (2026-10-21).** A `sec.call.direct` whose operands or results contain `!sec.place`, `!sec.ref` or `!sec.ref_mut` stays a Sec call until a representation-aware lowering transports its argument actions, `sec.reference` and origin metadata losslessly. Other calls lower as before, and P6 preserves applicable discardable call attributes.

---

## v11.33 P8 interaction

Signless normalization must not recursively erase semantic integer distinction
inside reference/place wrappers.

**Amendment MD-087 (2026-10-21).** P8 must accept modules containing Places and references. Rejecting a whole module only because they appear is nonconforming, and supersedes the P15-60 blanket rejection.
- P8 lowers every integer computation whose representations are known. It never signless-normalizes or erases referent identity inside `!sec.place` / `!sec.ref` / `!sec.ref_mut`, and preserves their metadata.
- At `sec.place.read` / `sec.place.write`, calls, storage, block arguments and checked-operation results, the semantic `siN`/`uiN` contract is kept. A legal, verified representation transition to signless `iN` is materialized: exact width and bit pattern, signedness from the Sema-resolved operation, and overflow paths intact. `arith.bitcast` between signed and signless integers is not legal MLIR. `builtin.unrealized_conversion_cast` may serve only as a temporary bridge that must be reconciled before the LLVM boundary.
- Widths come from the `CompilationPlan`.
- A failed boundary conversion leaves no partially rewritten published IR.
- A specific unsupported conversion is diagnosed with its operation; it is never reported as a module-wide prohibition.

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

**Amendment MD-085 (2026-10-21; `rules/corrections/applied/md085-return-origin-verification-correction-20261021.md`).** "Returned origin metadata" is verified structurally and path-aware; this supersedes P15-56's root-only depth and its "every reference argument of a call" fallback.
- **Summary.** Every function with a reference result carries a valid MD-084 `sec.return_origins`. Each root names an eligible reference parameter, a real receiver, or a permitted static root, and each projection is type-compatible.
- **Tracing.** Each reachable reference return is traced through copies, moves, reborrows, dereferences, Place projections, block arguments and joins, using deterministic fixed-point or visited-state logic. Field steps add `.field#N`, constant elements `[K]` and dynamic elements `[*]`.
- **Calls.** A direct call contributes only the callee's own `sec.return_origins`, instantiated against the caller's actual operands. The caller prefix is followed by the callee suffix. A missing, malformed or uninstantiable callee summary fails verification.
- **Inclusion.** The observed origin set must be included in the declared set. Conservative summaries may be broader, but never omit a possible origin or name a non-existent one. A bare root does not cover projected paths unless that abstraction is defined, and distinct fields never match.
- **Local origins.** Local storage and value origins are rejected.
- **Boundaries.** The verifier never patches IR, never guesses origins, and runs no second source-level borrow or lifetime analysis. A failure in compiler-generated IR is a compiler defect, not a user error.

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

## v11.40 Exact integer conversion contract transport

The schema-11 integer admission of `sec.numeric.convert_exact` and
`sec.enum.to_integer` carries the verified Semantic IR exactness contract
(`rules/compiler/semantic_ir.md` § 124), using the discardable attribute:

```mlir
{sec.conversion_proof = {
    basis = "closed-enum-domain",
    plan_pointer_width = 0 : i32
}}
```

The closed basis set is `constant`, `source-domain`,
`closed-enum-domain`, `open-enum-domain` and `compiler-widening`.
The dictionary contains exactly these two fields. A zero plan width denotes
a plan-independent proof. A nonzero width is 32 or 64 and must equal the
selected module's explicit index data-layout width. Missing, malformed,
unknown-basis or incompatible-plan contracts are invalid.

The operand and result types remain the conversion's source and target
contract. This attribute transports the Sema-established proof; it is not
permission for a later pass to independently classify an unproven conversion
as exact. Scalar type resolution preserves the contract. Enum representation
lowering consumes it without recomputing case-domain exactness and preserves
it on any resulting `sec.numeric.convert_exact`. The contract may be removed
once its conversion semantics have been discharged.

Schemas 7–10 remain readable without this newly transported enum metadata.
Binary enum-conversion lowering still requires the explicit contract and
fails closed without it. Checked conversions remain separate fallible
operations; absence of a proof never authorizes truncation.


## v11.41 Early admissions on the single native track


Schema 11 is the executable schema of the one native track while schemas 12–15
(P16–P19) are still predecessor work. The following operations were admitted
into schema 11 ahead of their owning package so the track can build programs.
Each keeps the owning package's semantics; the owning package's schema
installation re-homes or confirms them rather than redefining them, and no
other operation may be used in schema 11 without being recorded here.

| Operation(s) | Owning package / decision | Admitted |
|---|---|---|
| `sec.numeric.convert_exact` (integer operands only), `sec.enum.to_integer` proof transport | P20 § 70; MD-104; § 40 above | 2026-10-17/18 |
| `sec.int.cmp` over `!sec.char` / `!sec.rune` | P7 amendment; MD-105 | 2026-10-17 |
| `sec.fail.panic` (reason, optional message) | panic.md §§ 8.1, 12(2) | 2026-10-18 |
| `sec.static.global`, `sec.static.load`, `sec.static.store` | static storage (P15 places, `rules/declarations/static.md`) | 2026-10-18 |
| `sec.const.float {bits}`, `sec.float.neg`, `sec.float.binary`, `sec.float.cmp` | P20 §§ 20–26, 63–65, 73–77 (`lowering.sec-mlir-package20`) | 2026-10-18 |
| `!sec.register<T>`, `!sec.hw_instance<T>`, `sec.register.extract`, `sec.register.update`, `sec.hw.bind`, `sec.hw.observe`, `sec.hw.request_write`, `sec.hw.shadow_observe`, `sec.hw.shadow_intent`, `sec.hw.shadow_invalidate`, `sec.hw.commit` | MD-077 § 19 (v11.41.1 below); P15-42/42a | 2026-10-22 (specification; implementation tracked in governance) |
| `sec.registers` (`#sec.register_def`, `#sec.register_field`), `sec.field_path`, `sec.write_proof` (`#sec.register_write_proof`) | MD-114 (v11.41.1a below); P15-42 | 2026-10-27 (specification; implementation tracked in governance) |
| `sec.register.fits_bits`, `sec.register.from_bits`, `sec.register.to_bits` | MD-110 § 8 (v11.41.2 below); P15-42 | 2026-10-25 (specification; implementation tracked in governance) |
| `sec.place.static`; `sec.static_id` and `sec.address_taken = true` on `sec.static.global` | MD-116 (v11.41.3 below); P15-42/42a/47 | 2026-10-27 (specification; implementation tracked in governance) |

Schemas 7–10 remain readable without these operations. Lowering of each one is
listed in `sec-check-lowering-support`; a module using any of them outside
schema 11 or later is rejected by the version gate of the installing package.

### v11.41.1 Register values and hardware operations (MD-077 early admission)

Installed 2026-10-22 from `rules/corrections/applied/md077-register-semantic-ir-final-decision-20261022.md` § 19. Schema 12 inherits this subsection verbatim. Schemas 1–10 reject every construct in it. The default emitted schema is unchanged.

**Types.**
- `!sec.register<"module::TypeName">` is a detached nominal register value.
- `!sec.hw_instance<"module::TypeName">` is a software implementation-instance and authority descriptor.

Neither is equivalent to the other or to an integer. The quoted name must resolve to the same canonical nominal register type in the module's verified register-definition table, and neither spelling implies an instance-state layout or an endpoint representation.

**Operations.** The Semantic IR → MLIR mapping is:
- `register.value.extract` → `sec.register.extract`
- `register.value.update` → `sec.register.update`
- `register.hw.bind` → `sec.hw.bind`
- `register.hw.observe` → `sec.hw.observe`
- `register.hw.request_write` → `sec.hw.request_write`
- `register.hw.shadow_observe` → `sec.hw.shadow_observe`
- `register.hw.shadow_intent` → `sec.hw.shadow_intent`
- `register.hw.shadow_invalidate` → `sec.hw.shadow_invalidate`
- `register.hw.commit` → `sec.hw.commit`

**Value operations.**
- `sec.register.extract` consumes `!sec.register<T>` and a validated `sec.field_path`, and produces the exact resolved field value type.
- `sec.register.update` consumes `!sec.register<T>`, one value of the field's carrier or nominal enum type, and the same `sec.field_path`, and produces `!sec.register<T>`.
- Both are pure and verified against range, layout, reserved bits and field type.
- The module-level register definition lists ordered field names and IDs, bit widths and offsets, representation modes, effective access contracts and nominal identity.

**Hardware operations.**
- `sec.hw.bind` yields `!sec.hw_instance<T>`. It carries the canonical instance, binding and resource IDs, the endpoint kind, the implementation identity and target-plan applicability. A runtime-mapped resource takes its mapping as an explicit operand. The operation performs no physical I/O and rejects a binding without the mandatory `impl T`.
- `sec.hw.observe` and `sec.hw.request_write` take a compatible instance, a validated projection or path (and a request value for writes) and an immutable verified plan reference. Observation returns a detached value on its success path, and failures use the resolved Result/failure CFG. Both are forbidden outside validated `impl T` access paths.
- The shadow operations and `sec.hw.commit` act on identified per-instance shadow groups. They keep `Unknown` / `Known(value)` knowledge separate from pending logical intents.
- `sec.hw.commit` may have no physical effect when no intents are pending, and is never synthesized at destruction or at a borrow end.

**Mandatory metadata** on hardware operations:
- `sec.register_type_id`, `sec.impl_instance_id`, `sec.hardware_binding_id`, `sec.hardware_resource_id`, `sec.endpoint_id`;
- `sec.field_path` or an explicit whole-register projection;
- `sec.access_behavior`, `sec.effect_contract`, `sec.verified_plan_id`, `sec.compilation_plan_identity`, `sec.source_provenance`;
- the physical width, alignment and footprint, shadow, ordering, completion, privilege or access-context, and fault-plan facts.

These may reference immutable module-scoped plan records, which must exist and satisfy every constraint. Free-form descriptive strings never substitute for canonical IDs or typed lists. Definition tables, identities and plan records are module-scoped, unique, deterministically ordered and resolved during verification.

**References.**
- `!sec.ref<!sec.hw_instance<T>>` and `!sec.ref_mut<!sec.hw_instance<T>>` are ordinary reference categories over a referable software instance Place.
- `!sec.ref<!sec.register<T>>` references a detached register value.
- The verifier rejects `sec.place.read`, `sec.place.write` or `sec.ref.borrow_*` applied directly to a physical resource or endpoint.

**Effects and verification.**
- Hardware operations carry memory/effect information that prevents elimination, duplication, speculation, movement, coalescing and reordering, even when their result is unused.
- The verifier checks preservation of the established contracts: operand and result types, required attributes, field width, range and enum facts, instance and resource bindings, plan and target eligibility, fault edges and effect declarations. A missing plan or capability is a fail-closed diagnostic.
- `emit-sec-mlir` may emit these operations only once their complete parse, print and verify contract is implemented. Physical lowering stays capability-gated per endpoint kind.

#### v11.41.1a MD-114 amendment — register table, field paths and write proofs

Installed 2026-10-27 from `rules/corrections/MD-114-canonical-register-value-mlir-contract-correction.md`. It spells the module register-definition table, `sec.field_path` and the update proof that v11.41.1 requires. This amendment covers only the detached value operations and the MD-110 width lookup. It is an explicit schema-11 early admission: schema 12 inherits it unchanged, and schemas 1–10 reject these constructs. It adds no source syntax, physical layout, ABI, byte swap or hardware access. Hardware records, endpoints and access plans stay with MD-111, and a register table never makes a `sec.hw.*` operation acceptable.

**Module table `sec.registers`.** A module that uses any register type has exactly one module attribute `sec.registers`: an `ArrayAttr` of the typed attribute `#sec.register_def<...>`. A module without register types may omit it, and `sec.registers = []` is an explicit empty table. A module that uses register types may not omit it, and no table is ever assembled from operation names.
- **Record fields.** `#sec.register_def<identity = ..., width = ..., order = ..., byte_order = ..., fields = [...]>`:
  - `identity`: the nonempty, complete nominal identity used in `!sec.register<"...">`;
  - `width`: the positive logical bit width;
  - `order`: `"lsb-first"` or `"msb-first"`, always emitted explicitly;
  - `byte_order`: `"little-endian"` or `"big-endian"`, present only when explicitly resolved. It is omitted when byte order is contextual and is never a host guess;
  - `fields`: every declared field, including reserved ones, in declaration order.
- **Uniqueness and order.** Each identity occurs once, even if it is used in several functions or nested in several registers. Identities are case-sensitive and nominal, so equal layout never means equal identity. The table follows the deterministic first-use order of the Semantic IR register table, never hash or alphabetical order.
- **Rejection.** Duplicate or conflicting definitions, a referenced type without a definition, missing mandatory keys and unknown keys are all rejected.

**Field records `#sec.register_field<ordinal, name, offset, width, type, access, shadow, unit, reserved>`.** Keys are printed in that order, and `unit` is present only when applicable.
- **`ordinal`.** A nonnegative i32. Ordinals are contiguous `0..count-1` in declaration order, reserved fields included.
- **`name`.** The declared name, or `"_"` for a reserved field. `reserved` is `true` exactly for those fields. Nonreserved names are unique within a register, and a reserved field is never addressable.
- **Layout.** `offset` is the lowest logical bit, with bit 0 the LSB, and `width` is positive. The intervals `[offset, offset + width)` lie inside the register, never overlap, and together cover its whole width. They must agree with the declaration-order allocation for the register's `order`, and an offset is never read as MSB-relative.
- **`type`.** The resolved value type:
  - a one-bit `bit` field is `i1`;
  - a `bit[N]` field with `N >= 2` uses the smallest standard unsigned carrier (8 to 256 bits), with the field's `N`-bit range as its contract;
  - a bit-backed enum keeps its nominal enum type;
  - a nested register is `!sec.register<"...">`, with its own definition in the table and exactly matching width.

  No serializer replaces any of these with an arbitrary same-width integer. Nested layouts that are cyclic or inconsistent are rejected. A nested register keeps its own `order` and `byte_order`, with no implicit byte swap.
- **`access` and `shadow`.** `access` is the effective access spelling of `registers.md` § 11: `read-write`, `read-only`, `write-only`, `read-clear`, `read-set` or `write-one/zero-{clear,set,toggle}`. `shadow` is the effective boolean policy after defaults are applied. Unknown spellings are rejected. On detached values both are layout metadata and have no side effects.
- **`unit`.** The canonical unit identity resolved by Sema, never a display abbreviation. A type, unit or enum identity that the schema cannot represent fails with an unsupported-lowering diagnostic instead of being erased.

**`sec.field_path`.** This is a required, nonempty `ArrayAttr` of i32 ordinals on `sec.register.extract` and `sec.register.update`. The first ordinal selects a field of the operand's register, and each later ordinal selects a field of the nested register selected by the previous step. A step into a scalar field, an unknown ordinal, a reserved field or a missing nested definition is rejected. The verifier composes the effective slice from the table. Bit offsets, source names and addresses never stand in for ordinals, and equal-width slices are never interchangeable. Whole-register conversions, copies and moves carry no field path. An empty path is never used as a stand-in for them.

**Operation typing.**
- **`sec.register.extract`** takes one `!sec.register<T>` operand and produces the leaf field's exact type. It has no `sec.write_proof`.
- **`sec.register.update`** takes the original register and a replacement of the leaf field's exact type, and returns the same `!sec.register<T>`. Only the selected slice changes; every other bit, including reserved bits, is preserved.
- **Purity.** Both operations are pure: they never bind, read, write, observe or commit hardware. A field's hardware access contract does not forbid a detached operation. Ordinary value mutability and the field-write fit rules still apply.
- **Nested fields.** A nested path, or the equivalent sequence of one-level typed operations, keeps every intermediate nominal identity and the evaluation order.

**`sec.write_proof`.** This attribute is required on `sec.register.update` and is the typed `#sec.register_write_proof<basis = ..., low = ..., high = ...>`, which serializes Semantic IR `RegisterWriteProof`:
- `basis = "typed"` has no bounds. It is allowed only when the replacement already has the exact `bool`, bit-backed enum or nested register type. A plain unsigned carrier never qualifies: `uint8` does not prove that a `bit[3]` value fits.
- `basis = "constant"` requires `low == high`, equal to the proven constant of the replacement and within the field's interval. The verifier checks it against the represented constant and never accepts the text alone.
- `basis = "interval"` requires the certified inclusive range `0 <= low <= high <= 2^width - 1` that the producer transported from Sema. It adds no runtime check or failure edge.
- Bounds are canonical arbitrary-precision decimal strings: no sign, whitespace or redundant leading zeros, compared without truncation. Missing or extra bounds, an unknown basis, reversed or out-of-field bounds, and a proof that belongs to another field, operand, register or unit are all rejected.
- This proof is distinct from MD-110's `sec.register_fit_proof`, and the two are never interchanged.

**MD-110 lookup.** `sec.register.fits_bits`, `sec.register.from_bits` and `sec.register.to_bits` resolve their register identity and width through the same `sec.registers` table and never through a guessed integer type. Their names, bases (`constant`, `source-domain`, `guarded`) and semantics are unchanged. The Semantic IR basis `bit-extraction` still has no MLIR spelling (MD-110 § 8(4)). It is reconciled under MD-110 and is never emitted as an invented fourth basis.

**Verification and preservation.**
- **Attributes.** Both attributes are registered TableGen attributes with exact key names, omission of optional keys and canonical print order. A dictionary that merely looks similar is not accepted.
- **Module verifier.** It checks identities, definition presence, widths, field layout, allocation and byte order, field types, policy spellings and nesting.
- **Operation verifiers.** They check arity, types, the path, reserved-field exclusion, effects and the shape and value of the proof.
- **Passes.** Passes keep register nominal identity and never strip a table that is still referenced. A table may be regenerated only if it is derived deterministically and completely from the verified semantic identities, with no reordering, renumbering or reserved-bit loss. Layout and proof facts are removed only after a verified lowering has discharged them.
- **Failure.** A failed verification or an unsupported register operation publishes no partial output and never falls back to plain integer operations.

### v11.41.2 Register bit-pattern conversions (MD-110 early admission)

Installed 2026-10-25 from `rules/corrections/applied/md110-register-bit-pattern-conversion-ir-20261025.md` § 8, after v11.41.1. Schema 12 inherits it verbatim, schemas 1–10 reject it, and the default emitted schema is unchanged. The register type remains MD-077's `!sec.register<"module::TypeName">`.

**Mapping.**
- `register.value.fits-bits` → `sec.register.fits_bits`
- `register.value.from-bits` → `sec.register.from_bits`
- `register.value.to-bits` → `sec.register.to_bits`

**Operations.**
- **`sec.register.fits_bits`** takes one resolved scalar-integer operand and a canonical destination register type-identity attribute, and returns `i1`. Its predicate is exactly the bit-pattern fit rule (`registers.md` § 12.3): `0 <= x <= 2^N-1` for unsigned or non-negative sources, and `-2^(N-1) <= x` for negative signed sources. It is never a plain unsigned range test, and it has no side effects.
- **`sec.register.from_bits`** takes one resolved primitive-integer operand and produces `!sec.register<T>`, resolving `T` and its `N` in the module register-definition table. It performs no byte-order or endian transform. It carries the mandatory discardable `sec.register_fit_proof = {basis = "constant" | "source-domain" | "guarded"}`, with exactly that one field:
  - `constant`: the operand is a verifiable constant that fits;
  - `source-domain`: every value of the operand's resolved type fits;
  - `guarded`: the true edge of a `sec.register.fits_bits` on the same SSA value and register type dominates the conversion on every path, and its false edge reaches the resolved checked-conversion failure.

  The verifier rejects a missing or unknown basis, mismatched operands, absent evidence, wrong control flow and an unresolved target integer width. The attribute transports proof; it cannot create it.
- **`sec.register.to_bits`** takes one `!sec.register<T>` and produces a signed or unsigned integer of width `W >= N`: zero-extended for an unsigned result, sign-extended from bit `N-1` for a signed one. It is pure and total, with no proof and no error successor. A narrower or non-integer result is rejected.

**Shared rules.**
- **Representation.** The Sec dialect boundary uses the public `siW` / `uiW` scalar representations. Later signless lowering preserves the bits and carries the signed interpretation; it must not infer the interpretation from a bare `iW`.
- **What these operations are not.** They are detached, effect-free value computations: never byte serialization, native-memory copies, hardware loads or stores, or an illegal signed-to-signless `arith.bitcast`.
- **Admission.** C++ registration, round trips, verifiers, the Go emitter, the version gate and the lowering-support manifest must agree on these spellings before any of them is claimed. `from_bits` resolves its register type and width through the MD-114 `sec.registers` table (v11.41.1a, decided 2026-10-27).

### v11.41.3 Static Place roots and static identity (MD-116)

Installed 2026-10-27 from `rules/corrections/applied/MD-116-static-place-root-mlir-correction-20261027.md`, after v11.41.2. Schema 12 inherits it unchanged, schemas 1–10 reject it, and the default emitted schema is unchanged. It is the MLIR form of the Semantic IR `place.static-root` (`semantic_ir.md` §§ 10(12), 28(6)). It adds no source syntax, initialization policy, TLS rule, pointer layout or static ABI.

**Three identities.** They are independent and never derived from one another:
- the module-local `StaticID` selects one `Static` in the Semantic IR module (module-level or function-local `static let`);
- the Place's `PlaceRootID` and `StorageIdentityID` keep their P15 meaning;
- the `sec.static.global` symbol (for example `@sec_static_1`) is only a name. No verifier parses its suffix or `sec.source_name` to infer an identity.

**Declaration attributes.**
- Every `sec.static.global` emitted in schema 11 or later carries the discardable `sec.static_id = N : i32`, where `N` is the nonzero Semantic IR `StaticID` read as an unsigned 32-bit identity. IDs and symbols are unique in a module and deterministic for one verified Semantic IR module.
- A static that is reference-addressable carries the MD-084 marker `sec.address_taken = true`, mirroring `Static.AddressTaken`. Every global targeted by a `sec.place.static` must carry it; an explicit `false` does not satisfy the requirement.
- A legacy scalar-only global without `sec.static_id` is accepted only through a documented compatibility path, and no `sec.place.static` or `static@N` may refer to it. A missing, zero, duplicated, truncated or mismatched ID on a referenced static is an error, never a default.
- `StaticID` is module-scoped. Equal IDs in different modules never denote the same storage, and an imported static origin that cannot be linked to its owning module's verified declaration fails closed. No cross-module ABI is introduced.

**`sec.place.static`.**

```mlir
%place = "sec.place.static"() <{
  global = @sec_static_1,
  place_root_id = 12 : i32,
  storage_identity = 7 : i32,
  address_space = "ordinary"
}> : () -> !sec.place<si32, "ro">
```

- No SSA operands and one result, `!sec.place<T, "ro" | "rw">`. `T` is exactly the element type of the selected global at the same stage; P6 resolves both together.
- `global` is a `FlatSymbolRefAttr` that resolves to exactly one `sec.static.global`. `place_root_id`, `storage_identity` and `address_space` use the existing P15 representation and vocabulary, and are independent of the static ID.
- `"rw"` requires `is_mutable = true`. A mutable static may also yield `"ro"` under a Sema-granted shared borrow, and `"ro"` establishes no thread safety.
- It denotes the existing storage that `sec.static.load` / `store` name by the same symbol and ID. It creates no initialization, backing store, temporary, pointer, allocation or indirection, does not load the value and performs no I/O.
- Function-local statics are lowered to one module-level global and are never reinitialized per call. Places rooted at the same static keep the same storage identity, and conflicting root or storage claims are rejected.
- A nonordinary or `target:` address space is admitted only when the CompilationPlan and the schema can represent and verify it, and is never mapped to `ordinary`. MMIO and hardware endpoints stay with MD-111 / MD-114.

**Returned static origins.** `static@N` (MD-084 grammar, unchanged) names `StaticID` N. The MD-085 tracer recognizes `sec.place.static` as a root, resolves `global`, reads the explicit `sec.static_id` and records `static@N`; projections, borrows, reborrows, copies, moves, block arguments and verified calls follow v11.37. A `static@N` string without a traceable static root, an absent ID or a foreign module's ID is rejected. A static origin is provenance only, never permission for a borrow Sema did not grant. A `!sec.place` never escapes as a returned value.

**Verification and passes.**
- The operation verifier checks operand and result counts, the Place type, symbol resolution and kind, type equality, mutability, root metadata types, the address space and the address-taken marker. The module verifier checks `sec.static_id` presence and uniqueness for identity-bearing statics.
- `--sec-verify-places` accepts the operation as a legal zero-operand Place root. `--sec-verify-borrow-semantics` applies static return-origin tracing, and `--sec-verify-reference-guards` keeps its guard requirements; static lifetime is never blanket `proven` validity.
- P5 never discharges an address-taken static, P6 resolves its scalar types consistently and P8 keeps its Place and reference wrappers (MD-087). `sec-lower-static-storage` keeps lowering scalar-only statics and rejects a reference-dependent static it cannot represent instead of erasing the Place.
- Failed verification or unsupported emission publishes no partial MLIR.


---

# Schema version 12 — borrowed slices

## v12.1 Version history

Schema 12 inherits the MD-077 register and hardware admission of v11.41.1 verbatim (2026-10-22). It likewise inherits the MD-110 register bit-pattern conversions of v11.41.2 (2026-10-25).

```text
v1   dialect foundation
v2   Semantic IR bridge
v3   scalar/target coverage
v4   checked integer operations
v5   typed arithmetic failure and Result construction
v6   Result branching/local try handlers
v7   enum and union values
v8   verified match CFG
v9   struct semantic values
v10  fixed-array semantic values
v11  places and direct references
v12  slice semantic values
```

Compiler-generated v12 modules carry:

```mlir
sec.dialect_version = 12 : i32
```

---

## v12.2 `!sec.slice<T>`

Shared safe slice.

High-level semantics:

```text
bounded
runtime-length
non-owning
shared
copyable
trivially destructible
```

No length/origin/epoch type parameter.

---

## v12.3 `!sec.slice_mut<T>`

Mutable safe slice.

High-level semantics:

```text
bounded
runtime-length
non-owning
exclusive mutable
move-only
trivially destructible
```

No length/origin/epoch type parameter.

---

## v12.4 No owning sequence conflation

Neither slice type represents:

```text
T[]
```

owning dynamic sequence values.

---

## v12.5 Reference metadata

Slice-producing operations retain the same semantic metadata family as direct
references (existing names below identify facts, not new serialization choices):

```text
sec.borrow_id
sec.lifetime_id
sec.validity_policy
sec.validity_proof
sec.storage_identity
sec.epoch_dependency
address stability (storage.md § 14)
separate pin dependency (storage.md § 24)
sec.address_space
sec.reference_origin
```

Address stability uses `Movable`, `Stable`, `Fixed` or `Unknown`; pinning is
separate, as required by P15 § 28 (revised 2026-10-03). These are semantic facts,
not prescribed physical fields. The legacy snapshot's `sec.relocation_class`
is withdrawn. The concrete schema-11 metadata spellings for parameters, calls,
returned origins and optional facts were decided by MD-084 (2026-10-21; v11.7
amendment) and are inherited unchanged by schema 12. Slices reuse the same P15
metadata family rather than defining a parallel one.

---

## v12.6 Range provenance metadata

Optional canonical metadata:

```text
sec.slice_range_id
sec.slice_static_start
sec.slice_static_end_exclusive
```

Static values use canonical arbitrary-precision decimal representation.

---

## v12.7 `sec.slice.range_normalize`

Operands:

```text
source_length
optional start
optional end
```

Results:

```text
start: uint
length: uint
```

Required attrs identify:

```text
start present
end present
endpoint signedness
upper-bound kind
proof kind
```

Valid only for Sema-proven-safe range.

---

## v12.8 `sec.slice.range_check`

Operands:

```text
source_length
optional start
optional end
```

Results:

```text
normalized_start: uint
normalized_length: uint
failed: i1
error: canonical RangeError enum
```

Total high-level checked range operation.

---

## v12.9 Range error mapping

Canonical runtime mapping:

```text
InvalidRange
StartAfterEnd
OutOfBounds
```

according to the synchronized P16 range rules.

---

## v12.10 Range check branch convention

Compiler-generated runtime form:

```text
failed == true  -> failure path
failed == false -> success path
```

Same convention as other checked Sec operations.

---

## v12.11 `sec.slice.borrow_shared`

Operands:

```text
compatible contiguous source Place
normalized start
normalized length
```

Result:

```text
!sec.slice<T>
```

No allocation.

---

## v12.12 `sec.slice.borrow_mut`

Requires writable compatible contiguous source Place.

Result:

```text
!sec.slice_mut<T>
```

---

## v12.13 `sec.slice.reborrow_shared`

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

No range/lifetime/authority widening.

---

## v12.14 `sec.slice.reborrow_mut`

Source:

```text
!sec.slice_mut<T>
```

Result:

```text
!sec.slice_mut<T>
```

No mutable reborrow from shared slice.

---

## v12.15 `sec.slice.copy_shared`

```text
!sec.slice<T> -> !sec.slice<T>
```

Copies descriptor/reference semantics only.

---

## v12.16 `sec.slice.move_mut`

```text
!sec.slice_mut<T> -> !sec.slice_mut<T>
```

Move-only holder transfer.

---

## v12.17 `sec.slice.len`

Operand:

```text
shared or mutable slice
```

Result:

```text
!sec.uint
```

before P6 target scalar resolution.

---

## v12.18 Unsigned length compatibility

Source `.Len` and `len(slice)` both use `sec.slice.len` with result `!sec.uint`.
The historical `sec.slice.len_int` proposal is superseded by MD-074/MD-088
(`rules/corrections/applied/md051-md074-md088-length-numeric-string-correction-20261017.md`
§§ 1–3). It is not an admitted schema-12 operation. No signed representability
proof, truncation, wrapping, saturation or implicit conversion is introduced.
A signed offset uses a separate explicit checked conversion.

---

## v12.19 `sec.slice.index_in_bounds`

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

Total spatial bounds predicate.

---

## v12.20 `sec.slice.element_place`

Operands:

```text
slice
index
```

Result:

```text
!sec.place<T,"ro">
```

for shared source or:

```text
!sec.place<T,"rw">
```

for mutable source.

Required:

```text
bounds_kind
bounds_proof
```

---

## v12.21 `sec.ref.is_valid` extension

Schema v12 extends direct-reference validity operand support to:

```text
!sec.slice<T>
!sec.slice_mut<T>
```

No new slice generation-check operation is introduced.

---

## v12.22 `sec.ref.end_borrow` extension

Borrow IDs associated with direct slices may use the same canonical borrow-end
marker.

No slice-specific duplicate lifetime op.

---

## v12.23 `sec.fail.bounds` extension

Allowed operation provenance includes:

```text
fixed-array-index
slice-index
slice-range
```

Semantic panic reason remains:

```text
panic.bounds
```

---

## v12.24 Slice guard verifier

Register:

```text
--sec-verify-slice-guards
```

It validates:

```text
range checked CFG
range normalized output use
RangeError failure use
reborrow narrowing
index guard
element-place guard
slice identity
mutable authority
dynamic reference-validity dominance
```

---

## v12.25 Reference verifier integration

Existing:

```text
--sec-verify-reference-guards
--sec-verify-borrow-semantics
--sec-verify-places
```

are extended for slice direct-reference values and slice element Places.

---

## v12.26 No slice equality

Schema v12 defines no:

```text
sec.slice.cmp
```

Source slice equality remains unsupported.

---

## v12.27 No physical descriptor

Schema v12 does not define:

```text
base pointer
physical length field
epoch field
capability bounds
handle representation
FFI layout
```

---

## v12.28 No default slice op

There is no:

```text
sec.slice.default
```

Safe slices require a valid explicit origin.

---

## v12.29 Empty slice

An empty slice is represented by the same semantic slice type and runtime length
zero.

No special nullable slice type exists.

---

## v12.30 P6 compatibility

Target scalar resolution may recurse into element types while preserving slice
wrappers and metadata.

It also resolves:

```text
sec.slice.len -> uint width
```

---

## v12.31 P8 compatibility

Signless normalization does not recurse through slice wrappers.

---

## v12.32 Place integration

`sec.slice.element_place` creates canonical P15 Place values.

Value read/write/reference borrowing uses P15 operations rather than duplicate
slice memory ops.

---

## v12.33 Schema-v12 completion

Schema v12 is complete when:

```text
slice types parse/print/verify
range ops parse/print/verify
borrow/reborrow ops verify
copy/move classification is enforced
length ops verify
index/place ops verify
reference guard integration works
empty slices are valid
owning T[] is not represented as slice
schema-v11 regressions remain valid
no physical descriptor layout is selected
```

---

## v12.34 Returned slice range origins (MD-115)

Installed 2026-10-27 from `rules/corrections/applied/MD-115-returned-slice-origin-projections-correction-20261027.md`. This is a schema-12 amendment to the MD-084 `sec.return_origins` grammar (v11.7 amendment). Schemas 1–11 must reject the two new projections, and existing root and element projections stay valid in schema 12. It adds no source syntax, runtime slice tag, descriptor layout, ABI or error kind.

**Grammar.** In schema 12 the projection rule gains exactly two alternatives:

```ebnf
projection := ".field#" unsigned32 | "[" unsigned32 "]" | "[*]"
            | "[" natural "..<" natural "]"
            | "[?]"
natural    := "0" | nonzero_digit digit*   // arbitrary precision
```

- **`[A..<B]`** is an exact half-open range `[A, B)` relative to the immediately preceding origin's current view, with `0 <= A <= B`. The endpoints are arbitrary-precision decimals with no sign, whitespace, leading zeros or separators. The parser keeps their exact values (for example as APInt) and never narrows them to the `uint32_t` used for legacy IDs and indexes. It may reject resource-exhausting input with an explicit diagnostic, but never truncates.
- **`[?]`** is some valid sub-range of the preceding view whose bounds are not statically expressible. It may be empty or whole and proves no disjointness or narrower bound. It keeps the origin and its containment and is a statement about spatial precision only.
- **Not synonyms.** `[..]` is not a metadata projection: an unprojected root such as `param#0` already means the unchanged whole current view. `[*]` stays a single element at an unknown index. A range projection keeps a sequence or view, while an element projection yields an element; neither substitutes for the other, and no parser rewrites `[?]` as `[*]` or as a bare root.
- The MD-084 rules on case, whitespace, duplicates, deterministic order and rejecting malformed entries all apply. There is no second attribute such as `sec.slice_return_origins`.

**Typing and composition.**
- A range applies only to a fixed-array or slice view that admits narrowing, and yields a slice view of the same element type. A following element projection must keep the range and its verified bounds, and may fail closed until the whole chain is supported.
- Authority may be kept or reduced but never increased.
- An exact empty range such as `[2..<2]` keeps all storage, lifetime, epoch and pin dependencies.
- Exact nested ranges compose by source-relative addition with arbitrary precision: `[10..<20]` then `[2..<5]` gives `[12..<15]`. The summary records the normalized range relative to the original origin.
- A full-open reborrow keeps the current origin: `param#0` stays `param#0`, and `param#0[12..<15]` stays narrowed.
- A genuinely dynamic bound becomes `[?]`. An implementation or verifier gap is not permission to weaken a range Sema established exactly; that path stays unsupported.
- Distinct candidate origins, such as one exact and one unknown, remain distinct.

**Verification (extends v11.37 / MD-085).**
- For `sec.slice.borrow_shared` / `borrow_mut` / `reborrow_shared` / `reborrow_mut`, the tracer reads the normalized start and length operands and the source. An exact claim must be proved from the represented SSA range operations, never from the attribute text.
- `[?]` is accepted only when the represented operations prove containment within an eligible source and the return is otherwise permitted. It never hides a local origin, a missing guard, forged storage or escalated authority.
- An outer full-open operation never erases an inner narrowing.
- Direct calls substitute the caller's actual input path and compose ranges. A callee's `param#0[?]` is never treated as exact, and callee and caller statics are never conflated.
- Unsupported CFG relations, unvalidated callee summaries and unrepresentable projections fail closed. A local-array slice is rejected whatever its annotation.

Reference validity, lifetime, storage identity, address stability, epoch, pin and address space stay in `sec.reference` and slice metadata, and are not duplicated in the range suffix. Range checks, `RangeError`, `try` and evaluation order are unchanged; the metadata performs no check of its own. Emitting a returned sub-range requires schema 12 through explicit schema selection, or the emission is rejected. The schema is never upgraded silently, and schema-12 suffixes are never embedded in schema 11.
