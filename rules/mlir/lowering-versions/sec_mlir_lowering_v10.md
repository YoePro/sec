# Sec MLIR Lowering

## Status

Normative lowering specification.

Current lowering specification version: `10`

Version 10 defines lowering from fixed-array Semantic IR to high-level schema-v10
Sec MLIR.

Physical fixed-array layout remains deferred.

---

# 1. Type lowering

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

# 2. Literal construction

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

# 3. Ordinary element segment

Lower the already-evaluated element value as one operand.

Action:

```text
construct-direct
```

---

# 4. Spread segment

Lower the already-evaluated source fixed-array value as one operand.

P14 requires:

```text
copy-trivial
```

No temporary dynamic storage.

No per-element IR expansion.

---

# 5. Default lowering

Semantic IR ArrayDefault maps to:

```text
sec.array.default
```

Do not generate O(N) default operands.

No `undef` or poison.

---

# 6. Array length

Semantic IR ArrayLength maps to:

```text
sec.array.len
```

The operation remains high-level and compile-time foldable.

P6 may later resolve its uint representation.

---

# 7. Proven-safe index

For a Sema-proven access:

```text
lower index expression once
sec.array.extract / replace
bounds_kind = proven-safe
bounds_proof = recorded proof kind
```

No runtime bounds branch.

---

# 8. Runtime-checked ordinary index

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

# 9. Runtime-checked fallible index

Use the same predicate.

Failure constructs:

```text
IndexError.OutOfBounds
```

through canonical enum operations and enters existing Result/try handler flow.

Success performs guarded extraction/replacement.

No bounds panic endpoint exists on the handled failure edge.

---

# 10. IndexError representation

After P11:

```text
IndexError
```

is an ordinary canonical enum.

Do not add a dedicated index/bounds error MLIR type.

---

# 11. Local handler integration

P10/P11 local handler CFG is generic over representable enum errors.

P14 feeds it:

```text
IndexError.OutOfBounds
```

No bounds-specific handler IR is introduced.

---

# 12. Mutable local array replacement

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

# 13. Nested array replacement

Rebuild semantic arrays leaf-to-root.

Do not calculate physical element addresses.

Do not duplicate index expression evaluation.

---

# 14. P5 interaction

Fixed-array semantic storage remains high-level.

No MemRef conversion in lowering v10.

---

# 15. P6 interaction

Target scalar resolution may recurse through array element semantic types while
preserving array wrapper/length.

The plan-specific uint length validation must use the same CompilationPlan.

---

# 16. P8 interaction

Do not recursively normalize signed/unsigned element types inside high-level
array wrappers.

Index scalar computations may follow their separately resolved scalar lowering
rules.

---

# 17. P13 interaction

Structs and arrays may nest recursively as high-level aggregate types.

No physical nested aggregate layout is selected.

---

# 18. Effect/failure semantics

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

# 19. Unsafe

The lowering does not contain an unchecked mode for ordinary array indexing.

`unsafe` does not remove `sec.array.index_in_bounds`.

---

# 20. No physical array lowering

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

# 21. No aggregate equality/membership

Array equality and membership remain separate operator lowerings.

P14 only establishes the array semantic value/index primitives they may later
consume.

---

# 22. Optimization independence

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

# 23. Completion

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
