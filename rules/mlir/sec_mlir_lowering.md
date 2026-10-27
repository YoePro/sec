# Sec MLIR Lowering

## Status

Normative lowering specification.

Current lowering specification version: `12`

Version 8 retains the high-level enum/union representation boundary and adds
resolved variant-oriented match lowering as explicit verified CFG.

It does not perform physical tagged-union lowering.

Version 9 additionally lowers canonical Semantic IR struct values to
high-level Sec MLIR struct operations without choosing physical aggregate
layout.

Version 10 defines lowering from fixed-array Semantic IR to high-level
schema-v10 Sec MLIR (SEC-MLIR-P14). Physical fixed-array layout remains
deferred.

Version 11 defines lowering from canonical Semantic IR places/direct
references to high-level schema-v11 Sec MLIR (SEC-MLIR-P15). It deliberately
stops before physical pointer/reference representation.

Version 12 defines high-level slice lowering, installed below on 2026-10-19
from `rules/mlir/lowering-versions/sec_mlir_lowering_v12.md`, with current
unsigned length and P15 address-stability decisions applied. Versions 10 and
11 were installed on 2026-10-11. The versioned inputs remain historical
snapshots; the corrected consolidated sections govern current integration.

This specification does not assert implementation completion. Default emission
remains schema 11; explicit schema-12 selection does not imply complete slice
operation emission, physical lowering or P16 acceptance. Follow
`lowering.sec-mlir-package16` for implemented boundaries. All supported work
uses the one Semantic IR → Sec MLIR → standard MLIR → LLVM dialect → LLVM IR
→ clang track; unsupported operations fail explicitly.

---

# 1. Enum high-level boundary

`!sec.enum` remains a high-level semantic type after Package 11.

Even though the native representation is the underlying integer type, enum
identity remains useful for:

```text
safe explicit conversions
handler dispatch
future match lowering
debugging
verification
```

Do not erase enum identity in lowering v7.

---

# 2. Enum target resolution

The scalar-layout pass may resolve target-sized enum underlying types.

Example:

```text
!sec.enum<..., !sec.int, ...>
    -> !sec.enum<..., si32, ...>
```

on a 32-bit plan.

Likewise:

```text
-> si64
```

on a 64-bit plan.

The wrapper/case table remains unchanged.

---

# 3. No enum signless normalization yet

The checked-integer Arith pass must not recurse into `!sec.enum`.

A dedicated future enum-representation pass will convert the enum to the lower
integer representation when semantic variant identity is no longer required.

---

# 4. Enum constants remain high-level

`sec.enum.constant` remains.

Do not prematurely convert it to `arith.constant` in P11.

This preserves declared-case provenance and enum identity.

---

# 5. Enum conversions remain high-level

Keep:

```text
sec.enum.from_integer
sec.enum.to_integer
```

until integer conversion semantics and enum representation erasure are lowered
together.

No hidden truncation.

---

# 6. Enum comparison remains high-level

Keep:

```text
sec.enum.cmp
```

through P11.

Future enum representation lowering may convert it to integer equality once both
operands use the same erased representation.

---

# 7. Union high-level boundary

`!sec.union` remains a high-level semantic tagged union.

Keep:

```text
sec.union.construct
sec.union.is_variant
sec.union.unwrap_payload
sec.union.unwrap_field
```

No operation becomes a raw tag/load/store in P11.

---

# 8. Optional layout metadata

When Semantic IR carries a valid resolved layout reference, preserve:

```text
sec.layout_ref
```

where the existing metadata convention applies.

When no resolved layout exists, do not invent one.

---

# 9. Union construction is semantic

`sec.union.construct` means:

```text
create a semantic union value with active variant V and initialized payload
```

It does not mean:

```text
allocate stack storage
write integer tag
memcpy payload bytes
```

Those are later representation operations.

---

# 10. Union variant test is semantic

`sec.union.is_variant` tests the semantic active variant.

It does not expose or compare the physical tag integer.

---

# 11. Union projection safety

`sec.union.unwrap_payload` and `sec.union.unwrap_field` require a verified
matching path.

Run:

```text
--sec-verify-union-guards
```

