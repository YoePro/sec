# Sec MLIR Lowering

## Status

Normative lowering specification.

Current lowering specification version: `2`

This document is subordinate to:

```text
all applicable Sec language/domain rulebooks
rules/layout.md
rules/types.md
rules/semantic_ir.txt
rules/sec_mlir.md
rules/sec_mlir_dialect.md
```

It defines when resolved Sec MLIR semantics may be discharged into lower MLIR.

---

# 1. Version history

## Lowering specification version 1

Defined Package 5 trivial-core partial conversion:

```text
sec.const.bool -> arith.constant
trivial scalar storage -> rank-zero memref
sec.call.direct -> func.call
```

## Lowering specification version 2

Adds:

```text
mandatory correction: bool storage is not memref<i1>
plan-aware scalar type resolution
explicit DLTI index width requirement
wide scalar coverage
integer constant lowering after scalar resolution
canonical one-byte bool storage lowering
scalar-core pipeline
```

---

# 2. Lowering invariant

A Sec representation may be lowered only when all semantic obligations relevant
to that representation have been discharged.

Target-dependent lowering consumes the resolved CompilationPlan.

It never derives target properties from the host or target-name spelling.

---

# 3. Corrected trivial-core storage predicate

The Package 5 generic storage predicate accepts:

```text
fixed-width builtin integers 8/16/32/64/128/256
with signless/signed/unsigned MLIR signedness

f32
f64
```

It explicitly excludes:

```text
i1
```

because canonical addressable Sec bool storage occupies one byte.

It also excludes:

```text
index
all !sec.* semantic scalar types
named/distinct wrappers
decimal types
aggregate/custom unresolved types
```

---

# 4. Bool constant remains trivial

`sec.const.bool` still lowers to:

```text
arith.constant : i1
```

This is an SSA representation rule and does not imply one-bit addressable
storage.

---

# 5. Explicit target data

Plan-aware scalar lowering requires an explicit module data-layout specification:

```mlir
dlti.dl_spec = #dlti.dl_spec<
    #dlti.dl_entry<index, N>
>
```

where Package 6 supports:

```text
N = 32 or 64
```

The value is emitted from the active Sec CompilationPlan.

The scalar-resolution pass rejects a missing entry.

It does not use MLIR's default index bit width.

---

# 6. Sec int is not index

The data-layout index width is used as the resolved machine-word width query.

Lowering:

```text
!sec.int  -> siN
!sec.uint -> uiN
```

No Sec value becomes `index` merely because `int`/`uint` are pointer-sized.

---

# 7. Plan-aware scalar type resolution

Canonical rules:

```text
!sec.int   -> si32 or si64
!sec.uint  -> ui32 or ui64
!sec.float -> f64
!sec.char  -> ui8
!sec.rune  -> ui32
```

The active pointer width selects `int`/`uint`.

Other rules are target-independent scalar layout facts already fixed by the
Sec layout rulebook.

---

# 8. Fixed-width integer preservation

These remain exact:

```text
si8
si16
si32
si64
si128
si256

ui8
ui16
ui32
ui64
ui128
ui256
```

No width change.

No signedness normalization.

---

# 9. Signedness boundary

Version 2 does not convert:

```text
siN -> iN
uiN -> iN
```

This is deferred until semantic numeric operations choose exact signed or
unsigned Arith behavior.

The lower type alone must not erase information needed by future:

```text
division
comparison
shift
extension
conversion
ABI handling
```

---

# 10. Named/distinct preservation

Plan-aware scalar resolution may convert the base type:

```text
!sec.named<id, !sec.int>
    -> !sec.named<id, siN>
```

but must not remove the wrapper.

The same rule applies to `!sec.distinct`.

---

# 11. Decimal boundary

Remain high-level:

```text
!sec.decimal
!sec.decimal128
```

The canonical component layouts are known but aggregate lowering is not part of
this stage.

---

# 12. String and never boundary

Remain:

```text
!sec.string
!sec.never
```

---

# 13. Function/block conversion

Scalar-resolution type conversion applies consistently to:

```text
func.func signatures
entry block arguments
other block arguments
func.return
cf.br operands
cf.cond_br operands
func.call operands/results
sec.call.direct operands/results
sec.call.foreign operands/results
```

Use MLIR Dialect Conversion type/signature support.

---

# 14. Foreign calls

