# Sec MLIR Lowering

## Status

Normative lowering specification.

Current lowering specification version: `11`

Version 11 defines lowering from canonical Semantic IR places/direct references
to high-level schema-v11 Sec MLIR.

It deliberately stops before physical pointer/reference representation.

---

# 1. Place lowering

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

# 2. Immutable value place

An address-taken immutable SSA binding may lower to:

```text
sec.place.value
```

This does not immediately allocate physical storage.

A future representation pass may materialize an address if required.

---

# 3. Storage place

High-level semantic storage lowers to:

```text
sec.place.storage
```

while reference semantics remain unresolved.

Do not run P5 MemRef conversion on that storage first.

---

# 4. Field place

Lower P13 resolved stored-field projection to:

```text
sec.place.field
```

using field ordinal/type identity.

Do not lower property access as field place.

---

# 5. Fixed-array element place

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

# 6. Union payload place

On a P11/P12 proven variant path:

```text
sec.place.union_payload
```

No payload copy.

---

# 7. Reference borrow

Lower direct source borrow to:

```text
sec.ref.borrow_shared
sec.ref.borrow_mut
```

using Sema-resolved reference facts.

No physical address extraction.

---

# 8. Reborrow

Lower Sema-resolved reborrow to:

```text
sec.ref.reborrow_shared
sec.ref.reborrow_mut
```

Preserve narrowed authority/lifetime/origin.

---

# 9. Reference validity

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

# 10. Reference read

For P15 copy-trivial whole-value read:

```text
validated/proven ref
sec.place.deref
sec.place.read
```

Aggregate field reborrow may instead derive subplace without whole-value read.

---

# 11. Reference write

For P15 trivial replacement:

```text
validated/proven ref mut
sec.place.deref
optional subplace derivation
sec.place.write
```

No hidden destruction.

---

# 12. Shared copy

Sema-resolved shared reference copy lowers to:

```text
sec.ref.copy_shared
```

where an explicit semantic copy value is needed.

Using the same immutable SSA value directly is allowed only if doing so preserves
the resolved holder/liveness semantics and verifier expectations.

---

# 13. Mutable move

Sema-resolved `ref mut` move lowers to:

```text
sec.ref.move
```

when a distinct post-move reference SSA value is useful for liveness/provenance.

Do not copy.

---

# 14. End borrow

Emit:

```text
sec.ref.end_borrow
```

at resolved path-specific lifetime ends when required by Semantic IR liveness
metadata.

No runtime instruction is implied.

---

# 15. Reference equality

Lower source ref equality to:

```text
sec.ref.compare
```

after required validity proof/guard.

Do not lower to `cmpi` or raw pointer equality in P15.

---

# 16. Function parameters/results

Keep:

```text
!sec.ref<T>
!sec.ref_mut<T>
```

in high-level function signatures.

Preserve returned-reference origin summary metadata.

---

# 17. CompilationPlan reference policy

P15 lowering may inspect/record:

```text
logical epoch width
validity policy
relocation/address-space support
```

but it does not materialize runtime fields.

Default logical epoch width is 64 bits.

---

# 18. P5 interaction

`--sec-lower-trivial-core` must leave storage high-level when it is address-taken
or participates in place/reference semantics.

P15 should provide one authoritative query/attribute for this decision.

---

# 19. P6 interaction

Scalar target resolution may recurse through referent type arguments while
preserving place/reference wrappers.

---

# 20. P8 interaction

Do not recursively signless-normalize semantic referent types inside place/ref
wrappers before dedicated representation lowering.

---

# 21. P12 integration

Match payload borrow actions now lower through:

```text
payload place
ref borrow
branch-scoped lifetime
end-borrow
```

No payload copy for borrowed binding.

---

# 22. P13 integration

Struct field borrowing derives:

```text
struct place
field place
reference
```

No whole struct load/rebuild.

---

# 23. P14 integration

Fixed-array element borrowing derives:

```text
array place
bounds proof/check
element place
reference
```

No whole array extraction/replacement.

---

# 24. Stale failure

Ordinary stale direct reference:

```text
sec.fail.reference_generation
```

No Result conversion.

Stable/weak handle resolution remains outside this pass.

---

# 25. Effects

Reference validity proof classification controls semantic panic effects.

Optional redundant hardening does not create a semantic panic effect when
failure is already statically impossible.

---

# 26. No runtime representation lowering

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

# 27. Optimization independence

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

# 28. Completion

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

## Amendments (2026-10-21)

Historical numbering is preserved. The consolidated `rules/mlir/sec_mlir_lowering.md` amendments take precedence where they conflict:

- **MD-084** (v11.7): metadata transport and preservation.
- **MD-087** (v11.18–v11.20): the authoritative storage query, reference-bearing calls, and selective P8 lowering with legal, verified integer-representation transitions.
- **MD-116** (v11.29, 2026-10-27): `place.static-root` is emitted as `sec.place.static` against the static's `sec.static.global`, which carries `sec.static_id` and `sec.address_taken = true`. Reference-dependent statics stay high-level through P5/P6/P8. `sec-lower-static-storage` rejects any such static it cannot represent, and schema 12 inherits this (`rules/corrections/applied/MD-116-static-place-root-mlir-correction-20261027.md`).
- **MD-114** (v11.30, 2026-10-27): Semantic IR `Module.Registers`, `RegisterDefinition`, `RegisterFieldDefinition`, `RegisterFieldID` steps and `RegisterWriteProof` map exactly to `sec.registers`, `sec.field_path` and `sec.write_proof`. No names or bit layout are reconstructed after Sema (`rules/corrections/applied/MD-114-canonical-register-value-mlir-contract-correction-20261030.md`).
- **Native tagged values** (v11.33, 2026-10-30): `sec-lower-tagged-values` selects an internal LLVM struct for Result and union values with scalar payloads (discriminator plus non-overlapping payload slots), including calls, merges and entry-block storage; other payloads are rejected by name at the track entry (`rules/mlir/sec_mlir_lowering.md` v11.33; `rules/memory/layout.md` §§ 15–16).