before transformations that may discard high-level control information.

---

# 12. Payload ownership gate

Lowering v7 compiler generation permits:

```text
copy-trivial
```

payload action only.

If a union payload requires:

```text
move
semantic copy
non-copyable handling
conditional copy
non-trivial destruction
```

the P11 pipeline rejects that value operation as unsupported.

Do not lower it as an ordinary SSA copy.

---

# 13. ArithmeticError migration

Schema-v7 new output uses:

```text
!sec.enum
```

for ArithmeticError.

The local try handler path uses:

```text
enum constant
enum equality
```

for specific variants.

Do not emit new `!sec.core_error`/`sec.core_error.is_variant` operations for
ArithmeticError.

---

# 14. Legacy schema compatibility

Schema-v6 regression tests may retain:

```text
!sec.core_error
sec.core_error.is_variant
```

No automatic migration pass is required in Package 11.

The schema-v7 compiler emitter simply uses the new representation.

---

# 15. Result boundary

`!sec.result<T,E>` remains high-level.

It may contain:

```text
E = !sec.enum<...>
```

P10 Result guard/unwrapping remains valid.

Do not lower Result to `!sec.union` in P11.

---

# 16. Option boundary

Concrete Option may use ordinary union representation.

If older code has a dedicated Option representation, migration may be deferred
provided new P11 union semantics remain canonical.

---

# 17. Generic union boundary

Runtime union ops require concrete substituted payload types.

Do not lower unresolved generic union template values.

The same boundary applies to every compiler-generated executable
generic-dependent Sec MLIR type and operation, as specified by
`rules/compiler/generics_lowering.md`. Unresolved parameters, partially
specialized nominal types, and unresolved constraint operations must not cross
this boundary. A valid concrete specialization uses the ordinary concrete
function, method, struct, named-type, union, enum, interface, impl-member, and
associated-state lowering paths; missing coverage is an implementation gap.

---

# 18. Struct-like source evaluation order

Lowering from source/Semantic IR to `sec.union.construct` preserves source
expression evaluation order.

Canonical operand ordering inside the final construct op may follow declaration
field order only after all source expressions have been evaluated to SSA values.

---

# 19. No general match lowering

P11 lowering does not create complete match CFG.

The operations introduced by P11 are the primitives consumed by Package 12.

---

# 20. No physical union lowering

Do not lower to:

```text
arith tag constants
memref byte buffer
LLVM struct
LLVM insertvalue/extractvalue
manual alignment padding
```

in lowering v7.

---

# 21. No union equality lowering

Current frontend may classify unions as equality-comparable when every payload
is comparable.

The baseline rulebook explicitly notes union equality lowering is still
pending.

P11 leaves it pending.

Do not synthesize recursive equality here.

---

# 22. No runtime dependency

Enum and union semantic operations are static compiler IR constructs.

No runtime variant table, reflection table, allocation service or type registry
is introduced.

---

# 23. Verification pipeline

Schema-v7 compiler-generated output should pass:

```text
normal MLIR verification
sec-verify-checked-integer-guards when applicable
sec-verify-result-guards when applicable
sec-verify-try-handlers when applicable
sec-verify-union-guards when union projections exist
```

---

# 24. Completion

Lowering specification version 7 is implemented when:

```text
enum identity survives scalar/integer lowering stages
default enum underlying int resolves by target plan
bit-backed width remains exact
ArithmeticError uses ordinary enum representation
Result enum errors remain high-level
union active variant remains semantic
union projections remain guarded
trivial payload action is explicit
non-trivial ownership is rejected rather than hidden
no physical union layout is selected
no LLVM dialect is generated
```

---

# 25. Match CFG lowering

Lowering consumes verified Semantic IR and its Sema-resolved match provenance.
It evaluates one subject SSA value, emits source-order pattern blocks, evaluates
guards only after pattern success, and uses one typed merge argument for match
expressions or a continuation block for match statements.

