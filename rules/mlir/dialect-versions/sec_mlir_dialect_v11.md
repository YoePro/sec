# Sec MLIR Dialect

## Status

Normative detailed representation specification.

Current Sec MLIR dialect schema version: `11`

Schema version 11 introduces compiler-internal semantic places and direct safe
reference values.

The schema does not select one physical pointer/reference representation.

---

# 1. Version history

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
v11  places and direct safe references
```

Compiler-generated v11 modules carry:

```mlir
sec.dialect_version = 11 : i32
```

---

# 2. `!sec.place`

Conceptual syntax:

```text
!sec.place<T, "ro">
!sec.place<T, "rw">
```

`!sec.place` is compiler-internal.

It is not a source-visible type.

---

# 3. Place authority

Allowed:

```text
ro
rw
```

A place derivation may preserve or reduce authority.

It may not increase authority.

---

# 4. `!sec.ref<T>`

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

# 5. `!sec.ref_mut<T>`

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

# 6. Place root metadata

Root place operations carry canonical semantic metadata such as:

```text
sec.place_root_id
sec.storage_identity
sec.address_space
sec.source_name
```

`sec.source_name` is provenance only.

---

# 7. Reference metadata

Reference-producing operations carry:

```text
sec.borrow_id
sec.lifetime_id
sec.validity_policy
sec.validity_proof
sec.epoch_dependency
sec.relocation_class   (superseded by address_stability + pinned; see Amendments, MD-084)
sec.reference_origin
```

where applicable.

These attributes are semantic.

They do not force physical fields.

---

# 8. `sec.place.value`

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

# 9. `sec.place.storage`

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

# 10. `sec.place.field`

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

# 11. `sec.place.array_index_in_bounds`

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

# 12. `sec.place.array_element`

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

# 13. `sec.place.union_payload`

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

# 14. `sec.place.deref`

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

# 15. `sec.place.read`

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

# 16. `sec.place.write`

Operands:

```text
!sec.place<T,"rw">
T
```

No result.

The compiler-generation layer enforces trivial replacement safety.

---

# 17. `sec.ref.borrow_shared`

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

# 18. `sec.ref.borrow_mut`

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

# 19. `sec.ref.copy_shared`

Operand/result:

```text
!sec.ref<T> -> !sec.ref<T>
```

No ownership transfer of the referent.

No mutable authority.

---

# 20. `sec.ref.move`

Operand/result:

```text
!sec.ref_mut<T> -> !sec.ref_mut<T>
```

The operation represents semantic reference-value move.

It does not move referent storage.

---

# 21. `sec.ref.reborrow_shared`

Produces a new shared direct reference whose authority/lifetime/extent do not
exceed the source grant.

The source may be:

```text
shared
mutable
```

according to verified Sema facts.

---

# 22. `sec.ref.reborrow_mut`

Produces a constrained mutable reborrow.

Source authority must be mutable.

No shared-to-mutable reborrow.

---

# 23. `sec.ref.is_valid`

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

# 24. `sec.ref.compare`

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

# 25. `sec.ref.end_borrow`

No results.

Required:

```text
borrow_id
lifetime_id
```

Compile-time semantic lifetime marker.

---

# 26. `sec.fail.reference_generation`

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

# 27. Reference validity policy

Allowed compiler-generated policy values:

```text
proven
dynamic-epoch
```

A proven policy requires proof provenance.

A dynamic policy requires reference-guard validation at protected uses.

---

# 28. Epoch width is not a type parameter

Do not define:

```text
!sec.ref<T, epoch=64>
```

or equivalent source type identity.

Epoch width belongs to CompilationPlan representation policy.

---

# 29. Place escape rule

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

# 30. Reference function signatures

`!sec.ref<T>` and `!sec.ref_mut<T>` may appear in high-level function
parameters/results.

This does not select ABI representation.

---

# 31. P5 storage interaction

Storage that participates in a live place/reference identity remains high-level.

P5 must not lower it to MemRef before reference representation has been
discharged.

---

# 32. P6 interaction

Target scalar type resolution may recurse through the referent type while
preserving:

```text
place/ref wrapper
borrow metadata
reference semantics
```

---

# 33. P8 interaction

Signless normalization must not recursively erase semantic integer distinction
inside reference/place wrappers.

---

# 34. Verifiers

Schema v11 registers:

```text
--sec-verify-places
--sec-verify-reference-guards
--sec-verify-borrow-semantics
```

They verify emitted semantic structure.

They do not replace Sema source analysis.

---

# 35. `--sec-verify-places`

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

# 36. `--sec-verify-reference-guards`

Checks dynamic-validity protected use:

```text
same reference SSA
true-edge dominance
false edge reaches invalid-generation failure
```

Proven uses require proof metadata.

---

# 37. `--sec-verify-borrow-semantics`

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

# 38. No physical representation

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

# 39. Schema-v11 completion

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

# 40. Exact integer conversion contract transport

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

# 41. Early admissions on the single native track

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

Schemas 7–10 remain readable without these operations. Lowering of each one is
listed in `sec-check-lowering-support`; a module using any of them outside
schema 11 or later is rejected by the version gate of the installing package.

---

## Amendments (2026-10-21)

This snapshot keeps its historical paragraph numbering. Where it conflicts with the following owner-approved corrections, the consolidated `rules/mlir/sec_mlir_dialect.md` amendments take precedence:

- **MD-084** (v11.7): the canonical `sec.reference`, `sec.return_origins` grammar and `sec.address_taken` spellings. `sec.relocation_class` is superseded by `address_stability`, and pinning is separate (`rules/corrections/applied/md084-schema11-reference-metadata-correction-20261021.md`).
- **MD-085** (v11.37): path-aware structural return-origin verification with callee-summary instantiation (`rules/corrections/applied/md085-return-origin-verification-correction-20261021.md`).
- **MD-087** (v11.31–v11.33): the P5 storage query, P6 reference-bearing calls staying high-level, and selective P8 coexistence (`rules/corrections/applied/md087-p5-p6-p8-reference-boundaries-correction-20261021.md`).
- **MD-077** (v11.41.1, 2026-10-22): explicit early admission of `!sec.register<T>`, `!sec.hw_instance<T>`, `sec.register.extract` / `update` and `sec.hw.bind` / `observe` / `request_write` / `shadow_*` / `commit`, with mandatory structured metadata. Schemas 1–10 reject these constructs and schema 12 inherits them (`rules/corrections/applied/md077-register-semantic-ir-final-decision-20261022.md` § 19).
- **MD-110** (v11.41.2, 2026-10-25): explicit early admission of `sec.register.fits_bits`, `sec.register.from_bits` (with the mandatory `sec.register_fit_proof` basis `constant` / `source-domain` / `guarded`) and `sec.register.to_bits` for detached register ↔ integer bit-pattern conversion. Schemas 1–10 reject them and schema 12 inherits them (`rules/corrections/applied/md110-register-bit-pattern-conversion-ir-20261025.md` § 8).

- **MD-116** (v11.41.3, 2026-10-27): explicit schema-11 admission of `sec.place.static` (zero operands; `global`, `place_root_id`, `storage_identity`, `address_space` properties) as the MLIR form of the Semantic IR `place.static-root`, plus the mandatory `sec.static_id = N : i32` and, for referenced statics, `sec.address_taken = true` on `sec.static.global`. `static@N` names that explicit ID, never a parsed symbol suffix. Schemas 1–10 reject the operation, and schema 12 inherits it unchanged (`rules/corrections/applied/MD-116-static-place-root-mlir-correction-20261027.md`).
- **MD-114** (v11.41.1a, 2026-10-27): the canonical consolidated dialect spells the detached register family's module table `sec.registers` (typed `#sec.register_def` / `#sec.register_field` with nominal identity, logical width, explicit `order`, optional resolved `byte_order`, and every field including reserved ones with ordinal, offset, width, type, effective access/shadow and optional unit), the required `sec.field_path` (nonempty i32 ordinals through nested registers) on `sec.register.extract` / `update`, and the required `sec.write_proof = #sec.register_write_proof<basis = "typed" | "constant" | "interval", low, high>` on `update`. The MD-110 conversions resolve width through the same table. Schemas 1–10 reject these constructs and schema 12 inherits them (`rules/corrections/MD-114-canonical-register-value-mlir-contract-correction.md`).
