# Sec MLIR Lowering

## Status

Normative lowering specification for the currently implemented Sec MLIR
lowering pipeline.

Initial lowering specification version: `1`

This document is subordinate to:

```text
all applicable Sec language/domain rulebooks
rules/semantic_ir.txt
rules/sec_mlir.md
rules/sec_mlir_dialect.md
```

It defines when already-resolved Sec MLIR semantics may be discharged into
lower MLIR representations.

It does not define Sec source-language behavior.

---

# 1. Core lowering principle

A Sec MLIR construct may be lowered only when the target representation
preserves every semantic obligation that still belongs to that construct.

Lowering must not:

```text
guess missing source semantics
guess ownership
guess target width
guess ABI
guess layout
guess failure behavior
erase named identity prematurely
erase foreign-boundary information prematurely
```

When a semantic obligation remains unresolved, the Sec operation or type remains
explicit.

---

# 2. Package 5 lowering stage

The first lowering stage is named:

```text
trivial core lowering
```

Canonical pass:

```text
--sec-lower-trivial-core
```

This stage is a partial conversion.

It lowers only:

```text
sec.const.bool
Package-3-compatible trivial local storage
sec.call.direct
```

It leaves other schema-v2 Sec constructs legal.

---

# 3. Dialect schema

Trivial core lowering consumes:

```text
Sec MLIR dialect schema version 2
```

and may produce a mixed module containing both:

```text
remaining schema-v2 Sec MLIR
lower standard MLIR
```

The Sec dialect schema version remains `2`.

Lowering state is not encoded as a new dialect schema version.

---

# 4. Required MLIR conversion model

The trivial core stage uses MLIR Dialect Conversion with:

```text
ConversionTarget
conversion patterns
narrow TypeConverter where needed
applyPartialConversion
```

Legality is explicit.

The Sec dialect is not globally illegal.

---

# 5. Legal target dialects

The trivial core stage may produce:

```text
builtin
arith
func
cf
memref
```

It does not produce LLVM dialect operations.

---

# 6. Boolean constant lowering

Schema-v2:

```text
sec.const.bool
```

lowers completely to:

```text
arith.constant
```

with result:

```text
i1
```

The operation location is preserved.

After successful trivial-core lowering, no `sec.const.bool` remains.

Reason:

schema version 2 already fixes Sec `bool` as MLIR `i1`; no unresolved Sec
semantic obligation remains on a boolean literal.

---

# 7. Integer constant boundary

`sec.const.int` does not lower in the trivial-core stage.

The stage does not decide:

```text
target-sized int/uint width
signed/unsigned normalization
later arithmetic signedness representation
```

Integer representation lowering belongs to the target/scalar lowering stage.

---

# 8. Floating constant boundary

`sec.const.float` does not lower in the trivial-core stage.

Final conversion of the preserved source lexeme to a concrete floating-point
bit pattern requires the normative Sec float-literal rounding policy.

The trivial-core stage does not invent that policy.

---

# 9. Decimal and string constant boundary

The trivial-core stage does not lower:

```text
sec.const.decimal
sec.const.string
```

Their runtime/physical representations are not defined by this stage.

---

# 10. Lowerable storage elements

`!sec.storage<T>` may lower during trivial-core lowering only when `T` is a
builtin scalar with already-fixed physical width and no remaining Sec identity.

Accepted:

```text
i1

fixed-width builtin IntegerType:
    width 8, 16, 32 or 64
    signless, signed or unsigned

f32
f64
```

Rejected for this stage:

```text
index
all !sec.* semantic scalar types
!sec.named<...>
!sec.distinct<...>
aggregates
other custom types
shaped types
nested storage
```

Named/distinct types are not recursively unwrapped.

---

# 11. Storage type lowering

For an accepted element `T`:

```text
!sec.storage<T>
```

lowers to:

```text
memref<T>
```

where the memref has rank zero.

This represents one scalar automatic storage location.

It does not represent a one-element array.

---

# 12. Storage declaration lowering

Lowerable:

```text
sec.storage.declare
```

becomes:

```text
memref.alloca
```

of rank-zero `memref<T>`.

The target operation preserves:

```text
MLIR Location
sec.storage_id
sec.source_name
sec.storage_class
sec.mutable
```

when those attributes were present and valid.

No `memref.dealloc` is emitted.

---

# 13. Storage initialization lowering

Lowerable:

```text
sec.storage.init
```

becomes:

```text
memref.store
```

with zero indices.

The first-initialization distinction is discharged because upstream Semantic IR
has already verified it and the trivial-core storage element has no non-trivial
destruction or ownership semantics.

---