Enum patterns use `sec.enum.constant` plus `sec.enum.cmp eq`; union and Option
patterns use `sec.union.is_variant` and guarded copy-trivial projection; Result
patterns use `sec.result.is_err` and guarded unwrap operations. An impossible
exhaustive residual emits `sec.unreachable`. The output carries deterministic
function-local match provenance and runs `sec-verify-match-cfg` in addition to
applicable Result/union verifiers. No physical representation or switch-table
optimization is selected in this pass.

---

# 26. Struct type mapping

A verified Semantic IR `StructDefinition` maps to nominal `!sec.struct` with
its canonical identity, concrete type arguments, declaration-order stored
fields, names, types and tags. Properties are excluded.

# 27. Struct literal evaluation and construction

Lowering evaluates explicit fields and spread sources in source-entry order.
Each spread source is evaluated once and immediately expanded through
`sec.struct.spread_fields`. Omitted canonical semantic defaults are materialized
after source entries, recursively in declaration order. Final values are then
selected according to the resolved override plan and passed to one
`sec.struct.construct` in field declaration order. Lowering consumes the
read-only Sema plan and `DefaultResolution`; it must not use AST-appended
compatibility defaults, assume zero, or emit `undef`/poison.

# 28. Struct fields and mutable storage

Resolved stored-field reads lower to `sec.struct.extract`; properties do not.
For a trivial mutable local, lowering evaluates the RHS, loads the whole root
struct, applies `sec.struct.replace_field`, and stores the replacement. Nested
assignment loads the root once, extracts nested aggregates, rebuilds leaf to
root, and stores the root exactly once. Struct storage remains high-level and
is not converted to MemRef by the trivial-core pass.

# 29. Parameters, results and scalar passes

Struct parameters and results retain `!sec.struct`; aggregate ABI
classification is deferred. Target-sized scalar resolution recursively updates
field and type-argument types while preserving the nominal wrapper, field
identity and tags. Checked-integer signless normalization does not recurse into
struct fields.

# 30. Struct-like union payload binding

On a proven matching struct-like union path, lowering projects every payload
field with guarded `sec.union.unwrap_field`, constructs the deterministic
synthetic payload `!sec.struct`, and binds that whole value. The representation
is a semantic view, not a second physical payload layout. P13 permits only the
copy-trivial subset.

# 31. Ownership, destruction and layout gates

P13 accepts only `construct-direct` and `copy-trivial` complete value actions.
Move, semantic copy, borrows, conditional or non-copyable transfer, partial
move and non-trivial destruction are rejected explicitly until their canonical
lowering exists. Struct equality, physical fields/offsets, LLVM aggregate
operations, aggregate ABI and runtime dependencies remain out of scope.

# 32. Lowering-v9 completion

Version 9 is complete when source order, spread source-once behavior, explicit
defaults and origins, fully initialized declaration-order construction,
resolved field identity, trivial leaf-to-root replacement, high-level storage,
synthetic union payload materialization and ownership rejection all verify
without selecting physical layout.

---

# Lowering version 10 — fixed arrays

## v10.1 Type lowering

Semantic IR fixed array:

```text
element TypeID
exact length N
```

maps to:

```text
!sec.array<lowered-element-type, "N">
```

where N is canonical arbitrary-precision decimal text.

No host integer conversion.

---

## v10.2 Literal construction

Lower each Semantic IR array construction segment to one
`sec.array.construct` operand.

Preserve:

```text
source segment order
segment kind
segment exact length
segment transfer action
```

Do not expand spread segments.

---

## v10.3 Ordinary element segment

Lower the already-evaluated element value as one operand.

Action:

```text
construct-direct
```

---

## v10.4 Spread segment

Lower the already-evaluated source fixed-array value as one operand.

P14 requires:

```text
copy-trivial
```

No temporary dynamic storage.

No per-element IR expansion.

---

## v10.5 Default lowering

Semantic IR ArrayDefault maps to:

```text
sec.array.default
```

Do not generate O(N) default operands.

No `undef` or poison.

---

## v10.6 Array length

Semantic IR ArrayLength maps to:

```text
sec.array.len
```

The operation remains high-level and compile-time foldable.

P6 may later resolve its uint representation.

---

## v10.7 Proven-safe index

For a Sema-proven access:

```text
lower index expression once
sec.array.extract / replace
bounds_kind = proven-safe
bounds_proof = recorded proof kind
```