`sec.call.foreign` remains explicit.

Only its scalar operand/result types may become plan-resolved.

Do not discharge:

```text
ABI
calling convention
link name
foreign ownership contract
```

---

# 15. Integer constants

`sec.const.int` lowers to `arith.constant` after its result type is a plain
builtin integer type.

Supported widths include:

```text
8
16
32
64
128
256
```

Use arbitrary-precision integer construction.

No host-width truncation.

Named/distinct integer constants remain `sec.const.int`.

---

# 16. Float constants

`sec.const.float` remains high-level.

The fact that `!sec.float` resolves to `f64` does not define the source-literal
rounding rule.

---

# 17. Canonical bool storage

Semantic bool SSA:

```text
i1
```

Canonical addressable bool storage:

```text
memref<i8>
```

rank zero.

`!sec.storage<i1>` is a special lowering case and must never enter the generic
Package 5 `memref<T>` rewrite.

---

# 18. Bool declaration

```text
sec.storage.declare -> memref.alloca : memref<i8>
```

Preserve storage provenance attributes and location.

No dealloc for automatic storage.

---

# 19. Bool init/store

Before storing semantic `i1`:

```text
zero-extend i1 -> i8
memref.store
```

Safe lowering therefore materializes only byte values zero and one.

---

# 20. Bool load

```text
memref.load : memref<i8> -> i8
truncate i8 -> i1
```

Representation validity checks for unsafe/corrupted storage are outside this
stage.

---

# 21. Pre-correction bool memrefs

A provenance-marked canonical Sec bool storage represented as:

```text
memref<i1>
```

is invalid lowering-spec-v2 input.

Reject it clearly.

Do not heuristically rewrite arbitrary `memref<i1>` values.

---

# 22. `sec-resolve-scalar-layout`

Canonical independently invokable pass:

```text
--sec-resolve-scalar-layout
```

It:

```text
requires explicit DLTI index width
queries DataLayout once per module
resolves Sec semantic scalar types
converts signatures/block arguments/calls
lowers plain builtin integer constants
lowers canonical bool storage
preserves unresolved semantic types
```

It emits no LLVM dialect.

---

# 23. Scalar-core named pipeline

Canonical pipeline:

```text
--sec-lower-scalar-core
```

Semantically equivalent order:

```text
sec-lower-trivial-core
sec-resolve-scalar-layout
sec-lower-trivial-core
```

Pattern sharing may implement this more efficiently.

The result must be the same.

---

# 24. Legality

After `sec-resolve-scalar-layout`, these are illegal:

```text
!sec.int
!sec.uint
!sec.float
!sec.char
!sec.rune
```

These remain legal:

```text
!sec.string
!sec.decimal
!sec.decimal128
!sec.never
named/distinct wrappers
```

Plain-builtin-result `sec.const.int` is illegal and must lower.

Named/distinct-result `sec.const.int` remains legal.

`sec.call.foreign` remains legal.

---

# 25. No implicit default target

The pass must reject missing explicit plan/DLTI width even though upstream MLIR
defines a default index width.

Sec compilation is plan-specific.

A default belonging to MLIR infrastructure must not become an implicit Sec
language target choice.

---

# 26. Multi-output isolation

The same Semantic IR may be lowered independently under multiple plans.

No cached scalar result from one plan may be reused by another plan.

Example:

```text
plan A: pointer width 32 -> int si32
plan B: pointer width 64 -> int si64
```

Both are valid outputs from the same target-independent semantic source.

---

# 27. Idempotence

Both:

```text
sec-resolve-scalar-layout
sec-lower-scalar-core
```

are idempotent.

Successful second execution makes no additional semantic changes.

---

# 28. No unrealized conversion residue

Normal successful scalar resolution must not leave conversion casts introduced
only as unresolved plumbing.

Use standard signature/branch conversion infrastructure.

---

# 29. Source provenance

Every replacement operation preserves source location.

Bool storage's extension/truncation operations should use the originating
storage operation location.

---

# 30. Next lowering boundary

The next package should define semantic numeric operations and only then decide
how/when to normalize signed/unsigned integer types to signless integer
representation for Arith.

It must define:

```text
signed and unsigned arithmetic mapping
comparison predicates
division
shifts
integer conversions
overflow semantics
division-by-zero semantics
float literal rounding
```

before erasing integer signedness from the type layer.