# 14. Storage load lowering

Lowerable:

```text
sec.storage.load
```

becomes:

```text
memref.load
```

with zero indices.

The upstream operation represents a trivial value copy.

No borrow, move or semantic copy operation is implied.

---

# 15. Storage store lowering

Lowerable:

```text
sec.storage.store
```

becomes:

```text
memref.store
```

with zero indices.

This is legal only for the trivial-core subset.

The rule must not be applied to non-trivial replacement.

---

# 16. Automatic storage rationale

Package-3-compatible storage entering this lowering stage is:

```text
local automatic
trivially copyable
trivially destructible
non-reference
non-owning
non-escaping under the implemented call subset
```

For this subset, automatic stack-backed memref storage is semantics-preserving.

This rule does not apply to future escaping values, resources, borrows,
aggregates, closures, arenas or heap objects.

---

# 17. Non-lowerable storage

Storage with unresolved semantic element types remains as schema-v2 Sec MLIR.

Examples:

```text
!sec.storage<!sec.int>
!sec.storage<!sec.decimal>
!sec.storage<!sec.named<"main::ID", si32>>
```

Partial conversion succeeds with these operations still present.

---

# 18. Direct-call lowering

Schema-v2:

```text
sec.call.direct
```

lowers completely to:

```text
func.call
```

The lowering preserves:

```text
exact MLIR callee symbol
operand order
result order
result types
location
```

It does not perform name lookup or overload resolution.

---

# 19. Direct-call argument actions

The schema-v2 Package-3 call subset carries only:

```text
copy-trivial
```

After validation, this action is discharged by lowering to ordinary
`func.call`.

No `sec.argument_actions` attribute is required on the resulting call.

Unknown or future actions must cause conversion failure rather than being
discarded.

---

# 20. Foreign-call boundary

`sec.call.foreign` does not lower in the trivial-core stage.

The foreign boundary still has unresolved obligations including ABI,
calling-convention and external linking semantics.

It remains explicit Sec MLIR.

---

# 21. Function and CFG boundary

The stage does not rewrite:

```text
func.func
func.return
cf.br
cf.cond_br
MLIR block arguments
```

These were already selected by schema version 2.

Sec function metadata remains attached to `func.func`.

---

# 22. Source provenance

Every replacement operation preserves the source operation's MLIR Location.

Storage provenance metadata is preserved on the resulting `memref.alloca`.

Lowering must not reconstruct source provenance from textual names.

---

# 23. No signedness normalization

The trivial-core stage does not convert:

```text
si8 -> i8
si16 -> i16
si32 -> i32
si64 -> i64

ui8 -> i8
ui16 -> i16
ui32 -> i32
ui64 -> i64
```

Such normalization belongs to the later scalar/target representation stage.

---

# 24. No target-sized lowering

The stage leaves:

```text
!sec.int
!sec.uint
!sec.float
```

unchanged.

No pointer width, index width or default float width is guessed.

---

# 25. No named/distinct erasure

The stage leaves:

```text
!sec.named
!sec.distinct
```

unchanged.

Type identity is a semantic obligation and may be erased only after the
appropriate lowering rule explicitly discharges it.

---

# 26. Conversion completeness

For a storage chain classified as lowerable:

```text
declare
init
load
store
```

must all lower consistently.

A successful stage must not leave a schema-v2 storage operation consuming a
converted rank-zero memref for that same lowerable chain.

Conversion failure is preferable to inconsistent mixed representation.

---

# 27. No unrealized conversion residue

Normal successful trivial-core lowering must not leave newly introduced:

```text
builtin.unrealized_conversion_cast
```

All supported conversions are closed over their use sets.

---

# 28. Idempotence

The trivial-core stage is idempotent.

After one successful application:

```text
sec.const.bool is absent
sec.call.direct is absent
lowerable sec.storage.* chains are absent
```

A second application does not change semantics.

---

# 29. Determinism

The lowering stage introduces no nondeterministic global identity.

It preserves existing symbols and block order.

New operations are emitted at the replaced operation's position.

---

# 30. Optimization independence

Correctness of this lowering stage does not depend on:

```text
canonicalization
CSE
inlining
mem2reg
dead-code elimination
```

Optimization may run later.

Lowering and optimization remain conceptually separate.

---

# 31. Next lowering stage

The next normative lowering area is target/scalar representation.

It must define before implementation:

```text
target information representation
DLTI use where appropriate
!sec.int width
!sec.uint width
!sec.float representation
signed/unsigned fixed-width normalization
integer constant lowering
float literal rounding and lowering
function/block/call type conversion rules
```

The trivial-core stage must not anticipate those decisions.