No runtime bounds branch.

---

## v10.8 Runtime-checked ordinary index

Lower:

```text
array expression once
index expression once
sec.array.index_in_bounds
cf.cond_br
```

Failure:

```text
sec.fail.bounds
```

Success:

```text
sec.array.extract / replace
```

---

## v10.9 Runtime-checked fallible index

Use the same predicate.

Failure constructs:

```text
IndexError.OutOfBounds
```

through canonical enum operations and enters existing Result/try handler flow.

Success performs guarded extraction/replacement.

No bounds panic endpoint exists on the handled failure edge.

---

## v10.10 IndexError representation

After P11:

```text
IndexError
```

is an ordinary canonical enum.

Do not add a dedicated index/bounds error MLIR type.

---

## v10.11 Local handler integration

P10/P11 local handler CFG is generic over representable enum errors.

P14 feeds it:

```text
IndexError.OutOfBounds
```

No bounds-specific handler IR is introduced.

---

## v10.12 Mutable local array replacement

For trivial mutable local storage:

```text
resolve target/index exactly once
validate bounds
evaluate RHS completely
storage.load array
sec.array.replace
storage.store result
```

Preserve source ordering.

---

## v10.13 Nested array replacement

Rebuild semantic arrays leaf-to-root.

Do not calculate physical element addresses.

Do not duplicate index expression evaluation.

---

## v10.14 P5 interaction

Fixed-array semantic storage remains high-level.

No MemRef conversion in lowering v10.

---

## v10.15 P6 interaction

Target scalar resolution may recurse through array element semantic types while
preserving array wrapper/length.

The plan-specific uint length validation must use the same CompilationPlan.

---

## v10.16 P8 interaction

Do not recursively normalize signed/unsigned element types inside high-level
array wrappers.

Index scalar computations may follow their separately resolved scalar lowering
rules.

---

## v10.17 P13 interaction

Structs and arrays may nest recursively as high-level aggregate types.

No physical nested aggregate layout is selected.

---

## v10.18 Effect/failure semantics

Ordinary unproven index:

```text
panic-capable bounds failure
```

Proven-safe index:

```text
no bounds panic effect
```

Fallible try index:

```text
IndexError flow
no bounds panic effect for the check
```

Operand effects remain.

---

## v10.19 Unsafe

The lowering does not contain an unchecked mode for ordinary array indexing.

`unsafe` does not remove `sec.array.index_in_bounds`.

---

## v10.20 No physical array lowering

Do not lower to:

```text
LLVM array
LLVM insertvalue/extractvalue
GEP
MemRef
byte storage
physical stride arithmetic
```

in version 10.

---

## v10.21 No aggregate equality/membership

Array equality and membership remain separate operator lowerings.

P14 only establishes the array semantic value/index primitives they may later
consume.

---

## v10.22 Optimization independence

Correctness does not depend on:

```text
constant folding
bounds-check elimination
SROA
mem2reg
CSE
DCE
```

Proven-safe classifications come from Sema.

Runtime checks are correct before optimization.

---

## v10.23 Completion

Lowering version 10 is implemented when:

```text
exact fixed length survives
spread remains compact
default remains compact and initialized
len remains semantic
index expressions evaluate once
proven-safe indexes avoid runtime checks
unproven safe indexes branch deterministically
ordinary failure reaches high-level bounds endpoint
fallible failure produces IndexError
trivial replacement remains semantic
array storage remains high-level
no physical representation is selected
```

---

# Lowering version 11 — places and direct references

## v11.1 Place lowering

Semantic IR place paths lower to `!sec.place` operations.

Use semantic:

```text
root
field ordinal
array index
union variant
dereference
```

Do not calculate physical offsets.

---

## v11.2 Immutable value place

An address-taken immutable SSA binding may lower to:

```text
sec.place.value
```

This does not immediately allocate physical storage.

A future representation pass may materialize an address if required.

---

## v11.3 Storage place

High-level semantic storage lowers to:

```text
sec.place.storage
```

while reference semantics remain unresolved.

Do not run P5 MemRef conversion on that storage first.

