# Sec MLIR Dialect

## Status

Normative detailed representation specification.

Current Sec MLIR dialect schema version: `9`

Schema version 9 adds canonical high-level struct value representation.

Physical struct layout remains deferred.

---

# 1. Version history

```text
v1  dialect foundation
v2  Semantic IR bridge
v3  scalar/target coverage
v4  checked integer operations
v5  typed arithmetic failure and Result construction
v6  Result branching/local try handlers
v7  enum and union values
v8  verified match CFG
v9  struct semantic values
```

Compiler-generated v9 modules carry:

```mlir
sec.dialect_version = 9 : i32
```

Schema versions 1 through 8 remain regression inputs.

---

# 2. Struct tag attribute

Conceptual syntax:

```text
#sec.struct_tag<"key", "value">
```

Tags are metadata.

They do not affect type compatibility in schema v9.

---

# 3. Struct field attribute

Conceptual syntax:

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
type valid
tags valid
```

---

# 4. `!sec.struct`

Conceptual syntax:

```text
!sec.struct<
    "type-id",
    [type-arguments],
    [
        #sec.struct_field<0, "first", T0, [...]>,
        #sec.struct_field<1, "second", T1, [...]>
    ]
>
```

Empty field lists are valid.

---

# 5. Struct type verifier

Require:

```text
type-id non-empty
runtime type arguments concrete
field ordinals contiguous from zero
field names unique
fields declaration ordered
no property metadata stored as fields
```

---

# 6. Nominal identity

Two struct types with different canonical type identities remain distinct even
when every field type/name/tag is identical.

No structural typing is introduced.

---

# 7. `sec.struct.construct`

Operands:

```text
one value per stored field
```

Result:

```text
one !sec.struct
```

Required attributes:

```text
field_origins
field_actions
```

Allowed origins:

```text
explicit
spread
default
```

Allowed schema-v9 compiler-generated actions:

```text
construct-direct
copy-trivial
```

---

# 8. Construct verifier

Verify:

```text
operand count == field count
each operand type == corresponding declaration-order field type
origin count == field count
action count == field count
origin values valid
action values valid
```

The operation always represents a fully initialized semantic struct.

---

# 9. `sec.struct.spread_fields`

Operand:

```text
source: !sec.struct
```

Results:

```text
one result per stored field in declaration order
```

Required:

```text
actions
```

Schema-v9 compiler output accepts:

```text
copy-trivial
```

for every result.

---

# 10. Spread verifier

Verify:

```text
result count == source field count
result types == source field types in declaration order
action count == result count
actions valid
```

The operation does not re-evaluate the source expression.

---

# 11. `sec.struct.extract`

Operand:

```text
source: !sec.struct
```

Required attributes:

```text
field ordinal
action
```

Result:

```text
selected field type
```

Schema-v9 compiler output uses:

```text
copy-trivial
```

for source field reads.

---

# 12. Extract verifier

Verify:

```text
field ordinal exists
result type exactly matches field type
action valid
```

---

# 13. `sec.struct.replace_field`

Operands:

```text
source struct
replacement field value
```

Required:

```text
field ordinal
```

Result:

```text
same struct type
```

Verifier:

```text
source/result exact struct type match
replacement type exact selected field type
field ordinal valid
```

The compiler-generation layer additionally enforces P13 trivial safety.

---

# 14. Functional aggregate semantics

`sec.struct.replace_field` means:

```text
new semantic value with one field replaced
```

It does not imply:

```text
physical insertvalue
field offset
memcpy
destruction
store
```

Physical lowering is later.

---

# 15. Operation purity boundary

Schema-v9 P13 forms are restricted to transfer actions that make the semantic
value operation pure:

```text
construct-direct
copy-trivial
```

Future ownership-aware actions may require new operations/effects.

Do not generalize P13 purity traits to future move/semantic-copy operations.

---

# 16. Default provenance

Compiler-generated omitted defaults preserve at least construction-level:

```text
field_origins = default
```

Optional discardable metadata may include:

```text
sec.default_kind
sec.default_synthesized
```

where useful.

No `undef`/poison default exists.

---

# 17. Struct storage

High-level semantic storage may contain:

```text
!sec.struct
```

for the P13 trivial subset.

Schema v9 does not define physical struct allocation/storage layout.

Existing storage operations remain high-level when their element is struct.

---

# 18. P5 boundary

`--sec-lower-trivial-core` must not convert:

```text
!sec.storage<!sec.struct<...>>
```

to MemRef in schema v9.

Scalar storage rules remain unchanged.

---

# 19. P6 scalar resolution

Target-sized scalar resolution may recurse through field types inside
`!sec.struct` while preserving:

```text
struct identity
field ordinal
field name
field tags
type argument identity
```

Nested struct fields recurse.

---

# 20. P8 signless boundary

Checked-integer signless normalization must not recurse through `!sec.struct`.

Dedicated aggregate lowering owns later representation erasure.

---

# 21. Layout metadata

If available, existing:

```text
sec.layout_ref
```

may identify canonical resolved struct layout.

Schema v9 type syntax does not embed byte offsets or padding.

---

# 22. Synthetic union payload struct

Schema-v9 may contain compiler-internal `!sec.struct` identities representing
the whole semantic payload of one struct-like union variant.

The identity is deterministic from union type identity + variant index.

Such a struct is not a separately materialized physical object unless later
lowering chooses one.

---

# 23. P12 match integration

On a proven matching struct-like union path:

```text
sec.union.unwrap_field
...
sec.struct.construct
```

may create the whole payload struct for a P12 binding.

All involved actions are copy-trivial in P13.

---

# 24. Properties

Properties do not appear in `!sec.struct`.

Property access must not lower to:

```text
sec.struct.extract
sec.struct.replace_field
```

merely because parser syntax uses member access.

---

# 25. Field tags

Struct tags are preserved on field attributes.

Do not hard-code recognized tag keys.

They are not physical layout directives in schema v9.

---

# 26. No physical layout

Schema v9 does not define:

```text
field offset
aggregate size
aggregate alignment
padding
packing
LLVM struct body
ABI classification
```

---

# 27. Schema-v9 completion

Schema v9 is complete when:

```text
tag/field attrs parse/print/verify
empty/nested/generic structs parse/print/verify
construct/spread/extract/replace operations verify
wide field types survive
target scalar resolution preserves wrapper
signless pass preserves wrapper
trivial high-level struct storage remains legal
synthetic union payload structs verify
schema-v8 regression remains valid
no physical struct layout is selected
```
