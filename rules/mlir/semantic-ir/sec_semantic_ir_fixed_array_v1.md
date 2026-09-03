# Semantic IR Amendment - Fixed Array Values

## Status

Normative amendment for:

```text
rules/compiler/semantic_ir.md
```

Package:

```text
SEC-MLIR-P14
```

Local predecessor:

```text
SEC-MLIR-P13
```

Repository baseline:

```text
152c772
```

This amendment defines canonical target-independent Semantic IR for fixed arrays.

---

# 1. Fixed array type

Semantic IR fixed-array type identity contains:

```text
element TypeID
exact non-negative arbitrary-precision length
```

The length is immutable semantic data.

Do not truncate it to host width.

---

# 2. Fixed versus dynamic array distinction

The fixed-array Semantic IR type is distinct from:

```text
dynamic owning array/sequence
slice
reference to array/slice
```

No negative-length sentinel exists in Semantic IR.

---

# 3. Target validation

Semantic IR may retain an exact fixed length independently of physical layout.

Before a target-dependent output requiring concrete representation:

```text
validate length fits target uint
validate complete physical layout when required
```

Multi-output compilation performs this independently per CompilationPlan.

---

# 4. Array construction

Semantic IR represents array-literal construction using compact ordered
segments.

Segment kinds:

```text
element
spread
```

One source literal entry corresponds to one segment.

Do not expand a fixed-array spread into one Semantic IR operand per copied
element.

---

# 5. Segment order

Segments remain in source order.

Source expressions are evaluated exactly once in source order before/while
their segment is created.

The result array semantic element order is the concatenation of the segment
element sequences.

---

# 6. Segment transfer

A spread segment records the resolved element transfer action.

Package 14 executable support accepts:

```text
copy-trivial
```

Only.

No hidden move or semantic copy.

---

# 7. Array default operation

Semantic IR provides a compact fixed-array default operation.

It means:

```text
construct exactly N initialized elements
each from the canonical element default
in increasing index order
```

For `N == 0`:

```text
construct no element
```

No O(N) IR expansion is required merely to state the semantic default.

---

# 8. Full initialization

A safe readable fixed-array Semantic IR value never contains:

```text
undefined element
poison element
partially initialized element set
```

---

# 9. Array length

Semantic IR provides an explicit pure ArrayLength operation.

For `T[N]` it returns:

```text
uint value N
```

It is compile-time foldable and does not inspect storage.

---

# 10. Index plans

Sema supplies an already-resolved fixed-array index plan containing:

```text
element type
index type
index signedness
constant index when known
proven-safe versus runtime-check classification
proof provenance
use context
transfer action
error type
```

Semantic IR does not redo bounds reasoning from AST syntax.

---

# 11. Bounds predicate

Semantic IR provides a total ArrayIndexInBounds operation.

For fixed length `N`:

```text
signed index:
    index >= 0 && index < N

unsigned index:
    index < N
```

It does not compute a physical address.

---

# 12. Element extraction

Semantic IR provides ArrayExtract.

It identifies:

```text
array value
index value
bounds classification/proof
transfer action
```

Package 14 source reads support:

```text
copy-trivial
```

only.

---

# 13. Element replacement

Semantic IR provides functional ArrayReplace for the P14 trivial subset.

It produces a new semantic array value with one element replaced.

It does not imply a physical store.

---

# 14. Runtime guarded projection

For runtime-check indexing, ArrayExtract/ArrayReplace is valid only on the true
path of the matching ArrayIndexInBounds predicate for the same array and index.

This relationship is verifier-enforced.

---

# 15. Proven-safe projection

For Sema-proven indexing, no runtime branch is required.

The operation retains compiler proof provenance.

The verifier does not re-run full range analysis.

---

# 16. Ordinary bounds failure

Semantic IR has a high-level non-returning BoundsFailure endpoint for ordinary
unhandled safe indexing failure.

Its panic reason is:

```text
panic.bounds
```

It does not choose a runtime symbol or backend trap.

---

# 17. Fallible bounds failure

Fallible fixed-array indexing uses the canonical core enum value:

```text
IndexError.OutOfBounds
```

and ordinary Result/try control flow.

No separate bounds-error representation exists.

---

# 18. Place semantics

Indexing is source-level place formation.

Package 14 materializes only the resolved:

```text
copy-trivial read
trivial replacement
```

uses.

Borrowed element places wait for canonical Place/Reference Semantic IR.

The array/index resolved plan must retain enough place/use classification for the
later reference package.

---

# 19. Mutable local array storage

Copy-trivial, trivially destructible fixed arrays may use high-level semantic
storage.

Element assignment may lower as:

```text
load whole array
ArrayReplace
store whole array
```

Physical aggregate storage remains separate.

---

# 20. Nested arrays

Nested fixed-array types and replacement remain recursive semantic aggregates.

Each dimension has its own exact length and bounds proof/check.

---

# 21. Ownership boundary

Fixed-array type metadata may describe non-trivial element ownership.

Package 14 complete value operations do not execute:

```text
move
semantic copy
borrow
non-trivial replacement destruction
reverse-order element destruction
```

Those require later ownership/reference/cleanup operations.

---

# 22. Struct/union integration

Fixed arrays may appear as:

```text
P13 struct fields
P11 union payloads
P12 match values
function arguments/results
```

when the current value-transfer subset supports them.

No nested aggregate receives a physical layout from P14.

---

# 23. Zero-length identity

For zero-length arrays:

```text
length == 0
no live elements
no valid index
no destruction
default does not construct element
```

Element type identity remains part of the array type even though no element
value exists.

---

# 24. Zero-sized element identity

Array element identity remains:

```text
array identity + semantic index
```

even when physical element addresses later coincide because element size/stride
is zero.

Semantic IR indexing therefore never uses address equality as element identity.

---

# 25. Verifier

Semantic IR verifier must validate:

```text
non-negative exact length
fixed/dynamic distinction
construction segment types/actions/length sum
default legality recorded by builder
ArrayLength result type
index operand integer semantics
constant proof metadata
runtime guard dominance
same array/index for guarded projection
extract result type
replace new-value/result type
trivial P14 ownership gate
ordinary versus fallible failure CFG
```

---

# 26. Deterministic printer

Print fixed-array lengths as canonical base-10 integers.

Print construction segments in source order.

Do not materialize expanded spread elements in printer output.