---

## v11.4 Field place

Lower P13 resolved stored-field projection to:

```text
sec.place.field
```

using field ordinal/type identity.

Do not lower property access as field place.

---

## v11.5 Fixed-array element place

Reuse P14 exact-once index evaluation and proof facts.

Proven-safe:

```text
sec.place.array_element
```

Runtime check:

```text
sec.place.array_index_in_bounds
cf.cond_br
success -> sec.place.array_element
failure -> ordinary P14 bounds failure path
```

No GEP.

---

## v11.6 Union payload place

On a P11/P12 proven variant path:

```text
sec.place.union_payload
```

No payload copy.

---

## v11.7 Reference borrow

Lower direct source borrow to:

```text
sec.ref.borrow_shared
sec.ref.borrow_mut
```

using Sema-resolved reference facts.

No physical address extraction.

**Amendment MD-084 (2026-10-21).** Reference facts travel in the canonical `sec.reference` dictionary, operation properties, `sec.return_origins` and `sec.address_taken` spellings of `sec_mlir_dialect.md` v11.7 (amendment); lowering and optimization may remove a fact only after its obligation is discharged and verified, and every pass that keeps high-level references preserves them.

---

## v11.8 Reborrow

Lower Sema-resolved reborrow to:

```text
sec.ref.reborrow_shared
sec.ref.reborrow_mut
```

Preserve narrowed authority/lifetime/origin.

---

## v11.9 Reference validity

Proven reference:

```text
no runtime sec.ref.is_valid
```

Dynamic reference:

```text
sec.ref.is_valid
cf.cond_br
false -> sec.fail.reference_generation
true  -> reference use
```

The pass does not decide how `sec.ref.is_valid` is physically implemented.

---

## v11.10 Reference read

For P15 copy-trivial whole-value read:

```text
validated/proven ref
sec.place.deref
sec.place.read
```

Aggregate field reborrow may instead derive subplace without whole-value read.

---

## v11.11 Reference write

For P15 trivial replacement:

```text
validated/proven ref mut
sec.place.deref
optional subplace derivation
sec.place.write
```

No hidden destruction.

---

## v11.12 Shared copy

Sema-resolved shared reference copy lowers to:

```text
sec.ref.copy_shared
```

where an explicit semantic copy value is needed.

Using the same immutable SSA value directly is allowed only if doing so preserves
the resolved holder/liveness semantics and verifier expectations.

---

## v11.13 Mutable move

Sema-resolved `ref mut` move lowers to:

```text
sec.ref.move
```

when a distinct post-move reference SSA value is useful for liveness/provenance.

Do not copy.

---

## v11.14 End borrow

Emit:

```text
sec.ref.end_borrow
```

at resolved path-specific lifetime ends when required by Semantic IR liveness
metadata.

No runtime instruction is implied.

---

## v11.15 Reference equality

Lower source ref equality to:

```text
sec.ref.compare
```

after required validity proof/guard.

Do not lower to `cmpi` or raw pointer equality in P15.

---

## v11.16 Function parameters/results

Keep:

```text
!sec.ref<T>
!sec.ref_mut<T>
```

in high-level function signatures.

Preserve returned-reference origin summary metadata.

---

## v11.17 CompilationPlan reference policy

P15 lowering may inspect/record:

```text
logical epoch width
validity policy
relocation/address-space support
```

but it does not materialize runtime fields.

Default logical epoch width is 64 bits.

---

## v11.18 P5 interaction

`--sec-lower-trivial-core` must leave storage high-level when it is address-taken
or participates in place/reference semantics.

P15 should provide one authoritative query/attribute for this decision.

**Amendment MD-087 (2026-10-21).** The authoritative query is `sec::isReferenceDependentStorage` (P15-59); see `sec_mlir_dialect.md` v11.31 amendment.

---

## v11.19 P6 interaction

Scalar target resolution may recurse through referent type arguments while
preserving place/reference wrappers.

**Amendment MD-087 (2026-10-21).** Reference-bearing `sec.call.direct` stays a Sec call until lossless representation-aware call lowering exists (`sec_mlir_dialect.md` v11.32 amendment).

