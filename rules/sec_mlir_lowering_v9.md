# Sec MLIR Lowering

## Status

Normative lowering specification.

Current lowering specification version: `9`

Version 9 defines lowering from canonical Semantic IR struct values to
high-level Sec MLIR struct operations.

It does not lower structs to physical aggregate layout.

---

# 1. Input

Struct lowering consumes:

```text
verified Semantic IR StructDefinition
verified ResolvedStructLiteralPlan-derived construction
resolved member identity
resolved transfer actions
canonical DefaultResolution
```

It does not infer struct semantics from AST text.

---

# 2. Struct type mapping

Semantic IR StructDefinition maps to:

```text
!sec.struct
```

with:

```text
canonical type identity
concrete type arguments
stored fields in declaration order
field names/types/tags
```

Properties are excluded.

---

# 3. Literal lowering

For a resolved struct literal:

```text
1. evaluate source entries in source order
2. emit explicit spread operations at spread source positions
3. materialize omitted semantic defaults after source entries
4. select final field SSA values according to resolved override plan
5. emit one sec.struct.construct with fields in declaration order
```

Do not use AST-appended default fields as the plan.

---

# 4. Explicit field evaluation

An explicit source field expression executes at its source position.

Its resulting SSA value may be held until final declaration-order construction.

Do not reorder expression execution to field declaration order.

---

# 5. Spread lowering

For a spread source:

```text
evaluate source once
sec.struct.spread_fields
```

P13 accepts copy-trivial expanded field actions only.

Later source entries may override spread results according to the resolved plan.

---

# 6. Default lowering

An omitted field uses the resolved canonical semantic default.

Do not:

```text
assume zero
use undef
use poison
leave bytes uninitialized
```

Default origin remains visible on final construction.

---

# 7. Recursive struct defaults

A nested trivial struct default lowers recursively:

```text
field defaults
nested sec.struct.construct
outer sec.struct.construct
```

Field default construction follows declaration order.

---

# 8. Field extraction

Stored field read lowers to:

```text
sec.struct.extract
```

using resolved field ordinal and transfer action.

P13 end-to-end requires copy-trivial.

Property member access does not enter this lowering.

---

# 9. Mutable local storage

P13 trivial structs may use existing high-level semantic storage.

Do not lower struct storage to MemRef in this version.

---

# 10. Field replacement

For a mutable local trivial struct:

```text
evaluate new value
storage.load whole struct
sec.struct.replace_field
storage.store whole replacement
```

No physical field address is required.

---

# 11. Nested field replacement

Lower nested trivial assignment leaf-to-root:

```text
evaluate RHS
load root
extract nested aggregate(s)
replace leaf
replace parent(s)
store root once
```

This preserves semantic update ordering.

---

# 12. Struct parameters/results

Keep:

```text
!sec.struct
```

in `func.func` parameter/result types.

Do not choose aggregate ABI classification.

---

# 13. P5 interaction

P5's scalar storage conversion remains narrow.

Struct storage is legal high-level Sec storage and remains unlowered.

---

# 14. P6 interaction

Resolve target-sized scalar types recursively inside struct field types.

Preserve wrapper and nominal field identity.

---

# 15. P8 interaction

Do not recurse with signless integer normalization into struct fields.

Struct representation lowering comes later.

---

# 16. P11/P12 union payload integration

For a struct-like union whole-payload match binding:

```text
on proven variant path
unwrap each payload field
construct synthetic payload !sec.struct
bind result
```

No physical payload copy/layout is introduced.

---

# 17. Ownership gate

P13 accepts only:

```text
construct-direct
copy-trivial
```

for complete new-pipeline struct value operations.

Reject resolved:

```text
move
copy-semantic
borrow-shared
borrow-mutable
conditional/non-copyable transfer
```

until canonical ownership/reference lowering exists.

---

# 18. Destruction gate

Do not lower P13 replacement/storage paths for a struct whose relevant value
requires non-trivial destruction.

No implicit destructor or cleanup region.

---

# 19. Layout separation

Do not map `!sec.struct` to physical fields/offsets in lowering version 9.

Canonical layout resolution remains a separate stage.

---

# 20. No equality lowering

Do not lower struct equality in P13.

Frontend equality-comparable metadata may be preserved for a later aggregate
operator pass.

---

# 21. Optimization independence

Correctness does not depend on:

```text
CSE
canonicalize
SROA
mem2reg
LLVM insertvalue folding
```

High-level struct operations are correct before optimization.

---

# 22. Completion

Lowering version 9 is implemented when:

```text
source evaluation order is preserved
spread source-once semantics preserved
defaults are explicit semantic values
every construct is fully initialized
field access uses resolved identity
trivial replacement is semantic
struct storage remains high-level
union payload structs materialize canonically
no non-trivial ownership is hidden
no physical layout is selected
```
