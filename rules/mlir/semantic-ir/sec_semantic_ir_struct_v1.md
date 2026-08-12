# Semantic IR Amendment - Struct Values

## Status

Normative amendment for:

```text
rules/semantic_ir.txt
```

Package:

```text
SEC-MLIR-P13
```

Repository baseline:

```text
152c772
```

This amendment defines the canonical target-independent Semantic IR model for
struct values.

---

# 1. Struct definitions

Semantic IR struct definitions retain:

```text
stable TypeID
declaration SymbolID
qualified nominal identity
concrete generic type arguments
stored fields in declaration order
stable field identity
field type identity
field tags
copy classification
trivial-destruction classification
defaultability
optional resolved LayoutRef
source locations
```

Properties and methods are not stored fields.

---

# 2. Field identity

A stored field is identified by:

```text
Struct TypeID
StructFieldID
```

Field IDs follow zero-based declaration order.

They are not byte offsets.

---

# 3. Empty structs

A struct definition may contain zero stored fields.

This is a valid semantic value type.

No artificial byte is introduced in Semantic IR.

---

# 4. Concrete generics

Runtime struct values use concrete generic type arguments.

Unresolved generic field placeholders do not appear in runtime struct
operations.

---

# 5. Struct construction

Semantic IR represents struct construction explicitly.

Construction contains:

```text
one field value per stored field
field values in declaration order
field initialization origin
field transfer action
source location
```

Construction produces one fully initialized struct value.

---

# 6. Full initialization invariant

No readable struct value may contain:

```text
undefined field
poison field
uninitialized field
missing required field
```

Every stored field has a semantic value before construction completes.

---

# 7. Initialization origins

Semantic IR distinguishes:

```text
explicit source initialization
spread-provided initialization
omitted-field default initialization
aggregate default construction
```

This distinction must remain visible for diagnostics and verification.

---

# 8. Default construction

Omitted fields consume Sema's canonical `DefaultResolution`.

Semantic IR does not infer default from backend representation.

It does not equate default initialization with zeroed bytes.

---

# 9. Source evaluation versus field order

Source struct literal entries evaluate in source order.

Final struct field operands use declaration order.

The builder may store already-evaluated SSA values and then reorder only those
SSA references into canonical field order.

It must never reorder source expression evaluation.

---

# 10. Struct spread

Struct spread is explicit before normalization.

A spread operation records:

```text
source struct value
source evaluation point
field declaration order
per-field transfer action
```

The P13 compiler-generated subset supports copy-trivial field transfer.

---

# 11. Field extraction

Semantic IR uses explicit field extraction.

The operation records:

```text
source struct
field identity
resolved transfer action
result type
```

P13 compiler output supports copy-trivial source field reads.

---

# 12. Field replacement

Semantic IR uses explicit functional field replacement for the P13 trivial
subset.

It produces a new struct semantic value.

It does not imply physical `insertvalue`, byte copy, field offset or storage
mutation.

---

# 13. Mutable local struct storage

Trivial struct values may be placed in semantic local storage.

Field assignment may lower through:

```text
load whole struct
replace semantic field
store whole struct
```

This is a semantic representation.

Physical aggregate storage lowering remains separate.

---

# 14. Nested replacement

Nested trivial field assignment rebuilds affected aggregate values leaf-to-root
and commits the updated root after the right-hand side is safely evaluated.

This preserves source replacement ordering.

---

# 15. Ownership transfer

Struct construction/extraction must not hide ownership actions.

Resolved actions may include:

```text
construct-direct
copy-trivial
move
semantic-copy
borrow-shared
borrow-mutable
```

P13 executable support accepts only:

```text
construct-direct
copy-trivial
```

Other actions remain explicit unsupported until canonical ownership/reference
operations exist.

---

# 16. Non-trivial destruction

A struct definition may describe non-trivially destructible fields.

P13 does not execute those value paths unless cleanup semantics are already
represented.

No incomplete destructor is emitted.

---

# 17. Member resolution

Member syntax is not sufficient to identify a stored field.

Semantic IR field operations consume Sema-resolved member identity.

Properties and other members must not be converted to field operations by name
heuristics.

---

# 18. Struct-like union payload view

A struct-like union variant receives a canonical compiler-internal synthetic
struct value type for whole-payload use.

Identity derives from:

```text
union TypeID
variant index
```

It is a semantic whole-payload view.

It does not create a second physical layout.

---

# 19. Match integration

On a proven struct-like union variant path, the compiler may:

```text
extract each payload field
construct the synthetic payload struct
bind that struct to the match payload name
```

when every required transfer is copy-trivial.

This removes the P12 temporary whole-payload limitation for the trivial subset.

---

# 20. Physical layout separation

Struct Semantic IR does not encode physical field offsets merely because
declaration order is known.

Plan-resolved Semantic IR may reference canonical resolved layout metadata.

Backend lowering must consume that layout rather than independently calculating
another representation.

---

# 21. Verifier

Semantic IR verifier must validate:

```text
field ID sequence
field name uniqueness
field type validity
concrete generic substitution
construct field count/type/order
full initialization
origin/action arity
spread source/result type/order
extract field identity/type/action
replace field identity/type
trivial safety gate for P13 replacement/storage
synthetic union payload struct consistency
```

---

# 22. Deterministic printer

Print:

```text
struct definitions in deterministic type order
fields in declaration order
tags in source order
construction fields in declaration order
source-origin metadata deterministically
```

Do not use map iteration order.