---

## v11.20 P8 interaction

Do not recursively signless-normalize semantic referent types inside place/ref
wrappers before dedicated representation lowering.

**Amendment MD-087 (2026-10-21).** P8 is selective, not module-rejecting: it lowers representable integer computation in mixed modules, preserves wrappers and metadata, and materializes legal verified `siN`/`uiN` ↔ `iN` transitions at Place, call, storage and CFG boundaries, never via `arith.bitcast` on signed types (`sec_mlir_dialect.md` v11.33 amendment). Implemented 2026-10-22 with exact-width `builtin.unrealized_conversion_cast` bridges that remain until physical reference-representation lowering reconciles them.

---

## v11.21 P12 integration

Match payload borrow actions now lower through:

```text
payload place
ref borrow
branch-scoped lifetime
end-borrow
```

No payload copy for borrowed binding.

---

## v11.22 P13 integration

Struct field borrowing derives:

```text
struct place
field place
reference
```

No whole struct load/rebuild.

---

## v11.23 P14 integration

Fixed-array element borrowing derives:

```text
array place
bounds proof/check
element place
reference
```

No whole array extraction/replacement.

---

## v11.24 Stale failure

Ordinary stale direct reference:

```text
sec.fail.reference_generation
```

No Result conversion.

Stable/weak handle resolution remains outside this pass.

---

## v11.25 Effects

Reference validity proof classification controls semantic panic effects.

Optional redundant hardening does not create a semantic panic effect when
failure is already statically impossible.

---

## v11.26 No runtime representation lowering

Do not lower to:

```text
memref as reference ABI
LLVM ptr
address + epoch struct
side table
capability
slot table
raw integer address
```

in lowering v11.

---

## v11.27 Optimization independence

Correctness must not depend on:

```text
mem2reg
SROA
CSE
DCE
inlining
optional bounds-check elimination
optional generation-check elimination
```

Proof versus dynamic validity is established before optional optimization.

---

## v11.28 Completion

Lowering version 11 is implemented when:

```text
places remain semantic
subplace borrowing avoids aggregate copies
shared/mutable authority is preserved
reference origin/epoch facts survive
proven references emit no required dynamic check
dynamic references have deterministic guard/failure CFG
ordinary stale failure does not become Result
reference equality remains semantic
borrow ends remain visible
address-taken storage remains high-level
no physical pointer/reference representation is chosen
```

---

## v11.29 Static Place roots (MD-116)

Added 2026-10-27 (`rules/corrections/applied/MD-116-static-place-root-mlir-correction-20261027.md`; dialect v11.41.3).
- **Emission.** The Go emitter lowers Semantic IR `place.static-root` to `sec.place.static`, with `global` naming the static's `sec.static.global` and Place root, storage identity and address space copied from the producer.
- **Static declarations.** Every emitted `sec.static.global` carries `sec.static_id` (the Semantic IR `StaticID`) and mirrors `Static.AddressTaken` as `sec.address_taken = true`. `sec.static.load`, `sec.static.store` and `sec.place.static` resolve through the same declaration.
- **Returned origins.** A returned reference borrowed from a static Place is summarized as `static@N`, with `N` the explicit ID.
- **Reference-dependent statics.** They stay high-level through P5, P6 and P8 (MD-087). `sec-lower-static-storage` lowers scalar-only statics as before and rejects any reference-dependent static it cannot represent. It never erases the Place, synthesizes a raw pointer or duplicates storage.
- **Failure.** Unsupported emission fails closed with no partial output.
- **Inheritance.** Schema 12 inherits this section unchanged.

---

## v11.30 Detached register values (MD-114)

Added 2026-10-27 (`rules/corrections/MD-114-canonical-register-value-mlir-contract-correction.md`; dialect v11.41.1a).
- **Register table.** The Go emitter writes `sec.registers` deterministically from the verified Semantic IR `Module.Registers` table, in its first-use order. Every field is included (reserved ones too), with its ordinal, logical offset, width, resolved type, effective access and shadow policy and unit identity. Definitions are never fabricated from operation-local types or guessed offsets.
- **Extract and update.** `register.value.extract` / `update` lower to `sec.register.extract` / `update`. Their resolved `RegisterFieldID` steps become `sec.field_path`, either as one-element paths per nested operation or as a proven composed path. Paths are never synthesized from source text.
- **Write proof.** `RegisterWriteProof` becomes `sec.write_proof` with the same basis and exact bounds. An update without an established proof is rejected, never emitted as an unchecked mask.
- **MD-110 conversions.** They resolve width through the same table. The Semantic IR basis `bit-extraction` has no MLIR spelling yet (MD-110 § 8(4)), so such conversions fail closed.
- **Hardware and unsupported cases.** Hardware operations stay with MD-077 / MD-111. Unrepresentable categories fail with no partial output.
- **Inheritance.** Schema 12 inherits this section unchanged.

---

# Lowering version 12 — borrowed slices

## v12.1 Type mapping

Shared slice semantic type:

```text
ref T[]
```

maps to:

```text
!sec.slice<T>
```

Mutable:

```text
ref mut T[]
```

maps to:

```text
!sec.slice_mut<T>
```

Do not map either to owning `T[]`.

---

## v12.2 Slice creation from fixed array

Lower:

```text
fixed-array Place
normalized start
normalized length
```

to:

```text
sec.slice.borrow_shared
or
sec.slice.borrow_mut
```

No array load/copy.

---

## v12.3 Slice reborrow

Existing slice source lowers through:

```text
source validity guard when required
range normalization/check
sec.slice.reborrow_shared / reborrow_mut
```

No element copy.

---

## v12.4 Range operand evaluation

Preserve:

```text
source
start
end
```

evaluation order.

Each expression evaluates once.

---

## v12.5 Proven range

Use:

```text
sec.slice.range_normalize
```

with proof provenance.

No runtime range branch.

---

## v12.6 Runtime range

Use:

```text
sec.slice.range_check
cf.cond_br failed
```

True is failure.

False is success.

---

## v12.7 Ordinary range failure

Failure:

```text
sec.fail.bounds
operation = slice-range
```

No backend trap chosen.

---

## v12.8 Fallible range failure

Failure uses the produced:

```text
RangeError
```

in existing Result/local-handler lowering.

No slice-specific Result representation.

---

## v12.9 Dynamic source validity

When the source is a dynamically validated existing slice:

```text
sec.ref.is_valid
cf.cond_br
false -> sec.fail.reference_generation
true  -> slice range/index/len operation
```

This validity branch precedes fallible range/index failure.

---

## v12.10 Shared slice copy

Lower resolved shared slice copy to:

```text
sec.slice.copy_shared
```

when a distinct semantic copy is represented.

Do not copy elements.

---

## v12.11 Mutable slice move

Lower resolved mutable slice transfer to:

```text
sec.slice.move_mut
```

when a distinct moved SSA value is represented.

Do not copy descriptor semantics.

---

## v12.12 Slice length

Both source `.Len` and `len(slice)` lower through `sec.slice.len -> !sec.uint`.
Preserve one operand evaluation, the exact unsigned count, non-consuming use,
no allocation and any required temporal-validity guard. There is no
`sec.slice.len_int` or signed representability gate. An explicit checked
conversion to a signed offset remains a separate operation
(MD-074/MD-088, the applied 2026-10-17 length correction §§ 1–3).

---

## v12.13 Slice index

Preserve:

```text
slice expression once
index expression once
validity guard when required
bounds guard when required
element Place
```

---

## v12.14 Proven slice index

Emit:

```text
sec.slice.element_place
bounds_kind = proven-safe
```

with proof provenance.

No runtime bounds branch.

---

## v12.15 Runtime slice index

Emit:

```text
sec.slice.index_in_bounds
cf.cond_br
```

Failure:

```text
ordinary -> sec.fail.bounds(slice-index)
fallible -> IndexError.OutOfBounds flow
```

Success:

```text
sec.slice.element_place
```

---

## v12.16 Element read

Use:

```text
sec.place.read
```

on the element Place.

P16 complete value-read subset remains copy-trivial.

---

## v12.17 Element write

Use:

```text
sec.place.write
```

on a writable mutable-slice element Place.

Do not add array-style whole-aggregate replacement because the slice does not
own the sequence.

---

## v12.18 Element borrow

Use:

```text
sec.ref.borrow_shared
sec.ref.borrow_mut
```

on the slice element Place.

No whole-sequence copy.

---

## v12.19 Slice borrow end

Use/generalize:

```text
sec.ref.end_borrow
```

for slice BorrowID/LifetimeID.

No runtime destructor.

---

## v12.20 Returning slices

Keep high-level slice type in function result.

Preserve returned-reference origin/range metadata.

No ABI descriptor choice.

---

## v12.21 Function parameters

Keep:

```text
!sec.slice<T>
!sec.slice_mut<T>
```

in high-level signatures.

Explicit fixed-array-to-slice source construction remains visible at the caller.

---

## v12.22 P13 integration

Struct fields containing slices remain high-level.

No descriptor field expansion.

---

## v12.23 P14 integration

Fixed-array source stays as high-level Place.

No fixed-array value copy is needed to create a slice.

---

## v12.24 P15 integration

Reuse:

```text
reference validity
place authority
storage identity
borrow/lifetime
epoch dependency
address stability and separate pin dependency
reference generation failure
```

No duplicate logic.

---

## v12.25 Dynamic owning array boundary

If the source slice plan identifies:

```text
owning dynamic T[]
```

and canonical owning-array Semantic IR is unavailable:

```text
return explicit UnsupportedFeatureError
```

Do not fake the owner as a slice.

---

## v12.26 Effect semantics

Ordinary runtime range/index failure:

```text
panic.bounds
```

Fallible range:

```text
RangeError
```

Fallible index:

```text
IndexError
```

Dynamic stale slice:

```text
panic.invalid-reference-generation
```

These effects remain distinct.

---

## v12.27 Unsafe

No P16 lowering path removes bounds or validity checks because the source is
inside `unsafe`.

Unchecked access remains RawPtr-only.

---

## v12.28 No equality lowering

Do not lower slice `==`/`!=`.

They are source-invalid in Sec 0.1.

---

## v12.29 No physical lowering

Do not lower to:

```text
LLVM ptr
LLVM struct
MemRef
address+length tuple
address+length+epoch tuple
capability
foreign span
```

in lowering version 12.

---

## v12.30 Optimization independence

Correctness does not depend on:

```text
range-check elimination
bounds-check elimination
generation-check elimination
CSE
SROA
mem2reg
inlining
```

Proof classification exists before optional optimization.

---

## v12.31 Completion

Lowering version 12 is complete when:

```text
explicit slice creation remains explicit
ranges normalize correctly
source/endpoints evaluate once
RangeError/panic paths are distinct
shared/mutable authority is preserved
slice reborrow narrows
slice indexing produces Places
P15 read/write/ref operations are reused
dynamic source validity precedes spatial failure
returned origin metadata survives
owning T[] remains distinct
no physical slice descriptor is chosen
```

---

## v12.32 Returned slice range origins (MD-115)

Added 2026-10-27 (`rules/corrections/applied/MD-115-returned-slice-origin-projections-correction-20261027.md`; dialect v12.34). This amends v12.20.
- **Range source.** Sema's resolved slice plans (`StaticSliceRange`, `SliceRangeID`, candidate origins) are the only source of a returned range. The builder never re-derives one from source expressions.
- **Exact ranges.** A statically normalized sub-range is emitted as `[A..<B]` relative to the original eligible origin. Nested exact ranges compose by arbitrary-precision addition.
- **Dynamic ranges.** A genuinely dynamic bound yields `[?]`, while the dynamic range checks and guards stay in the executable IR.
- **Full-open reborrows.** They keep the current origin, whole or narrowed.
- **Gaps fail closed.** An exact range that cannot yet be transported or verified keeps the function unsupported and is never weakened to `[?]` or the bare root. A compiler-local `SliceRangeID` is never serialized as a fabricated bound.
- **Schema selection.** Emission that needs these projections selects schema 12 explicitly or is rejected.
- **Unchanged.** Reference, epoch, pin and address-stability facts stay in their existing metadata.
